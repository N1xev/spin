#!/usr/bin/env bash
# scripts/check-v1-leaks.sh
#
# Greps a scaffolded (or any) Go project for v1 charmbracelet API leaks.
# This is the second line of defense after the post-scaffold `go build`
# smoke test — it catches patterns that compile but are semantically wrong
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
declare -a GO_PATTERNS=(
  'github\.com/charmbracelet/'
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
