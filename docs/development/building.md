# Building spin

From `go build .` to a versioned release binary. The flags, the env vars, the ldflags recipe.

Source: `Taskfile.yml:14-52`, `main.go:1-28`, `internal/version/version.go`.

## Taskfile targets

`Taskfile.yml` is the dev-task entry point. Install [task](https://taskfile.dev) once and run `task <name>` from the repo root.

| Target | Command | What it does |
| --- | --- | --- |
| `default` | `task build` | Alias of `build`. |
| `build` | `go build -o ./bin/spin .` | Compile the binary into `./bin/spin`. |
| `test` | `go test ./... -count=1` | Full test suite, no test cache. |
| `grep-v1-leaks` | `bash scripts/check-v1-leaks.sh ./internal && bash scripts/check-v1-leaks.sh ./cmd` | Deny-list grep for charm v1 API/path regressions. |
| `dogfood` | `bash scripts/dogfood.sh` | End-to-end smoke (init, render, list, test). |
| `lint` | `golangci-lint run ./...` (if installed) | Lint; skips with a one-line message if `golangci-lint` is missing. |
| `clean` | `rm -rf ./bin ./tmp ./dist ./.tmp` | Remove build + work dirs. |

The `env:` block pins `CGO_ENABLED=0` for every task. This keeps the binary statically linkable and cross-compile-friendly.

## Plain `go build`

If you don't have `task`, the equivalent is:

```sh
CGO_ENABLED=0 go build -o ./bin/spin .
```

`main.go:1-28` calls `fang.Execute(ctx, rootCmd)` with the cobra root. The binary is a single static executable; no runtime dependencies besides libc (and even libc is unnecessary when `CGO_ENABLED=0` on Linux/macOS).

## Version injection via ldflags

The `version` subcommand reads `internal/version/version.go`:

```go
package version

var (
    Version = "dev"
    Commit  = "none"
    Date    = "unknown"
)
```

Three vars, all `var` (not `const`) so the linker can override them. To bake real values into the binary at build time:

```sh
VERSION="$(git describe --tags --always --dirty)"
COMMIT="$(git rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

CGO_ENABLED=0 go build \
  -ldflags "-X github.com/<org>/spin/internal/version.Version=$VERSION \
            -X github.com/<org>/spin/internal/version.Commit=$COMMIT \
            -X github.com/<org>/spin/internal/version.Date=$DATE" \
  -o ./bin/spin .
```

`./bin/spin version` then prints the real values instead of `dev` / `none` / `unknown`.

The exact import path (`github.com/<org>/spin/internal/version`) depends on the module path declared in `go.mod`. The package itself is tiny - one file, three vars - so the path is easy to find.

## Cross-compilation

The `CGO_ENABLED=0` env is the only piece that matters. With it set, `GOOS=linux GOARCH=arm64 go build .` produces a static binary that runs on a Raspberry Pi. Without it, cross-compile can fail with cgo toolchain errors.

| Target OS | `GOOS` | `GOARCH` | Notes |
| --- | --- | --- | --- |
| Linux amd64 | `linux` | `amd64` | Default CI runner. |
| Linux arm64 | `linux` | `arm64` | Raspberry Pi, Graviton. |
| macOS arm64 | `darwin` | `arm64` | Apple Silicon. |
| Windows amd64 | `windows` | `amd64` | No `os.Symlink` fallback to copy mode applies; the symlink/copy split is automatic. |

## Clean

`task clean` removes `./bin`, `./tmp`, `./dist`, and `./.tmp`. The latter is the dogfood work dir (see [Scripts](scripts.md)). It is `.gitignore`d.

## Edge cases

- **`CGO_ENABLED` env override**: Taskfile's `env:` block sets it for the spawned `go` process. Setting `CGO_ENABLED=1` in your shell before invoking `task` does **not** win - the task env comes from the file.
- **Build with no ldflags**: `spin version` prints `dev` / `none` / `unknown`. This is intentional. CI runs and release builds set the vars; local dev runs do not.
- **`go build ./...` (with three dots)**: builds every package but produces **no binary**. Use it for "did the tree compile?" sanity checks. For an actual binary, use `go build -o ./bin/spin .` (single dot, main package).
- **Build on Windows with symlink fallbacks**: `internal/registry/client.go:208` (`os.Symlink`) is best-effort. Without Developer Mode / `SeCreateSymbolicLinkPrivilege`, the build still succeeds; the runtime code falls back to `copyDir`. The build does not require symlink support.

## Related

- [Testing](testing.md) - the `go test ./... -count=1` half of `task test`.
- [Scripts](scripts.md) - the `check-v1-leaks.sh` and `dogfood.sh` that `task` wraps.
- [CI](ci.md) - the GitHub Actions workflows that mirror `task test` and `task dogfood`.
- [`spin version`](../commands/version.md) - the user-facing command that reads the injected vars.
- [`internal/version` package](../packages/version.md) - the var declarations.
