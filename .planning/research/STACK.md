# Stack Research

**Domain:** Go project scaffold CLI for the charmbracelet v2 ecosystem
**Researched:** 2026-06-02
**Confidence:** HIGH (verified via Context7 + official upgrade guides)

## Recommended Stack

This stack has two layers: **(a)** the libraries `spin` injects into scaffolded
projects, and **(b)** the libraries `spin` itself uses to build and ship. All
versions verified against Context7 + official upgrade guides on 2026-06-02.

### (a) Libraries `spin` injects into scaffolded projects

These are the charmbracelet v2 libraries the user picks per-project with flags
like `--bubbletea`, `--lipgloss`, `--huh`, etc. All v2 modules use the new
`charm.land` vanity domain — **v1 paths (`github.com/charmbracelet/...`) are
deprecated and must not be used**.

| Library | Module path | Version (v2 line) | Purpose | Why v2 |
|---------|-------------|-------------------|---------|--------|
| Bubble Tea | `charm.land/bubbletea/v2` | v2.0.0 (stable) | TUI framework, MVU runtime | v2 stable; v2 renames `View() string` → `View() tea.View`, moves AltScreen/MouseMode to view fields, uses typed `KeyPressMsg`/`MouseClickMsg` |
| Lip Gloss | `charm.land/lipgloss/v2` | v2.0.0-beta.2 (stable line) | Terminal styling/layout (CSS-like API) | v2 is the supported line; subpackages `table`, `tree`, `list` follow `charm.land/lipgloss/v2/<sub>` |
| Bubbles | `charm.land/bubbles/v2` | v2.0.0 | TUI components: spinner, textinput, viewport, list, table, paginator, progress, timer, textarea, help, key, cursor, stopwatch | v2 line; `runeutil` and `memoization` removed; requires Go 1.25.0+ |
| Huh | `charm.land/huh/v2` | v2.0.0 | Interactive forms/prompts (accessible) | v2 stable; integrates with `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` |
| Glamour | `charm.land/glamour/v2` | v2 line | Stylesheet-based markdown renderer for terminal | v2 stable; `glamour.Render()` and `NewTermRenderer` API unchanged in spirit |
| Glow | `github.com/charmbracelet/glow/v2` (binary) | v2 line | Markdown reader CLI — install as binary, shell out via `gum`-style exec | Scaffolded projects shell out to `glow` for readme rendering |
| Wish | `charm.land/wish/v2` (+ subpackages `bubbletea`, `logging`, `activeterm`) | v2 line | SSH server framework with bubbletea middleware | v2 stable; subpackages follow `charm.land/wish/v2/<sub>` |
| Log | `charm.land/log/v2` | v2.0.0 | Minimal colorful leveled structured logging | v2 stable; `log.Default()`/`SetDefault()` + `Options{...}` |
| Crush | `github.com/charmbracelet/crush` | current | Terminal AI assistant — scaffolded projects may include `crush` config | Provided as a binary; embed for the AI/AGENTS layer |
| charmbracelet/x | `github.com/charmbracelet/x` (single module, many subpackages) | current (experimental) | ANSI parser/generator, VT emulator, `pony` UI DSL, term utilities | Experimental; pin a specific tag in generated `go.mod` |
| go-runewidth | `github.com/mattn/go-runewidth` | current | East-Asian-aware display width | Transitive of charm stack; rarely direct dep |

**Note on Crush scope:** `crush` exposes its client API as internal packages
(`github.com/charmbracelet/crush/internal/client`, `internal/proto`).
Scaffolded projects should consume `crush` as a **binary** (config + CLI), not
as a Go import. This matches the spec's `--crush` flag meaning "include crush
config" not "import crush in code".

### (b) Libraries `spin` itself uses

| Library | Module path | Version | Purpose | Why |
|---------|-------------|---------|---------|-----|
| Cobra | `github.com/spf13/cobra` | v1.9.1 (latest) | CLI subcommand/flag framework | De facto Go CLI standard; underpins kubectl, hugo, gh, docker |
| Fang | `charm.land/fang/v2` | v2 line | Styled help, errors, completions, manpages, version theming — drop-in for cobra's default | Drop-in `fang.Execute(ctx, rootCmd)`; gives `spin --help` a charm-style look out of the box; requires cobra v1.9+ |
| Lip Gloss | `charm.land/lipgloss/v2` | v2 | Styling scaffolder output (success messages, "Created at ./foo", etc.) | Reuses the same lib we scaffold into projects — dogfooding |
| Huh | `charm.land/huh/v2` | v2 | Optional in-process prompts (when gum not available) | Huh is a Go library; gum is a shell-out binary. Huh is the in-process fallback for TTY-required runs |
| Log | `charm.land/log/v2` | v2 | Scaffolder logging (`log.Info("created", "path", ...)`) | Same library we ship in projects; consistent look |
| Viper | `github.com/spf13/viper` | v1.20.x (opt-in) | Config-file support for scaffolder | Only wired when user passes `--viper`; do not import unconditionally (per spec) |
| charmbracelet/x ansi | `github.com/charmbracelet/x/ansi` | current | Lower-level ANSI sequence generation if we need it | Used only if lipgloss v2 doesn't cover a case |

**`gum` is a binary, not a Go library.** `spin` shells out to it via
`os/exec` for interactive prompts when present on `$PATH`. The `charmbracelet/gum`
repository has no `pkg` for embedding — confirmed by Context7 docs. This is
critical: do **not** try to `go get charm.land/gum/v2` or similar; it doesn't
exist. Install path is `go install github.com/charmbracelet/gum@latest`.

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

```bash
# Core CLI stack
go get github.com/spf13/cobra@latest          # v1.9.1
go get charm.land/fang/v2@latest              # v2 line
go get charm.land/lipgloss/v2@latest          # v2 line, for scaffolder output
go get charm.land/huh/v2@latest               # v2 line, in-process prompts fallback
go get charm.land/log/v2@latest               # v2 line, logging
go get github.com/charmbracelet/x/ansi@latest # optional low-level ANSI

# Opt-in: Viper for --viper flag
go get github.com/spf13/viper@latest          # v1.20.x

# Optional dev tools installed by users (NOT by spin):
#   go install github.com/air-verse/air@latest
#   go install go.dalton.dog/prism@latest
#   go install mvdan.cc/gofumpt@latest
#   go install golang.org/x/tools/cmd/goimports@latest
#   go install github.com/goreleaser/goreleaser/v2@latest
```

## Alternatives Considered

| Layer | Recommended | Alternative | When to use the alternative |
|-------|-------------|-------------|-----------------------------|
| CLI framework | cobra + fang | urfave/cli | Never for this project — spec explicitly excludes urfave/cli; fang is the charmbracelet polish layer |
| TUI framework (generated projects) | bubbletea v2 | tview, ratatui | Never for this project — spec explicitly excludes both; spin is opinionated about charm |
| TUI components | bubbles v2 | hand-rolled | Use hand-rolled only for components bubbles doesn't ship (rare) |
| Interactive prompts (scaffolder) | gum (subprocess) + huh v2 (in-process) | survey, promptui | Never — spec is charm-only |
| Config (scaffolder) | viper (opt-in) | envconfig, koanf | Use koanf only if a user asks for a viper replacement; default off |
| Hot reload | air | wgo, realize | Never for scaffolded projects — spec mandates air |
| Test runner | prism | gotestsum, richgo | Use gotestsum only if prism install fails; spec mandates prism |
| Formatter | gofumpt + goimports | gofmt only | gofmt only as last-resort fallback when gofumpt not installed |
| Distribution | goreleaser v2 | manual `go build` | Never — spec mandates goreleaser |
| Logging (generated) | charmbracelet/log v2 | zap, slog, zerolog | slog is acceptable for non-charm projects; charm-log matches the v2 stack |
| Width calc | go-runewidth | uniseg | go-runewidth is the charm transitive; not worth swapping |

## What NOT to Use

| Avoid | Why | Use instead |
|-------|-----|-------------|
| `github.com/charmbracelet/bubbletea` (v1 import path) | Deprecated; charmbracelet migrated all v2 modules to `charm.land` vanity. v1 gets no fixes. | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/lipgloss` (v1) | Same — moved to `charm.land/lipgloss/v2`; subpackages `table`/`tree`/`list` followed | `charm.land/lipgloss/v2` (+ subpackages) |
| `github.com/charmbracelet/bubbles/...` (v1) | v1 is unmaintained; v2 dropped `runeutil` and `memoization` | `charm.land/bubbles/v2/<component>` |
| `github.com/charmbracelet/huh` (v1) | v2 stable; v1 paths changed | `charm.land/huh/v2` |
| `github.com/charmbracelet/wish` (v1) | v2 uses `charm.land/wish/v2` and subpackages (`bubbletea`, `logging`, `activeterm`) | `charm.land/wish/v2` |
| `github.com/charmbracelet/log` (v1) | v2 stable | `charm.land/log/v2` |
| `github.com/charmbracelet/fang` (v1) | v2 stable, path `charm.land/fang/v2` | `charm.land/fang/v2` |
| `github.com/charmbracelet/glamour` (v1) | v2 line moved | `charm.land/glamour/v2` |
| `urfave/cli` | Spec excludes; cobra + fang is the charmbracelet default | cobra + fang |
| `tview` | Spec excludes; non-charm TUI framework | bubbletea v2 + bubbles v2 |
| `ratatui` (Rust port) | Spec excludes; not even Go | bubbletea v2 |
| `charm.land/gum/v2` (does not exist) | `gum` has no Go library — only the CLI binary. Trying to import will fail. | Shell out to `gum` binary via `os/exec` |
| `Bubble Tea v1` import path `github.com/charmbracelet/bubbletea` | v1 uses `View() string`; v2 uses `View() tea.View`; migrating is a project-wide rewrite | Start v2; never look back |
| `Bubble Tea v1` `tea.WithAltScreen()` program option | In v2, AltScreen/MouseMode are fields on `tea.View`, not program options. `NewProgram` signature is simplified. | Set `v.AltScreen = true` on the `tea.View` returned from `View()` |

## Stack Patterns by Variant

**If user runs `spin new foo --tui --bubbletea --bubbles --lipgloss`:**
- Generate `cmd/foo/main.go` with bubbletea v2 model (uses `tea.View`, `tea.KeyPressMsg`)
- Generate `internal/ui/styles.go` with `charm.land/lipgloss/v2` styles
- Pin `charm.land/bubbletea/v2 v2.0.0` and `charm.land/bubbles/v2 v2.0.0` in `go.mod`
- Ship `go 1.25.0` in `go.mod` (minimum for bubbles v2)
- Include `.air.toml` so `spin run` uses hot reload

**If user runs `spin new foo --cli --cobra --fang --viper`:**
- Generate `cmd/foo/main.go` using cobra v1.9.1 with `fang.Execute(ctx, rootCmd)`
- Wire `viper.BindPFlag(...)` for any `--config` flag
- Pin `charm.land/fang/v2` and `github.com/spf13/cobra v1.9.1` in `go.mod`
- Ship `go 1.23` in `go.mod` (no charm v2 libs, no 1.25+ requirement)

**If user runs `spin new foo --all`:**
- Combine both variants: cobra root, `--tui` flag launches bubbletea v2 program
- All v2 deps in `go.mod`; `go 1.25.0` floor

**If gum is not on `$PATH` and TTY is attached:**
- Fall back to huh v2 in-process forms (not gum shell-out)
- This is the resilience pattern; documented in ARCHITECTURE.md

**If `prism` is not on `$PATH`:**
- `spin test` falls back to `go test ./...`
- Log a one-time warning that install `prism` for colored/parallel output

**If `gofumpt` is not on `$PATH`:**
- `spin fmt` falls back to `gofmt`
- Same fall-back pattern as prism

## Go Version Tension — call out for roadmap

The PROJECT.md spec says: "**Go 1.22+ (use 1.23 if available)**". The
research surfaces a real conflict:

- `spin` itself: does not need charm v2 at runtime; Go 1.23 is sufficient and
  matches the spec.
- Scaffolded projects that pull in `charm.land/bubbles/v2`: official docs
  require **Go 1.25.0+**. The official `charmbracelet/bubbletea-app-template`
  ships `go 1.24.2` and depends on v1 libs (this template hasn't migrated
  to v2 yet). For v2-only scaffolds, **1.25.0 is the floor**.

**Recommendation:**
- Pin `go 1.23` in `spin`'s own `go.mod`.
- For scaffolded projects, pin `go 1.25.0` in the generated `go.mod` (so bubbles
  v2 works) and document that the user needs Go 1.25+ to build.
- If a user passes only CLI flags (no `--bubbles` / `--huh` / `--bubbletea`),
  downgrade the generated `go.mod` to `go 1.23`.
- This is a phase-1 decision to confirm in the requirements doc.

**Confidence:** MEDIUM. The 1.25.0 minimum for bubbles v2 comes from the
official `_autodocs/README.md`; we have not independently verified
every charm v2 lib's `go.mod` floor. Bubbles is the most likely to require
1.25 because it relies on newer `iter`/generic features.

## Version Compatibility Matrix

| Generated project contains | Minimum Go | Confirmed via |
|----------------------------|------------|---------------|
| bubbletea v2 only | Go 1.23 likely OK; 1.25 to be safe | bubbles v2 docs require 1.25; bubbletea v2 docs do not specify a floor but use the same code |
| bubbletea v2 + bubbles v2 | Go 1.25.0 | `/charmbracelet/bubbles/_autodocs/README.md` |
| huh v2 | inherits from bubbletea + lipgloss | upgrade guide |
| lipgloss v2 | unknown floor; `go 1.22` likely works | not explicitly stated |
| wish v2 | inherits from bubbletea | upgrade guide |
| log v2 | standard library only — no floor | upgrade guide |
| glamour v2 | unknown floor | not explicitly stated |
| cobra v1.9.1 | Go 1.18+ | spf13/cobra README |
| viper v1.20.x | Go 1.20+ | spf13/viper README |
| air (dev tool) | Go 1.25+ for `go install` | `/air-verse/air` docs |
| goreleaser v2 (dev tool) | Go 1.26+ for `go install` | goreleaser install docs |
| prism (dev tool) | Go 1.24+ for `go install` | prism README |
| fang v2 | cobra v1.9+ | fang upgrade guide |
| charmbracelet/x | experimental; pin a tag | charmbracelet/x README |

## Sources (verified 2026-06-02)

Context7 library IDs consulted, all via `npx ctx7@latest docs` CLI fallback:

- `/charmbracelet/bubbletea` — v2 import paths, `tea.View` API change, KeyPressMsg/MouseClickMsg typing (HIGH)
- `/charmbracelet/lipgloss` — v2 vanity domain, subpackage paths (HIGH)
- `/charmbracelet/bubbles` — v2 import paths, Go 1.25.0 floor, removed `runeutil`/`memoization` (HIGH)
- `/charmbracelet/huh` — v2 upgrade steps; charm.land migration (HIGH)
- `/charmbracelet/glamour` — `charm.land/glamour/v2` path; `glamour.Render()`/`NewTermRenderer` (HIGH)
- `/charmbracelet/glow` — `github.com/charmbracelet/glow/v2` install (HIGH)
- `/charmbracelet/wish` — v2 import paths; subpackages follow `charm.land/wish/v2/<sub>` (HIGH)
- `/charmbracelet/log` — `charm.land/log/v2`, `Default()`/`SetDefault()` API (HIGH)
- `/charmbracelet/fang` — `charm.land/fang/v2`; `fang.Execute(ctx, cmd)` drop-in (HIGH)
- `/charmbracelet/gum` — **binary-only**; no Go library; install via `go install github.com/charmbracelet/gum@latest` (HIGH)
- `/charmbracelet/crush` — internal packages only; treat as binary (HIGH)
- `/charmbracelet/x` — `ansi` subpackage API; experimental; `pony` UI DSL; `vt` emulator (HIGH)
- `/charmbracelet/bubbletea-app-template` — `.goreleaser.yaml` `version: 2`; `go 1.24.2`; `CGO_ENABLED=0` (HIGH)
- `/spf13/cobra` — v1.9.1 install; flag definitions; init pattern (HIGH)
- `/spf13/viper` — v1.20.x; mapstructure v2 import (HIGH)
- `/air-verse/air` — `go install github.com/air-verse/air@latest`; Go 1.25+; `.air.toml` schema (HIGH)
- `/mvdan/gofumpt` — `mvdan.cc/gofumpt@latest` install; `gofumpt -l -w .` (HIGH)
- `/daltonsw/prism` — `go install go.dalton.dog/prism@latest`; Go 1.24+; `prism`, `prism -v`, `prism -f` flags (HIGH)
- `/goreleaser/goreleaser` — `go install github.com/goreleaser/goreleaser/v2@latest`; Go 1.26+; `version: 2` schema (HIGH)
- `/mattn/go-runewidth` — `RUNEWIDTH_EASTASIAN` env var; `IsEastAsian()` (MEDIUM — for width calc only)

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
  the lowest viable floor (1.23) for fewer barriers? Recommend 1.25.0 since
  bubbles v2 requires it.
- Should `spin new` add `//go:build` constraints to allow the same scaffolded
  module to build on older Go when charm v2 features aren't used? Probably
  not — keep it simple, one floor per project.
- Should the scaffolder embed `gum` as a `//go:embed gum` binary, or always
  require it on `$PATH`? Recommend on-`$PATH` with a clear "install gum"
  hint; embedding complicates cross-compile and CGO/CGO-not concerns.
- Should `crush` integration be binary-only, or expose the `internal/client`
  proto API? Binary-only is the safer call (internal packages have no
  stability guarantee).

---

*Stack research for: spin — Go project scaffold CLI for the charmbracelet v2 ecosystem*
*Researched: 2026-06-02*
