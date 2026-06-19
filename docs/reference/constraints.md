# Constraints

The hard rules baked into spin's design. Constraints that govern what the scaffolder can import, what the generated projects can assume, and what behaviour is not configurable.

Source: `go.mod`, `CLAUDE.md`, `Taskfile.yml:10-12` (`env: CGO_ENABLED: '0'`), `scripts/check-v1-leaks.sh:47-64`.

## Go version floors

| Component | Floor | Why |
| --- | --- | --- |
| `spin` itself | **Go 1.25.8** (declared in `go.mod`) | Matches the v2 charm stack floor; required by `charm.land/bubbles/v2` transitively. |
| Scaffolded CLI-only projects | **Go 1.23** | No charm v2 libs that demand 1.25; cobra + fang is fine on 1.23. |
| Scaffolded projects that use `--bubbles`, `--huh`, or `--bubbletea` | **Go 1.25.0+** | `charm.land/bubbles/v2` explicitly requires 1.25.0 per its README. |

The split is documented in `CLAUDE.md` ("Go version tension") as an open question. Currently, the scaffolder pins `go 1.23` in the generated `go.mod` of a CLI-only project and `go 1.25.0` for projects that import bubbles. Verify on each scaffold by reading the generated `go.mod`.

## `CGO_ENABLED=0`

Pinned at the Taskfile level (`Taskfile.yml:10-12`) and at the dev-process level. The generated `go.mod` and `.goreleaser.yaml` (when the scaffolder emits one in a future variant) also default to `CGO_ENABLED=0`. This keeps the binary statically linkable, cross-compile-friendly, and minimal in container size.

To override locally:

```sh
CGO_ENABLED=1 task build
```

The override only works for the `go build` invocation, not for the runtime cgo requirements of the resulting binary. Most charm v2 libraries are pure Go; if a transitive dep imports cgo, the build will surface the cgo toolchain error.

## No charm v1 paths

The `scripts/check-v1-leaks.sh` deny-list (lines 47-64) enumerates the 16 import paths that indicate a v1 leak:

| Forbidden path | v2 replacement |
| --- | --- |
| `github.com/charmbracelet/bubbletea"` | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/lipgloss"` | `charm.land/lipgloss/v2` |
| `github.com/charmbracelet/bubbles"` | `charm.land/bubbles/v2` |
| `github.com/charmbracelet/huh"` | `charm.land/huh/v2` |
| `github.com/charmbracelet/glamour"` | `charm.land/glamour/v2` |
| `github.com/charmbracelet/wish"` | `charm.land/wish/v2` |
| `github.com/charmbracelet/log"` | `charm.land/log/v2` |
| `github.com/charmbracelet/fang"` | `charm.land/fang/v2` |

Plus the 8 v2-with-wrong-base variants (`github.com/charmbracelet/<lib>/v2"`). All of these are caught by the grep suite against `./internal` in CI (`.github/workflows/ci.yml:48-49`).

Deliberately **not** in the deny-list:

- `github.com/charmbracelet/harmonica` - v0.2.0 pre-dates the migration; still on github.com.
- `github.com/charmbracelet/glow/v2` - the v2 glow binary lives on github.com; `charm.land/glow/v2` does not exist.

## No v1 API patterns

The v2 API regression patterns (lines 69-92) catch v1 function calls that survived in v2 code by accident. Highlights:

- `View() string` - the v1 `tea.Model` signature. v2 returns `tea.View`.
- `tea.WithAltScreen` and friends - v1 program options. v2 puts `AltScreen` on the `tea.View` struct.
- `lipgloss.NewRenderer` / `DefaultRenderer` - v1 renderer API.
- `msg.Type` / `msg.Runes` / `msg.X` / `msg.Y` - v1 message-field access. v2 uses typed fields on `tea.KeyPressMsg` / `tea.MouseClickMsg`.

The full table is in [Scripts](../development/scripts.md).

## No deprecated `.air.toml` form

`bin = "tmp/main"` is the legacy form. v2 of air uses `build.entrypoint = ["./tmp/main"]`. The grep pattern in `check-v1-leaks.sh:97` catches the legacy form on a `.air.toml` file.

## Single static binary

spin is one executable. No plugins, no init scripts, no companion daemon. `go install github.com/<org>/spin@latest` is the install path. The `internal/` packages are not separately importable - the public surface is the CLI.

## No scaffolded `spin.toml`

The renderer runs `deleteSpinToml` (template.go:148-166) on the destination after the post-hook. A `spin.toml` that accidentally ships in a template's `_base/` is silently removed from the user's project. The template author can never leak the manifest into a scaffolded output. See [Template engine](../concepts/template-engine.md) for TPL-16.

## No v1 runner, no per-ecosystem scaffolders

There is no `internal/runner/`, no `internal/ecosystems/`, no `cmd/new_charm.go` or `cmd/new_rust.go`. The codebase has exactly one scaffolder: `cmd/new.go`.

`spin new` reads an external `spin.toml` from a pinned template and renders its `_base/`. There is no `spin new go` vs `spin new rust` form - the template the user chose determines the output.

## What's configurable vs not

| Aspect | Configurable? | How |
| --- | --- | --- |
| Registry URL | yes | `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` |
| Cache dir | yes (Linux) | `XDG_CONFIG_HOME` |
| Cache dir (macOS) | yes | `HOME` |
| Cache dir (Windows) | yes | `AppData` |
| Templates dir (relative to cache) | no | always `<CacheDir>/templates` |
| `pinned.json` path | no | always `<CacheDir>/pinned.json` |
| HTTP timeout | no | hard-coded 15s in `client.go:45` |
| `GIT_TERMINAL_PROMPT` | no | always `0` in the spawned git env |
| 8 param types | no | fixed set in `internal/params/param.go:16-86` |
| The 9 text/template funcs | no | fixed set in `engine.go:34-75` |
| Post-hook shell | no | always `sh -c` |

## Edge cases

- **The `charm.land/glow/v2` path does not exist**: a user who tries to import it gets a `go get` error. The `glow` binary is the v2 distribution channel.
- **`CGO_ENABLED=0` with a dep that needs cgo**: build fails with a cgo toolchain error. Override `CGO_ENABLED=1` only if you can install the cgo toolchain.
- **Go 1.25 floor for scaffolded projects**: a user on Go 1.24 who scaffolds with `--bubbletea` will see a `go: command 'mod' requires go 1.25` error on their first `go mod tidy`. The scaffolder does not detect or warn about this. It's the user's responsibility to upgrade.
- **v1 grep suite is wide on purpose**: it catches patterns that no current template emits. False positives are possible if a project's code happens to look v1-shaped, but the patterns are designed against the v2 API surface specifically.
- **`hub` or `pony` from `charmbracelet/x`**: `x` is experimental and not pinned to a specific major version. Imports of `github.com/charmbracelet/x/...` are **not** in the deny-list (only the `charmbracelet/<lib>/v2` paths are). A generated project may import `x` subpackages; the grep suite won't flag them.

## Related

- [CLAUDE.md Constraints section](../../CLAUDE.md#constraints) - the source-of-truth list the scaffolder enforces.
- [Building](../development/building.md) - the `CGO_ENABLED=0` env block in Taskfile.
- [Scripts](../development/scripts.md) - what `check-v1-leaks.sh` actually grep'd for.
- [CI](../development/ci.md) - where the grep suite runs.
- [What is spin?](../overview/what-is-spin.md) - the "single static binary" pitch.
