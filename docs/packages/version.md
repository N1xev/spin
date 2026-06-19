# `internal/version`

A single `var Version = "0.1.0"`. The build-time version string. Override via `-ldflags`.

Source: `internal/version/version.go` (5 lines).

## What it is

```go
package version

// Version is the current spin release. Override via -ldflags at build time.
var Version = "0.1.0"
```

That's the whole package. One exported var, one comment.

## How it's read

- `cmd/root.go:19` passes it to fang so the `--version` flag works.
- `cmd/version.go:15` prints it via `fmt.Println(version.Version)`.
- `internal/template/loader.go:129-137` compares the running spin's version against a template's `min_spin_version` to emit a non-fatal warning.

## How to override at build time

```sh
go build -ldflags '-X github.com/N1xev/spin/internal/version.Version=0.2.0' -o ./bin/spin .
```

The `-X` flag injects a string into a package-level var. Quote the entire `-ldflags` value so the shell doesn't expand `$Version`.

For `task` users, the `build` task at `Taskfile.yml:18-21` is the canonical form. To release a version:

```sh
task build VERSION=0.2.0
# equivalent: go build -ldflags '-X github.com/N1xev/spin/internal/version.Version=0.2.0' -o ./bin/spin .
```

(The `task build` target currently doesn't read `VERSION` - it just runs `go build`. Add a `VERSION` env var to the task if you want release builds to be a one-liner.)

## Why a separate package

Two reasons:

1. **Single import path for the var.** Anywhere in the codebase that needs the version imports `internal/version`. No duplicated constants.
2. **The `-X` injection point is a known Go pattern.** Putting the var in its own tiny package makes the build line obvious and the override trivially auditable.

## Edge cases

- **Forgetting the quotes** around the `-ldflags` value: the shell expands `$Version` in the env, you get `-X github.com/.../version.Version=` (empty), and the binary prints a blank version. Always quote.
- **The var is a string, not semver**: there's no comparison done at runtime by `version` itself. `Loader.warnMinSpinVersion` does its own loose semver compare via `compareSemver` (loader.go:319-339) which treats missing semver components as 0 and non-numeric segments as 0. The two pieces of code don't share an implementation.
- **No `git describe` integration**: spin doesn't try to derive the version from git tags at build time. The `Version` var is whatever the build line says it is.

## Related

- [`spin version`](../commands/version.md) - the user-facing command.
- [Building](../development/building.md) - the `go build` + `-ldflags` recipe.
