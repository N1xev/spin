# Research Summary -- spin

**Project:** spin -- Go project scaffold CLI for the charmbracelet v2 ecosystem
**Researched:** 2026-06-02
**Confidence:** HIGH

## Executive Summary

`spin` sits in a well-defined product category (language/ecosystem scaffolders) where category leaders (`cargo new`, `npm init`, `cobra-cli init`, `spring initializr`) do the same job: take a name + flags, emit a directory tree, write a manifest, exit cleanly. The opportunity for `spin` is **vertical integration with the charmbracelet v2 stack** -- the scaffolder itself uses fang + gum + lipgloss + huh so the tool *demonstrates* the experience it scaffolds. The differentiator is per-library flags (mirroring Spring Initializr's "Starters"), embedded offline templates, and the dogfooded charm aesthetic.

All `charmbracelet` v1 import paths are deprecated and forbidden in generated code. v2 vanity domain `charm.land/<lib>/v2` is the only acceptable path. `gum` is a binary (no Go library) -- shell out via `os/exec`. `charm.land/bubbles/v2` requires Go 1.25.0+; `spin` itself pins `go 1.23`, generated projects pin `go 1.25.0` if they pull bubbles, `go 1.23` otherwise.

The single highest-priority risk is **v1 charmbracelet API leakage into generated projects**. Defense: post-scaffold `go build ./...` smoke test + CI grep suite for every forbidden v1 symbol (`View() string`, `tea.KeyMsg`, `lipgloss.NewRenderer`, `github.com/charmbracelet/...` imports, `tea.WithAltScreen`).

## Key Findings

### Stack (scaffolder itself)
- `cobra v1.9.1` + `charm.land/fang/v2` -- subcommands + styled help
- `charm.land/lipgloss/v2` + `charm.land/log/v2` -- styled output + logging
- `github.com/charmbracelet/gum` (binary, subprocess) -- interactive prompts
- `charm.land/huh/v2` (in-process) -- fallback when gum missing
- `github.com/spf13/viper v1.20.x` (opt-in) -- config support
- `go:embed` + `text/template` -- zero-dep template engine with base+variant+lib overlays
- `go 1.23` floor (own binary)

### Stack (scaffolded projects)
- `charm.land/bubbletea/v2` -- TUI framework
- `charm.land/bubbles/v2` -- components (Go 1.25.0+)
- `charm.land/lipgloss/v2` -- styling
- `charm.land/huh/v2` -- forms
- `charm.land/glamour/v2` -- markdown rendering
- `charm.land/wish/v2` -- SSH server
- `charm.land/log/v2` -- logging
- `github.com/charmbracelet/glow` (binary) -- markdown reader
- Dev tools: `air` (hot reload), `prism` (test runner, Go 1.24+), `gofumpt` + `goimports` (format), `goreleaser v2` (dist)
- `go 1.25.0` floor when bubbles included, `go 1.23` otherwise

### Features

**Table stakes (P1):** `spin new <name>` with name validation, `--tui`/`--cli`/`--all` umbrellas, per-library subflags (`--bubbletea`, `--bubbles`, `--lipgloss`, `--huh`, `--glamour`, `--glow`, `--wish`, `--log`, `--crush`, `--modifiers`, `--ansi`, `--runewidth`, `--harmonica`), `--cobra`/`--fang` default on, `--viper` opt-in, `--template`/`--template-repo`, `--ai`/`--agents` for AGENTS.md, wrappers `spin run`/`build`/`test`/`vet`/`fmt`, interactive gum prompts with `--no-interactive` escape, `.air.toml`+`Taskfile.yml`+working example shipped, `CGO_ENABLED=0` guarantee, fang-styled help.

**Differentiators (P2):** `spin doctor` health check, `spin add <lib>` post-scaffold injection, `--update` non-conflicting re-apply, VHS demo tape.

**Anti-features (v1):** no template marketplace, no plugin system, no scaffolder TUI mode, no CI/Dockerfile generation, no multi-language, no auto-update of generated projects.

### Architecture

Four layers:
1. **CLI Layer** -- `cmd/{root,new,run,build,test,vet,fmt}.go` (cobra+fang subcommands)
2. **Flag & State** -- single `Project` struct resolved from flags + gum prompts (single contract)
3. **Template Engine** -- `internal/scaffold` uses `go:embed` FS; walks `templates/_base/` + `variant_<type>/` + `lib/<name>/` overlays, last-write-wins
4. **Filesystem Sink** -- emits `./<name>/`, runs `git init`, post-scaffold `go build` smoke test

`Prompter` interface behind `internal/interactive` keeps gum swappable. `internal/wrappers` has one file per wrapped command with `exec.LookPath` fallback chain.

### Critical Pitfalls

1. **v1 import paths leak into generated code** -- `charm.land/<lib>/v2` only; CI grep for `github.com/charmbracelet/`
2. **v2 message API changes** -- `KeyPressMsg`/`MouseClickMsg` typed (not `tea.KeyMsg` struct); `View() tea.View` (not `string`); `tea.NewView` + `view.AltScreen`/`view.MouseMode` fields (not `tea.WithAltScreen`)
3. **Lipgloss v2** -- `lipgloss.NewRenderer`/`DefaultRenderer`/`AdaptiveColor` removed; `HasDarkBackground(in, out)`; `Color` is a function returning `image/color.Color`
4. **`go:embed` + `text/template` collisions** -- forward-slash globs even on Windows; unique basenames; `Funcs` before `Parse`; `ExecuteTemplate` not `Execute`; set `missingkey=error` in dev
5. **`gum`/`bubbletea` non-TTY hang** -- `isatty.IsTerminal(os.Stdin)` guard + `--no-interactive` escape hatch from day one
6. **`air` config drift** -- `build.entrypoint` (not deprecated `build.bin`); set `include_ext` and `exclude_dir`
7. **CGO leakage** -- CI matrix builds for 5 OS/arch combos with `CGO_ENABLED=0`
8. **`gofumpt` silent fallback to `gofmt`** -- fail loudly with install instructions; opt-out `--no-strict`
9. **`AGENTS.md` has no versioned spec** -- include `<!-- AUTOGENERATED by spin X.Y.Z -->` marker
10. **Generated projects can't be auto-update** -- mark with `// generated by spin X.Y.Z` headers, never break "guaranteed to compile"

## Confidence

| Area | Confidence |
|------|------------|
| Stack | HIGH |
| Features | HIGH |
| Architecture | HIGH |
| Pitfalls | MEDIUM-HIGH (LOW on gum/tea non-TTY specifics) |

## Open Decisions for Requirements Phase

- Go version floor policy: `spin` = 1.23, generated with bubbles = 1.25.0, generated without = 1.23
- `--all` scope: curated subset vs literally every lib
- Template naming: match umbrella-flag form (`tui-bubbletea`, `cli-cobra-fang`)
- AGENTS.md format: spin invents own convention with version marker (no de facto schema)
- `~/.spin.yaml` config: defer unless flag fatigue is real

## Implications for Roadmap

4 phases suggested:

1. **Phase 1: Scaffolder Foundation + Charm v2 Templates** -- embed+overlay engine, TUI variant, post-scaffold smoke test, v1 grep CI, self-template
2. **Phase 2: CLI Variant + Wrappers + Per-Library Coverage** -- `--cli` flag, all wrappers with fallback chains
3. **Phase 3: Interactive Layer (gum) + AI/AGENTS.md + External Templates** -- `Prompter` interface, `--ai` flag, `--template-repo` override, bundled variants
4. **Phase 4: Verification, Dogfooding, Polish** -- full CI grep suite, 5-OS/arch CGO=0 matrix, GoReleaser v2, `spin doctor`, VHS demo
