#!/usr/bin/env bash
# scripts/dogfood.sh
#
# This is the local-runnable form of the GitHub Actions `dogfood` job
# (see .github/workflows/dogfood.yml). If you change the scaffolder
# templates or any cmd/* file, run this before pushing; the CI job
# runs the same pipeline.
#
# Pipeline:
#   1. Build the spin binary at $REPO_ROOT/bin/spin.
#   2. Scaffold a fresh fixture project in a tempdir:
#        spin new spin --cli --cobra --fang
#          --module github.com/example/spin-fixture --quiet
#      The hardcoded --module override avoids colliding with the
#      spin repo's real module path (github.com/example/spin).
#   3. In the fixture, run `go mod tidy`, `CGO_ENABLED=0 go build ./...`,
#      and `go test ./... -count=1` — the same smoke test a new
#      user's project would have to pass on first run.
#   4. Run scripts/check-v1-leaks.sh on the fixture to catch
#      template regressions that compile but introduce a v1
#      charmbracelet import (the "second line of defense" per
#      Taskfile.yml's grep-v1-leaks target).
#
# Exits 0 on a clean repo. On any step failure, prints the failing
# step name and the last 50 lines of its captured output, then exits 1.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$REPO_ROOT/bin/spin"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

run_step() {
  local name="$1"; shift
  echo "==> $name"
  local logfile="$WORK/${name// /_}.log"
  if ! "$@" > "$logfile" 2>&1; then
    echo "FAIL: $name" >&2
    echo "--- last 50 lines of $logfile ---" >&2
    tail -50 "$logfile" >&2
    exit 1
  fi
}

run_step "Building spin" \
  bash -c "cd '$REPO_ROOT' && go build -o '$BIN' ."

run_step "Scaffolding fixture project" \
  bash -c "cd '$WORK' && '$BIN' new spin --cli --cobra --fang --module github.com/example/spin-fixture --quiet"

run_step "Running go mod tidy" \
  bash -c "cd '$WORK/spin' && CGO_ENABLED=0 go mod tidy"

run_step "Running CGO_ENABLED=0 go build" \
  bash -c "cd '$WORK/spin' && CGO_ENABLED=0 go build ./..."

run_step "Running go test" \
  bash -c "cd '$WORK/spin' && go test ./... -count=1"

run_step "Running v1-leak grep" \
  bash -c "bash '$REPO_ROOT/scripts/check-v1-leaks.sh' '$WORK/spin'"

echo "==> dogfood passed"
