# Template schema (`spin.toml`)

The manifest at the root of every external template. Parsed by `internal/template/parse.go` via `BurntSushi/toml`.

Source: `internal/template/spin_toml.go:37-105`, `parse.go:11-133`.

## File location

`<template-dir>/spin.toml`. Required - `Detect` errors out if missing (`template/template.go:27-30`).

## Top-level scalars

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | **yes** | The template's display name. `ParseSpinTomlBytes` errors out if empty (spin_toml.go:101-103). |
| `version` | string | no | Free-form semver-ish string. Recorded in `pinned.json` as the template version. |
| `description` | string | no | One-line description. Shown in `spin list` and `spin search`. |
| `type` | string | no | Free-form: `"tui"`, `"cli"`, `"lib"`, anything. |
| `language` | string | no | Free-form: `"go"`, `"rust"`, `"ts"`, anything. |
| `license` | string | no | The template's own license (not the scaffolded project's). |
| `repository` | string | no | URL. Pure metadata. |
| `min_spin_version` | string | no | Semver-ish. Compared against the running spin's version via `compareSemver` (loader.go:319-339). Triggers a non-fatal stderr warning if newer. |
| `exclude` | []string | no | `filepath.Match` globs. Matched against the rel path under `_base/` with the `.tmpl` extension already stripped. |
| `tags` | []string | no | For `spin search` filtering. |

## `[author]` table

```toml
[author]
name  = "Your Name"
email = "you@example.com"
url   = "https://you.example.com"
```

All three fields are optional. Templates only need to fill what they want to publish.

## `[params.<name>]` blocks

Each top-level key under `[params]` is a param spec. Two forms are supported.

### Shorthand (string default only)

```toml
[params]
project_name = "my-project"
```

Decodes to `{Type: "text", Default: "my-project"}`. Sufficient for simple text params.

### Inline table (full control)

```toml
[params.port]
type    = "number"
prompt  = "Port to listen on"
default = 8080
min     = 1
max     = 65535
```

Full spec: `type`, `prompt`, `default`, `min`, `max`, `options`. The `asInt64` helper (parse.go:119-129) accepts int / int64 / float64 (TOML numbers come back as float64 from BurntSushi).

### The 8 param types

| Type | Huh widget | Coerced from `--param` as | Notes |
| --- | --- | --- | --- |
| `text` | `huh.NewInput` | string (verbatim) | Default for missing `type`. |
| `textarea` | `huh.NewText` (unlimited) | string | `CharLimit(0)`. |
| `number` | `huh.NewInput` + Validate | int | Re-parses on submit, applies Min/Max. |
| `select` | `huh.NewSelect[string]` | string (must be in `options`) | `--param` validates against the options list. |
| `multiselect` | `huh.NewMultiSelect[string]` | comma-split `[]string` | Defaults pre-selected via `opt.Selected(true)`. |
| `bool` | `huh.NewConfirm` | `true`/`1`/`yes`/`y`/`on` or `false`/`0`/`no`/`n`/`off` | Loose parsing. |
| `path` | `huh.NewFilePicker` | string | File mode by default. `NewDir` is the directory-mode constructor but not currently wired in `ParseOne`. |
| `secret` | `huh.NewInput` with `EchoMode(Password)` | string | Default intentionally not pre-filled into the placeholder. |

For `multiselect`, the `default` can be a TOML array:

```toml
[params.features]
type    = "multiselect"
prompt  = "Features to enable"
options = ["ci", "release", "docs"]
default = ["ci"]
```

BurntSushi gives back a `[]any`; `asStringSlice` (parse.go:77-91) coerces to `[]string`.

## `[[post]]` array of tables

```toml
[[post]]
run = "go mod init {{.module_path}}"

[[post]]
run = "go mod tidy"
```

Each entry is `{run = "<shell command template>"}`. The `run` string is rendered as `text/template` against the resolved values via `renderHook` (post_hook.go:80-90). The post-hook uses a **fresh** FuncMap with **no funcs** - the post-hook is a thin shell wrapper, not the full template engine. So `{{upper .name}}` works in `_base/*.tmpl` files but **not** in `[[post]].run`.

The rendered string is then run via `sh -c <rendered>` in the project dir with `c.CombinedOutput()`. Steps run in declaration order; the hook stops on the first failure.

## TPL-16: the spin.toml deletion

After the post-hook completes, `deleteSpinToml(dest)` (template.go:148-166) walks the dest and `os.Remove`s every file named `spin.toml`. This is defensive: a template author who accidentally includes a `spin.toml` in `_base/` would otherwise leak it into the user's project. The deletion runs **after** the post-hook, so a `[[post]]` step can observe the full scaffolded state (including any `spin.toml`), but the user never sees it.

## Full example

```toml
name             = "go-cli"
description      = "Minimal Go CLI with cobra + fang"
version          = "0.1.0"
type             = "cli"
language         = "go"
license          = "MIT"
repository       = "https://github.com/me/go-cli-template"
min_spin_version = "0.1.0"
exclude          = ["docs/*", "*.bak"]
tags             = ["go", "cli", "starter"]

[author]
name  = "Your Name"
email = "you@example.com"
url   = "https://you.example.com"

[params]

[params.license]
type    = "select"
prompt  = "License"
options = ["MIT", "Apache-2.0", "BSD-3-Clause", "Proprietary"]
default = "MIT"

[params.module_path]
type    = "text"
prompt  = "Go module path"
default = "example.com/myapp"

[params.features]
type    = "multiselect"
prompt  = "Optional features"
options = ["ci", "release", "docs"]
default = ["ci"]

[[post]]
run = "go mod tidy"

[[post]]
run = "git init"
```

## Edge cases

- **The docstring at `spin_toml.go:83-99` is stale**: it describes a hand-rolled parser. The live code uses BurntSushi. The discrepancy is harmless but worth knowing if you're reading the file expecting a hand-rolled implementation.
- **`name = ""` errors out** at `ParseSpinTomlBytes`. This is the only required field.
- **Empty `type` defaults to `text`** in `ParseOne` (parse.go:27-28). The shorthand form `name = "default"` produces a text param.
- **`.tmpl` extension stripping before `exclude` matching**: `exclude = ["*.md"]` matches `README.md` but not `file.md.tmpl` (which strips to `file.md`). To exclude a templated file, write `exclude = ["file.md"]`.
- **`filepath.Match` semantics, not doublestar**: `**/*.md` is not supported. To match nested files, list each depth: `docs/*` matches `docs/foo.md` but not `docs/sub/bar.md`. Use multiple patterns.
- **Params are not deduplicated by name**: two `[params.license]` blocks is a TOML parse error from BurntSushi, not a silent override.

## Related

- [Template engine](template-engine.md) - the funcs available in `_base/*.tmpl`.
- [Non-interactive use](non-interactive.md) - the `--param` grammar and the per-type coercions.
- [`internal/template` package](../packages/template.md) - the `SpinToml` type and the parser.
- [`spin init`](../commands/init.md) - the starter manifest.
