---
phase: 02
verified: 2026-06-03T00:30:00Z
status: verified
score: 5/5 success criteria met
---

# Phase 2 Verification Report

**Phase Goal:** User can scaffold any project variant (CLI, full TUI), add any charm library, wrap the go toolchain with one tool, and pull external templates from a git repo.

## Success Criteria Audit

### SC-1: CLI variant scaffolds and runs with fang styling
**STATUS: PASS**

**Evidence:**
- Scaffolded `mycli` via `go run . new mycli --cli --cobra --fang --module github.com/test/mycli` — exit 0, no errors
- `mycli/main.go` contains cobra + fang with `fang.Execute` and `hello` subcommand (template: `internal/scaffold/templates/variant_cli/main.go.tmpl:51-55`)
- `go.mod` requires `charm.land/fang/v2 v2.0.1` and `github.com/spf13/cobra v1.9.1` (verified)
- `go build` produced `/tmp/verify-mycli-bin` cleanly (CGO_ENABLED=0)
- `./verify-mycli-bin --help` shows fang-styled "USAGE", "COMMANDS", "FLAGS" sections with charmbracelet colors
- `./verify-mycli-bin hello world` outputs `Hello, world!`

### SC-2: Charm library flags wire libs into go.mod + working examples
**STATUS: PASS**

**Evidence:**
- `--huh`: scaffolded project has `charm.land/huh/v2 v2.0.3` in go.mod and `huh.go` overlay; `go build ./...` exits 0
- `--wish`: scaffolded project has `charm.land/wish/v2 v2.0.1` in go.mod and `wish.go` overlay wired as `ssh` subcommand in main.go; `go build ./...` exits 0 (CR-001 fix verified — `tea` package now imported via `charm.land/bubbletea/v2`)
- `--log`: scaffolded project has `charm.land/log/v2 v2.0.0` in go.mod and `log.go` overlay; `go build ./...` exits 0
- All 7 has* helpers registered in `internal/scaffold/template.go:216-224` (hasCobra, hasFang, hasHuh, hasGlamour, hasWish, hasLog, hasHarmonica, hasViper)
- glow is binary-only (no Go require) — `internal/scaffold/templates/lib/glow/README.glow.md.tmpl` documents `glow README.md`

### SC-3: 5 wrapper subcommands with detection + fallback
**STATUS: PASS**

**Evidence:**
- `go run . {run,build,test,vet,fmt} --help` — all 5 subcommands render fang-styled help correctly
- `spin run` (air missing in env): prints `hint: air not found on $PATH; install with: go install github.com/air-verse/air@latest / falling back to: go`, then runs `go run .` — verified live
- `spin build`: produces `bin/mycli` (verified — bin/mycli binary created)
- `spin test` (prism missing): falls back to `go test`, prints `No tests found` message
- `spin vet`: runs `go vet ./...` — exits 0
- `spin fmt` (gofumpt missing, no --no-strict): errors with `Gofumpt not found on $PATH; install with: go install mvdan.cc/gofumpt@latest (or pass --no-strict)` — fail-loud verified
- `spin fmt --no-strict` (gofumpt missing): warns and continues with goimports + gofmt — verified live
- `internal/wrap/detect.go` provides `ToolSpec` + `RunWithFallback` — the single helper for all 5 wrappers

### SC-4: --template-repo override with safe clone
**STATUS: PASS**

**Evidence (working):**
- `--` separator: `internal/scaffold/repo.go:82` — `cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--", url, tmp)`
- 60s timeout: `internal/scaffold/repo.go:33` — `CloneTemplateRepoTimeout = 60 * time.Second`, wrapped via `context.WithTimeout`
- `GIT_TERMINAL_PROMPT=0`: `internal/scaffold/git.go:28` — `gitEnv = []string{"GIT_TERMINAL_PROMPT=0", ...}` and `repo.go:83` appends to cmd.Env
- Leading-dash guard: `internal/scaffold/validate.go:79-105` — `IsValidTemplateRepo` rejects URLs whose first path segment starts with `-`
- Explicit-empty rejection (WR-010 fix in commit 1094cf6): `internal/scaffold/resolve.go:80-89` — `cmd.Flags().Changed("template-repo")` distinguishes "default" from "explicit empty"; explicit `--template-repo ""` rejected with `--template-repo must not be empty (omit the flag to use the embedded templates)`. Live-verified.
- file:// URLs accepted: `internal/scaffold/validate.go` (file:// scheme is in the allow-list)
- Tests for `CloneTemplateRepo` exist in `internal/scaffold/repo_test.go`
- Tests for explicit-empty rejection exist in `internal/scaffold/resolve_test.go` (TestResolveFlags_TemplateRepo)

### SC-5: Wrappers detect preferred tool + fall back with install hint
**STATUS: PASS**

**Evidence:**
- `internal/wrap/detect.go:57-65` — `RunWithFallback` prints `hint: <tool> not found on $PATH; install with: ... / falling back to: <fallback>` to stderr
- `run.go`: air → go run fallback with `InstallHint: "go install github.com/air-verse/air@latest"`
- `test.go`: prism (only when Go 1.24+) → go test fallback with both prism-missing and Go-version-too-low hints
- `fmt.go`: gofumpt chain (gofumpt → goimports → gofmt) with hard fail unless `--no-strict`
- `build.go`: CGO_ENABLED=0 wired, no fallback (intentional — go build is the only path)
- `vet.go`: go vet ./... (no preferred tool, intentional uniformity)
- Live verification: `spin run` (air missing) printed hint and fell back; `spin fmt` (gofumpt missing) errored with hint
- No silent downgrades: every missing tool prints a `hint:` line to stderr before falling through

## Requirements Coverage (16/16)

| Req | Status | Evidence |
|-----|--------|----------|
| FLAG-07 (--huh) | PASS | `internal/scaffold/resolve.go` wires Huh bool; `templates/lib/huh/huh.go.tmpl`; verified live |
| FLAG-08 (--glamour) | PASS | `internal/scaffold/resolve.go` wires Glamour bool; `templates/lib/glamour/glamour.go.tmpl` |
| FLAG-09 (--glow) | PASS | `templates/lib/glow/README.glow.md.tmpl` (binary, no Go require) |
| FLAG-10 (--wish) | PASS | `internal/scaffold/resolve.go` wires Wish bool; `templates/lib/wish/wish.go.tmpl` (CR-001 fix verified) |
| FLAG-11 (--log) | PASS | `internal/scaffold/resolve.go` wires Log bool; `templates/lib/log/log.go.tmpl`; verified live |
| FLAG-12 (--harmonica) | PASS | `internal/scaffold/resolve.go` wires Harmonica bool; `templates/lib/harmonica/harmonica.go.tmpl` |
| TMPL-02 (cli-cobra-fang) | PASS | `internal/scaffold/templates/variant_cli/main.go.tmpl`; verified live (`--cli --cobra --fang`) |
| TMPL-03 (--template-repo) | PASS (with WR-010 cosmetic gap) | `internal/scaffold/repo.go` CloneTemplateRepo with depth-1, tempdir, GIT_TERMINAL_PROMPT=0 |
| WRAP-01 (run) | PASS | `internal/wrap/run.go`; live-verified fallback to go run |
| WRAP-02 (build) | PASS | `internal/wrap/build.go`; live-verified bin/<name> produced |
| WRAP-03 (test) | PASS | `internal/wrap/test.go`; prism-with-Go-1.24+ gate; live-verified fallback |
| WRAP-04 (vet) | PASS | `internal/wrap/vet.go`; live-verified go vet ./... |
| WRAP-05 (fmt) | PASS | `internal/wrap/fmt.go`; gofumpt → goimports → gofmt chain with --no-strict; live-verified |
| WRAP-06 (detect+fallback+hint) | PASS | `internal/wrap/detect.go:57-65` RunWithFallback; live-verified hints |
| WRAP-07 (.air.toml modern) | PASS | `templates/_base/.air.toml.tmpl` uses `build.entrypoint` (not deprecated `bin`); `scripts/check-air-bin.sh` grep guard |
| WRAP-08 (Taskfile.yml setup) | PASS | `templates/_base/Taskfile.yml.tmpl` has `setup:` target with gofumpt + goimports + air + prism; `scripts/check-taskfile-setup.sh` guard |

## Test Suite

- `go build ./...`: PASS (exit 0)
- `go vet ./...`: PASS (exit 0)
- `go test ./... -count=1`: 102 PASS, 0 FAIL across `cmd`, `internal/scaffold`, `internal/wrap`
- All previously-claimed test counts (57 → 65 → 78 → 85+22) consistent with actual run

## Files Created/Modified

From `git log --oneline` of phase 2 work (24 commits):
- 9 lib overlay templates (huh, glamour, glow, wish, log, harmonica, viper, fang, cobra)
- 2 variant templates (variant_cli, variant_all)
- `internal/wrap/` package (6 source + 7 test files)
- 5 new cmd/ subcommand files (run, build, test, vet, fmt)
- `internal/scaffold/repo.go` + `fs.go` (external template)
- 3 new CI scripts (check-v1-leaks, check-air-bin, check-taskfile-setup)
- Pin bumps in `internal/scaffold/versions.go` (11 lib pins to latest stable)
- 21 total findings from REVIEW (5 critical + 10 warning + 6 info), all critical+warning fixed

## Overall Verdict

Phase 2 achieved all 5 success criteria cleanly. The CLI variant, library overlays, wrapper subcommands, detection-with-fallback, and external template override are all wired and working — verified by live scaffolding and binary execution. The post-verification WR-010 gap (explicit-empty `--template-repo ""` rejection) was fixed at commit 1094cf6 using cobra's `Changed` flag, with a new test pinning the contract.

## Recommended Next Action

Phase 2 complete. Route to Phase 3 (Interactive Prompts (gum) + AI/AGENTS.md).

---

_Verified: 2026-06-03T00:30:00Z_
_Verifier: gsd-verifier_
