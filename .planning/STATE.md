---
gsd_state_version: 1.0
milestone: v2.x-pivot
milestone_name: v2.x pivot (templates only)
status: pivot_in_progress
stopped_at: v2.x pivot on 2026-06-10 -- Phase 5 deliverables archived; cmd/new.go rewritten on internal/template
last_updated: 2026-06-10
last_activity: 2026-06-10 -- v2.x pivot: archive ecosystem+runner; rewrite cmd/new.go on internal/template
progress:
  total_phases: 5
  completed_phases: 5
  total_plans: 23
  completed_plans: 23
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-02)

**Core value:** Generate a perfect, runnable Go project using charmbracelet v2 libraries with a single command.
**Current focus:** Milestone complete

## Current Position

Phase: v2.x pivot
Plans: Phase 5 (4/4) was built and validated, then archived in the v2.x pivot
Status: Pivot in progress -- build green on templates-only foundation
Last activity: 2026-06-10

Progress: [███████░░░] templates-only foundation, real templates TBD

## Performance Metrics

**Velocity:**

- Total plans completed: 23
- Average duration: ~17m
- Total execution time: ~5 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 4 | 4 | ~15m |
| 02 | 5 | 5 | ~17m |
| 03 | 4 | 4 | ~20m |
| 04 | 6 | 6 | ~20m |
| 05 | 4 | - | - |

## Phase 4 Summary

| Plan | Subject | Status |
|------|---------|--------|
| 04-01 | spin doctor (4 universal checks) | ✓ complete |
| 04-02 | spin lint (golangci-lint wrapper) | ✓ complete |
| 04-03 | spin update engine (parse/resolve/apply) | ✓ complete |
| 04-04 | spin update huh v2 form + cobra cmd | ✓ complete |
| 04-05 | strip generated-by markers (D-12) | ✓ complete |
| 04-06 | CI dogfood (workflow + reusable script) | ✓ complete |

**Decisions delivered:**

- D-01..D-05: doctor scope, output format, exit codes, deep/lint split, auto-fix
- D-06..D-10: update universality, --all for indirect, 3-column versions, huh form, apply mechanics
- D-12: strip owned-by-spin markers
- D-13: dogfood CI job

**Drops honored:** HLTH-02 (spin add) and HLTH-04 (file headers) per user direction.

## Phase 5 Summary

| Plan | Subject | Status |
|------|---------|--------|
| 05-01 | Rust ecosystem (cargo binary/lib/example) | ✓ complete |
| 05-02 | Ecosystem dispatch + external template loader | ✓ complete |
| 05-03 | Registry client hardening (friendly-failure search + clone-or-pin) | ✓ complete |
| 05-04 | Runner integration (cargo fallbacks + JSON) | ✓ complete |

**Decisions delivered (so far):**

- Charm + Rust ecosystems are the v2.0 first-class citizens; templates are second-class
- Per-process one-time deprecation notice on `spin new <name>` (deprecationPrinted bool guard)
- Template loader cache moved to `~/.config/spin/templates/` per XDG spec
- `GIT_TERMINAL_PROMPT=0` always set so missing creds never block the scaffolder
- `ResolveForm` applies defaults FIRST then user values (fixes `<nil>` interpolation bug)
- `params.Value` unwrapped to raw primitives before `text/template` rendering
- `spin.toml` removed from output via defensive `filepath.Walk` (catches nested copies)
- Single source of truth for the registry: `cmd/ecosystem.go` `defaultRegistry()` (only one `NewRegistry` call site)
- Registry default URL is `https://registry.spin.invalid/v1` (RFC 2606 reserved `.invalid` TLD, never resolves) so the friendly "not yet deployed" message is always shown in v2.0 without waiting for a 15s DNS timeout
- `SPIN_REGISTRY_URL` is the canonical env var; `SPIN_REGISTRY` is honored as a fallback for any v2.0-skeleton caller
- `ErrNotDeployed` is the v2.0 canonical name; `ErrNotImplemented` is kept as an alias
- `Pinned.LocalPath` defaults to `CacheDir/templates/<name>` for older callers; new pins always carry a real path from `client.Add()`
- `client.Pin` uses atomic temp-file-then-rename writes (writePinned) so a partial write never corrupts `~/.config/spin/pinned.json`
- `client.Add` clones or copies BEFORE the caller writes the JSON -- a failed clone never leaves a half-written pin file referencing a non-existent directory
- `spin add` shorthand `user/repo` is rejected with a clear error (not a network attempt) until the public registry ships; the user is directed to a full git URL or a local path
- `cmd/add.go` is relaxed to `MinimumNArgs(0)` so `spin add` (no args) prints the pinned list (matches `spin add --list`)
- Ecosystem source at Order=5 wires the rust ecosystem's `Tasks()` into the runner's source chain, so `spin run build` in a Cargo project invokes `cargo build` (Phase 5 success criterion 4 load-bearing verified)
- `Task.Env []string` field added for the v2.0 env-var contract; surfaces in `Explain` (human + JSON); Execute is a follow-up
- spin.config.toml inline-table form: `{ command, description, env }` (RUN-14); shorthand `name = "cmd"` still works
- `--list --json` and `--explain X --json` wired via `ListJSON` / `ExplainJSON` (stdlib `encoding/json`); ExplainJSON encodes `ErrNotFound` as `{"error":"..."}`
- 25 new unit tests across runner, runner/sources, params, and template (all passing in <0.05s combined; zero regressions in v1 commands)

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Phase 4 additions:

- [Phase 4]: `spin doctor` is universal -- no charm-specific checks. Four checks only: Go version, tool presence, go.mod hygiene, `CGO_ENABLED=0 go build`. `--format human|json`, `--strict` upgrades warnings, `--deep` adds lint, `--fix` runs safe repairs.
- [Phase 4]: `spin update` is universal -- works on any go.mod, no spin-project detection. Huh v2 multi-field form, options `[Skip, newStable, newLatest]`, default `newStable`. Apply = `go get` + `tidy` + `CGO=0 go build` (no test).
- [Phase 4]: `// generated by spin {{.SpinVer}}` and `<!-- AUTOGENERATED by spin -->` markers stripped from all 26 generated-file templates. Owned-by-spin signal retired (D-12).
- [Phase 4]: CI dogfood job: triggers on `internal/scaffold/templates/**` and `cmd/**`, runs `spin new spin --cli --cobra --fang --module github.com/example/spin-fixture` in a tempdir, then build + test. Reusable as `scripts/dogfood.sh`.

### Pending Todos

None for Phase 4. Project complete to milestone v1.0.

### Blockers/Concerns

None.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260604-7jt | remove glow library + modifiers flag from spin | 2026-06-04 | 2630bca | [260604-7jt-remove-glow-library-modifiers-flag-from-](./quick/260604-7jt-remove-glow-library-modifiers-flag-from-/) |

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|------------|
| gum pre-selection in multi-select | gum's `choose --no-limit` does not support pre-selection via the CLI (huh backend DOES pre-select via .Selected(true)). Documented as future-enhancement hook. | Open | 03-03 |
| HLTH-02 `spin add <lib>` | Dropped from Phase 4 per user. If reintroduced, separate phase. | Parked | 04-00 |

## Session Continuity

Last session: 2026-06-09
Stopped at: Phase 05 Plan 04 complete; phase 05 complete; milestone v2.0-skeleton DONE
Next: Milestone v2.0-skeleton is complete. Phase 5 success criteria all met. Future: v2.x roadmap (Builder, external ecosystems, hosted registry).

## v2.0 Skeleton (in progress, 2026-06-08)

User direction: pivot spin from a Go+charm scaffolder into a universal,
language-agnostic scaffolder + task runner. v1.0 milestone is complete;
v2.0 is the pivot.

### New packages (skeleton -- interfaces + key paths, not full logic)

- `internal/params/` -- the 7 param types: text, textarea, number, select,
  multiselect, bool, path, secret. Each type implements `Param` and
  builds the appropriate `huh.Field`. `Form()` builds a `huh.Form`;
  `SetDefaults()` for `--no-interactive`. `Parse()` is the TOML→Param
  bridge.

- `internal/ecosystem/` -- `Ecosystem` interface, `Flag` type with
  chainable `With*` builders, `Registry` (builtin + external), `Loader`
  stub, `Detector` helpers (`FileDetector`, `ContentDetector`).

- `internal/ecosystems/charm/` -- the v2 charm ecosystem; implements
  `Ecosystem`; converts `Context`→`scaffold.Project` and delegates to
  the existing scaffold engine via the new `RenderToMap`/`WriteTo` shims.

- `internal/runner/` + `runner/sources/` -- the universal task runner.
  `Runner.All()` resolves tasks across sources in precedence order;
  `List`, `Explain`, `Run` for the three CLI surfaces. Sources:
  spin.config.toml, Taskfile.yml, Makefile, package.json, scripts/,
  language-specific fallback.

- `internal/template/` -- external templates (git repos). `Template` wraps
  a parsed spin.toml + _base/ + _post/. `Loader` handles local paths
  and git URLs. `BuildForm` + `ResolveForm` build the huh form.

- `internal/registry/` -- public template registry client.
  `Search()` returns `ErrNotDeployed` with a friendly message when
  the server is unreachable (REG-05, REG-08). `Add()` handles
  local paths (symlink-then-copyDir) and git URLs (shallow clone
  with `GIT_TERMINAL_PROMPT=0`); shorthand `user/repo` returns a
  clear error. `Pin`/`Unpin` work against `~/.config/spin/pinned.json`
  with atomic writes (REG-06, REG-07).

- `internal/builder/` -- interface stub for the future "builder" concept
  (go-blueprint / better-t-stack style). No implementation yet.

- `internal/huh/` -- placeholder for shared huh helpers (deferred).

### New commands (in cmd/)

- `spin run [task] [--list|--explain <task>]` -- universal task runner.
- `spin new --list-ecosystems` -- list every registered ecosystem.
- `spin new <ecosystem> <name> [flags]` -- first ecosystem: `charm`.
  The legacy `spin new <name> --tui --bubbletea` form still works
  (no deprecation warning yet in v2.0; will be added in v2.x).

- `spin search <query>` -- registry search (stub until server ships).
- `spin add <name>` / `spin list` -- pin/list templates.
- `spin ecosystem {list,info,add,remove}` -- manage ecosystems.
- `spin version` -- version subcommand.
- `spin build/test/vet/fmt/lint` -- DEPRECATION WARNING added via
  `cmd/deprecate.go` PreRun hooks. Suggests `spin run <task>`.

### Backward compatibility

All v1.0 commands and flags work unchanged. The v2 skeleton coexists
with the v1 implementation; nothing in `internal/scaffold/` or
`internal/wrap/` was removed. The deprecation warnings on
build/test/vet/fmt/lint are the only behaviour change in v2.0.

### Build state

- `go build ./...` -- green
- `go vet ./...` -- green
- `go test ./internal/wrap/...` -- pre-existing test `TestRun_WithAirToml`
  times out (660s). Not introduced by the v2 skeleton; appears to be an
  environment issue with the `air` binary in the test runner.

- `go test ./...` for the new packages -- green (most packages have no
  tests yet; the skeleton ships interfaces + signatures, full tests
  come with the v2.x phases that fill in the implementation).

### What's stubbed (not real yet)

- `internal/template/parse.go` is a hand-rolled mini-parser; not full TOML.
  The v2.x plan: use `encoding/toml/v2` (stdlib in Go 1.23+) or
  `pelletier/go-toml` and validate manifests server-side before publishing.

- `internal/registry/client.go` `Search()` returns `ErrNotDeployed`
  with a friendly message when the registry server isn't deployed.
  `Add()` handles local paths and git URLs; `user/repo` shorthand is
  rejected with a clear error. `Pin`/`Unpin` work against
  `~/.config/spin/pinned.json` with atomic writes.

- `internal/ecosystem/loader.go` -- external ecosystem loading deferred
  to v2.x.

- `cmd/new_charm.go`'s `runNewCharm` runs the new ecosystem path, but
  the legacy `cmd/new.go` is the canonical entry point. Full migration
  to the v2 flow is a v2.x task.

- `internal/builder/builder.go` -- interface stub only.

### Decisions still open

- Should `spin new <name>` (no ecosystem) keep working indefinitely
  as an alias for `spin new charm <name>`, or get a hard deprecation
  cycle?

- Should ecosystems be Go packages compiled in (current) or external
  plugins from v2.0 (more complex; more flexible)? Current: compiled in.

- MVP cut for v2.0 release: ship the skeleton, migrate `charm` to the
  new flow, leave the registry/builder as stubbed. Defer Rust/Next.js
  ecosystems until v2.x.

### Next step

Propose a v2.0 implementation roadmap as Phase 5 of `.planning/ROADMAP.md`:
5.1 -- fill in ecosystem/template/registry logic; 5.2 -- add Rust ecosystem
as proof of universality; 5.3 -- registry server MVP (separate project).

## v2.x Pivot (2026-06-10)

User direction after Phase 5 completion: focus on the **scaffolding layer only**. Archive the v2 ecosystem + task runner implementation. Templates become the sole extension surface.

Archived to `~/Projects/Golang/spin-ecosys-tasks-archieve/`:
- `internal/ecosystem/` -- Ecosystem interface + Registry + Flag + Detector + Loader stub
- `internal/ecosystems/{charm,rust}/` -- first-class ecosystem implementations
- `internal/runner/` + `runner/sources/` -- universal task runner
- `cmd/ecosystem.go`, `cmd/new_charm.go`, `cmd/new_rust.go`, `cmd/run.go` -- CLI wiring

Restored to `cmd/` (was mis-archived; uses `internal/registry`, not `internal/runner`):
- `cmd/list.go` -- pinned-template lister

Rewritten on `internal/template`:
- `cmd/new.go` -- was v2 ecosystem dispatch (dispatchV2); now loads templates via `template.Loader`, resolves params via `tpl.ResolveForm`, renders via `tpl.RenderToWithPost`.

Build state post-pivot:
- `go build ./...` -- green
- `go vet ./...` -- green
- `go test -run='^$' ./...` (test compile) -- all packages green

Next: ship 2-3 real templates (Go CLI, Python CLI, Rust CLI) to validate the template system end-to-end, then design the registry.
