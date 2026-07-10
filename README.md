# spin

`spin` is a scaffolder for external templates. A template is just a directory — a git repo, a local path, or a pinned name — that has a `spin.toml` manifest and a `_base/` tree of files. `spin new <name> --template <spec>` resolves the template, prompts you for the params (or accepts them as flags), renders the tree into a new project, and runs the template's `[[pre]]` and `[[post]]` steps.

Templates are the only extension surface. The language, framework, build tool, and test runner are entirely up to the template author. `spin` does not care. It can scaffold Go, Rust, Node, Python, static sites, or anything else that fits in a directory with a `spin.toml` and a `_base/` tree.

```sh
spin add https://github.com/me/go-cli-template.git   # pin a template for offline use
spin new myapp --template go-cli-template            # scaffold from the pin
spin list                                            # show pinned templates
spin update go-cli-template                          # refresh the cache
spin search tui                                      # search registered local registries
```

## Install

If you have a Go toolchain:

```sh
go install github.com/N1xev/spin@latest
```

You get a single static binary. No runtime deps, no CGO. `git` must be on `$PATH` for cloning git URLs and registries.

## Quick start

```sh
# Scaffold from a local template directory
spin new myapp --template ~/code/templates/go-cli

# Scaffold from a git URL
spin new myapp --template https://github.com/me/go-cli-template.git

# Pin a template for offline use, then scaffold from the pin
spin add https://github.com/me/go-cli-template.git
spin new myapp --template go-cli-template

# Non-interactive, perfect for CI
spin new myapp --template go-cli --param port=8080 --param verbose=true

# Preview the resolved params without writing files
spin new myapp --template go-cli --print-params

# Dry run, list, and refresh
spin new myapp --template go-cli --dry-run
spin list
spin update go-cli
```

## Commands

| Command | What it does |
| --- | --- |
| `spin new [<name>] [<template>]` | Scaffold a project from a template |
| `spin add <spec>` | Pin a template locally for offline use |
| `spin list` | Show pinned templates (table; `--json` for scripts) |
| `spin update [name]` | Refresh a pinned template's on-disk cache |
| `spin remove <name>` | Drop a pin (`--purge` to also delete the cache) |
| `spin search <query>` | Search registered local registries |
| `spin registry ...` | Add / list / update / remove local registries |
| `spin init <name>` | Scaffold a new template directory |
| `spin version` | Print the spin version |

## Templates

A template is two things in a directory:

```
my-template/
  spin.toml         # manifest: name, params, hooks
  _base/            # file tree rendered into the user's project
    file.txt.tmpl   # .tmpl files are run through text/template
    file.txt        # everything else is copied verbatim
```

`spin init my-template` writes a working starter you can edit. Full documentation lives at <https://spin.dev>.

## Security: hooks run shell commands

A template's `[[pre]]` and `[[post]]` steps — and any executable file dropped in `_pre/` or `_post/` — are executed with `sh -c` on your machine, in the new project directory, with your user's permissions. `spin new user/repo` therefore runs whatever shell the template author wrote.

Treat templates like any other code you run: only scaffold from sources you trust. When trying out an unknown template, inspect its `spin.toml` (the `[[pre]]`/`[[post]]` `run` lines) and the `_pre/` and `_post/` directories first, or run with hooks disabled:

```sh
# Skip all pre/post hooks
spin new myapp --template <spec> --no-hooks

# See what the hooks would run without executing them (also skips hooks)
spin new myapp --template <spec> --dry-run
```

## Non-interactive

`spin new` opens an interactive Huh form by default. To skip the prompts (CI, scripts, IDEs), pass `--param key=value` for every param the template declares. The CLI validates each value against the param's type and errors out on bad input.

```sh
spin new myapp --template go-cli \
  --param port=8080 \
  --param verbose=true \
  --param features=ci,release \
  --param name=myapp
```

## Registries

A registry is a git repo or local directory with a `registry.toml` manifest and a `templates/` directory of per-template metadata files. Register one locally:

```sh
spin registry add official https://github.com/example/spin-registry.git
spin search tui
spin add official/tui-template
```

`spin search` reads the local registry cache; it does not query a remote server.

## Requirements

- Go 1.25 or newer to build or install spin.
- `git` on `$PATH`. Used for `spin add <git-url>`, `spin update`, and `spin registry add`. Not required for local-path templates or registries.
- `CGO_ENABLED=0` for any binary you intend to ship.

## Documentation

Full docs are at <https://spin.dev>.

## Development

```sh
git clone <repo> && cd spin
go test ./...                    # full unit and integration suite
task build                       # ./bin/spin
task grep-v1-leaks               # charm v1-purity check across the source
task dogfood                     # end-to-end: init -> new -> build -> test
```

## Release

```sh
git tag v0.2.0
git push origin v0.2.0
```

The release workflow runs goreleaser. Multi-arch binaries land in the GitHub release.

## CI

- `ci.yml`: build, test, vet, v1-leak grep.
- `dogfood.yml`: end-to-end smoke (init to new to build).
- `release.yml`: tag-driven goreleaser release.

## License

MIT. See [LICENSE](LICENSE).
