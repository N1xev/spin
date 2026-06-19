# spin

`spin` is a scaffolder for external templates. A template is just a directory somewhere - a git repo, a local path - that has a `spin.toml` manifest and a `_base/` tree of files. `spin new <name> --template <spec>` resolves the template, prompts you for the params (or accepts them as flags), renders the tree into a new project, and runs the template's `[[post]]` steps.

Templates are the only extension surface. The language, the framework, the build tool, the test runner: all of that is up to the template author. `spin` does not care. It can scaffold Go projects, Rust projects, Node projects, Python projects, anything that has a `spin.toml` and a tree of files. The bundled scaffolder UI uses huh forms, lipgloss styling, and fang-rendered help so it looks nice in the terminal, but that is a tooling choice for `spin` itself, not a constraint on what templates can produce.

```
spin add https://github.com/me/go-cli-template.git   # pin a template for offline use
spin new myapp --template go-cli-template            # scaffold from the pin
spin list                                            # show pinned templates
spin update go-cli-template                          # refresh the cache
spin search tui                                      # search the public registry
```

## Install

The install script picks the right archive for your OS and arch, drops it in `~/.local/bin`, and is a no-op if you are already up to date.

```sh
curl -sSfL https://raw.githubusercontent.com/N1xev/spin/main/scripts/install.sh | sh
```

If you have a Go toolchain, `go install` works too:

```sh
go install github.com/N1xev/spin/cmd/spin@latest
```

Either way you get a single static binary. No runtime deps, no CGO. If you want a Homebrew formula, the goreleaser config opens a PR on the homebrew-tap repo on every release, so `brew install N1xev/tap/spin` works once that tap exists.

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
| `spin new <name> --template <spec>` | Scaffold a project from a template |
| `spin add <spec>` | Pin a template locally for offline use |
| `spin list` | Show pinned templates (table; `--json` for scripts) |
| `spin update [name]` | Refresh a pinned template's on-disk cache |
| `spin remove <name>` | Drop a pin (`--purge` to also delete the cache) |
| `spin search <query>` | Query the public template registry |
| `spin init <name>` | Scaffold a new template directory |
| `spin version` | Print the spin version |

## Templates

A template is two things in a directory:

```
my-template/
  spin.toml         # manifest: name, params, post steps
  _base/            # file tree rendered into the user's project
    file.txt.tmpl   # .tmpl files are run through text/template
    file.txt        # everything else is copied verbatim
```

`spin init my-template` writes a working starter you can edit. The full `spin.toml` grammar, all eight param types, the `[[post]]` runner, and the `exclude` glob are in `docs/concepts/template-schema.md`. To author a template, fork any directory that already builds and add `spin.toml` plus a `_base/` tree; there is nothing else to do.

The earlier v2.x design had compiled-in "ecosystems" for charmbracelet and rust with a `spin new <ecosystem> <name>` form and a universal task runner. That whole concept is gone. Templates are the only extension surface, and they target whatever language you want.

## Non-interactive

`spin new` opens an interactive huh form by default. To skip the prompts (CI, scripts, IDEs), pass `--param key=value` for every param the template declares. The CLI validates each value against the param's type and errors out on bad input.

```sh
spin new myapp --template go-cli \
  --param port=8080 \
  --param verbose=true \
  --param features=ci,release \
  --param name=myapp
```

## The three-repo split

`spin` is one of three repos. The CLI never writes to the registry, and authors never talk to the server directly. They talk to the catalog, and the catalog deploys the server.

| Repo | Role | Public surface |
| --- | --- | --- |
| github.com/N1xev/spin | The CLI | `spin new`, `spin add`, `spin list`, `spin update`, `spin search` |
| github.com/N1xev/spin-index | The catalog | `templates/index.toml` (PR-based) |
| github.com/N1xev/spin-registry | The server | `GET /v1/search`, `GET /v1/templates/:name`, `GET /v1/healthz`, `GET /v1/metrics` |

The server is read-only JSON over HTTP, no auth, no cookies. Per-IP rate limit is enforced server-side at 20 req/s with a burst of 20, so a CI loop cannot take it down. Real healthz (the server pings its database and returns 503 if the DB is down) means a Fly healthcheck can route around a broken machine. Prometheus metrics at `/v1/metrics` cover request count, latency, and search duration.

Full design notes are in `docs/concepts/registry-design.md`. The wire format the CLI talks is in `docs/concepts/registry-protocol.md`.

## Status

All three repos are production-ready. The release pipeline runs goreleaser v2 on every `v*` tag and ships multi-arch binaries plus a Homebrew formula. The registry has real healthz, Prometheus metrics, per-IP rate limiting, a request-body size cap, and a graceful shutdown timeout. The catalog has PR-side validation that walks every entry, clones it, runs `spin new` non-interactively, and posts a markdown report on the PR.

## Requirements

- Go 1.25 or newer to install spin itself.
- `git` on `$PATH`. Used for `spin add <git-url>`. Not required for local-path or pinned-name templates.
- `CGO_ENABLED=0` for any binary you intend to ship.

## Documentation

The reference tree lives in `docs/`. It is organised by topic, not by source file. Start with `docs/overview/quickstart.md` if you are new, or jump straight to a command under `docs/commands/` or a concept under `docs/concepts/`.

Key entry points:
- `docs/concepts/registry-design.md`: the three-repo split, the deploy pipeline, the data model.
- `docs/concepts/template-schema.md`: the `spin.toml` grammar, all eight param types, `[[post]]`, `exclude`.
- `docs/concepts/pinning.md`: how `spin add` resolves a name to a source URL, and what the cache layout looks like.
- `docs/concepts/non-interactive.md`: the `--param` and `--print-params` semantics, including all eight type coercions.

## Development

```sh
git clone <repo> && cd spin
go test ./...                    # full unit and integration suite
task build                       # ./bin/spin
task grep-v1-leaks               # charm v2-purity check across the source
bash scripts/dogfood.sh          # end-to-end: init -> new -> build -> test
```

To cut a release:

```sh
git tag v0.2.0
git push origin v0.2.0
```

The release workflow runs goreleaser. Multi-arch binaries land in the GitHub release and a Homebrew formula PR opens on the tap repo automatically.

## CI

- `ci.yml`: build, test, vet, v1-leak grep.
- `dogfood.yml`: end-to-end smoke (init to new to build), plus a live-registry smoke that asserts the `.invalid` fallback and a custom `SPIN_REGISTRY_URL` both behave correctly.
- `release.yml`: tag-driven goreleaser v2 release.

## License

MIT. See [LICENSE](LICENSE).
