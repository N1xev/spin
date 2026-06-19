# Pinning model

How spin remembers a template for offline use, refreshes it, and finds new ones. The pin store is `~/.config/spin/pinned.json`; the cache is `~/.config/spin/templates/<name>/`.

Source: `internal/registry/client.go`, `cmd/update.go`, `cmd/remove.go`, `cmd/add.go`.

## The `Pinned` struct

Defined in `internal/registry/types.go:53-59`:

```go
type Pinned struct {
    Name      string `json:"name"`
    Source    string `json:"source"`
    PinnedAt  string `json:"pinned_at"`
    Version   string `json:"version"`
    LocalPath string `json:"local_path"`
}
```

| Field | Source | Meaning |
| --- | --- | --- |
| `Name` | The cache directory basename. | The pin's identifier. Used by `spin new --template <name>`. |
| `Source` | The original spec. | What `spin update` re-clones from. May be a URL or a local path. |
| `PinnedAt` | `time.Now().UTC().Format(time.RFC3339)` (set in `cmd/add.go:52`). | When the pin was created. Not updated by `spin update`. |
| `Version` | Git HEAD SHA, or `"local"`, or `"git"`. | What the cache currently contains. The SHA is the freshness marker. |
| `LocalPath` | `<CacheDir>/templates/<Name>` (defaulted in `Pin` at client.go:413-415). | Where the on-disk cache lives. |

`Version` is `"local"` for local-path sources. For git sources, it's the 40-char HEAD SHA captured by `git -C <dest> rev-parse HEAD` (client.go:322-330), or the literal string `"git"` if the capture failed (no git on PATH, empty repo, etc).

## The on-disk layout

```
~/.config/spin/
  pinned.json             # the pin index, JSON array of Pinned
  templates/
    go-cli-template/      # symlink to ~/code/templates/go-cli (local)
    rust-cli/             # git clone --depth=1 of the source
    my-template/          # another git clone
```

For local-path sources, the cache entry is a **symlink** to the original (client.go:208). On Windows or filesystems that don't support symlinks, falls back to a recursive copy. For git sources, it's a `git clone --depth=1` result with `GIT_TERMINAL_PROMPT=0` in the env.

## The `pinned.json` file

A top-level JSON array. Written atomically (client.go:504-538):

1. Marshal to JSON (indented).
2. Write to a sibling temp file (`.pinned-*.json.tmp`).
3. `Sync` + `Close`.
4. `os.Rename` over the real file.

This prevents a partial write (process killed, disk full) from leaving `pinned.json` in a corrupt state. The `Pin` and `Unpin` paths both go through it.

A missing or empty file is `(nil, nil)` from `ListPinned` (client.go:387-403) - "no pins" is a normal state, not a failure.

## `user/repo` shorthand

`isShorthand` (client.go:165-174) matches a string with exactly one slash, no scheme, no leading `.`/`~`, both sides non-empty. So:

- `charmbracelet/bubbletea` → shorthand
- `./my-template` → local path (not shorthand)
- `https://github.com/foo/bar` → git URL (not shorthand)
- `foo/bar/baz` → not shorthand (two slashes)

`expandShorthand` (client.go:177-179) returns `https://github.com/<shorthand>.git`. So `spin add charmbracelet/bubbletea` clones `https://github.com/charmbracelet/bubbletea.git`.

The shorthand is only expanded in `Client.Add` (called by `spin add`). The loader (`internal/template/loader.go:67-84`) does **not** expand shorthand; it expects `spin add` to have already produced a pin. So `spin new --template charmbracelet/bubbletea` errors with "re-run `spin add`" until you pin it first.

## The pin name

The pin's `Name` is the cache directory basename, which comes from the source:

- `user/repo` → the `repo` part. `charmbracelet/bubbletea` → `bubbletea`.
- `https://github.com/foo/bar.git` → `bar` (via `sanitiseRepoName`, client.go:356-374).
- `git@github.com:foo/bar.git` → `bar` (same).
- Local path → `filepath.Base(src)`.

Two URLs that resolve to the same basename collide. `https://github.com/foo/bar.git` and `git@github.com:foo/bar.git` both pin as `bar`. The second `spin add` wipes the first's cache (since `addGit` removes any existing dest at client.go:230-232).

## The rollback contract in `spin update`

`cmd/update.go:118-187`. Three phases per pin:

1. **Backup**: rename `LocalPath` to `<LocalPath>.bak-<unix-ts>`. The timestamp suffix prevents rapid-fire updates from clobbering each other (cmd/update.go:193-199).
2. **Refresh in place**: call `Client.Refresh(pin)`. Local paths are recopied; git sources are re-cloned.
3. **Persist**: call `Client.Pin(updated)`. The new SHA goes into `pinned.json`.

If any step in phase 2 or 3 fails, phase 1's backup is renamed back so on-disk state matches the (now still-old) pin record. A failed rollback is reported as a warning, not a fatal error.

The contract: **on-disk cache matches `pinned.json` after `spin update` returns, regardless of success**.

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

## `spin remove` and `--purge`

`cmd/remove.go`. Default: unpin only. With `--purge`: also `os.RemoveAll(LocalPath)`. The default keeps the cache so a future `spin add` reuses it (no network round-trip). A typo in the name shouldn't lose data.

`Unpin` (client.go:430-439) only edits `pinned.json`. It does **not** touch the on-disk cache. The cache delete is `spin remove --purge`'s job, not `Unpin`'s.

## HEAD SHA capture

`gitHeadSHA(dir)` (client.go:322-330) runs `git -C <dir> rev-parse HEAD`. Called after every successful git operation:

- `addGit` (client.go:248-251): on the initial clone.
- `Refresh` (client.go:490-493): on the re-clone.

Best-effort: if it fails, `Version` is set to `"git"` instead of the SHA. The user can still see the pin is git-sourced (the source URL is the truth), they just don't get a precise SHA to compare against.

## JSON wire format

`spin list --json` emits the `pinnedRow` view (cmd/list.go:44-51), which is JSON-stable: `name`, `version`, `pinned_at`, `description` (read from the on-disk `spin.toml`), `source`, `local_path`. This is the same shape the registry server will speak (modulo the `description` field, which is local-only).

## Edge cases

- **Shorthand detection rejects paths and URLs**: the check is on the first character of the spec (client.go:165-167).
- **Local path `~` expansion**: `addLocal` calls `expandHome` (client.go:263-275) which handles `~` and `~/` but not `~user/...`.
- **Symlink vs copy**: `addLocal` tries `os.Symlink` first. On Windows without SeCreateSymbolicLinkPrivilege, or on filesystems that don't support symlinks, falls back to `copyDir`.
- **Two URLs that resolve to the same basename collide**: the second `add` wipes the first's cache.
- **Local path that has been deleted on disk**: `Client.Refresh` errors with "source %s is gone" (client.go:472-474). The cache is **not** silently kept stale; the user must re-pin or restore the source.
- **Pin record is missing `LocalPath`**: defaulted in `Pin` to `<CacheDir>/templates/<Name>` (client.go:413-415). So older pin records without the field work transparently with newer code.

## Related

- [XDG layout and env vars](xdg-layout.md) - where `pinned.json` and the cache live on disk.
- [`spin add`](../commands/add.md), [`spin list`](../commands/list.md), [`spin update`](../commands/update.md), [`spin remove`](../commands/remove.md).
- [`internal/registry` package](../packages/registry.md) - the `Client` and `Pinned` types.
