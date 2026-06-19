# Non-interactive use

Skip the huh form. Pass params as flags. Preview the result. The `spin new` flags that turn a scaffolder into a scriptable tool.

Source: `cmd/new.go:62-202`.

## The three "don't make me think" flags

| Flag | Effect |
| --- | --- |
| `--param key=value` (repeatable) | Set a template param. Skips the interactive form. |
| `--print-params` | Print the resolved params as JSON and exit. No files written. |
| `--dry-run` | Render to a temp dir, print the file list, clean up. No project written. |

All three (plus non-TTY stdin) cause `interactive` to be false in `runNew` (cmd/new.go:129). The form never opens.

## `--param key=value`

Repeatable. Each entry is `key=value`. The CLI validates each value against the param's declared type (cmd/new.go:375-393, 414-456) and errors out with a clear message on bad input.

### Coercion by type

| Param `type` | Accepted `--param` value(s) | Example |
| --- | --- | --- |
| `text` | the string verbatim | `--param license=MIT` |
| `textarea` | the string verbatim (multi-line) | `--param bio="line 1\nline 2"` |
| `number` | integer; must satisfy min/max | `--param port=8080` |
| `select` | the string; must be in the param's `options` | `--param license=MIT` |
| `multiselect` | comma-separated list | `--param features=ci,release` |
| `bool` | `true`/`1`/`yes`/`y`/`on` / `false`/`0`/`no`/`n`/`off` | `--param verbose=true` |
| `path` | the string (no validation done at CLI layer) | `--param config=./config.toml` |
| `secret` | the string | `--param api_key=...` |

Loose bool parsing is in `parseLooseBool` (cmd/new.go:461-469). Anything not in either list errors out.

### Unknown keys

A `--param foo=bar` where `foo` is not in the template's `[params]` block errors with `--param[N] "<entry>": unknown key "foo" (known: bar, license, port)`. The known list is sorted (cmd/new.go:473-480) so the error is stable.

### Defaults and layering

The values map is layered in `runNew` (cmd/new.go:110-113, 118-124, 129):

1. Built-in: `name` and `project_name` are both set to the positional `<name>`.
2. Template defaults: applied by `params.SetDefaults` inside `Template.ResolveForm`.
3. Caller-supplied: `--param` values layer on top of the defaults.
4. (Interactive only) Huh form: the user's typed answers layer on top of the defaults.

So an explicit `--param name=other` overrides the positional `<name>`. A `--param port=9090` overrides the template's `port = 8080` default.

## `--print-params`

Prints the resolved params map as JSON and exits. No files written. The output shape (cmd/new.go:170-187):

```json
{
  "template": {
    "name": "go-cli",
    "version": "0.1.0",
    "description": "Minimal Go CLI with cobra + fang",
    "type": "cli",
    "language": "go"
  },
  "values": {
    "name": "myapp",
    "project_name": "myapp",
    "license": "MIT",
    "port": 8080
  }
}
```

Template metadata is grouped under `template` so user params named "description" or "version" don't collide with template-level fields.

Useful for piping into other tools:

```sh
spin new myapp --template go-cli --print-params | jq '.values.port'
# 8080
```

## `--dry-run`

Renders to a temp dir, prints the file list, cleans up. No project is left on disk (cmd/new.go:192-202).

```sh
spin new myapp --template go-cli --dry-run
# dry run: would write 5 files to /home/user/myapp
#   /home/user/myapp/main.go
#   /home/user/myapp/go.mod
#   /home/user/myapp/README.md
#   /home/user/myapp/.gitignore
#   /home/user/myapp/Makefile
```

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success (including `--print-params` and `--dry-run` exits). |
| 1 | Loader / parse / render / post-hook error. |
| 130 | SIGINT during the interactive form. **Never reached for `--param` users** - the form doesn't open. |

## Edge cases

- **Built-in `name` / `project_name` collision**: a user-supplied `--param name=other` overrides the positional `<name>`. This is the documented behaviour but can surprise users who assume the positional is "the name of the project". If you need the positional to win, don't pass `--param name`.
- **`multiselect` with no options**: `coerceParamValue` returns `[]string{}` (the empty slice). The template gets `.features = []`, which renders as no entries.
- **Trailing comma in multiselect**: `--param features=ci,release,` is split, trimmed, empties dropped, so the trailing comma is silent. No phantom "empty" option.
- **`--param` with no value**: `--param name=` is rejected with "empty key" or "missing '='" depending on the exact form. `key=value` with an empty value is a valid (but possibly surprising) text param; the CLI does not reject it because text params often have meaningful empty defaults.
- **Type-coercion errors are per-`--param`**: a bad number in one entry doesn't prevent the other entries from being processed; the whole `--param` set errors as a unit, but the error message names the offending entry.
- **JSON output is pretty-printed**: `json.NewEncoder.SetIndent("", "  ")`. Piping into `jq` works because the JSON is valid.
- **`--dry-run` and `--print-params` are mutually exclusive in spirit**: both short-circuit before the write, but the order in `runNew` is "print-params first, then dry-run" (cmd/new.go:141-146). If both are set, `--print-params` wins.

## Related

- [Template schema](template-schema.md) - what `[params.<name>]` blocks look like.
- [Template engine](template-engine.md) - how the values reach the templates.
- [`spin new`](../commands/new.md) - the command and its full flag tour.
