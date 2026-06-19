# `spin remove <name>`

Drop a pin from `~/.config/spin/pinned.json`. With `--purge`, also delete the on-disk cache.

Source: `cmd/remove.go:22-85`.

## Synopsis

```sh
spin remove <name>          # drop the pin
spin remove <name> --purge  # drop the pin AND delete the cache
```

Alias: `spin rm <name>` (cmd/remove.go:26).

## Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--purge` | bool | `false` | Also `os.RemoveAll` the pin's `LocalPath` (the cache directory). |

`<name>` is a positional argument. `cobra.ExactArgs(1)`.

## What it does

1. Reads the current pin set via `Client.ListPinned()`.
2. Looks up the pin by name. **Not found is a hard error** - the command does not silently no-op on typos (cmd/remove.go:55-62). The error message lists the known names so the user can see what they meant.
3. Calls `Client.Unpin(name)`. This only edits `pinned.json`; the on-disk cache is left alone.
4. If `--purge` is set AND the pin had a non-empty `LocalPath`, calls `os.RemoveAll(LocalPath)`. A failure here is reported as a warning but does not change the exit code (cmd/remove.go:74-83).
5. Prints a success line: "unpinned" for the default, "unpinned and purged cache" for `--purge`.

## Why the default keeps the cache

`spin remove` without `--purge` leaves the on-disk clone in place. Two reasons:

- Re-pinning (`spin add <same-source>`) is then free - no network round-trip, the existing clone is reused.
- A typo in the name shouldn't lose data. The user can `spin add` it back.

If the user wants the disk space back, `--purge` does it.

## Examples

```sh
spin remove go-cli-template
# -> unpinned go-cli-template (cache kept at /home/user/.config/spin/templates/go-cli-template)

spin remove go-cli-template --purge
# -> unpinned and purged cache go-cli-template

# Alias
spin rm rust-cli --purge
```

## Exit codes

- `0` - success (pin removed; cache optionally purged)
- `1` - pin not found, or `pinned.json` is corrupt

## Edge cases

- **Unknown name**: hard error with a hint listing the known names (cmd/remove.go:55-62). Not a silent no-op.
- **`pinned.json` is missing or empty**: the pin lookup fails with "not found", same error path as an unknown name.
- **`--purge` but the pin has no `LocalPath`**: silently no-ops the cache delete (cmd/remove.go:74-77). The default `LocalPath` is `<CacheDir>/templates/<name>` for older callers (registry/client.go:413-415), so this only happens if the pin record is malformed.
- **`--purge` but the cache delete fails** (permissions, etc.): the unpin has already succeeded, the cache is left in an inconsistent state with the pin. Reported as a warning via `printHint`; the command exits 0.
- **Race with a concurrent `spin update`**: not really a race - the unpin and update both touch `pinned.json` via `writePinned`'s atomic rename (client.go:504-538). Whichever rename lands last wins; the other side gets an error on its next read.
- **`.bak-<ts>` siblings in the cache dir**: not touched by `spin remove` (purge or no). If one is there from a crashed `spin update`, it survives the purge. Clean it up by hand.

## Internal calls

- `registry.New()` (registry/client.go:31)
- `client.ListPinned()` (registry/client.go:387) - to find the pin and report typos.
- `client.Unpin(name)` (registry/client.go:430) - in-place filter + atomic write.
- `os.RemoveAll(LocalPath)` - only when `--purge`.

## Related

- [`spin add`](add.md) - the inverse; creates the pin and the initial cache.
- [`spin update`](update.md) - re-clones the cache; does not touch `pinned.json` other than to update the SHA.
- [`spin list`](list.md) - the read side; the source of the "known names" error hint.
- [Pinning model](../concepts/pinning.md) - the `Pinned` struct and the `LocalPath` defaulting.
- [`internal/registry` package](../packages/registry.md) - the `Unpin` source.
