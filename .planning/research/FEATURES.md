# Feature Research: spin (charmbracelet v2 Go scaffold CLI)

**Domain:** CLI tooling -- Go project scaffolder
**Researched:** 2026-06-02
**Confidence:** HIGH (v2 import paths verified via Context7; feature landscape triangulated against cobra-cli, spring initializr, cargo, cookiecutter, yeoman; charmbracelet-app-template structure verified)

---

## Executive Summary

`spin` sits in a well-defined category -- language/ecosystem scaffolders -- where the table stakes are clear and the differentiators are mostly *aesthetic* and *opinionated*. Every competitor (`cargo new`, `npm init`, `cobra-cli init`, `cookiecutter`, `spring initializr`, `yeoman`) does the same job: take a name + flags, emit a directory tree, write a `go.mod`-equivalent, and exit cleanly. The reason `spin` can win a small niche is not feature breadth but *vertical integration with the charmbracelet v2 stack* -- the scaffolder itself uses fang + gum + lipgloss + huh so the tool *demonstrates* the experience it scaffolds. That is the differentiator; everything else is execution quality.

The anti-feature list is short but firm: no template marketplace, no plugin system, no TUI mode for the scaffolder itself, no CI generation, no GUI. All are tempting and all break the "small, sharp, opinionated" thesis.

Charmbracelet v2 has matured substantially (v2 import paths use `charm.land/<lib>/v2`). The `bubbletea-app-template` repo already ships a reference structure (lint config, goreleaser, dependabot, GH Actions) that `spin` should embed as a starting point, but `spin` should *improve* on it by (a) consolidating `Makefile`+`Taskfile` into one, (b) wiring `air` and `prism` by default, (c) using fang, and (d) generating `AGENTS.md` for AI assistants -- none of which the reference template does.

---

## Charmbracelet v2 Library Inventory

All import paths verified via Context7 UPGRADE_GUIDE_V2.md docs. v2 uses the `charm.land/<lib>/v2` vanity domain (replaces `github.com/charmbracelet/<lib>`).

### TUI Framework & Components (for `--tui` projects)

| Library | v2 Import Path | Purpose | Flag |
|---------|----------------|---------|------|
| bubbletea | `charm.land/bubbletea/v2` | Elm-architecture TUI framework (Model-View-Update) | `--bubbletea` |
| bubbles | `charm.land/bubbles/v2` | TUI components (spinner, textinput, list, table, viewport, paginator, progress, timer, help, key, cursor, textarea, filepicker) | `--bubbles` (implies `--bubbletea`) |
| huh | `charm.land/huh/v2` | Interactive forms/prompts (input, select, confirm, file picker) | `--huh` |
| harmonica | `charm.land/harmonica/v2` | Spring-based animation toolkit | `--harmonica` |

### CLI Framework (for `--cli` projects)

| Library | v2 Import Path | Purpose | Flag |
|---------|----------------|---------|------|
| cobra | `github.com/spf13/cobra` (no v2 path; spf13 maintains it) | Subcommand + POSIX flag parser | `--cobra` (default on for CLI) |
| fang | `charm.land/fang/v2` | Styled help, errors, version, manpages, completions, theming for cobra | `--fang` (default on for CLI) |
| viper | `github.com/spf13/viper` (no v2 path) | Config/env/flag merging | `--viper` (opt-in) |

### Styling & Rendering (cross-cutting; useful in any project)

| Library | v2 Import Path | Purpose | Flag |
|---------|----------------|---------|------|
| lipgloss | `charm.land/lipgloss/v2` | Terminal style/layout (CSS-like API, color profiles) | `--lipgloss` |
| ansi | `charm.land/x/ansi` (under `charmbracelet/x`) | Low-level ANSI/VT100 escape sequence generation & parser | `--ansi` |
| runewidth | `charm.land/x/runewidth` (or `github.com/mattn/go-runewidth` upstream) | Display-width calculation (CJK, emoji) | `--runewidth` |
| modifiers | `charm.land/x/modifiers` (under `charmbracelet/x`) | Input/entering-style event modifiers | `--modifiers` |
| glamour | `charm.land/glamour/v2` | Stylesheet-driven markdown renderer | `--glamour` |
| glow | `github.com/charmbracelet/glow` (CLI binary, embeds glamour) | Markdown reader CLI | `--glow` (binary dep, not import) |

### SSH (for `--ssh` projects)

| Library | v2 Import Path | Purpose | Flag |
|---------|----------------|---------|------|
| wish | `charm.land/wish/v2` | SSH server framework with middlewares | `--wish` |
| wish/bubbletea | `charm.land/wish/v2/bubbletea` | Middleware to serve Bubble Tea apps over SSH | (auto with `--wish --bubbletea`) |
| wish/logging | `charm.land/wish/v2/logging` | Middleware that pipes wish logs through charm log | (auto with `--wish`) |
| wish/activeterm | `charm.land/wish/v2/activeterm` | Tracks active terminal sessions for broadcast | (opt-in, `--activeterm`) |

### Utility (cross-cutting)

| Library | v2 Import Path | Purpose | Flag |
|---------|----------------|---------|------|
| log | `charm.land/log/v2` | Leveled, colorful, structured logger | `--log` |
| crush | `github.com/charmbracelet/crush` (binary, not import -- AI agent) | AI coding assistant in the terminal | `--crush` (binary, opt-in `AGENTS.md` integration) |

### Scaffolder itself (dogfooding)

| Library | Path | Why |
|---------|------|-----|
| cobra | `github.com/spf13/cobra` | Standard subcommand structure |
| fang | `charm.land/fang/v2` | Styled help output (showcase) |
| gum | (binary `charm.land/gum`) | Interactive prompts when flags are missing -- invoked as a binary; no Go import needed |

**Flag design note:** Flags follow the user-role grouping above. `--tui` (umbrella for bubbletea + bubbles + lipgloss) and `--cli` (umbrella for cobra + fang) are top-level; per-library subflags give fine control. `--wish` is orthogonal to `--tui`/`--cli` (an SSH project can be `--tui` or `--cli` over SSH). `--all` is a convenience that pulls every charm v2 lib.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing = product feels incomplete, broken, or untrustworthy.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `spin new <name>` scaffolds a project in `./<name>` | Mirrors `cargo new`, `npm init`, `cobra-cli init`, `spring init`, `django-admin startproject`. The single most basic scaffolder contract. | LOW | Default; required positional arg. |
| Project name validation (kebab-case, lowercase, no spaces) | All Go tooling (modules, package paths, binary names) assumes this; `go mod init` rejects invalid names. | LOW | Use cobra's `Args` validator. |
| Generates `go.mod` with correct module path (`github.com/<user>/<name>`) | Non-negotiable. Every Go project needs it. | LOW | Default module = `github.com/<user-or-org>/<name>`; override with `--module`. |
| Generates `main.go` (or `cmd/<name>.go`) that compiles and runs | The "builds on first try" promise is the core value. | LOW | Two templates: bubbletea `main.go` and cobra `cmd/root.go`+`main.go`. |
| Sensible default `.gitignore` (covers `bin/`, IDE files, OS junk) | Every reference template includes one. Users will add it anyway if you don't. | LOW | Embed via `go:embed`. |
| Sensible default `README.md` with project name + commands | The first file the user opens. Empty or missing = looks broken. | LOW | Template: title, install, run, structure. |
| License file generation (or `--no-license`) | Every scaffolder asks. Missing = users complain. | LOW | Default `MIT`; `--license apache2/gpl/...`; `--no-license` to skip. |
| `--help` that is readable and accurate | Cobra gives this for free; fang styles it. | LOW | fang gives styled help -- table-stakes-quality. |
| Runs offline by default (embedded templates) | `cargo new`, `go mod init`, `cobra-cli init` all work offline. Network-on-first-run surprises users. | MEDIUM | `go:embed` the templates directory. |
| Non-zero exit on error with clear message | Standard CLI hygiene. | LOW | `os.Exit(1)` + `fmt.Fprintln(os.Stderr, ...)` style; or let cobra/fang handle. |
| Generated project builds with `go build` on first try | The single most common "did it work?" test. | LOW | Verified by CI in spin itself (test fixture → build → run). |
| Flag-only mode (no interactive prompts when all flags set) | Power users will scream if forced through prompts. | LOW | `--no-interactive` and detect completeness automatically. |

### Differentiators (Competitive Advantage -- the charm-flavored features)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Interactive gum prompts when flags are missing** | Onboarding path for new users who don't know the flag surface. Reinforces the "this is a charm tool" identity. | MEDIUM | Use gum as binary (`exec.LookPath("gum")`); if missing, fall back to flag-only. Default behavior: prompt when interactive TTY and flags incomplete. |
| **Fang-styled `--help` and error output** | Makes `spin --help` *feel* like a charm product. Visually distinct from `cobra-cli --help`. | LOW | Already in fang; just `fang.Execute(ctx, rootCmd)`. |
| **Lipgloss-styled progress output during scaffold** ("creating go.mod...", "embedding bubbles...") | Sets the tone: this is a *crafted* CLI, not `mkdir`-in-a-trench-coat. | MEDIUM | Use lipgloss for status lines; gate behind `--no-style` for piping. |
| **`--ai` / `--agents` flag generates `AGENTS.md`** | A growing number of users want AI assistants (Claude/Crush) to understand their new project immediately. Not done by any competitor scaffolder. | LOW-MEDIUM | Templated from project type + chosen libs; describes module path, lib list, common commands. |
| **Per-library subflags (`--bubbletea`, `--lipgloss`, ...)** | `cobra-cli` and `cargo new` give you a skeleton; `spin` lets you pre-wire *exactly* the libraries you want without post-scaffold edit. | LOW | Simple boolean flags; the flag set is the user-role grouping above. |
| **Top-level umbrella flags (`--tui`, `--cli`, `--all`)** | Single-flag on-ramp: `spin new myapp --tui` does the obvious thing. Reduces flag memorization. | LOW | Convenience over `cobra-cli`-style granularity. |
| **External template override via `--template-repo <url>`** | Power users want their own templates; companies want branded scaffolds. Already planned in PROJECT.md. | MEDIUM | `git clone --depth 1` into temp dir, copy; cache or ignore per `--no-cache`. |
| **Bundled `--template <name>` variants** (e.g. `tui-minimal`, `tui-full`, `cli-minimal`, `ssh-bubbletea`, `crush-agent`) | Lets users pick a curated, opinionated starting point without writing one. | MEDIUM | Each is a directory under `templates/`. |
| **`spin run` wraps `go run` (or `air` if `.air.toml` present)** | One tool to learn: scaffolded projects use the same `spin` verbs as the scaffolder. | LOW | Detect `.air.toml` and exec `air`; else `go run .`. |
| **`spin build` wraps `go build` → `bin/<name>`** | Consistent path; matches Go convention. | LOW | `go build -o bin/$(name) ./...`. |
| **`spin test` wraps `prism` (not `go test`)** | `prism` is the de facto parallel test runner with better output; this is a deliberate DX choice. | LOW | `prism` if installed, else `go test ./...`. |
| **`spin vet` wraps `go vet ./...`** | Convenience; consistent surface. | LOW | One-liner. |
| **`spin fmt` runs `gofumpt` then `goimports`** (falls back to `gofmt`) | Stricter formatting is the modern Go default; one command for the whole pipeline. | LOW | Detect gofumpt/goimports; fall back gracefully. |
| **Generated `.air.toml` with sensible defaults** | Hot-reload is the modern Go dev loop; users shouldn't have to write this file. | LOW | Embed a small `.air.toml`; detect in `spin run`. |
| **Generated `Taskfile.yml` (or `Makefile` as fallback)** | Task runners are standard; one canonical choice reduces cognitive load. | LOW | Default `Taskfile.yml`; `--makefile` for legacy. |
| **`go.mod` pins specific charm v2 versions** (not `@latest`) | Reproducible builds. `@latest` in a fresh template = surprise later. | LOW | Embed the version matrix in `spin` itself. |
| **No-CGO guarantee** (`CGO_ENABLED=0` builds) | Cross-compile, minimal container images. Already in PROJECT.md constraints. | LOW | All charm v2 libs are pure Go; no CGO needed. |
| **Showcase via dogfooding** (the scaffolder is built with fang + gum) | Walking the talk. Users `go install spin` and immediately see the charm aesthetic. | MEDIUM | The cost of building `spin` is the cost of demonstrating. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Online template marketplace / registry** | Sounds like a community play. | Hosting + curation + moderation burden; spin would need a backend, accounts, ratings. PROJECT.md explicitly defers. | Local `--template-repo <url>` is enough for v1. |
| **Plugin system for custom scaffolders** | Lets users extend `spin` with their own generators. | Plugin ABI stability becomes a tax on every release. Premature. | External `--template-repo` is the de facto plugin story. |
| **TUI/GUI mode for the scaffolder itself** | "Use huh to build the scaffolder!" is the obvious charm-flavored move. | Scaffolders are invoked by humans *and* scripts (CI, agents, copy-paste docs). A TUI breaks non-TTY flows. The generated app is the TUI; the scaffolder is a CLI. PROJECT.md says this explicitly. | Use `gum` (binary) for prompts -- works in TTY, gracefully no-ops in non-TTY. |
| **CI/CD pipeline generation** (GitHub Actions, etc.) | "Just one click to set up CI!" | CI is opinionated; users have strong preferences (GitHub vs GitLab vs Drone vs Buildkite). Forcing one is hostile. | Ship a reference `.github/workflows/` in templates; users copy/edit. PROJECT.md defers. |
| **Dockerfile / docker-compose generation** | Same reasoning as CI. | Same problem: opinionated, user-specific, often wrong. | Defer. |
| **Auto-updating generated projects after scaffold** | "If spin releases v0.2, can it update my old project?" | Template drift vs user edits = data loss waiting to happen. | Document that users re-scaffold or hand-merge. PROJECT.md defers. |
| **Non-charmbracelet UI frameworks** (tview, ratatui, urfave/cli) | Some users already know these. | `spin` is opinionated; PROJECT.md says no. v1 is the moment to nail the thesis. | Document that `spin` is charm-only; use the other scaffolders for those frameworks. |
| **Non-Go language scaffolds** (Rust, TS, Python) | "While you're at it..." | Each language has its own scaffolder ecosystem (`cargo new`, `npm init`, `poetry new`). Spin would be a worse version of all of them. | Stay sharp. PROJECT.md says no. |
| **Remote/cloud execution of scaffolds** | "Spin a project on a remote box!" | Adds auth, network, sandboxing. PROJECT.md defers to local-only. | Keep it local. |
| **Bespoke configuration file (e.g. `~/.spin.yaml`)** | Scaffolders often grow a config file (`cobra-cli` has `~/.cobra.yaml`). | Premature; v1 should be flag-driven. Re-introduce if multiple-flag invocations become painful. | Pure flags. Add config in v2 if needed. |
| **Built-in Git init / first commit** | "Just `git init` and commit for me!" | Git history is sacred; auto-committing on behalf of users is a footgun. | Print the suggested commands; let the user run them. |

---

## Feature Dependencies

```
spin new <name>
    └──requires──> templates/ embedded (go:embed)
                       └──requires──> go:embed wiring
                       └──requires──> template variant selection (cli | tui | ssh | custom)

flag parsing
    └──requires──> cobra + fang (already chosen)
    └──enhances──> gum prompts (when interactive TTY and flags incomplete)

gum prompts
    └──requires──> gum binary on PATH (fall back gracefully)
    └──conflicts──> non-TTY (stdin pipe, CI) -- must auto-disable

per-library flags (--bubbletea, --lipgloss, --wish, ...)
    └──requires──> template variant exists in templates/
    └──enhances──> umbrella flags (--tui, --cli, --all)

--ai / --agents → AGENTS.md
    └──requires──> project type known (--tui or --cli chosen first)
    └──requires──> library list (for "uses bubbletea v2, lipgloss v2" etc.)
    └──independent──> template (works with any template)

--template-repo
    └──requires──> external git available OR tarball download
    └──conflicts──> network access (must fail gracefully offline)

spin run
    └──requires──> in scaffolded project (subcommand, not used in scaffolder dir)
    └──enhances──> .air.toml (auto-detect; use air when present)

spin build
    └──requires──> CGO_ENABLED=0 policy (no CGO deps in templates)

spin test
    └──requires──> prism OR fall back to go test

spin fmt
    └──requires──> gofumpt (fall back to gofmt)
    └──requires──> goimports (fall back to nothing if missing)

templates
    └──requires──> pinned charm v2 versions (version matrix in spin source)
    └──requires──> no CGO deps
    └──requires──> builds cleanly with `go build` (CI-verified)
```

### Dependency Notes

- **`spin new` requires embedded templates:** No network on first run; no surprises; works in air-gapped environments.
- **Gum prompts enhance flag parsing, not replace it:** Flags always win; gum is the "what's missing?" fallback. Critical: gum must be a graceful no-op in non-TTY.
- **`--ai` requires project type:** AGENTS.md content depends on whether it's a TUI app or CLI tool. Order of prompting must be: type → libs → ai.
- **`--template-repo` conflicts with embedded:** When external repo is provided, embedded templates are ignored for that invocation. Two different code paths in the scaffolder.
- **Templates require pinned versions:** The scaffolder owns a version matrix (e.g. `bubbletea@v2.0.0`, `lipgloss@v2.0.0`) that is updated at spin release time. No `@latest` floats.

---

## MVP Definition

### Launch With (v1) -- everything in PROJECT.md Active

These are non-negotiable. `spin new myapp --tui --bubbletea --ai` must work, end-to-end, on first try.

- [ ] `spin new <name>` with project-name validation
- [ ] Top-level umbrella flags: `--tui`, `--cli`, `--all` (plus `--ssh` orthogonal)
- [ ] Per-library subflags for charm v2 libs (bubbletea, bubbles, huh, lipgloss, ansi, runewidth, modifiers, glamour, wish, log, harmonica, glamour)
- [ ] `--cobra` (default on) and `--fang` (default on) for CLI projects; `--viper` opt-in
- [ ] `--template <name>` (bundled variants: `tui-minimal`, `tui-bubbletea`, `cli-cobra-fang`, `ssh-bubbletea`, `crush-agent`)
- [ ] `--template-repo <url>` external override
- [ ] `--ai` / `--agents` generates `AGENTS.md`
- [ ] `spin run` (auto-detect air)
- [ ] `spin build` (→ `bin/`)
- [ ] `spin test` (prism with go test fallback)
- [ ] `spin vet` (wraps `go vet`)
- [ ] `spin fmt` (gofumpt + goimports, with fallbacks)
- [ ] Interactive gum prompts (TTY only, flag-driven otherwise)
- [ ] Generated project: `.air.toml`, `Taskfile.yml`, `go.mod` with pinned v2 deps, working example
- [ ] Fang-styled `--help` and error output (dogfooded)
- [ ] Embedded templates via `go:embed` (offline default)
- [ ] No-CGO guarantee (`CGO_ENABLED=0` builds clean)
- [ ] `--license` and `--module` (so users can self-identify the project)
- [ ] `--no-interactive` to force flag-only

### Add After Validation (v1.x)

Add once the core scaffold is trusted and the first wave of users has run it.

- [ ] `--update` re-applies non-conflicting changes from a template to an existing project (after settling on a merge strategy)
- [ ] `~/.spin.yaml` config (if flag fatigue is real)
- [ ] `spin add <lib>` adds a library to an existing project (post-scaffold dependency injection)
- [ ] `spin doctor` checks generated project for common issues (outdated deps, missing .air.toml, etc.)
- [ ] VHS tape for the README demo (since `vhs` is a charm product)
- [ ] GitHub Action to run `spin` in CI (use `spin` itself to verify the templates compile)

### Future Consideration (v2+)

Defer until spin has product-market fit.

- [ ] Template marketplace / registry (hosted or self-hosted)
- [ ] Plugin system (custom scaffolder types)
- [ ] CI generation as opt-in
- [ ] Dockerfile generation as opt-in
- [ ] Multi-language support (Rust, TS, Python) -- likely never; spin is the *charm* scaffolder
- [ ] Cloud execution / `spin deploy`
- [ ] Auto-update mechanism (use Go's `go install`; consider `charm.land`-style)
- [ ] TUI mode for the scaffolder itself (anti-feature in v1; revisit only if gum prompts prove insufficient)

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| `spin new <name>` with go.mod | HIGH | LOW | P1 |
| Per-library subflags + umbrella flags | HIGH | LOW | P1 |
| Cobra + fang + gum in scaffolder itself | HIGH | MEDIUM | P1 |
| Bundled templates (tui, cli, ssh, crush) | HIGH | MEDIUM | P1 |
| Embedded templates (offline) | HIGH | LOW | P1 |
| Working example per template (compiles, runs) | HIGH | MEDIUM | P1 |
| `.air.toml` + `Taskfile.yml` generated | HIGH | LOW | P1 |
| `spin run` / `build` / `test` / `vet` / `fmt` | HIGH | LOW | P1 |
| Interactive gum prompts (TTY-aware) | MEDIUM | MEDIUM | P1 |
| `--ai` / `AGENTS.md` | MEDIUM | LOW | P1 |
| `--template-repo <url>` | MEDIUM | MEDIUM | P1 |
| Fang-styled help/error | MEDIUM | LOW | P1 |
| License + module override | MEDIUM | LOW | P1 |
| `--no-interactive` | MEDIUM | LOW | P1 |
| `~/.spin.yaml` config | LOW | LOW | P3 |
| `spin add <lib>` post-scaffold | MEDIUM | HIGH | P2 |
| VHS demo tape | LOW | LOW | P2 |
| `spin doctor` | MEDIUM | MEDIUM | P2 |
| Template marketplace | LOW (until v2) | HIGH | P3 |
| Plugin system | LOW (until v2) | HIGH | P3 |
| CI/Dockerfile generation | LOW | MEDIUM | P3 (anti-feature for v1) |
| TUI mode for scaffolder | LOW | MEDIUM | Anti (out of scope) |
| Multi-language scaffolds | LOW | HIGH | Anti (out of scope) |

**Priority key:**
- P1: Must have for v1 launch
- P2: Should have, add in v1.x after validation
- P3: Nice to have, future consideration (or never)

---

## Competitor Feature Analysis

| Feature | `cargo new` | `npm init` | `cobra-cli init` | `cookiecutter` | `spring initializr` | `spin` (planned) |
|---------|-------------|------------|------------------|----------------|---------------------|------------------|
| Generates a runnable project | yes | yes | yes (cobra only) | yes (templated) | yes | yes (templated, opinionated) |
| Flag-only mode | yes | yes | yes | yes | yes (URL-encoded flags) | yes |
| Interactive prompts | no (uses defaults) | yes (wizard) | no | no (uses prompts.json) | yes (web UI / CLI flag builder) | yes (gum, TTY-aware) |
| Multiple templates / variants | no | no | no | yes (any) | yes (Spring Starters) | yes (bundled + `--template-repo`) |
| Per-library flags (add libs at scaffold time) | no | no | no | no | yes (Spring Starters) | yes (the differentiator) |
| Wraps `go build` / `go test` etc. | no | no | no | no | no | yes (`spin run`/`build`/`test`/`vet`/`fmt`) |
| AI-assistant context (`AGENTS.md`) | no | no | no | no | no | yes (opt-in `--ai`) |
| External template override | no | no | no | yes (point at any repo) | no | yes (`--template-repo`) |
| Offline by default | yes | yes | yes | yes | no (requires server) | yes (embedded) |
| Hot-reload (air / similar) wired | no | no | no | no | yes (spring-boot-devtools) | yes (`.air.toml` + `spin run` detects it) |
| Formatter wraps stricter-than-default tool | no | no | no | no | no | yes (`gofumpt` via `spin fmt`) |
| Colored/styled output (charm-flavored) | n/a | n/a | no | no | no | yes (fang + lipgloss) |
| **Friction to first `go run` success** | 1 cmd | 1 cmd + install | 1 cmd | 1 cmd + prompts.json | 1 cmd (or web) | 1 cmd (`spin new myapp --tui --bubbletea`) |

**Positioning:** `spin` is to Go what `spring initializr` is to Java -- opinionated, library-aware, gives you a runnable starting point. But unlike Spring Initializr, `spin` is local-first, has zero server cost, and dogfoods the same stack it scaffolds. The closest direct competitor is `cobra-cli init`, but that one is CLI-only and one-shape-fits-all; `spin` is multi-shape (TUI/CLI/SSH) and library-aware.

---

## Sources

**Verified via Context7 (HIGH confidence):**
- [charmbracelet/bubbletea -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import path `charm.land/bubbletea/v2`
- [charmbracelet/lipgloss -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/lipgloss/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import path `charm.land/lipgloss/v2`
- [charmbracelet/bubbles -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import paths for all 14 component subpackages
- [charmbracelet/huh -- README.md](https://github.com/charmbracelet/huh/blob/main/README.md) -- confirmed v2 import path `charm.land/huh/v2`
- [charmbracelet/wish -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/wish/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import path `charm.land/wish/v2` and middleware subpaths
- [charmbracelet/log -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/log/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import path `charm.land/log/v2`
- [charmbracelet/glamour -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/glamour/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import path `charm.land/glamour/v2`
- [charmbracelet/fang -- UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/fang/blob/main/UPGRADE_GUIDE_V2.md) -- confirmed v2 import path `charm.land/fang/v2` and `fang.Execute(ctx, cmd)` usage
- [charmbracelet/bubbletea-app-template -- README & llms.txt](https://github.com/charmbracelet/bubbletea-app-template) -- confirmed reference project structure: `.github/workflows/build.yml`, `.github/workflows/release.yml` (GoReleaser), `.golangci.yml` (thelper, gofumpt, tparallel, unconvert, unparam, wastedassign), `.goreleaser.yaml` (CGO_ENABLED=0, `go_first_class` targets, changelog groups), `.github/dependabot.yml` (gomod + github-actions weekly groups)
- [charmbracelet/gum -- README.md](https://github.com/charmbracelet/gum/blob/main/README.md) -- confirmed `gum input`, `gum confirm`, `gum choose` for interactive prompts
- [charmbracelet/x -- llms.txt](https://context7.com/charmbracelet/x/llms.txt) -- confirmed `ansi`, `modifiers`, `runewidth` packages under `github.com/charmbracelet/x`
- [spf13/cobra -- README.md & user_guide.md](https://github.com/spf13/cobra) -- confirmed `cobra-cli` scaffolder structure (`init`, `add`, `--author`, `--license`, `--viper`); no v2 path for cobra itself

**Other (MEDIUM confidence):**
- [charm.land/](https://charm.land/) -- official charmbracelet landing; lists all major libraries (Bubble Tea, Lip Gloss, Bubbles, Huh, Wish, Log, Glamour, Harmonica, Glow, gum, Crush, Mods, Skate)
- [github.com/charmbracelet](https://github.com/charmbracelet) -- confirmed 54 repos in org; pinned repos include bubbletea (42.8k), lipgloss (11.4k), bubbles (8.4k), huh (6.9k), wish (5.2k), freeze (4.6k), vhs (19.8k), glow (25.6k), gum (23.8k), crush (24.9k), catwalk (725), fantasy (786)
- [start.spring.io](https://start.spring.io/) -- Spring Initializr, the closest analog to `spin`'s per-library flag concept (Spring Starters)
- [github.com/spf13/cobra-cli](https://github.com/spf13/cobra-cli) -- competitor analysis: subcommand scaffolder for cobra projects

**Inferred from PROJECT.md (authoritative for scope):**
- All Out of Scope items in PROJECT.md (online registry, plugin system, auto-update, CI generation, Dockerfile, remote execution, TUI mode for scaffolder, non-charm frameworks, non-Go languages) confirmed as anti-features.
- All Active requirements in PROJECT.md mapped to P1 features in MVP.

---

*Feature research for: spin -- charmbracelet v2 Go scaffold CLI*
*Researched: 2026-06-02*
