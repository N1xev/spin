# spin

## What This Is

`spin` is a Go project scaffold CLI for the charmbracelet v2 ecosystem. It generates ready-to-run Go projects — TUI apps, CLI tools, or both — pre-wired with the right charmbracelet libraries, modern Go tooling (cobra, fang, gum), hot reload (air), and the prism test runner. One command produces a project that builds, tests, and runs without extra setup. Built with cobra + fang + gum so the tool itself demonstrates the charmbracelet experience.

## Core Value

Generate a perfect, runnable Go project using charmbracelet v2 libraries with a single command — `spin new myapp --tui --bubbletea` produces a project that `go run`s cleanly on first try.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — ship to validate)

### Active

<!-- Current scope. Building toward these. -->

- [ ] `spin new <name>` scaffolds a new Go project in `./<name>`
- [ ] Top-level project-type flags: `--tui`, `--cli`, `--all`
- [ ] Per-library subflags for charmbracelet libs (e.g. `--bubbletea`, `--lipgloss`, `--huh`, `--glow`, `--glamour`, `--wish`, `--log`, `--crush`, `--modifiers`, `--ansi`, `--runewidth`)
- [ ] `--cobra` (default on) and `--fang` (default on) for CLI projects; `--viper` opt-in for config
- [ ] `--template <name>` selects a bundled template variant
- [ ] `--template-repo <url>` overrides the embedded template with an external git repo
- [ ] `--ai` (or `--agents`) generates an `AGENTS.md` describing the project for AI assistants
- [ ] `spin run` — runs the project (uses `air` for hot reload if `.air.toml` present)
- [ ] `spin build` — builds binary to `bin/`
- [ ] `spin test` — runs `prism` instead of bare `go test`
- [ ] `spin vet` — wraps `go vet` (whole module)
- [ ] `spin fmt` — wraps `gofumpt` (or `go fmt` if gofumpt unavailable) + `goimports`
- [ ] Interactive prompts (gum) when flags are missing, asking project type / libs / template / AI
- [ ] Generated project includes `.air.toml`, `Taskfile.yml` or `Makefile`, `go.mod` with pinned charm v2 deps
- [ ] Generated project ships a working example (bubbletea "hello" TUI or cobra/fang "hello" CLI)

### Out of Scope

- Non-charmbracelet UI frameworks (tview, ratatui, urfave/cli) — `spin` is opinionated about charm — explicit user request
- Non-Go languages (Rust/TS scaffolds) — wrong tool for the job
- Online template registry / marketplace — local + override is enough for v1
- Plugin system for custom scaffolders — defer to v2
- Auto-updating generated projects after scaffold — out of scope, regenerate instead
- CI/CD pipeline generation — out of scope, project authors configure their own
- Dockerfile/compose generation — out of scope for v1
- Remote execution / cloud scaffolds — local-only
- GUI/TUI mode for the scaffolder itself (TUI for the generated project is fine; the scaffolder is a CLI)

## Context

- charmbracelet published v2 of most libraries in 2024–2025; v1 paths/APIs differ and v2 must be used to get current style and stability
- Charm ecosystem pieces: bubbletea (TUI framework), lipgloss (styling), huh (forms), bubbles (components), glamour (markdown), glow (markdown reader), wish (SSH), log (logging), crush (codec), modifiers, ansi, runewidth, cobra + fang (CLI framework + styled help), gum (interactivity), viper (config)
- `gofumpt` is stricter than `gofmt` and is the de facto standard in modern Go projects
- `prism` is a `go test` replacement that runs tests in parallel workers with better output
- `air` is the de facto Go hot-reload tool; configured via `.air.toml`
- `fang` is the styled, accessible help renderer for cobra — it gives cobra CLIs a polished feel
- The scaffolder should be a showcase for the charm stack — running `spin --help` should feel like using a charm product
- Target user: a Go developer who has heard of charmbracelet and wants a zero-friction on-ramp

## Constraints

- **Tech stack**: Go 1.22+ (use 1.23 if available); built with cobra + fang + gum; consumes charmbracelet v2 libs only — Why: user specified charm-only, modern Go
- **Distribution**: single static binary; install via `go install github.com/<org>/spin@latest` — Why: standard Go CLI distribution, no runtime deps
- **Templates**: embedded via `go:embed` (default) + `--template-repo` for external override — Why: works offline by default, flexible for advanced users
- **Test runner**: `prism` (https://github.com/DaltonSW/prism), not `go test` directly — Why: user requested, better DX for parallel/colored output
- **Formatter**: `gofumpt` (primary) with `goimports`; fall back to `gofmt` if gofumpt not installed — Why: stricter formatting is the modern Go default
- **Hot reload**: `air` with a sensible `.air.toml` — Why: user requested, industry standard
- **No CGO**: scaffolded projects should build with `CGO_ENABLED=0` — Why: cross-compile and minimal container sizes
- **Charm v2 only**: do not import v1 paths or APIs — Why: v1 deprecated, v2 is current; researched via context7

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Binary name = `spin` | User-selected; short verb, evokes "spinning up" a project | — Pending |
| Templates embedded + override | Offline default, flexibility for power users | — Pending |
| Interactive gum prompts (default) | Friendly for new users; flag-only via `--no-interactive` | — Pending |
| Scaffolder wraps `go run`/`prism`/`go vet`/`gofumpt` | One tool to learn; consistent commands across projects | — Pending |
| Charm v2 only | v1 deprecated; user explicitly wants v2 | — Pending |
| Cobra + fang + gum for the scaffolder itself | Dogfooding; showcase the charm stack | — Pending |
| Viper as opt-in (`--viper`) | Not every CLI needs config; don't force it | — Pending |
| `AGENTS.md` opt-in via `--ai` | Some users want AI-assistant context, some don't | — Pending |
| Project root = working dir at scaffold time, project in subdir `name/` | Matches `cargo new`, `npm init` conventions | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-06-02 after initialization*
