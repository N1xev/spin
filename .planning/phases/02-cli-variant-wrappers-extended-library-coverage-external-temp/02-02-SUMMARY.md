---
phase: 02
plan: 02
title: External template override + FuncMap scaffolding
completed: 2026-06-02T22:55:00Z
tasks: 7
files_created:
  - internal/scaffold/fs.go
  - internal/scaffold/repo.go
  - internal/scaffold/repo_test.go
  - internal/scaffold/fs_test.go
files_modified:
  - internal/scaffold/project.go
  - internal/scaffold/template.go
  - internal/scaffold/validate.go
  - cmd/new.go
  - internal/scaffold/resolve.go
  - internal/scaffold/versions.go
  - internal/scaffold/resolve_test.go
  - internal/scaffold/validate_test.go
requirements:
  - TMPL-03
  - TMPL-05
  - FLAG-07
  - FLAG-08
  - FLAG-09
  - FLAG-10
  - FLAG-11
  - FLAG-12
  - FLAG-15
test_count: 57 → 65 (+8)
key_findings:
  - templateFS interface: both embed.FS and os.DirFS satisfy; currentFS() switches
  - CloneTemplateRepo: depth-1 git clone with GIT_TERMINAL_PROMPT=0 to os.MkdirTemp
  - 7 new FuncMap helpers: hasHuh, hasGlamour, hasGlow, hasWish, hasLog, hasHarmonica, hasViper
  - charmPin extension: added 3 more cases (viper, harmonica, glow); 4 already in 02-01
  - URL gate accepts https/http/git/file/git@
  - Walker root prefix branches on ExternalDir (templates/ for embed, "" for external)
  - Missing-layer check matches both embed and os.DirFS error wordings
one_line_summary: |
  02-02 complete (7 commits, fd8f6c2..d76baa4). External template override
  shipped. SUMMARY.md written post-hoc by orchestrator after the executor
  reported skipping the docs file due to a system rule.
deviations_from_plan:
  - 02-01's plan said "Plan 02-02's charmPin switch will reference the new CharmPins fields"
    but 02-01 already extended the switch in commit c9832e2 because tests required it.
    02-02 added the remaining 3 cases (viper, harmonica, glow) and the Viper field.
  - 02-02 also modified internal/scaffold/versions.go (added Viper field) and
    internal/scaffold/resolve_test.go / validate_test.go (not in plan frontmatter
    but necessary for the new flag tests).
  - file:// URLs accepted (plan §5 said "reject" but plan §verification used file://).
    Reconciled by accepting file:// in IsValidTemplateRepo.
  - Walker root prefix and missing-file wording fixes bundled into Task 7 commit
    (found during smoke test 2 — plan-oversight).
