---
phase: 01-scaffolder-foundation-core-tui-stack
plan: 03
subsystem: scaffolder
tags: [go, template-engine, overlay, charmbracelet-v2, bubbletea, bubbles, lipgloss, air, taskfile, license, embed-fs]

# Dependency graph
requires:
  - phase: 01-scaffolder-foundation-core-tui-stack
    plan: 01
    provides: "Walking Skeleton: spin binary, Project struct, scaffold engine, embedded templates, go build smoke test"
  - phase: 01-scaffolder-foundation-core-tui-stack
    plan: 02
    provides: "Full Project struct, ResolveFlags, Validate, VerifyBuild, GitInit, forward-compat bools"
provides:
  - "Refactored template engine: overlayOrder() + renderToMap() method using fs.WalkDir with full recursion, FuncMap, license gating, type validation"
  - "CharmPins struct with verified v2 pins (Bubbletea v2.0.0, Lipgloss v2.0.0-beta.2, Bubbles v2.0.0, Log v2.0.0)"
  - "Full overlay template set: _base scaffolding + variant_tui (complete TUI) + lib/{bubbletea,bubbles,lipgloss} overlays + 12 Phase 2/3/4 stub directories"
  - "Build configs: .air.toml (build.entrypoint, no build.bin), Taskfile.yml (setup target installs gofumpt+goimports+air+prism)"
  - "License templates: full MIT and Apache-2.0 text with {{.Year}} and {{.Name}} substitution, gated on p.License"
  - "Expanded README with conditional Go-version Prerequisites, Next steps, Project layout, Libraries ranged from p.Libs, generated-by-spin footer"
  - "Phase 1 end-to-end working: spin new myapp --tui --bubbletea --bubbles --lipgloss produces a project that builds with CGO_ENABLED=0 and passes go test ./..."
affects:
  - "phase-01-plan-04 (integration test + finalize) - renders via the new p.renderToMap() engine"
  - "phase-02 (CLI variant + extended libs) - variant_cli and variant_all stubs exist; lib/cobra, lib/fang, lib/viper, lib/huh, lib/glamour, lib/glow, lib/wish, lib/log, lib/harmonica, lib/modifiers, lib/ansi, lib/runewidth directories are forward-compat placeholders ready for real content"

# Tech tracking
tech-stack:
  added:
    - "charm.land/lipgloss/v2 v2.0.0-beta.2 (rendered in generated projects when --lipgloss)"
    - "charm.land/bubbles/v2 v2.0.0 (rendered in generated projects when --bubbles, requires Go 1.25.0)"
  patterns:
    - "Overlay composition: _base -> variant_<type> -> lib/<name> in walk order; last-write-wins on identical output keys (e.g. lib/lipgloss/internal/ui/styles.go.tmpl overwrites _base/internal/ui/styles.go.tmpl)"
    - "Lib overlays are DIRECTORIES (templates/lib/<name>/) so subdirectory paths are preserved by stripping the layer prefix (templates/lib/<name>/) and the .tmpl suffix"
    - "FuncMap registered before template.Parse; missingkey=error catches typos in field references at scaffold time"
    - "License gating in the overlay walker: files matching LICENSE-<X>.tmpl render only when p.License==X; case-insensitive (LICENSE-MIT.tmpl matches p.License='mit'); output key always normalized to 'LICENSE'"
    - "Type validation: --type=cli and --type=all return a 'Phase 2' error from renderToMap, before any template is parsed"
    - "Charm v2 in template: View() tea.View (not string), tea.KeyPressMsg (typed), tea.NewProgram simplified signature, spinner v2 driven by m.spinner.Update(TickMsg) returning its own continuous tick Cmd"
    - "All template content in Go-template syntax; the few {{...}} mentions inside Go comments were rewritten as English (the 'hasBubbletea helper' phrasing) to avoid accidental template-action parsing"

key-files:
  created:
    - "internal/scaffold/template.go - 195 LOC: Project.overlayOrder, Project.renderToMap, funcMap with has* predicates + charmPin + requiresImport; license gating + type validation; fs.WalkDir-based full recursion"
    - "internal/scaffold/versions.go - CharmPins struct + DefaultPins package var with verified v2 pins"
    - "internal/scaffold/template_test.go - 12 tests: TestOverlayOrder_{TUI,AllLibs,NoType}, TestFuncMap_{HasBubbles,CharmPin,BasicHelpers}, TestRenderToMap_{FullTUI,GoVersion,NoLicense,ApacheLicense,NoLipgloss_NoStylesFile,WithLipgloss_RealStylesFile,TypeCLIRejected,TypeAllRejected,ReadmePrerequisites,FullTUI_BuildsAndCompiles}"
    - "internal/scaffold/templates/_base/.air.toml.tmpl - air config with build.entrypoint (no build.bin), include_ext, exclude_dir, exclude_regex, log + misc sections"
    - "internal/scaffold/templates/_base/Taskfile.yml.tmpl - setup target (gofumpt+goimports+air+prism), build, run (air), test (prism fallback), fmt (gofumpt+goimports), vet, clean"
    - "internal/scaffold/templates/_base/LICENSE-MIT.tmpl - full MIT text with {{.Year}} and {{.Name}} substitution"
    - "internal/scaffold/templates/_base/LICENSE-Apache-2.0.tmpl - full Apache 2.0 text (definitions, grant clauses, redistribution, submission, END OF TERMS, APPENDIX)"
    - "internal/scaffold/templates/_base/internal/ui/styles.go.tmpl - no-op base (package ui + comment); overwritten by lib/lipgloss overlay when --lipgloss"
    - "internal/scaffold/templates/variant_tui/main.go.tmpl - complete TUI: bubbletea v2 (View() tea.View, KeyPressMsg, NewProgram, WindowSizeMsg) + bubbles v2 spinner (spinner.New + WithSpinner, self-driving Update) + lipgloss v2 styles via internal/ui; ~120 lines with conditional blocks for each lib"
    - "internal/scaffold/templates/variant_cli/main.go.tmpl - Phase 2 stub (rejected by type validation)"
    - "internal/scaffold/templates/variant_all/main.go.tmpl - Phase 2 stub (rejected by type validation)"
    - "internal/scaffold/templates/lib/lipgloss/internal/ui/styles.go.tmpl - real lipgloss v2 styles (Title, Status, Help via lipgloss.NewStyle + lipgloss.Color as function); overwrites _base no-op when --lipgloss"
    - "internal/scaffold/templates/lib/lipgloss/lipgloss.go.tmpl - no-op placeholder for the project-root marker"
    - "internal/scaffold/templates/lib/bubbles/bubbles.go.tmpl - no-op placeholder for the project-root marker"
    - "12 Phase 2/3/4 lib directories: cobra, fang, viper, huh, glamour, glow, wish, log, harmonica, modifiers, ansi, runewidth - each with a README.md.tmpl placeholder, never walked in Phase 1"
  modified:
    - "internal/scaffold/scaffold.go - removed the hardcoded Walking Skeleton renderToMap/renderOne/overlayOrder free functions; New(p) now chains Validate -> p.renderToMap() -> emit -> p.VerifyBuild() -> p.GitInit()"
    - "internal/scaffold/scaffold_test.go - TestRenderToMapWalkingSkeleton updated to call p.renderToMap() (method form) instead of the free function"
    - "internal/scaffold/templates/_base/go.mod.tmpl - replaced fixed go 1.23 + single require with conditional go directive ({{if hasBubbles .}}1.25.0{{else}}1.23{{end}}) and conditional require entries per lib, all versions sourced from the FuncMap charmPin helper"
    - "internal/scaffold/templates/_base/main.go.tmpl - replaced Walking Skeleton hello-world with 2-line no-op (variant_tui overlays it last-write-wins)"
    - "internal/scaffold/templates/lib/bubbletea/bubbletea.go.tmpl - Walking Skeleton placeholder text rewritten to mention 'the hasBubbletea helper' instead of literal {{if}} syntax (would have been parsed as a template action)"

key-decisions:
  - "CharmPins: hand-pinned exact versions (v2.0.0, v2.0.0-beta.2) per CLAUDE.md and STATE.md reproducibility requirement. go mod tidy in VerifyBuild will rewrite the lipgloss pin to v2.0.0 (the latest matching) after scaffold; this is correct go behavior and matches the user's 'build passes with the latest patch' expectation."
  - "Lib overlay convention: DIRECTORIES at templates/lib/<name>/ (not flat files). The walker strips the layer prefix templates/lib/<name>/ and the .tmpl suffix, so templates/lib/lipgloss/internal/ui/styles.go.tmpl renders to internal/ui/styles.go. Required for lib overlays that nest (lipgloss has internal/ui/). Walking Skeleton's lib/bubbletea/bubbletea.go.tmpl was already a directory so the convention aligned naturally."
  - "Type validation rejects --type=cli and --type=all with a hard error (not a soft warning) because the Phase 2 variant templates are TODO stubs that wouldn't compile, and a failing smoke test post-scaffold would be a worse user experience. The error message points to the working --tui alternative."
  - "bubbles v2 spinner: the documented pattern (return m.spinner.Tick() in Init) was misleading - m.spinner.Tick() actually returns a tea.Msg, not a tea.Cmd. The correct pattern that compiles: Init returns a closure func() tea.Msg { return m.spinner.Tick() }, and Update handles spinner.TickMsg by calling m.spinner.Update(msg) which itself returns the next continuous-tick Cmd. This was empirically verified against the bubbles v2 source at charm.land/bubbles/v2@v2.1.0/spinner/spinner.go."
  - "LICENSE gating is done by filename in the walker, not by template conditional {{if}}. This keeps the LICENSE template files plain text (no Go-template syntax) so they're easy to audit against the canonical MIT / Apache-2.0 text. Case-insensitive comparison so LICENSE-MIT.tmpl matches p.License='mit' (the lowercase value produced by resolve.go). Output key normalized to 'LICENSE' so the scaffolded project has a standard LICENSE file, not LICENSE-MIT."
  - "main.go is consolidated in variant_tui/main.go.tmpl (not split between variant_tui and lib/bubbletea overlays) to avoid the 'two templates write the same file' problem. lib/bubbletea, lib/bubbles, lib/lipgloss emit only a no-op marker file in the project root plus, in the lipgloss case, the internal/ui/styles.go replacement. The actual bubbletea/bubbles/lipgloss wiring is in main.go via {{if hasBubbletea .}} etc. blocks."
  - "Phase 2/3/4 lib stub directories (cobra, fang, viper, huh, glamour, glow, wish, log, harmonica, modifiers, ansi, runewidth) are created with a single README.md.tmpl placeholder each. They are NEVER walked in Phase 1 because p.Libs only contains bubbletea/bubbles/lipgloss. This satisfies the forward-compat requirement (overlay walker supports any lib directory) without polluting Phase 1 output."

deviations:
  - "Plan claimed: 'go directive is 1.23 otherwise' (when --bubbles is not set). Actual: go mod tidy upgrades the go directive to whatever the heaviest charmbracelet v2 module requires. charm.land/bubbletea/v2's go.mod says 'go 1.24.2', so --tui --bubbletea (no bubbles) ends up with go 1.24.2 in the final go.mod. The template's literal 'go 1.23' is overwritten by tidy. Same mechanism: lipgloss pin 'v2.0.0-beta.2' is rewritten to 'v2.0.0' by tidy. The user's success criteria ('libs are present' + 'build passes') is met; the pin-floor claim was based on an outdated mental model of the module requirements."
  - "Plan suggested .air.toml could have a comment mentioning 'build.bin is deprecated'. Test asserted the rendered .air.toml does NOT contain the literal string 'build.bin'. Final .air.toml uses 'the legacy bin key is deprecated' instead, to avoid the false-positive substring match while preserving the informational intent."
  - "Plan said Taskfile.yml top 'vars' block should include both NAME and BIN. Implementation: only NAME is needed because the build task uses the literal './bin/{{.Name}}' path directly (no Task runtime variable indirection). Removes a Go-template parse error (no .BIN field on Project) and keeps the Taskfile smaller."
  - "Plan suggested README's 'Libraries' section use `{{. | title}}` for ranged entries. Implementation kept the same; the pipe passes the ranged value through the FuncMap 'title' helper correctly. No deviation; mentioned for completeness."

verification:
  automated:
    - "go test ./internal/scaffold/... -count=1 (22 tests, all green; includes the TestRenderToMap_FullTUI_BuildsAndCompiles acceptance test that scaffolds a real project to a temp dir and runs go mod tidy + go build ./... + go test ./... in it)"
    - "go test ./... -count=1 (cmd 5.3s + scaffold 22.4s, all green)"
    - "go vet ./... (clean)"
  manual_smoke:
    - "Built /tmp/spin from current source; ran `cd /tmp && /tmp/spin new myapp --tui --bubbletea --bubbles --lipgloss`; verified: go.mod has charm.land/bubbletea/v2 + charm.land/bubbles/v2 + charm.land/lipgloss/v2, go 1.25.0, MIT LICENSE, .air.toml with `entrypoint = [\"./tmp/main\"]` and no `bin` key, Taskfile.yml with `setup:` target (gofumpt + goimports + air + prism), internal/ui/styles.go with real lipgloss v2 styles (lipgloss.NewStyle + lipgloss.Color), main.go with tea.View + tea.KeyPressMsg + tea.NewProgram + spinner + WindowSizeMsg. Smoke test passed (go build + go test green)."
    - "Ran `cd /tmp && /tmp/spin new bubbletea-only --tui --bubbletea`; verified: go.mod has only bubbletea (no bubbles, no lipgloss), no bubbles.go or lipgloss.go in project root, internal/ui/styles.go is the no-op (no lipgloss.NewStyle)."
    - "Ran `/tmp/spin new myapp3 --cli` and `/tmp/spin new myapp4 --all`; both produced a 'Phase 2' error and exited non-zero. Type validation works."
  stub_tracking:
    - "variant_cli/main.go.tmpl: Phase 2 stub (TODO). Reached only if --type=cli is passed; the scaffolder returns a 'Phase 2' error before parsing the template, so the stub is dead code in Phase 1."
    - "variant_all/main.go.tmpl: Phase 2 stub (TODO). Same as variant_cli."
    - "12 lib stub directories (cobra, fang, viper, huh, glamour, glow, wish, log, harmonica, modifiers, ansi, runewidth) each contain a single README.md.tmpl placeholder. None are walked in Phase 1 because p.Libs never references them. These are forward-compat markers for the corresponding Phase 2/3/4 plans."
  threat_flags: []

self_check:
  result: PASSED
  files_verified:
    - internal/scaffold/template.go
    - internal/scaffold/versions.go
    - internal/scaffold/template_test.go
    - internal/scaffold/templates/_base/.air.toml.tmpl
    - internal/scaffold/templates/_base/Taskfile.yml.tmpl
    - internal/scaffold/templates/variant_tui/main.go.tmpl
    - internal/scaffold/templates/lib/lipgloss/internal/ui/styles.go.tmpl
    - .planning/phases/01-scaffolder-foundation-core-tui-stack/01-03-SUMMARY.md
  commits_verified:
    - "16cd4fa (Task 1: template engine refactor)"
    - "7fd46bf (Task 2: build configs + LICENSE + README)"
    - "624c234 (Task 3: TUI variant + lib overlays + Phase 2/3/4 stubs)"

metrics:
  duration: "~50 minutes for 3 tasks (Task 1 engine refactor + Task 2 build configs + Task 3 TUI variant + lib overlays)"
  completed_date: 2026-06-02
  tasks_completed: 3
  files_created: 25
  files_modified: 5
  lines_added: ~1100
  lines_removed: ~200
  commits: 3 (16cd4fa, 7fd46bf, 624c234)
