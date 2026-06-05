---
phase: 04-post-scaffold-health-verification-dogfooding
plan: 02
subsystem: tooling
tags: [cobra, golangci-lint, wrap, passthrough, install-hint, golang]

# Dependency graph
requires:
  - phase: 04-01
    provides: "internal/doctor DeepLintCheck with inline golangci-lint exec and shared install-hint string"
provides:
  - "`spin lint` cobra subcommand wrapping `golangci-lint` with argv passthrough"
  - "wrap.Lint(args) function with detect+exec+install-hint pattern"
  - "Discoverable install command in `spin lint --help` Long text"
affects: [04-03, 04-04, 04-05, 04-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "wrap.Lint reuses runTool() from detect.go for argv-passthrough exec (no new wrapper needed)"
    - "Const golangciLintInstallHint duplicated on each side (wrap and doctor) to avoid cross-package coupling"
    - "Test pattern: set PATH to empty tempdir to force the not-found branch, then assert on captured stderr"

key-files:
  created:
    - internal/wrap/lint.go
    - internal/wrap/lint_test.go
    - cmd/lint.go
    - cmd/lint_test.go
  modified: []

key-decisions:
  - "Did NOT use RunWithFallback (no sensible fallback for a linter — silently running `go vet` would downgrade the user's lint signal without consent)"
  - "No spin-specific flags — the lint command's flag set mirrors golangci-lint's by passing through (per CONTEXT Claude's Discretion)"
  - "Install hint printed to stderr (not stdout) so it does not pollute piped tool output"
  - "Returned error from Lint is `fmt.Errorf` (not the underlying exec.LookPath err) so the cobra RunE surfaces a clean 'golangci-lint not found' message"

patterns-established:
  - "Pattern: thin tool-wrapper subcommand = `internal/wrap/<tool>.go` (detect+exec) + `cmd/<tool>.go` (ArbitraryArgs RunE) — add no spin-specific flags unless the tool needs spin-only behavior"
  - "Pattern: test argv-passthrough with a shell-shim in a tempdir; the shim writes its $@ to a marker file for assertion"

requirements-completed: [HLTH-01]

# Metrics
duration: ~10min
completed: 2026-06-05
status: complete
---

# Phase 4 Plan 2: spin lint Summary

**`spin lint` cobra subcommand wrapping `golangci-lint` with argv passthrough and a discoverable install hint**

## Performance

- **Started:** 2026-06-05T00:00Z (approx)
- **Completed:** 2026-06-05
- **Tasks:** 2
- **Files created:** 4

## Accomplishments

- `wrap.Lint(args)` — detects `golangci-lint` on $PATH, exec's it with argv passthrough, or returns a non-nil error + one-line install hint on stderr when missing
- `spin lint` cobra subcommand — `ArbitraryArgs`, RunE forwards to `wrap.Lint`, Long help text embeds the install recipe so `spin lint --help` doubles as a recovery path
- 9 new unit tests across 2 packages, all passing
- End-to-end verified: `spin lint version` exec's `golangci-lint version` and shows `golangci-lint has version v1.64.8`; `spin lint --help` renders fang-styled help with the install command

## Task Commits

1. **Task 1: Implement internal/wrap/lint.go (detect + exec + hint)** - `5a1f6e0` (feat)
2. **Task 2: Wire cmd/lint.go cobra subcommand + tests** - `b5d93b9` (feat)

## Files Created/Modified

- `internal/wrap/lint.go` — `Lint(args []string) error` + `golangciLintInstallHint` const
- `internal/wrap/lint_test.go` — 4 tests: not-on-path error, stderr hint content, argv passthrough via shell shim, const shape regression
- `cmd/lint.go` — `lintCmd` (`ArbitraryArgs`, RunE forwards to wrap.Lint) + `init()` registration
- `cmd/lint_test.go` — 5 tests: registration by pointer identity, Use mentions "golangci-lint args", Long mentions install hint, RunE wiring, rendered help contains install recipe

## Decisions Made

None - plan executed exactly as written.

## Deviations from Plan

None - the plan called for 4 tests in `cmd/lint_test.go`; 5 were added (the 5th is a parallel of `TestDoctorCmd_Help` in `cmd/doctor_test.go` — a regression catcher for the help-text rendering). No deviation from the spec, only an additional test.

## Threat Model Coverage

All five STRIDE entries applied:

- **T-04-09 (Tampering / argv forwarding):** `runTool` uses `exec.Cmd` with no shell interpolation. golangci-lint handles its own flag validation. The argv passthrough is exercised by `TestLint_ArgvPassThrough`.
- **T-04-10 (Info Disclosure / install command on stderr):** accepted — the recipe is the canonical public hint, also published in CLAUDE.md.
- **T-04-11 (Elevation / env):** `runTool` does not modify inherited env (PATH, GOPATH, HOME all pass through).
- **T-04-12 (DoS / hang):** accepted — users can pass `--timeout` themselves.
- **T-04-13 (Repudiation / non-zero exit):** `Lint` returns the error directly; cobra RunE propagates to fang. `TestLintCmd_RunEForwardsArgs` exercises the path.

## Verification Gate Results

| Check | Result |
|-------|--------|
| `go test ./internal/wrap/... ./cmd/... -count=1 -run Lint` | PASS (9 tests across 2 packages) |
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `bash scripts/check-v1-leaks.sh ./cmd` | OK |
| `bash scripts/check-v1-leaks.sh ./internal/wrap` | OK |
| `./bin/spin lint --help` | renders fang-styled help with install hint in Long |
| `./bin/spin lint version` (golangci-lint installed) | prints `golangci-lint has version v1.64.8` |

## Issues Encountered

- **Pre-existing test brittleness (not caused by this plan):** `internal/wrap/fmt_test.go:TestFmt_GofumptMissing_NoStrict` fails in environments where `gofumpt` is on the original PATH (this env has `gofumpt` installed globally at `/home/samouly/go/bin/gofumpt`). The test does not strip `gofumpt` from the inherited PATH before prepending its shim dir, so the gofumpt-missing branch is never reached. Unrelated to the lint wrapper and out of scope; documented here for the next executor who runs the full wrap test suite.

## Next Phase Readiness

- HLTH-01 now has two paths to the lint capability: direct (`spin lint`) and aggregated via `spin doctor --deep` (Plan 04-01).
- Other Phase 4 plans (`spin update`, dogfood CI) are unblocked — `spin lint` does not depend on or affect them.
- The `golangciLintInstallHint` const in `internal/wrap/lint.go` is duplicated as a string literal in `internal/doctor/checks.go`'s `DeepLintCheck`. Future work could extract this to a shared package if a third caller appears, but v1 keeps them parallel (avoids the cross-package import the plan calls out).

---

*Phase: 04-post-scaffold-health-verification-dogfooding*
*Completed: 2026-06-05*
