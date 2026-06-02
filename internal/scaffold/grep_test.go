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
