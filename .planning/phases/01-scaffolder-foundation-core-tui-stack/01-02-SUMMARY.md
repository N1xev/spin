---
phase: 01-scaffolder-foundation-core-tui-stack
plan: 02
subsystem: scaffolder
tags: [go, cobra, fang, validation, levenshtein, post-scaffold, git-init, tui, smoke-test]

# Dependency graph
requires:
  - phase: 01-scaffolder-foundation-core-tui-stack
    plan: 01
    provides: "Walking Skeleton: spin binary, Project struct, scaffold engine, embedded templates, go build smoke test"
provides:
  - "Full Project struct (TMPL-07 + 13 forward-compat bools) covering Phase 1/2/3/4 flags"
  - "ResolveFlags() binding every registered cobra flag to *Project with cross-field invariants (--bubbles implies --bubbletea)"
  - "Project.Validate() rejecting invalid names (Go module path segment regex) and existing directories without --force"
  - "Project.VerifyBuild() post-scaffold smoke test (go mod tidy + go build ./... with CGO_ENABLED=0 + go test ./...) with stderr surfacing"
  - "Project.GitInit() env-guarded git init -b main + add + commit producing 1 initial commit"
  - "Custom FlagErrorFunc adding Levenshtein-based 'Did you mean --X?' suggestions for unknown flags (FLAG-18)"
  - "fang.WithVersion wiring so `spin --version` outputs the real version instead of 'unknown (built from source)'"
affects:
  - "phase-01-plan-03 (template engine expansion, license templates) - uses the new Project fields"
  - "phase-02 (CLI variant + extended libs) - ResolveFlags already binds --cli/--cobra/--fang/etc; only template content remains"

# Tech tracking
tech-stack:
  added:
    - "internal/version package (var Version = '0.1.0', ldflags-overridable)"
  patterns:
    - "ResolveFlags + Validate + New pipeline: cmd/new.go now chains flag binding -> validation -> scaffold"
    - "Method-based hooks (p.VerifyBuild, p.GitInit) instead of free functions for symmetry with future plan tasks"
    - "Shared runCmd helper for os/exec: sets Dir, appends env to os.Environ(), returns combined output"
    - "go mod tidy retained in VerifyBuild (Walking Skeleton deviation #4) - populates go.sum before build for fresh scaffolds"
    - "Env-guard for git: GIT_TERMINAL_PROMPT=0 + GIT_AUTHOR_*/GIT_COMMITTER_* spin@localhost"
    - "Custom FlagErrorFunc for flag suggestions (cobra only suggests commands, not flags)"
    - "internal/version.Version is the ldflags target (-X github.com/example/spin/internal/version.Version=X.Y.Z)"

key-files:
  created:
    - "internal/scaffold/resolve.go - ResolveFlags (224 LOC): mustString/mustBool helpers, containsString/dedupStrings, ArgError/FlagError types"
    - "internal/scaffold/resolve_test.go - 8 TestResolveFlags subtests covering default, --bubbles-implies, sort/dedup, --module override, --cli variant, --license override, all-bools-bind, --template"
    - "internal/scaffold/validate.go - Project.Validate (99 LOC), ModuleSegmentRegex exported, IsValidGoModuleSegment, reservedGoWords map"
    - "internal/scaffold/validate_test.go - 19 TestIsValidGoModuleSegment cases + TestProjectValidate_DirConflict + TestProjectValidate_NameRegex + TestProjectValidate_ErrorFormat"
    - "internal/scaffold/hooks.go - Project.VerifyBuild (85 LOC), runCmd shared helper, charm/log v2 structured logging"
    - "internal/scaffold/hooks_test.go - TestVerifyBuild_{Passing,Failing,Skipped} + TestGitInit_{Success,NoGit}"
    - "internal/scaffold/git.go - Project.GitInit (101 LOC), gitEnv env-guard var, isUnknownFlagErr/isNotFoundErr helpers, -b main with git < 2.28 fallback"
    - "internal/version/version.go - var Version = '0.1.0' with doc comment explaining ldflags override"
    - "cmd/help_test.go - 6 fang/version/suggestion tests: TestFangStyledHelp, TestFangTTYEmitsANSI, TestUnknownFlagSuggestion, TestVersionFlag, TestRootCmdVersionWiring, TestFangExecuteNoPanic"
  modified:
    - "internal/scaffold/project.go - expanded from 6 to 23 fields: added License, Template, Force, NoGit, NoVerify, Quiet, plus 13 forward-compat bools (Cobra, Fang, Viper, Huh, Glamour, Glow, Wish, Log, Harmonica, Modifiers, Ansi, Runewidth, AI)"
    - "cmd/new.go - replaced Walking Skeleton flag list (--tui/--bubbletea) with full Phase 1 + forward-compat set (16 Phase 1 active + 13 forward-compat = 29 flags total); runNew chains ResolveFlags -> Validate -> scaffold.New"
    - "cmd/root.go - Version now sources from internal/version.Version, Long expanded with example invocation, custom FlagErrorFunc with Levenshtein suggester added via init()"
    - "main.go - fang.Execute now passes fang.WithVersion(version.Version)"
    - "internal/scaffold/scaffold.go - removed inline --force-blind dir-exists check (canonical now in Validate); calls p.VerifyBuild() then p.GitInit() in that order so a failing build never gets committed"

key-decisions:
  - "internal/version ships only the var form (var Version = '0.1.0'), not a function. cobra/fang read the var directly; the plan's func Version() would name-collide with the var and not compile."
  - "Project.Validate() removed the Walking Skeleton's inline --force-blind check from scaffold.New. The check now lives in exactly one place; cmd/new.go's runNew -> ResolveFlags -> Validate -> New is the canonical order."
  - "go mod tidy retained in VerifyBuild (3 steps: tidy, build, test) despite the plan specifying 2 steps (build, test). Without tidy, fresh scaffolds fail with 'missing go.sum entry' before any real check can fire - this is the same deviation the Walking Skeleton landed in Plan 01-01."
  - "Validate stub committed in Task 1 (returns nil) to keep cmd/new.go's runNew body compileable across task boundaries. Task 2 replaced it with the real implementation. Trade-off: a 'transient' file exists in the Task 1 commit that doesn't match the plan's task partitioning."
  - "internal/version/version.go committed in Task 1 (not Task 4 as the plan listed) because resolve.go's p.SpinVer = version.Version needs the package to exist for the build to be green at every commit boundary. Task 4 expanded cmd/root.go to use it."
  - "Custom FlagErrorFunc implements Levenshtein-based flag suggestions (cobra's built-in SuggestionsMinimumDistance only handles command suggestions, not flags - verified in cobra/command.go findSuggestions). Max edit distance = 2 (constant, not pulled from cmd.SuggestionsMinimumDistance because that field is 0 for subcommands)."

patterns-established:
  - "Validate/VerifyBuild/GitInit as Project methods (not free functions) - makes the scaffolder API discoverable via `p.Method()` and testable in isolation"
  - "runCmd is the single os/exec helper used by both VerifyBuild and GitInit; new hooks should use it too"
  - "ResolveFlags + Validate + New is the canonical pipeline; cmd/new.go is the only caller of ResolveFlags (tests construct a fresh cobra command and call ResolveFlags directly)"
  - "Cobra flag errors get a 'Did you mean' suggestion via FlagSuggestionError wrapper that implements Unwrap()"
  - "internal/version.Version is the single source of truth for the version string across cobra, fang, and any future commit message"

requirements-completed: [SCAF-02, SCAF-04, SCAF-08, FLAG-02, FLAG-03, FLAG-13, FLAG-14, FLAG-15, FLAG-16, FLAG-17, FLAG-18, TMPL-07]

# Metrics
duration: 26min
completed: 2026-06-02
---

# Phase 1 Plan 2: Flag Wiring + Validation + Post-Scaffold Hooks

**Expanded the Walking Skeleton's `Project` struct into the full TMPL-07 contract, wired all Phase 1 + forward-compat cobra flags via `ResolveFlags`, added name-regex + dir-conflict validation, post-scaffold `go build` + `git init` hooks, and fang version + unknown-flag suggestions**

## Performance

- **Duration:** 26 min
- **Started:** 2026-06-02T19:39:22Z
- **Completed:** 2026-06-02T20:05:51Z
- **Tasks:** 4 (all auto, all TDD)
- **Files modified:** 14 (1600 insertions, 79 deletions)
- **Test runtime:** ~28s total (cmd 5.9s + scaffold 22.9s; scaffold dominated by TestVerifyBuild_Failing's 20s go mod tidy timeout for a deliberately broken module)

## Accomplishments

- **Full `Project` struct** with 23 fields: TMPL-07 (Name/Module/Type/Libs/License/Template) + scaffolding controls (Force/NoGit/NoVerify/Quiet/Year/SpinVer) + 13 forward-compat bools (Cobra/Fang/Viper/Huh/Glamour/Glow/Wish/Log/Harmonica/Modifiers/Ansi/Runewidth/AI) so later phases never need to touch the struct again
- **`ResolveFlags()`** with cross-field invariants: `--bubbles` implies `--bubbletea`, Libs sorted and deduped, Module defaults to Name, Year = current year, SpinVer sourced from `internal/version.Version`. 8 table-driven tests cover every documented behavior.
- **`Project.Validate()`** rejects invalid names via `IsValidGoModuleSegment` (14 regex cases + 8 reserved Go words + path-traversal guard) and existing directories without `--force`. Error message names the constraint and the example invocation.
- **`Project.VerifyBuild()`** runs `go mod tidy` -> `go build ./...` with `CGO_ENABLED=0` -> `go test ./...`, surfacing go command stderr verbatim on failure. `--no-verify` short-circuits before any exec. 3 tests cover passing/failing/skipped.
- **`Project.GitInit()`** runs `git init -b main` (with fallback for git < 2.28) + `git add .` + `git commit -m "scaffold <name> with spin <ver>"`, all env-guarded with `GIT_TERMINAL_PROMPT=0` + `GIT_AUTHOR_*`/`GIT_COMMITTER_*`=`spin@localhost`. Missing `git` on `$PATH` warns and skips. 2 tests cover success and `--no-git` skip.
- **Custom `FlagErrorFunc`** adds Levenshtein-based "Did you mean --X?" to unknown-flag errors. Verified `spin new myapp --bubbltea` -> `Did you mean --bubbletea?` in production binary.
- **`fang.WithVersion(version.Version)`** wires the version into fang; `spin --version` now outputs `spin version 0.1.0` instead of `unknown (built from source)`.

## Task Commits

1. **Task 1: Expand Project struct + ResolveFlags (flag binding)** - `b823e89` (feat)
2. **Task 2: Validate() (name regex + dir conflict + --force)** - `cad2eee` (feat)
3. **Task 3: VerifyBuild smoke test + GitInit (env-guarded)** - `86ccb74` (feat)
4. **Task 4: internal/version + wire fang version + SCAF-07/FLAG-18 polish** - `77cecc0` (feat)

## Files Created/Modified

- `internal/scaffold/project.go` - Expanded from 6 to 23 fields; doc comment tracks which fields are Phase 1 active vs forward-compat
- `internal/scaffold/resolve.go` - ResolveFlags + mustString/mustBool/containsString/dedupStrings helpers; ArgError/FlagError types for missing flag/arg
- `internal/scaffold/resolve_test.go` - 8 TestResolveFlags subtests; newResolveCmd helper mirrors cmd/new.go's flag list
- `internal/scaffold/validate.go` - ModuleSegmentRegex (exported) + IsValidGoModuleSegment + reservedGoWords map + Project.Validate with multi-line error referencing example invocation
- `internal/scaffold/validate_test.go` - 19 subtests covering all RESEARCH §6 cases + dir-conflict + name-regex + error format
- `internal/scaffold/hooks.go` - runCmd shared exec helper + Project.VerifyBuild with 3-step smoke test (tidy/build/test) and NoVerify short-circuit
- `internal/scaffold/hooks_test.go` - 5 tests using chdirTemp/minimalScaffold/brokenScaffold fixtures
- `internal/scaffold/git.go` - Project.GitInit with gitEnv env-guard; -b main with git < 2.28 fallback via symbolic-ref; isNotFoundErr distinguishes missing git from other exec failures
- `internal/scaffold/scaffold.go` - Removed inline --force-blind dir check; New() now calls p.VerifyBuild() -> p.GitInit() in that order
- `internal/version/version.go` - Single var Version = "0.1.0" with ldflags doc comment
- `cmd/new.go` - 29 flags registered (16 Phase 1 active + 13 forward-compat); runNew chains ResolveFlags -> Validate -> New
- `cmd/root.go` - Version from internal/version; expanded Long; FlagErrorFunc set via init() with Levenshtein suggester (closestFlag + levenshtein + min3 helpers)
- `main.go` - fang.Execute now passes fang.WithVersion(version.Version)
- `cmd/help_test.go` - 6 tests: fang styling structural markers, TTY-allocated ANSI codes, unknown flag suggestion, version flag, version wiring, no-panic regression

## Decisions Made

- **Var-only `internal/version.Version`.** Plan's spec said `var Version = "0.1.0"` AND `func Version() string` but those collide; kept the var since cobra/fang both accept the var directly. Documented in the package doc comment.
- **Plan-partitioning pragmatism.** Plan's `files:` lists `validate.go` under Task 2 and `version.go` under Task 4, but Task 1's runNew body calls `p.Validate()` and resolve.go reads `version.Version`. To keep every commit boundary green (every commit must produce a working build per the success criteria), the validate.go stub and the var-only version.go were committed with Task 1; Task 2 replaced validate.go with the real implementation, and Task 4 expanded cmd/root.go to use version.Version. Trade-off: a "stub" file appears in the Task 1 commit.
- **3-step VerifyBuild (tidy + build + test) over plan's 2-step (build + test).** Without `go mod tidy` first, fresh scaffolds have no go.sum and `go build` fails with "missing go.sum entry" before any real check fires. This was the Walking Skeleton's deviation #4 in Plan 01-01; carried forward to keep the "perfect first run" promise.
- **Method-based hooks (p.VerifyBuild, p.GitInit) over free functions.** Mirrors the Project struct as the single source of truth; tests can construct a minimal Project and call each method in isolation. Replaces the Walking Skeleton's unexported `verifyBuild` function.
- **Custom FlagErrorFunc with constant max-distance 2.** Cobra's `SuggestionsMinimumDistance` is auto-set to 2 only for command suggestions (verified in `cobra/command.go:findSuggestions`); for flag suggestions the field is 0 unless manually set on every subcommand. Hardcoding 2 in the function is simpler and more predictable.
- **Removed inline dir-exists check from `scaffold.New`.** The Walking Skeleton's check was --force-blind and used a stale "use --force in Plan 02" message. The check is now canonical in `Project.Validate`; direct callers of `scaffold.New` (tests) are expected to call Validate first.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed duplicate inline dir-exists check from scaffold.New**
- **Found during:** Task 2 (after Project.Validate was implemented and runNew wired it)
- **Issue:** `scaffold.New` had its own `os.Stat(target)` check that was --force-blind and used a stale message "(use --force in Plan 02)". Smoke test confirmed `spin new myapp --force` (when `./myapp` exists) failed with the inline check's error before Validate could even see the request.
- **Fix:** Removed lines 59-62 of scaffold.go; the check is now canonical in `Project.Validate`. `scaffold.New`'s comment documents the new contract.
- **Files modified:** internal/scaffold/scaffold.go
- **Verification:** `spin new myapp` (existing) -> exit 1 "already exists; pass --force"; `spin new myapp --force` -> exit 0, scaffold succeeds
- **Committed in:** cad2eee (Task 2 commit)

**2. [Rule 1 - Bug] Hardcoded max-distance 2 for flag suggestions (not cmd.SuggestionsMinimumDistance)**
- **Found during:** Task 4 (debugging TestUnknownFlagSuggestion)
- **Issue:** Initial implementation read `cmd.SuggestionsMinimumDistance` but this field is 0 for subcommands (cobra's `findSuggestions` only auto-sets it to 2 for command suggestions, not flags). With maxDist=0, no flag with any edit distance matches, so suggestions never appear.
- **Fix:** Hardcoded `const maxFlagDist = 2` inside `flagErrorFuncWithSuggestion`. Verified `--bubbltea` (distance 1) and `--bubblles` (distance 2) both suggest `--bubbletea`; `--completely-different` (distance 17) does not.
- **Files modified:** cmd/root.go
- **Verification:** `spin new myapp --bubbltea` exits 1 with "Did you mean --bubbletea?"
- **Committed in:** 77cecc0 (Task 4 commit)

**3. [Rule 1 - Bug] Built spin binary to /tmp instead of t.TempDir() in TestFangTTYEmitsANSI**
- **Found during:** Task 4 (debugging the TTY test)
- **Issue:** `t.TempDir()` creates a directory with mode 0700; when `script -qc <bin> --help /dev/null` tries to exec the binary, the parent dir's restrictive perms cascade and `script` reports "Permission denied" even though the binary itself is mode 0755. Reproduced manually with `ls -la` confirming the parent's 0700 mode.
- **Fix:** Built the binary to `filepath.Join(os.TempDir(), fmt.Sprintf("spin-pty-%d", os.Getpid()))` and registered `t.Cleanup(func() { _ = os.Remove(bin) })`. Also added a graceful `t.Skipf` when the script error contains "Permission denied" (Nix sandboxes where the workaround also fails).
- **Files modified:** cmd/help_test.go
- **Verification:** Test passes; emits `x1b[` ANSI codes
- **Committed in:** 77cecc0 (Task 4 commit)

**4. [Rule 1 - Bug] Fixed uint32 overflow in randStr test helper**
- **Found during:** Task 3 (TestGitInit_NoGit panic)
- **Issue:** `randStr` used `int(name[i])` for the hash; on some test names the cumulative product wraps to negative, then `letters[negative % 26]` panics with "index out of range [-16]". The first failed test was `TestGitInit_NoGit` (the t.Name() produced a hash that happened to wrap).
- **Fix:** Changed hash accumulator to `uint32` and shifted the modulo indexing to be unsigned. Added `h = h*7 + 1` per byte to spread bits.
- **Files modified:** internal/scaffold/validate_test.go
- **Verification:** All 19 subtests + 3 ProjectValidate tests + 5 hooks/git tests pass
- **Committed in:** 86ccb74 (Task 3 commit)

**5. [Rule 2 - Missing Critical] Added `go mod tidy` to VerifyBuild (3-step instead of 2-step)**
- **Found during:** Task 3 (planning the VerifyBuild implementation against the Walking Skeleton's existing 3-step helper)
- **Issue:** Plan said "Run two steps sequentially: `go build ./...` with CGO_ENABLED=0, then `go test ./...`". The Walking Skeleton's existing verifyBuild used 3 steps (tidy + build + test) added in Plan 01-01's deviation #4 because `go build` fails on a fresh scaffold with "missing go.sum entry". Dropping tidy would break the smoke test on every fresh scaffold.
- **Fix:** Kept the 3-step order. Documented in hooks.go's VerifyBuild doc comment.
- **Files modified:** internal/scaffold/hooks.go
- **Verification:** `spin new myapp --tui --bubbletea` from a fresh dir produces a `./myapp/` whose `go build ./...` and `go test ./...` both pass
- **Committed in:** 86ccb74 (Task 3 commit)

**6. [Rule 2 - Missing Critical] Added custom FlagErrorFunc for flag-suggestions (FLAG-18)**
- **Found during:** Task 4 (writing TestUnknownFlagSuggestion; `spin new myapp --bubbltea` did not suggest `--bubbletea`)
- **Issue:** Plan claimed `cobra.SuggestionsMinimumDistance = 2` enables unknown-flag suggestions. Verified in `cobra/command.go:findSuggestions` and the ParseFlags path that this only applies to command suggestions, not flag suggestions. The plan's verify step would have failed without this addition.
- **Fix:** Implemented `flagErrorFuncWithSuggestion` (Levenshtein distance against all registered flags on this cmd + parents, threshold 2). Set via `rootCmd.SetFlagErrorFunc(...)` in init().
- **Files modified:** cmd/root.go
- **Verification:** `spin new myapp --bubbltea` -> exit 1 with "Unknown flag: --bubbltea\nDid you mean --bubbletea?"
- **Committed in:** 77cecc0 (Task 4 commit)

---

**Total deviations:** 6 auto-fixed (6 Rule 1/2 — all bugs or missing-critical fixes; no architectural changes)
**Impact on plan:** All auto-fixes necessary for correctness or for the plan's own success criteria (FLAG-18 specifically required the FlagErrorFunc). The Walking Skeleton's overlay engine, Project struct design, embed root, and post-scaffold flow are unchanged. Only the smoke test step count and the flag-suggestion mechanism diverged from the plan's literal spec.

## Issues Encountered

- **Fang suppresses ANSI codes in non-TTY pipes** (lipgloss auto-detects TTY). TestFangStyledHelp originally asserted on `\x1b[`; rewrote to assert on fang's structural markers (uppercase "USAGE" / "COMMANDS" / "FLAGS" headers that cobra's plain default doesn't use). The TTY-allocated test (TestFangTTYEmitsANSI via `script -qc`) covers the actual ANSI assertion.
- **TestE2EScaffold (Walking Skeleton) still works** after the scaffold.New refactor — it constructs a Project with a non-existent target dir, chdirs into tmp, and calls New(). Validate is called by runNew in production but tests bypass runNew; the test passes because the dir is fresh.
- **20s `TestVerifyBuild_Failing` runtime** is dominated by `go mod tidy` trying to fetch the deliberately broken `nonexistent.invalid/nope` module before timing out. Acceptable for a unit test; could be sped up with a local proxy in CI but that's out of scope.
- **`script` binary and t.TempDir() interaction in Nix sandboxes** produced "Permission denied" errors. Workaround: build outside t.TempDir() + graceful skip on permission errors. The test still validates fang on TTY in normal Linux environments.

## User Setup Required

None - no external service configuration required. The CLI is a pure Go binary with embedded templates; `go install github.com/example/spin@latest` (after the orchestrator wires the real org) produces a working CLI. The version is ldflags-overridable for release builds.

## Next Phase Readiness

- Phase 1 Plan 3 (full template engine + overlays + license/README) can build directly:
  - Project struct has every field Plan 03's templates will reference (License, Template, all forward-compat bools)
  - The smoke test pipeline (VerifyBuild + GitInit) is stable; templates just need to render correctly for the overlay engine to pass
  - License templates (`LICENSE-MIT.tmpl`, `LICENSE-Apache-2.0.tmpl`) plug into the existing overlay engine via the new `p.License` field
  - The `gofumpt` / `goimports` / `air` / `prism` install commands from the Taskfile can land in Plan 03 without touching the scaffold engine
- Phase 2 (CLI variant + extended libs) needs only template content; ResolveFlags already binds every Phase 2 flag
- Phase 3 (interactive prompts) needs to add a `Prompter` interface and the `--no-interactive` flag in cmd/new.go; the Validate -> New pipeline can stay unchanged
- Known gap: `--bubbles` implies `--bubbletea` in `p.Libs` but the generated `go.mod` `go` directive still reads 1.23 in the Walking Skeleton's templates. Plan 03 should switch the go.mod template to use `{{goVersion .}}` to bump to 1.25.0 when bubbles is present.

## Self-Check

PASSED.

- All 4 task commits exist in git log: b823e89, cad2eee, 86ccb74, 77cecc0
- `go test ./... -count=1` passes (cmd 5.9s + scaffold 22.9s; all 36 tests green)
- `go vet ./...` is clean
- `go build .` produces a `spin` binary
- `./spin --help` shows fang-styled output (USAGE/COMMANDS/FLAGS uppercase, "new" subcommand listed)
- `./spin --version` outputs `spin version 0.1.0`
- `./spin new myapp --bubbltea` exits 1 with "Did you mean --bubbletea?"
- `./spin new MyApp` (uppercase) exits 1 with clear "must be 2-62 chars, lowercase..." error
- `./spin new test` (reserved) exits 1 with the same error
- `./spin new myapp` (existing dir) exits 1 with "directory './myapp' already exists; pass --force to overwrite"
- `./spin new myapp --force` (existing dir) overwrites and exits 0
- `./spin new myapp --bubbles` (implies --bubbletea) - scaffold generates both libs in p.Libs
- `./spin new myapp --no-git` exits 0; no `.git/` in the project
- `./spin new myapp --no-verify` exits 0; smoke test skipped (no `go mod tidy`/`go build` run)
- `./spin new myapp --license apache-2.0` exits 0; `p.License = "apache-2.0"` (no LICENSE file yet — that's Plan 03's template content)
- `spin new myapp --tui --bubbletea --bubbles --lipgloss` produces a project with `.git/` and 1 initial commit "scaffold myapp with spin 0.1.0"
- No `STATE.md` or `ROADMAP.md` modifications (orchestrator owns those)

---
*Phase: 01-scaffolder-foundation-core-tui-stack*
*Plan: 02*
*Completed: 2026-06-02*
