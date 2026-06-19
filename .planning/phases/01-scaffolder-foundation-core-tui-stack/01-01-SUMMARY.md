---
phase: 01-scaffolder-foundation-core-tui-stack
plan: 01
subsystem: scaffolder
tags: [go, cobra, fang, bubbletea, lipgloss, log, go-embed, tui, walking-skeleton]

# Dependency graph
requires: []
provides:
  - "go install-able spin binary built with cobra v1.9.1 + fang v2"
  - "embedded template engine (//go:embed all:templates) with _base + variant_tui + lib/bubbletea overlay layers"
  - "Project struct contract (Name, Module, Type, Libs, Year, SpinVer) that all subsequent phases extend"
  - "post-scaffold `go mod tidy` + `go build ./...` smoke test with CGO_ENABLED=0"
  - "end-to-end CLI smoke test (TestE2EScaffold) that builds spin, runs it, validates output, and greps for v1 leaks"
affects:
  - "phase-02 (CLI variant, more libs, more flags) -- extends Project struct and overlay engine"
  - "phase-03 (interactive prompts, --ai) -- runs prompter before scaffold.New"
  - "phase-04 (doctor/add/update) -- operates on the Project struct and template tree"

# Tech tracking
tech-stack:
  added:
    - "github.com/spf13/cobra v1.9.1 (CLI framework; fang-tested floor)"
    - "charm.land/fang/v2 v2.0.1 (styled --help, errors, completions)"
    - "charm.land/lipgloss/v2 v2.0.3 (dogfooding; scaffolder output styling)"
    - "charm.land/log/v2 v2.0.0 (structured scaffolding logs)"
    - "go:embed for offline template tree"
  patterns:
    - "Walking Skeleton: _base + variant_<type> + lib/<name> overlay, last-write-wins"
    - "rootCmd as package-level var with RootCmd() accessor for testability"
    - "renderToMap(p) extracted as a pure helper (no FS writes) for unit testing"
    - "verifyBuild runs go mod tidy + go build ./... with CGO_ENABLED=0"
    - "Post-scaffold: emit files then smoke-test the generated project"

key-files:
  created:
    - "main.go (fang.Execute entrypoint)"
    - "cmd/root.go (cobra root with SuggestionsMinimumDistance=2)"
    - "cmd/new.go (Walking Skeleton flags: --tui, --bubbletea)"
    - "internal/scaffold/project.go (Project struct, TMPL-07 fields)"
    - "internal/scaffold/scaffold.go (New, renderToMap, verifyBuild; //go:embed all:templates)"
    - "internal/scaffold/scaffold_test.go (TestRenderToMapWalkingSkeleton, TestNewEndToEndWalkingSkeleton)"
    - "internal/scaffold/scaffold_e2e_test.go (TestE2EScaffold: black-box CLI smoke test)"
    - "internal/scaffold/templates/_base/{go.mod,main.go,README.md,.gitignore}.tmpl"
    - "internal/scaffold/templates/variant_tui/main.go.tmpl"
    - "internal/scaffold/templates/lib/bubbletea/bubbletea.go.tmpl"
  modified: []

key-decisions:
  - "Used github.com/example/spin as the safe-default module path (plan suggested <org>/spin or github.com/example/spin)"
  - "Pinned cobra v1.9.1 (fang-tested floor) per the plan's explicit pin; let fang/lipgloss/log resolve to latest"
  - "templates/ moved into internal/scaffold/templates/ because //go:embed cannot reach parent directories"
  - "verifyBuild now runs `go mod tidy` before `go build ./...` (plan omission: go build fails with 'missing go.sum entry' otherwise)"
  - "cmd/new.go builds the Project struct inline in runNew for the Walking Skeleton; Plan 02 introduces ResolveFlags"

patterns-established:
  - "Package-level rootCmd var + RootCmd() accessor: subcommand init() functions attach via the var; tests use the accessor"
  - "Walking Skeleton overlay order is hardcoded in overlayOrder(p): [templates/_base, templates/variant_<type>, templates/lib/<name>]"
  - "TestNewEndToEndWalkingSkeleton auto-skips when no templates exist (deferred-from-Task-2 pattern) so engine and templates can land independently"
  - "TestE2EScaffold is a black-box test: builds spin into os.TempDir(), runs the CLI, asserts file contents, runs go build + go test, and greps for v1 leaks"

requirements-completed: [SCAF-01, SCAF-05, SCAF-06, SCAF-07, FLAG-01, FLAG-04, TMPL-01, TMPL-04, TMPL-06, TOOL-02, TOOL-03]

# Metrics
duration: 18min
completed: 2026-06-02
---

# Phase 1 Plan 1: Walking Skeleton Summary

**Thinnest end-to-end vertical slice of `spin`: cobra + fang binary, embedded template engine, and a runnable bubbletea v2 TUI scaffold from `spin new myapp --tui --bubbletea`**

## Performance

- **Duration:** 18 min
- **Started:** 2026-06-02T19:22:06Z
- **Completed:** 2026-06-02T19:40:00Z
- **Tasks:** 4 (all auto, all TDD)
- **Files modified:** 11 (created)
- **Test runtime:** 0.93s (all three tests pass)

## Accomplishments

- `go install`-able `spin` binary with cobra v1.9.1 + fang v2; `spin --help` shows fang-styled help (not raw cobra)
- `spin new <name> --tui --bubbletea` from any directory produces a runnable bubbletea v2 TUI project in `./<name>/` that builds and tests clean with `CGO_ENABLED=0`
- Embedded template engine (//go:embed all:templates) walking three overlay layers (_base, variant_tui, lib/bubbletea) with last-write-wins
- Single `Project` struct (Name, Module, Type, Libs, Year, SpinVer) as the scaffold contract -- Plan 02/03 extend this without touching the engine
- Post-scaffold `go mod tidy` + `go build ./...` smoke test catches v1 imports, wrong version pins, and missing dependencies
- TestE2EScaffold: black-box test that builds spin, runs the CLI, validates every emitted file, runs go build + go test, and greps for v1 charmbracelet import leaks (<1s wall clock, no special tags)

## Task Commits

1. **Task 1: spin binary scaffold (go.mod + main.go + cmd/root.go + cmd/new.go)** - `83c02af` (feat)
2. **Task 2: scaffold engine (Project struct + renderToMap + verifyBuild)** - `2d0bedc` (feat)
3. **Task 3: Walking Skeleton templates (_base + variant_tui + lib/bubbletea)** - `bf3ef3b` (feat)
4. **Task 4: Walking Skeleton end-to-end smoke test (TestE2EScaffold)** - `5b779a1` (feat)

**Plan metadata:** No separate docs commit -- summary committed as part of plan completion.

_Note: TDD was applied at the task level (one feat commit per task with tests written first within the task). The Task 2/Task 3 split (engine-then-templates) is intentional: Task 2's tests fail RED until Task 3's templates land, then both flip GREEN. This is the same engine/templates decoupling documented in 01-SKELETON.md._

## Files Created/Modified

- `go.mod` / `go.sum` - spin module (github.com/example/spin), go 1.25.8 (auto-bumped from 1.23 by fang), cobra v1.9.1, fang v2.0.1, lipgloss v2.0.3, log v2.0.0
- `main.go` - 14-line fang.Execute entrypoint
- `cmd/root.go` - cobra root with SuggestionsMinimumDistance=2, Long description, hardcoded version="0.1.0"
- `cmd/new.go` - new subcommand with --tui and --bubbletea flags; constructs Project inline and calls scaffold.New
- `internal/scaffold/project.go` - Project struct with Walking Skeleton fields (Name, Module, Type, Libs, Year, SpinVer)
- `internal/scaffold/scaffold.go` - New, renderToMap, overlayOrder, emit, verifyBuild; //go:embed all:templates
- `internal/scaffold/scaffold_test.go` - TestRenderToMapWalkingSkeleton, TestNewEndToEndWalkingSkeleton
- `internal/scaffold/scaffold_e2e_test.go` - TestE2EScaffold (black-box CLI smoke test)
- `internal/scaffold/templates/_base/go.mod.tmpl` - module + go 1.23 + bubbletea v2.0.0
- `internal/scaffold/templates/_base/main.go.tmpl` - minimal hello-world (overwritten by variant_tui)
- `internal/scaffold/templates/_base/README.md.tmpl` - title, Next steps, Project layout, Prerequisites
- `internal/scaffold/templates/_base/.gitignore.tmpl` - tmp/, bin/, dist/, *.exe, .DS_Store, *.test, *.out
- `internal/scaffold/templates/variant_tui/main.go.tmpl` - counter with tea.KeyPressMsg + tea.View + tea.Quit
- `internal/scaffold/templates/lib/bubbletea/bubbletea.go.tmpl` - package main placeholder for Plan 03

## Decisions Made

- **Module path = `github.com/example/spin`.** The plan offered `<org>/spin` as a placeholder; the actual org isn't known to the executor and a future user can replace it with `go mod edit -module=...`. This keeps the Walking Skeleton committable today.
- **Pinned cobra to v1.9.1** as the fang-tested floor per the plan. Let fang, lipgloss, and log resolve to their latest stable (v2.0.1, v2.0.3, v2.0.0 respectively) since the plan listed them as "latest stable" without a specific pin.
- **Moved templates/ into `internal/scaffold/templates/`.** The plan specified `templates/` at the repo root, but `//go:embed` cannot reach parent directories. The Go embed directive is resolved relative to the source file, so templates must live alongside the file declaring the embed.
- **Added `go mod tidy` to `verifyBuild`.** The plan specified only `go build ./...`, but a fresh generated project has no `go.sum`; `go build` fails with "missing go.sum entry for module providing package charm.land/bubbletea/v2" before the v1-leak gate can fire. `go mod tidy` populates the sum file and lets the build proceed.
- **Auto-bumped `go` directive to 1.25.8** in spin's own go.mod. The plan specified `go 1.23`, but `charm.land/fang/v2 v2.0.1` requires `go 1.25.0` -- `go mod tidy` would not accept a lower floor.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added missing `//go:embed` directive in scaffold.go**
- **Found during:** Task 3 (first test run after writing templates)
- **Issue:** `var FS embed.FS` was declared but the `//go:embed all:templates` directive was missing above it. The package compiled (empty embed.FS is valid Go) but the FS contained zero files, so `renderToMap` returned "no templates found".
- **Fix:** Added `//go:embed all:templates` on the line directly above the `var FS embed.FS` declaration. The `all:` prefix includes hidden files like `.gitignore` per RESEARCH §4.1.
- **Files modified:** internal/scaffold/scaffold.go
- **Verification:** `go test ./internal/scaffold/... -count=1 -run TestRenderToMapWalkingSkeleton` now passes
- **Committed in:** `bf3ef3b` (part of Task 3 commit)

**2. [Rule 1 - Bug] Moved templates/ into internal/scaffold/ to satisfy go:embed path semantics**
- **Found during:** Task 3 (after fix #1 above)
- **Issue:** `//go:embed all:templates` failed with "pattern all:templates: no matching files found" because the pattern is resolved relative to the source file (internal/scaffold/scaffold.go), but templates/ was at the repo root. `//go:embed` cannot use `..` in patterns.
- **Fix:** Moved the entire `templates/` tree under `internal/scaffold/templates/`. The Walking Skeleton's overlay structure is unchanged; only the depth shifted by one level.
- **Files modified:** internal/scaffold/templates/ (moved)
- **Verification:** embed compiles, all template files reachable via fs.ReadFile
- **Committed in:** `bf3ef3b` (part of Task 3 commit)

**3. [Rule 1 - Bug] Stripped `{{define}}` from bubbletea.go.tmpl comment to avoid template parse error**
- **Found during:** Task 3 (first embed test after fix #1)
- **Issue:** `text/template` parsed the literal `{{define}}` inside a `//` comment as a template directive and failed with "unexpected '}}' in define clause". The Walking Skeleton plan had this exact phrase in the comment.
- **Fix:** Reworded the comment to "define blocks" (no curly braces) so text/template ignores it.
- **Files modified:** internal/scaffold/templates/lib/bubbletea/bubbletea.go.tmpl
- **Verification:** `go test ./internal/scaffold/... -count=1` now passes
- **Committed in:** `bf3ef3b` (part of Task 3 commit)

**4. [Rule 2 - Missing Critical] Added `go mod tidy` to verifyBuild before `go build ./...`**
- **Found during:** Task 3 (TestNewEndToEndWalkingSkeleton first run)
- **Issue:** The plan specified verifyBuild runs only `go build ./...`, but a freshly scaffolded project has no `go.sum`. `go build` fails with "missing go.sum entry for module providing package charm.land/bubbletea/v2" before the v1-leak gate can fire. The Walking Skeleton's "perfect first run" promise requires the build to actually succeed.
- **Fix:** verifyBuild now runs `go mod tidy` first (populates `go.sum` and resolves the lockfile) then `go build ./...` with `CGO_ENABLED=0`. Both failures surface their stderr verbatim per RESEARCH §7.2.
- **Files modified:** internal/scaffold/scaffold.go
- **Verification:** TestNewEndToEndWalkingSkeleton passes; manual `spin new myapp --tui --bubbletea` produces a `myapp/` that builds and tests clean
- **Committed in:** `bf3ef3b` (part of Task 3 commit)

**5. [Rule 2 - Missing Critical] Auto-bumped `go` directive in spin's go.mod from 1.23 to 1.25.8**
- **Found during:** Task 1 (after `go mod tidy` to resolve deps)
- **Issue:** Plan specified `go 1.23` in spin's own go.mod, but `charm.land/fang/v2 v2.0.1` requires `go 1.25.0`. `go mod tidy` would not accept a `go 1.23` directive when a dependency requires 1.25.0; the build fails with "module declares its go directive as 1.23 but the resolved go.mod requires 1.25.0".
- **Fix:** Accepted the auto-bumped `go 1.25.8`. No `toolchain` directive is set, so the user can build with any Go 1.25+ toolchain. The Walking Skeleton's stated "go 1.23" floor was incompatible with the fang v2 choice the plan also mandated.
- **Files modified:** go.mod
- **Verification:** `go build .` succeeds; `go vet ./...` is clean
- **Committed in:** `83c02af` (part of Task 1 commit)

---

**Total deviations:** 5 auto-fixed (5 Rule 1/2 -- all bugs or missing-critical fixes; no architectural changes)
**Impact on plan:** All auto-fixes were necessary for correctness. The Walking Skeleton's architectural decisions (overlay engine, Project struct, embed root, post-scaffold smoke test) are unchanged. Only the template location and the go directive floor shifted to satisfy Go's `//go:embed` and module-system constraints.

## Issues Encountered

- **Test 2 (TestNewEndToEndWalkingSkeleton) auto-skip mechanism** -- Task 2's end-to-end test was written before templates existed. Implemented a "probe + skip" pattern: if the first renderToMap call returns a "no templates" error, the test skips with a clear message pointing at Task 3. This kept Task 2 (engine) and Task 3 (templates) committable independently, which matched the plan's structure. After Task 3 landed, the probe returned real content and the test ran end-to-end.
- **The Write tool interpreted `templates/lib/bubbletea.go.tmpl` as a file directly in `templates/lib/`, not in `templates/lib/bubbletea/`.** Caught by `find` and fixed with a `mv` before commit. No downstream impact -- the file ended up in the right place.
- **Generated `bubbletea.go` at project root** is a `package main` placeholder per the plan. Plan 03 will replace this with the proper "lib overlay injects via {{define}} blocks or internal/ui/" pattern. For the Walking Skeleton it's harmless: compiles, ships a comment, takes no runtime resources.

## User Setup Required

None - no external service configuration required. The Walking Skeleton is a pure Go binary with embedded templates; `go install github.com/example/spin@latest` (after the orchestrator wires the real org) produces a working CLI.

## Next Phase Readiness

- Phase 2 (CLI variant + extended libraries + toolchain wrappers) can build directly on this skeleton:
  - Extend `Project` struct with License, Template, Force, NoGit, Quiet, and the Phase 2 boolean flags (Cobra, Fang, Viper, Huh, Glamour, Glow, Wish, Log, Harmonica, Modifiers, Ansi, Runewidth).
  - Add the corresponding `templates/lib/<lib>/*.tmpl` overlays (most will be small additions to the existing overlay engine).
  - Wire `--cli` / `--cobra` / `--fang` template content under `variant_cli/`.
  - Add `.air.toml`, `Taskfile.yml`, `LICENSE` to `_base/`.
  - The hardcoded `overlayOrder()` in `internal/scaffold/scaffold.go` should be replaced with a `p.Type` / `p.Libs`-driven dynamic order at the start of Phase 2.
- Phase 3 (interactive prompts) needs the `--no-interactive` flag added to `cmd/new.go` and a `Prompter` interface stub created before `--ai`/`--agents` flags land.
- The `go mod tidy` step in `verifyBuild` is a candidate for replacement with `go mod download` (faster, no internet required if modules are cached) once Plan 02 considers build-time determinism.

## Self-Check

PASSED.

- All 4 task commits exist in git log: 83c02af, 2d0bedc, bf3ef3b, 5b779a1.
- `go test ./... -count=1` passes (3 tests in 0.93s).
- `go vet ./...` is clean.
- `go build .` produces a `spin` binary.
- `./spin --help` shows fang-styled output (not raw cobra).
- `./spin new` (missing arg) exits non-zero with cobra's "Accepts 1 arg(s)" error.
- Manual E2E: `spin new myapp --tui --bubbletea` from `/tmp/walking-skel-test/` produced a runnable bubbletea v2 project; `CGO_ENABLED=0 go build ./...` and `go test ./...` both pass; `grep -rE "github.com/charmbracelet/" --include='*.go' myapp/` returns 0 matches.
- All generated imports use `charm.land/<lib>/v2` paths.
- No `STATE.md` or `ROADMAP.md` modifications (orchestrator owns those).

---
*Phase: 01-scaffolder-foundation-core-tui-stack*
*Completed: 2026-06-02*
