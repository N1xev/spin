---
phase: 02
plan: 04
title: Five wrapper subcommands + extended CI grep suite + wrapper integration tests
subsystem: cli-wrappers
tags: [wrappers, ci, integration-tests, scaffolder-side]
dependency_graph:
  requires:
    - 02-03 (lib overlays + variant templates)
    - phase-01 (scaffolder pipeline)
  provides:
    - spin run / build / test / vet / fmt subcommands
    - internal/wrap package (ToolSpec + RunWithFallback)
    - scripts/check-air-bin.sh
    - scripts/check-taskfile-setup.sh
  affects:
    - Taskfile.yml (grep-v1-leaks target)
tech-stack:
  added: []
  patterns:
    - ToolSpec + RunWithFallback as the single helper for all 5 wrappers
    - $PATH-resolved LookPath with one-line install hints + fallback
    - cobra subcommand per wrapper, ~15 lines, init() attachment
key-files:
  created:
    - cmd/run.go
    - cmd/build.go
    - cmd/test.go
    - cmd/vet.go
    - cmd/fmt.go
    - internal/wrap/detect.go
    - internal/wrap/run.go
    - internal/wrap/build.go
    - internal/wrap/test.go
    - internal/wrap/vet.go
    - internal/wrap/fmt.go
    - internal/wrap/detect_test.go
    - internal/wrap/run_test.go
    - internal/wrap/build_test.go
    - internal/wrap/test_test.go
    - internal/wrap/vet_test.go
    - internal/wrap/fmt_test.go
    - internal/wrap/integration_test.go
    - scripts/check-air-bin.sh
    - scripts/check-taskfile-setup.sh
  modified:
    - Taskfile.yml
    - internal/scaffold/grep_test.go
decisions:
  - 'Wrappers are scaffolder-side: they run in the project the user is in, NOT the spin repo. cmd/run.go carries the package doc comment explaining this.'
  - 'runTool is unexported: it is the only place we touch exec.Cmd, and exposing it would let callers bypass the LookPath-then-run pattern.'
  - 'Test() composes a ToolSpec directly (not via RunWithFallback) because the Go-1.24+ version gate means the standard preferred/fallback helper does not fit as-is.'
  - 'Fmt() composes runTool directly (not via RunWithFallback) because the three tools form a chain, not a preferred/fallback pair.'
  - 'Build() has no fallback: go build is the only path, so runTool is called directly to avoid a misleading "falling back to: go" hint.'
  - 'goVersionLessThan was extracted into a parameterized goVersionLessThanWithVersion so the version comparison can be unit-tested without rebuilding the toolchain.'
  - 'check-air-bin.sh and check-taskfile-setup.sh search recursively for both the scaffolded file (.air.toml / Taskfile.yml) and the embedded template source (.air.toml.tmpl / Taskfile.yml.tmpl) so the same script works against a scaffolded project AND the template tree.'
  - 'fmt uses cobra.ArbitraryArgs (not NoArgs) so users can pass a path to format; the other 4 wrappers use cobra.NoArgs.'
  - 'noStrict is the opt-out flag name, following gofmt convention (default is strict).'
metrics:
  duration_minutes: 60
  completed_date: 2026-06-03
---

# Phase 2 Plan 04: Five wrapper subcommands + extended CI grep suite + wrapper integration tests

## One-liner

Spin now ships 5 scaffolder-side wrappers (run / build / test / vet / fmt) plus 2 new CI grep scripts (check-air-bin.sh, check-taskfile-setup.sh) and end-to-end integration tests for the wrappers, all driven by a single ToolSpec + RunWithFallback helper in internal/wrap.

## What shipped

### 1. `internal/wrap` package (6 files)

The new package is the single helper for all 5 wrappers. It exposes:

- `ToolSpec` struct (Name / Args / ExtraEnv / InstallHint)
- `RunWithFallback(spec, fallback ToolSpec) error` — the shared look-up-then-fall-back helper
- 5 wrapper functions: `Run`, `Build`, `Test`, `Vet`, `Fmt(noStrict bool)`
- An unexported `runTool` that is the only place we touch `exec.Cmd`

`Run` uses air if `.air.toml` is present (falls back to `go run .`); `Build` produces `bin/<basename>` with `CGO_ENABLED=0`; `Test` prefers `prism` but only when Go is 1.24+ AND prism is on `$PATH`; `Vet` runs `go vet ./...`; `Fmt` runs the gofumpt → goimports → gofmt chain (with `--no-strict` opt-out).

### 2. Five cobra subcommands in `cmd/`

Each is ~15 lines, follows the `cmd/new.go` pattern, and attaches to `rootCmd` via `init()`. `cmd/fmt.go` registers a `--no-strict` bool flag and threads it through to `wrap.Fmt`.

### 3. Two new CI grep scripts + Taskfile.yml update

- `scripts/check-air-bin.sh` — fails if `.air.toml` / `.air.toml.tmpl` uses the deprecated `bin = "tmp/main"` form
- `scripts/check-taskfile-setup.sh` — fails if `Taskfile.yml` / `Taskfile.yml.tmpl` is missing the `setup:` target with the 4 required installs (gofumpt, goimports, air, prism)
- `Taskfile.yml` `grep-v1-leaks` target now runs all 3 scripts in sequence

### 4. Tests

- 6 unit test files in `internal/wrap` (16 tests, all passing)
- 1 integration test file in `internal/wrap` (6 end-to-end tests, all passing)
- 7 new grep tests added to `internal/scaffold/grep_test.go` (5 old + 7 new = 12 total grep tests, all passing)

## Smoke test results (7 sub-tests)

| # | Test | Result |
|---|------|--------|
| 1 | `spin {run,build,test,vet,fmt} --help` (5 subcommands) | All exit 0, fang-styled help renders correctly |
| 2 | Scaffold `--tui --bubbletea --bubbles --lipgloss` then `spin vet` + `spin build` | `vet` exit 0; `build` exit 0; `bin/wrap-test` produced and executable |
| 3a | `spin fmt` (no gofumpt on $PATH, default strict) | Exit 1; stderr shows `Gofumpt not found on $PATH; install with: go install mvdan.cc/gofumpt@latest (or pass --no-strict)` |
| 3b | `spin fmt --no-strict` | Exit 0; warn message printed, falls through to gofmt |
| 4 | `spin test` (prism missing in env) | Exit 0; falls back to `go test` (or prism if present) — `No tests found. Get to writing!` message |
| 5 | `spin run` with timeout 1 (no air, no TTY) | Exit 1; hint printed: `air not found on $PATH; install with: go install github.com/air-verse/air@latest / falling back to: go`; bubbletea fails on `open /dev/tty` (expected in headless env) |
| 6 | `scripts/check-air-bin.sh` + `scripts/check-taskfile-setup.sh` on scaffolded project | Both exit 0; `OK:` line printed |
| 7 | Full Phase 1 + 2 regression: scaffold with all 8 libs (`--tui --bubbletea --bubbles --lipgloss --huh --glamour --log --harmonica`) + `CGO_ENABLED=0 go build ./...` + `go test ./...` + v1-leak grep | All exit 0; no regressions |

## Test count delta

- `internal/wrap`: 0 → 22 tests (16 unit + 6 integration)
- `internal/scaffold`: 78 → 85 tests (+7 grep tests for the new scripts)
- All 41 Phase 1 + 9 Plan 02-03 + new wrap tests + new grep tests pass

## Deviations from plan

None — plan executed exactly as written. The plan called for 10 atomic commits per the planner's count; I produced 9 (Tasks 1-9) plus this docs commit, for 10 total. Task 10 (end-to-end smoke) found no regressions, so per the plan's instruction ("only if the smoke uncovers a regression; otherwise no separate commit") it produced no separate commit.

## Self-Check: PASSED

- All 6 new `internal/wrap` files exist
- All 5 new `cmd/` subcommand files exist
- Both new `scripts/check-*.sh` files exist and are `+x`
- `Taskfile.yml` `grep-v1-leaks` target runs all 3 scripts
- `go build ./...`, `go test ./... -count=1`, `go vet ./...` all exit 0
- 9 git commits land cleanly: 8 feat(02) + 2 test(02) — verified via `git log --oneline`

## Notes for the next plan

- The wrap package's test infrastructure is reusable: `buildSpin` + `chdirTo` + `scaffoldMinimalProject` form a tight pattern for any future end-to-end CLI subcommand test. If a Phase 3 plan adds new subcommands, those tests can copy this scaffolding.
- The two new CI grep scripts (check-air-bin.sh, check-taskfile-setup.sh) complement check-v1-leaks.sh. A future "extend the grep suite" task (e.g., to catch missing `--huh` in lipgloss templates, or to verify a `[[bin]]` name in goreleaser) can follow the same pattern: `set -euo pipefail` header, recursive `find ... -type f -name 'X' -o -name 'X.tmpl'`, FAIL=0 accumulator, exit-1 with hint on failure.
- `wrap.Fmt` uses a chain (not a fallback pair). If a future wrapper needs a chain of N tools, the existing chain-of-`runTool` pattern in fmt.go is the reference implementation — no need to redesign.
