# Template engine

How `_base/` files become a project tree. The funcs, the `.tmpl` extension stripping, the path-traversal guard, and the TPL-16 cleanup.

Source: `internal/template/engine.go`, `template.go`, `form.go`, `post_hook.go`.

## The pipeline

```
Template.Render(values)
  -> walk _base/
     -> for each file:
        -> strip .tmpl ext from rel path
        -> if matches any exclude glob: skip
        -> if not .tmpl: read bytes verbatim
        -> if .tmpl: run through text/template with funcs
  -> return map[string][]byte (rel-path -> content)
Template.RenderToWithPost(dest, values)
  -> Render
  -> writeFiles(dest, files)        // path-traversal guarded
  -> RunPostHook(t, values, dest)
  -> deleteSpinToml(dest)           // TPL-16
```

## The 9 text/template funcs

Available in `_base/*.tmpl` files (engine.go:34-75). The post-hook (`renderHook`, post_hook.go:80-90) uses a **fresh** FuncMap with **no funcs** - so `{{upper .name}}` works in templated files but **not** in `[[post]].run`.

| Func | Args | Returns | Notes |
| --- | --- | --- | --- |
| `upper` | `v string` | `string` | `strings.ToUpper` via `golang.org/x/text/cases`. |
| `lower` | `v string` | `string` | `strings.ToLower`. |
| `title` | `v string` | `string` | `cases.Title(language.English)`. |
| `trim` | `v string` | `string` | `strings.TrimSpace`. |
| `join` | `sep string, vs []string` | `string` | `strings.Join`. |
| `default` | `v any, fallback string` | `string` | Returns `fallback` if `v` is nil or `""`. |
| `snake_case` | `v string` | `string` | `"MyProject"` → `"my_project"`. Algorithm at engine.go:79-99. |
| `kebab` | `v string` | `string` | `snake_case` with `_` → `-`. |
| `quote` | `v string` | `string` | Single-quote shell-escape: `'foo'` → `'foo'`, embedded `'` becomes `'"'"'`. |
| `now` | optional `layout string` | `string` | No-arg: `time.RFC3339`. With layout: that layout (passed to `time.Now().Format`). |
| `contains` | `s, substr string` | `bool` | `strings.Contains`. |

## `.tmpl` extension stripping

`stripTmplExt(p string) string` (template.go:168-173): if the rel path ends in `.tmpl`, trim the last 5 chars. The stripped rel path is the destination path.

So:

```
_base/file.txt.tmpl  -> ./file.txt
_base/cmd/main.go.tmpl -> ./cmd/main.go
_base/README.md       -> ./README.md   (no extension to strip)
```

The `exclude` glob is matched against the **stripped** rel path, not the raw path. So `exclude = ["*.md"]` matches `README.md` but **not** `file.md.tmpl` (which strips to `file.md`). To exclude a templated file, write `exclude = ["file.md"]` - the user writes the destination name, not the source name.

## Non-templated files

Any file under `_base/` that doesn't end in `.tmpl` is copied verbatim (template.go:75-83). `text/template` is not run on it. So binary assets, large data files, and anything that happens to contain `{{` is safe.

## The path-traversal guard

`writeFiles(dest, files)` (engine.go:116-135) is the safety net. For every file in `files`:

1. Compute `cleanDest = filepath.Clean(dest) + Sep`.
2. Compute `full = filepath.Join(dest, rel)`.
3. Compute `cleanFull = filepath.Clean(full)`.
4. Check `strings.HasPrefix(cleanFull+Sep, cleanDest)`.

A `..` segment or an absolute path inside `files` fails the prefix check and the function returns an error. **No files are written** on failure - the function is fail-closed. This handles the case where a template author (maliciously or by mistake) writes a rel path that would escape the destination.

The walk reads from `BaseDir`, so a `..` segment there is impossible to construct. The guard is on the destination side.

The exported alias `WriteFiles` (engine.go:110-112) is for callers that merge template + ecosystem files before writing. Currently unused by any other package in the tree.

## TPL-16: the spin.toml deletion

`deleteSpinToml(dest)` (template.go:148-166) walks the dest and `os.Remove`s every file named `spin.toml`. It runs **after** the post-hook, so a `[[post]]` step can observe the full scaffolded state (including any `spin.toml` that snuck in), but the user never sees it.

The deletion is defensive: a template author who accidentally includes a `spin.toml` in `_base/` (instead of relying on the manifest never being rendered in the first place) would otherwise leak it into the user's project. The walk handles this case without requiring every template author to remember.

## The post-hook

`RunPostHook(t, values, dir)` (post_hook.go:27-52). For each `step.Run`:

1. Render via `renderHook` (post_hook.go:80-90) - `text/template` with **no FuncMap** against `unwrapValues(values)`.
2. `exec.Command("sh", "-c", rendered)` with `c.Dir = dir` and `c.CombinedOutput()`.
3. Stop on first failure. Subsequent steps don't run.
4. The error includes the rendered command and the combined output.

Empty `Post` is a no-op. `sh` is hard-coded; templates that need bash-specific syntax will not work on systems where `sh` is `dash` (Debian) instead of `bash`.

## How text/template sees the values

The values map is the same one `params.ResolveForm` returns - already unwrapped. So `{{.name}}` resolves to a string, not a `Value` struct, and `{{.port}}` resolves to an int (not a `params.Value` with `Int = 8080`).

The template runs against the **root** of the values map. So if `values["myapp"] = "my-app"`, the template sees `.myapp`. Built-in fields populated by `cmd/new.go:110-113` are `name` and `project_name` (both set to the project name).

## Edge cases

- **Funcmap asymmetry between `_base/*.tmpl` and `[[post]].run`**: the post-hook has no funcs. If you need `upper` in a post step, do the transformation in a template file and emit a variable, or hardcode the transformation in the shell command.
- **Time-sensitive templates**: `{{now}}` returns the time the scaffolder ran, not the time the user committed the project. Don't bake timestamps into source files unless that's the intent.
- **`snake_case` is a best-effort algorithm**: `engine.go:79-99` walks runes and inserts `_` at transitions (uppercase / digit / `_`). It does not handle Unicode well. Use `kebab` for URLs and `snake_case` for variable names, but verify the output on a non-ASCII input before relying on it.
- **`quote` is shell-escape, not SQL/HTML/etc.**: the `'"'"'` trick is for single-quoted POSIX shell strings. Don't use it for anything else.

## Related

- [Template schema](template-schema.md) - what `spin.toml` declares.
- [Non-interactive use](non-interactive.md) - the `--param` grammar.
- [`internal/template` package](../packages/template.md) - the renderer in detail.
