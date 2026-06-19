# XDG layout and env vars

Where spin puts things on disk, and which environment variables it honours.

Source: `internal/registry/client.go:31-48`, `internal/template/loader.go:341-352`.

## The base directory

`os.UserConfigDir()` is the source of truth. It returns:

- **Linux**: `$XDG_CONFIG_HOME` if set, else `$HOME/.config`.
- **macOS**: `$HOME/Library/Application Support`.
- **Windows**: `%AppData%`.

spin uses `<UserConfigDir>/spin/` as its `CacheDir` (client.go:39-46). The fallback chain in `defaultCacheDir` (loader.go:341-352): `os.UserConfigDir()` → `sh -c 'echo $HOME'` (with a leading `.config/spin/templates`) → `/tmp/spin-templates`.

## The filesystem layout

```
<UserConfigDir>/spin/
  pinned.json             # the pin index, JSON array of Pinned
  templates/
    <name>/               # per-pin cache
```

`<UserConfigDir>/spin/templates/<name>/` holds the per-pin cache. For local-path sources, the entry is a symlink to the original (client.go:208). For git sources, it's a `git clone --depth=1` result.

## Environment variables

### `SPIN_REGISTRY_URL` (preferred)

Overrides the search endpoint. `Client.New()` (client.go:31-48) reads this first.

```sh
SPIN_REGISTRY_URL=https://my-registry.example.com/v1 spin search go
```

### `SPIN_REGISTRY` (fallback)

For v2.0-skeleton callers. Honored if `SPIN_REGISTRY_URL` is unset. Same shape. Documented in `client.go:26-30`.

### `XDG_CONFIG_HOME` (Linux only)

Redirects the entire `<UserConfigDir>` on Linux. The test suite uses this for isolation:

```go
t.Setenv("XDG_CONFIG_HOME", t.TempDir())
```

This puts `pinned.json` and `templates/` inside the temp dir, so tests don't pollute `~/.config/spin/`.

### `GIT_TERMINAL_PROMPT=0` (set internally, not user-overridable)

`Client.addGit` (client.go:240), `Client.Refresh` (client.go:486), and `Loader.cloneGit` (loader.go:179) all set this in the env passed to `git clone`. Prevents a missing/expired credential from blocking the scaffolder with a password prompt.

If a git source requires authentication (private repo, expired PAT), the clone fails fast with a non-zero exit. The user sees the error in the post-failure line of `dogfood.sh`-style scripts.

### Default registry URL

`https://registry.spin.invalid/v1` (types.go:14). The `.invalid` TLD is RFC 2606 reserved and never resolves. So `Search` always hits the friendly "not yet deployed" path until a real server is deployed - **no 15-second DNS wait**.

## What's NOT configurable

- The templates cache dir. There's no env var to override `<CacheDir>/templates`. If you need a different location, override `XDG_CONFIG_HOME` (Linux only).
- The `pinned.json` path. Same - it's always `<CacheDir>/pinned.json`. Atomic writes protect against partial updates, so the path being "wherever the OS thinks config goes" is fine.
- The `GIT_TERMINAL_PROMPT=0` setting. Hard-coded in three places (above). A user who wants to be prompted for credentials can shell out to `git` themselves.

## Edge cases

- **`HOME` unset**: `os.UserConfigDir` may return an error. The fallback chain in `defaultCacheDir` shelles out to `sh -c 'echo $HOME'` to get it. The final fallback is `/tmp/spin-templates`, which is writable in normal setups but may not be on locked-down systems.
- **macOS `~/Library/Application Support/spin/`**: long path. The `spin list` table truncates the local path with `shortenLocal` (cmd/list.go:128-139) when it can show a path relative to the cache root.
- **Windows `%AppData%\spin\`**: the `LocalAppData` vs `Roaming` distinction matters. `os.UserConfigDir` returns `Roaming`, which is fine for spin (the data is small and personal).
- **Tests in CI without a home dir**: set `XDG_CONFIG_HOME` and `HOME` to a temp dir, both. `cmd/doctor_test.go` does this.
- **Symlinks in the source path** (local-path pin): the source path is followed to a real directory; the cache is a symlink back to it. Renaming the source breaks the symlink. The next `spin update` errors with "source %s is gone".

## Related

- [Pinning model](pinning.md) - what goes in `pinned.json` and the cache.
- [Registry protocol](registry-protocol.md) - the search HTTP API and the `.invalid` TLD trick.
- [Environment variables](../reference/env-vars.md) - the env var reference.
- [`internal/registry` package](../packages/registry.md) - the `Client` constructor and the env-var read order.
