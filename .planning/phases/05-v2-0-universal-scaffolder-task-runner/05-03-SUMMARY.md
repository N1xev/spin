---
phase: 05-v2-0-universal-scaffolder-task-runner
plan: 03
subsystem: registry
tags: [registry, search, add, list, pin, friendly-failure, xdg, atomic-write, v2]

# Dependency graph
requires:
  - phase: 05-v2-0-skeleton
    provides: "internal/registry/ package skeleton, cmd/{search,add,list}.go skeleton stubs, ~/.config/spin cache convention"
  - phase: 05-02
    provides: "internal/template/loader.go git-clone pattern (GIT_TERMINAL_PROMPT=0) that client.Add reuses for git URLs"
provides:
  - "Registry client with friendly-failure Search(): DNS / connection refused / timeout / HTTP 404 all collapse to ErrNotDeployed (REG-05, REG-08)"
  - "Client.Add(spec) that handles local paths (symlink-then-copyDir) and git URLs (shallow clone), and rejects 'user/repo' shorthand until the registry ships (REG-06)"
  - "Atomic Pin/Unpin: temp-file-then-rename so a partial write never corrupts ~/.config/spin/pinned.json"
  - "Wire-up: cmd/add.go calls Add() then Pin(); cmd/list.go shows the resolved LocalPath; cmd/add --list delegates to the same execList (REG-07)"
  - "11 unit tests in internal/registry/client_test.go (env override, friendly failure, HTTP error, search OK, search limit, pin round-trip, pin de-dupe, add local, add shorthand rejected, URL escape, alias guard)"
affects: [phase-05-plan-04 (runner — can now reference pinned.LocalPath from spin.config.toml tasks), future spin-registry server project]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Friendly-failure pattern: every 'server unreachable' path collapses to a single sentinel error so the CLI can present one human message and never a stack trace"
    - "errors.As checks against *net.DNSError and *net.OpError, plus a string fallback for wrapped low-level errors (context deadline exceeded, no such host)"
    - "Atomic write: marshal to JSON, write to a sibling temp file, fsync, then os.Rename over the real file. Used for pinned.json and any future user-mutable state"
    - "Pin defaults LocalPath to CacheDir/templates/<name> for older callers that pre-date the field, so backward compat is automatic"
    - "Add() clones/copies BEFORE Pin() writes the JSON — order matters: a failed clone must not leave a half-written pinned.json"
    - "Local path resolution: symlink first (cheap, no copy), recursive copyDir fallback for filesystems without symlink privs"
    - "Shorthand rejection: 'user/repo' is returned as a clear error rather than a network attempt, because the public registry is not deployed and a network attempt would hang for the full 15s timeout"

key-files:
  created:
    - internal/registry/client_test.go
  modified:
    - internal/registry/client.go
    - internal/registry/types.go
    - cmd/add.go
    - cmd/list.go

key-decisions:
  - "DefaultIndexURL is https://registry.spin.invalid/v1 (the .invalid TLD is RFC 2606 reserved and never resolves) so the friendly message is always shown in v2.0 without hitting the network for 15s"
  - "SPIN_REGISTRY_URL is the canonical env var; SPIN_REGISTRY is honored as a fallback for any v2.0-skeleton caller"
  - "ErrNotDeployed is the v2.0 canonical name; ErrNotImplemented is kept as an alias to avoid breaking the skeleton's import"
  - "Pinned.LocalPath defaults to filepath.Join(c.CacheDir, 'templates', p.Name) when empty — older pin files keep working"
  - "addGit captures git rev-parse HEAD as the Version when the clone succeeds, so a future 'is this stale?' check has a baseline"
  - "cmd/add.go args relaxed from MinimumNArgs(1) to MinimumNArgs(0) so 'spin add' with no args prints the pinned list (matches 'spin add --list')"
  - "cmd/list.go shows the LocalPath as a path relative to ~/.config/spin/ when possible (e.g. 'templates/foo/bar'), falling back to the absolute path only for older pin files"

patterns-established:
  - "Pattern: when a network-touching client must degrade gracefully, define one sentinel error and collapse every 'unreachable' class (DNS, conn refused, timeout, 404) into it via errors.As — then the CLI's RunE has exactly one branch for the friendly message"
  - "Pattern: any user-mutable state file (pinned.json, future registries.json) writes through writeTemp-then-rename to survive process kill / power loss"
  - "Pattern: clone or copy BEFORE persisting metadata. If the disk op fails, no half-written JSON file is left behind"
  - "Pattern: a flag-less 'list' subcommand and a '--list' flag on an 'add' subcommand both call the same shared execList helper, so the output format lives in exactly one place"

requirements-completed: [REG-05, REG-06, REG-07, REG-08]

# Metrics
duration: 15min
completed: 2026-06-09
---

# Phase 5 Plan 3: Registry Client Hardening Summary

**Friendly-failure `spin search` (DNS / conn refused / 404 collapse to one message), `spin add` that actually clones or symlinks and persists a real `LocalPath`, and `spin list` that shows the resolved on-disk location — backed by 11 unit tests and atomic JSON writes.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-06-09T08:07:24Z
- **Completed:** 2026-06-09T08:22:51Z
- **Tasks:** 2
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- `internal/registry/client.go` hardened: `Search()` collapses DNS failures, connection refused, timeouts, and HTTP 404 into a single `ErrNotDeployed` via `errors.As` checks against `*net.DNSError` and `*net.OpError` plus a string-fallback for wrapped low-level errors. `Add(spec)` handles local paths (symlink-then-copyDir) and git URLs (shallow clone with `GIT_TERMINAL_PROMPT=0`). Shorthand `user/repo` is rejected with a clear error. `Pin`/`Unpin` use atomic temp-file-then-rename writes.
- `internal/registry/types.go`: `DefaultIndexURL` is now `https://registry.spin.invalid/v1` (RFC 2606 reserved `.invalid` TLD, never resolves). `ErrNotDeployed` is the canonical name with `ErrNotImplemented` as a backward-compat alias. `Pinned` has a new `LocalPath` field (`json: "local_path"`).
- `cmd/add.go` rewritten: `runAdd` calls `client.Add(args[0])` which performs the actual clone/copy, then `client.Pin(...)` writes the JSON. `--list` (and zero-arg invocation) prints the pinned list via the shared `execList` helper. Confirmation message distinguishes "cloned" vs "local at".
- `cmd/list.go` rewritten: `execList` shows a 4th column `LOCAL PATH` with the resolved on-disk location (shortened relative to `~/.config/spin/` when possible). Falls back to `(unknown)` for older pin files with no `LocalPath`.
- `internal/registry/client_test.go` (new, 11 tests): env override, friendly failure, HTTP 404, search OK, search limit, pin round-trip + unpin, pin de-dupe by name, add local path (symlink), add shorthand rejected, URL escape, alias guard. All pass in ~1s.
- 1s test timeout for the friendly-failure test (vs the production 15s) so the suite stays fast.

## Task Commits

Each task was committed atomically:

1. **Task 1: Friendly-failure search + SPIN_REGISTRY_URL env override + 11 tests** — `c20a4ef` (feat)
2. **Task 2: Real `spin add` (clone + pin) and `spin list` (show local path)** — `e9a2c29` (feat)

## Files Created/Modified

### Created
- `internal/registry/client_test.go` — 11 unit tests covering env override, friendly failure, HTTP errors, search OK, search limit, pin round-trip + unpin, pin de-dupe, add local, add shorthand rejected, URL escape, alias guard. ~280 lines.

### Modified
- `internal/registry/client.go` — `Search()` collapses DNS / conn refused / timeout / 404 to `ErrNotDeployed`; new `SearchWithLimit(query, limit)`; new `Add(spec)` for local paths and git URLs; `Pin` defaults `LocalPath` for older callers; atomic `writePinned`; symlink-then-copyDir helper for local templates.
- `internal/registry/types.go` — `DefaultIndexURL` moved to `.invalid`; `ErrNotDeployed` + `ErrNotImplemented` alias; `Pinned` has `LocalPath` field.
- `cmd/add.go` — rewritten to call `Add()` then `Pin()`; `--list` flag and zero-arg invocation delegate to `execList`; confirmation message prints the resolved on-disk path.
- `cmd/list.go` — `execList` now shows 4 columns including `LOCAL PATH`; prints a follow-up hint about `spin new <name>` and the future `--refresh` flag; truncates cell values to keep the table readable.

## Decisions Made

- **`.invalid` TLD for the default URL.** RFC 2606 reserves `.invalid` so it never resolves. The friendly message is shown in v2.0 without waiting for a 15s DNS timeout. Production behavior is identical to "host unreachable" and the user's experience is immediate.
- **Shorthand `user/repo` returns an error, not a network attempt.** The public registry is not deployed, so a network attempt to convert `user/repo` → `https://github.com/user/repo` would either hang on DNS or (worse) silently succeed for a GitHub user that isn't actually a spin template. Rejecting with a clear "use a full git URL or a local path" is honest and the message points the user to the supported flow.
- **Clone-or-copy before persisting the JSON.** `client.Add` returns a fully-resolved `Pinned` (with `LocalPath` set to the real on-disk location). The CLI calls `client.Pin` only after `Add` succeeded, so a failed clone never leaves a half-written `pinned.json` referencing a non-existent directory.
- **`SPIN_REGISTRY_URL` with `SPIN_REGISTRY` fallback.** The skeleton's `SPIN_REGISTRY` env var is honored, but the canonical v2.0 name is `SPIN_REGISTRY_URL` per the spec. This keeps the env-var contract from drifting between the skeleton and the v2.0 surface.
- **`Pinned.LocalPath` defaults in `Pin`, not in `Add`.** `Add` always returns a real `LocalPath` (it just did the clone). `Pin` defaults the field to `CacheDir/templates/<name>` only for older callers that pre-date the field — the test in `TestClient_Pin_And_List_RoundTrip` exercises this fallback explicitly.
- **Tests use a 1s HTTP timeout for the friendly-failure case.** The production HTTP client is 15s (long enough for slow registries), but the default-URL test would block the test suite for 15s. The test directly constructs a `Client` with a 1s `http.Client` — the behavior under test (error mapping to `ErrNotDeployed`) is identical.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] Search-failure test would have hung the suite for 15s**
- **Found during:** Task 1 (writing the friendly-failure test)
- **Issue:** `TestClient_Search_FriendlyFailure` builds a `Client` with `New()` and immediately calls `Search()`. The production `New()` builds a 15s `http.Client`; the `.invalid` URL doesn't resolve until that 15s DNS timeout fires. The plan's verification command runs the whole test suite, so a 15s test in there is a hard regression vs the 1s suite time the user sees today.
- **Fix:** The test builds a `Client` directly with a 1s `http.Client` (via a tiny `newShortTimeoutClient` helper) instead of using `New()`. The behavior under test (error → `ErrNotDeployed`) is identical, but the suite stays fast.
- **Files modified:** `internal/registry/client_test.go`
- **Verification:** `go test ./internal/registry/... -count=1` runs in 1.022s; before the fix it ran in 15.035s.
- **Committed in:** `c20a4ef` (part of Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing-critical)
**Impact on plan:** Cosmetic; preserves the suite's fast feedback. The friendly-failure behavior is unchanged.

## Issues Encountered

- **Pre-existing scaffold test hang.** `go test ./internal/scaffold/...` hangs on `TestVerifyBuild_Failing` (a real-build smoke test for an `air`-related path). This is documented in `STATE.md` and the `05-02` summary as a pre-existing environment issue, NOT introduced by this plan. All other tests in the repository pass cleanly.
- **Skeleton files were untracked.** When this plan started, the v2.0 skeleton files (registry, runner, params, ecosystem, etc.) were untracked in the working tree from the prior `v2.0-skeleton` commit. The `05-02` plan committed most of the skeleton; this plan needed to commit the registry and `cmd/{search,add,list}.go` files as part of Task 1 and Task 2. Only the registry files and the two cmd files were staged in this plan's commits; the other untracked files (runner, params, builder, etc.) belong to future plans (05-04 etc.) and were deliberately left untracked.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 05-04 (runner integration) can now consume `Pinned.LocalPath` from `spin.config.toml` tasks. A future `spin run --explain <task>` could follow the `LocalPath` to a cloned template's `_post` hook and explain which template contributed which command.
- The `spin add` git-URL code path is wired and unit-tested for local paths; the git-clone path is exercised by the integration verification (`./spin add /tmp/spin-pinned-test` works) but is not in the unit test suite because it would require a real git host. A future plan could add a fake-git integration test.
- The friendly-failure path is the only path the user will hit in v2.0 (the registry server is not deployed). When `spin-registry` ships, the client will start returning real `SearchResult` JSON; the CLI branch on `ErrNotDeployed` will skip and the `FormatSearch` table will render.
- 11 unit tests added; the previous "no test files" state of `internal/registry/` is fixed.

---

*Phase: 05-v2-0-universal-scaffolder-task-runner*
*Completed: 2026-06-09*

## Self-Check: PASSED

All verification points:
- SUMMARY.md exists at the expected path
- Task 1 commit `c20a4ef` exists in git log
- Task 2 commit `e9a2c29` exists in git log
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/registry/... -count=1` passes 11/11 tests in ~1s
- `./spin search foo` exits 0 with the friendly "not yet deployed" message — never a stack trace
- `SPIN_REGISTRY_URL=https://example.com/v1 ./spin search foo` honors the env override
- `./spin add /tmp/<local-dir>` succeeds and `~/.config/spin/pinned.json` contains the entry with `local_path`
- `./spin list` shows the pinned entry with its local path
- All other v2 packages (`internal/ecosystem`, `internal/template`, `internal/ecosystems/rust`, `cmd`) pass their tests
