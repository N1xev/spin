---
phase: 04-post-scaffold-health-verification-dogfooding
plan: 04
status: complete
---

# Plan 04-04: `spin update` huh v2 form + cobra subcommand

## What was done

Executor hit a transient socket error mid-execution; the two feature commits
were already on the worktree branch and merged. SUMMARY + docs commit
written manually here.

## Commits

| Hash | Subject |
|------|---------|
| `bc5f04e` | feat(04-04): huh v2 form for `spin update` with non-TTY table fallback |
| `397259a` | feat(04-04): wire `spin update` cobra subcommand with --all flag |
| `caf0e2e` | merge(04-04): spin update form + cobra subcommand |

## Files added (4)

- `internal/update/form.go` -- `PromptForUpdate`, `PromptOptions`,
  `UpdateChoice`, `printNonTTYTable`. Layering: `update` does NOT import
  `internal/prompt` (one-way: prompt → scaffold); local `ErrCanceled`
  defined here.
- `internal/update/form_test.go` -- 5+ tests covering non-TTY table, empty-deps
  no-op, form-construction, "(current)" annotation, long-module truncation.
- `cmd/update.go` -- `updateCmd` (cobra), `--all` bool flag, `runUpdate`
  calls `update.FindGoMod` + `update.PromptForUpdate`.
- `cmd/update_test.go` -- 4 tests: command registered, `--all` flag present,
  `Long` mentions `[Skip, newStable, newLatest]` and the no-`go test` policy,
  no-go.mod returns error.

## D-contract coverage

- **D-07** (`--all` includes indirect): flag registered on `updateCmd.Flags()`,
  threaded into `ListDeps(includeIndirect)`.
- **D-09** (one `huh.NewSelect` per dep, options `[Skip, newStable, newLatest]`,
  default `newStable`): form-builder iterates `deps` and allocates
  `var choice string` per dep, pre-set to `"stable"` so huh renders
  `newStable` pre-selected. One form, one submit, atomic apply.
- **D-10** (apply = `go get` + `tidy` + `CGO=0 go build ./...`, no test):
  `updateCmd.Long` documents the contract; tests in `update` package
  assert `Apply` never invokes `go test`.
- **INT-03 (non-TTY guard)**: `isatty.IsTerminal(os.Stdin)` check before
  building the form. Non-TTY path renders a 4-column `MODULE OLD STABLE LATEST`
  table and returns an error -- no hang.

## Verification

- `go test ./internal/update/... -count=1` -- PASS
- `go test ./cmd/... -count=1` -- PASS
- `go build ./...` -- clean
- `go vet ./...` -- clean

## Deviations from plan

- None of substance. The socket error killed the agent before it could write
  the SUMMARY; the code itself is on the branch and merged.

## Out-of-scope notes for the next phase

- `update.PromptForUpdate` returns the local `update.ErrCanceled` on user
  cancellation. cobra + fang render this as a styled "canceled" message.
- The non-TTY table is `MODULE OLD STABLE LATEST`. The "(current)" annotation
  appears when `newStable == old` (same for latest).
