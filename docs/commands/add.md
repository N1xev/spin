# `spin add <spec>`

Pin a template locally for offline use. The cache lives at `~/.config/spin/templates/<name>/`; the pin record lives at `~/.config/spin/pinned.json`.

Source: `cmd/add.go:12-65`.

## Synopsis

```sh
spin add <spec>          # pin a template
spin add                 # alias for `spin list` (no args)
spin add --list          # alias for `spin list` (explicit)
```

## Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--list` | bool | `false` | Show pinned templates and exit. Equivalent to `spin add` with no args, which is equivalent to `spin list`. |

`<spec>` is a positional argument. `cobra.MinimumNArgs(0)` - the command is a no-op without args unless `--list` is set, in which case it delegates to `execList` (cmd/list.go).

## Spec kinds

`<spec>` is dispatched by `internal/registry/Client.Add` (registry/client.go:144-159):

1. **`user/repo` shorthand** - exactly one slash, no scheme, no leading `.`/`~`. Expanded transparently to `https://github.com/<user>/<repo>.git` (client.go:165-179).
2. **Git URL** - starts with `http://`, `https://`, `git@`, `git://`, or `ssh://`. Shallow-cloned to the cache with `git clone --depth=1` and `GIT_TERMINAL_PROMPT=0` (client.go:222-259).
3. **Local path** - starts with `/`, `.`, or `~`. The path is symlinked into the cache; on Windows or filesystems that don't support symlinks, falls back to a recursive copy (client.go:181-220).

Anything else errors with "expected a local path, a git URL, or a user/repo shorthand".

## What it does

1. Trims whitespace from the spec.
2. Calls `Client.Add(spec)`. The clone/copy is performed before the `Pinned` record is returned.
3. Sets `Pinned.PinnedAt = time.Now().UTC().Format(time.RFC3339)`.
4. Calls `Client.Pin(*pinned)` to persist the record atomically to `pinned.json` (write-temp + fsync + rename, client.go:504-538).
5. Prints a success line: "cloned to" for git sources, "local at" for local sources.

The HEAD SHA is captured best-effort after a successful clone (`git -C <dest> rev-parse HEAD`, client.go:322-330). If it works, `Pinned.Version` is the 40-char SHA; if it fails, it falls back to the literal string `"git"`.

## Examples

```sh
# Pin a GitHub repo via shorthand
spin add charmbracelet/bubbletea

# Pin a git URL (full)
spin add https://github.com/me/go-cli-template.git

# Pin a local template directory
spin add ~/code/templates/go-cli

# Show what you've pinned
spin add --list
spin add         # no-args form
```

## Exit codes

- `0` - success (including the no-args/`--list` case)
- `1` - registry error, clone failure, or any other I/O error

## Edge cases

- **No args, no `--list`**: delegates to `execList` (cmd/add.go:41-43). The user gets the table or the empty-state hint.
- **Shorthand detection rejects paths**: `spin add foo/bar` is shorthand, but `spin add ./foo/bar` is a local path. The check is on the first character of the spec (client.go:165-167).
- **Two URLs that resolve to the same basename** (e.g. `https://github.com/foo/bar.git` and `git@github.com:foo/bar.git`) collide on the pin name `bar`. The second `add` wipes the first's cache.
- **Local-path symlink vs copy**: `addLocal` tries `os.Symlink` first (cheap, no copy). If that fails (Windows without SeCreateSymbolicLinkPrivilege, or a non-symlink filesystem), it falls back to `copyDir` (client.go:208-212).
- **Cache dir creation**: `templates/` is created with `os.MkdirAll` and `0o755` perms. Any previous clone/symlink with the same name is removed first.
- **No network fallback for local paths**: a local-path pin never touches the network. A git pin needs network at pin time; subsequent `spin new --template <name>` runs are offline against the cached clone.
- **Empty `pinned.json`**: treated the same as a missing file. `ListPinned` returns `(nil, nil)` (client.go:387-403) and `spin list` prints "no pinned templates".

## Internal calls

- `registry.New()` (registry/client.go:31) - reads `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` for the search URL, sets `CacheDir` to `os.UserConfigDir()/spin`.
- `client.Add(spec)` (registry/client.go:144) - the spec dispatcher.
- `client.Pin(pinned)` (registry/client.go:408) - atomic write of the pin record.

## Related

- [`spin list`](list.md) - the read side of the same pin store.
- [`spin remove`](remove.md) - the unpin command; `--purge` also removes the cache.
- [`spin update`](update.md) - re-clones the cache, captures a new HEAD SHA, rolls back on failure.
- [`spin new`](new.md) - the consumer; uses the pin as a `--template <name>` spec.
- [Pinning model](../concepts/pinning.md) - the full `pinned.json` schema and the shorthand expansion.
- [`internal/registry` package](../packages/registry.md) - the `Client` type and the `Pinned` struct.
