---
phase: 03-interactive-prompts-gum-ai-agents-md
reviewed: 2026-06-04T00:00:00Z
depth: standard
files_reviewed: 57
files_reviewed_list:
  - cmd/new.go
  - go.mod
  - internal/prompt/catalog.go
  - internal/prompt/catalog_test.go
  - internal/prompt/detect.go
  - internal/prompt/detect_test.go
  - internal/prompt/gum.go
  - internal/prompt/gum_test.go
  - internal/prompt/huh.go
  - internal/prompt/huh_test.go
  - internal/prompt/prompt_backend_test.go
  - internal/prompt/prompt.go
  - internal/prompt/prompt_test.go
  - internal/scaffold/agents_test.go
  - internal/scaffold/grep_test.go
  - internal/scaffold/integration_test.go
  - internal/scaffold/project.go
  - internal/scaffold/project_test.go
  - internal/scaffold/resolve.go
  - internal/scaffold/resolve_test.go
  - internal/scaffold/scaffold_e2e_test.go
  - internal/scaffold/scaffold_test.go
  - internal/scaffold/template.go
  - internal/scaffold/templates/_base/cmd/_name_/main.go.tmpl
  - internal/scaffold/templates/_base/README.md.tmpl
  - internal/scaffold/templates/lib/ai/AGENTS.md.tmpl
  - internal/scaffold/templates/variant_all/cmd/_name_/main.go.tmpl
  - internal/scaffold/templates/variant_all/internal/app/app.go.tmpl
  - internal/scaffold/templates/variant_all/internal/app/keys.go.tmpl
  - internal/scaffold/templates/variant_all/internal/app/update.go.tmpl
  - internal/scaffold/templates/variant_all/internal/app/view.go.tmpl
  - internal/scaffold/templates/variant_all/internal/cmd/hello.go.tmpl
  - internal/scaffold/templates/variant_all/internal/cmd/root.go.tmpl
  - internal/scaffold/templates/variant_all/internal/cmd/ssh.go.tmpl
  - internal/scaffold/templates/variant_all/internal/cmd/tui.go.tmpl
  - internal/scaffold/templates/variant_all/internal/config/config.go.tmpl
  - internal/scaffold/templates/variant_all/internal/ui/styles.go.tmpl
  - internal/scaffold/templates/variant_cli/cmd/_name_/main.go.tmpl
  - internal/scaffold/templates/variant_cli/internal/cmd/hello.go.tmpl
  - internal/scaffold/templates/variant_cli/internal/cmd/root.go.tmpl
  - internal/scaffold/templates/variant_cli/internal/cmd/ssh.go.tmpl
  - internal/scaffold/templates/variant_cli/internal/config/config.go.tmpl
  - internal/scaffold/templates/variant_cli/internal/ui/styles.go.tmpl
  - internal/scaffold/templates/variant_tui/cmd/_name_/main.go.tmpl
  - internal/scaffold/templates/variant_tui/internal/app/app.go.tmpl
  - internal/scaffold/templates/variant_tui/internal/app/keys.go.tmpl
  - internal/scaffold/templates/variant_tui/internal/app/update.go.tmpl
  - internal/scaffold/templates/variant_tui/internal/app/view.go.tmpl
  - internal/scaffold/templates/variant_tui/internal/ui/styles.go.tmpl
  - internal/scaffold/template_test.go
  - internal/scaffold/versions.go
  - main.go
findings:
  critical: 1
  warning: 6
  info: 6
  total: 13
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-06-04T00:00:00Z
**Depth:** standard
**Files Reviewed:** 57
**Status:** issues_found

## Summary

This phase wires the interactive prompt layer (gum shell-out + huh v2 in-process) into the `spin new` command, completes the variant template set (tui/cli/all), and adds the `AGENTS.md` opt-in flow.

The core chokepoint (`prompt.Fill`), the backend resolution (`resolveBackend`), the cancel-error mapping (`*prompt.Canceled` → exit 130), and the per-lib bool mirror in `askLibs` are all implemented with consistent contracts. The template engine correctly composes `_base` → `variant_<type>` → `lib/ai/` (the only surviving per-lib overlay), handles path-level `_name_` substitution, and gates LICENSE files by name.

The most material findings are in the concurrency/testability story (mutable package-level seams prevent `t.Parallel()`), a UX error in the template-repo re-prompt that reports an empty URL, and the auto-default block in `resolve.go` that silently overrides explicit user negation. Several quality items (redundant test assertions, dead branches, inconsistent FuncMap signatures) are also flagged.

## Critical Issues

### CR-01: Package-level mutable test seams in prompt package prevent parallel tests and create race-condition footgun

**Files:** `internal/prompt/prompt.go:93, 100`, `internal/prompt/gum.go:47, 58`
**Issue:** The prompt package exposes four package-level mutable variables as test seams:
- `gumLookPath` (prompt.go:93) -- reassigned per-test in `prompt_backend_test.go`
- `gumVersionCheck` (prompt.go:100) -- reassigned per-test
- `gumRunner` (gum.go:47) -- reassigned per-test in `gum_test.go`
- `gumCtx` (gum.go:58) -- swapped by `fillWithGum` at entry and restored via `defer`

The `gumCtx` swap is especially subtle: `fillWithGum` mutates the global during a 5-minute window. If two tests in the same package invoke `fillWithGum` concurrently, one test's `prevCtx` save/restore would clobber the other's. All current tests in `gum_test.go` and `prompt_backend_test.go` are safe because none of them use `t.Parallel()`, but the global pattern makes that contract implicit. A future maintainer adding `t.Parallel()` to a `fillWithGum`-driven test would introduce a data race that would not surface until CI parallelism increased.

**Fix:** Move the seams onto a struct (e.g. `type promptDeps struct { lookPath func(string)(string,error); versionCheck func(string) error; runner func(context.Context, ...string) (string, error) }`) and have the prompt functions take a `*promptDeps` receiver or read it from a per-`Fill` context. For `gumCtx`, store the context on the local call stack only -- there is no need for a package-level mutable var since the wrappers (`gumChoose`, `gumInput`, etc.) already capture it via closure when they call `gumRunner(gumCtx, args...)`. Convert the wrappers to take a `ctx context.Context` parameter and pass it through, eliminating the global entirely.

## Warnings

### WR-01: `askGumTemplateRepo` error reports empty `p.TemplateRepo` instead of the invalid input

**File:** `internal/prompt/gum.go:430`
**Issue:** The re-prompt loop only writes to `p.TemplateRepo` on a successful validation:
```go
for attempt := 1; attempt <= 2; attempt++ {
    r, err := gumInput(...)
    ...
    repo := strings.TrimSpace(r)
    if repo == "" {
        return nil
    }
    if scaffold.IsValidTemplateRepo(repo) {
        p.TemplateRepo = repo
        return nil
    }
}
return fmt.Errorf("spin: invalid template repo URL %q", p.TemplateRepo)
```
When both attempts fail, `p.TemplateRepo` is still its initial empty value (or whatever the user passed via `--template-repo`, which would be valid and would have returned at the skip check). The error message therefore reads `spin: invalid template repo URL ""` rather than the URL the user actually typed. The same pattern is mirrored in `huh.go`'s `askTemplateRepo` at line 396.

**Fix:** Track the last invalid input in a local variable (e.g. `var last string`) and use it in the error: `return fmt.Errorf("spin: invalid template repo URL %q", last)`. Apply the same fix to the huh backend.

### WR-02: Path-traversal check has a redundant second condition that is always true

**File:** `internal/scaffold/scaffold.go:122-127`
**Issue:** The `emit()` path-traversal guard:
```go
cleanFull := filepath.Clean(full)
if !strings.HasPrefix(cleanFull+string(filepath.Separator), cleanRoot) &&
    cleanFull+string(filepath.Separator) != cleanRoot {
```
Both `cleanFull+"/"` and `cleanRoot` carry the separator suffix (`cleanRoot` is built as `filepath.Clean(root) + string(filepath.Separator)`). For any candidate `cleanFull`, the second clause `cleanFull+"/" != cleanRoot` is always true. The first clause alone correctly catches the project-root case (`cleanFull == cleanRoot` → `cleanFull+"/"` is `cleanRoot+"/"` which does not have the prefix). The dead second clause is a code smell that obscures the intent.

**Fix:** Drop the second clause:
```go
if !strings.HasPrefix(cleanFull+string(filepath.Separator), cleanRoot) {
    return fmt.Errorf("path traversal: ...")
}
```

### WR-03: Auto-default block silently overrides explicit `--cobra=false` / `--fang=false`

**File:** `internal/scaffold/resolve.go:243-256`
**Issue:** The variant auto-default block runs unconditionally after the bool-flag binding loop:
```go
if p.Type == "cli" || p.Type == "all" {
    p.Cobra = true
    p.Fang = true
}
if p.Type == "all" {
    if !containsString(p.Libs, "bubbletea") {
        p.Libs = append(p.Libs, "bubbletea")
        ...
    }
}
```
The comment at line 240-242 acknowledges this overrides the bound (false) values, but the user's intent is silently dropped. A user who runs `spin new foo --cli --cobra=false` (an unusual but valid invocation) gets a project with `p.Cobra = true`. The same applies to `--all` adding `bubbletea` to `p.Libs` even if the user passed `--bubbletea=false` (pflag's negation syntax).

**Fix:** Use `cmd.Flags().Changed("cobra")` to detect explicit user input and skip the auto-default when the user has set the flag. The same approach used at line 91-95 for `--template-repo` would apply here. Document the change in a comment so future maintainers know why the bool-flag binding loop's value is preserved on explicit user override.

### WR-04: `askGumLibs` computes `pre` then immediately discards it

**File:** `internal/prompt/gum.go:317-319`
**Issue:**
```go
pre := preSelectedLibs(p) // documented; not directly used by gum here
_ = pre
picks, err := gumMultiSelect("Pick libraries", options, pre)
```
The `pre` is passed to `gumMultiSelect` as the third argument, but the wrapper at line 124-135 does not consume `preSelected` when constructing the gum CLI args (gum's `choose --no-limit` has no pre-selection flag). The function comment at line 124-128 says the parameter is kept "for symmetry with huh.NewMultiSelect", but the symmetry is illusory: the wrapper never uses it, and the same value could be passed as `nil` without changing behavior. The `_ = pre` line and the `pre := preSelectedLibs(p)` allocation are dead code that future readers will wonder about.

**Fix:** Drop the `pre` argument entirely. `gumMultiSelect` should take `(header string, options []string)`. Update the caller to pass only the two used arguments.

### WR-05: `TestAskLicense_RunsWhenMitSet` is a placeholder that doesn't test what its name promises

**File:** `internal/prompt/huh_test.go:82-93`
**Issue:** The test:
```go
options := []string{"mit", "apache-2.0", "none"}
if len(options) != 3 {
    t.Errorf("askLicense options = %d, want 3", len(options))
}
t.Logf("askLicense with License=mit would show the form; TTY-only behavior, documented")
```
The test does not call `askLicense`, does not exercise the form path, and asserts nothing about the function under test. The local `options` slice is hardcoded and unrelated to the actual `options` variable in `huh.go:282-286`. A reader expects "RunsWhenMitSet" to validate the form's behavior when `p.License == "mit"`; instead it is a no-op assertion of a constant.

**Fix:** Either implement the test (e.g. by injecting a huh input/output seam) or rename to `TestAskLicense_OptionsCount` and assert against the actual `options` slice built inside `askLicense`. Better still, extract the `options` builder into a small `askLicenseOptions() []huh.Option[string]` helper that is testable without a TTY.

### WR-06: `huh.NewSelect` skip-when-non-mit pre-select loop mutates the slice during range

**File:** `internal/prompt/huh.go:289-293`
**Issue:**
```go
for i, opt := range options {
    if opt.Value == p.License {
        options[i] = opt.Selected(true)
    }
}
```
This works in Go (range copies the value, and `options[i] = ...` writes back to the slice), but the pattern is unusual and easy to get wrong if `huh.Option` ever becomes a non-value type. Future maintainers reading this may be tempted to add `append(options, ...)` to the loop, which would corrupt iteration. The idiomatic form is `for i := range options { ... }`.

**Fix:** Replace with `for i := range options { if options[i].Value == p.License { options[i] = options[i].Selected(true) } }`. Same behavior, clearer intent.

## Info

### IN-01: `gumRunCapture` accesses `args[0]` without a length check

**File:** `internal/prompt/gum.go:96`
**Issue:** `fmt.Errorf("gum %s: %w: %s", args[0], err, ...)` will panic with an index-out-of-range error if a caller ever passes zero args. All current wrappers pass at least the subcommand, but the function is package-internal and could be called from new code. Adding `if len(args) == 0 { return "", errors.New("gum: no subcommand") }` is one line of defense.

**Fix:** Add a defensive length check at the top of `gumRunCapture`.

### IN-02: FuncMap helpers inconsistently use closure-captured `p` vs parameter `p2`

**File:** `internal/scaffold/template.go:269-286`
**Issue:** The `funcMap` mixes two styles:
- `has` uses closure-captured `p`: `func(v string) bool { return slices.Contains(p.Libs, v) }` and is called as `{{has "bubbletea"}}` (no dot).
- `hasBubbles` etc. take a `*Project` parameter: `func(p2 *Project) bool { return slices.Contains(p2.Libs, "bubbles") }` and are called as `{{hasBubbles .}}`.

In practice `p == p2` so both work, but the templates have to know which style each helper uses. A unified convention (parameter form, called as `{{hasBubbles .}}` everywhere) would be cleaner.

**Fix:** Pick one style and apply it to all helpers. The parameter form is more testable.

### IN-03: `stubFillWithGum` has unused `_ = i` line

**File:** `internal/prompt/gum_test.go:257`
**Issue:** The line `_ = i` after the closure that mutates `i` is dead code. The `i` variable is captured by the closure (incremented on each call) but is never read outside the closure. The `_ = i` was probably copy-pasted from a test that needed to read `i` after the closure ran.

**Fix:** Remove the `_ = i` line.

### IN-04: Two TTY-detection tests unconditionally `t.Skip` on real terminals

**File:** `internal/prompt/detect_test.go:31-41, 100-114`
**Issue:** `TestIsInteractive_TTYCheck` and `TestIsInteractive_AllLayersOff` both `t.Skip` when stdin is a tty, which is the common case on developer machines. The TTY-present path is therefore never exercised by the test suite. A future regression in the isatty check (e.g. always returning true) would not be caught.

**Fix:** Inject `isatty.IsTerminal` as a package-level seam (same pattern as `gumLookPath`) and assert both true and false branches in tests. Alternatively, refactor `IsInteractive` to take a `isTerminal func(uintptr) bool` parameter and pass a stub in tests.

### IN-05: `Fill`'s `backendNone` default case is documented as defensive but is genuinely unreachable

**File:** `internal/prompt/prompt.go:178-186`
**Issue:** The `default` branch returns `&Canceled{Reason: "no TTY available for prompts"}` for `backendNone`, but `resolveBackend()` only ever returns `backendHuh` or `backendGum` (lines 124-138). The case is reachable only if a future code change adds a third backend. Either remove the case (and let the type system enforce exhaustiveness on the `switch be`) or replace it with `panic("unreachable: backendNone")` so a future regression crashes loudly.

**Fix:** Either delete the default case (the switch becomes a true-enum enumeration over a `backend` value) or `panic` with a clear message. The current `&Canceled{...}` return hides the unreachable invariant.

### IN-06: Spin itself uses older charm versions than it scaffolds

**File:** `go.mod:16-17`, `internal/scaffold/versions.go:72-82`
**Issue:** `go.mod` requires `charm.land/bubbletea/v2 v2.0.2` (indirect) and `charm.land/bubbles/v2 v2.0.0` (indirect), but `DefaultPins` in `versions.go` emits `v2.0.7` and `v2.1.0` for the same libraries in the scaffolded `go.mod`. After `go mod tidy`, the scaffolded project is upgraded to the newer versions. This is correct behavior, but worth flagging that the spin binary itself tests against older versions of the charm v2 stack than it ships to users.

**Fix:** Bump the `// indirect` charm v2 deps in `go.mod` to the pinned versions in `versions.go`. This eliminates a subtle "we test on the wrong version" hazard and makes the spin binary's smoke test match the scaffolded user's experience.

---

_Reviewed: 2026-06-04T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
