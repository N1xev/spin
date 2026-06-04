---
phase: 02-cli-variant-wrappers-extended-library-coverage-external-temp
plan: 05
type: summary
status: complete
verified: 2026-06-04
---

# Plan 02-05 — Template Restructure SUMMARY

## Why

Mid-Phase 3, the user flagged a design defect: scaffolded apps shipped
one file per lib at the project root (`lib/huh/huh.go`,
`lib/wish/wish.go`, ...). That layout reads like a parts catalog, not a
real Go project. A user inspecting the output would not learn how the
libs compose; they would just see disconnected files.

> "the same file can have simple bubbletea or bubbletea with lipgloss or
> bubbletea with lipgloss with bubbles, and just like that! inspire from
> each lib example! charmbracelet have examples almost for each lib!"

Resolution chosen interactively (AskUserQuestion): **restructure now**
with **minimal working** content per lib (small, real working demo per
conditional section), modelled on charmbracelet's official example apps.

## What Changed

### Layout (scaffolded output)

```
<name>/
├── cmd/<name>/main.go      # thin entry: app.Run() or fang.Execute
├── internal/
│   ├── app/                # TUI variant (--tui / --all)
│   │   ├── app.go          # Model + New + Init + Run
│   │   ├── update.go       # Update — inlines huh/glamour/harmonica/spinner/log
│   │   ├── view.go         # View() returning tea.View
│   │   └── keys.go         # KeyMap
│   ├── cmd/                # CLI variant (--cli / --all)
│   │   ├── root.go         # cobra root, fang.Execute
│   │   ├── hello.go        # styled subcommand (--lipgloss)
│   │   ├── readme.go       # glamour render + glow shell-out
│   │   ├── ssh.go          # wish SSH on :2222
│   │   └── tui.go          # (--all only) launches the TUI
│   ├── ui/styles.go        # lipgloss styles (--lipgloss)
│   └── config/config.go    # viper wiring (--viper)
├── go.mod                  # go 1.25.0, charm.land/*/v2 pins
└── ...
```

### Template tree (`internal/scaffold/templates/`)

- **`_base/`** — shared scaffolding: go.mod, README, .air.toml,
  Taskfile.yml, LICENSE-*, .gitignore (unchanged shape; some content
  re-pinned)
- **`variant_tui/`**, **`variant_cli/`**, **`variant_all/`** — variant
  files with `{{ if has<Lib> . }}` conditional blocks inlining EVERY
  charm lib's wiring (no separate lib/<name>/<file>.go.tmpl)
- **`lib/glow/README.glow.md.tmpl`** — only surviving lib overlay
  (binary install hint for the glow markdown reader)

DELETED: `lib/{huh,wish,glamour,harmonica,bubbles,bubbletea,cobra,fang,log,lipgloss,viper,ansi,modifiers,runewidth}/`.

### Walker substitution

`templates/.../cmd/_name_/main.go.tmpl` → `cmd/<actual-name>/main.go`.
The `_name_` placeholder is replaced in the output PATH (not the
template body) so authors can address per-project paths without
templating the filesystem itself. `<name>` was rejected — angle
brackets are valid in filenames but harder to type in editors.

### Bool→Name map split (`project.go` + `template.go`)

The single bool→name map was split into two:

- **`boolFlagOverlayMap()`** in `template.go` — only entries with a
  surviving `lib/<name>/` overlay. Now `{"glow": p.Glow}`. Drives the
  overlay walker.
- **`libBoolMap()`** in `project.go` — full 9-entry map: cobra, fang,
  viper, huh, glamour, glow, wish, log, harmonica. Drives `AllLibs()`
  for prompts and (future) AGENTS.md.

This split fixed `TestProject_AllLibs_OnlyBoolsSet` which regressed
when the overlay map shrunk.

### `overlayOrder()` filter

Old behavior added `lib/<name>` to the layer list for every entry in
`p.Libs`, then the walker silently skipped missing directories. Plan
02-05 filters `p.Libs` through `boolFlagOverlayMap` keys before
appending, so the layer list reflects what actually exists. Fixed
`TestOverlayOrder_TUI`.

### v2 API drift fixes

- `variant_cli/internal/cmd/ssh.go.tmpl` + `variant_all/internal/cmd/ssh.go.tmpl`:
  removed the v1 `tea.WithAltScreen()` program option (returns `nil`
  options now — bubbletea v2 has no `WithAltScreen`)
- `variant_all/internal/ui/styles.go.tmpl`: merged TUI styles (Title,
  Status, Help) with CLI styles (Header, Success, Error) into one
  6-field struct gated on `{{ if hasLipgloss . }}`

## Tests Added

In `internal/scaffold/integration_test.go`:

1. **`TestIntegrationScaffold_TUIAllLibs`** — `--tui --bubbletea
   --bubbles --lipgloss --huh --glamour --harmonica --log`; asserts
   restructured tree exists, no per-lib files in `internal/app/`,
   `internal/app/update.go` inlines huh.NewForm, glamour.NewTermRenderer,
   harmonica.NewSpring, spinner.TickMsg, log.Info; `internal/app/app.go`
   has spinner.New; builds clean, zero v1 leaks
2. **`TestIntegrationScaffold_CLIAllLibs`** — `--cli --cobra --fang
   --lipgloss --glamour --wish --log --viper`; asserts restructured tree
   exists, builds, runs `hello world` + `readme` subcommands
   end-to-end with expected output
3. **`TestIntegrationScaffold_AllVariant`** — `--all` with full lib set;
   asserts both `internal/app/` + `internal/cmd/` exist, root `--help`
   lists tui, hello, readme, ssh subcommands, hello + readme execute
4. **`TestIntegrationScaffold_NameInPath`** — scaffolds `weird-name_123`;
   asserts `cmd/weird-name_123/main.go` exists and no scaffolded path
   contains the unsubstituted `_name_` placeholder

Updated `internal/scaffold/integration_test.go` `assertMainGoV2` and
`assertAppGoV2` helpers to validate the new thin-main + internal/app
split.

## Verification

- `go build ./...` — exit 0
- `go test ./internal/scaffold/... ./internal/prompt/... ./cmd/... -count=1`
  — all green (scaffold 69.2s, prompt 0.006s, cmd 6.4s)
- Manual smoke tests for each variant — all pass (see 02-VERIFICATION.md
  Addendum)
- Pre-existing flaky `wrap.TestRun_WithAirToml` + `wrap.TestFmt_GofumptMissing_NoStrict`
  still flake at base commit — NOT Plan 02-05 regressions

## Downstream Impact

- **Plan 03-04** (`/gsd:execute-phase 3` later): the `lib/ai/AGENTS.md.tmpl`
  overlay is the SECOND surviving lib overlay alongside `lib/glow/`.
  Task 2 in 03-04-PLAN.md updated to reflect this (the old comment
  "existing pattern: cobra, huh, ..." was stale). The AGENTS.md
  rendered content also updated to describe the new variant-with-inline
  template tree.

## Commits (worktree branch `worktree-agent-02-05`)

```
9184ef8 feat(02-05): walker substitutes _name_ -> p.Name in output path
91a8baa feat(02-05): TUI variant restructured layout (cmd/_name_ + internal/app + internal/ui)
163d1d4 feat(02-05): CLI variant restructured layout (cmd/_name_ + internal/cmd + internal/ui + internal/config)
8432139 feat(02-05): All variant restructured layout (combines TUI + CLI)
cbf9414 feat(02-05): drop obsolete lib/* overlays; keep only lib/glow
<this commit> docs(02-05): plan + summary; verification addendum; 03-04 update
<test commit> test(02-05): integration tests for restructured layout + walker substitution
<engine commit> refactor(02-05): split bool->name map (boolFlagOverlayMap vs libBoolMap); filter overlayOrder by surviving overlays
<v2 fix commit> fix(02-05): drop v1 tea.WithAltScreen from ssh.go.tmpl; merge all-variant styles struct
```
