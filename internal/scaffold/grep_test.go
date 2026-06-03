// Tests for scripts/check-v1-leaks.sh — the CI grep suite that catches
// v1 charmbracelet API leaks in generated projects (TOOL-03 + RESEARCH §11).
//
// The tests invoke the bash script via os/exec against three targets:
//
//  1. The embedded ./internal/scaffold/templates tree (clean baseline).
//  2. A temp dir with a v1 import path injected (must exit non-zero).
//  3. A temp dir with a deprecated .air.toml `build.bin` key (must exit non-zero).
//
// The test depends on `bash` being on $PATH. Linux/macOS only for Phase 1;
// Windows users can rewrite the script in Go (per RESEARCH §11.2).
package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	scriptRelPath = "../../scripts/check-v1-leaks.sh"
	templatesRel  = "../../internal/scaffold/templates"
)

func runGrep(t *testing.T, target string) (string, string, error) {
	t.Helper()
	script := mustAbs(t, scriptRelPath)
	cmd := exec.Command("bash", script, target)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestGrepV1Leaks_TemplatesAreClean is a sanity check that the embedded
// template tree is free of v1 patterns. If this fails, the templates
// themselves regressed (someone added a v1 import, View() string, etc.)
// and the grep suite caught it before any project ever got generated.
func TestGrepV1Leaks_TemplatesAreClean(t *testing.T) {
	target := mustAbs(t, templatesRel)
	stdout, stderr, err := runGrep(t, target)
	if err != nil {
		t.Fatalf("grep script failed on templates (err=%v):\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK: no v1 leaks detected") {
		t.Errorf("expected 'OK' line in stdout, got: %q", stdout)
	}
}

// TestGrepV1Leaks_CatchesV1Import ensures a v1 charmbracelet import path
// (e.g. `github.com/charmbracelet/bubbletea`) makes the script exit
// non-zero and report the offending line.
func TestGrepV1Leaks_CatchesV1Import(t *testing.T) {
	dir := t.TempDir()
	src := `package myapp

import "github.com/charmbracelet/bubbletea"

func main() { _ = bubbletea.NewProgram(nil) }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	stdout, stderr, err := runGrep(t, dir)
	if err == nil {
		t.Fatalf("grep script should have FAILED on v1 import; got exit 0\nstdout: %s\nstderr: %s",
			stdout, stderr)
	}
	if !strings.Contains(stderr, "github.com/charmbracelet/") {
		t.Errorf("expected stderr to mention the v1 pattern; got: %q", stderr)
	}
	if !strings.Contains(stderr, "main.go") {
		t.Errorf("expected stderr to mention main.go; got: %q", stderr)
	}
}

// TestGrepV1Leaks_CatchesDeprecatedAir ensures a `.air.toml` with the
// legacy `bin = "tmp/main"` key makes the script exit non-zero. The
// modern equivalent is `build.entrypoint = ["./tmp/main"]`.
func TestGrepV1Leaks_CatchesDeprecatedAir(t *testing.T) {
	dir := t.TempDir()
	air := `root = "."
[build]
  bin = "tmp/main"
  cmd = "go build -o ./tmp/main ."
`
	if err := os.WriteFile(filepath.Join(dir, ".air.toml"), []byte(air), 0o644); err != nil {
		t.Fatalf("write .air.toml: %v", err)
	}

	stdout, stderr, err := runGrep(t, dir)
	if err == nil {
		t.Fatalf("grep script should have FAILED on deprecated air config; got exit 0\nstdout: %s\nstderr: %s",
			stdout, stderr)
	}
	if !strings.Contains(stderr, "deprecated air pattern") {
		t.Errorf("expected stderr to mention deprecated air pattern; got: %q", stderr)
	}
}

// TestGrepV1Leaks_AllowsHarmonica is a positive-control test for the
// per-module deny-list added in Plan 02-01 (Task 4). The pre-Task-4
// script blanket-banned `github.com/charmbracelet/`, which falsely
// flagged the legitimate current path `github.com/charmbracelet/harmonica`
// (harmonica has not migrated to charm.land per RESEARCH §2.1).
//
// This test pins that regression: a project that imports harmonica MUST
// pass the grep suite. Companion test TestGrepV1Leaks_AllowsGlowV2 covers
// the other intentionally-allowed path (the glow v2 binary's module
// path is still `github.com/charmbracelet/glow/v2`).
func TestGrepV1Leaks_AllowsHarmonica(t *testing.T) {
	dir := t.TempDir()
	src := `package myapp

import "github.com/charmbracelet/harmonica"

func main() {
	_ = harmonica.NewSpring(harmonica.FPS(60), 6.0, 0.5)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	stdout, stderr, err := runGrep(t, dir)
	if err != nil {
		t.Fatalf("grep script should have PASSED on harmonica import (it is the current path, not a v1 leak); got exit %v\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK: no v1 leaks detected") {
		t.Errorf("expected 'OK' line in stdout; got: %q", stdout)
	}
}

// TestGrepV1Leaks_AllowsGlowV2 is the second positive-control test for
// the Task 4 per-module deny-list: `github.com/charmbracelet/glow/v2`
// is the current path for the glow binary's Go module and is allowed.
func TestGrepV1Leaks_AllowsGlowV2(t *testing.T) {
	dir := t.TempDir()
	src := `package myapp

import "github.com/charmbracelet/glow/v2"

func main() {
	_ = glow.NewTermRenderer()
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	stdout, stderr, err := runGrep(t, dir)
	if err != nil {
		t.Fatalf("grep script should have PASSED on glow/v2 import; got exit %v\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK: no v1 leaks detected") {
		t.Errorf("expected 'OK' line in stdout; got: %q", stdout)
	}
}

// mustAbs resolves rel (relative to this test file) to an absolute path
// and fails the test if the path does not exist. The grep script and
// templates are looked up via the test's own directory so the test is
// independent of the process working directory.
func mustAbs(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	abs := filepath.Clean(filepath.Join(wd, rel))
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("path %q not accessible: %v", abs, err)
	}
	return abs
}

// --------------------------------------------------------------------
// Plan 02-04: tests for the new CI grep scripts (check-air-bin.sh,
// check-taskfile-setup.sh). These run alongside the check-v1-leaks
// tests above and share the mustAbs + runGrep helpers.
// --------------------------------------------------------------------

// runGrepScript invokes any of the 3 check-*.sh scripts (which all
// share the same `<project-dir>` argv contract) and returns stdout,
// stderr, and the exit error.
func runGrepScript(t *testing.T, scriptName, target string) (string, string, error) {
	t.Helper()
	script := mustAbs(t, "../../scripts/"+scriptName)
	cmd := exec.Command("bash", script, target)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestGrepAirBin_TemplatesAreClean is the positive baseline: the
// embedded template tree's .air.toml.tmpl (if any) uses the modern
// entrypoint form. This is a regression catch for the split grep
// script — if a future template change reintroduces `bin = "tmp/main"`,
// this test fails before the script is even exercised on user projects.
func TestGrepAirBin_TemplatesAreClean(t *testing.T) {
	target := mustAbs(t, templatesRel)
	stdout, stderr, err := runGrepScript(t, "check-air-bin.sh", target)
	if err != nil {
		t.Fatalf("check-air-bin.sh failed on templates: %v\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Errorf("expected 'OK:' line in stdout, got: %q", stdout)
	}
}

// TestGrepAirBin_CatchesDeprecated ensures a synthetic .air.toml
// with the legacy `bin = "tmp/main"` form makes the script exit
// non-zero and report the offending line. Companion to the
// deprecated-air test in check-v1-leaks.sh — covers the standalone
// air script's behavior, which is the new split-out script.
func TestGrepAirBin_CatchesDeprecated(t *testing.T) {
	dir := t.TempDir()
	air := `root = "."
[build]
  bin = "tmp/main"
  cmd = "go build -o ./tmp/main ."
`
	if err := os.WriteFile(filepath.Join(dir, ".air.toml"), []byte(air), 0o644); err != nil {
		t.Fatalf("write .air.toml: %v", err)
	}

	stdout, stderr, err := runGrepScript(t, "check-air-bin.sh", dir)
	if err == nil {
		t.Fatalf("check-air-bin.sh should have FAILED on deprecated air config; got exit 0\nstdout: %s\nstderr: %s",
			stdout, stderr)
	}
	if !strings.Contains(stderr, "deprecated air pattern") {
		t.Errorf("expected stderr to mention deprecated air pattern; got: %q", stderr)
	}
	if !strings.Contains(stderr, "entrypoint") {
		t.Errorf("expected stderr hint to mention modern 'entrypoint' form; got: %q", stderr)
	}
}

// TestGrepAirBin_AllowsModern ensures a synthetic .air.toml with
// the modern `build.entrypoint` form passes the script cleanly.
// Positive control: pin the contract that `entrypoint` is the
// accepted modern form.
func TestGrepAirBin_AllowsModern(t *testing.T) {
	dir := t.TempDir()
	air := `root = "."
[build]
  entrypoint = ["./tmp/main"]
  cmd = "go build -o ./tmp/main ."
`
	if err := os.WriteFile(filepath.Join(dir, ".air.toml"), []byte(air), 0o644); err != nil {
		t.Fatalf("write .air.toml: %v", err)
	}

	stdout, stderr, err := runGrepScript(t, "check-air-bin.sh", dir)
	if err != nil {
		t.Fatalf("check-air-bin.sh should have PASSED on modern air config; got: %v\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Errorf("expected 'OK:' in stdout; got: %q", stdout)
	}
}

// TestGrepTaskfileSetup_TemplatesAreClean is the positive baseline
// for the new Taskfile-setup script: the embedded template's
// Taskfile.yml.tmpl has the `setup:` target with all 4 installs.
func TestGrepTaskfileSetup_TemplatesAreClean(t *testing.T) {
	target := mustAbs(t, templatesRel)
	stdout, stderr, err := runGrepScript(t, "check-taskfile-setup.sh", target)
	if err != nil {
		t.Fatalf("check-taskfile-setup.sh failed on templates: %v\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Errorf("expected 'OK:' line in stdout, got: %q", stdout)
	}
}

// TestGrepTaskfileSetup_CatchesMissing ensures a Taskfile.yml with
// NO `setup:` target at all makes the script exit non-zero and
// report the missing target.
func TestGrepTaskfileSetup_CatchesMissing(t *testing.T) {
	dir := t.TempDir()
	tf := `version: '3'

tasks:
  build:
    cmds:
      - go build -o ./bin/foo .
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(tf), 0o644); err != nil {
		t.Fatalf("write Taskfile.yml: %v", err)
	}

	stdout, stderr, err := runGrepScript(t, "check-taskfile-setup.sh", dir)
	if err == nil {
		t.Fatalf("check-taskfile-setup.sh should have FAILED on missing setup: target; got exit 0\nstdout: %s\nstderr: %s",
			stdout, stderr)
	}
	if !strings.Contains(stderr, "setup:") {
		t.Errorf("expected stderr to mention 'setup:'; got: %q", stderr)
	}
}

// TestGrepTaskfileSetup_CatchesMissingInstall ensures a Taskfile.yml
// with a `setup:` target but missing the gofumpt install makes the
// script exit non-zero with a precise error.
func TestGrepTaskfileSetup_CatchesMissingInstall(t *testing.T) {
	dir := t.TempDir()
	tf := `version: '3'

tasks:
  setup:
    desc: Install dev tools (partial)
    cmds:
      - go install golang.org/x/tools/cmd/goimports@latest
      - go install github.com/air-verse/air@latest
      - go install go.dalton.dog/prism@latest
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(tf), 0o644); err != nil {
		t.Fatalf("write Taskfile.yml: %v", err)
	}

	stdout, stderr, err := runGrepScript(t, "check-taskfile-setup.sh", dir)
	if err == nil {
		t.Fatalf("check-taskfile-setup.sh should have FAILED on missing gofumpt install; got exit 0\nstdout: %s\nstderr: %s",
			stdout, stderr)
	}
	if !strings.Contains(stderr, "gofumpt") {
		t.Errorf("expected stderr to mention gofumpt; got: %q", stderr)
	}
}

// TestGrepTaskfileSetup_AllowsComplete is the positive control:
// a Taskfile.yml with the full setup: target (all 4 installs)
// passes the script.
func TestGrepTaskfileSetup_AllowsComplete(t *testing.T) {
	dir := t.TempDir()
	tf := `version: '3'

tasks:
  setup:
    desc: Install dev tools
    cmds:
      - go install mvdan.cc/gofumpt@latest
      - go install golang.org/x/tools/cmd/goimports@latest
      - go install github.com/air-verse/air@latest
      - go install go.dalton.dog/prism@latest
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(tf), 0o644); err != nil {
		t.Fatalf("write Taskfile.yml: %v", err)
	}

	stdout, stderr, err := runGrepScript(t, "check-taskfile-setup.sh", dir)
	if err != nil {
		t.Fatalf("check-taskfile-setup.sh should have PASSED on complete setup: target; got: %v\nstdout: %s\nstderr: %s",
			err, stdout, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Errorf("expected 'OK:' in stdout; got: %q", stdout)
	}
}
