---
phase: 01-scaffolder-foundation-core-tui-stack
reviewed: 2026-06-02T23:50:00Z
depth: standard
files_reviewed: 57
files_reviewed_list:
  - .gitignore
  - .golangci.yml
  - LICENSE
  - README.md
  - Taskfile.yml
  - cmd/help_test.go
  - cmd/new.go
  - cmd/root.go
  - go.mod
  - go.sum
  - internal/scaffold/git.go
  - internal/scaffold/grep_test.go
  - internal/scaffold/hooks.go
  - internal/scaffold/hooks_test.go
  - internal/scaffold/integration_test.go
  - internal/scaffold/project.go
  - internal/scaffold/resolve.go
  - internal/scaffold/resolve_test.go
  - internal/scaffold/scaffold.go
  - internal/scaffold/scaffold_e2e_test.go
  - internal/scaffold/scaffold_test.go
  - internal/scaffold/template.go
  - internal/scaffold/template_test.go
  - internal/scaffold/validate.go
  - internal/scaffold/validate_test.go
  - internal/scaffold/versions.go
  - internal/scaffold/templates/_base/.air.toml.tmpl
  - internal/scaffold/templates/_base/.gitignore.tmpl
  - internal/scaffold/templates/_base/LICENSE-Apache-2.0.tmpl
  - internal/scaffold/templates/_base/LICENSE-MIT.tmpl
  - internal/scaffold/templates/_base/README.md.tmpl
  - internal/scaffold/templates/_base/Taskfile.yml.tmpl
  - internal/scaffold/templates/_base/go.mod.tmpl
  - internal/scaffold/templates/_base/internal/ui/styles.go.tmpl
  - internal/scaffold/templates/_base/main.go.tmpl
  - internal/scaffold/templates/lib/ansi/README.md.tmpl
  - internal/scaffold/templates/lib/bubbles/bubbles.go.tmpl
  - internal/scaffold/templates/lib/bubbletea/bubbletea.go.tmpl
  - internal/scaffold/templates/lib/cobra/README.md.tmpl
  - internal/scaffold/templates/lib/fang/README.md.tmpl
  - internal/scaffold/templates/lib/glamour/README.md.tmpl
  - internal/scaffold/templates/lib/glow/README.md.tmpl
  - internal/scaffold/templates/lib/harmonica/README.md.tmpl
  - internal/scaffold/templates/lib/huh/README.md.tmpl
  - internal/scaffold/templates/lib/lipgloss/internal/ui/styles.go.tmpl
  - internal/scaffold/templates/lib/lipgloss/lipgloss.go.tmpl
  - internal/scaffold/templates/lib/log/README.md.tmpl
  - internal/scaffold/templates/lib/modifiers/README.md.tmpl
  - internal/scaffold/templates/lib/runewidth/README.md.tmpl
  - internal/scaffold/templates/lib/viper/README.md.tmpl
  - internal/scaffold/templates/lib/wish/README.md.tmpl
  - internal/scaffold/templates/variant_all/main.go.tmpl
  - internal/scaffold/templates/variant_cli/main.go.tmpl
  - internal/scaffold/templates/variant_tui/main.go.tmpl
  - internal/version/version.go
  - main.go
  - scripts/check-v1-leaks.sh
findings:
  critical: 2
  warning: 4
  info: 2
  total: 8
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-06-02
**Depth:** standard
**Files Reviewed:** 57
**Status:** issues_found

## Summary

Phase 1 ships a working scaffolder that produces a runnable charmbracelet v2 TUI project. The embed-driven overlay engine, lib gating, license gating, post-scaffold verify-build, and git init all behave correctly for the canonical `--tui --bubbletea [--bubbles] [--lipgloss]` combinations. The v1-leak grep suite is in place and the bubbletea/lipgloss/bubbles v2 API calls in the generated code are correct.

Two critical defects were found that block the "perfect first run" promise for the lib-only flag combinations, and four warnings / two info items were identified.

## Critical Issues

### CR-001: variant_tui/main.go.tmpl unconditionally imports bubbletea -- breaks `--tui --lipgloss` and `--tui` alone

**File:** `/home/samouly/Projects/Golang/loom/internal/scaffold/templates/variant_tui/main.go.tmpl:10`
**Issue:** The `charm.land/bubbletea/v2` import on line 10 is **not** gated by `{{- if hasBubbletea .}}`, but the corresponding entry in `templates/_base/go.mod.tmpl:6-8` is. The "no-bubbletea" branch was never tested (integration tests always pass `--bubbletea` explicitly). Any user running `spin new myapp --tui --lipgloss` (or `spin new myapp --tui` with no libs) gets a `main.go` that imports `charm.land/bubbletea/v2` while `go.mod` does not require it. The post-scaffold `go build ./...` smoke test in `VerifyBuild` then fails with "package charm.land/bubbletea/v2: cannot find package", surfacing to the user as a broken scaffold -- the exact regression TOOL-01 was supposed to prevent.

The two reachable failure modes:
- `spin new myapp --tui` (no lib flags) -- bubbles/lipgloss/bubbletea all absent from go.mod; bubbletea still imported.
- `spin new myapp --tui --lipgloss` (lipgloss without bubbletea) -- same problem.

This is asymmetric with the working combinations (`--tui --bubbletea`, `--tui --bubbletea --bubbles`, `--tui --bubbletea --lipgloss`) where the import and the require line up.

**Fix:** Gate the bubbletea import in `variant_tui/main.go.tmpl:10-17` on `{{- if hasBubbletea .}}`. Since `variant_tui` only runs when `p.Type == "tui"`, the walker is already correct -- only the import block needs to be made conditional. Concretely:

```gotemplate
import (
{{- if hasBubbletea .}}
    "fmt"
    "os"

    "charm.land/bubbletea/v2"
{{- else}}
    "fmt"
    "os"
{{- end}}
{{- if hasBubbles .}}
    "charm.land/bubbles/v2/spinner"
{{- end}}
{{- if or (hasLipgloss .) (hasBubbles .)}}
    "{{.Module}}/internal/ui"
{{- end}}
)
```

And gate the model / Update / View / main() body on the same predicate so a `--tui --lipgloss` scaffold produces a sensible non-TUI program. Alternatively, enforce `--tui implies --bubbletea` in `ResolveFlags` (the bubbles-implies-bubbletea invariant is already there; add a tui-implies-bubbletea one) so the import doesn't need to be conditional. The latter is closer to the spec language in CLAUDE.md ("`spin new myapp --tui --bubbletea` produces a project that `go run`s cleanly") and is the recommended fix.

A regression test should be added: `TestRenderToMap_TUI_Lipgloss_Only` that builds a `Project{Type: "tui", Libs: []string{"lipgloss"}, ...}` and asserts the go.mod `require` block and the main.go import block are mutually consistent (both contain bubbletea, or neither does).

### CR-002: `--license` value not validated -- unknown values silently emit no LICENSE

**File:** `/home/samouly/Projects/Golang/loom/internal/scaffold/resolve.go:44-48` and `/home/samouly/Projects/Golang/loom/internal/scaffold/template.go:94-99`
**Issue:** The `--license` flag is registered as a free-form string with default `"mit"` in `cmd/new.go:36` and `ResolveFlags` reads it without validation. The walker at `template.go:94-99` matches `LICENSE-<p.License>.tmpl` case-insensitively, and silently produces **no LICENSE** if the value doesn't match any template. There is no error path for `"gpl"`, `"bsd"`, `"MIT "` (typo with trailing space), `"unlicense"`, etc. -- the user gets a project with no LICENSE file and no diagnostic.

This violates the "perfect first run" promise: a user who mistypes `--license mt` (instead of `mit`) thinks they're getting MIT and gets nothing. There's no follow-up smoke test that would catch this (LICENSE is not compiled, and `go build ./...` doesn't read it).

**Fix:** Validate the license value in `ResolveFlags` (or as a new `IsValidLicense` helper in `validate.go` against the set `{mit, apache-2.0, none}`) and return a `FlagError` for unknown values. Add a regression test in `resolve_test.go` (and an integration-level assertion in `integration_test.go` that `--license gpl` exits non-zero).

## Warnings

### WR-001: Forward-compat lib/<name>/README.md.tmpl placeholders would clobber the base README.md in Phase 2

**File:** `/home/samouly/Projects/Golang/loom/internal/scaffold/templates/lib/cobra/README.md.tmpl` (and 11 sibling files for fang, viper, huh, glamour, glow, wish, log, harmonica, modifiers, ansi, runewidth)
**Issue:** Each placeholder file has the path `templates/lib/<name>/README.md.tmpl`. When the overlay walker renders it, the layer prefix `templates/lib/<name>/` is stripped and `.tmpl` is removed, yielding `outKey = "README.md"`. The walker then writes that key into the output map, overwriting the `_base/README.md.tmpl` output for the same key. Currently `ResolveFlags` does not add the forward-compat booleans (`Cobra`, `Fang`, ...) to `p.Libs`, so these layers are not walked. But the moment Phase 2 wires e.g. `--cobra` to append `"cobra"` to `p.Libs` (the natural extension of the `bubbles implies bubbletea` logic), the user's actual README gets silently replaced with a Go comment block. This is invisible in the current test suite because no Phase 2 flag is exercised end-to-end.

**Fix:** Two options:
1. Move placeholder content out of `README.md` to a non-clobbering path (e.g. `templates/lib/<name>/OVERLAY.md.tmpl` or `templates/lib/<name>/_lib_<name>.md.tmpl`) until Phase 2 has real content.
2. Make the placeholder content valid Markdown that augments the README (e.g. a "Libraries" subsection), so the overlay is additive rather than destructive.

Option 1 is safer for Phase 1 hand-off.

### WR-002: scaffold.go init() mutates global log level on import

**File:** `/home/samouly/Projects/Golang/loom/internal/scaffold/scaffold.go:31-33`
**Issue:** The package-level `init()` calls `log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{Level: log.InfoLevel}))`. This is a side effect of importing the `scaffold` package -- it overrides the default logger for the entire process. If `cmd/help_test.go` or any other test that depends on charm/log ever runs in the same process (e.g. via a future test that imports both), it inherits the Info level silently. The fix is to defer logger configuration to the `New` entry point (or to `main.go`) so package import has no side effects.

**Fix:** Move the `log.SetDefault` call out of `init()` and into `New()` at the top, or have `main.go` call a public `scaffold.InitLogger()` after `os.Args` parsing. This makes log configuration explicit and test-overridable.

### WR-003: Project.Validate() called twice in the runNew pipeline

**File:** `/home/samouly/Projects/Golang/loom/cmd/new.go:65-74` and `/home/samouly/Projects/Golang/loom/internal/scaffold/scaffold.go:47-49`
**Issue:** `runNew` calls `p.Validate()` before calling `scaffold.New(p)`, and `New` also calls `p.Validate()` at its entry. The double-call is idempotent but redundant, and risks subtle drift if the two Validate implementations diverge (one calls the canonical `IsValidGoModuleSegment` + `os.Stat`, the other might add a new check in the future). The contract should be "the caller validates, or the scaffolder validates -- pick one."

**Fix:** Either drop the `Validate()` call in `runNew` and rely on `New` to call it, or drop the call in `New` and document that callers must validate first. The `New`-side call is preferable because it makes the scaffolder harder to misuse from a non-CLI entry point (e.g. a future `spin update` subcommand).

### WR-004: isUnknownFlagErr in git.go is fragile (misses "unknown switch" wording)

**File:** `/home/samouly/Projects/Golang/loom/internal/scaffold/git.go:91-95`
**Issue:** The fallback for `git < 2.28` parses the error string with `strings.Contains(s, "unknown option") || strings.Contains(s, "unrecognized")`. Older git versions actually report "unknown switch `b'" (note: **switch**, not option). On those systems the function returns false, `GitInit` returns the raw error, and the user sees "git init failed" with no fallback to the working `git init` + `git symbolic-ref HEAD refs/heads/main` path. The integration test never exercises this branch (the test env has modern git).

**Fix:** Either probe the version with `git --version` and parse the major version (preferred), or broaden the matcher: `strings.Contains(s, "unknown") && strings.Contains(s, "switch"+" -"+shortFlag) || strings.Contains(s, "unknown") && strings.Contains(s, "option"+" -"+shortFlag) || strings.Contains(s, "unrecognized") && ...`. Add a unit test that synthesizes each git error string and asserts the matcher.

## Info

### IN-001: TestRootCmdVersionWiring hardcodes "0.1.0"

**File:** `/home/samouly/Projects/Golang/loom/cmd/help_test.go:131-133`
**Issue:** The test asserts `rc.Version == "0.1.0"` against `version.Version`. Every version bump will fail this test even though the wiring is correct. The test's stated purpose is to "catch regressions where someone replaces the wiring" -- but the assertion is too strict for that purpose. The actual regression vector is `rc.Version = "hardcoded literal"` (which would be caught by `rc.Version == ""` only when omitted; if someone sets it to a literal `"1.2.3"`, this test would not catch the regression but a future bump would).

**Fix:** Assert `rc.Version == version.Version` (the variable, not the literal). The constant `version.Version` is the single source of truth.

### IN-002: walkErr string match in template.go should use errors.Is

**File:** `/home/samouly/Projects/Golang/loom/internal/scaffold/template.go:74-82`
**Issue:** `if strings.Contains(walkErr.Error(), "file does not exist")` is a string match on the formatted error. The underlying error is `*fs.PathError` wrapping `fs.ErrNotExist`. The `errors.Is(walkErr, fs.ErrNotExist)` form is type-stable and unaffected by formatting changes in future Go releases.

**Fix:** Replace the string match with `errors.Is(walkErr, fs.ErrNotExist)` (importing `errors` and `io/fs` -- `io/fs` is already imported as `fs`).

---

_Reviewed: 2026-06-02_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
