# `internal/template`

The renderer concern: load a template (local path, git URL, or pin name), resolve its params, render its `_base/` tree, run its `[[post]]` steps, and delete any `spin.toml` from the output.

Source: `internal/template/template.go`, `loader.go`, `form.go`, `engine.go`, `post_hook.go`, `spin_toml.go`, `parse.go`.

## The `Template` type

`template.go:15-22`:

```go
type Template struct {
    Name        string    // directory basename
    Source      string    // local path on disk (post-clone)
    Repo        string    // git URL, if any
    SpinToml    *SpinToml // parsed manifest
    BaseDir     string    // _base/ inside Source
    PostHookDir string    // _post/ inside Source (currently unused)
}
```

`Source` is always set. `Repo` is non-empty only for git-sourced templates - the `cmd/` code uses `tpl.Repo != ""` as the gate for the post-success pin prompt.

## The `Loader` type

`loader.go:20-39`. Holds the cache dir and two optional interactive prompts:

```go
type Loader struct {
    CacheDir            string
    PromptInvalidPinned func(name, localPath string, detectErr error) (bool, error)
    PromptExistingDest  func(name, localPath string) (destAction, error)
}
```

`NewLoader(cacheDir)` (loader.go:56-61) defaults `CacheDir` to `defaultCacheDir()` (loader.go:341-352), which is `os.UserConfigDir()/spin/templates` with fallbacks to `sh -c 'echo $HOME'/.config/spin/templates` and finally `/tmp/spin-templates`.

`Load(spec)` (loader.go:67-84) tries, in order:

1. **Local path** (`isLocalPath`: starts with `/`, `.`, or `~`) - `Detect(dir)` directly.
2. **Git URL** (`isGitURL`: starts with `http://`, `https://`, `git@`, `git://`, `ssh://`) - `cloneGit(url)` into the cache, then `Detect`.
3. **Pinned name** (anything else) - `loadPinned(spec)` reads `pinned.json` and looks up the name.

Falls through to "not a local path, git URL, or pinned name" with a hint to run `spin add` first.

### `cloneGit` (loader.go:139-185)

If the dest already exists, asks the user via `PromptExistingDest` (Reuse / Pin / Wipe / Cancel). Then `git clone --depth=1 <url> <dest>` with `GIT_TERMINAL_PROMPT=0` in the env (no terminal prompts on missing creds). Then `detectOrPromptInvalid` to check it's actually a valid template.

### `loadPinned` (loader.go:89-123)

Reads `pinned.json` via `registry.Client.ListPinned()`. Matches by `Name`. If the on-disk cache is missing, errors with "re-run `spin add`". If `Detect` fails on a present-but-malformed cache, calls `PromptInvalidPinned` (defaults to always-keep in non-TTY).

### `warnMinSpinVersion` (loader.go:129-137)

Non-fatal stderr line if the template's `min_spin_version` is greater than the running spin's `version.Version`. Uses `compareSemver` (loader.go:319-339) which treats missing semver components as 0 and non-numeric segments as 0.

## Rendering

### `Detect(dir)` (template.go:26-46)

Rejects directories without `spin.toml` or `_base/`. Sets `PostHookDir = <dir>/_post` regardless of whether it exists (the post hook reads from `SpinToml.Post`, not from disk).

### `Render(values)` (template.go:60-92)

Walks `_base/`, strips the `.tmpl` extension from the rel path, copies non-templated files verbatim, runs templated files through `renderFile` (engine.go:18-32). Excluded paths are filtered via `isExcluded` (template.go:97-107) which uses `filepath.Match` per glob. Returns `map[string][]byte` of rel-path → rendered content.

### `RenderTo(dest, values)` (template.go:111-117)

Calls `Render` then `writeFiles`.

### `RenderToWithPost(dest, values)` (template.go:130-146)

The full v2.0 template pipeline:

1. `Render(values)` → file map.
2. `writeFiles(dest, files)` - path-traversal guarded.
3. `RunPostHook(t, values, dest)` - `[[post]]` steps.
4. `deleteSpinToml(dest)` - **TPL-16**: walk dest and `os.Remove` every `spin.toml`.

`deleteSpinToml` (template.go:148-166) is defensive - a template author who accidentally includes a `spin.toml` in `_base/` would otherwise leak it into the user's project.

### `writeFiles` (engine.go:116-135)

The path-traversal guard. Computes `cleanDest + Sep` once, then for every file checks `strings.HasPrefix(cleanFull+Sep, cleanDest)`. A file with a `..` segment or absolute path inside `files` errors out and writes nothing. The exported alias `WriteFiles` (engine.go:110-112) is for callers that merge template + ecosystem files.

## The text/template funcs

Available in `_base/*.tmpl` files (engine.go:34-75):

- `upper`, `lower`, `title` (golang.org/x/text/cases)
- `trim`, `join`
- `default` (two-arg, returns the first if v is nil/"")
- `snake_case` (`"MyProject"` → `"my_project"`, see engine.go:79-99 for the algorithm)
- `kebab` (snake_case with `_` → `-`)
- `quote` (single-quote shell-escape, engine.go:101-104)
- `now` (no-arg: RFC3339; with layout: that layout)
- `contains`

The post-hook (`renderHook`, post_hook.go:80-90) uses a **fresh** `text/template` with **no funcs** - the post-hook is a thin shell wrapper, not a full template engine.

## `SpinToml` (the manifest)

`spin_toml.go:37-51`:

```go
type SpinToml struct {
    Name           string
    Version        string
    Description    string
    Type           string // "tui" | "cli" | "lib" | ...
    Language       string // "go" | "rust" | "ts" | ...
    Author         Author
    License        string
    Repository     string
    MinSpinVersion string
    Exclude        []string
    Params         map[string]params.Spec
    Post           []PostStep
    Tags           []string
}
```

`Author` (spin_toml.go:55-59): `{name, email, url}`, all optional. `PostStep` (spin_toml.go:70-72): `{run string}`. Both are siblings of `SpinToml` in the same file.

`parseTOML` (parse.go:44-71) uses **BurntSushi/toml** (go.mod:9). The docstring at spin_toml.go:83-99 still describes a hand-rolled parser; the live code uses BurntSushi. The comment is stale.

## `BuildForm` and `ResolveForm` (template/form.go)

`BuildForm(t)` (form.go:14-31) builds the huh form for a template. Currently unused by `cmd/new.go`, which calls `ResolveForm` directly.

`ResolveForm(values, interactive)` (form.go:45-78) is the live path:

1. If non-interactive (`!interactive`): call `params.SetDefaults(values)` to apply template defaults to anything the caller didn't supply.
2. If interactive: call `params.Run(values)` to open the huh form.
3. Layer caller-supplied values on top (form.go:59-63). Explicit `--param` wins over the form, which wins over defaults.
4. Return the unwrapped primitive values (form.go:64-76) so `text/template` sees `{{.name}}` as a string, not a `Value` struct.

`UnwrapValue` (form.go:98-114) is exported because `post_hook.go` also needs it.

`Hints()` (form.go:118-124) returns a one-line-per-param summary. Not currently wired to any print call.

## `RunPostHook` (post_hook.go:27-52)

For each `step.Run`: render against the resolved values via `renderHook` (no funcs, plain `text/template`), then `sh -c <rendered>` in `dir` with `c.CombinedOutput()`. Stops on first failure. Empty `Post` is a no-op.

The post-hook runs **after** files are written and **before** the spin.toml deletion. So the hook can observe the full scaffolded state but the user never sees a leaked `spin.toml`.

## How it fits in the pipeline

```
spin new
  -> Loader.Load(spec)                // returns *Template
  -> tpl.ResolveForm(values, ...)     // applies defaults or runs huh
  -> tpl.RenderToWithPost(dest, ...)  // render + post-hook + TPL-16
```

The registry pipeline (`spin add`) shares the `Template` type at the boundary: `Client.Add` calls `Template.Detect` after a successful clone to validate the result.

## Edge cases

- **Template name vs directory name**: `Name` is always the directory basename. The `spin.toml` `name` field is *metadata only* - it can differ from the directory name. The loader doesn't check for consistency.
- **`.tmpl` extension stripping**: the stripped rel path is the destination path. `exclude = ["*.md"]` matches `README.md` but not `file.md.tmpl` (which would become `file.md` after stripping). To exclude a templated file, write `exclude = ["file.md"]`.
- **Path-traversal guard is on the destination side**: a `..` in a template rel path is fine as long as it doesn't escape `dest`. The walk reads from `BaseDir` so a `..` segment there is impossible anyway.
- **TPL-16 runs after the post-hook**: a template's `[[post]]` can read a `spin.toml` that was in `_base/`, but the user never sees it.

## Tests

- `internal/template/loader_test.go` - cache dir, the path-traversal guard, the prompts.
- `internal/template/parse_test.go` - `SpinToml` parsing.
- `internal/template/template_test.go` - render + write + post-hook.
- `cmd/new_test.go` (in `cmd/`) - end-to-end at the CLI layer.

## Related

- [Template schema](../concepts/template-schema.md) - what `spin.toml` actually declares.
- [Template engine](../concepts/template-engine.md) - the funcs, the stripping, the guard.
- [Architecture](../overview/architecture.md) - where this package sits in the two-pipeline model.
- [`internal/params` package](params.md) - the consumer of `Spec`.
- [`internal/registry` package](registry.md) - the pin-store side that calls `Detect` after a clone.
