---
phase: 02
plan: 03
title: Ship CLI variant + --all combo + 7 lib overlays + go.mod gates
completed: 2026-06-03T00:20:00Z
tasks: 9
files_created:
  - internal/scaffold/templates/variant_cli/main.go.tmpl
  - internal/scaffold/templates/variant_all/main.go.tmpl
  - internal/scaffold/templates/lib/huh/huh.go.tmpl
  - internal/scaffold/templates/lib/glamour/glamour.go.tmpl
  - internal/scaffold/templates/lib/glow/README.glow.md.tmpl
  - internal/scaffold/templates/lib/wish/wish.go.tmpl
  - internal/scaffold/templates/lib/log/log.go.tmpl
  - internal/scaffold/templates/lib/harmonica/harmonica.go.tmpl
  - internal/scaffold/templates/lib/viper/internal/config/config.go.tmpl
  - internal/scaffold/integration_v2_test.go
files_modified:
  - internal/scaffold/templates/_base/go.mod.tmpl
  - internal/scaffold/templates/_base/README.md.tmpl
  - internal/scaffold/templates/_base/.air.toml.tmpl
  - internal/scaffold/templates/variant_tui/main.go.tmpl
  - internal/scaffold/templates/variant_cli/main.go.tmpl (was Phase 1 stub, replaced)
  - internal/scaffold/templates/variant_all/main.go.tmpl (was Phase 1 stub, replaced)
  - internal/scaffold/template.go
requirements:
  - FLAG-07
  - FLAG-08
  - FLAG-09
  - FLAG-10
  - FLAG-11
  - FLAG-12
  - FLAG-15
  - TMPL-02
  - TMPL-05
  - TMPL-06
test_count: 65 → 78 (+13)
key_findings:
  - All 3 variants (--cli, --tui, --all) build clean with CGO_ENABLED=0
  - 6 lib overlays compile + test clean when their flag is passed
  - --wish adds an `ssh` subcommand (not a keybinding)
  - harmonica stays on github.com path; v1-leak grep per-module allow-list works
  - glow is a binary (no Go require) -- README mentions `glow README.md`
  - fang-styled help on /tmp/mycli --help verified
  - 141 sub-tests pass
one_line_summary: |
  02-03 complete: 3 variants work, 7 lib overlays shipped, 9 atomic commits.
  Tests 65→78, all smoke tests green. SUMMARY.md written post-quota-reset after
  the original 02-03 executor hit Token Plan Plus 5-hour cap mid-execution;
  all 9 plan commits landed before the cap; verification re-run by orchestrator.
