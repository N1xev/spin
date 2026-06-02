# Walking Skeleton — spin

**Phase:** 1
**Generated:** 2026-06-02

## Capability Proven End-to-End

A Go developer can run `spin new myapp --tui --bubbletea` from any directory, and spin creates `./myapp/` with a working `charm.land/bubbletea/v2` TUI hello-world whose `go build ./...` and `go run .` both succeed with zero edits — proving the full embed → render → emit → verify pipeline works against real charm v2 packages.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| CLI framework | `github.com/spf13/cobra` v1.9.1 + `charm.land/fang/v2` | CLAUDE.md mandates charm-styled help; fang is the drop-in wrapper around cobra root that produces styled `--help`, errors, completion, manpages. Cobra v1.9.1 is the fang-tested floor. |
| Embed root | `//go:embed all:templates` rooted at `templates/` | `*` glob would skip hidden files like `.air.toml` and `.gitignore`; `all:` prefix includes them. RESEARCH §4.1. |
| Overlay composition | `_base/` + `variant_<type>/` + `lib/<name>/`, last-write-wins | RESEARCH §4.2. Clean separation: base always renders, variant overlays, lib overlays last. Forward-compat for Phase 2 (more variants + libs) without template-engine refactor. |
| Template engine | stdlib `text/template` with `FuncMap` registered before `Parse` | RESEARCH §4.3. Deterministic, testable, no extra dep. `t.Option("missingkey=error")` catches typos in dev. |
| Project contract | Single `Project` struct in `internal/scaffold/project.go` | TMPL-07 + INT-05 (forward-compat). Every flag, every prompt answer populates the same struct — no second source of truth. |
| Smoke test | `go build ./...` with `CGO_ENABLED=0` post-scaffold | RESEARCH §7. Catches v1 import leaks, wrong version pins, missing `go 1.25.0` directive, `View() string` mistakes. Hard fail with `go build` stderr surfaced. |
| Git init | env-guarded: `GIT_TERMINAL_PROMPT=0`, `GIT_AUTHOR_NAME=spin`, `GIT_AUTHOR_EMAIL=spin@localhost` | RESEARCH §12.3. Never blocks in CI; never fails when user has no global git identity. `-b main` for modern default. |
| Distribution | `go install github.com/<org>/spin@latest`; single static binary | CLAUDE.md. No runtime deps, no embedded `gum` (cross-compile complications). |
| Go version (spin itself) | `go 1.23` in `spin`'s own `go.mod` | STATE.md. spin does not import `charm.land/bubbles/v2`, so it does not need 1.25. |
| Go version (generated) | `go 1.25.0` when `--bubbles` is set, `go 1.23` otherwise | STATE.md. bubbles v2 README requires 1.25.0; lipgloss and bubbletea alone work on 1.23. |
| Charm v2 only | All generated imports under `charm.land/<lib>/v2`; v1 paths forbidden | CLAUDE.md + STATE.md. Enforced by (1) post-scaffold `go build` smoke test, (2) CI grep suite (Phase 1 deliverable), (3) PITFALLS #1–4. |
| No CGO | All generated projects build with `CGO_ENABLED=0` | CLAUDE.md. Charm v2 stack is pure Go; CGO is unneeded. |
| Forward-compat flags | All Phase 2/3/4 flags (`--cli`, `--cobra`, `--huh`, `--ai`, etc.) are registered on `Project` as bools with `false` zero value | RESEARCH §4.4. Flag binding is one-shot; later phases only add template content, not flag-registration churn. |

## Stack Touched in Phase 1

- [x] Project scaffold — `go.mod` (spin), `go.mod` (generated), cobra + fang wiring
- [x] One real "DB read/write" — `go:embed` reads template files; renderer writes them to disk under `./<name>/`
- [x] One real "UI interaction" wired to "the API" — `spin new myapp --tui --bubbletea` CLI command scaffolds a runnable bubbletea app (the scaffolder dogfoods the fang-styled help it ships to its own users)
- [x] Local-run command — `cd myapp && go run .` exercises the full stack on the user's machine

## Out of Scope (Deferred to Later Slices)

- `--cli` variant template content (only flag binding in Phase 1) → Phase 2
- `--cobra`, `--fang`, `--viper`, `--module`, `--license` template wiring (flag bindings exist; template content is Phase 2) → Phase 2
- `--huh`, `--glamour`, `--glow`, `--wish`, `--log`, `--harmonica`, `--modifiers`, `--ansi`, `--runewidth` template content → Phase 2
- `--template-repo <url>` external override (flag NOT registered in Phase 1) → Phase 2
- Interactive `gum` prompts + `--no-interactive` → Phase 3
- `AGENTS.md` / `--ai` → Phase 3
- `spin run`/`build`/`test`/`vet`/`fmt` toolchain wrappers → Phase 2
- `spin doctor`/`add`/`update` post-scaffold commands → Phase 4
- `.goreleaser.yaml` in generated project → Phase 2/3
- `.golangci.yml` in generated project → Phase 2
- `Makefile` alternative to `Taskfile.yml` → Phase 2

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton without altering its architectural decisions:

- **Phase 2: CLI Variant + Wrappers + Extended Library Coverage + External Templates** — adds `--cli` template, all remaining charm library overlays (`huh`, `glamour`, `wish`, `log`, `harmonica`), `spin run`/`build`/`test`/`vet`/`fmt` wrappers, `--template-repo` clone, and GoReleaser config.
- **Phase 3: Interactive Prompts (gum) + AI/AGENTS.md** — adds in-process `huh v2` prompts (with `gum` shell-out when available), `--no-interactive`/`--yes`/`--batch` flags, `isatty.IsTerminal` guards, `--ai`/`--agents` flag, `AGENTS.md` generation with `<!-- AUTOGENERATED by spin X.Y.Z -->` marker.
- **Phase 4: Post-Scaffold Health + Dogfooding** — adds `spin doctor` (Go version, tool presence, charm v2 path check, CGO=0 buildability), `spin add <lib>` (inject a lib into an existing project), `spin update` (re-apply non-conflicting template changes), `// generated by spin X.Y.Z` headers on every emitted file, and dogfooding CI that runs `spin new spin --cli --cobra --fang` inside its own build matrix.
