---
phase: 02
verified: 2026-06-03T01:25:00Z
checker: gsd-plan-checker
status: pass_with_warnings
success_criteria_coverage:
  SC-1: "pass | 02-03 Task 1+2 ship variant_cli + variant_all main.go.tmpl with fang.Execute, cobra hello subcommand, and 3 integration tests (TestIntegrationScaffold_CLI, TestIntegrationScaffold_All, TestIntegrationScaffold_CLI_Viper) verify --help + hello world end-to-end"
  SC-2: "partial | 02-03 Tasks 3-5 deliver 6 lib overlays + 9 go.mod require entries + TestIntegrationScaffold_<Lib> tests; --harmonica correctly stays on github.com/charmbracelet/harmonica; but --wish main.go wiring is comment-only stub (no ssh subcommand), and --glow has no Taskfile setup target install (FLAG-09 partial)"
  SC-3: "pass | 02-04 Tasks 1-6 ship internal/wrap/{detect,run,build,test,vet,fmt}.go with RunWithFallback helper; Task 6 wires 5 cobra subcommands; --no-strict flag on fmt; prism Go 1.24+ detector in test.go; 6 integration tests + 5 unit tests verify"
  SC-4: "pass | 02-02 Tasks 1-5 ship templateFS interface, CloneTemplateRepo (depth-1, GIT_TERMINAL_PROMPT=0, _base/ validation), --template-repo + --keep-template-cache flags, runNew lifecycle, walker refactor; Task 7 end-to-end smoke proves external repo content overrides embedded"
  SC-5: "pass | 02-04 Task 1's ToolSpec + RunWithFallback is the single helper for all 5 wrappers; every wrapper prints a one-line install hint on missing preferred tool and falls back to stock Go; Task 8 unit tests (TestFmt_GofumptMissing_Strict, TestTest_GoTestFallback, etc.) verify no silent downgrade"
requirement_coverage:
  FLAG-07: "02-02 (hasHuh funcMap helper) + 02-03 (lib/huh/huh.go.tmpl + TestIntegrationScaffold_Huh) | template helper added in template.go Task 4; overlay file created Task 3; integration test asserts go.mod has charm.land/huh/v2 v2.0.3 and huhExample compiles"
  FLAG-08: "02-02 (hasGlamour funcMap) + 02-03 (lib/glamour/glamour.go.tmpl + TestIntegrationScaffold_Glamour) | same shape as huh"
  FLAG-09: "02-02 (hasGlow funcMap) + 02-03 (lib/glow/README.glow.md.tmpl + TestIntegrationScaffold_Glow) | README mention added; NO Taskfile setup target install for glow (deferred gap)"
  FLAG-10: "02-02 (hasWish funcMap) + 02-03 (lib/wish/wish.go.tmpl + TestIntegrationScaffold_Wish) | lib overlay created; main.go wiring is comment-only stub (no ssh subcommand)"
  FLAG-11: "02-02 (hasLog funcMap) + 02-03 (lib/log/log.go.tmpl + TestIntegrationScaffold_Log) | log.SetDefault + log.SetLevel example wired into variant_tui main"
  FLAG-12: "02-02 (hasHarmonica funcMap) + 02-03 (lib/harmonica/harmonica.go.tmpl + TestIntegrationScaffold_Harmonica) | correctly uses github.com/charmbracelet/harmonica path (not charm.land)"
  FLAG-13: "02-03 (variant_cli/main.go.tmpl default-on) | cobra root + fang.Execute with hello subcommand per research §3.1"
  FLAG-14: "02-03 (variant_cli/main.go.tmpl) | fang.Execute(ctx, rootCmd, fang.WithVersion) drop-in per research §3.1"
  FLAG-15: "02-02 (hasViper funcMap) + 02-03 (lib/viper/internal/config/config.go.tmpl + TestIntegrationScaffold_CLI_Viper) | Bind() + LogLevel() functions; main.go init() calls config.Bind(rootCmd) when hasViper is true"
  TMPL-02: "02-03 (variant_cli template content) | not a new --template selector string; variant is selected by --cli/--all flags, not --template cli-cobra-fang (deviation from REQUIREMENTS.md text; matches research §3.1)"
  TMPL-03: "02-02 (CloneTemplateRepo + --template-repo flag + templateFS abstraction) | depth-1 git clone, _base/ validation, GIT_TERMINAL_PROMPT=0, os.RemoveAll cleanup; verified by Task 6 + 7 tests"
  WRAP-01: "02-04 (cmd/run.go + internal/wrap/run.go) | air preferred, go run . fallback when .air.toml absent; RunWithFallback + stat check"
  WRAP-02: "02-04 (cmd/build.go + internal/wrap/build.go) | bin/<name> with CGO_ENABLED=0; name from filepath.Base(cwd)"
  WRAP-03: "02-04 (cmd/test.go + internal/wrap/test.go) | prism preferred, go test fallback; Go 1.24+ check via lexicographic runtime.Version() compare"
  WRAP-04: "02-04 (cmd/vet.go + internal/wrap/vet.go) | go vet ./... direct, no fallback needed (WRAP-06 trivially satisfied)"
  WRAP-05: "02-04 (cmd/fmt.go + internal/wrap/fmt.go) | gofumpt -> goimports -> gofmt chain; --no-strict flag bypasses gofumpt with warning; missing gofumpt in strict mode exits non-zero with install hint"
  WRAP-06: "02-04 (internal/wrap/detect.go) | ToolSpec + RunWithFallback is the single helper for all 5 wrappers; hint printed via fmt.Fprintf(os.Stderr, 'hint: %s not found on $PATH; %s\n...')"
  WRAP-07: "02-01 (grep refinement) + 02-04 (check-air-bin.sh + Taskfile wire) | .air.toml uses build.entrypoint; new bash script greps for deprecated bin = 'tmp/main' form"
  WRAP-08: "02-01 (grep refinement) + 02-04 (check-taskfile-setup.sh) | new bash script greps Taskfile.yml for top-level setup: with 4 tool installs (gofumpt, goimports, air, prism)"
wave_structure:
  wave_1: "[02-01, 02-02] | parallel_eligible: partial | overlap: none (different file lists), but implicit code-level dep exists: 02-02 Task 4 charmPin switch references new CharmPins struct fields (Huh, Glamour, Wish, Fang) created in 02-01 Task 1; if merged in parallel, 02-02 build breaks until 02-01 lands"
  wave_2: "[02-03, 02-04] | parallel_eligible: yes | overlap: none (02-03: internal/scaffold/templates/* + integration_v2_test.go + template.go; 02-04: cmd/*.go + internal/wrap/* + scripts/*.sh + Taskfile.yml + grep_test.go); 02-03 depends_on 02-02 declared; 02-04 depends_on [] but prompt says zero overlap with 02-03 (confirmed)"
critical_issues: []
warnings:
  - "W-01 (implicit code-level dep): 02-02 Task 4 extends the charmPin switch in template.go to cover 8 charm.land libs (bubbletea, lipgloss, bubbles, log, huh, glamour, wish, fang) plus harmonica (LegacyPins). The new pin fields Huh, Glamour, Wish, Fang are introduced in 02-01 Task 1's CharmPins struct. The plans' depends_on declarations claim 02-01+02-02 are parallel Wave 1, but 02-02 cannot build in isolation. Either re-declare 02-02 depends_on: ['02-01'] OR add the new fields to CharmPins in 02-02's commit before 02-01's pin bump (split the work). The cleanest fix: change 02-02 depends_on to ['02-01'] and reorder as 02-01 -> 02-02 in Wave 1."
  - "W-02 (frontmatter inconsistency): 02-04's files_modified lists scripts/check-v1-leaks.sh, but no Task in 02-04 modifies it. The v1-leaks script refinement is the entirety of 02-01 Task 4. 02-04's only script work is creating check-air-bin.sh + check-taskfile-setup.sh (Task 7) and updating Taskfile.yml grep-v1-leaks target. Drop scripts/check-v1-leaks.sh from 02-04's files_modified frontmatter."
  - "W-03 (FLAG-09 partial): 02-03 Task 5 explicitly defers the Taskfile.yml setup target install for glow ('added in Plan 02-04's Taskfile updates if needed; for v1, the README mentions it but the setup target is unchanged'), and 02-04's check-taskfile-setup.sh only checks for 4 tools (gofumpt, goimports, air, prism) — glow is never installed by the generated project. SC-2 says --glow is 'wired into a working example'; the README mention + Taskfile 'setup' target adding the install is the canonical 'wired' pattern. Either: (a) add glow to check-taskfile-setup.sh's expected list AND update _base/Taskfile.yml.tmpl setup: target, or (b) document this as a known gap and update ROADMAP SC-2 to acknowledge --glow is a binary with README mention only."
  - "W-04 (FLAG-10 partial): 02-03 Task 4 explicitly defers wish subcommand wiring to a comment-only stub in variant_tui/main.go.tmpl ('// --wish enabled: run go run . ssh to launch the SSH server'). The lib overlay at lib/wish/wish.go.tmpl exists and the go.mod entry fires (TestIntegrationScaffold_Wish passes), but no real 'ssh' subcommand is wired into main.go. SC-2 says 'wired into a working example'; the comment-only stub does not satisfy 'working example'. Either: (a) add a real ssh subcommand in variant_tui main.go gated by hasWish (a 15-line addition per research §6.4), or (b) document this gap and update SC-2 to acknowledge wish needs manual wiring."
  - "W-05 (ROADMAP SC-2 imprecision): SC-2 text says 'the library appears in go.mod (under charm.land/<lib>/v2)' but harmonica is on github.com/charmbracelet/harmonica (research §2.1 acknowledged) and glow is a binary (no go.mod entry). The plans handle this correctly but the SC text should be relaxed to 'appears in go.mod or in the scaffold's install instructions'."
  - "W-06 (02-04 wave tag): 02-04's frontmatter declares depends_on: [] which technically allows it to run in Wave 1 in parallel with 02-01+02-02. The prompt's intent is Wave 2 parallel with 02-03 (zero overlap confirmed). The wave: 2 tag in the frontmatter aligns with the prompt's intent; either the orchestrator must honor the wave tag (not just depends_on), or 02-04 should declare depends_on: ['02-02'] (a soft serialization gate, not a real dep) to enforce the prompt's wave structure."
  - "W-07 (TMPL-02 selector ambiguity): REQUIREMENTS.md TMPL-02 says 'User can select cli-cobra-fang template via --template cli-cobra-fang'. The plan uses --cli --cobra --fang flags (not a --template selector). The variant is selected by flags, not by --template. This is consistent with how Phase 1 binds --tui (not --template tui-bubbletea), but the TMPL-02 text in REQUIREMENTS.md is now imprecise. Either: (a) document that --template cli-cobra-fang is an alias for --cli --cobra --fang in resolve.go, or (b) update TMPL-02 to 'User can scaffold a cli-cobra-fang project via --cli --cobra --fang'."
  - "W-08 (Task 7 in 02-04 vs 02-01): Both 02-01 Task 4 and 02-04 Task 7 reference the v1-leaks grep suite. 02-01 refines the deny-list patterns; 02-04 creates the 2 new scripts and wires the Taskfile. If they ran truly in parallel (Wave 1 + Wave 2 simultaneously), 02-04's Task 7 wire of all 3 scripts into Taskfile.yml could collide with 02-01's refinement of check-v1-leaks.sh. Since 02-01 is Wave 1 and 02-04 is Wave 2, this is fine. But the frontmatter ambiguity in W-02 makes the boundary unclear."
  - "W-09 (TestIntegrationScaffold_AllLibs excludes wish+glow): 02-03 Task 8's TestIntegrationScaffold_AllLibs intentionally excludes --wish and --glow from the 'all libs' smoke. The reason is given (wish needs host key, glow is binary), but this means no integration test exercises the wish-or-glow scaffolded project's main.go end-to-end. A simple `go build ./...` test on a --wish scaffold would catch compilation errors even without ssh subcommand wiring; recommend adding TestIntegrationScaffold_Wish_Builds (compile-only assertion) to close the gap."
  - "W-10 (02-03 Task 6 modifies _base/.air.toml.tmpl but plan doesn't list this in 02-01's protections): 02-01's anti-patterns list says 'Do NOT modify _base/.air.toml.tmpl' is NOT explicitly listed (only _base/go.mod.tmpl and _base/main.go.tmpl are). 02-03 Task 6 modifying .air.toml.tmpl is fine for Wave 2 (after 02-01 lands), but the boundary is implicit. Consider adding a 02-01 anti-pattern note: 'Do NOT touch _base/.air.toml.tmpl; include_ext extension is Plan 02-03 work.'"
  - "W-11 (charmbracelet/x sub-libs not addressed): 02-02's funcMap adds hasHuh, hasGlamour, etc. but NOT hasAnsi, hasModifiers, hasRunewidth (the 4 charmbracelet/x forward-compat booleans from PROJECT.md / State.md). The Project struct has these bools. The plans don't claim to wire them but also don't defer them explicitly. If Phase 2 doesn't add overlays for them, the user is silently opted in to dead bool fields. Add to 02-02's out-of-scope: 'charmbracelet/x overlays (ansi, modifiers, runewidth, vt) are deferred to Phase 3+; their bool fields remain on Project but are not wired into funcMap.'"
---

# Phase 2: Plan Quality Check Report

**Checked:** 2026-06-03
**Plans reviewed:** 4 (02-01, 02-02, 02-03, 02-04)
**Waves:** 2 (Wave 1: 02-01 + 02-02; Wave 2: 02-03 [depends 02-02] + 02-04 [zero overlap])
**Status:** PASS_WITH_WARNINGS — all 5 success criteria achievable, all 16 requirements covered, but 11 warnings (mostly minor frontmatter/anti-pattern gaps and 2 partial-delivery items for FLAG-09/10).

## Summary

The 4 plans collectively deliver Phase 2's user-facing value. The split is sensible:

- **02-01** is a clean foundation refactor (pin bump, go 1.25.0 simplification, path-traversal guard, grep refinement). Small, well-bounded, ~5 atomic commits.
- **02-02** ships the external template override (`--template-repo`) via the `templateFS` interface + `CloneTemplateRepo`, and scaffolds the FuncMap helpers (`hasHuh` etc.) the 02-03 overlays need. The 5-line walker refactor is correctly minimal.
- **02-03** is the user-visible content: variant_cli + variant_all templates, 6 lib overlays + viper, _base/go.mod.tmpl + .air.toml.tmpl extensions, and integration tests. This is the bulk of Phase 2's deliverable.
- **02-04** ships the 5 toolchain wrappers (`run`, `build`, `test`, `vet`, `fmt`) with the shared `ToolSpec` + `RunWithFallback` helper, plus 2 new CI grep scripts and wrapper integration tests.

The wave structure is sound for 02-03 → 02-04 (zero file overlap, parallel-eligible). The wave structure is **partially** sound for 02-01 + 02-02 (no file overlap, but an implicit code-level dep: 02-02's `charmPin` switch references new `CharmPins` struct fields added in 02-01).

The 16 requirements are all covered (each plan's frontmatter `requirements:` field names them; the plan tasks deliver the actual implementation, not just the names). The 5 success criteria are achievable from the union of the 4 plans (with 2 partial-delivery gaps: FLAG-09/10).

No critical issues — the plans will not block the phase from shipping if executed as written. The warnings are pre-execution polish opportunities for the planner to address before the executor starts.

## Recommendation

**Status: pass_with_warnings.** The orchestrator can proceed to execution. The planner should consider addressing W-01 (re-declare 02-02 depends_on: ['02-01']), W-03/W-04 (FLAG-09/10 partial delivery — decide whether to fill the gaps or document them in ROADMAP.md), and W-02 (drop `scripts/check-v1-leaks.sh` from 02-04's `files_modified` frontmatter) before execution starts. The remaining warnings (W-05..W-11) are non-blocking and can be addressed inline during execution.

---

_Checked: 2026-06-03T01:25:00Z_
_Checker: gsd-plan-checker_
_Plans reviewed: 02-01, 02-02, 02-03, 02-04_
_Phase 2 status: pass_with_warnings_
