# `spin version`

Print the spin version.

Source: `cmd/version.go:11-22`.

## Synopsis

```sh
spin version
```

## Flags

None. `cobra.NoArgs`. The command uses `Run` (not `RunE`) so any error path is a fang error rather than a `RunE` returned error.

## What it does

Prints `version.Version` from `internal/version/version.go:5`. The default is `"0.1.0"`. Override at build time:

```sh
go build -ldflags '-X github.com/N1xev/spin/internal/version.Version=0.2.0' -o ./bin/spin .
```

fang also wires `--version` on the root command, so `spin --version` produces the same output as `spin version`. The dedicated subcommand is the documented form for scripts and the README.

## Examples

```sh
spin version
# 0.1.0

spin --version
# 0.1.0
```

## Exit codes

- `0` - always

## Edge cases

- **Build-time override**: the `Version` var is exported; the only thing keeping it static is the Go linker not seeing an `-ldflags` override. If you forget the quotes around the `-X` arg, the shell expands `$Version` and you get an empty version string.
- **The version is a string, not semver**: there's no comparison done at runtime. `Loader.warnMinSpinVersion` (loader.go:129-137) does its own loose semver compare against the *template's* `min_spin_version` - that's the only place a version comparison happens in spin.

## Internal calls

- `version.Version` (internal/version/version.go:5) - the var.

## Related

- [Building](../development/building.md) - the `-ldflags` version-injection recipe.
- [`internal/version` package](../packages/version.md) - the var definition.
