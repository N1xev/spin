---
phase: 02
plan: 01
title: Foundation refactor — pins, go directive, path-traversal guard, grep refinement
subsystem: scaffold
tags: [pins, go-version, security, grep]
completed: 2026-06-02
duration: ~25m
tasks: 5 commits (Task 6 was smoke-only, no separate commit per plan)
files_created: []
files_modified:
  - internal/scaffold/versions.go
  - internal/scaffold/scaffold.go
  - internal/scaffold/template.go
  - internal/scaffold/templates/_base/go.mod.tmpl
  - internal/scaffold/templates/_base/README.md.tmpl
  - scripts/check-v1-leaks.sh
  - internal/scaffold/scaffold_test.go
  - internal/scaffold/integration_test.go
  - internal/scaffold/template_test.go
  - internal/scaffold/grep_test.go
  - internal/scaffold/scaffold_e2e_test.go
requirements:
  - TOOL-01
  - TOOL-02
  - TOOL-03
  - WRAP-07
  - WRAP-08
key_findings:
  - "All 11 charmbracelet pins bumped to the latest stable per Phase 2 research §2.1 (verified via go list + Context7)"
  - "The v1 conditional `{{if hasBubbles}}` go directive is dead code and was removed; every generated go.mod is now unconditionally 1.25.0"
  - "emit() now has a path-traversal guard that rejects any rendered path resolving outside the project root"
  - "The grep suite moved from a blanket `github.com/charmbracelet/` ban to a per-module deny-list, allowing harmonica and glow (per the §2.1 correction that those libs have not migrated to charm.land)"
  - "Test count: 47 → 57 (+10 across 4 new top-level tests and 6 new sub-tests)"
one_line_summary: "Foundation refactor: pin bumps, unconditional 1.25.0 go directive, path-traversal guard in emit(), and per-module v1-leak grep deny-list (allow harmonica + glow)"
---

# Phase 2 Plan 1: Foundation refactor — Summary

This plan is the foundation for Phase 2. It fixes the four things the v1
research got wrong (per the Phase 2 research §1 / §2 / §7) so that
everything else in Phase 2 builds on solid ground:

1. **Pin freshness**: bumped all 11 lib pins (10 `charm.land/<lib>/v2` +
   1 `github.com/charmbracelet/harmonica` per the §2.1 correction) to the
   latest stable per the research.
2. **Go version floor**: collapsed the v1 `{{if hasBubbles .}}1.25.0{{else}}1.23{{end}}`
   dead branch to `go 1.25.0` always. Per research §2.2, every charm v2
   lib requires 1.25.0+ transitively; the 1.23 branch was unreachable.
3. **Path-traversal guard**: hardened `emit()` so a template that renders
   `{{.Name}}` to `../../etc/passwd` cannot write outside the project
   root.
4. **Grep suite refinement**: stopped the blanket `github\.com/charmbracelet/`
   ban from false-positiving on `github.com/charmbracelet/harmonica` and
   `github.com/charmbracelet/glow/v2` (which the research §2.1 confirms
   are still the current paths for those two libs).

## Tasks executed

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Bump version pins | 3ee4929 | internal/scaffold/versions.go |
| 2 | Simplify go directive | 0efa836 | go.mod.tmpl, README.md.tmpl |
| 3 | Path-traversal guard | 9585618 | internal/scaffold/scaffold.go |
| 4 | Per-module v1-leak grep | 8cf36e1 | scripts/check-v1-leaks.sh |
| 5 | Test updates + new tests | 58274c7 | 6 test/source files |
| 6 | End-to-end smoke | (no commit) | — verification only, no regression found |

## Deviations from plan

### [Plan-extension] `template.go` charmPin switch extended (in Task 5 commit)

**Found during:** Task 5 test run.

**Issue:** The plan stated "Plan 02-02's `charmPin` template-func
switch will reference the new fields" — implying Task 5's new
`TestFuncMap_CharmPin` assertions for huh/glamour/wish/fang should
NOT pass yet, with Plan 02-02 making the test go green. But the
Task 5 commit also asserts those new pins work end-to-end in
`TestRenderToMap_FullTUI` (which goes through `charmPin`). The
`template.go` switch therefore had to be extended now so the new
pin assertions could pass.

**Fix:** Added four new cases (huh, glamour, wish, fang) to the
`charmPin` switch in `template.go` (≈8 lines). They are pure
additive lookups against the new `DefaultPins` fields introduced
in Task 1.

**Files modified:** `internal/scaffold/template.go`

**Commit:** 58274c7 (bundled with Task 5 to keep the test commit
logically atomic: the tests are green because the switch is
extended, both live in the same change).

### [Plan-extension] `scaffold_e2e_test.go` and `integration_test.go` pin assertions updated (in Task 5 commit)

**Found during:** Task 5 test run (the integration tests had hard-coded
the old `v2.0.0` / `v2.0.0-beta.2` pins that Task 1 had just replaced).

**Issue:** `TestIntegrationScaffold` (in `integration_test.go`) and
`TestE2EScaffold` (in `scaffold_e2e_test.go`) hard-coded the old pin
expectations. With Task 1's pin bump, those assertions failed.

**Fix:** Updated both tests to the new pins
(bubbletea v2.0.7, lipgloss v2.0.3, bubbles v2.1.0). The plan's
Task 5 description listed `integration_test.go` in the files list
but did not explicitly call out the scaffold_e2e_test.go update;
the latter is a parallel file-level change driven by the same
pin bump and is in scope of the test-update commit.

**Files modified:** `internal/scaffold/scaffold_e2e_test.go`,
`internal/scaffold/integration_test.go`

**Commit:** 58274c7.

### [Plan-extension] `scaffold_test.go` `TestRenderToMapWalkingSkeleton` `go 1.23` → `1.25.0` (in Task 5 commit)

**Found during:** Task 5 test run.

**Issue:** `TestRenderToMapWalkingSkeleton` in `scaffold_test.go`
still hard-coded `go 1.23` as the expected go directive for the
Walking Skeleton scaffold. With Task 2's removal of the conditional
branch, that assertion is wrong.

**Fix:** Updated the assertion to `go 1.25.0` (the new unconditional
value). The plan listed `scaffold_test.go` in the Task 5 files
list but did not name this specific test in its action list; the
update is the natural consequence of Task 2's directive change and
belongs with the test commit.

**Files modified:** `internal/scaffold/scaffold_test.go`

**Commit:** 58274c7.

## Test count delta

- Before: **47** tests
- After: **57** tests (+10)
  - 4 new top-level tests: `TestEmit_PathTraversal`,
    `TestEmit_HappyPath`, `TestGrepV1Leaks_AllowsHarmonica`,
    `TestGrepV1Leaks_AllowsGlowV2`
  - 6 new sub-cases:
    - `TestEmit_PathTraversal` — 3 sub-cases (absolute_unix,
      relative_traversal, mixed_traversal)
    - `TestRenderToMap_GoVersion` — 1 new sub-case
      (bubbles_implies_bubbletea) on top of 2 existing
    - `TestGrepV1Leaks_AllowsGlowV2` — no sub-cases (top-level)
  - Net 1 renamed test: `TestIntegrationScaffold_NoBubblesGoVersion`
    → `TestIntegrationScaffold_AlwaysGo1250` (semantic flip —
    was asserting the absence of 1.25.0, now asserts its presence)

## Verification

- `go build ./...` exits 0
- `go test ./... -count=1` exits 0 (57 tests, 0 skipped)
- `go vet ./...` exits 0
- Smoke test 1 (`/tmp/testapp` with `--tui --bubbletea --bubbles --lipgloss`):
  builds clean, `go test ./...` exits 0, `go.mod` has `go 1.25.0`
  unconditional, `scripts/check-v1-leaks.sh` exits 0 (no v1 leaks
  detected), and `grep -E "github.com/charmbracelet/(bubbletea|lipgloss|bubbles|huh|glamour|wish|log|fang)" go.mod`
  returns no matches.
- Smoke test 2 (`/tmp/testapp2` with `--cli --cobra --fang`):
  fails with the expected Phase 2 placeholder error
  (`--type=cli: this variant ships in Phase 2; use --tui --bubbletea
  (and optionally --bubbles, --lipgloss) instead`). This is the
  intended Phase 1 state; --cli is deferred to Plan 02-03.

## Known Stubs

None. The plan's Task 6 verified that the refactor introduced no
new stubs; all rendered files (go.mod, main.go, README.md, .air.toml,
Taskfile.yml, LICENSE, internal/ui/styles.go) continue to render
their Phase 1 content with the new pins and unconditional 1.25.0
go directive.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: path-traversal-mitigation | internal/scaffold/scaffold.go | New `path traversal` error path in `emit()` blocks a template that renders `{{.Name}}` (or any other user-controlled value) to a path that resolves outside the project root. Constant-time per file, defense-in-depth. |
| threat_flag: v1-leak-mitigation | scripts/check-v1-leaks.sh | New per-module deny-list (`V1_LEAK_PATTERNS`) replaces the blanket `github\.com/charmbracelet/` ban, allowing the legitimate current paths `github.com/charmbracelet/harmonica` and `github.com/charmbracelet/glow/v2` while still catching the 8 migrated modules. |
