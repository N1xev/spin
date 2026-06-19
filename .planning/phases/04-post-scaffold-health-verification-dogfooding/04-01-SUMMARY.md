---
phase: 04-post-scaffold-health-verification-dogfooding
plan: 01
subsystem: testing
tags: [doctor, health-checks, lipgloss, cobra, golang.org/x/mod]
status: complete

# Dependency graph
requires:
  - phase: 03-interactive-prompts-ai-agents-md
    provides: cobra + fang + charm v2 wiring that cmd/doctor.go extends
provides:
  - spin doctor subcommand with --format human|json, --strict, --deep, --fix
  - internal/doctor package: Check interface, Registry, 4 universal checks + DeepLintCheck
  - Stable JSON schema for CI integration: {"checks":[{"name","status","message","hint"}]}
  - Two-tier exit code (0=ok, 1=fail; Strict promotes warn to fail)
affects: [04-02-spin-lint, 04-06-ci-dogfood]

# Tech tracking
tech-stack:
  added:
    - charm.land/lipgloss/v2 (promoted from indirect to direct for human renderer)
  patterns:
    - "Check interface + Registry + RunOptions orchestrator pattern for extensible health checks"
    - "Fixer interface (optional) for safe repairs under --fix"
    - "Stable JSON schema with on-the-wire view type (jsonCheck) separate from public CheckResult"

key-files:
  created:
    - internal/doctor/doctor.go
    - internal/doctor/checks.go
    - internal/doctor/render.go
    - internal/doctor/doctor_test.go
    - internal/doctor/checks_test.go
    - internal/doctor/render_test.go
    - cmd/doctor.go
    - cmd/doctor_test.go
  modified:
    - go.mod (charm.land/lipgloss/v2 promoted from indirect to direct)

key-decisions:
  - "ToolPresenceCheck is warn (not fail) when a tool is missing: --fix can install it; per CONTEXT D-05 the install is repairable"
  - "Fix errors are surfaced in CheckResult.Message (with 'fix failed: <err>' prefix) rather than returned as a Go error, so the CLI keeps rendering the report and the user can see what was tried"
  - "JSON schema is locked in render.go via a separate jsonCheck view type so the public CheckResult stays free of json tags (the human renderer doesn't need them) and the schema is easy to bump in one place"
  - "The 5th check (DeepLintCheck) is registered only when RunOptions.Deep is true, keeping the base doctor fast (sub-5s) per CONTEXT D-04"

requirements-completed: [HLTH-01]

# Metrics
duration: 25min
completed: 2026-06-05
---

# Phase 4 Plan 1: spin doctor Summary

**Universal Go project health checker with 4 base checks (go version, tool presence, go.mod hygiene, CGO=0 build), lipgloss-styled human output, stable JSON schema, and --strict/--deep/--fix flags**

## Performance

- **Duration:** 25 min
- **Started:** 2026-06-05T00:08:29Z
- **Completed:** 2026-06-05T00:33:09Z
- **Tasks:** 3
- **Files modified:** 9 (8 created + 1 go.mod promotion)

## Accomplishments

- `spin doctor` audits any Go project: Go toolchain version (semver compared via golang.org/x/mod/semver), tool presence for air/prism/gofumpt/goimports, go.mod hygiene (golang.org/x/mod/modfile parse with direct/indirect duplicate detection), and a `CGO_ENABLED=0 go build ./...` smoke test with a 60s timeout
- `--format json` emits a stable `{"checks":[{"name","status","message","hint"}]}` schema; CI scripts can parse it without a version negotiation
- `--strict` promotes warnings to exit 1 (matches go vet / golangci-lint convention)
- `--deep` registers the lint check (`golangci-lint run ./...`) only when set
- `--fix` invokes the optional `Fixer` interface on `ToolPresenceCheck` (`go install <pkg>@latest`) and `GoModHygieneCheck` (`go mod tidy`)
- All 21 tests pass; the spin repo's own `spin doctor` reports `3 passed, 1 warned` (goimports missing in this worktree), proving the human renderer + JSON schema + exit-code math all work end-to-end

## Task Commits

1. **Task 1: Build internal/doctor package -- Check interface, registry, 4 universal checks** - `133f659` (feat)
2. **Task 2: Add human + JSON renderers + format selector** - `5c867e8` (feat)
3. **Task 3: Wire cmd/doctor.go cobra subcommand with all 4 flags** - `ef07af7` (feat)
4. **Plan metadata: go.mod promotion of lipgloss/v2 to direct** - `ebc6090` (chore)

## Files Created/Modified

- `internal/doctor/doctor.go` -- `Status` enum (pass/warn/fail), `CheckResult`, `Check` interface, `Fixer` interface, `Registry`, `RunOptions`, top-level `Run` orchestrator, `exitCode` math (0/1 with strict promotion)
- `internal/doctor/checks.go` -- `GoVersionCheck` (parses `go version` output, classifies via `semver.Compare`), `ToolPresenceCheck` (exec.LookPath for the 4 default tools; implementer of Fixer), `GoModHygieneCheck` (modfile.Parse with go/module presence and direct/indirect duplicate detection; implementer of Fixer), `CGOBuildCheck` (60s timeout exec.CommandContext with CGO_ENABLED=0 env), `DeepLintCheck` (registered only when opts.Deep is true; warn-not-fail when golangci-lint missing)
- `internal/doctor/render.go` -- `RenderHuman` (lipgloss-styled icon glyphs only; summary line "N passed, M warned, K failed"; indented hint lines), `RenderJSON` (json.Encoder to a stable schema with `jsonCheck` view type), `FormatSelector` (dispatcher; rejects unknown formats with a message naming the allowed set)
- `internal/doctor/doctor_test.go` -- 5 tests for the exit-code math: all-pass, any-fail, warn-no-strict, warn-strict, end-to-end fix-error-surfaces-in-result
- `internal/doctor/checks_test.go` -- 8 tests: GoVersionCheck accepts current + rejects too old (hermetic boundary), ToolPresenceCheck detects missing, GoModHygieneCheck no-go-mod + valid-mod, CGOBuildCheck pass-on-spin-itself, DeepLintCheck registered-when-deep, default-registry-has-four-base-checks
- `internal/doctor/render_test.go` -- 5 tests: RenderHuman all-pass + fail-with-hint, RenderJSON schema unmarshal, FormatSelector rejects-unknown + defaults-to-human
- `cmd/doctor.go` -- `doctorCmd` registered via init() with `--format`, `--strict`, `--deep`, `--fix`; `runDoctor` reads flags, builds `doctor.RunOptions`, calls `doctor.Run` then `doctor.FormatSelector`; non-zero exit wraps "N check(s) failed" in a cobra error for fang
- `cmd/doctor_test.go` -- 3 tests: registration via pointer identity, all 4 flags with expected defaults, `spin doctor --help` mentions all flags
- `go.mod` -- `charm.land/lipgloss/v2` promoted from `// indirect` to direct (no new modules; lipgloss was already in the graph via fang/huh/log)

## Decisions Made

- ToolPresenceCheck returns warn-not-fail when a tool is missing, with the install command in the Hint field. This is consistent with CONTEXT D-05: the install is repairable under `--fix`, and missing tools don't block `go build` so they shouldn't fail the audit by default
- Fix errors are surfaced in `CheckResult.Message` with a `fix failed: <err>` prefix rather than returned as a Go error from `Run()`. The CLI keeps rendering the report even when one repair failed, and the user sees what was tried. The post-fix status is downgraded pass→warn so the exit code reflects the situation
- The JSON schema is locked via a separate `jsonCheck` view type rather than `json:` tags on the public `CheckResult`. Two reasons: (1) the human renderer doesn't need JSON tags, so the public type stays clean; (2) the schema can be bumped in one place without touching the rest of the package
- The 5th check (`DeepLintCheck`) is registered only when `RunOptions.Deep` is true. The base doctor stays fast (sub-5s) per CONTEXT D-04, and the lint check is opt-in

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `go version` output parser failed on host's `go1.26.2 linux/amd64` output**
- **Found during:** Task 1 (`TestGoVersionCheck_AcceptsCurrent`)
- **Issue:** The plan's parser used `strings.TrimPrefix(raw, "go")` then `TrimPrefix(v, "version ")`, but for input `go version go1.26.2 linux/amd64` this produced `version go1.26.2 linux/amd64` (extra leading space) and the second trim left the prefix intact, so the version didn't extract correctly. The check returned "could not parse go version" and failed the host environment
- **Fix:** Replaced prefix-trim with a `strings.Fields` + prefix scan: find the field that starts with `go` followed by a digit, take everything from index 2 onward. This handles `go1.X.Y`, `go1.X.Y linux/amd64`, `go version go1.X.Y` uniformly
- **Files modified:** `internal/doctor/checks.go`
- **Verification:** `TestGoVersionCheck_AcceptsCurrent` passes on the host (go 1.26.2); `TestGoVersionCheck_RejectsTooOld` continues to pass (hermetic boundary check, not host-dependent)
- **Committed in:** `133f659` (Task 1 commit)

**2. [Rule 1 - Bug] `ToolPresenceCheck` lost missing tool names when all 4 tools were missing**
- **Found during:** Task 1 (`TestToolPresenceCheck_DetectsMissing`)
- **Issue:** The `len(found) == 0` branch set Message = "no optional tools on $PATH" without naming the missing tools. The plan's test asserts the Message mentions the missing tool name ("definitely-not-a-real-tool-xyzzy"). The test was correctly specified; the impl was the bug
- **Fix:** In the all-missing branch, build `mnames` from the missing tools and include them in the Message: "no optional tools on $PATH (missing: <names>)". The Hint stays as the install commands so users can fix all 4 in one go
- **Files modified:** `internal/doctor/checks.go`
- **Verification:** `TestToolPresenceCheck_DetectsMissing` passes; `TestRenderHuman_FailWithHint` (Task 2) unaffected
- **Committed in:** `133f659` (Task 1 commit)

**3. [Rule 1 - Bug] `modfile.Module.Mod` is a struct, not a pointer -- `== nil` is invalid**
- **Found during:** Task 1 (first build)
- **Issue:** `internal/doctor/checks.go` had `mf.Module == nil || mf.Module.Mod == nil || mf.Module.Mod.Path == ""`. The compiler error: `mismatched types module.Version and untyped nil`. `module.Version` is a value type, not a pointer
- **Fix:** Drop the `mf.Module.Mod == nil` check; the struct's `Path` field being empty is sufficient
- **Files modified:** `internal/doctor/checks.go`
- **Verification:** `go build ./internal/doctor/...` succeeds; `TestGoModHygieneCheck_NoGoMod` and `TestGoModHygieneCheck_ValidMod` both pass
- **Committed in:** `133f659` (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (all Rule 1 -- code-doesn't-work bugs caught by tests)
**Impact on plan:** All three fixes were necessary for the package to compile and tests to pass. No scope creep; no spec changes; no architectural decisions.

## Issues Encountered

- **Pre-existing test hangs in `internal/wrap` and `internal/scaffold`:** Running `go test ./...` hung on `TestRun_WithAirToml` (wrap) and a scaffold test, both of which shell out to `air` and never receive a TTY signal to exit. These hang in this worktree environment regardless of my changes; they predate plan 04-01. Verification was done by running `./internal/doctor/...`, `./cmd/...`, and `./internal/prompt/...` individually with `-short` and timeouts. Not in scope; the orchestrator can address these when consolidating the test suite.
- **`semver` import missing in `checks_test.go`:** `TestGoVersionCheck_RejectsTooOld` needed the `golang.org/x/mod/semver` package for its hermetic boundary check. The fix was a one-line import addition -- not a deviation, but worth noting in case a future plan runs the same boundary test pattern.
- **Initial `exitCode` test in `doctor_test.go` over-engineered:** First draft of `TestRun_Orchestrator_FixFlagDoesNotChangeExitCode` used two boolean helpers (`yesFix`, `optsFix`) that obscured the assertion. Simplified to drive the same code paths `Run()` does (build registry, run, conditionally fix, annotate, compute exit) without the wrapper helpers. Net: the test is now readable, and the fix-error annotation path is exercised directly.

## Threat Model Coverage

All STRIDE mitigations in PLAN.md were applied:

- **T-04-01 (GoModHygieneCheck):** uses `golang.org/x/mod/modfile.Parse` (no eval, no shell-out). Verified by `TestGoModHygieneCheck_NoGoMod` and `TestGoModHygieneCheck_ValidMod`
- **T-04-02 (CGOBuildCheck):** static argv (no interpolation), `CGO_ENABLED=0` env passed via `cmd.Env` (not shell), 60s context timeout, output truncated to 1KB. Verified by `TestCGOBuildCheck_PassOnSpinItself`
- **T-04-03 (ToolPresenceCheck):** `exec.LookPath` only (no execution). Verified by `TestToolPresenceCheck_DetectsMissing`
- **T-04-04 (JSON output):** schema contains only name/status/message/hint -- no env vars, no paths beyond the `module <path>` string in the message
- **T-04-05 (--fix elevation):** `go mod tidy` is read-mostly; `go install` only fires for the 4 whitelisted tools with pinned install commands in code -- no user input interpolated. Verified by code review (defaultTools is a package-level const)
- **T-04-06 (DoS via huge monorepo):** 60s `exec.CommandContext` timeout on CGOBuildCheck, 90s on DeepLintCheck, 5min per tool in ToolPresenceCheck.Fix
- **T-04-07 (exit code contract):** 5 tests in `doctor_test.go` exercise the exit-code matrix (all-pass, any-fail, warn-no-strict, warn-strict, fix-failure downgrades pass→warn)
- **T-04-08 (no new package installs):** verified -- `charm.land/lipgloss/v2` was already in the module graph; the only go.mod change is the `// indirect` → direct promotion

## Next Phase Readiness

Plan 04-02 (`spin lint` wrapper) can now build on:
- The `golangci-lint` exec pattern in `DeepLintCheck.Run()` (the cmd/lint wrapper mirrors this)
- The `internal/wrap/RunWithFallback` pattern for the missing-tool install hint
- The `cmd/doctor.go` registration pattern (`init()` + `rootCmd.AddCommand(...)` + 4 flags) as the template for the new `cmd/lint.go`

No blockers. No carry-forward TODOs. `internal/doctor` and `cmd/doctor.go` are ready for integration.

## Self-Check: PASSED

- All 8 created files exist on disk (8 .go files in internal/doctor/ and cmd/, plus SUMMARY.md)
- All 4 task commits exist in git log: `133f659`, `5c867e8`, `ef07af7`, `ebc6090`
- `go test ./internal/doctor/... -count=1`: ok (18 tests pass)
- `go test ./cmd/... -count=1 -run Doctor`: ok (3 tests pass)
- `go build ./...`: succeeds
- `go vet ./...`: clean
- `bash scripts/check-v1-leaks.sh ./internal/doctor`: OK, no v1 leaks detected
- `bash scripts/check-v1-leaks.sh ./cmd`: OK, no v1 leaks detected
- Manual verification: `spin doctor` (human) prints 4 checks + summary; `spin doctor --format json` emits stable schema; `spin doctor --strict` exits 1 on warn; `spin doctor --deep` adds lint; `spin doctor --help` lists all 4 flags

---
*Phase: 04-post-scaffold-health-verification-dogfooding*
*Plan: 01*
*Completed: 2026-06-05*
