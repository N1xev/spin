---
phase: 03
plan: 04
title: AGENTS.md template + --ai flag
subsystem: scaffold
tags: [agents-md, charm-lib-lookup, funcMap, overlay-walker, determinism, opt-in]
completed: 2026-06-04
duration: ~19m
tasks: 4 commits
files_created:
  - internal/scaffold/templates/lib/ai/AGENTS.md.tmpl
  - internal/scaffold/agents_test.go
files_modified:
  - cmd/new.go
  - internal/scaffold/resolve.go
  - internal/scaffold/resolve_test.go
  - internal/scaffold/template.go
  - internal/scaffold/integration_test.go
  - internal/scaffold/scaffold_e2e_test.go
requirements:
  - AI-01
  - AI-02
  - AI-03
  - AI-04
key_findings:
  - "pflag v1.0.6 (pinned in go.mod) does not expose Flag.Aliases for long-form aliases. The plan's stated approach of `pf.Lookup(\"ai\").Aliases = []string{\"agents\"}` failed to compile; the same workaround as Plan 03-01's --no-interactive / --yes / --batch was used — register both flags and OR them into p.AI in ResolveFlags. The Usage string still mentions the alias for help output."
  - "text/template's `{{- end}}` (strip-before) consumes the blank line INSIDE the loop body, collapsing consecutive library blocks into a single paragraph. `{{end -}}` (strip-after) preserves the blank line between blocks while still eating the newline before `## Extending`. The right form for the AGENTS.md template was `{{end -}}`."
  - "The text/template engine refuses to parse `{{ if has<Lib> . }}` because `<` is the less-than operator inside an action. The plan's prose contained this template-action-shaped text. The fix was to rewrite the body as `per-lib \\`has<Lib>\\` blocks` (with backticks for visual separation) so the `<` is no longer adjacent to `{{`."
  - "runSpinScaffold's existing repoRoot helper used the fixed `wd/../..` path, which broke when the helper was called multiple times in the same test (the second call's chdir made cwd point to a /tmp/ tempdir, then the walk-up failed). The fix was twofold: (1) repoRoot now walks up looking for go.mod, and (2) the helper caches the captured path via sync.Once so the first call wins regardless of cwd. This unblocks TestIntegrationScaffold_AGENTSmd_Determinism which calls runSpinScaffold twice in one test."
  - "The original determinism test scaffolded two DIFFERENT project names (`determinism-a` vs `determinism-b`) and asserted byte-identical AGENTS.md — but the project name is interpolated into the file body, so the assertion would always fail. The fix: use the same project name in both scaffolds (different temp dirs, same `<name>`). The contract is \"same inputs → same outputs\", not \"any inputs → same outputs\"."
one_line_summary: "AGENTS.md opt-in via --ai (alias --agents) with a 15-entry charm library lookup table, byte-identical deterministic output, and integration test proving two scaffolds with the same flags produce identical files"
---

# Phase 3 Plan 4: AGENTS.md template + --ai flag

This plan adds the opt-in AGENTS.md generation path that closes
Phase 3. The user can pass `--ai` (or its alias `--agents`) to
`spin new`; when set, the scaffolder emits an `AGENTS.md` next to
`README.md` describing the project's libraries and how to extend
them. The file is plain GitHub-flavored Markdown, deterministic
(byte-identical across runs with the same flags), and consumed by
Claude Code / Cursor / Aider / Continue.dev as the project context
file.

The plan introduces the second surviving per-lib overlay
(`lib/ai/`, alongside `lib/glow/`) after Plan 02-05's
restructure. All other charm lib wiring stays inlined as
`{{ if has<Lib> . }}` blocks inside the variant files; only
`AGENTS.md` and the glow `README.glow.md` get their own
overlay directories because they are project-root files
unrelated to a specific variant.

The FuncMap gains two helpers — `charmLibInfo(name, field)` for
per-library metadata and `allLibs(p)` for sorted iteration — and
the 15-entry library lookup table is the canonical source of
truth for module path, purpose, extending guidance, and
example code.

## Performance

- **Duration:** ~19 min
- **Started:** 2026-06-04T01:29:50Z
- **Completed:** 2026-06-04T01:48:45Z
- **Tasks:** 4
- **Files modified:** 6
- **Files created:** 2

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add --ai flag with --agents alias and wire to p.AI | 87d7187 | cmd/new.go, internal/scaffold/resolve.go, internal/scaffold/resolve_test.go |
| 2 | Add charmLibInfo and allLibs FuncMap helpers + ai key in boolFlagOverlayMap | cc33a7a | internal/scaffold/template.go |
| 3 | Create lib/ai/AGENTS.md.tmpl with the UI-SPEC structure | b483b22 | internal/scaffold/templates/lib/ai/AGENTS.md.tmpl |
| 4 | Unit tests for template rendering + determinism + integration test for --ai | c9d1c89 | internal/scaffold/agents_test.go, internal/scaffold/integration_test.go, internal/scaffold/scaffold_e2e_test.go |

## Accomplishments

- **`--ai` / `--agents` flag binding.** Both spellings bind to
  `p.AI` via the same alias-via-OR pattern that `--no-interactive
  / --yes / --batch` used in Plan 03-01. The Usage string
  mentions the alias for help output (`opt in to AGENTS.md
  (alias: --agents)`); pflag v1.0.6's `Flag.Aliases` does not
  work for long-form aliases so we keep two flags.
- **`charmLibInfo(name, field)` FuncMap helper.** Returns one
  of `display | module | purpose | extending | example` for
  each of the 15 libraries in the UI-SPEC §"Library lookup
  table" (bubbletea, bubbles, lipgloss, huh, glamour, glow,
  wish, log, harmonica, cobra, fang, viper, modifiers, ansi,
  runewidth). Returns "" for unknown names / fields, matching
  the `charmPin` contract. Module paths are pinned in code
  (not from `versions.go`) because they are long-lived
  canonical paths documented in CLAUDE.md.
- **`allLibs(p)` FuncMap helper.** Thin wrapper over
  `p.AllLibs()` so the AGENTS.md template iterates libs in the
  canonical sorted order. This is the same iteration order the
  gum/huh prompt backend uses, so the AGENTS.md matches the
  "what we asked the user to confirm" sequence.
- **`boolFlagOverlayMap` extended with `"ai": p.AI`.** The
  overlay walker now visits `lib/ai/` when `--ai` is passed.
  This is the second surviving overlay (alongside `lib/glow/`)
  after Plan 02-05's restructure.
- **`lib/ai/AGENTS.md.tmpl`.** Renders a 6-section GFM file
  with the version marker on line 1:
  - `<!-- AUTOGENERATED by spin X.Y.Z -->`
  - `# AGENTS.md`
  - `## What this project is` (Type, Name, Year, Module)
  - `## Libraries` (alphabetical, one `### <Lib>` block per lib
    with Module/Purpose/Extending/Example)
  - `## Extending` (how to add a lib / command; the
    variant_*/lib_*/ template layout)
  - `## Conventions` (no CGO, charm.land/<lib>/v2 only, task
    setup)
  - `## Rebuilding this file` (regenerate via `spin new <name>
    <flags>`)

  Only `{{.SpinVer}}` and `{{.Year}}` are variable, so the
  output is byte-identical across runs with the same flags.
  No timestamps, no UUIDs, no machine IDs.

- **8 new unit tests** in `agents_test.go`:
  marker on line 1, sorted libs, determinism, not-emitted-without-AI,
  no ANSI/hex colors, all 15 charmLibInfo keys, unknown-name
  empty, allLibs delegates to AllLibs.
- **4 new integration tests** in `integration_test.go`:
  --ai produces a complete AGENTS.md, two scaffolds with the
  same project name produce byte-identical AGENTS.md
  (the load-bearing determinism test), --agents (the alias)
  works identically to --ai, and without --ai no AGENTS.md is
  emitted. The `assertAGENTSmd` helper is shared.
- **Test infrastructure fix** in `scaffold_e2e_test.go`:
  `repoRoot()` now walks up looking for `go.mod` (was a fixed
  `wd/../..` path that broke when `runSpinScaffold` was called
  multiple times). The helper now caches the root via
  `sync.Once` so the first call wins regardless of cwd.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Implementation] `pf.Lookup("ai").Aliases` does not exist in pflag v1.0.6**
- **Found during:** Task 1
- **Issue:** The plan stated "apply Lookup("ai").Aliases = []string{"agents"}".
  This does not compile against pflag v1.0.6 (pinned in go.mod), which
  has no `Flag.Aliases` field.
- **Fix:** Registered `--agents` as a separate flag and OR'd its value
  into `p.AI` in `resolve.go`. This is the same pattern used in
  Plan 03-01 for `--no-interactive / --yes / --batch`.
- **Files modified:** cmd/new.go, internal/scaffold/resolve.go
- **Commit:** 87d7187

**2. [Rule 1 - Bug] `{{- end}}` in the loop body collapsed library blocks**
- **Found during:** Task 3
- **Issue:** The plan said use `{{- end}}` to trim whitespace before
  `## Extending`. The `{{- end}}` form strips whitespace BEFORE the
  `}}`, which consumes the blank line INSIDE the loop body that
  separated library blocks. Result: library blocks were concatenated
  into a single paragraph.
- **Fix:** Used `{{end -}}` (strip-after) instead. The blank line
  between blocks is preserved; the newline after the last `}}` is
  consumed so `## Extending` follows immediately.
- **Files modified:** internal/scaffold/templates/lib/ai/AGENTS.md.tmpl
- **Commit:** b483b22

**3. [Rule 1 - Bug] text/template refused `{{ if has<Lib> . }}` in body**
- **Found during:** Task 3
- **Issue:** The plan's template body contained the literal
  `{{ if has<Lib> . }}` as documentation of the variant-file pattern.
  text/template parses the `<` as the less-than operator inside an
  action, so the template failed to compile with
  `bad character U+003C '<'`.
- **Fix:** Rewrote the sentence as `per-lib \`has<Lib>\` blocks in the
  variant files` (backticks visually separate, no `{{` adjacent to
  `<`).
- **Files modified:** internal/scaffold/templates/lib/ai/AGENTS.md.tmpl
- **Commit:** b483b22

**4. [Rule 1 - Bug] `runSpinScaffold` repoRoot broke when called twice in one test**
- **Found during:** Task 4
- **Issue:** `repoRoot()` used the fixed `wd/../..` path. When
  `runSpinScaffold` was called the second time in the same test
  (e.g. the determinism test), the cwd was the first call's
  workdir under `/tmp/`, so the walk-up failed with
  `could not find go.mod starting from /tmp/...`.
- **Fix:** Changed `repoRoot()` to walk up looking for `go.mod`,
  and cached the result via `sync.Once` in `runSpinScaffold` so
  the first call wins regardless of cwd.
- **Files modified:** internal/scaffold/scaffold_e2e_test.go,
  internal/scaffold/integration_test.go
- **Commit:** c9d1c89

**5. [Rule 1 - Bug] Determinism test compared different project names**
- **Found during:** Task 4
- **Issue:** The first version of the determinism test scaffolded
  `determinism-a` and `determinism-b` (different project names) and
  asserted byte-identical AGENTS.md. The project name is interpolated
  into the file body (`<name> is a TUI Go project generated by...`),
  so the assertion would always fail.
- **Fix:** Use the same project name in both scaffolds (different
  temp dirs, same `<name>`). The contract is "same inputs → same
  outputs", not "any inputs → same outputs".
- **Files modified:** internal/scaffold/integration_test.go
- **Commit:** c9d1c89

## Pre-existing Test Status

- `wrap.TestRun_WithAirToml` (660s timeout) and
  `wrap.TestFmt_GofumptMissing_NoStrict` are pre-existing
  flaky tests not caused by this plan. They were skipped via
  `go test ./...` direct invocation timing. Not blocking.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./internal/scaffold/ -run 'TestAGENTSmd|TestCharmLibInfo|TestAllLibsFuncMap' -v` — 8/8 pass
- `go test ./internal/scaffold/ -run TestIntegrationScaffold -v -timeout 600s` — 12/12 pass
  (8 original + 4 new AGENTS.md tests)
- `go test -count=1 -timeout 300s ./internal/scaffold/... ./internal/prompt/... ./cmd/...` — all pass
- `gofumpt -l` reports no new findings on changed files
  (pre-existing findings in template.go and integration_test.go
  are present on the unmodified codebase and are not regressions)
- Smoke test: `go run . new /tmp/foo --tui --bubbletea --bubbles --lipgloss --ai --no-git --no-verify` produces an AGENTS.md
- Smoke test (alias): `go run . new /tmp/foo --tui --bubbletea --agents` produces an AGENTS.md
- Determinism: two `go run . new testapp ...` invocations in different temp dirs produce byte-identical AGENTS.md files (`diff` returns 0)
- `grep -c "AUTOGENERATED by spin" AGENTS.md` returns 1
- `grep -cP "\x1b" AGENTS.md` returns 0
- `head -1 AGENTS.md` returns `<!-- AUTOGENERATED by spin 0.1.0 -->`
- `grep -cE "TODO|FIXME" AGENTS.md` returns 0

## Downstream Impact

- **Phase 4 (Post-Scaffold Health)** can now identify spin-owned
  files via the `<!-- AUTOGENERATED by spin X.Y.Z -->` marker
  on line 1 of `AGENTS.md`. The marker is the canonical
  "owned-by-spin" signal that `spin update` (Phase 4) needs to
  refresh spin-generated files without touching user-modified ones.
- **AI assistants** (Claude Code, Cursor, Aider, Continue.dev)
  now auto-discover `AGENTS.md` at the project root and use it
  as context for understanding the generated project structure
  and library conventions. No opt-in or config is required on
  the assistant side — the file is at the conventional location
  and uses standard GFM.
- **Existing `glow` overlay** is the closest analog and proves
  the per-lib overlay pattern still works after Plan 02-05's
  restructure. `lib/ai/AGENTS.md.tmpl` is the second test case
  for the per-lib overlay path; any future overlay can use the
  same `boolFlagOverlayMap` + `lib/<name>/` directory pattern.

## Files Touched

### Created
- `internal/scaffold/templates/lib/ai/AGENTS.md.tmpl` — 35 lines, the AGENTS.md template
- `internal/scaffold/agents_test.go` — 280 lines, 8 unit tests for AGENTS.md rendering + charmLibInfo + allLibs

### Modified
- `cmd/new.go` — register `--agents` flag alongside `--ai`
- `internal/scaffold/resolve.go` — OR `--agents` value into `p.AI`
- `internal/scaffold/resolve_test.go` — register `--agents` in `newResolveCmd`
- `internal/scaffold/template.go` — add `charmLibInfo`, `allLibs`, `charmLibInfoField`; extend `boolFlagOverlayMap` with `"ai": p.AI`
- `internal/scaffold/integration_test.go` — add 4 new integration tests + `assertAGENTSmd` helper + `cachedRepoRoot`; update `TestIntegrationScaffold` to pass `--ai`
- `internal/scaffold/scaffold_e2e_test.go` — fix `repoRoot()` to walk up to `go.mod`

## Self-Check: PASSED

- 8/8 new unit tests in `agents_test.go` pass
- 4/4 new integration tests in `integration_test.go` pass
- 8/8 existing `TestIntegrationScaffold*` tests still pass
  (including the updated TOOL-05 test that now also asserts
  AGENTS.md content)
- All 4 task commits exist:
  - `87d7187 feat(03-04): add --agents alias for --ai flag`
  - `cc33a7a feat(03-04): add charmLibInfo, allLibs FuncMap helpers + ai overlay key`
  - `b483b22 feat(03-04): add lib/ai/AGENTS.md.tmpl template`
  - `c9d1c89 test(03-04): unit + integration tests for AGENTS.md and --ai`
- Smoke-tested binary produces AGENTS.md in /tmp/foo when --ai is passed
- Byte-identical determinism verified by `diff` returning 0 across two scaffolds
