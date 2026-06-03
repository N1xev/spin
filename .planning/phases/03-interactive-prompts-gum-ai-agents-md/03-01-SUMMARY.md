---
phase: 03
plan: 01
title: TUI guard + non-interactive flag + prompt package skeleton
subsystem: scaffold
tags: [tty, ci, no-interactive, prompt, errors, exit-130]
completed: 2026-06-03
duration: ~15m
tasks: 4 commits
files_created:
  - internal/prompt/prompt.go
  - internal/prompt/prompt_test.go
  - internal/prompt/detect.go
  - internal/prompt/detect_test.go
files_modified:
  - cmd/new.go
  - internal/scaffold/project.go
  - internal/scaffold/resolve.go
  - internal/scaffold/resolve_test.go
  - main.go
  - go.mod
  - go.sum
requirements:
  - INT-02
  - INT-03
key_findings:
  - "pflag v1.0.6 does not support multi-char Flag.Aliases (only single-letter Shorthand). Worked around by registering --no-interactive, --yes, and --batch as three separate bool flags and OR-ing the alias values into p.NoInteractive in ResolveFlags."
  - "errors.As needs a non-nil pointer to the error type's pointer (*prompt.Canceled, not prompt.Canceled). A *prompt.Canceled value pointer would not satisfy the type constraint — the error type IS the pointer, so the second arg to errors.As must be a pointer to the pointer."
  - "isatty.IsTerminal takes a uintptr fd, not *os.File. Must call os.Stdin.Fd() to extract the descriptor before passing."
  - "go mod tidy silently removes go-isatty from go.mod when no source file imports it. The dependency only becomes a direct require after detect.go imports it in Task 2."
  - "Plan 01 ships a stub detect.go (IsInteractive returns false) so the package compiles before Task 2. The Task 2 commit replaces the stub body with the real three-layer guard."
one_line_summary: "TTY/CI guard chokepoint, --no-interactive/--yes/--batch aliases, and the prompt package skeleton with Canceled→exit-130 wiring"
---

# Phase 3 Plan 1: TUI guard + non-interactive flag + prompt package skeleton

This plan establishes the single chokepoint through which every prompt UI
call must pass in Phase 3. Per the locked UI-SPEC contract, the plan ships
the contract surface (ShouldPrompt, Fill, Canceled) but no actual prompt
UI — Plans 02 and 03 wire huh v2 and gum respectively.

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-03T15:00:00Z
- **Completed:** 2026-06-03T15:15:00Z
- **Tasks:** 4
- **Files modified:** 7
- **Files created:** 4
- **Commits:** 4

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add prompt package skeleton with Canceled error + Fill stub | f86aeda | go.mod, go.sum, internal/prompt/{prompt,prompt_test,detect}.go |
| 2 | Implement three-layer TTY + CI guard in IsInteractive | 6a41b1d | go.mod, internal/prompt/{detect,detect_test}.go |
| 3 | Wire --no-interactive/--yes/--batch; relax Args; add prompt.Fill chokepoint | 595c3ff | cmd/new.go, internal/scaffold/{project,resolve,resolve_test}.go |
| 4 | Map *prompt.Canceled to exit code 130 in main.go | 8601b8d | main.go |

## Accomplishments

- **internal/prompt** package skeleton: `Canceled` typed error with
  `Reason` field, `Is(target)` matching `ErrCanceled`, `Fill(p)` as a
  documented no-op, `ShouldPrompt()` delegating to `IsInteractive()`.
- **Three-layer prompt guard** in `IsInteractive`: `SPIN_NO_INTERACTIVE`
  env var wins → `isatty.IsTerminal(os.Stdin.Fd())` must be true → none
  of `$CI`/`$GITHUB_ACTIONS`/`$GITLAB_CI`/`$BUILDKITE`/`$CIRCLECI` may
  be set. All three layers must pass to enable prompting.
- **CLI surface**: `--no-interactive`, `--yes`, `--batch` (three
  spellings, one field). `cobra.ExactArgs(1)` relaxed to
  `cobra.MaximumNArgs(1)` so `spin new` (no args, TTY) can prompt for
  the name. `prompt.Fill(p)` called between `ResolveFlags` and `Validate`,
  gated on `!p.NoInteractive`.
- **Exit-code mapping**: `*prompt.Canceled` matched via `errors.As` in
  `main.go` → exit 130. Other errors → exit 1. fang handles styled
  error output as before.
- **Dependency**: `github.com/mattn/go-isatty v0.0.22` added to
  `go.mod` (direct require; was previously a planned-but-uninstalled
  transitive dep).

## Files Created/Modified

### Created
- `internal/prompt/prompt.go` — Canceled error, ErrCanceled, Fill, ShouldPrompt
- `internal/prompt/prompt_test.go` — TestFillNoop, TestCanceledErrorIs
- `internal/prompt/detect.go` — IsInteractive + ciEnv (replaced stub in Task 2)
- `internal/prompt/detect_test.go` — TestIsInteractive_TTYCheck, _EnvOverride, _CIEnv (5 sub-cases), _AllLayersOff

### Modified
- `cmd/new.go` — relaxed Args, added 3 bool flags, wired prompt.Fill, imported internal/prompt
- `internal/scaffold/project.go` — added `NoInteractive bool` field
- `internal/scaffold/resolve.go` — wired no-interactive/yes/batch → p.NoInteractive
- `internal/scaffold/resolve_test.go` — registered 3 new flags in newResolveCmd; added TestResolveFlags_NoInteractiveAliases (4 sub-cases)
- `main.go` — 3-branch error handling; errors.As for *prompt.Canceled → exit 130
- `go.mod` / `go.sum` — go-isatty v0.0.22 (direct require); promoted pflag and golang.org/x/mod from indirect to direct (project source imports them)

## Decisions Made

1. **Three separate bool flags, not multi-char aliases** — pflag v1.0.6
   does not support `Flag.Aliases` (only single-letter `Shorthand`). The
   plan's instruction to use `pf.Lookup("no-interactive").Aliases =
   []string{"yes", "batch"}` would not compile. Implemented by
   registering all three as separate flags and OR-ing the alias values
   into `p.NoInteractive` in ResolveFlags. Documented in the cmd/new.go
   init block with a code comment.

2. **`*prompt.Canceled` matched via `errors.As` with `var canceled *prompt.Canceled`**
   — the error type IS the pointer (`*prompt.Canceled` implements
   `error` via the pointer receiver). `errors.As` needs a non-nil
   pointer to that pointer. A `var canceled prompt.Canceled` would
   not satisfy the type constraint.

3. **Stub `detect.go` in Task 1, full body in Task 2** — Task 1's
   `prompt.go` references `IsInteractive()` (defined in `detect.go`).
   Without a stub in Task 1, the package fails to compile before
   Task 2 lands. Stub is marked with a clear "Plan 02 Task 2 replaces
   this body" comment.

4. **p.NoInteractive checked in cmd/new.go, not in Fill** — per
   prompt.go's docstring (locked by Task 1's action): Fill itself
   consults only the three-layer guard (env/TTY/CI). The flag
   `--no-interactive` is the explicit user opt-out, which is read
   in runNew BEFORE calling Fill. Keeps the env/TTY/CI check
   independent of the cobra flag plumbing (RESEARCH §"Don't
   Hand-Roll" — single chokepoint).

## Deviations from Plan

### [Rule 1 - Plan-extension] pflag Flag.Aliases not available; implemented as three separate bool flags

**Found during:** Task 3 (cmd/new.go build).

**Issue:** Plan's Task 3 action specifies
`pf.Lookup("no-interactive").Aliases = []string{"yes", "batch"}`. The
`pflag.Flag` type at v1.0.6 has no `Aliases` field (only `Shorthand`,
which is a single letter). The plan's API call does not compile.

**Fix:** Registered `--no-interactive`, `--yes`, and `--batch` as three
separate `pf.Bool` flags. In ResolveFlags, the canonical
`--no-interactive` value is read into `p.NoInteractive`; the alias
values are OR'd into the same field. The three CLI spellings all
disable the prompt layer identically. Documented with a code comment
explaining the pflag v1.0.6 limitation.

**Files modified:** `cmd/new.go`, `internal/scaffold/resolve.go`

**Commit:** 595c3ff (Task 3)

### [Rule 1 - Bug] Plan's `gofumpt` verification step is incomplete

**Found during:** Final verification (after Task 4 commit).

**Issue:** The plan's verification block requires
`gofumpt -l ./internal/prompt ./cmd ./internal/scaffold ./main.go returns
no output`. `gofumpt` is not installed by default in the executor's
environment.

**Fix:** Installed `gofumpt` via `go install mvdan.cc/gofumpt@latest`,
then ran it on all files I modified. The output is clean for all four
files I touched in Plan 01 (`internal/prompt/{prompt,prompt_test,detect,detect_test}.go`,
`cmd/new.go`, `main.go`, `internal/scaffold/{project,resolve,resolve_test}.go`).
Pre-existing scaffold files (`repo.go`, `scaffold_test.go`, `template.go`,
etc.) have gofumpt issues that are out of scope (not introduced by my
changes — they predate Plan 01 in Phase 2 work).

**Files modified:** none (read-only check)

**Note:** Installing `gofumpt` to `$HOME/go/bin/gofumpt` adds it to PATH,
which has the side effect of breaking a pre-existing test
(`TestFmt_GofumptMissing_NoStrict` in `internal/wrap/fmt_test.go`) that
expects `gofumpt` to be missing. The test fails at the base commit
(`8c82071`) too when gofumpt is installed — this is a pre-existing
environment-dependent test, not a regression from Plan 01.

**Commits:** none (read-only check)

## Test count delta

- Before: **80** tests (approximate; based on full `go test ./...` output)
- After: **88** tests (+8)
  - 2 new top-level in `internal/prompt`:
    - `TestFillNoop`
    - `TestCanceledErrorIs`
  - 4 new top-level in `internal/prompt` (Task 2):
    - `TestIsInteractive_TTYCheck`
    - `TestIsInteractive_EnvOverride`
    - `TestIsInteractive_CIEnv` (5 sub-cases for the 5 env vars)
    - `TestIsInteractive_AllLayersOff`
  - 1 new top-level in `internal/scaffold` (Task 3):
    - `TestResolveFlags_NoInteractiveAliases` (4 sub-cases: default,
      --no-interactive, --yes, --batch)

## Verification

- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/prompt/...` all pass (6 tests)
- `go test ./internal/scaffold/... -run TestResolveFlags` all pass (13+ tests, including new NoInteractiveAliases)
- `go test ./cmd/...` all pass (TestNew, TestFang*, TestUnknown, TestVersion, TestRootCmd)
- `gofumpt -l` clean on all files modified in this plan
- `spin new --help` shows `--no-interactive`, `--yes`, `--batch` as documented flags
- `go test ./... -count=1` — all packages pass except the pre-existing
  `TestFmt_GofumptMissing_NoStrict` (environment-dependent: this test
  fails whenever `gofumpt` is installed in `$PATH`. The test was
  failing at the base commit `8c82071` too — not a regression from
  Plan 01.)

## Known Stubs

The plan ships 1 documented stub:

- **`internal/prompt/detect.go` is replaced wholesale by Task 2** — the
  Plan 01 commit ships a stub `IsInteractive() bool { return false }`
  body so the package compiles before Task 2 lands. Task 2's commit
  replaces this body with the full three-layer guard. The stub is
  marked with a "Plan 02 Task 2 replaces this body" comment.

- **`prompt.Fill` is a documented no-op** — Plan 01's body returns
  `nil` when prompting is gated off. Plans 02 and 03 wire the huh
  v2 and gum backends respectively. The chokepoint is established
  so the rest of the system can wire against it without churning.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: tty-guard-implementation | internal/prompt/detect.go | Three-layer guard enforces the chokepoint: env var (`SPIN_NO_INTERACTIVE`) → TTY (`isatty.IsTerminal(os.Stdin.Fd())`) → CI env vars (5 names). Any false layer disables prompting. UI-SPEC §"TTY guard" contract. |
| threat_flag: cancellation-exit-code | main.go | `errors.As(err, &*prompt.Canceled)` matches the typed error and exits 130 (standard for Ctrl-C). Reason field preserved on the struct for future logging per T-03.01-R. |

## Issues Encountered

- **Write tool wrote to the main repo, not the worktree** (#3099).
  First attempt at creating `internal/prompt/prompt.go` and
  `prompt_test.go` silently landed in
  `/home/samouly/Projects/Golang/loom/internal/prompt/` (the main
  repo) instead of the worktree. Detected via `ls` showing the files
  were missing from the worktree's `internal/` directory. Cleaned up
  with `rm -rf` on the main-repo directory and re-wrote the files
  using the explicit worktree path. Subsequent edits used the same
  explicit worktree path; no further drift.

- **Pre-existing test environment dependency** (not a regression).
  `TestFmt_GofumptMissing_NoStrict` in `internal/wrap/fmt_test.go`
  fails when `gofumpt` is installed in `$PATH`. Confirmed by
  checking out the base commit (`8c82071`) and running the same
  test — it fails identically. The test was passing in the
  Phase 2 closeout state because `gofumpt` was not installed in
  the executor's environment. My `go install mvdan.cc/gofumpt@latest`
  (to enable the plan's `gofumpt -l` verification step) put
  `gofumpt` on `$PATH` for the first time in this session.

## Next Phase Readiness

- **Plan 02 (huh v2 backend)** can wire `fillWithHuh` against the
  established `Fill` chokepoint. The `Canceled` error type and
  `errors.As` exit-code mapping are ready to receive
  `huh.ErrUserAborted` translations.
- **Plan 03 (gum backend)** can wire `fillWithGum` similarly.
- **Plan 04 (AGENTS.md template)** is independent of this plan's
  chokepoint — it adds a `lib/ai/AGENTS.md.tmpl` overlay gated on
  `p.AI` (already on the struct from Phase 2) and uses the existing
  template engine.
- The `--no-interactive` / `--yes` / `--batch` flags are wired and
  tested; Plans 02/03 will not need to change the flag plumbing.

## Commits (chronological)

```
8601b8d feat(03-01): map *prompt.Canceled to exit code 130 in main.go
595c3ff feat(03-01): wire --no-interactive/--yes/--batch; relax Args; add prompt.Fill chokepoint
6a41b1d feat(03-01): implement three-layer TTY + CI guard in IsInteractive
f86aeda feat(03-01): add internal/prompt skeleton with Canceled error + Fill stub
```

---

*Phase: 03-interactive-prompts-gum-ai-agents-md*
*Plan: 01*
*Completed: 2026-06-03*
