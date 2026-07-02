# Roadmap: spin

## Overview

Build a Go project scaffolder that delivers the "perfect first run" promise -- one command produces a charmbracelet v2 project that builds, tests, and runs without extra setup. The journey moves from a single working TUI template (Phase 1), through full library coverage and toolchain wrappers (Phase 2), into interactive prompts and AI-context generation (Phase 3), and lands on a post-scaffold health/maintenance layer that lets users evolve generated projects over time (Phase 4). The scaffolder itself is built with the same charm stack it ships, so the tool dogfoods the experience end-to-end.

## v2.x Pivot (2026-06-10)

After Phase 5 validation (2026-06-09), the user pivoted: ship the **scaffolding layer only**, defer the task runner, and validate the ecosystem concept against real template usage before committing further. Phase 5's deliverables (the v2 ecosystem system + task runner) were archived to `~/Projects/Golang/spin-ecosys-tasks-archieve/` rather than kept in-tree.

Forward direction:
- **Templates are the only extension surface** for projects. A template is a git repo (or local path) with `spin.toml` + `_base/` + optional `_post/`.
- **Ecosystem concept is deferred** until templates prove insufficient.
- **Task runner is deferred** indefinitely.
- The `internal/template` package is the foundation. `internal/scaffold` is the v1 charm-flavoured engine; deprecated in favour of templates.
- v2.x is the "ship templates, build real ones, learn what users need" milestone.

## v2.x Milestone: local-registry (Phases 6-8)

**Milestone Goal:** Replace the HTTP registry stub (`https://registry.spin.invalid/v1`) with a zero-backend git/local registry model. Registries are directories on disk containing `registry.toml` + `templates/*.toml`. `spin search` reads them directly; `<alias>/<id>` shorthand works in `spin add` and `spin new`.

Storage layout:
- `~/.config/spin/registries.json` -- registry config
- `~/.config/spin/registries/<alias>/` -- cloned or symlinked registry
- `~/.config/spin/pinned.json` -- unchanged format; existing pins keep working

## Phases

- [x] **Phase 1: Scaffolder Foundation + Core TUI Stack** - One-command runnable TUI project (the "perfect first run" MVP) (completed 2026-06-02)
- [x] **Phase 2: CLI Variant + Wrappers + Extended Library Coverage + External Templates** - All variants, all charm libs, toolchain wrappers, --template-repo (completed 2026-06-03)
- [x] **Phase 3: Interactive Prompts (gum) + AI/AGENTS.md** - Prompts when flags missing, AGENTS.md opt-in (completed 2026-06-04)
- [x] **Phase 4: Post-Scaffold Health + Verification + Dogfooding** - spin doctor + lint + update + CI dogfood (completed 2026-06-05)
- [x] **Phase 5: v2.0 Universal Scaffolder & Task Runner** - migrate charm to new Ecosystem/Template/Builder model; add Rust ecosystem; external template loader; spin.config.toml with fallback chain; registry MVP (completed 2026-06-09)
- [ ] **Phase 6: Registry Manager + `spin registry` CLI + `registries.json`** - CRUD over registries.json; clone or symlink registries; atomic writes; alias validation; pinned-template dependency check
- [ ] **Phase 7: Index Reader + `<alias>/<id>` Resolver + Rewire Search/Add/New** - local TOML index; shorthand resolution; transient clone lifecycle; search rewire
- [ ] **Phase 8: Delete HTTP Client + Docs Pass** - drop HTTP stub, env vars, error symbols; rename client.go to pin.go; final docs pass

## Phase Details

### Phase 1: Scaffolder Foundation + Core TUI Stack
**Goal**: User can scaffold a runnable charm v2 TUI project in one command that builds and runs cleanly without edits.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: SCAF-01, SCAF-02, SCAF-03, SCAF-04, SCAF-05, SCAF-06, SCAF-07, SCAF-08, FLAG-01, FLAG-02, FLAG-03, FLAG-04, FLAG-05, FLAG-06, FLAG-13, FLAG-14, FLAG-15, FLAG-16, FLAG-17, FLAG-18, TMPL-01, TMPL-04, TMPL-05, TMPL-06, TMPL-07, TOOL-01, TOOL-02, TOOL-03, TOOL-04, TOOL-05
**Success Criteria** (what must be TRUE):
  1. User can run `spin new myapp --tui --bubbletea --bubbles --lipgloss` and get a project in `./myapp/` whose `main.go` runs cleanly with `go run` and exits with a working bubbletea "hello" example.
  2. Generated project builds with `CGO_ENABLED=0 go build ./...`, runs `go test ./...` with no failures, and is committed by an automated `git init` + initial commit.
  3. Generated project contains a working `.air.toml` (using `build.entrypoint`, not deprecated `build.bin`) and a `Taskfile.yml` (or `Makefile`) with a `setup` target.
  4. `spin --help`, `spin new --help`, and every subcommand help render with fang styling (no raw cobra default output).
  5. spin rejects invalid Go module path names, refuses to overwrite an existing directory without `--force`, and reports unknown flags with a clear suggestion.
**Plans**: 4 plans in 3 waves
  - [x] **01-01-PLAN.md** (Wave 1): Walking Skeleton -- SKELETON.md + minimal `spin new <name> --tui --bubbletea` + go build smoke test
  - [x] **01-02-PLAN.md** (Wave 2): Flag wiring + validation + post-scaffold hooks + git init + version
  - [x] **01-03-PLAN.md** (Wave 2, parallel): Full template engine + all overlays + build configs + license + README
  - [x] **01-04-PLAN.md** (Wave 3): CI grep suite + integration test + spin repo polish

### Phase 2: CLI Variant + Wrappers + Extended Library Coverage + External Templates
**Goal**: User can scaffold any project variant (CLI, full TUI), add any charm library, wrap the go toolchain with one tool, and pull external templates from a git repo.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: FLAG-07, FLAG-08, FLAG-09, FLAG-10, FLAG-11, FLAG-12, TMPL-02, TMPL-03, WRAP-01, WRAP-02, WRAP-03, WRAP-04, WRAP-05, WRAP-06, WRAP-07, WRAP-08
**Success Criteria** (what must be TRUE):
  1. User can run `spin new mycli --cli --cobra --fang` (or `--all`) and get a working CLI project whose `main.go` runs `mycli --help` with fang styling and an executable cobra hello-world command.
  2. User can pass any charm library flag (`--huh`, `--glamour`, `--glow`, `--wish`, `--log`, `--harmonica`) to a TUI project and the library appears in `go.mod` (under `charm.land/<lib>/v2`) and is wired into a working example.
  3. `spin run` uses `air` for hot reload when `.air.toml` is present and falls back to `go run`; `spin build` emits `bin/<name>`; `spin test` invokes `prism` (falling back to `go test` on Go < 1.24 or when prism is missing); `spin vet` runs `go vet ./...`; `spin fmt` runs `gofumpt` then `goimports` (failing loud with an install hint when gofumpt is missing unless `--no-strict`).
  4. User can pass `--template-repo <url>` to override the embedded template with an external git repo (shallow-cloned to a temp dir with `GIT_TERMINAL_PROMPT=0`); offline default still works.
  5. Every wrapper detects the preferred tool on `$PATH` and falls back to stock Go with a one-line install hint; wrappers do not silently downgrade to weaker behavior.
**Plans**: TBD

### Phase 3: Interactive Prompts (gum) + AI/AGENTS.md
**Goal**: When the user runs `spin new` without enough flags, spin asks for the missing pieces via gum (or huh v2 fallback) and can opt-in to an AGENTS.md describing the project for AI assistants.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: INT-01, INT-02, INT-03, INT-04, INT-05, AI-01, AI-02, AI-03, AI-04
**Success Criteria** (what must be TRUE):
  1. When stdin is a TTY and flags are missing, spin prompts the user via `gum` for project type, libraries, template, and AI opt-in; when `gum` is not on `$PATH`, spin transparently falls back to in-process `huh v2` prompts.
  2. User can pass `--no-interactive` (alias `--yes`, `--batch`) to disable all prompts -- flags-only mode works in CI and scripted environments.
  3. spin never hangs in non-TTY environments: every TUI/prompt call is guarded with `isatty.IsTerminal(os.Stdin)`.
  4. User can pass `--ai` (alias `--agents`) to generate an `AGENTS.md` containing a `<!-- AUTOGENERATED by spin X.Y.Z -->` marker and a list of the project's charm libraries with extension guidance.
  5. Prompt answers and flag values populate the same single `Project` struct -- there is exactly one source of truth resolved at command time.
**UI hint**: yes
**Plans**: TBD

### Phase 4: Post-Scaffold Health + Verification + Dogfooding
**Goal**: User can audit, extend, and refresh a generated project after the initial scaffold, and the scaffolder is dogfooded on its own codebase.
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: HLTH-01, HLTH-03
**Success Criteria** (what must be TRUE):
  1. User can run `spin doctor` on any Go project (universal; not just spin-scaffolded) to verify Go version, tool presence (`air`, `prism`, `gofumpt`, `goimports`), `go.mod` hygiene, and `CGO_ENABLED=0 go build ./...` success -- with `--format human|json`, `--strict`, `--deep` (includes `golangci-lint`), and `--fix` flags.
  2. User can run `spin lint` to invoke `golangci-lint` with a one-line install hint when the binary is missing; all golangci-lint subcommands (`run`, `cache clean`, etc.) pass through.
  3. User can run `spin update` (universal Go dep updater) to see a huh v2 form with one Select per direct dep, each defaulting to `newStable` (highest non-pre-release) with options `Skip` / `newStable` / `newLatest` (highest including pre-releases). Submitting applies `go get` + `go mod tidy` + `CGO_ENABLED=0 go build ./...` atomically; `go test` is NOT run.
  4. No generated file carries a `// generated by spin X.Y.Z` or `<!-- AUTOGENERATED by spin X.Y.Z -->` header; the "owned-by-spin" signal concept is retired.
  5. The `spin` project itself is dogfooded: a CI job (`dogfood` workflow) runs `spin new spin --cli --cobra --fang` in a fresh temp dir, then `go build ./...` + `go test ./...` on the result, and triggers on changes to `internal/scaffold/templates/**` and `cmd/**`.
**Plans**: 6 plans in 3 waves
  - [x] **04-01-PLAN.md** (Wave 1): `spin doctor` core -- internal/doctor/ package with 4 universal checks, --format human|json, --strict, --deep, --fix; cmd/doctor.go
  - [x] **04-02-PLAN.md** (Wave 1): `spin lint` wrapper -- internal/wrap/lint.go with golangci-lint detect + install hint; cmd/lint.go with ArbitraryArgs
  - [x] **04-03-PLAN.md** (Wave 1): `spin update` engine -- internal/update/{parse,resolve,apply}.go (modfile parse, proxy.golang.org fetch, go get + tidy + CGO=0 build; no go test)
  - [x] **04-04-PLAN.md** (Wave 2): `spin update` huh v2 form + cmd -- internal/update/form.go (multi-Select per dep, non-TTY table fallback); cmd/update.go with --all flag
  - [x] **04-05-PLAN.md** (Wave 1): Strip `// generated by spin` and `<!-- AUTOGENERATED by spin` markers from 26 .tmpl files; invert TestAGENTSmd_MarkerOnLine1
  - [x] **04-06-PLAN.md** (Wave 3): CI dogfood -- .github/workflows/{ci,dogfood}.yml + scripts/dogfood.sh (local-runnable)

### Phase 5: v2.0 Universal Scaffolder & Task Runner
**Goal**: spin v2.0 -- universal, language-agnostic scaffolder and task runner. The v2.0 skeleton (built 2026-06-08) defines the architecture; this phase fills in the implementation, migrates the v1 charm scaffolder to the new Ecosystem/Template/Builder model, and proves universality with a second ecosystem.
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: ECO-01..ECO-12, TPL-12..TPL-18, RUN-09..RUN-14, REG-05..REG-08, BC-01..BC-03
**Success Criteria** (what must be TRUE):
  1. `spin new <name>` (no ecosystem) still works for backward compat but prints a one-time deprecation notice pointing to `spin new charm <name>`; `spin new charm <name> --tui --bubbletea --bubbles --lipgloss --module <m>` produces an identical tree to the v1 path.
  2. `spin new rust <name> --bin` (or `--lib` / `--example`) produces `./<name>/` with a working `Cargo.toml`, `src/main.rs` (or `src/lib.rs`) that builds with `cargo build` and runs with `cargo run`. `spin run build|test|run|clippy|fmt` invoke the cargo fallbacks.
  3. `spin new --template <user/repo>` (or `spin new <name> --template <ref>`) clones the external git repo (shallow, `GIT_TERMINAL_PROMPT=0`), reads `spin.toml` for params, runs the huh form when TTY, applies defaults in non-TTY, renders `_base/` + overlays via text/template, and writes the project to `./<name>/`. The `spin.toml` is deleted after use; the post-hooks run on success.
  4. `spin run` resolves tasks across the source precedence chain: `spin.config.toml` -> `Taskfile.yml` -> `Makefile` -> `package.json` -> `scripts/` -> language fallback. `--list` shows the merged list with source labels; `--explain <task>` shows origin + raw command. The fallback for go projects is `go build/test/run/vet/fmt`; for cargo projects it is `cargo build/test/run/clippy/fmt`.
  5. `spin search <query>` returns registry results when a registry server is reachable, and a friendly "registry not yet deployed, see github.com/example/spin-registry" message when not -- never a stack trace. `spin add <user/repo>` pins to `~/.config/spin/pinned.json`; `spin list` shows pinned entries with their resolved local path.
**Plans**: 4 plans in 2 waves
  - [x] **05-01-PLAN.md** (Wave 1): Rust ecosystem - internal/ecosystems/rust/ with Flags, Validate, Render, PostScaffold, Tasks; registered in defaultRegistry; cmd/new_rust.go dispatch
  - [x] **05-02-PLAN.md** (Wave 1): Ecosystem dispatch + template loader - cmd/new.go deprecation shim + ecosystem-name detection; upgraded internal/template/loader.go (XDG cache, GIT_TERMINAL_PROMPT=0); new post_hook.go with RunPostHook; RenderToWithPost deletes spin.toml; registry unit tests
  - [x] **05-03-PLAN.md** (Wave 1): Registry client hardening - friendly-failure Search; SPIN_REGISTRY_URL env override; real spin add (clone or local-symlink); spin list shows LocalPath; 11 unit tests
  - [x] **05-04-PLAN.md** (Wave 2, depends on 01-03): Runner integration - ecosystem source in defaultSourceChain (cargo fallbacks win); --list/--explain JSON output; Task.Env field; cross-cutting unit tests

### Phase 6: Registry Manager + `spin registry` CLI + `registries.json`
**Goal**: User can register, list, update, and remove local or git-backed registries that drive `spin search` and the `<alias>/<id>` shorthand. The manager owns `registries.json` and the `~/.config/spin/registries/` cache root; every CLI subcommand proves the manager end-to-end.
**Depends on**: Phase 5
**Requirements**: REG-09, REG-10, REG-11, REG-12, REG-13, REG-14, REG-15, REG-16, REG-17, REG-18, REG-19, BCX-01, BCX-02, BCX-03, SRCH-05, SRCH-06
**Success Criteria** (what must be TRUE):
  1. User can run `spin registry add official https://github.com/example/spin-registry` and see `official` appear in `spin registry list`; the registry is shallow-cloned to `~/.config/spin/registries/official/` and `registries.json` records the alias, source, kind, cache path, and added_at timestamp.
  2. User can run `spin registry add mylocal ~/work/my-registry` to symlink a local registry directory (or copy it on Windows); `spin registry list` shows `kind=local` and the resolved cache path.
  3. `spin registry list` renders a styled table with columns alias, source, kind, cache path, template count, and last-updated timestamp; `spin registry list --json` outputs the same data as a structured array for scripting.
  4. `spin registry update [alias]` runs `git fetch + reset` for git registries (no-op + clear notice for local registries); per-registry outcomes are reported, and a summary line prints at the end. The `last_updated` timestamp is set on successful git refresh.
  5. `spin registry remove <alias>` deletes the entry from `registries.json` and removes the cache directory; the command refuses and lists dependent pin names if pinned templates reference the registry, unless `--purge-pinned` is passed.
  6. Invalid aliases (containing `/`, `\\`, `:`, whitespace, `..`, leading `-`, or NUL bytes) are rejected before any filesystem write; duplicate aliases are rejected unless `--force` is passed; if the source lacks `registry.toml` after clone/symlink, the operation rolls back and errors.
  7. Invalid registry or template metadata files are skipped with a warning (not indexed) and reported in `spin registry update` summary; one bad file never aborts the run.
  8. `registries.json` is written atomically (temp file + rename); a crash mid-write never leaves an unparseable file. Existing `~/.config/spin/pinned.json` keeps working unchanged.
**Plans**: TBD

### Phase 7: Index Reader + `<alias>/<id>` Resolver + Rewire Search/Add/New
**Goal**: `spin search` becomes local-only and reads every `templates/*.toml` from disk. `<alias>/<id>` shorthand resolves through the registry index in `spin add` and `spin new`, with a documented resolution precedence and a transient clone lifecycle that never leaks temp dirs.
**Depends on**: Phase 6
**Requirements**: SRCH-01, SRCH-02, SRCH-03, SRCH-04, RES-01, RES-02, RES-03, RES-04, RES-05, RES-06
**Success Criteria** (what must be TRUE):
  1. User can run `spin search go-api` and see matching templates from every registered registry listed with alias, id, name, description, source, and tags; results come from `~/.config/spin/registries/*/templates/*.toml` only -- no network call is made.
  2. `spin search --json` outputs `{query, total, entries[]}` where each entry carries `alias`, `id`, `name`, `description`, `source`, `tags`; `spin search` (no `--json`) renders a styled table.
  3. Search results are sorted by relevance: exact id match > name match > description match > tag match, with `id` alphabetical tie-break.
  4. User can run `spin add official/go-api` to pin a template from the `official` registry by id; the same shorthand works in `spin new myapp --template official/go-api` and produces a project that builds.
  5. Resolution precedence is fixed and tested: local path > git URL > `<alias>/<id>` > legacy `Pinned.Name` > `user/repo` shorthand. A spec containing exactly one `/` (neither side empty, neither side containing another `/`) is treated as `<alias>/<id>`; other patterns fall through to `user/repo` shorthand.
  6. If a registry template's `source` is itself a `<alias>/<id>`, the resolver recurses once (max depth 2); cycles are rejected with a clear error.
  7. Transient clones for `spin new <alias>/<id>` (when not yet pinned) are written to a temp dir under the cache root, then either renamed to the pinned location or removed; no transient clone leaks on disk after the run completes. A startup sweep cleans stale transients older than 1 hour.
**Plans**: TBD

### Phase 8: Delete HTTP Client + Docs Pass
**Goal**: Remove every HTTP-based registry symbol from the codebase; rename `client.go` to `pin.go`; update docs to describe the local-registry model. The cleanup is safe because Phase 7 eliminated every reference to the deleted symbols.
**Depends on**: Phase 7
**Requirements**: CLN-01, CLN-02, CLN-03, CLN-04, CLN-05
**Success Criteria** (what must be TRUE):
  1. `internal/registry/search.go` is deleted; `internal/registry/client.go` is renamed to `pin.go` with HTTP-only code removed (no `Client.Search`, `Client.SearchWithLimit`, HTTP timeout, or `isNetworkError`).
  2. `SPIN_REGISTRY_URL` and `SPIN_REGISTRY` env vars are dropped; the env-var reader in `Client.New()` is removed.
  3. `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`, `Entry`, and `SearchResult` are removed from `internal/registry/types.go`.
  4. `go build ./...` and `go test ./...` pass with zero references to any deleted symbol; tests that set `SPIN_REGISTRY_URL` are removed and replaced with `manager_test.go`, `index_test.go`, and `resolve_test.go` covering the new surface.
  5. Docs are updated: `README.md` describes the local-registry model; `PROJECT.md` "Validated" section reflects the new capabilities; any `spin-registry.md` revision note is added.
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Scaffolder Foundation + Core TUI Stack | 4/4 | Complete | 2026-06-02 |
| 2. CLI Variant + Wrappers + Extended Library Coverage + External Templates | 4/4 | Complete | 2026-06-03 |
| 3. Interactive Prompts (gum) + AI/AGENTS.md | 4/4 | Complete | 2026-06-04 |
| 4. Post-Scaffold Health + Verification + Dogfooding | 6/6 | Complete | 2026-06-05 |
| 5. v2.0 Universal Scaffolder & Task Runner | 4/4 | Complete | 2026-06-09 |
| 6. Registry Manager + `spin registry` CLI + `registries.json` | 0/TBD | Not started | - |
| 7. Index Reader + `<alias>/<id>` Resolver + Rewire Search/Add/New | 0/TBD | Not started | - |
| 8. Delete HTTP Client + Docs Pass | 0/TBD | Not started | - |