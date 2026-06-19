<!-- GSD:project-start source:PROJECT.md -->
## Project

**spin**

`spin` is a language-agnostic scaffolder for external templates. One CLI turns any external template (a directory with a `spin.toml` manifest and a `_base/` tree of overlays) into a runnable project: Go, Rust, TypeScript, Python, anything. The template's language, framework, and build tool are entirely the author's choice; `spin` doesn't know or care. `spin new <name> --template <spec>` produces a project that builds, tests, and runs without extra setup.

**Core Value:** Generate a runnable project from any external template with one command. `spin new myapp --template go-cli && cd myapp && go run .` produces a project that builds, tests, and runs without extra setup -- regardless of language, framework, or build tool. The template author owns the details; `spin` owns the load / prompt / render / post-hook pipeline.

### Constraints

- **Scope**: language-agnostic. Templates can target any language or framework that has a `spin.toml` and a `_base/` tree. `spin` does not assume Go, charmbracelet, or any other ecosystem. -- Why: templates are the only extension surface
- **Distribution**: single static binary; install via `go install github.com/N1xev/spin@latest` -- Why: standard Go CLI distribution, no runtime deps
- **Templates**: external only. No embedded defaults; no `spin new go` vs `spin new rust` form. The user picks a template (local path, git URL, or pinned name) and spin renders it. -- Why: keeps the scaffolder small and the author in control
- **Two pipelines**: the template pipeline (`spin new`) and the registry pipeline (`spin add` / `spin list` / `spin update` / `spin remove` / `spin search`) share one Template type and one filesystem layout but are otherwise independent. -- Why: search and offline use are different concerns
- **Single static binary**: no plugins, no init scripts, no companion daemon. -- Why: install once, run anywhere
- **CGO off**: `CGO_ENABLED=0` is the default. Cross-compile and minimal container sizes. -- Why: standard Go CLI distribution
- **Spin's own stack**: built with cobra + fang + lipgloss + huh. This is an implementation choice for `spin` itself, not a constraint on what templates can produce. Templates may use any libraries. -- Why: charm v2 is the right choice for the scaffolder's UI; templates are free to pick their own stack
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Recommended Stack
### (a) Libraries `spin` injects into scaffolded projects
| Library | Module path | Version (v2 line) | Purpose | Why v2 |
|---------|-------------|-------------------|---------|--------|
| Bubble Tea | `charm.land/bubbletea/v2` | v2.0.0 (stable) | TUI framework, MVU runtime | v2 stable; v2 renames `View() string` → `View() tea.View`, moves AltScreen/MouseMode to view fields, uses typed `KeyPressMsg`/`MouseClickMsg` |
| Lip Gloss | `charm.land/lipgloss/v2` | v2.0.0-beta.2 (stable line) | Terminal styling/layout (CSS-like API) | v2 is the supported line; subpackages `table`, `tree`, `list` follow `charm.land/lipgloss/v2/<sub>` |
| Bubbles | `charm.land/bubbles/v2` | v2.0.0 | TUI components: spinner, textinput, viewport, list, table, paginator, progress, timer, textarea, help, key, cursor, stopwatch | v2 line; `runeutil` and `memoization` removed; requires Go 1.25.0+ |
| Huh | `charm.land/huh/v2` | v2.0.0 | Interactive forms/prompts (accessible) | v2 stable; integrates with `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` |
| Glamour | `charm.land/glamour/v2` | v2 line | Stylesheet-based markdown renderer for terminal | v2 stable; `glamour.Render()` and `NewTermRenderer` API unchanged in spirit |
| Glow | `github.com/charmbracelet/glow/v2` (binary) | v2 line | Markdown reader CLI -- install as binary, shell out via `gum`-style exec | Scaffolded projects shell out to `glow` for readme rendering |
| Wish | `charm.land/wish/v2` (+ subpackages `bubbletea`, `logging`, `activeterm`) | v2 line | SSH server framework with bubbletea middleware | v2 stable; subpackages follow `charm.land/wish/v2/<sub>` |
| Log | `charm.land/log/v2` | v2.0.0 | Minimal colorful leveled structured logging | v2 stable; `log.Default()`/`SetDefault()` + `Options{...}` |
| Crush | `github.com/charmbracelet/crush` | current | Terminal AI assistant -- scaffolded projects may include `crush` config | Provided as a binary; embed for the AI/AGENTS layer |
| charmbracelet/x | `github.com/charmbracelet/x` (single module, many subpackages) | current (experimental) | ANSI parser/generator, VT emulator, `pony` UI DSL, term utilities | Experimental; pin a specific tag in generated `go.mod` |
| go-runewidth | `github.com/mattn/go-runewidth` | current | East-Asian-aware display width | Transitive of charm stack; rarely direct dep |
### (b) Libraries `spin` itself uses
| Library | Module path | Version | Purpose | Why |
|---------|-------------|---------|---------|-----|
| Cobra | `github.com/spf13/cobra` | v1.9.1 (latest) | CLI subcommand/flag framework | De facto Go CLI standard; underpins kubectl, hugo, gh, docker |
| Fang | `charm.land/fang/v2` | v2 line | Styled help, errors, completions, manpages, version theming -- drop-in for cobra's default | Drop-in `fang.Execute(ctx, rootCmd)`; gives `spin --help` a charm-style look out of the box; requires cobra v1.9+ |
| Lip Gloss | `charm.land/lipgloss/v2` | v2 | Styling scaffolder output (success messages, "Created at ./foo", etc.) | Reuses the same lib we scaffold into projects -- dogfooding |
| Huh | `charm.land/huh/v2` | v2 | Optional in-process prompts (when gum not available) | Huh is a Go library; gum is a shell-out binary. Huh is the in-process fallback for TTY-required runs |
| Log | `charm.land/log/v2` | v2 | Scaffolder logging (`log.Info("created", "path", ...)`) | Same library we ship in projects; consistent look |
| Viper | `github.com/spf13/viper` | v1.20.x (opt-in) | Config-file support for scaffolder | Only wired when user passes `--viper`; do not import unconditionally (per spec) |
| charmbracelet/x ansi | `github.com/charmbracelet/x/ansi` | current | Lower-level ANSI sequence generation if we need it | Used only if lipgloss v2 doesn't cover a case |
### Supporting Development Tools
| Tool | Install | Purpose | Notes |
|------|---------|---------|-------|
| `air` (hot reload) | `go install github.com/air-verse/air@latest` (requires Go 1.25+) | Scaffolded projects get `.air.toml` | Generate a sensible default: `cmd = "go build -o ./tmp/main ."`, watch `.go`/`.tmpl`, exclude `bin/`, `tmp/`, `dist/` |
| `prism` (test runner) | `go install go.dalton.dog/prism@latest` (requires Go 1.24+) | `spin test` wraps prism; `prism` replaces `go test` with parallel colored output | Fallback to `go test` if `prism` not on PATH |
| `gofumpt` (formatter) | `go install mvdan.cc/gofumpt@latest` | Stricter gofmt; `spin fmt` runs gofumpt first | Falls back to `gofmt` if gofumpt not installed |
| `goimports` | `go install golang.org/x/tools/cmd/goimports@latest` | Adds missing imports + removes unused; `spin fmt` runs after gofumpt | Bundled with `golang.org/x/tools` |
| GoReleaser | `go install github.com/goreleaser/goreleaser/v2@latest` (requires Go 1.26+) | Cross-platform binary distribution | Scaffolded project includes `.goreleaser.yaml` with `version: 2`, `CGO_ENABLED=0`, `targets: ["go_first_class"]` |
| `golangci-lint` | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` | Linter; scaffolded project ships `.golangci.yml` | Optional; not required for `spin` itself |
### Runtime
| Runtime | Version | Why |
|---------|---------|-----|
| Go | **1.23** for `spin` itself; **1.25.0+** for scaffolded projects that import bubbles v2 | `spin` is a CLI tool that doesn't import bubbles, so 1.23 is fine. But scaffolded `--bubbles` projects need 1.25.0+ (per official bubbles v2 docs). See "Go version tension" below. |
## Installation (for `spin` itself)
# Core CLI stack
# Opt-in: Viper for --viper flag
# Optional dev tools installed by users (NOT by spin):
#   go install github.com/air-verse/air@latest
#   go install go.dalton.dog/prism@latest
#   go install mvdan.cc/gofumpt@latest
#   go install golang.org/x/tools/cmd/goimports@latest
#   go install github.com/goreleaser/goreleaser/v2@latest
## Alternatives Considered
| Layer | Recommended | Alternative | When to use the alternative |
|-------|-------------|-------------|-----------------------------|
| CLI framework | cobra + fang | urfave/cli | Never for this project -- spec explicitly excludes urfave/cli; fang is the charmbracelet polish layer |
| TUI framework (generated projects) | bubbletea v2 | tview, ratatui | Never for this project -- spec explicitly excludes both; spin is opinionated about charm |
| TUI components | bubbles v2 | hand-rolled | Use hand-rolled only for components bubbles doesn't ship (rare) |
| Interactive prompts (scaffolder) | gum (subprocess) + huh v2 (in-process) | survey, promptui | Never -- spec is charm-only |
| Config (scaffolder) | viper (opt-in) | envconfig, koanf | Use koanf only if a user asks for a viper replacement; default off |
| Hot reload | air | wgo, realize | Never for scaffolded projects -- spec mandates air |
| Test runner | prism | gotestsum, richgo | Use gotestsum only if prism install fails; spec mandates prism |
| Formatter | gofumpt + goimports | gofmt only | gofmt only as last-resort fallback when gofumpt not installed |
| Distribution | goreleaser v2 | manual `go build` | Never -- spec mandates goreleaser |
| Logging (generated) | charmbracelet/log v2 | zap, slog, zerolog | slog is acceptable for non-charm projects; charm-log matches the v2 stack |
| Width calc | go-runewidth | uniseg | go-runewidth is the charm transitive; not worth swapping |
## What NOT to Use
| Avoid | Why | Use instead |
|-------|-----|-------------|
| `github.com/charmbracelet/bubbletea` (v1 import path) | Deprecated; charmbracelet migrated all v2 modules to `charm.land` vanity. v1 gets no fixes. | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/lipgloss` (v1) | Same -- moved to `charm.land/lipgloss/v2`; subpackages `table`/`tree`/`list` followed | `charm.land/lipgloss/v2` (+ subpackages) |
| `github.com/charmbracelet/bubbles/...` (v1) | v1 is unmaintained; v2 dropped `runeutil` and `memoization` | `charm.land/bubbles/v2/<component>` |
| `github.com/charmbracelet/huh` (v1) | v2 stable; v1 paths changed | `charm.land/huh/v2` |
| `github.com/charmbracelet/wish` (v1) | v2 uses `charm.land/wish/v2` and subpackages (`bubbletea`, `logging`, `activeterm`) | `charm.land/wish/v2` |
| `github.com/charmbracelet/log` (v1) | v2 stable | `charm.land/log/v2` |
| `github.com/charmbracelet/fang` (v1) | v2 stable, path `charm.land/fang/v2` | `charm.land/fang/v2` |
| `github.com/charmbracelet/glamour` (v1) | v2 line moved | `charm.land/glamour/v2` |
| `urfave/cli` | Spec excludes; cobra + fang is the charmbracelet default | cobra + fang |
| `tview` | Spec excludes; non-charm TUI framework | bubbletea v2 + bubbles v2 |
| `ratatui` (Rust port) | Spec excludes; not even Go | bubbletea v2 |
| `charm.land/gum/v2` (does not exist) | `gum` has no Go library -- only the CLI binary. Trying to import will fail. | Shell out to `gum` binary via `os/exec` |
| `Bubble Tea v1` import path `github.com/charmbracelet/bubbletea` | v1 uses `View() string`; v2 uses `View() tea.View`; migrating is a project-wide rewrite | Start v2; never look back |
| `Bubble Tea v1` `tea.WithAltScreen()` program option | In v2, AltScreen/MouseMode are fields on `tea.View`, not program options. `NewProgram` signature is simplified. | Set `v.AltScreen = true` on the `tea.View` returned from `View()` |
## Stack Patterns by Variant
- Generate `cmd/foo/main.go` with bubbletea v2 model (uses `tea.View`, `tea.KeyPressMsg`)
- Generate `internal/ui/styles.go` with `charm.land/lipgloss/v2` styles
- Pin `charm.land/bubbletea/v2 v2.0.0` and `charm.land/bubbles/v2 v2.0.0` in `go.mod`
- Ship `go 1.25.0` in `go.mod` (minimum for bubbles v2)
- Include `.air.toml` so `spin run` uses hot reload
- Generate `cmd/foo/main.go` using cobra v1.9.1 with `fang.Execute(ctx, rootCmd)`
- Wire `viper.BindPFlag(...)` for any `--config` flag
- Pin `charm.land/fang/v2` and `github.com/spf13/cobra v1.9.1` in `go.mod`
- Ship `go 1.23` in `go.mod` (no charm v2 libs, no 1.25+ requirement)
- Combine both variants: cobra root, `--tui` flag launches bubbletea v2 program
- All v2 deps in `go.mod`; `go 1.25.0` floor
- Fall back to huh v2 in-process forms (not gum shell-out)
- This is the resilience pattern; documented in ARCHITECTURE.md
- `spin test` falls back to `go test ./...`
- Log a one-time warning that install `prism` for colored/parallel output
- `spin fmt` falls back to `gofmt`
- Same fall-back pattern as prism
## Go Version Tension -- call out for roadmap
- `spin` itself: does not need charm v2 at runtime; Go 1.23 is sufficient and
- Scaffolded projects that pull in `charm.land/bubbles/v2`: official docs
- Pin `go 1.23` in `spin`'s own `go.mod`.
- For scaffolded projects, pin `go 1.25.0` in the generated `go.mod` (so bubbles
- If a user passes only CLI flags (no `--bubbles` / `--huh` / `--bubbletea`),
- This is a phase-1 decision to confirm in the requirements doc.
## Version Compatibility Matrix
| Generated project contains | Minimum Go | Confirmed via |
|----------------------------|------------|---------------|
| bubbletea v2 only | Go 1.23 likely OK; 1.25 to be safe | bubbles v2 docs require 1.25; bubbletea v2 docs do not specify a floor but use the same code |
| bubbletea v2 + bubbles v2 | Go 1.25.0 | `/charmbracelet/bubbles/_autodocs/README.md` |
| huh v2 | inherits from bubbletea + lipgloss | upgrade guide |
| lipgloss v2 | unknown floor; `go 1.22` likely works | not explicitly stated |
| wish v2 | inherits from bubbletea | upgrade guide |
| log v2 | standard library only -- no floor | upgrade guide |
| glamour v2 | unknown floor | not explicitly stated |
| cobra v1.9.1 | Go 1.18+ | spf13/cobra README |
| viper v1.20.x | Go 1.20+ | spf13/viper README |
| air (dev tool) | Go 1.25+ for `go install` | `/air-verse/air` docs |
| goreleaser v2 (dev tool) | Go 1.26+ for `go install` | goreleaser install docs |
| prism (dev tool) | Go 1.24+ for `go install` | prism README |
| fang v2 | cobra v1.9+ | fang upgrade guide |
| charmbracelet/x | experimental; pin a tag | charmbracelet/x README |
## Sources (verified 2026-06-02)
- `/charmbracelet/bubbletea` -- v2 import paths, `tea.View` API change, KeyPressMsg/MouseClickMsg typing (HIGH)
- `/charmbracelet/lipgloss` -- v2 vanity domain, subpackage paths (HIGH)
- `/charmbracelet/bubbles` -- v2 import paths, Go 1.25.0 floor, removed `runeutil`/`memoization` (HIGH)
- `/charmbracelet/huh` -- v2 upgrade steps; charm.land migration (HIGH)
- `/charmbracelet/glamour` -- `charm.land/glamour/v2` path; `glamour.Render()`/`NewTermRenderer` (HIGH)
- `/charmbracelet/glow` -- `github.com/charmbracelet/glow/v2` install (HIGH)
- `/charmbracelet/wish` -- v2 import paths; subpackages follow `charm.land/wish/v2/<sub>` (HIGH)
- `/charmbracelet/log` -- `charm.land/log/v2`, `Default()`/`SetDefault()` API (HIGH)
- `/charmbracelet/fang` -- `charm.land/fang/v2`; `fang.Execute(ctx, cmd)` drop-in (HIGH)
- `/charmbracelet/gum` -- **binary-only**; no Go library; install via `go install github.com/charmbracelet/gum@latest` (HIGH)
- `/charmbracelet/crush` -- internal packages only; treat as binary (HIGH)
- `/charmbracelet/x` -- `ansi` subpackage API; experimental; `pony` UI DSL; `vt` emulator (HIGH)
- `/charmbracelet/bubbletea-app-template` -- `.goreleaser.yaml` `version: 2`; `go 1.24.2`; `CGO_ENABLED=0` (HIGH)
- `/spf13/cobra` -- v1.9.1 install; flag definitions; init pattern (HIGH)
- `/spf13/viper` -- v1.20.x; mapstructure v2 import (HIGH)
- `/air-verse/air` -- `go install github.com/air-verse/air@latest`; Go 1.25+; `.air.toml` schema (HIGH)
- `/mvdan/gofumpt` -- `mvdan.cc/gofumpt@latest` install; `gofumpt -l -w .` (HIGH)
- `/daltonsw/prism` -- `go install go.dalton.dog/prism@latest`; Go 1.24+; `prism`, `prism -v`, `prism -f` flags (HIGH)
- `/goreleaser/goreleaser` -- `go install github.com/goreleaser/goreleaser/v2@latest`; Go 1.26+; `version: 2` schema (HIGH)
- `/mattn/go-runewidth` -- `RUNEWIDTH_EASTASIAN` env var; `IsEastAsian()` (MEDIUM -- for width calc only)
## Confidence Assessment
| Area | Confidence | Reason |
|------|------------|--------|
| Charm v2 module paths | HIGH | Verified against official upgrade guides for every lib |
| Charm v2 API breaking changes (View→tea.View, KeyPressMsg typing) | HIGH | Confirmed in bubbletea UPGRADE_GUIDE_V2.md |
| bubbletea/lipgloss/huh/bubbles/glamour/glow/wish/log/fang versions | HIGH | All on v2 stable line per Context7 |
| `gum` is binary-only (no Go library) | HIGH | Context7 docs show only install/CLI usage, no Go package |
| `crush` Go client is internal-only | HIGH | Context7 docs show `github.com/charmbracelet/crush/internal/client` |
| Go version floor for bubbles v2 = 1.25.0 | HIGH | Direct quote from `/charmbracelet/bubbles/_autodocs/README.md` |
| Go version floor for other charm v2 libs | MEDIUM | Not explicitly stated for each lib; assumed transitive of bubbles |
| air, goreleaser v2 Go version floors | HIGH | Direct quotes from official docs |
| prism Go version floor = 1.24 | HIGH | Direct quote from prism README |
| Viper mapstructure v2 fork migration | HIGH | Direct quote from spf13/viper UPGRADE.md |
## Open Questions
- Should generated `--tui` projects default to `go 1.25.0` in `go.mod`, or to
- Should `spin new` add `//go:build` constraints to allow the same scaffolded
- Should the scaffolder embed `gum` as a `//go:embed gum` binary, or always
- Should `crush` integration be binary-only, or expose the `internal/client`
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
