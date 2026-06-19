#!/usr/bin/env bash
# scripts/check-v1-leaks.sh
#
# Greps a scaffolded (or any) Go project for v1 charmbracelet API leaks.
# This is the second line of defense after the post-scaffold `go build`
# smoke test -- it catches patterns that compile but are semantically wrong
# (e.g., hard-coded v1-looking code, deprecated `.air.toml` `build.bin`).
#
# Per RESEARCH §11, the patterns cover 22 v1 -> v2 forbidden APIs in Go
# source plus 1 deprecated air config key. The set is deliberately wider
# than what currently renders into a project: future template regressions
# get caught even if no current scaffold would emit them.
#
# Usage:
#   check-v1-leaks.sh <project-dir>
#
# Exit 0 if no v1 pattern is found; exit 1 (with the offending lines
# printed to stderr) otherwise. Missing directory exits 2.
#
# Allow-list:
#   This script has no --allow flag by design. v1 leaks are
#   always-avoidable in v2 code, so a maintainer who needs to
#   suppress a match should either fix the source or update this
#   script's deny-list with a comment explaining the exception.
#   Adding a per-run --allow would create "I disabled the check
#   in my local run" patterns that are easy to copy into CI.
set -euo pipefail

ROOT="${1:-}"
if [[ -z "$ROOT" ]]; then
  echo "usage: $0 <project-dir>" >&2
  exit 2
fi
if [[ ! -d "$ROOT" ]]; then
  echo "error: directory '$ROOT' does not exist" >&2
  exit 2
fi

# Go file patterns. Sources: RESEARCH §11.1 + PITFALLS #1, #2, #3, #4.
#
# V1_LEAK_PATTERNS is a per-module deny-list of the charmbracelet libraries
# that MIGRATED to charm.land/<lib>/v2. Using the github.com path for any
# of these is a v1 leak and must be caught.
#
# Deliberately EXCLUDED from this list (and therefore allowed):
#   - github.com/charmbracelet/harmonica -- still on github.com; v0.2.0
#     pre-dates the migration. See 02-RESEARCH.md §2.1.
#   - github.com/charmbracelet/glow/v2 -- the v2 line of the glow binary
#     lives on github.com; charm.land/glow/v2 does not exist.
#
# The closing-quote anchor (`"`) on each pattern means a template like
# `import "github.com/charmbracelet/harmonica"` is NOT matched (it ends
# in `harmonica"`, not `bubbletea"`), but a bare
# `import "github.com/charmbracelet/bubbletea"` IS matched.
declare -a V1_LEAK_PATTERNS=(
  'github\.com/charmbracelet/bubbletea"'
  'github\.com/charmbracelet/bubbletea/v2"'
  'github\.com/charmbracelet/lipgloss"'
  'github\.com/charmbracelet/lipgloss/v2"'
  'github\.com/charmbracelet/bubbles"'
  'github\.com/charmbracelet/bubbles/v2"'
  'github\.com/charmbracelet/huh"'
  'github\.com/charmbracelet/huh/v2"'
  'github\.com/charmbracelet/glamour"'
  'github\.com/charmbracelet/glamour/v2"'
  'github\.com/charmbracelet/wish"'
  'github\.com/charmbracelet/wish/v2"'
  'github\.com/charmbracelet/log"'
  'github\.com/charmbracelet/log/v2"'
  'github\.com/charmbracelet/fang"'
  'github\.com/charmbracelet/fang/v2"'
)

# v2 API patterns (sources: RESEARCH §11.1 + PITFALLS #1, #2, #3, #4).
# These are v1-style API calls that are removed or renamed in v2; the
# grep matches the function call shape regardless of import path.
declare -a GO_PATTERNS=(
  "${V1_LEAK_PATTERNS[@]}"
  'View\(\) string'
  'tea\.WithAltScreen'
  'tea\.WithMouseCellMotion'
  'tea\.EnterAltScreen'
  'tea\.HideCursor'
  'tea\.ExitAltScreen'
  'lipgloss\.NewRenderer'
  'lipgloss\.DefaultRenderer'
  'lipgloss\.SetDefaultRenderer'
  'lipgloss\.AdaptiveColor\{'
  'lipgloss\.ColorProfile\('
  'lipgloss\.HasDarkBackground\(\)'
  'tea\.KeyCtrlC'
  'tea\.MouseButtonLeft'
  'tea\.MouseButtonRight'
  'tea\.MouseButtonMiddle'
  'msg\.Type'
  'msg\.Runes'
  'msg\.Alt'
  'msg\.X'
  'msg\.Y'
)

# .air.toml: forbid the legacy `build.bin = "tmp/main"` form (RESEARCH §11.1,
# PITFALL #10). The modern equivalent is `build.entrypoint = ["./tmp/main"]`.
declare -a AIR_PATTERNS=(
  'bin\s*=\s*"tmp/main"'
)

FAIL=0
for pat in "${GO_PATTERNS[@]}"; do
  if matches=$(grep -rEn --include='*.go' --include='*.tmpl' "$pat" "$ROOT" 2>/dev/null); then
    echo "FAIL: v1 pattern matched: $pat" >&2
    echo "$matches" >&2
    echo "" >&2
    FAIL=1
  fi
done

AIR_FILE="$ROOT/.air.toml"
if [[ -f "$AIR_FILE" ]]; then
  for pat in "${AIR_PATTERNS[@]}"; do
    if matches=$(grep -En "$pat" "$AIR_FILE" 2>/dev/null); then
      echo "FAIL: deprecated air pattern: $pat" >&2
      echo "$matches" >&2
      echo "" >&2
      FAIL=1
    fi
  done
fi

if [[ $FAIL -ne 0 ]]; then
  exit 1
fi
echo "OK: no v1 leaks detected in $ROOT"
