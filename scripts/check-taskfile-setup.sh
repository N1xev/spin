#!/usr/bin/env bash
# scripts/check-taskfile-setup.sh
#
# Greps a scaffolded (or any) Go project for the Taskfile.yml
# `setup:` target that installs the four dev tools (gofumpt,
# goimports, air, prism). The setup target is what makes
# `task setup` a one-shot onboarding step for new contributors
# (RESEARCH §4.7); if any of the four installs is missing, the
# generated project loses its "perfect first run" promise.
#
# Searches recursively for both `Taskfile.yml` (a scaffolded
# project) and `Taskfile.yml.tmpl` (an embedded template source).
#
# Usage:
#   check-taskfile-setup.sh <project-dir>
#
# Exit 0 if every Taskfile found has a `setup:` target that
# installs all four tools; exit 1 (with the offending lines
# printed to stderr) if any are missing. Missing directory
# exits 2.
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

mapfile -t TASKFILES < <(find "$ROOT" -type f \( -name 'Taskfile.yml' -o -name 'Taskfile.yml.tmpl' \) 2>/dev/null)
if [[ ${#TASKFILES[@]} -eq 0 ]]; then
  echo "FAIL: no Taskfile.yml or Taskfile.yml.tmpl in $ROOT" >&2
  exit 1
fi

# The four `go install` lines the setup: target must contain.
# Anchored on the module path to avoid false positives in comments
# or other targets.
declare -a REQUIRED_INSTALLS=(
  'go install mvdan\.cc/gofumpt@latest'
  'go install golang\.org/x/tools/cmd/goimports@latest'
  'go install github\.com/air-verse/air@latest'
  'go install go\.dalton\.dog/prism@latest'
)

FAIL=0
for tf in "${TASKFILES[@]}"; do
  # Check the top-level `setup:` target exists at all in this file.
  if ! grep -Eq '^[[:space:]]*setup:' "$tf"; then
    echo "FAIL: $tf missing top-level 'setup:' target" >&2
    FAIL=1
    continue
  fi
  for pat in "${REQUIRED_INSTALLS[@]}"; do
    if ! grep -Eq "$pat" "$tf"; then
      echo "FAIL: $tf setup: target missing install line matching: $pat" >&2
      FAIL=1
    fi
  done
done

if [[ $FAIL -ne 0 ]]; then
  echo "" >&2
  echo "hint: the setup: target should look like:" >&2
  echo "  setup:" >&2
  echo "    desc: Install dev tools (gofumpt, goimports, air, prism)" >&2
  echo "    cmds:" >&2
  echo "      - go install mvdan.cc/gofumpt@latest" >&2
  echo "      - go install golang.org/x/tools/cmd/goimports@latest" >&2
  echo "      - go install github.com/air-verse/air@latest" >&2
  echo "      - go install go.dalton.dog/prism@latest" >&2
  exit 1
fi

echo "OK: Taskfile.yml files in $ROOT have a setup: target with all 4 installs"
