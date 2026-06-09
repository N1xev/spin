# spin

## What This Is

`spin` is a universal, language-agnostic scaffolder and task runner. One CLI for both jobs:

- **Scaffolder** — `spin new <ecosystem> <name> [flags]` generates ready-to-run projects for any ecosystem (Go+charm, Rust, or anything with a template). `spin new <name> --template <user/repo>` pulls an external template (git repo) that declares its own params and post-hooks.
- **Task runner** — `spin run <task>` resolves tasks from `spin.config.toml`, `Taskfile.yml`, `Makefile`, `package.json`, `scripts/`, or a language-aware fallback. `--list` shows the merged list with sources; `--explain <task>` shows origin + raw command.
- **Discovery** — `spin search <query>`, `spin add <user/repo>`, `spin list` against a public registry of templates and ecosystems (server ships separately).

The first two ecosystems are **charm** (Go + charmbracelet v2) and **rust** (cargo). The first-class citizen is the ecosystem; the second-class citizen is the template; the third is the builder (deferred to v2.x). Built with cobra + fang + gum so the tool itself dogfoods the charm stack.

**Core Value:** One tool to scaffold any project and run its tasks, for any language — `spin new rust myapp --bin && spin run build && spin run test` works the same as `spin new charm myapp --tui --bubbletea && spin run build && spin run test`.

## Requirements

### Validated

- [x] `spin new <name>` scaffolds a Go+charmbracelet v2 project in `./<name>`
- [x] Top-level project-type flags: `--tui`, `--cli`, `--all`
- [x] Per-library subflags for charmbracelet libs (`--bubbletea`, `--lipgloss`, `--huh`, `--glow`, `--glamour`, `--wish`, `--log`, `--crush`, `--modifiers`, `--ansi`, `--runewidth`)
- [x] `--cobra` (default on) and `--fang` (default on) for CLI projects; `--viper` opt-in for config
- [x] `--template <name>` selects a bundled template variant
- [x] `--template-repo <url>` overrides the embedded template with an external git repo
- [x] `--ai` (or `--agents`) generates an `AGENTS.md` describing the project for AI assistants
- [x] `spin run` — runs the project (uses `air` for hot reload if `.air.toml` present)
- [x] `spin build` — builds binary to `bin/`
- [x] `spin test` — runs `prism` instead of bare `go test`
- [x] `spin vet` — wraps `go vet` (whole module)
- [x] `spin fmt` — wraps `gofumpt` (or `go fmt` if gofumpt unavailable) + `goimports`
- [x] Interactive prompts (gum) when flags are missing, asking project type / libs / template / AI
- [x] Generated project includes `.air.toml`, `Taskfile.yml` or `Makefile`, `go.mod` with pinned charm v2 deps
- [x] Generated project ships a working example (bubbletea "hello" TUI or cobra/fang "hello" CLI)
- [x] `spin doctor`, `spin lint`, `spin update` (post-scaffold health)
- [x] CI dogfooding — spin rebuilds itself in CI
- [x] v2.0 skeleton — ecosystems, templates, runner, registry, builder stub (compiled, all CLI surfaces working)
- [x] **Phase 5 — v2.0 Universal Scaffolder & Task Runner** (validated 2026-06-09, 36/36 must-haves, all 5 success criteria)

Ecosystem model:
- [x] `spin new <ecosystem> <name> [flags]` dispatches to a compiled-in ecosystem (charm, rust)
- [x] `spin ecosystem {list,info}` — discover and inspect ecosystems
- [x] `spin new --list-ecosystems` — quick ecosystem listing
- [x] Charm migrated to the new flow; `spin new <name>` (no ecosystem) keeps working with a deprecation notice
- [x] Rust ecosystem: `cargo new` (binary, lib, example), cargo-aware `spin run` fallbacks
- [x] Each ecosystem declares its own flags and tasks; the runner merges them with the source chain

Template model:
- [x] `spin new <name> --template <user/repo>` clones a template (shallow, `GIT_TERMINAL_PROMPT=0`)
- [x] `spin.toml` declares metadata + params (text/textarea/number/select/multiselect/bool/path/secret) + post-hooks
- [x] huh v2 form from params when TTY; defaults in non-TTY
- [x] `spin.toml` is deleted after render; post-hooks run on success
- [x] Templates are path-traversal-safe

Runner:
- [x] `spin run <task>` resolves from `spin.config.toml` first
- [x] Fallback chain: spin.config → Taskfile → Makefile → package.json → scripts/ → language default
- [x] `--list` shows merged tasks with source labels
- [x] `--explain <task>` shows origin + command
- [x] Language fallbacks for go (build/test/run/vet/fmt) and rust (build/test/run/clippy/fmt)

Registry:
- [x] `spin search <query>` against hosted registry; graceful "not deployed" message when server unreachable
- [x] `spin add <user/repo>` pins to `~/.config/spin/pinned.json`
- [x] `spin list` shows pinned entries with local paths
- [x] `SPIN_REGISTRY_URL` env override

Backward compat:
- [x] All v1.0 commands and flags still work
- [x] `spin new <name>` defaults to charm with a one-time deprecation notice
- [x] `spin build/test/vet/fmt/lint` print deprecation notice suggesting `spin run <task>` but still execute

### Active

*All v2.0 milestone requirements are now validated. The next milestone is v2.x, focused on the deferred items below.*

### Out of Scope

- Non-charm Go TUI frameworks (tview, ratatui) — `spin` is opinionated about charm (for the charm ecosystem)
- `Builder` concept (question tree with custom renderers) — v2.x; skeleton ships the interface
- Tauri/Next/Nuxt/Vue/React/Flutter/Dart/C#/Java ecosystems — v2.x
- External ecosystem loading (Go plugins) — v2.x; v2.0 is compiled-in
- `spin workspace` / `go.work` management — v2.x
- GUI/TUI mode for the scaffolder itself (TUI is for the generated project, not the scaffolder)
- `spin release` wrapping goreleaser — defer
- Dockerfile/compose generation — out of scope

## Context

- The charmbracelet v2 ecosystem (bubbletea, lipgloss, huh, bubbles, glamour, wish, log, fang) uses `charm.land/<lib>/v2` import paths as of 2024–2025
- `gum` is a binary (no Go library) — `spin` shells out to it for interactive prompts
- `huh` v2 is the in-process form backend — used as the fallback when `gum` is not on `$PATH` and the in-process form for the runner's update/dep picker
- The charm ecosystem is the first citizen; rust is the second; everything else is a template
- The user wants a "global and universal language- and ecosystem-agnostic scaffolder and task runner" — competing with `npx create-*` (per-template scaffolders) and `cargo`/`make`/`task` (per-tool task runners), with one CLI to do both
- Rust is a critical second ecosystem: it's the most-Go-like language, has a strong task model (`cargo run/build/test/clippy/fmt`), and proves the universal claim
- The registry server is a separate project (`spin-registry`) — `spin` ships the client, not the server
- The runner's source-precedence chain mirrors Task's and Just's: explicit project config wins; language defaults are the floor
- Template params (text/number/select/multiselect/bool/path/secret) cover the 80% case; huh v2 supports all of them; for the 20% case, templates can do their own prompting in a post-hook
- The v2.0 skeleton (built 2026-06-08) defines the package layout: `internal/params/`, `internal/ecosystem/`, `internal/ecosystems/{charm,rust,...}`, `internal/runner/`, `internal/runner/sources/`, `internal/template/`, `internal/registry/`, `internal/builder/`. This phase fills in the implementations.

## Constraints

- **Tech stack**: Go 1.23+ (use 1.25 for scaffolded projects that need bubbles v2); cobra + fang + gum; charm v2 only for spin itself — Why: dogfooding, modern Go
- **Distribution**: single static binary; `go install github.com/<org>/spin@latest` — Why: standard Go CLI
- **No CGO**: spin itself builds with `CGO_ENABLED=0`; scaffolded Go projects also CGO=0 — Why: cross-compile, minimal containers
- **Compiled-in ecosystems (v2.0)**: charm + rust are Go packages in `internal/ecosystems/` — Why: simpler ABI, no plugin contract to maintain
- **External templates via git**: shallow clone, `GIT_TERMINAL_PROMPT=0` — Why: works without auth for public repos; depth-1 keeps it fast
- **Graceful degradation**: registry server not deployed → friendly message, not a stack trace — Why: don't block users on the registry MVP
- **Charm v2 only**: never import `github.com/charmbracelet/...` v1 paths in spin or in scaffolded projects — Why: v1 deprecated

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|--------|
| Binary name = `spin` | User-selected; short verb, evokes "spinning up" a project | — Validated |
| Templates embedded + override (`--template-repo`) | Offline default; flexibility for power users | — Validated v1 |
| Three concepts: Ecosystem / Template / Builder | Ecosystem = compiled-in language; Template = external git; Builder = question tree | — Locked v2 |
| Charm is the first ecosystem | Dogfoods the charm stack; mature v2 libs | — Locked v2 |
| Rust is the second ecosystem | Proves universality; most-Go-like; strong task model | — Locked v2 |
| `spin.config.toml` for project tasks | User-owned; overrides language defaults | — Locked v2 |
| Runner source precedence: spin.config → Taskfile → Makefile → package.json → scripts/ → lang default | Explicit > auto-detected; per-project wins | — Locked v2 |
| `spin.toml` for template params; deleted after use | Templates self-describe; project never carries scaffolder config | — Locked v2 |
| 7 param types: text, textarea, number, select, multiselect, bool, path, secret | Covers 80% of template questions; huh v2 supports all of them | — Locked v2 |
| Registry client + server are separate | Server is its own project; client degrades gracefully | — Locked v2 |
| Builder concept is interface-only in v2.0 | Avoid premature design; ship the slot | — Deferred v2.x |
| `spin new <name>` defaults to charm | Backward compat with v1; one-time deprecation notice | — Locked v2 |
| Deprecation warnings on `spin build/test/vet/fmt/lint` | Backward compat for v1 users; suggest `spin run <task>` | — Locked v2 |
| v1 Go+charm project tree is unchanged | The legacy v2.0 charm ecosystem wraps the existing `scaffold.Project` | — Locked v2 |

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
*Last updated: 2026-06-09 after Phase 5 execution (v2.0 milestone complete)*
