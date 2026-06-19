# Quickstart

The 5 commands you actually need on day one. Everything else is reference.

## The happy path

```sh
# 1. Scaffold from a local template directory
spin new myapp --template ~/code/templates/go-cli

# 2. Scaffold from a git URL (one-shot; no caching)
spin new myapp --template https://github.com/me/go-cli-template.git

# 3. Pin a template for offline use
spin add https://github.com/me/go-cli-template.git
spin new myapp --template go-cli-template   # works offline now

# 4. List what you've pinned
spin list               # table
spin list --json        # for scripts

# 5. Refresh a pin when the upstream moves
spin update go-cli-template
```

That's the whole loop. Add a few flags for the non-interactive cases.

## Non-interactive (CI, scripts)

If a template declares params (e.g. `license`, `port`, `verbose`), `spin new` opens an interactive form by default. To skip the form, pass every param the template needs as a repeatable `--param key=value`:

```sh
spin new myapp --template go-cli \
  --param port=8080 \
  --param verbose=true \
  --param features=ci,release \
  --param name=myapp
```

Each value is coerced to the param's declared type. Unknown keys error out with the known list. See [Non-interactive use](../concepts/non-interactive.md) for the full grammar.

## Preview without writing files

```sh
# Print the resolved params as JSON (useful for piping into other tools)
spin new myapp --template go-cli --print-params

# Print the file list that WOULD be written, then exit
spin new myapp --template go-cli --dry-run
```

## Scaffold a new template

`spin` ships a starter template; `spin init` is the on-ramp for authors:

```sh
spin init my-template
# writes ./my-template/{spin.toml, _base/file.txt.tmpl, README.md}

# Then use it immediately
spin new myapp --template ./my-template --param license=MIT
```

Edit `spin.toml` (params, `[[post]]`, metadata) and the `_base/` tree to taste. See [Template schema](../concepts/template-schema.md) for the full grammar.

## Where things go on disk

| Path | What |
| --- | --- |
| `~/.config/spin/pinned.json` | The pin index, JSON. Atomic writes. |
| `~/.config/spin/templates/<name>/` | Per-pin cache (symlink for local, git clone for remote). |
| `./<name>/` (or `--dest`) | The rendered project, owned by the user. |

`XDG_CONFIG_HOME` redirects the whole `~/.config/spin/` tree on Linux. See [XDG layout and env vars](../concepts/xdg-layout.md).

## Install

```sh
go install github.com/N1xev/spin@latest
```

The binary lands at `$(go env GOPATH)/bin/spin`. Single static binary, no runtime deps, `CGO_ENABLED=0`.

## What's next

Read the [What is spin?](what-is-spin.md) overview for the architecture, then jump to the [commands](../commands/new.md) or [concepts](../concepts/template-schema.md) you need.

## Related

- [What is spin?](what-is-spin.md) - the one-paragraph pitch.
- [Architecture](architecture.md) - the two-pipeline model behind the 5 commands.
- [`spin new`](../commands/new.md) - the scaffolder, with all flags.
- [`spin add`](../commands/add.md) - the pin command, for offline templates.
