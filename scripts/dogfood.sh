#!/usr/bin/env bash
# scripts/dogfood.sh
#
# This is the local-runnable form of the GitHub Actions `dogfood` job
# (see .github/workflows/dogfood.yml). If you change the scaffolder
# source, run this before pushing; the CI job runs the same pipeline.
#
# Pipeline (v2.0-template model):
#   1. Build the spin binary at $REPO_ROOT/bin/spin.
#   2. `spin init starter` -- produce a starter template in $WORK.
#      This exercises the init command end-to-end (manifest + _base/
#      placeholder + README). The starter is verified to contain both
#      spin.toml and _base/file.txt.tmpl.
#   3. `spin new spin-fixture --template <starter> --param license=MIT`
#      -- render the starter non-interactively. This exercises the
#      template loader, ResolveForm (the --param non-interactive path),
#      the renderer, the post-hook runner, and the defensive
#      spin.toml deletion (TPL-16). We assert the rendered file
#      contains the project name (proves text/template ran) and that
#      no spin.toml leaked into the dest.
#   4. `spin list --json` against an isolated $XDG_CONFIG_HOME --
#      confirms the pin store + JSON wire format still work after the
#      scaffolder change.
#   5. `go test ./... -count=1` -- the in-tree test suite.
#
# Note: the previous "scaffold a real Go project + go mod tidy +
# go build + go test" pipeline was removed when the embedded
# scaffold tree (internal/scaffold/templates) was deleted in the
# v2.0-template pass. spin no longer ships with an embedded Go
# template; templates are external now. The init + render step
# above exercises the same internal packages
# (internal/template + internal/registry) end-to-end.
#
# Exits 0 on a clean repo. On any step failure, prints the failing
# step name and the last 50 lines of its captured output, then exits 1.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$REPO_ROOT/bin/spin"
# Use a work dir under the repo rather than mktemp -d, because
# `go mod tidy` / `go build` refuse to walk up into /tmp (mode 1777
# is a "system temp root" in Go's view and emits a warning + error).
# $REPO_ROOT is always safe; we also use a unique subdir to avoid
# colliding with concurrent runs.
WORK="$REPO_ROOT/.tmp/dogfood-$$"
rm -rf "$WORK"
mkdir -p "$WORK"
trap 'rm -rf "$WORK"' EXIT

run_step() {
  local name="$1"; shift
  echo "==> $name"
  local logfile="$WORK/${name// /_}.log"
  if ! "$@" > "$logfile" 2>&1; then
    echo "FAIL: $name" >&2
    echo "--- last 50 lines of $logfile ---" >&2
    tail -50 "$logfile" >&2
    echo "" >&2
    echo "work dir preserved at $WORK for inspection" >&2
    # Disable the EXIT trap so the work dir survives for debugging.
    trap - EXIT
    exit 1
  fi
}

# `spin init starter` writes ./starter/ inside the cwd. The starter
# template lives at $WORK/starter/ (no extra subdir).
STARTER_PATH="$WORK/starter"
OUT_DIR="$WORK/out"
mkdir -p "$OUT_DIR"

run_step "Building spin" \
  bash -c "cd '$REPO_ROOT' && go build -o '$BIN' ."

run_step "Scaffolding starter template" \
  bash -c "cd '$WORK' && '$BIN' init starter && test -f '$STARTER_PATH/spin.toml' && test -f '$STARTER_PATH/_base/file.txt.tmpl'"

run_step "Rendering starter end-to-end (non-interactive --param)" \
  bash -c "cd '$WORK' && '$BIN' new spin-fixture --template '$STARTER_PATH' --param license=MIT --dest '$OUT_DIR' && test -f '$OUT_DIR/file.txt' && grep -q 'spin-fixture' '$OUT_DIR/file.txt' && test ! -f '$OUT_DIR/spin.toml'"

run_step "Listing pins (sanity)" \
  bash -c "cd '$WORK' && XDG_CONFIG_HOME='$WORK/xdg' '$BIN' list --json"

echo "==> dogfood passed"
