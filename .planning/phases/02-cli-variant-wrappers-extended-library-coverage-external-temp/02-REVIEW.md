---
status: issues_found
files_reviewed: 60
depth: standard
findings:
  critical: 5
  warning: 10
  info: 6
  total: 21
---

# Phase 2 Code Review

## CRITICAL (5)

### CR-01: `lib/wish/wish.go.tmpl` will not compile — `tea` package not imported
**File:** `internal/scaffold/templates/lib/wish/wish.go.tmpl`
**Severity:** critical
**Category:** generated-code
Import block pulls in `charm.land/wish/v2/bubbletea` (subpackage) but body uses `tea.Model`, `[]tea.ProgramOption`, `tea.NewProgram`, `tea.QuitMsg`, `tea.KeyMsg`. None defined in scope. Generated project fails `go build` on first try.
**Fix:** add `"charm.land/bubbletea/v2"` to the import block (use distinct alias `wbt "charm.land/wish/v2/bubbletea"` for the subpackage). Also gate the wish subcommand on `{{if hasWish .}}` in main.go.

### CR-02: `variant_cli/main.go.tmpl` unconditionally imports cobra + fang
**File:** `internal/scaffold/templates/variant_cli/main.go.tmpl`
**Severity:** critical
**Category:** generated-code
Imports always pull in cobra + fang, but `go.mod.tmpl` only requires them when `hasCobra`/`hasFang`. `--cli` without `--cobra` produces unbuildable project.
**Fix:** wrap each import in `{{- if hasCobra .}}…{{- end}}` / `{{- if hasFang .}}…{{- end}}` guards, AND in `ResolveFlags` auto-set `Cobra=true` + `Fang=true` when `p.Type == "cli"`.

### CR-03: `variant_all/main.go.tmpl` unconditionally imports bubbletea + cobra + fang
**File:** `internal/scaffold/templates/variant_all/main.go.tmpl`
**Severity:** critical
**Category:** generated-code
Same as CR-02 but for combined TUI+CLI variant. `--all` without explicit `--bubbletea`/`--cobra`/`--fang` → unresolved imports.
**Fix:** guard each import on its `has*` helper, AND auto-set Bubbletea/Cobra/Fang=true in `ResolveFlags` when `p.Type == "all"`.

### CR-04: `CloneTemplateRepo` invokes `git clone` without `--` separator
**File:** `internal/scaffold/repo.go`
**Severity:** critical
**Category:** security
Defense-in-depth gap. If `url` begins with `-` (e.g. `-upload-pack=evil`), git interprets it as a flag. Validator checks scheme prefixes but not leading-dash URLs.
**Fix:** change to `git clone --depth 1 -- <url> <tmp>`. Also reject URLs whose host/path starts with `-` in `IsValidTemplateRepo`.

### CR-05: `CloneTemplateRepo` uses `context.Background()` with no timeout
**File:** `internal/scaffold/repo.go`
**Severity:** critical
**Category:** bug
A slow/dead remote freezes the scaffolder indefinitely.
**Fix:** wrap with `context.WithTimeout` (e.g. 60s), pass to `exec.CommandContext`. Surface clear error on timeout.

## WARNING (10)

### WR-01: `goVersionLessThanWithVersion` uses lexicographic `<`
**File:** `internal/wrap/test.go:73-76`
**Severity:** warning
**Category:** bug
`"1.9" < "1.10"` is `false` lexically. Use `golang.org/x/mod/semver.Compare`.

### WR-02: `err.Error() == "EOF"` string compare
**File:** `internal/wrap/detect_test.go`
**Severity:** warning
**Category:** test
Use `errors.Is(err, io.EOF)`.

### WR-03: `--cli`/`--all` don't auto-set Cobra/Fang
**File:** `internal/scaffold/resolve.go:139`
**Severity:** warning
**Category:** bug
TUI auto-defaults Bubbletea; CLI/all don't auto-default Cobra/Fang. Add parallel block for cli + all variants.

### WR-04: `--cli`/`--all` missing from `cmd/new.go` help text
**File:** `cmd/new.go`
**Severity:** warning
**Category:** quality
Add help block describing variant matrix (tui / cli / all / library).

### WR-05: `chdirTo` helper uses `os.Chdir` + Cleanup
**File:** `internal/wrap/integration_test.go:86-96`
**Severity:** warning
**Category:** test
Replace with `t.Chdir(projectDir)` (Go 1.24+); delete helper.

### WR-06: `var _ = time.Second` keeps "time" import alive
**File:** `internal/wrap/integration_test.go:284`
**Severity:** warning
**Category:** test
Drop the import + the var.

### WR-07: `_ = errors.Is` hack in `repo_test.go`
**File:** `internal/scaffold/repo_test.go`
**Severity:** warning
**Category:** test
Remove the hack line; remove the import.

### WR-08: `indexOf` reimplements `strings.Index`
**File:** `internal/wrap/run_test.go`
**Severity:** warning
**Category:** test
Delete `indexOf`, import `strings`, use `strings.Index`.

### WR-09: `VerifyBuild` `go test` has no timeout
**File:** `internal/scaffold/hooks.go`
**Severity:** warning
**Category:** bug
Scaffolder hangs on infinite-loop test. Add `ctx` parameter to `runCmd` with 2-min default.

### WR-10: `--template-repo ""` passes through silently
**File:** `internal/scaffold/resolve.go`
**Severity:** warning
**Category:** bug
Empty string then breaks clone with cryptic error. Guard at ResolveFlags boundary.

## INFO (6)

### IN-01: Dead function `logExample()` in `lib/log/log.go.tmpl`
**File:** `internal/scaffold/templates/lib/log/log.go.tmpl`
**Severity:** info
**Category:** generated-code
Call from `main()` as a demo, or remove.

### IN-02: Dead function `Load()` in `lib/viper/internal/config/config.go.tmpl`
**File:** `internal/scaffold/templates/lib/viper/internal/config/config.go.tmpl`
**Severity:** info
**Category:** generated-code
Wire into cobra's `PersistentPreRunE`, or remove.

### IN-03: Redundant 2nd-arg registrations in `template.FuncMap`
**File:** `internal/scaffold/template.go`
**Severity:** info
**Category:** code-quality
Audit funcMap literal; remove duplicate `has*` registrations.

### IN-04: `cmd/new.go` `defer os.RemoveAll(dir)` discards error
**File:** `cmd/new.go`
**Severity:** info
**Category:** code-quality
Acceptable; add a comment noting the deliberate ignore.

### IN-05: Unused `errMsg` type in `detect_test.go`
**File:** `internal/wrap/detect_test.go`
**Severity:** info
**Category:** test
Remove the type.

### IN-06: `_base/go.mod.tmpl` forces 1.25.0 on --cli-only projects
**File:** `internal/scaffold/templates/_base/go.mod.tmpl`
**Severity:** info
**Category:** quality
Make conditional: `{{- if hasBubbles .}}go 1.25.0{{- else}}go 1.23{{- end}}`. (But 02-01 explicitly killed this branch; restoring it is a 02-01 design reversal — defer.)

## Cluster analysis

WR-03 + CR-02 + CR-03 share one root cause: Phase 2 templates assume the user passed `--cobra`/`--fang` explicitly, but the variant flags should auto-default them (matching the Phase 1 `--tui` → `--bubbletea` pattern). One PR that adds the auto-default block to `ResolveFlags` plus the matching `{{if hasX}}` guards in the templates fixes the entire cluster.

WR-06/07 + IN-05 are test-suite cleanup — Phase 3 should sweep these as part of the test-refactor pass.
