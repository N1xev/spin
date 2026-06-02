# spin

A Go project scaffolder for the charmbracelet v2 ecosystem.

`spin` generates ready-to-run Go projects — TUI apps, CLI tools, or both —
pre-wired with the right charmbracelet libraries, modern Go tooling (cobra,
fang), hot reload (air), and the prism test runner. One command produces a
project that builds, tests, and runs without extra setup.

`spin` is built with cobra + fang + log so the tool itself demonstrates the
charmbracelet experience: `spin --help` is fang-styled, scaffolder output
is charm/log v2 structured logging, and the templating is stdlib Go.

## What it does

`spin new myapp --tui --bubbletea --bubbles --lipgloss` produces a project
in `./myapp/` whose `go build ./...` and `go run .` both succeed with zero
edits. The generated project ships with:

- `charm.land/bubbletea/v2` (TUI framework)
- `charm.land/bubbles/v2` (TUI components: spinner, textinput, ...)
- `charm.land/lipgloss/v2` (terminal styling)
- `go 1.25.0` directive (required by bubbles v2)
- `.air.toml` with modern `build.entrypoint` (not deprecated `build.bin`)
- `Taskfile.yml` with a `setup` target that installs gofumpt, goimports, air, prism
- `LICENSE` (MIT by default; `--license apache-2.0` or `--license none` available)
- Initial `git init` + 1 commit (skip with `--no-git`)

## Install

```sh
go install github.com/example/spin@latest
```

> Replace `github.com/example/spin` with the actual module path of the
> installed copy. The repo currently uses `github.com/example/spin` as a
> safe default until the canonical org is wired in.

The binary lands at `$(go env GOPATH)/bin/spin`. Single static binary, no
runtime dependencies.

## Quick start

```sh
# Scaffold a TUI project with the full Phase 1 charm v2 stack.
spin new myapp --tui --bubbletea --bubbles --lipgloss
cd myapp

# Run it.
go run .
```

Other variants:

```sh
# TUI with only bubbletea (no spinner, no styling).
spin new myapp --tui --bubbletea

# With explicit module path + Apache 2.0 license.
spin new myapp --tui --bubbletea --module github.com/foo/myapp --license apache-2.0

# Overwrite an existing directory.
spin new myapp --tui --bubbletea --force

# Skip post-scaffold smoke test or git init.
spin new myapp --tui --bubbletea --no-verify
spin new myapp --tui --bubbletea --no-git
```

## Status

**Phase 1 of 4 complete.** Phase 1 ships the scaffolder foundation and
the core TUI stack (bubbletea + bubbles + lipgloss, MIT/Apache-2.0 license,
.air.toml, Taskfile.yml, post-scaffold smoke test, git init, v1-leak
CI grep suite).

See [.planning/ROADMAP.md](.planning/ROADMAP.md) for the full 4-phase plan
and [.planning/REQUIREMENTS.md](.planning/REQUIREMENTS.md) for the 59
requirements driving it.

## Requirements

- `Go 1.23+` to install spin.
- `Go 1.25.0+` to build generated projects that use `--bubbles` (per
  charmbracelet/bubbles v2 docs). TUI projects without `--bubbles` work
  on `Go 1.23+`.

## Documentation

- [.planning/PROJECT.md](.planning/PROJECT.md) — project context and core value
- [.planning/ROADMAP.md](.planning/ROADMAP.md) — 4-phase plan
- [.planning/REQUIREMENTS.md](.planning/REQUIREMENTS.md) — v1 requirements
- [.planning/phases/01-scaffolder-foundation-core-tui-stack/01-RESEARCH.md](.planning/phases/01-scaffolder-foundation-core-tui-stack/01-RESEARCH.md)
  — Phase 1 technical research (charm v2 stack, pitfalls, CI grep patterns)
- [.planning/phases/01-scaffolder-foundation-core-tui-stack/01-SKELETON.md](.planning/phases/01-scaffolder-foundation-core-tui-stack/01-SKELETON.md)
  — Walking Skeleton architectural decisions

## Development

```sh
git clone <repo> && cd spin
task test            # unit + integration tests
task grep-v1-leaks   # CI grep suite — must exit 0
task build           # build spin into ./bin/spin
```

`task` is the [go-task](https://taskfile.dev) runner; not required to use
spin itself, but the spin dev workflow uses it.

## Charm v2 only

Generated projects use `charm.land/<lib>/v2` import paths. The v1 paths
under `github.com/charmbracelet/<lib>` are forbidden; the `scripts/check-v1-leaks.sh`
suite catches any regression in CI. Run it against a generated project:

```sh
bash scripts/check-v1-leaks.sh ./myapp
```

## License

MIT — see [LICENSE](LICENSE).
