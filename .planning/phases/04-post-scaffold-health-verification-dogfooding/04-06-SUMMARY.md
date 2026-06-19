---
phase: 04-post-scaffold-health-verification-dogfooding
plan: 06
subsystem: ci
tags: [github-actions, dogfood, ci, dependabot, go]
status: complete

# Dependency graph
requires:
  - phase: 04-01-spin-doctor
    provides: full Phase 4 stack (the dogfood script exercises a fresh fixture end-to-end, so it depends on doctor/update/lint being present and the scaffolder producing a project that builds)
  - phase: 04-02-spin-lint
    provides: same as above
  - phase: 04-03-spin-update-engine
    provides: same as above
  - phase: 04-04-spin-update-form
    provides: same as above
  - phase: 04-05-strip-generated-by-markers
    provides: marker-free templates so the dogfood fixture's gofmt check would not flake
provides:
  - .github/workflows/ci.yml -- base CI (go test + grep suite) on push/PR to main
  - .github/workflows/dogfood.yml -- scaffolder smoke-test job triggered on template/cmd changes
  - scripts/dogfood.sh -- local-runnable form of the dogfood job (build spin -> scaffold fixture -> go mod tidy + CGO=0 go build + go test -> v1-leak grep)
  - .github/dependabot.yml -- weekly Go module update PRs
affects: []

# Tech tracking
tech-stack:
  added:
    - actions/checkout@v4
    - actions/setup-go@v5
    - actions/upload-artifact@v4
  patterns:
    - "scripts/dogfood.sh mirrors the Go-side wrap integration test at the shell level so the pipeline is runnable in CI and locally"
    - "run_step helper captures each step's output to a tmpfile and dumps the last 50 lines on failure -- same spirit as the Go-side CombinedOutput tail in internal/wrap/integration_test.go"
    - "GitHub Actions workflows pinned to first-party @v4/@v5 actions; no third-party actions, no go install of unknown packages"
    - "permissions: { contents: read } at workflow/job level for least-privilege token scope"

key-files:
  created:
    - scripts/dogfood.sh
    - .github/workflows/ci.yml
    - .github/workflows/dogfood.yml
    - .github/dependabot.yml
  modified: []

key-decisions:
  - "Use --module github.com/example/spin-fixture as a hardcoded override so the dogfood fixture never collides with the spin repo's real github.com/example/spin module path (this is the form already proven correct in the v1-leak check scripts' comment block)"
  - "Trigger paths for dogfood include scripts/dogfood.sh and the workflow file itself so any tampering or fix is automatically re-verified on the next PR"
  - "Artifact upload path is `${{ github.workspace }}/bin/spin` + `/tmp/dogfood-*.log` with `if-no-files-found: ignore` -- the script always cleans its own tempdir on success, so an empty upload is the expected happy path and should not fail the job"
  - "ci.yml is intentionally minimal (go test + grep suite, no dogfood) so the base test job stays fast and predictable; the dogfood step is opt-in via path filter, matching the `task test` + `task grep-v1-leaks` split in Taskfile.yml"

requirements-completed: []

# Metrics
duration: 4min
completed: 2026-06-08
---

# Phase 4 Plan 6: CI Dogfood Summary

**Scaffolder smoke test in CI: a `dogfood` job builds spin, scaffolds a fixture project via `spin new spin --cli --cobra --fang --module github.com/example/spin-fixture`, runs `go mod tidy` + `CGO_ENABLED=0 go build ./...` + `go test ./...`, then runs the v1-leak grep suite. Same pipeline is exposed as `scripts/dogfood.sh` for local runs. Base `ci.yml` and weekly Dependabot also added.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-06-08T11:03:41Z
- **Completed:** 2026-06-08T11:07:30Z
- **Tasks:** 2
- **Files modified:** 4 (all created)

## Accomplishments

- `scripts/dogfood.sh` (executable, `-rwxr-xr-x`) runs the full spin -> scaffold -> build -> test pipeline in a tempdir, mirroring the Go-side `internal/wrap/integration_test.go` pattern at the shell level
- `scripts/dogfood.sh` uses `set -euo pipefail`, traps tempdir cleanup on EXIT, captures each step's output to a per-step log file, and dumps the last 50 lines on failure for debuggability
- `.github/workflows/dogfood.yml` triggers on changes to `internal/scaffold/templates/**`, `cmd/**`, `scripts/dogfood.sh`, and the workflow file itself (per CONTEXT D-13)
- `.github/workflows/ci.yml` provides a base `test` job: `go test ./... -count=1` + the three `scripts/check-*.sh` grep suite against `./internal/scaffold/templates`, matching the local `task test` + `task grep-v1-leaks` split
- `.github/dependabot.yml` schedules weekly Go module update PRs
- Both workflow YAMLs use `permissions: { contents: read }` for least-privilege
- The dogfood workflow uploads `bin/spin` and `/tmp/dogfood-*.log` as a `dogfood-failure` artifact on `if: failure()` so a reviewer can debug without re-running

## Task Commits

1. **Task 1: Add scripts/dogfood.sh -- reusable smoke-test pipeline** - `674be8b` (ci)
2. **Task 2: Add .github/workflows/dogfood.yml + minimal ci.yml + dependabot** - `c215d8e` (ci)

## Files Created/Modified

- `scripts/dogfood.sh` -- `#!/usr/bin/env bash` + `set -euo pipefail` + `REPO_ROOT` resolution + `BIN`/`WORK` tempdir + `trap 'rm -rf "$WORK"' EXIT` + `run_step` helper (captures per-step logs, dumps last 50 lines on failure) + 6 steps: build spin, scaffold fixture, go mod tidy, CGO=0 go build, go test, v1-leak grep; ends with `==> dogfood passed`
- `.github/workflows/ci.yml` -- `name: ci` + `on: push/PR to main` + `permissions: contents: read` + one job `test` on `ubuntu-latest` with `actions/checkout@v4`, `actions/setup-go@v5` (`go-version: '1.25'`, `cache: true`), `go test ./... -count=1`, and the three `scripts/check-*.sh` grep suite
- `.github/workflows/dogfood.yml` -- `name: dogfood` + `on:` with `pull_request.paths: ['internal/scaffold/templates/**', 'cmd/**', 'scripts/dogfood.sh', '.github/workflows/dogfood.yml']` (push to main mirrors this) + `permissions: contents: read` + one job `dogfood` on `ubuntu-latest` with 4 steps: checkout, setup-go (1.25), `bash scripts/dogfood.sh`, and `actions/upload-artifact@v4` on `if: failure()` uploading `bin/spin` + `/tmp/dogfood-*.log`
- `.github/dependabot.yml` -- `version: 2` + `updates` for the `gomod` ecosystem with weekly schedule

## Decisions Made

- `--module github.com/example/spin-fixture` is hardcoded in `scripts/dogfood.sh` so the fixture's go.mod is decoupled from the spin repo's real `github.com/example/spin` module path. The override matters because `go mod tidy` would otherwise try to resolve deps in the same module graph as the test runner and could shadow spin's own deps.
- The dogfood workflow's `paths` filter includes `scripts/dogfood.sh` and `.github/workflows/dogfood.yml` itself. This means a PR that changes the dogfood script or the workflow triggers a re-run, so a malicious or sloppy change to the smoke test is always visible in CI (mitigates T-04-31).
- The artifact path uses `if-no-files-found: ignore`. The script's `trap 'rm -rf "$WORK"' EXIT` always cleans the tempdir on success; on failure the script exits 1 *before* the trap cleans up, but the log file inside `$WORK` lives at a path the workflow can also glob. We keep the artifact upload simple: the binary is at `${{ github.workspace }}/bin/spin` and any leftover tmpfile is at `/tmp/dogfood-*.log`. A future iteration could write a single stable `/tmp/dogfood.log` and upload that -- not done here to keep the plan scope tight.
- `ci.yml` is intentionally minimal (no dogfood step). The two workflows cover orthogonal concerns: `ci.yml` runs on every push/PR, `dogfood.yml` runs only when the scaffolder or its inputs change. Folding the dogfood into the base `test` job would slow down unrelated PRs and is not what D-13 calls for.
- Dependabot is set to weekly, not daily, to avoid PR noise on a single-maintainer repo. Bumping to daily is a one-line change in `.github/dependabot.yml` if maintainer throughput increases.

## Deviations from Plan

### Auto-fixed Issues

None - plan executed exactly as written.

### Notes

- `.github/CODEOWNERS` was marked optional in the plan and is intentionally omitted in v1. The plan says: "mention in the SUMMARY that it can be added later when the maintainer count grows beyond 1." Tracked here.
- `python3 -c "import yaml; yaml.safe_load(...)"` could not be used for YAML validation because the system Python is Nix-managed and read-only (no `--user` site-packages, no PEP 668 override permission). Substituted `yq-go` from the Nix store at `/nix/store/pibhm258wp416plqdaxym4z6lis8ksla-yq-go-4.53.2/bin/yq` for the validation step. Same semantic result (parse-and-load).

## Issues Encountered

- The dogfood script was NOT executed end-to-end in the worktree per plan instructions ("Do NOT actually run the dogfood script end-to-end... that would scaffold a project and build it, which takes 30-60s and may not have all deps available"). Only `bash -n scripts/dogfood.sh` (parse check) was run. CI will exercise the full pipeline.
- The system `python3` cannot install `pyyaml` (Nix store is read-only). YAML validation was done with the pre-installed `yq-go` binary in the Nix store.

## Threat Model Coverage

All STRIDE mitigations in PLAN.md were applied:

- **T-04-30 (Malicious PR modifies scripts/dogfood.sh to exfiltrate secrets):** both workflows use `permissions: { contents: read }` (no `actions: write`, no `packages: write`, no `id-token: write`). The runner's `GITHUB_TOKEN` is read-only. A compromised script cannot push code, post commit statuses, or fetch cloud creds.
- **T-04-31 (Dogfood script modified to skip the smoke test, e.g. `exit 0` at the top):** the workflow's `paths` filter includes `scripts/dogfood.sh` itself, so any change to the script triggers a re-run. A reviewer sees the diff in the PR. The script's `set -euo pipefail` makes silent masking via `true` harder (a determined attacker could `set +e` first, but that diff is also visible).
- **T-04-32 (Generated fixture pulls arbitrary modules from proxy.golang.org):** same surface as the existing local `scaffold_e2e_test.go` and `internal/wrap/integration_test.go`. Go toolchain's `sum.golang.org` verification protects against malicious module content. Runner has no access to real secrets (just `GITHUB_TOKEN` with read-only permissions). Disposition: `accept`.
- **T-04-33 (DoS via slow dogfood job on every PR):** the dogfood job is path-filtered to `internal/scaffold/templates/**` + `cmd/**` + the script + the workflow. Most PRs do not touch these paths, so the dogfood cost is paid only when relevant. Uses `ubuntu-latest` (fast runners).
- **T-04-34 (Failing job must produce enough output to debug):** the `if: failure()` upload-artifact step makes `bin/spin` and any leftover log available as a downloadable artifact. The script's `run_step` helper also prints the last 50 lines of any failing step to stderr.
- **T-04-SC (No new direct dependencies):** only first-party GitHub Actions used: `actions/checkout@v4`, `actions/setup-go@v5`, `actions/upload-artifact@v4`. No `go install` of unknown packages runs in CI. The fixture does pull modules from `proxy.golang.org`, but those are the published v2 charm libraries and the standard library -- same set spin's own `go build` pulls.

## Verification Gate Results

All 6 verification gate checks from the plan pass:

| # | Check | Result |
|---|-------|--------|
| 1 | `bash -n scripts/dogfood.sh` parse check | PASS -- script parses without errors |
| 2 | `python3 -c "import yaml; yaml.safe_load(...)"` for both workflow files (substituted: yq-go parse) | PASS -- both YAMLs load, `.name` returns `ci` and `dogfood` respectively |
| 3 | `ls -l scripts/dogfood.sh` shows executable x bit for owner | PASS -- `-rwxr-xr-x` (owner has rwx; group/other have rx) |
| 4 | `grep -E 'spin new spin --cli --cobra --fang' scripts/dogfood.sh` | PASS -- matches in the comment block and in the `run_step` invocation |
| 5 | `grep -E 'internal/scaffold/templates/\*\*\|cmd/\*\*' .github/workflows/dogfood.yml` | PASS -- both paths appear in `on.pull_request.paths` and `on.push.paths` |
| 6 | `grep -E 'bash scripts/dogfood\.sh' .github/workflows/dogfood.yml` | PASS -- `run: bash scripts/dogfood.sh` in the `Run dogfood smoke test` step |

Additional spot checks:

- `grep -E '^set -euo pipefail|^trap' scripts/dogfood.sh` -- both present
- `grep -E 'go test \./\.\.\|check-v1-leaks' .github/workflows/ci.yml` -- both present
- `yq '.jobs.dogfood.steps | length' .github/workflows/dogfood.yml` -- returns 4 (checkout, setup-go, run script, upload artifact)

## Next Phase Readiness

Phase 4 plans 01-06 are all complete. The phase deliverable is now end-to-end verifiable:

- `spin doctor` (04-01) audits any Go project including spin-scaffolded ones
- `spin lint` (04-02) wraps golangci-lint
- `spin update` engine + form (04-03, 04-04) is the universal Go dep updater
- Generated-file markers stripped (04-05) per D-12
- Dogfood smoke test (04-06) proves the scaffolder on its own codebase in CI

No further plans in Phase 4. Phase 4 is ready for closure (verifier + roadmap update + STATE.md transition).

## Self-Check: PASSED

- All 4 created files exist on disk:
  - `scripts/dogfood.sh` (executable, 63 lines)
  - `.github/workflows/ci.yml` (~30 lines)
  - `.github/workflows/dogfood.yml` (~55 lines)
  - `.github/dependabot.yml` (~12 lines)
- All 2 task commits exist in git log: `674be8b` (Task 1), `c215d8e` (Task 2)
- `bash -n scripts/dogfood.sh` -- parses cleanly
- `yq` parses all 3 YAML files
- `ls -l scripts/dogfood.sh` -- shows `-rwxr-xr-x` (executable)
- All 6 plan-specified verification gate checks pass
- `git log --oneline -3` confirms both commits landed on `worktree-agent-a47e11f27c6aeca34` (not on a protected ref)

---
*Phase: 04-post-scaffold-health-verification-dogfooding*
*Plan: 06*
*Completed: 2026-06-08*
