# `spin init <name>`

Scaffold a new external template directory in the current working directory. The result is a ready-to-hack template: `spin.toml` + a `_base/` tree with one example file + a README.

Source: `cmd/init.go:27-206`.

## Synopsis

```sh
spin init <name>                # create ./<name>/
spin init <name> --dir <parent> # create <parent>/<name>/
```

## Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--dir` | string | cwd | Parent directory to create the template in. |

`<name>` is a positional argument. `cobra.ExactArgs(1)`.

## What it produces

`runInit` creates `<parent>/<name>/` (or `<parent>` if `--dir` is given, the parent of the cwd if not) with three files:

- **`spin.toml`** - the manifest. Rendered by `initSpinToml` (cmd/init.go:161-178). Includes `name`, `description`, `version = "0.1.0"`, `type = "cli"`, `language = "go"`, `min_spin_version = "0.1.0"`, a `[params.license]` select param with four options, and a no-op `[[post]]` step.
- **`_base/file.txt.tmpl`** - a placeholder file showing `{{.name}}` interpolation (cmd/init.go:52-61).
- **`README.md`** - the readme body at cmd/init.go:65-91, linking to the spin GitHub repo and listing the editable parts.

## What it does

1. Validates the name. Rejects empty, `.`, `..`, or anything containing `/`, `\`, or NUL (cmd/init.go:198-206). Path separators are an outright reject because they'd let the user create templates outside the intended parent.
2. Resolves `--dir` (or uses cwd).
3. Refuses to overwrite an existing directory. The user has to `rm -rf` first (cmd/init.go:119-123). A typo should not clobber a real template.
4. Creates `<dest>/_base/` and writes the three files. Parent dirs of nested entries are created with `os.MkdirAll` even though the current starter only has one nested dir (`_base/`) - this keeps the loop safe if the manifest is extended.
5. Calls `tryAutoPin` (cmd/init.go:188-191), which is currently a no-op that prints a hint. Templates in development are usually just a directory, and re-pinning on every `init` is annoying; the hint points the user at `spin add` for the offline case.
6. Prints a success line: `created template "<name>" at <abs-path>` and a hint: `edit spin.toml and _base/, then \`spin new <project> --template <name>\``.

## Examples

```sh
# Scaffold in the current directory
spin init my-cli-template
# -> created template "my-cli-template" at /home/user/code/my-cli-template
# -> edit spin.toml and _base/, then `spin new <project> --template my-cli-template`

# Scaffold in a different parent
spin init my-template --dir ./templates
# -> created template "my-template" at /home/user/code/templates/my-template

# Use it immediately
spin new myapp --template ./my-cli-template --param license=MIT
```

## Exit codes

- `0` - success
- `1` - any I/O error, validation failure, or "destination already exists" refusal

## Edge cases

- **Destination already exists**: hard error with a message naming the path. The user has to pick a different name or remove the existing dir (cmd/init.go:119-123).
- **Empty name**: rejected by `cobra.ExactArgs(1)` (an empty `<name>` is not matched as an arg, so the command errors out at arg validation).
- **`..` or `/` in name**: rejected by `validateTemplateName` (cmd/init.go:198-206). The error message names the offending characters.
- **`tryAutoPin` is a stub**: as of v2.0-template, it just prints a hint. It does NOT actually call `spin add`. The user is expected to run `spin add <path>` themselves if they want offline-name access. The comment at cmd/init.go:188-191 explains why.
- **The starter is intentionally minimal**: a fixed manifest, a single placeholder file, a README. The value of `spin init` is "give me a working skeleton in 1 second", not "guess what I want". The user is expected to edit everything.
- **The starter's `[[post]]` step is `echo 'post hook ran for {{.name}}'`** - a no-op that exists so the post-hook pipeline is exercised end-to-end. Replace it with real work (e.g. `go mod tidy`, `cargo init`) when editing.

## Internal calls

- `os.MkdirAll(<dest>/_base, 0o755)` (cmd/init.go:125-127) - creates the tree.
- `os.WriteFile(path, body, 0o644)` per file (cmd/init.go:142-144).
- `tryAutoPin(name, dest, ...)` (cmd/init.go:188) - the no-op stub.
- `validateTemplateName(name)` (cmd/init.go:198) - the validator.

## Related

- [Template schema](../concepts/template-schema.md) - what to put in the `spin.toml` after `init`.
- [Template engine](../concepts/template-engine.md) - the funcs available in `_base/*.tmpl`.
- [`spin new`](new.md) - the consumer of a freshly-init'd template.
- [`spin add`](add.md) - the offline-name wiring (`spin add <path-to-init-output>`).
