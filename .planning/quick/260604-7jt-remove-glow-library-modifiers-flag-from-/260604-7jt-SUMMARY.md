---
phase: 260604-7jt-remove-glow-library-modifiers-flag-from-
plan: 01
type: refactor
wave: 1
subsystem: scaffold
tags: [cli, templates, catalog, refactor, charm]
dependency_graph:
  requires: []
  provides:
    - "--glow and --modifiers flags removed from CLI surface"
    - "Project.Glow + Project.Modifiers fields removed from scaffolder"
    - "lib/glow/ template overlay + readme subcommand removed"
  affects:
    - "cmd/new.go (flag registrations)"
    - "internal/scaffold/{project,template,resolve,versions}.go (Project struct, funcMap, lookup tables, version pins)"
    - "internal/prompt/{catalog,huh}.go (LibCatalog, libBoolMirror, setBoolFieldByName)"
    - "internal/scaffold/templates/{variant_cli,variant_all}/internal/cmd/{root,readme}.go.tmpl"
    - "internal/scaffold/templates/_base/README.md.tmpl"
    - "internal/scaffold/templates/lib/ai/AGENTS.md.tmpl"
tech-stack:
  added: []
  patterns: []
key-files:
  created: []
  modified:
    - cmd/new.go
    - internal/scaffold/project.go
    - internal/scaffold/template.go
    - internal/scaffold/resolve.go
    - internal/scaffold/versions.go
    - internal/scaffold/resolve_test.go
    - internal/scaffold/project_test.go
    - internal/scaffold/agents_test.go
    - internal/scaffold/integration_test.go
    - internal/scaffold/grep_test.go
    - internal/prompt/catalog.go
    - internal/prompt/huh.go
    - internal/prompt/huh_test.go
    - internal/scaffold/templates/_base/README.md.tmpl
    - internal/scaffold/templates/variant_cli/internal/cmd/root.go.tmpl
    - internal/scaffold/templates/variant_all/internal/cmd/root.go.tmpl
    - internal/scaffold/templates/lib/ai/AGENTS.md.tmpl
  deleted:
    - internal/scaffold/templates/variant_cli/internal/cmd/readme.go.tmpl
    - internal/scaffold/templates/variant_all/internal/cmd/readme.go.tmpl
    - internal/scaffold/templates/lib/glow/README.glow.md.tmpl
    - internal/scaffold/templates/lib/glow/ (directory)
decisions: []
metrics:
  duration_seconds: 1520
  completed_date: 2026-06-04
  tasks_completed: 2
  files_modified: 17
  files_deleted: 4
  lines_added: 80
  lines_removed: 315
---

# Phase 260604-7jt Plan 01: Remove --glow + --modifiers flags Summary

Dropped the redundant `--glow` flag (glamour already provides in-process markdown rendering) and the non-existent `--modifiers` flag (`charm.land/x/modifiers` is not a real package). The `readme` subcommand is removed in the same change because its only motivation was the glow shell-out path; glamour-rendered readmes can be re-added as a separate quick task if desired. The `lib/glow/` overlay is gone; `boolFlagOverlayMap` now contains only the surviving `lib/ai/` entry.

## One-liner

Drop --glow and --modifiers flags, Project.Glow/Modifiers fields, lib/glow/ overlay, and the readme subcommand across both CLI variants.

## Tasks

| Task | Name                                                          | Commit   | Files |
|------|---------------------------------------------------------------|----------|-------|
| 1    | Remove --glow/--modifiers from source code                   | 3234b1e  | 11    |
| 2    | Remove readme subcommand templates, lib/glow/, related tests  | 2630bca  | 9     |

## Verification

- `go build ./...` — clean
- `go test -count=1 -timeout 300s ./internal/scaffold/... ./internal/prompt/... ./cmd/...` — all pass (scaffold 70.7s; prompt 0.004s; cmd 6.2s)
- `go run . new --help` — does not mention `--glow` or `--modifiers`
- `go run . new myapp --tui --bubbletea --glamour` — scaffolds cleanly, builds with `CGO_ENABLED=0 go build ./...`, no glow/modifier in go.mod
- `go run . new myapp-cli --cli --wish --viper` — CLI scaffold has only `hello` + `ssh` subcommands, no `readme`

## Deviations from Plan

None — plan executed exactly as written.

## Pre-existing Flaky Tests (not regressions)

- `internal/wrap:TestRun_WithAirToml` — hits 660s timeout; the wrap package was not touched by this plan. The constraints explicitly call this out: "Pre-existing flaky tests (`wrap.TestRun_WithAirToml` 660s timeout, `wrap.TestFmt_GofumptMissing_NoStrict`) are NOT regressions — note in SUMMARY but do not block."

## Self-Check

PASSED — all 2 task commits present in git log; all modified files exist; deleted files are gone.
