---
phase: 01-scaffolder-foundation-core-tui-stack
plan: 04
subsystem: testing
tags: [ci, grep, integration-test, scaffold, bash, go-test, charm-v2, template, repo-polish]

# Dependency graph
requires:
  - phase: 01-scaffolder-foundation-core-tui-stack
    plan: 01
    provides: "Walking Skeleton: spin binary, Project struct, scaffold engine, embedded templates, go build smoke test"
  - phase: 01-scaffolder-foundation-core-tui-stack
    plan: 02
    provides: "Full Project struct, ResolveFlags, Validate, VerifyBuild, GitInit"
  - phase: 01-scaffolder-foundation-core-tui-stack
    plan: 03
    provides: "Refactored template engine, CharmPins, full TUI overlay set, .air.toml/Taskfile.yml/LICENSE/README templates"
provides:
  - "scripts/check-v1-leaks.sh: bash CI grep suite (22 v1 Go patterns + 1 deprecated air pattern) with TOOL-03 + PITFALLS #1-4 + #10 coverage"
  - "Taskfile.yml: dev workflow with grep-v1-leaks, test, build, lint, clean targets"
  - "internal/scaffold/grep_test.go: 3 tests covering the grep suite's clean / fail-v1-import / fail-deprecated-air branches"
  - "internal/scaffold/integration_test.go: TOOL-05 end-to-end test (TestIntegrationScaffold) plus 2 cross-cutting sub-tests (NoBubblesGoVersion, LicenseVariants) -- scaffolds, builds, tests, greps; covers every TOOL-05 REQ-ID"
  - "spin repo polish: README.md (user-facing entry point), .gitignore (excludes /spin, /bin/, /tmp/, /dist/, *.exe, IDE dirs), .golangci.yml (gofmt+goimports+govet+errcheck+staticcheck+unused+ineffassign+misspell+gocritic), LICENSE (MIT)"
affects:
  - "phase-02 (CLI variant + extended libs) - integration test pattern extends cleanly to cover --cli + --cobra + --fang + --huh variants"
  - "phase-04 (doctor/add/update) - check-v1-leaks.sh can be invoked from `spin doctor` as the v1-leak health check"

# Tech tracking
tech-stack:
  added:
    - "bash 4+ (CI grep suite; Linux/macOS only; per RESEARCH §11.2)"
    - "go-task v3 (Taskfile.yml dev workflow runner)"
    - "golangci-lint v2 (opt-in via `task lint`; config in .golangci.yml)"
  patterns:
    - "Bash test invocation via os/exec - tests use exec.Command(\"bash\", scriptPath, targetDir) to run scripts/check-v1-leaks.sh; gives Go-test coverage of the bash script's exit-code + stderr contract"
    - "Test helper runSpinScaffold(t, name, flags) builds the spin binary, chdirs into a t.TempDir(), runs spin new, and returns the project dir + repo root - the canonical Phase 1+ E2E pattern"
    - "Black-box binary test that builds spin to os.TempDir() (NOT t.TempDir() -- its 0700 perms break downstream tooling in some sandboxes; same workaround as cmd/help_test.go TTY test)"
    - "Repo polish as a single feature commit: README + .gitignore + .golangci.yml + LICENSE ship together because they're all configuration/doc -- no code dependencies between them"

key-files:
  created:
    - "scripts/check-v1-leaks.sh - 76 lines: 22 Go patterns (v1 import paths, v1 API surface, v1 type assertions on msg) + 1 .air.toml pattern (deprecated build.bin); set -euo pipefail; per-pattern FAIL/OK with offending lines printed to stderr"
    - "Taskfile.yml - 30 lines: build/test/grep-v1-leaks/lint/clean targets; CGO_ENABLED=0 env; ./bin/spin binary path"
    - "internal/scaffold/grep_test.go - 142 lines: TestGrepV1Leaks_{TemplatesAreClean,CatchesV1Import,CatchesDeprecatedAir}; uses exec.Command(\"bash\", scriptPath, targetDir) to invoke the script"
    - "internal/scaffold/integration_test.go - 411 lines: TestIntegrationScaffold + TestIntegrationScaffold_NoBubblesGoVersion + TestIntegrationScaffold_LicenseVariants (3 sub-tests); 11 assertions in the main test (go.mod, main.go, internal/ui/styles.go, .air.toml, Taskfile.yml, LICENSE, README.md, .gitignore, .git/ + 1 commit, go build + go test, v1-leak grep)"
    - "README.md - 90 lines: What it does, Install, Quick start, Status (Phase 1 of 4 complete), Requirements, Documentation (links to .planning/), Development, Charm v2 only, License"
    - ".gitignore - 19 lines: /spin, /bin/, *.exe, *.test, *.out, /tmp/, /dist/, .DS_Store, IDE dirs, scratch test dirs"
    - ".golangci.yml - 22 lines: gofmt+goimports+govet+errcheck+staticcheck+unused+ineffassign+misspell+gocritic; test-file exclusion for errcheck+gocritic"
    - "LICENSE - 21 lines: spin repo MIT license (the root LICENSE is separate from the LICENSE template in templates/_base/)"
  modified: []

key-decisions:
  - "Pattern count: 22 Go patterns + 1 air pattern (per plan's expansion from 17 in RESEARCH §11). Catches msg.Type/Runes/Alt/X/Y as v1 struct-field accesses even though they could match unrelated `msg` structs in tests -- accepted per RESEARCH §11.2 (refine in Phase 2 by scoping to Bubble Tea files only)"
  - "Bash script lives at scripts/ (not internal/scaffold/scripts/) because the Taskfile target `task grep-v1-leaks` references it from the spin repo root; future CI jobs can also call it directly"
  - "TestIntegrationScaffold uses deterministic project name 'spin-integration-myapp' (matches a valid Go module path segment) so failures are reproducible"
  - "go.mod assertion forbids the v1 lib paths (github.com/charmbracelet/bubbletea, lipgloss, bubbles, huh, glamour, glow, wish, log, fang) but ALLOWS the github.com/charmbracelet/x/... indirect transitive deps that go mod tidy pulls in -- x/... is the current experimental namespace per CLAUDE.md tech stack, NOT a v1 leak"
  - "Lipgloss pin assertion: `charm.land/lipgloss/v2 v2.0.0` (post-tidy), not `v2.0.0-beta.2` (the template's literal). go mod tidy rewrites the pin to the latest matching (v2.0.0); this is correct go behavior per Plan 03 deviation"
  - "Spin repo LICENSE is MIT (not the org-name-based placeholder the templates use) - the spin repo MIT license applies to spin itself, the templates use whatever name the user passes"

patterns-established:
  - "Test helper runSpinScaffold(t, name, flags) -> (projectDir, repoRoot) - canonical Phase 1+ E2E test fixture; new E2E tests should use this helper, not reimplement the build/chdir/scaffold dance"
  - "assertNoV1Leaks(t, projectDir, repoRoot) runs scripts/check-v1-leaks.sh and treats non-zero exit as test failure - the v1-leak check is a hard assertion, not a soft warning"
  - "Repo polish (README + .gitignore + .golangci.yml + LICENSE) ships as a single feature commit because all four files are configuration/doc with no code dependencies on each other; partition by feature, not by file count"
  - "Spin binary built to os.TempDir() (not t.TempDir()) in E2E tests to avoid 0700 perms breaking downstream tooling - same workaround the Walking Skeleton + Plan 02 established for the TTY test"
  - "`task grep-v1-leaks` points at ./internal/scaffold/templates (the embedded templates) so it catches template-side regressions during development; the integration test in Task 2 covers the scaffolded-project path"

requirements-completed: [TOOL-03, TOOL-05]

# Metrics
duration: ~12min
completed: 2026-06-02
---

# Phase 1 Plan 4: CI Grep Suite + Integration Test + Repo Polish Summary

**CI grep suite catching 22 v1 charmbracelet API patterns, end-to-end TOOL-05 integration test (3 sub-tests covering scaffold + build + test + v1-leak grep + license variants), and a polished spin repo (README + .gitignore + .golangci.yml + LICENSE)**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-06-02T23:38:00Z (file timestamps from Task 1 start)
- **Completed:** 2026-06-02T23:48:00Z
- **Tasks:** 3 (all auto, all TDD)
- **Files modified:** 8 created (scripts/check-v1-leaks.sh, Taskfile.yml, internal/scaffold/grep_test.go, internal/scaffold/integration_test.go, README.md, .gitignore, .golangci.yml, LICENSE)
- **Test runtime:** TestIntegrationScaffold 1.09s; TestIntegrationScaffold_NoBubblesGoVersion 0.69s; TestIntegrationScaffold_LicenseVariants 2.08s; total integration suite 3.87s; full `go test ./... -count=1` 26.4s

## Accomplishments

- `scripts/check-v1-leaks.sh` (executable, bash) catches 22 v1 Go patterns (v1 import paths, `View() string`, `tea.WithAltScreen`, `tea.WithMouseCellMotion`, `tea.EnterAltScreen/HideCursor/ExitAltScreen`, `lipgloss.NewRenderer/DefaultRenderer/SetDefaultRenderer/AdaptiveColor{`/`ColorProfile(`/`HasDarkBackground()`, `tea.KeyCtrlC`, `tea.MouseButtonLeft/Right/Middle`, `msg.Type/Runes/Alt/X/Y`) plus 1 deprecated `.air.toml` `build.bin = "tmp/main"` pattern. Wired as `task grep-v1-leaks` in `Taskfile.yml`.
- `internal/scaffold/grep_test.go` covers the 3 grep branches (templates clean / v1 import detected / deprecated air detected) via `exec.Command("bash", scriptPath, targetDir)`. All 3 pass.
- `internal/scaffold/integration_test.go` ships 3 sub-tests that together prove Phase 1 works end-to-end:
  - `TestIntegrationScaffold` -- scaffolds `spin-integration-myapp --tui --bubbletea --bubbles --lipgloss`, asserts 11 file/structure/content properties (go.mod has the right v2 pins + go 1.25.0 + no v1 lib paths; main.go uses v2 API; internal/ui/styles.go uses `lipgloss.NewStyle`; .air.toml uses `build.entrypoint`; Taskfile.yml has the `setup:` target wiring all 4 tool installs; LICENSE is MIT with current year; README has Next steps + Prerequisites; .gitignore has tmp/ + bin/; .git/ has exactly 1 commit; `go build ./...` + `go test ./...` both exit 0; v1-leak grep exits 0)
  - `TestIntegrationScaffold_NoBubblesGoVersion` -- scaffolds with `--tui --bubbletea` only (no `--bubbles`); asserts go.mod does NOT bump to 1.25.0 (TOOL-02)
  - `TestIntegrationScaffold_LicenseVariants` -- scaffolds 3 projects with `--license mit`, `--license apache-2.0`, `--license none`; asserts LICENSE content matches (or file is absent for `none`)
- `README.md` is a polished user-facing entry point: What it does, Install, Quick start (with the canonical `spin new myapp --tui --bubbletea --bubbles --lipgloss` example), Status (Phase 1 of 4 complete), Requirements (Go 1.23+ for spin, Go 1.25.0+ for generated --bubbles projects), Documentation (links to .planning/), Development (task test, task grep-v1-leaks), Charm v2 only, License.
- `.gitignore` excludes the spin binary, `bin/`, `tmp/`, `dist/`, `*.exe`, IDE dirs, and test scratch dirs. `.golangci.yml` enables gofmt+goimports+govet+errcheck+staticcheck+unused+ineffassign+misspell+gocritic. `LICENSE` is MIT for the spin repo itself.
- **End-to-end smoke test PASSED**: from a clean `/tmp/smoke-test/`, built the spin binary, ran `spin new foo --tui --bubbletea --bubbles --lipgloss`, then `cd foo && CGO_ENABLED=0 go build ./... && go test ./...`, then `bash scripts/check-v1-leaks.sh /tmp/smoke-test/foo`. All three exit 0.

## Task Commits

1. **Task 1: CI grep suite (scripts/check-v1-leaks.sh + Taskfile.yml + grep_test.go)** - `8be935f` (feat)
2. **Task 2: TOOL-05 integration test (integration_test.go with 3 sub-tests)** - `3a3eec2` (feat)
3. **Task 3: spin repo polish (README.md + .gitignore + .golangci.yml + LICENSE)** - `6359aac` (feat)

**Plan metadata:** No separate docs commit -- summary committed as part of plan completion.

_Note: TDD was applied at the task level (one feat commit per task with tests written first within the task). All 3 tasks landed green on first run except for 2 minor assertion fixes in the integration test (lipgloss pin and v1-path scope -- see Deviations)._

## Files Created/Modified

- `scripts/check-v1-leaks.sh` - 76-line bash script; 22 Go patterns + 1 air pattern; `set -euo pipefail`; per-pattern FAIL/OK with offending lines
- `Taskfile.yml` - 30-line dev workflow; grep-v1-leaks/test/build/lint/clean targets; CGO_ENABLED=0 env
- `internal/scaffold/grep_test.go` - 142-line test file; 3 sub-tests covering the grep suite's clean / fail-v1-import / fail-deprecated-air branches
- `internal/scaffold/integration_test.go` - 411-line test file; `runSpinScaffold` test helper + 3 sub-tests (11 file/content assertions in the main one); uses `t.TempDir()` for the test dir, `os.TempDir()` for the spin binary
- `README.md` - 90-line user-facing entry point; sections: What it does, Install, Quick start, Status, Requirements, Documentation, Development, Charm v2 only, License
- `.gitignore` - 19-line ignore file; covers spin binary, build output, IDE/OS noise, test scratch dirs
- `.golangci.yml` - 22-line linter config; gofmt+goimports+govet+errcheck+staticcheck+unused+ineffassign+misspell+gocritic
- `LICENSE` - 21-line MIT license (separate from the templates/_base/LICENSE-MIT.tmpl template that gets emitted into generated projects)

## Decisions Made

- **Bash script at `scripts/` not `internal/scaffold/scripts/`.** The Taskfile target and the future CI job both need to reference it from the repo root, so the path is `scripts/check-v1-leaks.sh`. The grep_test.go uses `../../scripts/check-v1-leaks.sh` to find it from the test file's perspective.
- **22 Go patterns (not the 17 in RESEARCH §11.1).** Plan called for an expansion to 22 to cover the full set of v1->v2 removed APIs. The 5 extras beyond the original 17 are `tea.EnterAltScreen`, `tea.HideCursor`, `tea.ExitAltScreen`, and the 3 separate mouse constants (Left/Right/Middle). All catch patterns that compile but are semantically wrong.
- **Spin binary built to `os.TempDir()` in E2E tests (not `t.TempDir()`).** Walking Skeleton + Plan 02 already established this workaround for the TTY test (`script -qc` doesn't work under 0700 dirs). The integration test inherits the same pattern; documented in the test's helper comment.
- **Test name "spin-integration-myapp".** Deterministic name so test failures are reproducible. "spin-integration-myapp" is a valid Go module path segment (passes `IsValidGoModuleSegment`).
- **`assertNoV1Leaks` runs the bash script, not a Go-native grep.** Keeps the v1-leak check unified: the same script that CI runs is what the test runs. If the script ever regresses, the test fails too.
- **Spin repo LICENSE is MIT (not org-name-based).** The plan's template uses `Copyright (c) {{.Year}} {{.Name}}`; for the spin repo itself, the holder is "spin authors" and the year is 2026. This is a small editorial choice that keeps the spin repo MIT-licenseable today without committing to a specific org.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Integration test asserted lipgloss pin `v2.0.0-beta.2` (template's literal) but `go mod tidy` upgrades it to `v2.0.0`**
- **Found during:** Task 2 (first run of TestIntegrationScaffold)
- **Issue:** Plan said "contains `charm.land/lipgloss/v2 v2.0.0-beta.2`", but per Plan 03 deviation "go mod tidy in VerifyBuild will rewrite the lipgloss pin to v2.0.0 (the latest matching) after scaffold; this is correct go behavior". First test run failed with the upgraded pin.
- **Fix:** Updated the integration test's `assertGoModFullTUI` to expect `charm.land/lipgloss/v2 v2.0.0` (the post-tidy pin) and added a doc comment explaining the Plan 03 deviation.
- **Files modified:** internal/scaffold/integration_test.go
- **Verification:** TestIntegrationScaffold now passes; the pin in /tmp/smoke-test/foo/go.mod reads `charm.land/lipgloss/v2 v2.0.0`
- **Committed in:** `3a3eec2` (Task 2 commit)

**2. [Rule 1 - Bug] Integration test asserted go.mod does NOT contain `github.com/charmbracelet/` but `go mod tidy` adds indirect deps under `github.com/charmbracelet/x/...`**
- **Found during:** Task 2 (first run of TestIntegrationScaffold, same test run as #1)
- **Issue:** Plan said the go.mod assertion "Does NOT contain `github.com/charmbracelet/`". But after `go mod tidy` runs, the scaffolded go.mod pulls in indirect transitive deps like `github.com/charmbracelet/x/ansi`, `github.com/charmbracelet/x/term`, etc. -- these are the current `charmbracelet/x` experimental namespace per CLAUDE.md tech stack, NOT v1 leaks. The test false-positived on every scaffold.
- **Fix:** Tightened the forbidden list to the specific v1 library paths (bubbletea, lipgloss, bubbles, huh, glamour, glow, wish, log, fang) at `github.com/charmbracelet/<lib>`. The v1-leak check on the project's .go files is handled by the bash script (assertNoV1Leaks), so this go.mod assertion is just an extra safety net for the direct-require block.
- **Files modified:** internal/scaffold/integration_test.go
- **Verification:** TestIntegrationScaffold passes; the go.mod contains `github.com/charmbracelet/x/...` indirect deps (allowed) but no `github.com/charmbracelet/bubbletea` etc. (forbidden)
- **Committed in:** `3a3eec2` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 -- both bug fixes in test assertions; no architectural changes)
**Impact on plan:** Both auto-fixes were test-assertion adjustments, not changes to the scaffolder or the v1-leak check itself. The bash script (22 patterns + 1 air pattern), the integration test's 11 structural assertions, and the repo polish all landed as planned. The forbidden-pattern list in the test was narrowed to be precise about what constitutes a v1 leak (v1 library paths) vs what doesn't (current experimental namespace).

## Issues Encountered

- **End-to-end smoke test setup took 2 attempts.** First `go build -o /tmp/smoke-test/spin` from `/tmp/smoke-test` failed with "go.mod file not found" because I chdir'd into /tmp/smoke-test first; go build needs the module context. Fixed by removing the chdir from the build step (build from the loom repo root, output to /tmp/smoke-test/spin). Smoke test then completed in 1 attempt.
- **The Walking Skeleton + Plan 02 established the `os.TempDir()` (not `t.TempDir()`) workaround for the spin binary in E2E tests.** Reused the same pattern with a clear comment in the new `runSpinScaffold` helper.
- **Task partition: Task 3 ("repo polish") was a doc/config-only commit, not a TDD task.** README, .gitignore, .golangci.yml, LICENSE have no behavioral contract to test. The plan's "Tests" section for Task 3 correctly omits tests; verification is "files exist + grep checks for required patterns" via the `automated` verify line. This matches the plan's intent.

## User Setup Required

None - no external service configuration required. The CI grep suite and integration test are pure local Go + bash. `spin new <name> --tui --bubbletea --bubbles --lipgloss` continues to produce a runnable project on first try. End-to-end smoke test from a clean `/tmp/smoke-test/` confirmed all 3 steps (scaffold, build + test, v1-leak grep) exit 0.

## Next Phase Readiness

- Phase 2 (CLI variant + extended libs + toolchain wrappers) can build directly:
  - The `runSpinScaffold(t, name, flags)` helper + `assertNoV1Leaks` pattern extend to cover `--cli`, `--cobra`, `--fang`, `--viper`, `--huh`, etc. by adding more `assert*` helpers
  - `scripts/check-v1-leaks.sh` is generic enough to catch v1 leaks in any project (not just Phase 1 TUI scaffolds)
  - The Taskfile + .golangci.yml + README + .gitignore are reusable across all subsequent plans
  - The Walking Skeleton's `os.TempDir()` (not `t.TempDir()`) workaround is now documented in 2 places (cmd/help_test.go TTY test + integration_test.go E2E helper) -- no ambiguity for future plans
- Phase 3 (interactive prompts) needs the `runSpinScaffold` helper to support the `--no-interactive` flag (currently all tests scaffold with full TUI flags; Phase 3 will add a sub-test that omits the flags and asserts the defaults are used). One-line addition to the helper.
- Phase 4 (`spin doctor` + `spin add` + `spin update`) can call `scripts/check-v1-leaks.sh` as the v1-leak health check inside `spin doctor`. The script's contract (exits 0/1, prints offending lines) is exactly what a doctor subcommand needs.
- Known gap: the integration test scaffolds a single project per test (deterministic name); if two sub-tests run in parallel they could collide. Currently they run sequentially because `t.Parallel()` is not called, so this is fine for now. Future tests that add parallelism need unique project names.

## Self-Check

PASSED.

- All 3 task commits exist in git log: 8be935f, 3a3eec2, 6359aac
- `go test ./... -count=1` passes (cmd 5.4s + scaffold 26.4s; all tests green)
- `go vet ./...` is clean
- `go build .` produces a `spin` binary
- `scripts/check-v1-leaks.sh` is executable (`-rwxr-xr-x`) and runs cleanly against `./internal/scaffold/templates` (exit 0, "OK: no v1 leaks detected")
- `scripts/check-v1-leaks.sh` exits 1 when given a v1 import (verified by TestGrepV1Leaks_CatchesV1Import)
- `scripts/check-v1-leaks.sh` exits 1 when given a deprecated `.air.toml` (verified by TestGrepV1Leaks_CatchesDeprecatedAir)
- `task grep-v1-leaks` defined in Taskfile.yml (or `bash scripts/check-v1-leaks.sh ./internal/scaffold/templates` runs the same check)
- TestIntegrationScaffold scaffolds with all 3 TUI libs, validates 11 file/structure/content assertions, runs go build + go test + v1-leak grep -- all green
- TestIntegrationScaffold_NoBubblesGoVersion confirms TOOL-02: --tui --bubbletea (no --bubbles) does NOT bump go.mod to 1.25.0
- TestIntegrationScaffold_LicenseVariants confirms license flag works for mit, apache-2.0, none
- **End-to-end smoke test from clean /tmp**: `cd /tmp/smoke-test && spin new foo --tui --bubbletea --bubbles --lipgloss && cd foo && CGO_ENABLED=0 go build ./... && go test ./... && bash scripts/check-v1-leaks.sh /tmp/smoke-test/foo` -- all 3 steps exit 0
- All generated imports use `charm.land/<lib>/v2` paths
- README.md exists with `## Quick start` and `## Status` sections and the canonical `spin new myapp --tui --bubbletea --bubbles --lipgloss` example
- `.gitignore` excludes the spin binary, `bin/`, `tmp/`, `dist/`, `*.exe` (verified via `git check-ignore -v`)
- `.golangci.yml` configures at least gofmt + goimports + govet
- `LICENSE` is MIT for the spin repo itself
- No `STATE.md` or `ROADMAP.md` modifications (orchestrator owns those)

---
*Phase: 01-scaffolder-foundation-core-tui-stack*
*Plan: 04*
*Completed: 2026-06-02*
