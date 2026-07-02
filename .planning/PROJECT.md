# spin

## What This Is

`spin` is a universal, language-agnostic scaffolder. Projects are produced from **templates** -- git repos (or local paths) that contain a `spin.toml` manifest, a `_base/` tree of overlay files, and an optional `_post/` hook.

- **Scaffolder** -- `spin new <name> --template <spec>` loads a template (git URL, local path, or pinned), prompts for the template's declared params (huh v2 in TTY, defaults in non-TTY), renders `_base/` against the resolved values via `text/template`, and runs the post-hook.
- **Discovery** -- `spin search <query>`, `spin add <spec>`, `spin list` against a public registry of templates (server ships separately).

There is no `spin run` / task runner in v2.x. Each template ships its own build/test/run configuration (e.g. `Taskfile.yml`, `Makefile`, `package.json` scripts); users invoke those tools directly. Whether to bring back the task runner is deferred until the template ecosystem shows real demand.

The v2.0 milestone shipped an Ecosystem system (charm, rust) and a task runner; both were archived after v2.0 because templates proved a simpler unit of distribution. See **Evolution** below for the pivot details.

**Core Value:** Scaffold a ready-to-run project for any language, lib, or framework from a single template spec -- `spin new myapp --template <user/repo>` produces a project that builds and runs cleanly on first try.

## Current Milestone: v2.x local-registry

**Goal:** Replace the HTTP-based registry stub (`https://registry.spin.invalid/v1`) with a zero-backend git/local registry model. Registries are cloned (git) or symlinked (local) directories containing `registry.toml` + `templates/*.toml`; `spin search` reads them directly from disk.

**Target features:**
- `spin registry add <alias> <source>` — register a git URL or local path as a registry
- `spin registry list | update [alias] | remove <alias>` — manage registered registries
- `spin search <query>` — local-only, scans every `~/.config/spin/registries/*/templates/*.toml`
- `<alias>/<id>` shorthand accepted by `spin add` and `spin new`, resolved via the local index
- Storage layout: `~/.config/spin/registries.json` (config) + `~/.config/spin/registries/<alias>/` (clone/symlink)

**Phases:**
- Phase 6 (A): manager + `spin registry` CLI + `registries.json`
- Phase 7 (B): index reader + `<alias>/<id>` resolver + rewire `search`/`add`/`new`/`loader`
- Phase 8 (C): delete HTTP client code + docs pass

**Constraints carried forward:**
- `pinned.json` format unchanged — every existing pin keeps working
- No new deps needed; reuse `github.com/BurntSushi/toml` for registry metadata
- Drop `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` env vars; drop `ErrNotDeployed` / `DefaultIndexURL`

## v2.x Pivot (2026-06-10)

After Phase 5 (v2.0 Universal Scaffolder & Task Runner) completed, the user reviewed the design and chose to focus on the scaffolding layer first. The full v2 ecosystem system and task runner were archived to `~/Projects/Golang/spin-ecosys-tasks-archieve/` so the codebase can ship and validate templates before committing to the bigger architecture.

**Archived (no longer in `spin/`):**
- `internal/ecosystem/`, `internal/ecosystems/{charm,rust}/` -- Ecosystem interface and implementations
- `internal/runner/`, `internal/runner/sources/` -- universal task runner
- `cmd/ecosystem.go`, `cmd/new_charm.go`, `cmd/new_rust.go`, `cmd/run.go` -- their CLI wiring

**Retained in `spin/`:**
- `internal/template/` -- the v2 template engine (foundation of the new direction)
- `internal/registry/` -- public template registry client
- `internal/scaffold/` -- v1 charm-flavoured engine (deprecated, will be removed)
- `internal/{params,prompt,update,version}/` -- supporting packages
- `cmd/{root,new,add,list,search,update,version}.go` -- CLI surface (rewritten where it touched archived code)

**New direction (v2.x):**
- Templates are the sole extension surface. A template is a git repo (or local path) with `spin.toml` + `_base/` + optional `_post/`.
- Ecosystem and task runner concepts are deferred to future milestones -- they will be reconsidered when real template usage reveals the need.
- v2.x success criterion: 2-3 real templates (Go CLI, Python CLI, Rust CLI) shipped, all running on first try via `spin new <name> --template <user/repo>`.
- The registry design is downstream of "what templates actually look like" -- designed after the first real templates ship.

## Requirements

### Validated

- [x] `spin new <name>` scaffolds a Go+charmbracelet v2 project in `./<name>`
- [x] Top-level project-type flags: `--tui`, `--cli`, `--all`
- [x] Per-library subflags for charmbracelet libs (`--bubbletea`, `--lipgloss`, `--huh`, `--glow`, `--glamour`, `--wish`, `--log`, `--crush`, `--modifiers`, `--ansi`, `--runewidth`)
- [x] `--cobra` (default on) and `--fang` (default on) for CLI projects; `--viper` opt-in for config
- [x] `--template <name>` selects a bundled template variant
- [x] `--template-repo <url>` overrides the embedded template with an external git repo
- [x] `--ai` (or `--agents`) generates an `AGENTS.md` describing the project for AI assistants
- [x] `spin run` -- runs the project (uses `air` for hot reload if `.air.toml` present)
- [x] `spin build` -- builds binary to `bin/`
- [x] `spin test` -- runs `prism` instead of bare `go test`
- [x] `spin vet` -- wraps `go vet` (whole module)
- [x] `spin fmt` -- wraps `gofumpt` (or `go fmt` if gofumpt unavailable) + `goimports`
- [x] Interactive prompts (gum) when flags are missing, asking project type / libs / template / AI
- [x] Generated project includes `.air.toml`, `Taskfile.yml` or `Makefile`, `go.mod` with pinned charm v2 deps
- [x] Generated project ships a working example (bubbletea "hello" TUI or cobra/fang "hello" CLI)
- [x] `spin doctor`, `spin lint`, `spin update` (post-scaffold health)
- [x] CI dogfooding -- spin rebuilds itself in CI
- [x] v2.0 skeleton -- ecosystems, templates, runner, registry, builder stub (compiled, all CLI surfaces working)
- [x] **Phase 5 -- v2.0 Universal Scaffolder & Task Runner** (validated 2026-06-09, 36/36 must-haves, all 5 success criteria)

Ecosystem model:
- [x] `spin new <ecosystem> <name> [flags]` dispatches to a compiled-in ecosystem (charm, rust)
- [x] `spin ecosystem {list,info}` -- discover and inspect ecosystems
- [x] `spin new --list-ecosystems` -- quick ecosystem listing
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

- Non-charm Go TUI frameworks (tview, ratatui) -- `spin` is opinionated about charm (for the charm ecosystem)
- `Builder` concept (question tree with custom renderers) -- v2.x; skeleton ships the interface
- Tauri/Next/Nuxt/Vue/React/Flutter/Dart/C#/Java ecosystems -- v2.x
- External ecosystem loading (Go plugins) -- v2.x; v2.0 is compiled-in
- `spin workspace` / `go.work` management -- v2.x
- GUI/TUI mode for the scaffolder itself (TUI is for the generated project, not the scaffolder)
- `spin release` wrapping goreleaser -- defer
- Dockerfile/compose generation -- out of scope

## Context

- The charmbracelet v2 ecosystem (bubbletea, lipgloss, huh, bubbles, glamour, wish, log, fang) uses `charm.land/<lib>/v2` import paths as of 2024–2025
- `gum` is a binary (no Go library) -- `spin` shells out to it for interactive prompts
- `huh` v2 is the in-process form backend -- used as the fallback when `gum` is not on `$PATH` and the in-process form for the runner's update/dep picker
- The charm ecosystem is the first citizen; rust is the second; everything else is a template
- The user wants a "global and universal language- and ecosystem-agnostic scaffolder and task runner" -- competing with `npx create-*` (per-template scaffolders) and `cargo`/`make`/`task` (per-tool task runners), with one CLI to do both
- Rust is a critical second ecosystem: it's the most-Go-like language, has a strong task model (`cargo run/build/test/clippy/fmt`), and proves the universal claim
- The registry server is a separate project (`spin-registry`) -- `spin` ships the client, not the server
- The runner's source-precedence chain mirrors Task's and Just's: explicit project config wins; language defaults are the floor
- Template params (text/number/select/multiselect/bool/path/secret) cover the 80% case; huh v2 supports all of them; for the 20% case, templates can do their own prompting in a post-hook
- The v2.0 skeleton (built 2026-06-08) defines the package layout: `internal/params/`, `internal/ecosystem/`, `internal/ecosystems/{charm,rust,...}`, `internal/runner/`, `internal/runner/sources/`, `internal/template/`, `internal/registry/`, `internal/builder/`. This phase fills in the implementations.

## Constraints

- **Tech stack**: Go 1.23+ (use 1.25 for scaffolded projects that need bubbles v2); cobra + fang + gum; charm v2 only for spin itself -- Why: dogfooding, modern Go
- **Distribution**: single static binary; `go install github.com/<org>/spin@latest` -- Why: standard Go CLI
- **No CGO**: spin itself builds with `CGO_ENABLED=0`; scaffolded Go projects also CGO=0 -- Why: cross-compile, minimal containers
- **Compiled-in ecosystems (v2.0)**: charm + rust are Go packages in `internal/ecosystems/` -- Why: simpler ABI, no plugin contract to maintain
- **External templates via git**: shallow clone, `GIT_TERMINAL_PROMPT=0` -- Why: works without auth for public repos; depth-1 keeps it fast
- **Graceful degradation**: registry server not deployed → friendly message, not a stack trace -- Why: don't block users on the registry MVP
- **Charm v2 only**: never import `github.com/charmbracelet/...` v1 paths in spin or in scaffolded projects -- Why: v1 deprecated

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|--------|
| Binary name = `spin` | User-selected; short verb, evokes "spinning up" a project | -- Validated |
| Templates embedded + override (`--template-repo`) | Offline default; flexibility for power users | -- Validated v1 |
| Three concepts: Ecosystem / Template / Builder | Ecosystem = compiled-in language; Template = external git; Builder = question tree | -- Locked v2 |
| Charm is the first ecosystem | Dogfoods the charm stack; mature v2 libs | -- Locked v2 |
| Rust is the second ecosystem | Proves universality; most-Go-like; strong task model | -- Locked v2 |
| `spin.config.toml` for project tasks | User-owned; overrides language defaults | -- Locked v2 |
| Runner source precedence: spin.config → Taskfile → Makefile → package.json → scripts/ → lang default | Explicit > auto-detected; per-project wins | -- Locked v2 |
| `spin.toml` for template params; deleted after use | Templates self-describe; project never carries scaffolder config | -- Locked v2 |
| 7 param types: text, textarea, number, select, multiselect, bool, path, secret | Covers 80% of template questions; huh v2 supports all of them | -- Locked v2 |
| Registry client + server are separate | Server is its own project; client degrades gracefully | -- Locked v2 |
| Builder concept is interface-only in v2.0 | Avoid premature design; ship the slot | -- Deferred v2.x |
| `spin new <name>` defaults to charm | Backward compat with v1; one-time deprecation notice | -- Locked v2 |
| Deprecation warnings on `spin build/test/vet/fmt/lint` | Backward compat for v1 users; suggest `spin run <task>` | -- Locked v2 |
| v1 Go+charm project tree is unchanged | The legacy v2.0 charm ecosystem wraps the existing `scaffold.Project` | -- Locked v2 |

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
2. Core Value check -- still the right priority?
3. Audit Out of Scope -- reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-03 -- v2.x local-registry milestone Phases 6-8 complete (manager + index + resolver + HTTP cleanup)*

