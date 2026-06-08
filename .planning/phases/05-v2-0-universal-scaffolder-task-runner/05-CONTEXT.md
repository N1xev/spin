# Phase 5: v2.0 Universal Scaffolder & Task Runner - Context

**Gathered:** 2026-06-08
**Status:** Ready for planning
**Source:** Conversation context (the v2.0 vision pivot)

<domain>
## Phase Boundary

Phase 5 fills in the v2.0 skeleton that was built on 2026-06-08. The skeleton defines the package layout, interfaces, and CLI surface; this phase delivers the implementation, the migrations, and the second ecosystem (rust) that proves universality.

**In scope:**
- Filling in the ecosystem interface — full Validate/Render/PostScaffold/Tasks implementations
- Migrating the v1 charm scaffolder to be a first-class citizen in the new flow (no behavior change for `spin new <name> --tui --bubbletea`)
- Adding a second ecosystem (rust) that wires `cargo build/test/run/clippy/fmt` through `spin run`
- Filling in the external template loader: git clone, spin.toml parse, huh form, render, post-hooks, delete spin.toml
- Filling in `spin.config.toml` + the source-precedence chain (spin.config → Taskfile → Makefile → package.json → scripts/ → language fallback)
- Filling in the registry client: real HTTP search with graceful degradation; pin/add/list

**Out of scope (v2.x or later):**
- External ecosystem loading (Go plugins)
- The `Builder` concept (question tree with custom renderers) — interface stub only
- Hosted registry server (separate project)
- Tauri/Next/Nuxt/Vue/React/Flutter/Dart/C#/Java ecosystems
- TUI mode for the scaffolder itself
- `spin workspace` / `go.work` management

</domain>

<decisions>
## Implementation Decisions

### A. Architecture — three concepts, locked

Three concepts differentiate v2.0 from v1.0:

- **Ecosystem** — a compiled-in Go package under `internal/ecosystems/<name>/` that implements the `Ecosystem` interface. Knows how to scaffold for one language/stack. Charm and rust are the first two. External loading (plugins) is v2.x.
- **Template** — an external git repo with `spin.toml` + `_base/` + overlays + post-hooks. Loaded by the template package. Self-describes via `spin.toml`.
- **Builder** — a question-tree renderer with custom renderers per node. Interface stub only in v2.0; full implementation is v2.x.

The user explicitly wants all three in the long-term vision, with ecosystems as first-class citizens (sponsor model), templates as second-class, and builders as third-class.

### B. Task runner source precedence — locked

`spin run <task>` resolves from the highest-precedence source first:

1. `spin.config.toml` `[tasks]` block (user-authored, wins)
2. `Taskfile.yml` (go-task v3 schema)
3. `Makefile` targets
4. `package.json` `scripts`
5. `scripts/` directory
6. Language ecosystem fallback (go: build/test/run/vet/fmt; rust: build/test/run/clippy/fmt)

This is the same precedence model as Task/Just/Make. Explicit project config wins; per-language defaults are the floor.

`spin.config.toml` is simple: `[tasks] key = "shell command"` plus optional `description` and `env`. Lives in the project root.

### C. Template params — 7+ types, huh v2 backend — locked

7 user-facing types per the conversation, plus 1 bonus (textarea):

| Type | huh v2 field |
|------|--------------|
| `text` | `huh.NewInput()` |
| `textarea` | `huh.NewText()` |
| `number` | `huh.NewInput()` + `Validate(int)` |
| `select` | `huh.NewSelect[T]()` |
| `multiselect` | `huh.NewMultiSelect[T]()` |
| `bool` | `huh.NewConfirm()` |
| `path` | `huh.NewFilePicker()` |
| `secret` | `huh.NewInput()` + `EchoMode(EchoModePassword)` |

When stdin is a TTY, the params are presented as a huh form. In non-TTY (`--no-interactive`), defaults are applied silently. `spin.toml` is deleted from the output after a successful render.

### D. Charm as first-class ecosystem — locked

`spin new <name>` (no ecosystem) must keep working for v1 users. In v2.0 it routes to `charm` with a one-time deprecation notice. The new canonical form is `spin new charm <name> --tui --bubbletea --bubbles --lipgloss`.

The charm ecosystem wraps the existing `scaffold.Project` (no rewrite of v1 templates). Conversion: `Context` → `scaffold.Project` → `RenderToMap()` → `WriteTo(dir)`. Flags declared in `internal/ecosystems/charm/flags.go` (already present in skeleton).

### E. Rust as second ecosystem — locked

Rust proves universality. The second ecosystem ships in Phase 5 because:
- It's the most-Go-like language (compiles, modules, Cargo.toml)
- It has a strong task model (`cargo build/test/run/clippy/fmt`) that maps cleanly to `spin run`
- Users will compare spin vs `cargo` directly; we need to win on DX, not feature parity

`spin new rust <name> --bin` (default) / `--lib` / `--example` produces:
- `Cargo.toml` with `name`, `version`, `edition` (default 2021), `rust-version` (default 1.75)
- `src/main.rs` (or `src/lib.rs`) with a "hello, name!" example
- `spin.config.toml` with cargo fallbacks: `build`, `test`, `run`, `clippy`, `fmt`
- `.gitignore` (target/)

The rust ecosystem's `Tasks()` returns the cargo fallbacks so `spin run` works out of the box.

### F. External template loader — locked

External templates (e.g. `github.com/charmbracelet/spin-charm-api`) work as follows:
1. `spin new <name> --template <user/repo>` resolves the template ref to a git URL
2. Shallow-clone to `~/.config/spin/templates/<user>/<repo>` (`GIT_TERMINAL_PROMPT=0`)
3. Read `spin.toml` for metadata + params + post-hooks
4. If TTY: present params as a huh form. If not: apply defaults.
5. Render `_base/` + overlays via text/template; output to `./<name>/`
6. Run post-hooks in declaration order with `set -e` semantics
7. Delete `spin.toml` from the output (already not in `_base/`, but defend against templates that include it)

Path-traversal safe: reject any output path containing `..` segments. This is already enforced in the skeleton's `template.RenderTo()`.

### G. Registry — client now, server later — locked

`spin search <query>` calls a hosted registry server. The server is a separate project; until it ships, the client must:
- Return a friendly message ("registry not yet deployed, see github.com/example/spin-registry")
- Never throw a stack trace on connection failure
- Honor `SPIN_REGISTRY_URL` env override for self-hosted registries
- Default URL: a stub that's known-unreachable so the message is shown

`spin add <user/repo>` pins to `~/.config/spin/pinned.json` (one-line JSON, atomic write). `spin list` reads it and shows the resolved local path under `~/.config/spin/templates/`.

### H. Backward compat — locked

All v1.0 commands and flags work unchanged. The v2.0 additions are additive:
- `spin new <name>` (no ecosystem) → routes to charm with a one-time deprecation notice
- `spin build/test/vet/fmt/lint` → print a deprecation notice suggesting `spin run <task>`, but still execute
- New commands: `spin run`, `spin search`, `spin add`, `spin list`, `spin ecosystem`, `spin version`

The deprecation path is implemented in `cmd/deprecate.go` (already in skeleton) as PreRun hooks on the legacy commands.

### I. New ecosystem flag model — locked

Each ecosystem declares its flags via the `Flag` type in `internal/ecosystem/flag.go` (already in skeleton):

```go
ecosystem.NewBoolFlag("tui", "tui project type").WithAliases("T").WithPrompt("TUI project?")
ecosystem.NewChoiceFlag("type", "Project type", []string{"tui", "cli", "lib"}).WithDefault("tui")
```

The CLI binds these flags dynamically via `pflag.Flag` `VisitAll` (already implemented in `cmd/new_charm.go`'s `runNewCharm`). Values land in `Context.Flags` for the renderer.

### J. Migration of the v1 CLI surface — locked

The legacy `cmd/new.go` (`spin new <name> --tui ...`) becomes a thin shim that:
1. Prints a deprecation notice (one line, stderr)
2. Constructs the same `Context` the v2.0 charm ecosystem would build
3. Calls into the charm ecosystem's `Validate → Render → PostScaffold`
4. Writes the result to `./<name>/`

The legacy `cmd/build.go` etc. just add a PreRun deprecation warning (already done in skeleton).

### K. Goals and success criteria

This phase is judged on the 5 success criteria in ROADMAP.md:
1. `spin new <name>` works for backward compat, deprecation notice visible, charm route identical to v1
2. `spin new rust <name> --bin` builds with `cargo build` and runs with `cargo run`; `spin run` invokes cargo fallbacks
3. `spin new <name> --template <ref>` clones a real template, runs the huh form, renders, post-hooks, deletes spin.toml
4. `spin run` resolves across the source precedence chain with `--list` and `--explain` showing sources
5. `spin search` returns friendly "not deployed" message when server unreachable; `spin add` and `spin list` work against `~/.config/spin/pinned.json`

### Claude's Discretion

- Exact error messages and exit codes
- TOML parser choice — the skeleton's hand-rolled mini-parser is fine; full `encoding/toml/v2` is fine too if it lands cleaner
- Huh field keybindings for each param type
- Default cargo edition / rust-version (suggest 2021 / 1.75)
- Naming of internal helper functions in each ecosystem
- Whether the rust ecosystem's task fallbacks live in a constant in `internal/runner/sources/fallback.go` (cargo branch) or in the ecosystem's `Tasks()` method (preferred — keeps language-specific defaults in the ecosystem)
- Test coverage strategy: prefer `go test ./...` for unit tests, integration tests for end-to-end (`go run` a generated project)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### v2.0 skeleton (just built)
- `.planning/STATE.md` lines 111–207 — full v2.0 skeleton section: packages, commands, build state, what's stubbed
- `internal/params/param.go` — Param interface, Spec, Value
- `internal/params/{text,textarea,number,select,multiselect,bool,path,secret}.go` — concrete Param impls
- `internal/params/{form,parse}.go` — Form builder + TOML bridge
- `internal/ecosystem/ecosystem.go` — Ecosystem interface, Context, Detector
- `internal/ecosystem/flag.go` — Flag type with chainable With* builders
- `internal/ecosystem/registry.go` — thread-safe Registry
- `internal/ecosystems/charm/flags.go` — 25+ charm flags
- `internal/ecosystems/charm/{validate,render,post}.go` — charm lifecycle
- `internal/runner/{source,list,execute,explain}.go` — task runner core
- `internal/runner/sources/{spinconfig,taskfile,makefile,packagejson,scripts,fallback,os}.go` — source chain
- `internal/template/{template,spin_toml,parse,form,loader,engine}.go` — external template loader
- `internal/registry/{client,search,types}.go` — registry client
- `internal/builder/builder.go` — interface stub
- `cmd/{run,new_charm,new_extras,ecosystem,search,add,list,version,deprecate}.go` — v2.0 commands

### v1.0 baseline (must not regress)
- `.planning/ROADMAP.md` — Phases 1–4 success criteria
- `internal/scaffold/{scaffold,template}.go` — the existing scaffold engine the charm ecosystem wraps
- `cmd/{new,build,test,vet,fmt,lint,doctor,update}.go` — v1.0 commands (kept working)
- `internal/wrap/{run,build,test,vet,fmt,lint,detect}.go` — v1.0 toolchain wrappers

### Project docs
- `.planning/PROJECT.md` — updated 2026-06-08 for v2.0 pivot (core value, active reqs, decisions)
- `.planning/REQUIREMENTS.md` — v2.0 reqs added (ECO-*, TPL-12..18, RUN-09..14, REG-05..08, BC-01..03)
- `CLAUDE.md` — stack pins, charm v2 paths, Go version floors

</canonical_refs>

<specifics>
## Specific Ideas

- The rust ecosystem's `Tasks()` should return `{"build": "cargo build", "test": "cargo test", "run": "cargo run", "clippy": "cargo clippy", "fmt": "cargo fmt"}` — these then merge with the source chain at the fallback level
- Charm's `Tasks()` returns the existing v1.0 wrappers (`air` for `dev`, `prism` for `test`, `gofumpt` for `fmt`, etc.) — backward compat
- The registry client's `Search()` should return a structured `Result` slice with `Name`, `Description`, `Language`, `Stars`, `URL`; the CLI formats it as a table
- `spin new <name> --template <ref>` can be combined with an ecosystem: `spin new rust <name> --template <ref>` is also valid (template overlays on the ecosystem)
- The post-hook shell command is rendered with `text/template` against the resolved param values, so a post-hook can do `cargo init --name {{.project_name}}` after the params form runs
- The runner's `--explain <task>` should print: `task <name>\n  source: <source>:<line>\n  command: <cmd>\n  notes: <notes>` (or similar)
- The source chain in the runner is wired in `cmd/run.go` (already done) to avoid an import cycle: `internal/runner` → `internal/runner/sources`
- The deprecation notice for `spin new <name>` is a one-time message (rate-limited via a per-process bool), not per-invocation spam
- The v1 templates in `internal/scaffold/templates/` (26 files) are reused verbatim by the charm ecosystem; no rewrite
- The template loader's `Loader()` already does git clone with `GIT_TERMINAL_PROMPT=0`; needs to be filled in for real (clone, cache, refresh)
- The `Builder` interface stub has a single method `Build(ctx, questions) (Context, error)` — no implementation in v2.0

</specifics>

<deferred>
## Deferred Ideas

| Idea | Defer to | Reason |
|------|----------|--------|
| `Builder` concept (question tree with custom renderers) | v2.x | Skeleton ships the interface; full design is a separate phase |
| External ecosystem loading (Go plugins) | v2.x | Adds plugin ABI maintenance; compiled-in is sufficient for v2.0 |
| Hosted registry server | separate `spin-registry` project | Server is its own codebase; client ships first |
| Tauri ecosystem | v2.x | User mentioned it; defer until after rust ships |
| Next/Nuxt/Vue/React/Flutter/Dart/C#/Java ecosystems | v2.x | Each needs its own template bundle and post-hooks |
| TUI mode for the scaffolder | v2.x | CLI is sufficient; charm stack is for scaffolded projects |
| `spin workspace` / `go.work` management | v2.x | Per-project config; multi-project is later |
| `spin release` wrapping goreleaser | v2.x | Defer; users can run goreleaser directly |
| `spin doctor` checking rust projects | v2.x | Doctor is universal Go today; add cargo-aware checks in v2.x |
| Inline `text/template` rendering of `spin.config.toml` | v2.x | The v2.0 TOML uses literal strings; templated values are a v2.x feature |

</deferred>

---
*Phase: 05-v2-0-universal-scaffolder-task-runner*
*Context gathered: 2026-06-08 via conversation (the v2.0 vision pivot)*
