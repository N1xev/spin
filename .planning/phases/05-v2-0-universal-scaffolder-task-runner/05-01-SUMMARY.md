---
phase: 05-v2-0-universal-scaffolder-task-runner
plan: 01
subsystem: ecosystem
tags: [rust, cargo, ecosystem, universal-scaffolder, charmbracelet, v2]

# Dependency graph
requires:
  - phase: 05-v2-0-skeleton
    provides: "internal/ecosystem/ interfaces, internal/ecosystems/charm/ pattern, internal/scaffold/ engine, cmd/ecosystem.go + cmd/new_charm.go skeleton"
provides:
  - "internal/ecosystems/rust/ package: complete Ecosystem implementation (Flags, Detector, Validate, Render, PostScaffold, Tasks)"
  - "cmd/ecosystem.go defaultRegistry() seeds both charm and rust"
  - "cmd/new_rust.go: `spin new rust <name> [flags]` subcommand with --bin/--lib/--example aliases"
  - "Path-traversal-safe writeFiles helper (mirrors template.writeFiles)"
affects: [phase-05-plan-02, phase-05-plan-04, phase-05-plan-05, future ecosystem packages]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Ecosystem is a self-contained Go package under internal/ecosystems/<name>/ implementing the ecosystem.Ecosystem interface"
    - "writeFiles helper with filepath.Clean + HasPrefix path-traversal guard (same pattern as internal/scaffold/emit)"
    - "ChoiceFlag aliases become separate cobra Bool flags that translate to the canonical --type value in runNewRust"
    - "PostScaffold anchors relative dir at ./<name> and skips git ops gracefully when git is not on $PATH"

key-files:
  created:
    - internal/ecosystems/rust/ecosystem.go
    - internal/ecosystems/rust/flags.go
    - internal/ecosystems/rust/detector.go
    - internal/ecosystems/rust/validate.go
    - internal/ecosystems/rust/render.go
    - internal/ecosystems/rust/post.go
    - internal/ecosystems/rust/tasks.go
    - internal/ecosystems/rust/post_test.go
    - cmd/new_rust.go
  modified:
    - cmd/ecosystem.go
    - internal/ecosystems/rust/ecosystem.go (refactor: stub methods replaced by real ones in their own files)

key-decisions:
  - "Rust ecosystem is COMPLETELY self-contained -- no calls into internal/scaffold (the second citizen, not the first)"
  - "writeFiles is unexported but package-internal, so the path-traversal test in post_test.go can exercise it directly"
  - "Task 1 stub methods on *Ecosystem (Render/PostScaffold/Tasks) were removed once Task 2 added real implementations in their own files (Go forbids duplicate methods across files of the same package)"
  - "--bin/--lib/--example are ChoiceFlag aliases that translate to --type=bin|lib|example in runNewRust (cleaner than registering them as extra ChoiceFlag variants)"
  - "git init / commit skipped with a stderr warning when git is not on $PATH (graceful degradation)"
  - "Per cargo convention: libraries ignore Cargo.lock, binaries do not"

patterns-established:
  - "Pattern: each ecosystem lives in its own internal/ecosystems/<name>/ directory with ecosystem.go + flags.go + detector.go + validate.go + render.go + post.go + tasks.go (+ optional _test.go)"
  - "Pattern: <name>_test.go in same package exposes the unexported writeFiles helper for security-critical unit tests"
  - "Pattern: when a ChoiceFlag has aliases, bind each alias as a Bool cobra flag in init() and translate to the canonical flag value before constructing ecosystem.Context"

requirements-completed: [ECO-04, ECO-05, ECO-06, ECO-07, ECO-11, RUN-13]

# Metrics
duration: 8min
completed: 2026-06-08
---

# Phase 5 Plan 1: Rust Ecosystem Summary

**Cargo-based rust ecosystem (binary, library, example) with self-contained Render/PostScaffold/Tasks, path-traversal-safe write helper, and `spin new rust <name> --bin|--lib|--example` cobra subcommand wired into the universal registry.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-06-08T20:50:20Z
- **Completed:** 2026-06-08T20:58:05Z
- **Tasks:** 3
- **Files modified:** 11 (9 created, 2 modified)

## Accomplishments

- Complete `internal/ecosystems/rust/` package -- 8 Go files, compiles clean, satisfies `ecosystem.Ecosystem` interface
- 11 rust-specific flags (type, edition, rust-version, author, description, ai, gitignore, force, no-git, quiet, no-interactive) with chainable `WithHelp`/`WithPrompt`/`WithAliases` builders
- Self-contained `Render` produces Cargo.toml, src/main.rs (or src/lib.rs or examples/<name>.rs), .gitignore (with cargo-convention-aware Cargo.lock handling for libs), and spin.config.toml with the cargo fallback `[tasks]` block
- `PostScaffold` writes files via a path-traversal-safe `writeFiles` helper, then runs `git init` + `git add -A` + `git commit` (skipped with a one-line warning when git is not on $PATH or `--no-git` is set)
- `Tasks()` returns the 5 cargo fallbacks (build/test/run/clippy/fmt) for the runner's source-precedence chain (RUN-13)
- `cmd/ecosystem.go` `defaultRegistry()` now seeds both `charm` and `rust`; `./spin ecosystem list` shows both
- New `cmd/new_rust.go` cobra subcommand exposes `spin new rust <name> [flags]` with `--bin`/`--lib`/`--example` aliases that translate to `--type=bin|lib|example`
- `TestRustPost_NoTraversal` unit test guards the writeFiles helper against STRIDE T-05-01 path-traversal

## Task Commits

Each task was committed atomically:

1. **Task 1: Create rust package skeleton (flags, struct, detector, validate)** - `84ada31` (feat)
2. **Task 2: Implement Render, PostScaffold, and Tasks (with path-traversal test)** - `157ab0d` (feat)
3. **Task 3: Register rust in defaultRegistry and add `spin new rust` cobra subcommand** - `30a4428` (feat)

## Files Created/Modified

- `internal/ecosystems/rust/ecosystem.go` - `Ecosystem` struct, `New()`, `Name/Description/Version/Flags/Tasks` methods
- `internal/ecosystems/rust/flags.go` - 11 flags including ChoiceFlag `type` with `[bin, lib, example]` aliases
- `internal/ecosystems/rust/detector.go` - `Matches(dir)` checks for `Cargo.toml`; `FriendlyName()` returns `"Rust (Cargo)"`
- `internal/ecosystems/rust/validate.go` - Enforces `type` is bin/lib/example, `edition` is 2015/2018/2021/2024, `rust-version` non-empty
- `internal/ecosystems/rust/render.go` - `Render(ctx)` returns file map; `writeFiles(dest, files)` path-traversal-safe disk write
- `internal/ecosystems/rust/post.go` - `PostScaffold(ctx, dir)` writes files and runs git init/commit (skipped with warning if git missing or `--no-git`)
- `internal/ecosystems/rust/tasks.go` - `Tasks()` returns the 5 cargo fallback commands
- `internal/ecosystems/rust/post_test.go` - `TestRustPost_NoTraversal` rejects `../escape.txt` key in the file map
- `cmd/ecosystem.go` - `defaultRegistry()` now seeds both `charm` and `rust`
- `cmd/new_rust.go` - new `newRustCmd` subcommand with flag binding, alias translation, and dispatch to the rust ecosystem

## Decisions Made

- **Aliases as separate bool flags**: ChoiceFlag aliases (`--bin`, `--lib`, `--example`) are registered as separate cobra Bool flags and translated to the canonical `--type=bin|lib|example` value in `runNewRust` before constructing the ecosystem context. This is cleaner than coercing aliases into additional ChoiceFlag variants and keeps the renderer logic single-source-of-truth.
- **Stub methods then replace**: For Task 1, `*Ecosystem` had stub `Render/PostScaffold/Tasks` returning `nil` so the type satisfied `ecosystem.Ecosystem` and the build stayed green. Task 2 removed those stubs and defined real implementations in `render.go`/`post.go`/`tasks.go`. (Go forbids duplicate method definitions across files of the same package, so the stubs had to be removed rather than overwritten.)
- **Self-contained rust ecosystem**: The rust ecosystem never calls into `internal/scaffold/`. It has its own `writeFiles` helper that mirrors the path-traversal guard pattern from `template.writeFiles`/`scaffold.emit`. The charm ecosystem is the one that wraps scaffold; rust writes its own files because cargo projects have a different file layout, naming, and gitignore conventions.
- **Cargo.lock in .gitignore only for libraries**: Per cargo convention, libraries ignore `Cargo.lock` (it should match the published version); binaries do not. The `renderGitignore(projectType)` helper encodes this rule.
- **Graceful git-skip**: `PostScaffold` uses `exec.LookPath("git")` to detect missing git and logs a one-line warning to stderr before continuing (no fail-fast). This matches the STRIDE T-05-04 mitigation.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] ChoiceFlag aliases for `type`**
- **Found during:** Task 3 (verification)
- **Issue:** The plan's verification command was `./spin new rust scratchapp --bin --no-git --no-interactive` but the `flags.go` ChoiceFlag declaration didn't include `[bin, lib, example]` as aliases -- only `--type=bin|lib|example` was exposed. Running the verification command as written produced `Unknown flag: --bin`.
- **Fix:** Added `WithAliases([]string{"bin", "lib", "example"})` on the ChoiceFlag; `cmd/new_rust.go` `init()` now binds each alias as a Bool cobra flag and `runNewRust` translates them to the canonical `--type` value before constructing the ecosystem context.
- **Files modified:** `internal/ecosystems/rust/flags.go`, `cmd/new_rust.go`
- **Verification:** `./spin new rust scratchapp --bin` (and `--lib`, `--example`) all produce the expected file maps; `spin ecosystem info rust` lists all 11 flags.
- **Committed in:** `30a4428` (part of Task 3 commit)

**2. [Rule 1 - Blocking] Task 1 stub methods clashed with Task 2 real methods**
- **Found during:** Task 1 (build)
- **Issue:** Adding stub `Render/PostScaffold/Tasks` methods on `*Ecosystem` in `ecosystem.go` (so the type satisfied `ecosystem.Ecosystem`) blocked Task 2 from defining them in `render.go`/`post.go`/`tasks.go` (Go forbids duplicate methods across files of the same package).
- **Fix:** Task 2 removed the stub methods from `ecosystem.go` and defined real ones in their proper files.
- **Files modified:** `internal/ecosystems/rust/ecosystem.go`
- **Verification:** `go build ./...` and `go vet ./...` both pass after Task 2.
- **Committed in:** `157ab0d` (part of Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 missing critical, 1 blocking)
**Impact on plan:** Both auto-fixes essential for matching the plan's verification commands and for Task 1 / Task 2 to compile in sequence. No scope creep.

## Issues Encountered

- None significant. The cwd-reset between Bash tool calls confused one test invocation (`cd /tmp` did not persist); resolved by running verification inline in single commands.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The rust ecosystem is registered and produces cargo projects end-to-end (`spin new rust <name> --bin|--lib|--example`).
- The 5 cargo fallback tasks are exposed via `Tasks()` and ready to be consumed by the runner's source-precedence chain (Plan 04's job, per the task spec).
- The deprecation shim in `cmd/new.go` for the legacy `spin new <name>` route is **not** done in this plan (correctly per the plan's Task 3 instructions) -- that is Plan 02's responsibility.
- `cmd/new_charm.go` was deliberately not modified in this plan.

---

*Phase: 05-v2-0-universal-scaffolder-task-runner*
*Completed: 2026-06-08*
