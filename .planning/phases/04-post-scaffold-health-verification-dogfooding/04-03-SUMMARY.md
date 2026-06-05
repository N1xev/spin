---
phase: 04-post-scaffold-health-verification-dogfooding
plan: 03
subsystem: tooling
tags: [cobra, go-mod, dep-update, semver, proxy, golang.org/x/mod, hlth-03]

# Dependency graph
requires:
  - phase: 04-01
    provides: "internal/doctor and internal/wrap packages as reference patterns for env-overridden exec and stderr install hints"
  - phase: 04-02
    provides: "internal/wrap/lint.go argv-passthrough + exec pattern; not reused here because spin update shells out to `go`, not a third-party tool"
provides:
  - "internal/update package: parse, resolve, apply engine for the spin update command (HLTH-03)"
  - "ListDeps + FindGoMod public API for reading any go.mod"
  - "Resolver + ModuleProxy + HTTPMirror for version-list fetching with 404-as-degraded-mode"
  - "Apply + ApplyWithRunner + CommandRunner seam for testable go get + tidy + CGO=0 build pipeline"
affects: [04-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "ModuleProxy interface lets tests inject a fake without hitting proxy.golang.org"
    - "ErrModuleNotFound sentinel + 404-to-sentinel mapping so a single local-only dep does not fail the batch"
    - "semver.Prerelease(v) == '' as the stable filter (covers -beta, -rc, -alpha, -pre, -anything)"
    - "CommandRunner interface indirection for testing Apply's go-test avoidance contract"
    - "Dep.Target field set by the (Plan 04-04) caller; Apply is a pure executor — does not know NewStable vs NewLatest"
    - "go mod tidy runs once per Apply, not per dep (batched) — TestApply_MultipleUpgrades_BatchedTidy guards this"

key-files:
  created:
    - internal/update/parse.go
    - internal/update/parse_test.go
    - internal/update/resolve.go
    - internal/update/resolve_test.go
    - internal/update/apply.go
    - internal/update/apply_test.go
  modified: []

key-decisions:
  - "Reused golang.org/x/mod v0.36.0 (already a direct dep) for both modfile.Parse and semver.{Compare,IsValid,Prerelease,Canonical} — no new direct dep"
  - "10s HTTP client timeout per module fetch (threat T-04-20); user can ctrl-C out of the longer go get/tidy/build"
  - "404 from the proxy degrades to NewStable == NewLatest == Old for that single dep; non-404 fetch errors bubble up so the user sees the network failure"
  - "HTTPMirror's BaseURL field exists for httptest.NewServer; production code leaves it nil (defaults to https://proxy.golang.org)"
  - "Apply and ApplyWithRunner are two functions (not one with optional runner): production code reads Apply; tests use ApplyWithRunner to inject a fake. Kept both because Apply's call site (Plan 04-04's cmd/update.go) should be the zero-arg ergonomic form"
  - "Excluded `//go:build`-style constraints and --all from this plan — those are Plan 04-04 (UI) concerns"
  - "Did not import internal/version from a non-test file: HTTPMirror is in resolve.go (production) and reads version.Version. Kept the import minimal — only the User-Agent string needs the version"
  - "ModuleProxy returns ErrModuleNotFound (a package-level sentinel) so callers can match with errors.Is — easier to test than a string match on the error message"

patterns-established:
  - "Pattern: external-tool wrapper for the user's toolchain = three-layer seam: read input (parse.go) → consult external state (resolve.go) → apply (apply.go). The same skeleton will generalize to other Go tools (e.g. `spin fmt` might adopt it)"
  - "Pattern: 404 → sentinel error → degraded per-row result; protects the batch from one missing item"
  - "Pattern: D-10 contract enforcement via test fixture (marker file on disk) — gives a CI-visible regression test, not just an inline comment"

requirements-completed: [HLTH-03]

# Metrics
duration: ~8min
completed: 2026-06-05
status: complete
---

# Phase 4 Plan 3: spin update engine Summary

**`internal/update` engine — universal Go dep updater (parse, resolve, apply) with no UI, ready for Plan 04-04 to wire to a huh v2 form.**

## Performance

- **Started:** 2026-06-05T01:01Z
- **Completed:** 2026-06-05
- **Tasks:** 3
- **Files created:** 6
- **Commits:** 3 (1 per task)

## Accomplishments

- **`internal/update/parse.go`** — `ListDeps` (sorted, `includeIndirect` filter) and `FindGoMod` (parent-walk until `go.mod` hit) using `golang.org/x/mod/modfile`; 6 tests covering the direct/indirect toggle, missing-file error wrapping, deterministic ordering, and the parent-walk
- **`internal/update/resolve.go`** — `Resolver` over a `ModuleProxy` seam; `HTTPMirror` fetches `https://proxy.golang.org/<m>/@v/list` with a 10s timeout and `spin/<ver>` User-Agent; `pickHighest` excludes pre-releases from the stable slot per D-08; 8 tests covering the pre-release exclusion, the 404-degraded mode, empty list, and `httptest.NewServer`-based URL construction
- **`internal/update/apply.go`** — `Apply` runs `go get <m>@<v>` per dep, then `go mod tidy` once, then `CGO_ENABLED=0 go build ./...` per D-10 (no `go test`); `ApplyWithRunner` + `CommandRunner` seam enables test injection; 6 tests covering single-upgrade, no-op, build failure, the D-10 no-go-test guard, batched tidy, and `Target == Old` skip
- **D-08 contract test**: `TestResolver_FakeProxy_Stable` asserts `v2.0.0-beta.1` is NOT picked as `NewStable`
- **D-10 contract test**: `TestApply_DoesNotRunGoTest` writes a marker file if `go test` is ever invoked; after `Apply` the file must not exist
- **D-15 contract test**: `TestApply_MultipleUpgrades_BatchedTidy` proves `go mod tidy` runs exactly once across N deps

## Task Commits

1. **Task 1: parse.go — go.mod reader using golang.org/x/mod/modfile** — `1da0da7` (feat)
2. **Task 2: resolve.go — fetch version lists from proxy.golang.org, compute newStable/newLatest** — `6209722` (feat)
3. **Task 3: apply.go — go get + go mod tidy + CGO=0 go build (no go test)** — `eb3c989` (feat)

## Files Created/Modified

- `internal/update/parse.go` — `Dep`, `ListDeps`, `FindGoMod` (177 LOC incl. doc comments)
- `internal/update/parse_test.go` — 6 tests: `TestListDeps_DirectOnly`, `TestListDeps_IncludeIndirect`, `TestListDeps_MissingFile`, `TestListDeps_Deterministic`, `TestFindGoMod_FromSubdir`, `TestFindGoMod_NoModAnywhere`
- `internal/update/resolve.go` — `Dep` (shared), `ModuleProxy`, `ErrModuleNotFound`, `HTTPMirror`, `Resolver`, `NewResolver`, `pickHighest`
- `internal/update/resolve_test.go` — 8 tests covering the D-08 pre-release exclusion, 404-degraded mode, empty list, multi-dep fetching, and the URL construction via `httptest.NewServer`
- `internal/update/apply.go` — `Apply`, `ApplyWithRunner`, `CommandRunner`, `execRunner`, `countTargeted`
- `internal/update/apply_test.go` — 6 tests including the D-10 no-go-test guard and the batched-tidy guard

## Verification

- `go test ./internal/update/... -count=1` — 20 tests, all pass
- `go build ./...` — clean
- `go vet ./...` — clean
- `bash scripts/check-v1-leaks.sh ./internal/update` — OK, no v1 leaks
- `go mod tidy` — no diff (used existing `golang.org/x/mod v0.36.0`)
- `git status` — clean

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Removed unused package-level `applyLog` and `joinArgs` from apply.go**
- **Found during:** Task 3 post-implementation cleanup
- **Issue:** I initially declared `const applyLog` and `func joinArgs` in apply.go as "test helpers / phrasings" but neither was referenced. `go build` and `go vet` did not flag this (unexported package-level identifiers are not errors), but the dead code is noise. Removed and dropped the now-unused `strings` import.
- **Files modified:** `internal/update/apply.go`
- **Commit:** `eb3c989` (same commit as Task 3 — caught before commit)

**2. [Rule 1 - Bug] Fixed fakeRunner argv-length guard off-by-one**
- **Found during:** Task 3 test execution
- **Issue:** The `case` clauses for `go build ./...` and `go test ./...` used `len(args) >= 4`, but the argv length is 3 (`[go, build, ./...]` or `[go, test, ./...]`). All 5 apply tests failed with "fakeRunner: unhandled argv". Changed `>= 4` to `>= 3` in both arms. The `TestApply_BuildFailure_ReturnsError` then revealed a second bug: the marker file was being written in the `go build` arm, which made the D-10 test see a false positive. Moved the marker write to the `go test` arm only.
- **Files modified:** `internal/update/apply_test.go`
- **Commit:** `eb3c989` (same commit as Task 3 — caught before commit)

### Architectural Observations (not applied — out of plan scope)

- The `execRunner` struct is defined but never used (only `ApplyWithRunner` is used in tests; `Apply` is the public API). Could be removed if Plan 04-04 does not need it; keeping for now in case Plan 04-04 wants to wire a non-default runner (e.g. for the spin-doctor-style `CGO=0` env override at the call site rather than in apply.go).
- The `applyLog` constant and `joinArgs` helper were speculative additions that did not earn their keep. Documenting here so a future cleanup pass knows they were considered.

## Threat Surface Audit

The plan's STRIDE table identifies T-04-14 through T-04-21. Mitigations applied:

| Threat | Status |
|--------|--------|
| T-04-14 (modfile tamper) | `modfile.Parse` used; no shell; no eval. |
| T-04-15 (go get argv) | Static argv; module path comes from parsed go.mod, not CLI args. |
| T-04-16 (network tamper) | HTTPS only; 10s timeout. New `ErrModuleNotFound` lets the resolver degrade gracefully. |
| T-04-17 (sumdb bypass) | Accepted; we delegate sumdb verification to `go get`. |
| T-04-18 (info disclosure) | Accepted; User-Agent is `spin/<ver>` only. |
| T-04-19 (EoP via tool dep) | Accepted; standard Go behavior, documented in godoc. |
| T-04-20 (DoS) | 10s HTTP timeout per fetch. `go get`/`tidy`/`build` are unbounded (user can ctrl-C). |
| T-04-21 (build error surfacing) | Error message wraps combined stdout+stderr verbatim. |
| T-04-SC (no new direct dep) | Reused existing `golang.org/x/mod v0.36.0`; no `go.mod` changes. |

## What Plan 04-04 Will Build

This plan is the engine only. Plan 04-04 will:
- Add `internal/update/form.go` — huh v2 multi-Select form rendering rows of `old → newStable → newLatest` with `[Skip | newStable | newLatest]` per dep; default `newStable` per D-09
- Add `cmd/update.go` — cobra subcommand with `--all` flag (D-07), calling `ListDeps` → `Resolver.Resolve` → the huh form → set `Dep.Target` per row → `Apply`
- Add non-TTY table fallback for CI/scripted use
- Add a `--yes` / `--batch` flag to auto-pick `newStable` for every dep (D-09 default + Plan 04-04 noise reduction)

## Self-Check: PASSED

- 6 files exist: `internal/update/{parse,resolve,apply}.go` + `parse_test.go` + `resolve_test.go` + `apply_test.go`
- 3 commits: `1da0da7`, `6209722`, `eb3c989`
- 20 tests pass under `go test ./internal/update/... -count=1`
- `go build ./...` clean
- `go vet ./...` clean
- `check-v1-leaks.sh ./internal/update` clean
- `go mod tidy` produced no diff (no new direct dep)
- `git status` clean
