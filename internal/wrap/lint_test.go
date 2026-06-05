package wrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr redirects os.Stderr to a pipe for the lifetime of
// the test, returning a string of everything written. Used by
// Lint tests to assert on the install-hint line.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })
	return func() string {
		_ = w.Close()
		return readPipe(t, r)
	}
}

// TestLint_NotOnPath_ReturnsError points $PATH at an empty temp
// directory so exec.LookPath("golangci-lint") fails, then asserts
// Lint returns a non-nil error mentioning the tool name and emits
// a one-line install hint to stderr.
func TestLint_NotOnPath_ReturnsError(t *testing.T) {
	emptyBin := t.TempDir()
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", emptyBin); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	read := captureStderr(t)

	err := Lint(nil)
	if err == nil {
		t.Fatal("expected Lint(nil) to return error when golangci-lint is missing")
	}
	if !strings.Contains(err.Error(), "golangci-lint not found") {
		t.Errorf("expected error to mention 'golangci-lint not found'; got: %v", err)
	}

	out := read()
	if !strings.Contains(out, "hint: golangci-lint not found") {
		t.Errorf("expected stderr hint 'golangci-lint not found'; got: %q", out)
	}
}

// TestLint_StderrHint_HasInstallCommand asserts the stderr line
// mentions the literal `go install` module path (not just the
// generic "go install" verb). This is the discoverability contract:
// the user reading the hint must see the exact command to run.
func TestLint_StderrHint_HasInstallCommand(t *testing.T) {
	emptyBin := t.TempDir()
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", emptyBin); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	read := captureStderr(t)
	_ = Lint(nil)
	out := read()

	want := "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
	if !strings.Contains(out, want) {
		t.Errorf("expected stderr hint to contain %q; got: %q", want, out)
	}
}

// TestLint_ArgvPassThrough constructs a fake `golangci-lint`
// shell script that records its argv, prepends it to $PATH, and
// asserts Lint forwards `["version", "--help"]` verbatim. This is
// the only test that exercises the "tool found on PATH" branch.
func TestLint_ArgvPassThrough(t *testing.T) {
	binDir := t.TempDir()
	marker := filepath.Join(binDir, "argv.txt")
	// Write a POSIX shell shim: when called, write "$@" (one per
	// line) to marker, then exit 0. We use $@ inside double quotes
	// so each arg appears on its own line.
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + marker + "\n"
	fake := filepath.Join(binDir, "golangci-lint")
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	// Suppress the real binary's output during the test.
	read := captureStderr(t)
	if err := Lint([]string{"version", "--help"}); err != nil {
		t.Fatalf("Lint with fake on PATH returned error: %v", err)
	}
	_ = read()

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	gotStr := strings.TrimRight(string(got), "\n")
	want := "version\n--help"
	if gotStr != want {
		t.Errorf("argv not forwarded verbatim; want %q; got %q", want, gotStr)
	}
}

// TestLintInstallHintConst_Shape is a cheap regression catcher:
// if someone edits the const to point to a different module
// (e.g. drops the /v2/ segment or the @latest tag), this fails.
func TestLintInstallHintConst_Shape(t *testing.T) {
	if golangciLintInstallHint == "" {
		t.Fatal("golangciLintInstallHint must be non-empty")
	}
	if !strings.HasPrefix(golangciLintInstallHint, "go install ") {
		t.Errorf("golangciLintInstallHint must start with %q; got: %q", "go install ", golangciLintInstallHint)
	}
	if !strings.HasSuffix(golangciLintInstallHint, "@latest") {
		t.Errorf("golangciLintInstallHint must end with %q; got: %q", "@latest", golangciLintInstallHint)
	}
}
