# `spin update [name]`

Refresh a pinned template's on-disk cache. Captures a new HEAD SHA. Rolls back on failure.

Source: `cmd/update.go:26-203`.

## Synopsis

```sh
spin update           # refresh every pin
spin update <name>    # refresh one pin
```

## Flags

None.

`<name>` is optional. `cobra.MaximumNArgs(1)`. With no name, every pin in `pinned.json` is refreshed in order.

## What it does

For each target pin, `runUpdate` calls `refreshOne` (cmd/update.go:118-187), which has a three-phase rollback contract:

1. **Backup the existing cache** - rename `LocalPath` to `<LocalPath>.bak-<unix-ts>`. The timestamp suffix prevents rapid-fire updates from clobbering each other (cmd/update.go:193-199).
2. **Refresh in place** - call `registry.Client.Refresh(pin)`. For local-path sources: blow away the cache and recopy from the source path. For git sources: re-clone `git clone --depth=1 <source> <LocalPath>` with `GIT_TERMINAL_PROMPT=0` and a 2-minute context timeout. Re-capture the HEAD SHA.
3. **Persist the new pin record** - call `Client.Pin(updated)`. The new SHA goes into `pinned.json`.

If any step in phase 2 or 3 fails, phase 1's backup is renamed back so on-disk state matches the (now still-old) pin record. A failed rollback is reported as a warning, not a fatal error (the original error wins).

The summary line per pin shows `updated <name> -> <sha-prefix>` where `<sha-prefix>` is the first 10 characters of the new HEAD SHA. For local-path pins the line is `updated <name> -> local`.

Returns an error if any pin's refresh failed, after printing all per-pin results.

## Examples

```sh
# Refresh one pin
spin update go-cli-template
# -> updated go-cli-template -> 1f2e3d4c5b (was 0a9b8c7d6e)

# Refresh every pin
spin update
# -> updated go-cli-template -> 1f2e3d4c5b (was 0a9b8c7d6e)
# -> updated rust-cli       -> 9z8y7x6w5v
# -> noop  local-template  -> local (unchanged)
```

## Exit codes

- `0` - every pin refreshed successfully
- `1` - at least one pin failed (and was rolled back); the user sees the per-pin results and the failure reason

## Edge cases

- **No pins**: errors out with "no pinned templates" plus a hint pointing to `spin add` (cmd/update.go:53-60).
- **Unknown name**: errors with a hint listing the known pin names (cmd/update.go:80-87). Not a silent no-op, so typos are visible.
- **Local path source that has been deleted on disk**: `Client.Refresh` errors with "source %s is gone: %w" (client.go:472-474). The cache is **not** silently kept stale; the user must re-pin or restore the source.
- **Network failure during a git refresh**: the existing cache is restored from the `.bak-<ts>` snapshot. A subsequent `spin update` is fine - the backup timestamp is unique per call.
- **Pin persistence fails after a successful refresh** (e.g. `pinned.json` is now read-only): the cache is rolled back too, so on-disk state matches the pin record. The error wins; the user sees the original failure.
- **`.bak-<ts>` is left behind**: should not happen - on success the backup is `os.RemoveAll`'d, on failure it's renamed back. If one is ever found on disk, the most recent `spin update` was killed mid-rollback; manual cleanup is fine.
- **Two pins with the same `LocalPath`**: not possible by construction (the registry's `addGit` and `addLocal` use the same `<CacheDir>/templates/<basename>` path). But two pins with the same *name* would overwrite each other in `pinned.json` (`Pin` replaces by name, client.go:417-420).

## The rollback contract

The contract is: on-disk cache matches `pinned.json` after `spin update` returns, regardless of success. The pre-refresh backup makes this cheap - the rollback is a single `os.Rename`, not a network round-trip.

```
spin update <name>
  -> mv LocalPath  LocalPath.bak-<ts>
  -> Refresh(LocalPath)
       success: write updated pin; rm -rf LocalPath.bak-<ts>
       failure: mv LocalPath.bak-<ts>  LocalPath; return error
  -> Pin(updated)
       success: done
       failure: mv LocalPath.bak-<ts>  LocalPath; return error
```

## Internal calls

- `registry.New()` (registry/client.go:31)
- `client.ListPinned()` (registry/client.go:387)
- `client.Refresh(pin)` (registry/client.go:451) - the local-vs-git dispatcher.
- `client.Pin(updated)` (registry/client.go:408) - atomic write.

## Related

- [`spin add`](add.md) - creates the pin and the initial cache.
- [`spin list`](list.md) - shows the current SHAs.
- [`spin remove`](remove.md) - drops the pin (and optionally the cache).
- [Pinning model](../concepts/pinning.md) - the full `Pinned` struct and the SHA capture.
- [`internal/registry` package](../packages/registry.md) - the `Refresh` and `Pin` sources.
