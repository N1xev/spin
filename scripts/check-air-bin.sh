#!/usr/bin/env bash
# scripts/check-air-bin.sh
#
# Greps a scaffolded (or any) Go project for the deprecated
# .air.toml form `bin = "tmp/main"`. The modern equivalent is
# `build.entrypoint = ["./tmp/main"]` (see RESEARCH §11.1 +
# PITFALL #10). The check is split out from check-v1-leaks.sh so
# that the air-config regression can run independently of the v1
# API grep suite.
#
# Searches recursively for both `.air.toml` (a scaffolded project)
# and `.air.toml.tmpl` (an embedded template source). When run
# against the embedded template tree, only the `.tmpl` will match;
# when run against a scaffolded project, only the un-suffixed
# file will match.
#
# Usage:
#   check-air-bin.sh <project-dir>
#
# Exit 0 if no .air.toml / .air.toml.tmpl exists (nothing to check)
# or the file is clean; exit 1 (with the offending lines printed to
# stderr) if the deprecated pattern is found. Missing directory
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

# Match either the scaffolded form (.air.toml) or the template form
# (.air.toml.tmpl). The template is what the scaffolder would copy
# into a project, so a regression there is just as bad.
mapfile -t AIR_FILES < <(find "$ROOT" -type f \( -name '.air.toml' -o -name '.air.toml.tmpl' \) 2>/dev/null)
if [[ ${#AIR_FILES[@]} -eq 0 ]]; then
  echo "OK: no .air.toml in $ROOT (nothing to check)"
  exit 0
fi

# Deprecated pattern: `bin = "tmp/main"` under any [build] section.
# Anchored on the key to avoid false positives like `cmd_bin = "..."`.
PATTERN='bin\s*=\s*"tmp/main"'

FAIL=0
for air_file in "${AIR_FILES[@]}"; do
  if matches=$(grep -En "$PATTERN" "$air_file" 2>/dev/null); then
    echo "FAIL: deprecated air pattern in $air_file: $PATTERN" >&2
    echo "$matches" >&2
    echo "" >&2
    echo "hint: replace 'bin = \"tmp/main\"' with the modern form:" >&2
    echo "  [build]" >&2
    echo "    entrypoint = [\"./tmp/main\"]" >&2
    FAIL=1
  fi
done

if [[ $FAIL -ne 0 ]]; then
  exit 1
fi

echo "OK: .air.toml files in $ROOT use the modern entrypoint form"
