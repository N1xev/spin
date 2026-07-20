# spin

Scaffolder that doesn't care what language or framework you use. Point it
at a template directory with a `spin.toml` and a `_base/` tree, it asks
the template's questions, renders the files, runs the hooks. Done.

```sh
spin new myapp https://github.com/spin-templates/charm-tui.git
```

![spin logo](https://spincli.pages.dev/SpinLogo.png)

## Install

```sh
go install github.com/N1xev/spin@latest

# or

curl -sSfL https://spincli.pages.dev/install.sh | sh
```

Needs `git` on `$PATH`. Single static binary, nothing else.

## Use

```sh
spin new myapp ./templates/go-cli                   # local path
spin new myapp https://github.com/me/repo.git       # git URL
spin new myapp go-cli-template                      # pinned name
spin new myapp official/go-cli                      # registry shorthand

spin add https://github.com/me/repo.git             # pin for offline
spin list                                           # show pins
spin update go-cli-template                         # refresh cache
spin remove go-cli-template                         # unpin
spin search tui                                     # search registries
spin registry add official https://...              # register a registry
spin init my-tpl                                    # create a template
```

Non-interactive:

```sh
spin new myapp go-cli --param port=8080 --param name=myapp
spin new myapp go-cli --print-params                # show params, no write
spin new myapp go-cli --dry-run                     # show files, no write
spin new myapp go-cli --no-hooks                    # skip hooks
```

## Template

```
my-template/
  spin.toml                                         # params, hooks, include rules
  _base/                                            # file tree → project
```

`spin init my-template` writes a starter. Full docs: https://spincli.pages.dev

## License

[Apache 2.0](./LICENSE)

