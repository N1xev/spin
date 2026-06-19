# `internal/registry`

The pin store (`~/.config/spin/pinned.json`) and the public registry HTTP client. Atomic writes everywhere. The `Client` type is the entry point.

Source: `internal/registry/client.go`, `types.go`, `search.go`.

## The `Client` type

`client.go:20-24`:

```go
type Client struct {
    IndexURL string       // the search endpoint
    HTTP     *http.Client // 15s timeout
    CacheDir string       // where Pinned entries are stored
}
```

`New()` (client.go:31-48) reads the env vars in order: `SPIN_REGISTRY_URL` (preferred), `SPIN_REGISTRY` (fallback for v2.0-skeleton callers), then `DefaultIndexURL` (`https://registry.spin.invalid/v1`, types.go:14). The `.invalid` TLD is RFC 2606 reserved and never resolves, so `Search` always hits the friendly "not yet deployed" path until a real server is deployed. `CacheDir` is `os.UserConfigDir()/spin`.

## The `Pinned` type

`types.go:53-59`:

```go
type Pinned struct {
    Name      string
    Source    string
    PinnedAt  string
    Version   string
    LocalPath string
}
```

`Version` is `"local"` for local-path sources, a 40-char git SHA for git sources (or the literal string `"git"` if `git rev-parse HEAD` failed on the clone). `PinnedAt` is set by `cmd/add.go:52`, not by the registry layer.

`LocalPath` defaults to `<CacheDir>/templates/<Name>` in `Pin` for older callers (client.go:413-415).

## Search

`Search(query) (*SearchResult, error)` and `SearchWithLimit(query, limit) (*SearchResult, error)` (client.go:54-92). GET `<IndexURL>/search?q=<query>`. The limit clamps the response slice to the first N entries (client.go:88-90).

Error mapping (client.go:99-130):

- DNS failure → `ErrNotDeployed`
- Connection refused / timeout / unreachable host → `ErrNotDeployed`
- HTTP 404 → `ErrNotDeployed`
- Other HTTP failures → `fmt.Errorf("registry: %s: %s", status, body)` - the status code + the body
- Malformed JSON → `fmt.Errorf("registry: decode: %w", err)`

`isNetworkError` uses both `errors.As` against `*net.DNSError` and `*net.OpError`, AND a string-inspection fallback for messages that don't unwrap cleanly (e.g. "context deadline exceeded" from the 15s HTTP timeout).

`SearchResult` (types.go:43-47): `{Query, Total, Entries []Entry}`. `Entry` (types.go:30-40): `{name, description, tags, language, type, version, downloads, source, updated_at}`. The shape is "what the registry server will speak" - the wire format.

## Add / Pin / Unpin

`Add(spec)` (client.go:144-159) dispatches the spec:

- `user/repo` shorthand: `isShorthand` (client.go:165-174) matches exactly one slash, no scheme, no leading `.`/`~`. `expandShorthand` (client.go:177-179) returns `https://github.com/<shorthand>.git`.
- Local path: `addLocal` (client.go:181-220) tries `os.Symlink` first, falls back to `copyDir` on failure.
- Git URL: `addGit` (client.go:222-259) shallow-clones with `GIT_TERMINAL_PROMPT=0` and 2-minute context timeout, then captures the HEAD SHA.

`sanitiseRepoName` (client.go:356-374): drops known URL schemes, then `git@`, then takes everything after the last `/` or `:`, then strips a trailing `.git`. `https://github.com/foo/bar.git` → `bar`. `git@github.com:foo/bar.git` → `bar`.

`Pin(p)` (client.go:408-425): atomic write via `writePinned`. Replaces by `Name` if it already exists, otherwise appends. `LocalPath` is defaulted to `<CacheDir>/templates/<Name>` if empty.

`Unpin(name)` (client.go:430-439): in-place filter + atomic write. Does NOT touch the on-disk cache - that's `spin remove --purge`.

`ListPinned()` (client.go:387-403): missing or empty `pinned.json` returns `(nil, nil)` - "no pins" is a normal state, not a failure.

`PinnedPath()` (client.go:380-382): `<CacheDir>/pinned.json`.

## Refresh

`Refresh(pin)` (client.go:451-498): branches on `isLocalPath(pin.Source)` vs `isGitURL(pin.Source)`.

- Local: blows away `pin.LocalPath` and recopies from `pin.Source`. Errors if the source is gone - no silent stale-cache fallback.
- Git: re-clones `git clone --depth=1 <pin.Source> <pin.LocalPath>` with `GIT_TERMINAL_PROMPT=0` and 2-minute context timeout. Re-captures the HEAD SHA.

Returns the updated `Pinned` by value. The caller is expected to `Pin(updated)` to persist.

## Atomic writes

`writePinned(all)` (client.go:504-538):

1. Marshal `all` to JSON (indented).
2. `os.CreateTemp(<dir>, ".pinned-*.json.tmp")`.
3. Write, `Sync`, `Close`.
4. `os.Rename(tmp, final)`.
5. On any failure before the rename, the temp file is removed.

This prevents a partial write (process killed, disk full) from leaving `pinned.json` in a corrupt state. The `Spin` and `Unpin` paths both go through it.

## Search formatting

`FormatSearch(r, plain) string` (search.go:10-25): a human-readable table for the terminal. The `plain` flag is for future use; currently always styled.

`SortByPopularity(es) []Entry` (search.go:28-32): sorts by `Downloads` desc. Stable for equal counts (slice order is preserved by `sort.SliceStable`).

## How it fits in the pipeline

The registry pipeline. Triggered by `spin add` / `list` / `update` / `remove` / `search`. Shares the `Template` type with the template pipeline at the boundary: `addGit` calls `template.Detect` after a successful clone.

```
spin add     -> Client.Add(spec)  -> Client.Pin
spin list    -> Client.ListPinned
spin update  -> Client.Refresh    -> Client.Pin
spin remove  -> Client.Unpin      -> optional os.RemoveAll
spin search  -> Client.Search
```

## Edge cases

- **No `SPIN_REGISTRY_URL` set**: defaults to `https://registry.spin.invalid/v1`. DNS lookup fails fast (no 15s wait), `Search` returns `ErrNotDeployed`, the user gets the friendly "not yet deployed" message.
- **`.invalid` TLD trick**: deliberate. RFC 2606 reserves it, so it never resolves. Cheaper than the old "connection refused" wait, and the failure mode is the same.
- **Shorthand detection rejects paths and URLs**: `spin add foo/bar` is shorthand, `spin add ./foo/bar` is a local path, `spin add https://...` is a git URL. The check is on the first character + slash count.
- **Local path `~` expansion**: `addLocal` calls `expandHome` (client.go:263-275) which handles `~` and `~/` but not `~user/...`.
- **Symlink vs copy**: `addLocal` tries `os.Symlink` first. On Windows without SeCreateSymbolicLinkPrivilege, or on filesystems that don't support symlinks, falls back to `copyDir`. Symlinks inside the source tree are re-created as symlinks.
- **Two URLs that resolve to the same basename collide**: `https://github.com/foo/bar.git` and `git@github.com:foo/bar.git` both pin as `bar`. The second `add` wipes the first's cache (since the second `addGit` removes any existing dest at client.go:230-232).
- **`XDG_CONFIG_HOME` override**: setting it redirects the whole `~/.config/spin/` tree. The test suite uses this for isolation (see [Testing](../development/testing.md)).

## Tests

- `internal/registry/client_test.go` - the spec dispatcher, the SHA capture, the atomic writes.

## Related

- [Pinning model](../concepts/pinning.md) - the `pinned.json` schema and the shorthand expansion.
- [XDG layout and env vars](../concepts/xdg-layout.md) - the env vars and the cache layout.
- [Registry protocol](../concepts/registry-protocol.md) - the search wire format in detail.
- [`internal/template` package](template.md) - the renderer that consumes the pin store via the loader.
- [`spin add`](../commands/add.md), [`spin list`](../commands/list.md), [`spin update`](../commands/update.md), [`spin remove`](../commands/remove.md), [`spin search`](../commands/search.md).
