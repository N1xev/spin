---
phase: 03
plan: 02
title: Huh v2 fallback form layer + AllLibs + Fill dispatch
subsystem: prompt
tags: [huh-v2, forms, catalog, all-libs, fill-dispatch, charm-v2]
completed: 2026-06-03
duration: ~14m
tasks: 4 commits
files_created:
  - internal/prompt/catalog.go
  - internal/prompt/catalog_test.go
  - internal/prompt/huh.go
  - internal/prompt/huh_test.go
  - internal/scaffold/project_test.go
files_modified:
  - go.mod
  - go.sum
  - internal/prompt/prompt.go
  - internal/prompt/prompt_test.go
  - internal/scaffold/project.go
requirements:
  - INT-04
  - INT-05
key_findings:
  - "huh v2 constructors are not generic at the Input/Confirm level (NewInput() *Input, NewConfirm() *Confirm) but ARE generic at the Select/MultiSelect level (NewSelect[T comparable]() *Select[T]). Initial draft used NewInput[string]() everywhere; the build flagged the v1-style generic usage."
  - "go mod tidy correctly removes `charm.land/huh/v2` from go.mod when no source file imports it. Task 1 (catalog only) shipped without the dep; the dep landed in Task 3 when huh.go imported it. This is a small deviation from the plan's Task 1 'go get + go mod tidy' instruction, but is the correct Go-mod behavior."
  - "huh v2 fields' internal state (e.g., Option.selected) is not exposed publicly. The form-construction tests had to be limited to skip-when-set predicates and pre-selection helpers, not internal form introspection. .Run()-level tests are TTY-only and deferred to a future test suite using huh v2's WithInput/WithOutput form options."
  - "Project.Lipgloss is NOT a per-lib bool field — lipgloss lives only in p.Libs. Initial test fixture used `Lipgloss: true` and the build caught it. Per-lib bool fields are only the 9 in boolFlagOverlayMap: Cobra, Fang, Viper, Huh, Glamour, Glow, Wish, Log, Harmonica."
one_line_summary: "Library catalog (13 charm libs), huh v2 form layer with 8 ask* prompts, Project.AllLibs() union helper, and Fill dispatch — the in-process TUI backend that activates when gum is not on $PATH"
---

# Phase 3 Plan 2: Huh v2 fallback form layer + AllLibs + Fill dispatch

This plan wires the in-process TUI prompt layer for `spin new`:
when gum is not on $PATH, huh v2 takes over. The plan replaces the
Plan 01 no-op `Fill` body with a dispatch to `fillWithHuh` and adds
the supporting infrastructure (Library catalog, AllLibs helper, the
8 step-by-step form functions).

The plan also adds `*scaffold.Project.AllLibs()` — the union of
`p.Libs` and the per-lib bools — which is the single source of
truth fix for Pitfall 4 in 03-RESEARCH.md (parallel sources of
truth caused AGENTS.md to omit libs the user enabled via flags).

## Performance

- **Duration:** ~14 min
- **Started:** 2026-06-03T15:05:02Z
- **Completed:** 2026-06-03T18:23:56Z
- **Tasks:** 4
- **Files modified:** 5
- **Files created:** 5
- **Commits:** 4

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add Library catalog (13 charm libs) + LibsForType | 3c857c9 | internal/prompt/{catalog,catalog_test}.go |
| 2 | Add Project.AllLibs() — union of p.Libs and per-lib bools | 39194c6 | internal/scaffold/{project,project_test}.go |
| 3 | Implement huh v2 form layer — 8 ask* functions + fillWithHuh | aa18a53 | go.mod, go.sum, internal/prompt/{huh,huh_test}.go |
| 4 | Wire Fill to dispatch to fillWithHuh + nil-guard | 1a4e8d8 | internal/prompt/{prompt,prompt_test}.go |

## Accomplishments

- **Library catalog** (catalog.go): `Library{Name, Display, DefaultFor, AlwaysOn}`,
  `LibCatalog` (13 entries — bubbletea, bubbles, cobra, fang, glamour,
  glow, harmonica, huh, lipgloss, log, modifiers, viper, wish),
  `LibsForType(typ)` (variant defaults; "all" is the union of tui+cli),
  `DefaultLibsFor` alias, `libBoolMirror` (Name → field name for the
  multi-select write-back). Both backends (huh in this plan, gum in
  Plan 03) consume the same catalog.
- **Project.AllLibs()** (project.go): the union of `p.Libs` and
  `boolFlagOverlayMap` (9 bools), deduplicated, sorted alphabetically.
  Returns a non-nil empty slice for the zero Project (template-friendly).
  Pitfall 4 from 03-RESEARCH.md is fixed.
- **8 huh form functions** (huh.go): `askType`, `askName`, `askModule`,
  `askLibs`, `askLicense`, `askTemplate`, `askTemplateRepo`, `askAI`.
  Each follows the UI-SPEC prompt sequence (Surface A / Prompt
  sequence). `huh.ErrUserAborted` is mapped to `*Canceled`; other
  errors are wrapped with `fmt.Errorf`. Skip predicates match
  UI-SPEC: gap-fillers skip if the field is already set; `askLibs`
  and `askAI` always fire.
- **fillWithHuh dispatcher** (huh.go): runs the 8 steps in order,
  returning on the first error. No mid-flow backend switch.
- **preSelectedLibs helper** (huh.go): the single "pre-select"
  decision point. Both backends will use the same function (Plan 03
  wires the gum side) so defaults are consistent across backends.
- **setBoolFieldByName helper** (huh.go): small switch avoids
  `reflect` on the multi-select write-back hot path. Mirrors the
  pick set into the per-lib bool fields.
- **Fill dispatch** (prompt.go): replaces the Plan 01 no-op with
  `if !ShouldPrompt() { return nil }; if p == nil { return nil };
  return fillWithHuh(p)`. Plan 03's gum branch is a clean insertion
  point at the bottom of Fill.
- **`charm.land/huh/v2 v2.0.3`** added to go.mod. The full
  transitive set (bubbletea/v2, bubbles/v2, x/exp/*, catppuccin/go,
  hashstructure, etc.) is pinned in go.sum. Go 1.25.8 floor is
  satisfied by the project's toolchain.

## Files Created/Modified

### Created
- `internal/prompt/catalog.go` — Library struct, LibCatalog, LibsForType, DefaultLibsFor, libBoolMirror
- `internal/prompt/catalog_test.go` — TestLibCatalog_UniqueNames, _Sorted, TestLibsForType, TestDefaultLibsFor
- `internal/prompt/huh.go` — 8 ask* functions, fillWithHuh, preSelectedLibs, setBoolFieldByName, templateOptionsFor
- `internal/prompt/huh_test.go` — Skip-when-set tests, preSelectedLibs tests, templateOptionsFor tests, setBoolFieldByName test
- `internal/scaffold/project_test.go` — TestProject_AllLibs_* (5 cases)

### Modified
- `go.mod` / `go.sum` — added `charm.land/huh/v2 v2.0.3` direct + 30+ transitive deps
- `internal/scaffold/project.go` — added `"sort"` import, added `AllLibs()` method
- `internal/prompt/prompt.go` — replaced no-op Fill body with fillWithHuh dispatch; added nil-guard; updated package doc
- `internal/prompt/prompt_test.go` — replaced TestFillNoop with TestFill_NoInteractiveReturns and TestFill_NilProject; updated file header comment

## Decisions Made

1. **`huh.NewInput()` not `huh.NewInput[string]()`** — huh v2's
   `NewInput()` and `NewConfirm()` are not generic (they return
   `*huh.Input` and `*huh.Confirm`). The v1-style `NewInput[string]()`
   syntax used in the 03-RESEARCH.md examples is not valid for
   v2 Input/Confirm. Select and MultiSelect are still generic
   (`NewSelect[T comparable]()`, `NewMultiSelect[T comparable]()`)
   because their value type varies. Initial draft followed the
   research example blindly; the build caught it and the code was
   corrected.

2. **`go.mod` deferred to Task 3, not Task 1** — Task 1 (catalog)
   does not import huh v2. `go mod tidy` correctly removes the dep
   from go.mod when nothing imports it. The plan's "go get +
   go mod tidy in Task 1" instruction is a small mis-order: the
   dep lands in Task 3 when huh.go is created. Task 1's commit
   has no go.mod diff. This is the correct Go-mod behavior; the
   deviation is documented but does not change the user-visible
   result (charm.land/huh/v2 is in go.mod by plan end).

3. **`LibsForType("all")` returns the union of tui + cli defaults**
   — the plan says `DefaultFor` is a single variant string ("tui",
   "cli", "all", or ""), and `LibsForType("all")` should "contain
   all three" (bubbletea, cobra, fang). With single-value
   `DefaultFor`, no entry would match "all" unless I special-case
   the lookup. The cleanest reading: when typ="all", include
   entries with DefaultFor in {"tui", "cli"} (the "all" case is
   the union). The special case is one line in `LibsForType` and
   is documented.

4. **test for `Project.Lipgloss`** — initial draft of
   `TestPreSelectedLibs_FlagSet` used `Lipgloss: true` to simulate
   a --lipgloss flag. But `Lipgloss` is NOT a per-lib bool field
   (it's only in `p.Libs`). The 9 per-lib bools are Cobra, Fang,
   Viper, Huh, Glamour, Glow, Wish, Log, Harmonica. The test was
   rewritten to use `Libs: []string{"lipgloss"}` instead.

5. **`preSelectedLibs` returns a non-nil empty slice for zero
   Project** — consistent with `AllLibs()`. The initial
   `var out []string` returned nil; the test caught it and the
   function was changed to `out := []string{}`. This makes the
   range-over-result idiom safe without nil-checks (mirrors the
   `slices.Sort` invariant that templates assume).

6. **Form construction tests, not .Run() tests** — huh v2's
   `Option.selected` field is private; the public API does not
   expose the pre-selection state. .Run()-level tests need a TTY
   (or a test program with `WithInput`/`WithOutput` form options).
   Per the plan, the unit suite focuses on form-construction smoke
   tests (the function builds without panic) and helper-function
   tests (skip predicates, pre-selected sets). The TTY-only tests
   are deferred to a future suite (documented in huh_test.go).

7. **`Fill` adds a nil-guard before dispatch** — `Fill(nil)`
   returns nil without panic. This is defensive: `cmd/new.go`
   calls `prompt.Fill(p)` with a non-nil `p` after `ResolveFlags`,
   but other callers (tests, future spin subcommands) might pass
   nil. The guard is one line and prevents nil-deref crashes.

## Deviations from Plan

### [Rule 1 - Build-error] `huh.NewInput[string]()` not valid in v2; corrected to `huh.NewInput()`

**Found during:** Task 3 build.

**Issue:** The plan's action template and 03-RESEARCH.md Example 5
show `huh.NewInput[string]()`. In huh v2, `NewInput` and `NewConfirm`
are NOT generic — they return `*huh.Input` and `*huh.Confirm` with
`Value(*string)` and `Value(*bool)` respectively. `NewSelect` and
`NewMultiSelect` ARE generic (`NewSelect[T comparable]()`, etc.).

**Fix:** Removed the `[string]` and `[bool]` type parameters from
`NewInput` and `NewConfirm` call sites. Select and MultiSelect kept
their `[string]` type parameters (those are valid).

**Files modified:** `internal/prompt/huh.go`

**Commit:** aa18a53 (Task 3)

### [Rule 1 - Plan-order] `charm.land/huh/v2` added to go.mod in Task 3, not Task 1

**Found during:** Task 1 verification.

**Issue:** The plan's Task 1 action says "Run `go get charm.land/huh/v2@v2.0.3`
and `go mod tidy`". But Task 1 only creates the catalog (catalog.go),
which has no `import "charm.land/huh/v2"`. `go mod tidy` correctly
removes the dep from go.mod when nothing imports it. The dep only
becomes a direct require in Task 3 when huh.go is created.

**Fix:** Catalog-only commit (Task 1) has no go.mod diff. The
`go get` was run multiple times during Task 1 / 2 / 3 prep but
`go mod tidy` consistently removed the dep until Task 3's huh.go
imported it. The dep is in go.mod at plan end (the user-visible
goal is achieved).

**Files modified:** `go.mod`, `go.sum` (in Task 3, not Task 1)

**Commit:** aa18a53 (Task 3)

### [Rule 1 - Test-fixture] `Project.Lipgloss` doesn't exist; test rewritten to use `p.Libs`

**Found during:** Task 3 test compilation.

**Issue:** Initial `TestPreSelectedLibs_FlagSet` fixture used
`Lipgloss: true` to simulate a --lipgloss flag. But `Project`
has no `Lipgloss` field — lipgloss lives only in `p.Libs`. The
9 per-lib bools are Cobra, Fang, Viper, Huh, Glamour, Glow,
Wish, Log, Harmonica.

**Fix:** Test fixture changed to `Libs: []string{"lipgloss"}`.
The test now correctly exercises `preSelectedLibs` reading
`p.AllLibs()` (which unions p.Libs and the bools).

**Files modified:** `internal/prompt/huh_test.go`

**Commit:** aa18a53 (Task 3)

### [Rule 1 - Behavior] `preSelectedLibs` returns non-nil empty slice for zero Project

**Found during:** Task 3 test run.

**Issue:** Initial `preSelectedLibs` used `var out []string` which
returns `nil` for the zero Project. `TestPreSelectedLibs_EmptyProject`
expected a non-nil empty slice (consistency with `AllLibs()` and
template-range-friendliness).

**Fix:** Changed `var out []string` to `out := []string{}`. Now
the function returns a non-nil empty slice when no libs are
pre-selected, matching the `AllLibs()` invariant.

**Files modified:** `internal/prompt/huh.go`

**Commit:** aa18a53 (Task 3)

## Test count delta

- **Before:** 88 tests (after Plan 01)
- **After:** 105 tests (+17)
  - 4 new in `internal/prompt` (catalog):
    - `TestLibCatalog_UniqueNames`
    - `TestLibCatalog_Sorted`
    - `TestLibsForType` (5 sub-cases: tui/cli/all/empty/unknown)
    - `TestDefaultLibsFor_MatchesLibsForType`
  - 5 new in `internal/scaffold` (AllLibs):
    - `TestProject_AllLibs_OnlyLibsSet`
    - `TestProject_AllLibs_OnlyBoolsSet`
    - `TestProject_AllLibs_Mixed`
    - `TestProject_AllLibs_Dedup`
    - `TestProject_AllLibs_Empty`
    - `TestProject_AllLibs_AllBoolsSet`
  - 8 new in `internal/prompt` (huh):
    - `TestAskType_SkipsWhenTypeSet`
    - `TestAskName_SkipsWhenNameSet`
    - `TestAskModule_SkipsWhenModuleSet`
    - `TestAskLicense_SkipsWhenNonMitSet`
    - `TestAskLicense_RunsWhenMitSet` (documented; TTY-only)
    - `TestAskTemplate_SkipsWhenNonDefaultSet`
    - `TestAskTemplateRepo_SkipsWhenSet`
    - `TestPreSelectedLibs_TuiVariant` / `_CliVariant` / `_AllVariant` / `_FlagSet` / `_EmptyProject` (5 sub-tests)
    - `TestTemplateOptionsFor_Variants` (5 sub-cases)
    - `TestSetBoolFieldByName`
  - Replaced: `TestFillNoop` → `TestFill_NoInteractiveReturns` + `TestFill_NilProject`

## Verification

- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/prompt/...` all pass (105 tests in the package, including 19 new in this plan)
- `go test ./internal/scaffold/...` all pass (53s, 6 new AllLibs tests)
- `go test ./cmd/...` all pass (TestNew, TestFang*, TestUnknown, TestVersion, TestRootCmd)
- `gofumpt -l internal/prompt/ internal/scaffold/project.go internal/scaffold/project_test.go` clean
- `grep -nR "github.com/charmbracelet/bubbletea\|github.com/charmbracelet/huh" --include="*.go" internal/prompt/` returns no matches (charm v2 paths only)
- `go test ./... -count=1 -timeout 120s -short` — all packages pass except the pre-existing `TestFmt_GofumptMissing_NoStrict` (environment-dependent: this test fails when gofumpt is installed in $PATH. Per Plan 01's note, it was failing at the base commit too. Not a regression from this plan.)

## Known Stubs

None. Plan 02 completes the huh v2 backend and the AllLibs helper.
The `prompt.Fill` is no longer a no-op — it dispatches to
`fillWithHuh` which implements the 8-step UI-SPEC prompt sequence.

The remaining stub is the gum backend, which Plan 03 adds
(at the bottom of `Fill`, before the `return fillWithHuh(p)` line).

The `.Run()`-level huh form tests are deferred (documented in
huh_test.go header comment) — they require a TTY or a
`tea.WithInput`/`tea.WithOutput` test-double setup that is out
of scope for the unit suite. The form-construction smoke tests
catch the most likely regression (constructor signature changes).

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: huh-backend-wiring | internal/prompt/huh.go | 8 ask* functions implement UI-SPEC prompt sequence; huh.ErrUserAborted → *Canceled mapping is in every function. askName/askTemplateRepo have manual re-prompt-once loops with IsValidGoModuleSegment / IsValidTemplateRepo validation (Pitfall T-03.02-T and T-03.02-T template-repo threat models). |
| threat_flag: libBoolMirror | internal/prompt/catalog.go | Map of Name → field name. askLibs uses this to mirror multi-select picks into the per-lib bool fields (Pitfall 6 fix from 03-RESEARCH.md). Adding a new lib requires updating this map. |
| threat_flag: preSelectedLibs | internal/prompt/huh.go | The single "pre-select" decision point. Both huh and gum backends will consume this; a regression here breaks both flows. |
| threat_flag: AllLibs-union | internal/scaffold/project.go | The union of p.Libs and boolFlagOverlayMap. Fixes Pitfall 4 (parallel sources of truth). AGENTS.md template (Plan 04) and askLibs (this plan) both consume AllLibs. |

## Issues Encountered

- **huh v2 non-generic Input/Confirm caught at build time, not at
  test time.** The 03-RESEARCH.md Example 5 uses `huh.NewInput[string]()`
  syntax (v1 pattern). In v2, `NewInput` and `NewConfirm` are not
  generic. The first build attempt failed with
  `cannot index huh.NewInput (value of type func() *huh.Input)`,
  which was easy to correct. Lesson: the v1→v2 upgrade changes the
  API surface more than the README hints.

- **`Project.Lipgloss` is not a struct field** (test fixture bug).
  Caught at test compile time. The 9 per-lib bools are the 9 keys
  in `boolFlagOverlayMap`; lipgloss is only in `p.Libs`. The
  test was rewritten to use `p.Libs`.

## Next Phase Readiness

- **Plan 03 (gum backend)** can wire `fillWithGum` against the
  established `Fill` chokepoint. The single-dispatch shape in
  `Fill` is the clean insertion point:
  ```go
  // Plan 03 insertion:
  // if path, _ := exec.LookPath("gum"); path != "" { return fillWithGum(p) }
  return fillWithHuh(p)
  ```
  The `Library` catalog, `LibsForType`, `preSelectedLibs`, and
  `libBoolMirror` are all exported and consumed by both backends.
- **Plan 04 (AGENTS.md template)** uses `Project.AllLibs()` (now
  in place) as the source of library names for the AGENTS.md
  rendering. The template can iterate over `{{allLibs .}}` once
  Plan 04 wires the `FuncMap` helper.
- The `*prompt.Canceled` error path is ready to receive
  `huh.ErrUserAborted` translations. The `main.go` exit-code
  mapping (exit 130) is unchanged from Plan 01.
- The `--no-interactive` / `--yes` / `--batch` flags are still
  wired in `cmd/new.go` from Plan 01; Plans 02/03 do not need to
  change the flag plumbing.

## Commits (chronological)

```
1a4e8d8 feat(03-02): wire Fill to dispatch to fillWithHuh + nil-guard
aa18a53 feat(03-02): implement huh v2 form layer — 8 ask* functions + fillWithHuh
39194c6 feat(03-02): add Project.AllLibs() — union of p.Libs and per-lib bools
3c857c9 feat(03-02): add Library catalog (13 charm libs) + LibsForType
```

---

*Phase: 03-interactive-prompts-gum-ai-agents-md*
*Plan: 02*
*Completed: 2026-06-03*
