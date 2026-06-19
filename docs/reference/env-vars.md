# Environment variables

The env vars spin reads (and one it sets internally). The precedence chain, the test-isolation overrides, and the env vars that exist but are not user-overridable.

Source: `internal/registry/client.go:31-48`, `internal/template/loader.go:341-352`, `internal/registry/client.go:240-251`.

## The full list

| Var | Read by | Effect | Default |
| --- | --- | --- | --- |
| `SPIN_REGISTRY_URL` | `Client.New` (client.go:31-35) | Overrides the search endpoint. | unset |
| `SPIN_REGISTRY` | `Client.New` (client.go:36-39) | Fallback URL for v2.0-skeleton callers. | unset |
| `XDG_CONFIG_HOME` | `os.UserConfigDir` (loader.go:341-352) | Redirects `~/.config/` on Linux. | unset (so `~/.config` is used) |
| `HOME` | `os.UserConfigDir` fallback (loader.go:347-350) | Linux path resolution, macOS Library path. | unset |
| `AppData` | `os.UserConfigDir` (Windows) | `%AppData%` location. | unset |
| `GIT_TERMINAL_PROMPT` | Set internally to `0` in three places (see below) | Disables `git`'s interactive credential prompts. | set to `0` by spin |

## `SPIN_REGISTRY_URL` (preferred)

Overrides the search endpoint. The full URL, including scheme and any path prefix:

```sh
SPIN_REGISTRY_URL=https://my-registry.example.com/v1 spin search go
```

The client does not append a version segment; the var should point at the full search base (the client builds `<URL>/search?q=<q>`). A trailing slash is harmless - `Client.Search` does `c.IndexURL + "/search"` (client.go:64).

## `SPIN_REGISTRY` (fallback)

Honored only if `SPIN_REGISTRY_URL` is unset. Same shape. Exists for compatibility with v2.0-skeleton callers that hard-coded the older env var name. New code should use `SPIN_REGISTRY_URL`.

## `XDG_CONFIG_HOME` (Linux only)

`os.UserConfigDir` returns `$XDG_CONFIG_HOME` if set, else `$HOME/.config`. spin uses this to locate `<dir>/spin/`. Setting `XDG_CONFIG_HOME=/tmp/foo` makes spin's `pinned.json` and `templates/` live under `/tmp/foo/spin/`.

The test suite uses this for isolation (see [Testing](../development/testing.md)):

```go
t.Setenv("XDG_CONFIG_HOME", t.TempDir())
```

## `HOME` (Linux + macOS)

Used in two places:

1. **Linux** - `os.UserConfigDir` returns `$XDG_CONFIG_HOME` if set, else `$HOME/.config`. Setting `HOME=/tmp/foo` with no `XDG_CONFIG_HOME` makes spin's cache live at `/tmp/foo/.config/spin/`.
2. **macOS** - `os.UserConfigDir` returns `$HOME/Library/Application Support/spin/` regardless of `XDG_CONFIG_HOME`. To redirect on macOS, set `HOME`.

`loader.go:347-350` shells out to `sh -c 'echo $HOME'` as a fallback if `os.UserConfigDir` returns an error (e.g., `HOME` unset, `/etc/passwd` missing entries). The final fallback is `/tmp/spin-templates`.

## `AppData` (Windows)

`os.UserConfigDir` returns `%AppData%` on Windows. Tests on Windows can `t.Setenv("AppData", t.TempDir())` for the same isolation effect as `XDG_CONFIG_HOME` on Linux.

## `GIT_TERMINAL_PROMPT=0` (set internally, not overridable)

Set in three places:

- `Client.addGit` (client.go:240) - the initial `git clone` from `spin add`.
- `Client.Refresh` (client.go:486) - the re-clone in `spin update`.
- `Loader.cloneGit` (loader.go:179) - the clone performed by the loader when no pin exists.

The setting is in the env passed to `git`, not the parent process env. So `GIT_TERMINAL_PROMPT=1` in the user's shell does **not** override it for the spawned `git` process. A user who wants to be prompted for credentials must shell out to `git` themselves before invoking spin.

A missing or expired credential therefore fails the clone with a non-zero exit. The error is printed to the user verbatim. The `pinned.json` is not touched.

## What's NOT configurable

| Item | Why not |
| --- | --- |
| The templates cache dir | There's no env var for `<CacheDir>/templates`. Override `XDG_CONFIG_HOME` (Linux) or `HOME` (macOS/Windows-via-AppData) to relocate the whole tree. |
| The `pinned.json` path | Always `<CacheDir>/pinned.json`. Atomic writes protect against partial updates. |
| The 15s HTTP timeout on `Client.HTTP` | Hard-coded in `client.go:45`. The default URL is `.invalid`, so this only matters when `SPIN_REGISTRY_URL` points at a real server. A slow server looks the same as a missing one (see [Registry protocol](../concepts/registry-protocol.md)). |
| `GIT_TERMINAL_PROMPT=0` | Set unconditionally in three places. Users who want prompts must work around it. |

## Precedence chain

For the cache dir on Linux:

```
$XDG_CONFIG_HOME  (if set)
$HOME/.config     (else)
sh -c 'echo $HOME' fallback
/tmp/spin-templates (final fallback)
```

For the registry URL:

```
$SPIN_REGISTRY_URL  (if set)
$SPIN_REGISTRY      (else, if set)
https://registry.spin.invalid/v1  (default)
```

For the cache dir on macOS:

```
$HOME/Library/Application Support/spin/  (if $HOME set)
sh -c 'echo $HOME' fallback
/tmp/spin-templates  (final fallback)
```

## Edge cases

- **`XDG_CONFIG_HOME` set to a relative path**: `os.UserConfigDir` returns it as-is. spin then does `os.UserConfigDir() + "/spin"`, which becomes a relative path. The cache ends up at `<cwd>/<XDG_CONFIG_HOME>/spin/`, which is almost never what the user wanted. **Always set `XDG_CONFIG_HOME` to an absolute path.**
- **`SPIN_REGISTRY_URL` with no scheme**: `http://` is not prepended. The client passes the URL to `http.NewRequest`, which errors on a scheme-less URL. The user sees the http error verbatim.
- **`SPIN_REGISTRY_URL` with a path prefix**: works. `<URL>/search` is appended. So `SPIN_REGISTRY_URL=https://api.example.com/spin/v1` is fine.
- **`HOME` and `XDG_CONFIG_HOME` both unset**: `os.UserConfigDir` returns an error. The fallback chain handles it. The final fallback is `/tmp/spin-templates`, which is writable on most setups.
- **`GIT_TERMINAL_PROMPT=0` in the user shell**: ignored by spin's spawned git. This is by design - the scaffolder must not block on a credential prompt.
- **Test isolation on Windows**: `t.Setenv("AppData", t.TempDir())`. The current test suite doesn't have Windows-specific tests, but the pattern is straightforward.

## Related

- [XDG layout and env vars](../concepts/xdg-layout.md) - the on-disk layout these env vars produce.
- [Registry protocol](../concepts/registry-protocol.md) - the URL the env vars override.
- [Testing](../development/testing.md) - the `t.Setenv("XDG_CONFIG_HOME", ...)` pattern.
- [Building](../development/building.md) - the ldflags that inject version info (orthogonal to env vars, but a common confusion).
