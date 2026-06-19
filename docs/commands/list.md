# `spin list`

Show pinned templates. Reads `~/.config/spin/pinned.json` and renders a table (or JSON, for scripts).

Source: `cmd/list.go:23-139`.

## Synopsis

```sh
spin list          # table
spin list --json   # JSON
```

Also reachable as `spin add` (no args) and `spin add --list`. Both delegate to `execList` (cmd/list.go:57).

## Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--json` | bool | `false` | Emit a JSON array on stdout with no styling. |

`cobra.NoArgs`. No positional arguments.

## What it does

1. Calls `registry.Client.ListPinned()`. A missing or empty `pinned.json` returns `(nil, nil)` - not an error.
2. If the pin set is empty, prints a friendly "no pinned templates" line and a hint pointing to `spin add` (cmd/list.go:60-63).
3. Otherwise, builds a `[]pinnedRow` (cmd/list.go:44-51) and either:
   - Emits a JSON array via `json.NewEncoder(os.Stdout).SetIndent("", "  ")` (cmd/list.go:65-67). An empty set still emits `[]` so `jq` consumers don't choke.
   - Renders a lipgloss table with headers `NAME, VERSION, PINNED, DESCRIPTION, LOCAL PATH` (cmd/list.go:69-77). The 2-space left pad and no box-drawing borders come from the shared `printTable` helper in `cmd/print.go:82`.

The `pinnedDescription` helper (cmd/list.go:113-122) reads the `description` field from each pin's on-disk `spin.toml` so the table shows what the template says about itself, not just the URL. `shortenLocal` (cmd/list.go:128-139) renders the cache path relative to the cache root when possible.

## Examples

```sh
spin list
# NAME                  VERSION   PINNED                  DESCRIPTION                      LOCAL PATH
#   go-cli-template     abc123... 2026-06-14T22:27:00Z   Minimal Go CLI with cobra        templates/go-cli-template
#   rust-cli            def456... 2026-06-12T08:11:00Z   Rust CLI starter                 templates/rust-cli

spin list --json
# [
#   {
#     "name": "go-cli-template",
#     "version": "abc123...",
#     "pinned_at": "2026-06-14T22:27:00Z",
#     "description": "Minimal Go CLI with cobra",
#     "source": "https://github.com/me/go-cli-template.git",
#     "local_path": "/home/user/.config/spin/templates/go-cli-template"
#   },
#   ...
# ]
```

## Exit codes

- `0` - always (even when the pin set is empty, even when the JSON output is `[]`)

## Edge cases

- **Missing `pinned.json`**: treated as "no pins" - the empty-state hint or the empty JSON array. Not an error.
- **Empty `pinned.json`**: same as missing.
- **Corrupt `pinned.json`** (invalid JSON): `ListPinned` returns the parse error wrapped in `"registry: pinned.json: %w"`. The command surfaces it as a non-zero exit. The user can hand-edit the file (atomic, so a partial edit doesn't lose data) or delete it to start fresh.
- **Pinned template's `spin.toml` is unreadable** (e.g. the on-disk cache was clobbered): `pinnedDescription` returns "" and the table shows an empty description. The pin is still listed - the cache state is `spin update`'s problem, not `spin list`'s.
- **A `spin.lock` or `pinned.json.bak` file in the cache dir**: not a pin. `ListPinned` only reads the pin index; it doesn't walk the cache.
- **`XDG_CONFIG_HOME` override**: setting `XDG_CONFIG_HOME=/tmp/foo` redirects the whole `~/.config/spin/` tree. The test suite uses this for isolation (see [Testing](../development/testing.md)).

## Internal calls

- `registry.New()` (registry/client.go:31) - same `Client` constructor every command uses.
- `client.ListPinned()` (registry/client.go:387) - returns `(nil, nil)` for missing/empty.
- `printTable` or `json.Encoder` - the actual output.

## Related

- [`spin add`](add.md) - the write side of the same pin store.
- [`spin update`](update.md) - re-clones the cache, captures a new HEAD SHA.
- [`spin remove`](remove.md) - the unpin command.
- [Pinning model](../concepts/pinning.md) - the full `pinned.json` schema and the `Pinned` struct.
- [`internal/registry` package](../packages/registry.md) - the `ListPinned` source.
