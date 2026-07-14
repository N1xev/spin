# Contributing to spin

## Quick start

```sh
task build     # compiles to ./bin/spin
task test      # runs go test ./... -count=1
```

Or without Task:

```sh
go build -o bin/spin .
go test ./... -count=1
```

## Before submitting

Run these locally before pushing. CI runs the same checks.

```sh
go fmt ./...
go vet ./...
go mod tidy
go test ./... -count=1
```

`golangci-lint` is available via `task lint` but is not required for
contributions — `go vet` catches the critical issues.

## Code conventions

- **Single binary, no plugins.** No daemon, no embedded templates.
- **Language-agnostic.** Templates target any stack. spin only owns the
  load / prompt / render / hook pipeline.
- **Templates are external.** No defaults shipped with the binary.
- **Read files before editing them.** Match existing indentation (tabs),
  error formats, and comment style.
- **Context propagation.** All I/O functions accept `context.Context`.
- **Add tests** for new functionality. Use `go test -count=1` locally.
- **No comments** unless explaining *why*, not *what*. The code should
  be self-documenting.

## Project structure

| Directory | Purpose |
|---|---|
| `cmd/` | Cobra commands and CLI wiring |
| `internal/template/` | Template loading, spin.toml parsing, rendering, hooks |
| `internal/params/` | Param types and Huh form generation |
| `internal/registry/` | Pinned templates (pinned.json) and local registries (registries.json) |
| `internal/version/` | Build version injected at compile time |
| `internal/spec/` | Template spec detection (local path, git URL, shorthand) |
| `internal/log/` | Project-level charmbracelet/log logger |

## Releasing

Tag a version and push. goreleaser builds binaries for linux/amd64,
linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, publishes a
GitHub release, and opens a PR on the homebrew tap.

```sh
git tag v0.2.0
git push origin v0.2.0
```

## License

Apache 2.0. See [LICENSE](./LICENSE).
