# `spin new <name>`

Scaffold a new project from a template. The v2 scaffolding entry point.

Source: `cmd/new.go:35-481`.

## Synopsis

```sh
spin new <name> --template <spec> [--dest <dir>] [--param key=value]... [--print-params | --dry-run]
```

## Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--template, -t` | string | `""` (required) | Template spec: local path, git URL, or pinned name. |
| `--dest, -d` | string | `./<name>` | Destination directory. `~` and `~/` are expanded; everything is made absolute. |
| `--param` | string[] (repeatable) | `nil` | Set a template param as `key=value`. Skips the interactive form. |
| `--print-params` | bool | `false` | Print the resolved params as JSON and exit. No files written. |
| `--dry-run` | bool | `false` | Render to a temp dir, print the file list, clean up. No project written. |

`<name>` is a positional argument. `cobra.ExactArgs(1)`.

## What it does

1. Loads the template via `internal/template/Loader.Load(spec)`. The spec is dispatched: local path first, then git URL, then pinned name.
2. Pre-populates the values map with `name` and `project_name` (both set to `args[0]`), then layers any `--param` values on top.
3. Calls `Template.ResolveForm(values, interactive)`. If interactive (`os.Stdin` is a TTY AND no `--param` AND no preview flag), runs the huh form. Otherwise applies defaults silently.
4. Short-circuits to JSON output (`--print-params`) or the file-list preview (`--dry-run`) when those flags are set.
5. Renders via `Template.RenderToWithPost(dest, resolved)`. The full pipeline: render the `_base/` tree, write the files (path-traversal guarded), run the `[[post]]` steps, delete any `spin.toml` from the dest (TPL-16).
6. Prints a success line with the project name and absolute destination path.
7. If the source was a remote URL AND the user is on a TTY AND the template is not already pinned, offers to pin it for future offline use.

## Examples

```sh
# Local template
spin new myapp --template ~/code/templates/go-cli

# Git URL
spin new myapp --template https://github.com/me/go-cli-template.git

# Pinned name (must have been added first)
spin new myapp --template go-cli-template

# Non-interactive
spin new myapp --template go-cli --param port=8080 --param verbose=true

# Preview
spin new myapp --template go-cli --print-params
spin new myapp --template go-cli --dry-run
```

## Exit codes

- `0` - success
- `1` - any loader / parse / render / post-hook error
- `130` - SIGINT during the interactive huh form (treated as friendly cancellation, not a failure)

## Edge cases

- **Missing `--template`**: errors with a hint pointing to `spin search <query>` and `spin list` (cmd/new.go:88-90).
- **`~` in `--dest`**: expanded via `os.UserHomeDir()`. Absolute and relative paths are made absolute via `filepath.Abs` so the success line always shows a full path (cmd/new.go:342-361).
- **Pre-existing clone at the cache destination** (git URL with a prior cached version): the loader asks the user via `promptExistingDest`. Choices: `Reuse`, `Pin`, `Wipe`, `Cancel`. In non-interactive mode it falls back to `Wipe` (loader.go:52-54, defaultExistingDestPrompt).
- **Pre-existing pin with missing on-disk cache**: errors with "re-run `spin add`" (loader.go:98).
- **Pinned template that fails Detect** (malformed cache): asks the user to keep or remove via `promptInvalidPinned`. In non-interactive mode it always keeps (loader.go:44-46, defaultInvalidPinnedPrompt).
- **Interactive form cancelled via `Esc` or `Ctrl+C`**: caught explicitly as `huh.ErrUserAborted`, prints a friendly "cancelled" line, exits 130 (cmd/new.go:131-137).
- **TPL-16**: every `spin.toml` anywhere in the rendered dest is removed after the post-hook runs, so a template author who accidentally included a `spin.toml` in `_base/` doesn't leak it into the user's project (template.go:148-166).
- **Min-spin-version warning**: if the template declares `min_spin_version = "X.Y.Z"` and X.Y.Z is greater than the running spin's version, prints a non-fatal warning to stderr. Does not block the render (loader.go:129-137).

## Internal calls

- `template.NewLoader("")` (template/loader.go:56) - creates the loader; cache dir defaults to `os.UserConfigDir()/spin/templates`.
- `loader.Load(newTemplate)` (template/loader.go:67) - dispatches the spec.
- `tpl.ResolveForm(values, interactive)` (template/form.go:45) - applies defaults or runs the huh form.
- `tpl.RenderToWithPost(dest, resolved)` (template/template.go:130) - the full render pipeline.

## Related

- [Template schema](../concepts/template-schema.md) - what a `spin.toml` declares.
- [Template engine](../concepts/template-engine.md) - the funcs and the path-traversal guard.
- [Non-interactive use](../concepts/non-interactive.md) - the full `--param` grammar.
- [`spin add`](add.md) - the command that creates the pinned-name spec.
- [`internal/template` package](../packages/template.md) - the loader and renderer.
