# Scripts

The two shell scripts that wrap the dev workflow: `check-v1-leaks.sh` and `dogfood.sh`. The Taskfile targets call them; you can also call them directly.

Source: `scripts/check-v1-leaks.sh`, `scripts/dogfood.sh`.

## `scripts/check-v1-leaks.sh`

Greps a Go project for **v1 charmbracelet API/path leaks**. The second line of defense after the post-scaffold `go build` smoke test - it catches patterns that compile but are semantically wrong.

### Usage

```sh
bash scripts/check-v1-leaks.sh <project-dir>
```

Exits:

- `0` if no v1 pattern is found.
- `1` (with the offending lines printed to stderr) on any match.
- `2` on a missing arg or non-existent directory.

### What it checks

**V1 import path deny-list** (`V1_LEAK_PATTERNS`, lines 47-64). 16 patterns covering every charmbracelet module that migrated to `charm.land/<lib>/v2`. The closing-quote anchor (`"`) means `harmonica` is **not** matched (the matched library list excludes it because it pre-dates the migration), but `bubbletea"` is.

Deliberately excluded:

- `github.com/charmbracelet/harmonica` - v0.2.0 pre-dates the migration; still on github.com.
- `github.com/charmbracelet/glow/v2` - the v2 glow binary lives on github.com; `charm.land/glow/v2` does not exist.

**V2 API regression patterns** (`GO_PATTERNS`, lines 69-92). 16 v1-style API calls that were removed or renamed in v2:

| Pattern | What it catches |
| --- | --- |
| `View() string` | The v1 `tea.Model` signature. v2 returns `tea.View`. |
| `tea.WithAltScreen` | v1 program option. v2 puts `AltScreen` on the `tea.View`. |
| `tea.WithMouseCellMotion` | Same - v1 program option. |
| `tea.EnterAltScreen` / `ExitAltScreen` / `HideCursor` | v1 manual screen management. v2 uses the `View` fields. |
| `lipgloss.NewRenderer` / `DefaultRenderer` / `SetDefaultRenderer` | v1 renderer API. v2 uses styles directly. |
| `lipgloss.AdaptiveColor{...}` | v1 color type. v2 has its own color model. |
| `lipgloss.ColorProfile(...)` | v1 capability probe. v2 has a different mechanism. |
| `lipgloss.HasDarkBackground()` | v1 background detection. v2 has a different API. |
| `tea.KeyCtrlC` / `MouseButtonLeft/Right/Middle` | v1 key/mouse constants. v2 uses typed msgs. |
| `msg.Type` / `msg.Runes` / `msg.Alt` / `msg.X` / `msg.Y` | v1 message-field access. v2 uses typed fields. |

**Air config** (`AIR_PATTERNS`, lines 96-98). One pattern: the legacy `bin = "tmp/main"` form in `.air.toml`. The modern equivalent is `build.entrypoint = ["./tmp/main"]`.

### How it runs

The Taskfile target (`Taskfile.yml:34-38`) runs it against `./internal` and `./cmd`:

```sh
bash scripts/check-v1-leaks.sh ./internal
bash scripts/check-v1-leaks.sh ./cmd
```

CI runs only `./internal` (`.github/workflows/ci.yml:48-49`). The grep suite is wide on purpose: it catches patterns that no current template emits, so a future template regression is caught before it ships.

## `scripts/dogfood.sh`

End-to-end smoke test. If you change the scaffolder source, run this before pushing; CI runs the same pipeline.

### Usage

```sh
bash scripts/dogfood.sh
```

Exits 0 on a clean repo. On any step failure, prints the failing step name and the last 50 lines of its captured output, then exits 1. The work dir is preserved on failure (the EXIT trap is disabled) so you can inspect the partial output.

### Pipeline

1. **Build** the spin binary at `$REPO_ROOT/bin/spin`.
2. **`spin init starter`** in the work dir. Asserts the starter has both `spin.toml` and `_base/file.txt.tmpl`.
3. **`spin new spin-fixture --template <starter> --param license=MIT --dest <out>`**. Asserts the rendered file contains `spin-fixture` (proves `text/template` ran) and that no `spin.toml` leaked into the dest (TPL-16).
4. **`spin list --json`** against an isolated `XDG_CONFIG_HOME`. Confirms the pin store + JSON wire format still work.
5. **`go test ./... -count=1`** - the in-tree test suite.

### Work dir

`$REPO_ROOT/.tmp/dogfood-$$` where `$$` is the shell PID. The dir is created at start, removed by the EXIT trap on success, and **preserved on failure** for debugging. It is `.gitignore`d (the Taskfile `clean` target also removes it).

Why not `mktemp -d`? `go mod tidy` and `go build` (run inside scaffolded Go projects during the old full-stack pipeline) refuse to walk up from `/tmp` because its mode 1777 makes it a "system temp root" in Go's view. The dogfood script keeps everything under `$REPO_ROOT` so the Go tooling walks up to a normal module root.

### Why both scripts are bash, not Go

`check-v1-leaks.sh` is a pure grep pipeline - bash is the shortest expression. `dogfood.sh` orchestrates multiple `go build` / `spin ...` calls and asserts on the outputs; turning it into a Go test would require re-implementing process orchestration that bash gives for free. Both are testable in their own right: a `bash -n` syntax check, plus the CI run that exercises them on every push.

## Edge cases

- **`check-v1-leaks.sh` against a path with no Go files**: exits 0 with the message `OK: no v1 leaks detected in <dir>`. The grep finds nothing, the loop body never runs, FAIL stays 0.
- **`check-v1-leaks.sh` on a binary file**: `grep -rEn` skips binary files by default and reports "Binary file FOO matches" on stderr. The CI test step does not check the script's stderr, so a binary file with a v1-looking path inside it would be silently skipped. In practice this is fine - we only ever point it at `.go` / `.tmpl` / `.air.toml`.
- **`dogfood.sh` re-run after a clean `task clean`**: the work dir is recreated. No stale state survives.
- **`dogfood.sh` killed mid-pipeline**: the EXIT trap is only armed for the success path. A `Ctrl-C` may leave the work dir around. The trap is then re-armed with `trap - EXIT` on a failure to preserve the dir; a `Ctrl-C` is SIGINT, which the script doesn't trap. The `.tmp` is in `.gitignore`, so it's safe to leave for a manual `rm -rf .tmp`.

## Related

- [Building](building.md) - the `task build` and ldflags recipe.
- [Testing](testing.md) - the `go test ./... -count=1` half of dev workflow.
- [CI](ci.md) - the GitHub Actions workflows that mirror these scripts.
- [Architecture overview](../overview/architecture.md) - where the scaffolder fits in the bigger picture.
