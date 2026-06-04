---
phase: 03
plan: 03
title: Gum shell-out backend + backend resolver + dispatch
subsystem: prompt
tags: [gum, shell-out, backend-resolver, cancel-propagation, test-seams]
completed: 2026-06-04
duration: ~15m
tasks: 3 commits
files_created:
  - internal/prompt/gum.go
  - internal/prompt/gum_test.go
  - internal/prompt/prompt_backend_test.go
files_modified:
  - internal/prompt/prompt.go
requirements:
  - INT-01
key_findings:
  - "gum's `choose` widget writes the chosen option's display text to stdout (not a separate value), so the backend must reverse-map the label back to the machine key. Two maps were added: typeDisplayToKey (project type) and an inline displayToKey map per askGumTemplate call (variant-specific)."
  - "The widget wrapper signatures in the plan (gumChoose(header, options, defaultIdx)) do not take a ctx parameter, but gumRunCapture does (so the gumRunner var type matches it). The cleanest plumbing is a package-level gumCtx var that fillWithGum sets at entry (5-min timeout) and resets on exit — the wrappers read it and pass to gumRunner. This keeps the wrapper surface clean while honoring the 5-min timeout contract from the plan."
  - "resolveBackend is unexported; the existing prompt_test.go is `package prompt_test` (external). Added a new prompt_backend_test.go in `package prompt` (white-box) for the unexported symbol access. The 6 resolveBackend / backend tests are the load-bearing coverage for the dispatch logic."
  - "gum's `choose --no-limit` does not support pre-selection via the CLI; the user always sees the full list. askGumLibs accepts a preSelected []string param (for future pre-fill-via-stdin) but does not currently use it for arg construction. This is a known divergence from the huh backend (which DOES pre-select via .Selected(true)) and is documented in 03-RESEARCH.md as a future-enhancement hook."
  - "TestFillWithGum_CancelPropagates cannot assert `err == want` because the askGumXxx wrapper replaces the reason with a step-specific one (\"user canceled at project type selection\" etc.). The test now asserts the returned error matches `*Canceled` via errors.As, with a non-empty Reason — the same `*Canceled` instance contract that main.go uses to exit 130."
  - "gum's `Template` choice cannot distinguish a value from a label via stdout (it only writes the chosen option's text). The gum backend builds an inline displayToKey map per askGumTemplate call, rather than adding a (display,key) tuple type to a shared helper. This is local to one function and the inline map is the smallest change that matches the existing pattern in huh.go (which has a similar inline Option building approach)."
one_line_summary: "Gum shell-out backend (4 widget wrappers + fillWithGum dispatcher with the 8-step prompt sequence), resolveBackend() that prefers gum when found + healthy, and dispatch from Fill — the primary charmbracelet-style interactive prompt backend with huh v2 as the transparent fallback"
---

# Phase 3 Plan 3: Gum shell-out backend + backend resolver + dispatch

This plan adds the primary charmbracelet-style interactive prompt
backend for `spin new`: when `gum` is on $PATH and a 2-second
`gum --version` sanity check exits 0, `Fill` dispatches to
`fillWithGum`; otherwise the Plan 02 huh v2 backend takes over
transparently. The backend choice is locked for the duration of the
call (no mid-flow switch), and the chosen backend is logged at
Debug level per UI-SPEC §"gum vs huh decision".

The plan introduces two new test seams (gumLookPath, gumVersionCheck
in prompt.go; gumRunner in gum.go) that let the dispatch and
arg-construction logic be unit-tested in any environment — no real
gum binary, no real subprocess calls, no TTY required.

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-04T01:05:34Z
- **Completed:** 2026-06-04T01:20:05Z
- **Tasks:** 3
- **Files modified:** 1
- **Files created:** 3
- **Commits:** 3

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement gum.go — subprocess runner + 4 widget wrappers + fillWithGum dispatcher | 8037dc9 | internal/prompt/gum.go |
| 2 | Add resolveBackend() and wire Fill() to dispatch to gum-or-huh | 982d2d6 | internal/prompt/{prompt,prompt_backend_test}.go |
| 3 | Tests for gum arg construction (mock runner) | 42bd3bd | internal/prompt/gum.go, internal/prompt/gum_test.go |

## Accomplishments

- **gumRunCapture** (gum.go): the single place in the package that
  calls `os/exec` for gum. Wires `cmd.Cancel = Process.Kill` and
  `cmd.WaitDelay = 100ms` (same pattern as the git-clone path in
  `internal/scaffold/repo.go`) so Ctrl-C / 5-min timeout cannot leave
  the scaffolder hanging. On `ctx.Err() != nil`, returns
  `*Canceled{Reason: "gum canceled"}` so the main boundary (main.go)
  can exit 130. On other errors, returns `fmt.Errorf("gum %s: %w: %s",
  args[0], err, stderr)` with the stderr text included.
- **4 widget wrappers** (gum.go):
  - `gumChoose(header, options, defaultIdx)` → `choose --header H --selected N a b c`
    (1-based `--selected` per gum convention; the wrapper translates
    the 0-based Go defaultIdx to 1-based).
  - `gumMultiSelect(header, options, preSelected)` → `choose --no-limit --header H a b c`.
    Returns nil on empty stdout (user confirmed no selection).
    `preSelected` is accepted but currently unused — gum's
    `choose --no-limit` does not support pre-selection via the CLI.
  - `gumInput(header, placeholder, defaultValue)` → `input --header H --placeholder P [--value V]`
    (the `--value` flag is conditional on non-empty default).
  - `gumConfirm(prompt, defaultYes)` → `confirm --default=<bool> <prompt>`.
    Returns `out == "Yes"`.
- **fillWithGum dispatcher** (gum.go): mirrors `fillWithHuh` with
  8 step functions (`askGumType`, `askGumName`, `askGumModule`,
  `askGumLibs`, `askGumLicense`, `askGumTemplate`, `askGumTemplateRepo`,
  `askGumAI`). Sets a 5-minute context timeout at entry (resets on
  exit) via a package-level `gumCtx` var; the wrappers read it and
  pass to `gumRunner`. Write-back to `*scaffold.Project` matches
  the huh backend: Type, Name, Module, License, Template,
  TemplateRepo are conditional (skipped when set to a non-default
  value per UI-SPEC); Libs and AI always fire.
- **Backend type + resolver** (prompt.go): `backend` int enum
  (backendNone, backendGum, backendHuh), `backend.String()` for the
  Debug log line, and `resolveBackend()` with the resolution order:
  `SPIN_USE_HUH=1` → backendHuh; `gum` on $PATH AND `gum --version`
  exits 0 within 2s → backendGum; otherwise → backendHuh. A broken
  gum install (corrupt binary, missing shared lib) falls through
  to backendHuh per RESEARCH §Pitfall 3.
- **Test seams** (prompt.go, gum.go): `gumLookPath` and
  `gumVersionCheck` (in prompt.go) are package-level vars that
  default to the real `exec.LookPath` and a 2-second
  `exec.CommandContext(path, "--version").Run()`. `gumRunner` (in
  gum.go) is a package-level var that defaults to `gumRunCapture`.
  All three are stubbed per-test via `t.Cleanup` in the test files.
- **Fill dispatch** (prompt.go): replaces the Plan 02 always-huh
  body with `be := resolveBackend(); logBackend(be.String());
  switch be { ... }`. The Debug log line is `"prompt backend"
  backend=<name>` per UI-SPEC §"gum vs huh decision".
- **27 new tests** (16 in gum_test.go, 6 in prompt_backend_test.go, 5
  sub-cases in the table-driven GumConfirm_Args test). See
  "Test count delta" below for the breakdown.

## Files Created/Modified

### Created
- `internal/prompt/gum.go` — gumRunCapture, 4 widget wrappers,
  fillWithGum dispatcher, 8 askGum* step functions, typeDisplayToKey
  map, templateOptionsForType, isCanceled helper, logBackend
- `internal/prompt/gum_test.go` — TestGum{Choose,MultiSelect,Input,Confirm}_Args
  (arg construction, default plumbing, cancel propagation);
  TestFillWithGum_{WritesBackToProject, SkipsSetFields, CancelPropagates}
  (load-bearing write-back test + skip predicate + cancel);
  TestTemplateOptionsForType_Variants, TestTypeDisplayToKey_AllLabels,
  TestIsCanceled_AffectsOnlyCanceledErrors
- `internal/prompt/prompt_backend_test.go` — TestResolveBackend_{HuhWhenGumMissing,
  HuhWhenSPINUseHuh1, GumWhenAvailableAndHealthy, HuhWhenGumBroken};
  TestBackendString; TestFill_DispatchHiresolvesHuhWhenGumMissing

### Modified
- `internal/prompt/prompt.go` — added `backend` enum + `String()`;
  added `gumLookPath` and `gumVersionCheck` test seams; added
  `resolveBackend()`; replaced the Fill body with a backend dispatch
  + Debug log line; updated the package doc to reflect Plan 03
  status.

## Decisions Made

1. **Package-level `gumCtx` for ctx plumbing** — the plan's widget
   signatures (`gumChoose(header, options, defaultIdx)`) don't take
   a ctx parameter, but `gumRunCapture` does (so `gumRunner`'s type
   matches). A package-level `gumCtx` var — set by `fillWithGum`
   to a 5-min timeout and reset on exit — is the cleanest way to
   plumb the caller's context to the subprocess call site. The
   wrappers read `gumCtx` and pass it to `gumRunner`. This matches
   the existing pattern in huh.go (no ctx in wrappers) while
   honoring the 5-min timeout contract from the plan.

2. **Test seams `gumLookPath` and `gumVersionCheck`** — added as
   package-level vars in prompt.go. Default to the real
   `exec.LookPath` and a 2-second `gum --version` probe. Tests
   override them per-test via `t.Cleanup` to simulate gum
   present/absent/healthy/broken. The plan's verification section
   requires `os/exec.Command` to be called at exactly 2 sites
   (gumRunCapture + resolveBackend); the seams are var assignments
   to the real `exec.LookPath` / a closure that calls the real
   `exec.CommandContext` — so the count is unchanged.

3. **`typeDisplayToKey` map** for project-type label → machine
   key. The plan shows the gum labels as user-facing copy
   ("TUI — terminal app with bubbletea", etc.) and the test
   `TestFillWithGum_WritesBackToProject` stubs the runner to
   return these labels. The wrapper reverse-maps via a static
   map. A regression in the labels (typo, missing entry) would
   silently default to "" in `askGumType` and surface as a
   confusing "ask type: unexpected answer" error; the
   `TestTypeDisplayToKey_AllLabels` test pins the map to the
   UI-SPEC labels.

4. **Inline `displayToKey` map in `askGumTemplate`** — same shape
   as `typeDisplayToKey` but for the template options. Inline
   rather than a package-level var because the options are
   variant-specific and only one variant is asked per call. The
   test `TestTemplateOptionsForType_Variants` pins the option
   keys to UI-SPEC.

5. **`askGumLibs` returns picks as display labels, then maps to
   machine names** — the huh backend's `askLibs` returns the
   machine names directly (the huh form has a separate Value).
   The gum backend has no separate Value, so the labels are
   captured and mapped via `displayToName` (built once per call
   from `LibCatalog`). The write-back to `p.Libs` and the
   per-lib bools is identical to the huh backend (sort, dedup,
   mirror to bools via `libBoolMirror`).

6. **`preSelected []string` parameter unused in `gumMultiSelect`**
   — gum's `choose --no-limit` does not support pre-selection
   via the CLI; the user always sees the full list. The parameter
   is kept in the signature for symmetry with `huh.NewMultiSelect`
   and so a future plan can pre-fill via stdin (one line in
   gumMultiSelect: `cmd.Stdin = strings.NewReader(strings.Join(preSelected, "\n"))`)
   without churning the wrapper surface. Documented in
   `gumMultiSelect` doc comment.

7. **`TestFillWithGum_CancelPropagates` asserts matchable *Canceled,
   not same-instance** — the `askGumXxx` wrappers replace the
   `*Canceled.Reason` with a step-specific one ("user canceled at
   project type selection", etc.). The same `*Canceled` instance
   is not preserved. The test asserts the returned error matches
   `*Canceled` via `errors.As`, with a non-empty Reason — the same
   contract that main.go uses to exit 130.

8. **`SPIN_USE_HUH=1` escape hatch is the FIRST check in
   `resolveBackend`**, before any PATH lookup. This lets users
   force huh even if gum is installed (useful for debugging, or
   when the user doesn't want the gum TUI). Documented in
   the install hint per the plan.

## Deviations from Plan

### [Rule 1 - Test bug] TestFillWithGum_CancelPropagates expected wrong error identity

**Found during:** Task 3 first test run.

**Issue:** Initial test asserted `err == want` (same *Canceled
instance). The `askGumXxx` wrappers replace the `*Canceled.Reason`
with a step-specific one before returning, so the same-instance
invariant does not hold. The test also referenced a non-existent
`errorsAs` symbol (typo for `errors.As`).

**Fix:** Test now asserts the returned error matches `*Canceled`
via `errors.As`, with a non-empty Reason. The wrapping behavior
is by design (each step has a specific cancel reason for the log
line) and is the contract that main.go depends on.

**Files modified:** `internal/prompt/gum_test.go`

**Commit:** 42bd3bd (Task 3)

### [Rule 1 - Code bug] askGumLibs was missing sort.Strings(p.Libs)

**Found during:** Task 3 first test run.

**Issue:** Initial `askGumLibs` set `p.Libs = names` without
sorting. The huh backend's `askLibs` calls `sort.Strings(p.Libs)`
after the multi-select write-back. The gum backend's
`TestFillWithGum_WritesBackToProject` expects a sorted result
(`[bubbles, bubbletea]`, not `[bubbletea, bubbles]`).

**Fix:** Added `sort.Strings(p.Libs)` in `askGumLibs` after the
write-back. The sort is part of the public contract — the
AGENTS.md template and the overlay walker both assume sorted
`p.Libs`.

**Files modified:** `internal/prompt/gum.go`

**Commit:** 42bd3bd (Task 3)

### [Rule 1 - Test bug] TestIsCanceled had inverted assertion

**Found during:** Task 3 first test run.

**Issue:** Initial test had `if isCanceled(...) { t.Error("...
= false, want true") }` — the branch fires when isCanceled
returns true, but the error message describes the false case.
The test passed when isCanceled returned false (the wrong direction)
and failed when isCanceled returned true (the correct direction).

**Fix:** Removed the `!` operator on `isCanceled(...)` in the
*Canceled test case. The other two cases (nil, plain error) were
correctly written and unchanged.

**Files modified:** `internal/prompt/gum_test.go`

**Commit:** 42bd3bd (Task 3)

### [Rule 1 - Test fixture] TestFillWithGum_SkipsSetFields pre-set License="mit"

**Found during:** Task 3 first test run.

**Issue:** Initial fixture pre-set `License: "mit"` (the default)
expecting askGumLicense to skip. But the skip predicate is
`p.License != "" && p.License != "mit"` — License="mit" does NOT
skip (the user must be able to confirm the default per UI-SPEC).
The stub returned "" for every call, which got lowercased to ""
and assigned to `p.License`.

**Fix:** Fixture changed to `License: "apache-2.0"` (non-default),
which satisfies the skip predicate. The test now correctly asserts
that only Libs + AI fire (2 calls) when every other field is
pre-set to a non-default value.

**Files modified:** `internal/prompt/gum_test.go`

**Commit:** 42bd3bd (Task 3)

### [Plan adjustment] TestFillWithGum_SkipsSetFields pre-set Template="cli-cobra-fang"

**Found during:** Task 3 first test run.

**Issue:** Same as above — Template was pre-set to "tui-bubbletea"
(the default), so askGumTemplate's skip predicate
`p.Template != "" && p.Template != "tui-bubbletea"` did not fire.
The stub returned "" which didn't match any template display
label, so the wrapper returned "ask template: unexpected answer".

**Fix:** Fixture changed to `Template: "cli-cobra-fang"` (non-default).
Same fix pattern as the License case.

**Files modified:** `internal/prompt/gum_test.go`

**Commit:** 42bd3bd (Task 3)

## Test count delta

- **Before:** 105 tests (after Plan 02)
- **After:** 132 tests (+27)
  - 16 new in `internal/prompt` (gum):
    - `TestGumChoose_Args`
    - `TestGumChoose_DefaultIndex`
    - `TestGumMultiSelect_Args`
    - `TestGumMultiSelect_EmptyReturnsNil`
    - `TestGumInput_Args` (2 sub-cases: with default / no default)
    - `TestGumConfirm_Args` (3 sub-cases: default-true/yes, default-false/no, default-true/no)
    - `TestGumConfirm_CanceledPropagation`
    - `TestFillWithGum_WritesBackToProject`
    - `TestFillWithGum_SkipsSetFields`
    - `TestFillWithGum_CancelPropagates`
    - `TestTemplateOptionsForType_Variants` (5 sub-cases: tui/cli/all/empty/unknown)
    - `TestTypeDisplayToKey_AllLabels`
    - `TestIsCanceled_AffectsOnlyCanceledErrors`
  - 6 new in `internal/prompt` (backend resolver):
    - `TestResolveBackend_HuhWhenGumMissing`
    - `TestResolveBackend_HuhWhenSPINUseHuh1`
    - `TestResolveBackend_GumWhenAvailableAndHealthy`
    - `TestResolveBackend_HuhWhenGumBroken`
    - `TestBackendString`
    - `TestFill_DispatchHiresolvesHuhWhenGumMissing`

## Verification

- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test -count=1 -timeout 30s ./internal/prompt/...` all pass (44 tests, +27 from this plan)
- `go test -count=1 -timeout 300s ./internal/scaffold/... ./internal/prompt/... ./cmd/...` all pass
- `gofumpt -l internal/prompt/` clean
- `grep -nR "exec\." internal/prompt/` shows exactly 2 `os/exec` call sites:
  - `gum.go:71` — `exec.CommandContext(ctx, "gum", args...)` (gumRunCapture)
  - `prompt.go:103` — `exec.CommandContext(verCtx, path, "--version").Run()` (gumVersionCheck, the resolveBackend probe)
  - All other subprocess usage goes through `gumRunner` (test seam)
- `gum` not on $PATH in the test runner → all 4 stubbed-gum tests
  pass; resolveBackend correctly returns backendHuh in the
  default test env.

## Known Stubs

None for this plan. The `fillWithGum` dispatcher is fully
implemented; `resolveBackend` is fully implemented; the test seams
are intentional design, not stubs (they have real default values
that the production path uses).

The remaining stub-like behavior is `gumMultiSelect`'s unused
`preSelected []string` parameter (documented above as a
forward-compatibility hook for stdin pre-fill). This is a deliberate
API choice, not a missing implementation.

The `gum` binary itself is not a Go module — it's a runtime
requirement installed via
`go install github.com/charmbracelet/gum@latest`. The test
runner doesn't have gum installed; the suite passes because all
tests stub `gumRunner` (or `gumLookPath` + `gumVersionCheck`)
and never call the real subprocess.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: gum-subprocess-args | internal/prompt/gum.go | `gumRunCapture` is the single os/exec call site for gum. `cmd.Cancel = Process.Kill` + `cmd.WaitDelay = 100ms` enforce clean Ctrl-C / timeout (mirrors the git-clone pattern in internal/scaffold/repo.go). T-03.03-D from the plan's threat_model. |
| threat_flag: gum-version-sanity-check | internal/prompt/prompt.go | `resolveBackend` runs `gum --version` with a 2-second timeout before adopting backendGum. A broken install (corrupt binary, missing shared lib) returns non-zero and falls through to backendHuh. T-03.03-S from the plan's threat_model. |
| threat_flag: gum-stdout-trim | internal/prompt/gum.go | `gumRunCapture` does `strings.TrimRight(stdout, "\n")` to strip the trailing newline gum emits. Multi-select splits on "\n" per gum docs. T-03.03-T from the plan's threat_model. |
| threat_flag: ctx-err-cancel-mapping | internal/prompt/gum.go | `gumRunCapture` checks `ctx.Err() != nil` on subprocess error and returns `*Canceled{Reason: "gum canceled"}` so the main boundary (main.go) can exit 130. T-03.03-R from the plan's threat_model. |
| threat_flag: 5-min-ctx | internal/prompt/gum.go | `fillWithGum` wraps the run in `context.WithTimeout(context.Background(), 5*time.Minute)` via the `gumCtx` package var. Wrapper signatures stay clean (no extra ctx param per the plan's widget choice rule) while the timeout is enforced. |
| threat_flag: test-seam-overridable | internal/prompt/gum.go, internal/prompt/prompt.go | `gumRunner`, `gumLookPath`, `gumVersionCheck` are package-level vars that tests override per-test. The defaults are the real subprocess / PATH / version-probe calls. A regression in the seams (e.g., a test forgetting to restore via t.Cleanup) could leak state across tests; the test files use `t.Cleanup` consistently. |

## Issues Encountered

- **`gumRunner` ctx plumbing.** The plan's widget signatures don't
  include ctx, but `gumRunCapture` does. Tried three designs:
  (1) wrappers take ctx, tests pass `context.Background()` —
  breaks the plan's literal test signatures; (2) wrappers use
  `context.Background()` and the 5-min timeout lives in
  `gumRunCapture` — loses the "caller provides ctx" intent from
  the plan; (3) package-level `gumCtx` set by `fillWithGum` —
  matches the plan's literal signatures AND honors the 5-min
  timeout from the caller. Chose (3). The package var is a small
  stateful construct but it stays within the Fill-call scope
  (saved/restored via deferred func).

- **`gum`'s `choose` writes only the display text to stdout, not a
  separate value.** The huh form's Option has a `Value(*T)` for
  the machine key; gum has no such separation. The wrapper has to
  reverse-map the display label to the machine key. Two reverse
  maps were added: `typeDisplayToKey` (3 entries, package-level)
  and an inline `displayToKey` map per `askGumTemplate` call
  (variant-specific, 1-2 entries). Both are pinned by tests
  (`TestTypeDisplayToKey_AllLabels`, `TestTemplateOptionsForType_Variants`).

- **Test fixture bugs from assuming the skip predicate fires on
  default values.** The skip predicates in the gum backend match
  the huh backend exactly: `License="mit"` does NOT skip (the user
  must confirm the default per UI-SPEC); `Template="tui-bubbletea"`
  does NOT skip. The first test run had both pre-set to default
  values, which caused 2 unexpected gumRunner calls. Fixed by
  pre-setting both to non-default values in the test fixture.

- **`*Canceled` wrapping at the wrapper level** — the
  `askGumXxx` wrappers replace the `*Canceled.Reason` with a
  step-specific one before returning. This is the same behavior
  as the huh backend (each ask function maps `huh.ErrUserAborted`
  to `*Canceled{Reason: "user canceled at <step>"}`) and is the
  contract that main.go depends on. The
  `TestFillWithGum_CancelPropagates` test was updated to assert
  matchable *Canceled, not same-instance.

## Next Phase Readiness

- **Plan 04 (AGENTS.md template)** can iterate over
  `p.AllLibs()` (in place from Plan 02) to render the library
  sections. The gum/huh prompt answers now flow into the same
  `*scaffold.Project` that the template consumes — no separate
  `*PromptAnswers` struct. The FuncMap helper for the library
  lookup table is the only remaining piece.
- The `*prompt.Canceled` error path is now active on both
  backends (gum + huh); main.go's existing `errors.As` to exit 130
  works for both.
- The `prompt.Fill` chokepoint is now wired for both backends; no
  further plan needs to change the dispatch logic. The
  `SPIN_USE_HUH=1` env var is the documented escape hatch for
  users who want to force the in-process backend.
- The `--no-interactive` / `--yes` / `--batch` flags are still
  wired in `cmd/new.go` from Plan 01; Plan 03 does not change
  the flag plumbing.
- Future enhancements (out of scope for this plan):
  - Pre-fill the multi-select via stdin
    (`cmd.Stdin = strings.NewReader(strings.Join(preSelected, "\n"))`)
    so the user sees the variant defaults highlighted.
  - Move the 5-min timeout into the runner itself (e.g.,
    `gumRunCapture` accepts a `timeout time.Duration` param) so
    the wrappers don't need the `gumCtx` package var.

## Commits (chronological)

```
8037dc9 feat(03-03): implement gum shell-out backend
982d2d6 feat(03-03): add resolveBackend() and wire Fill to dispatch to gum-or-huh
42bd3bd test(03-03): gum arg construction + fillWithGum writeback + cancel tests
```

---

*Phase: 03-interactive-prompts-gum-ai-agents-md*
*Plan: 03*
*Completed: 2026-06-04*
