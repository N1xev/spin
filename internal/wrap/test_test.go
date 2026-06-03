package wrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTest_GoTestFallback exercises the most common path:
// prism is missing on $PATH, so Test() falls back to
// `go test ./...` and prints the install hint.
//
// We mock the missing-prism condition by pointing $PATH at an
// empty tempdir, then restore the original $PATH after the test.
func TestTest_GoTestFallback(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Empty $PATH → LookPath("prism") always fails.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	_ = Test() // expected to fail (no Go module in tempdir)
	_ = w.Close()

	out := readPipe(t, r)
	if !strings.Contains(out, "prism not found") {
		t.Errorf("expected prism-not-found hint; got: %q", out)
	}
	if !strings.Contains(out, "go.dalton.dog/prism") {
		t.Errorf("expected prism install hint; got: %q", out)
	}
	if !strings.Contains(out, "falling back") {
		t.Errorf("expected 'falling back' message; got: %q", out)
	}
}

// TestTest_PrismPreferred exercises the happy path: prism is on
// $PATH (via a tempdir with a fake `prism` shell script that
// writes its args to a file). Test() must exec the fake prism
// (NOT fall back to go test).
func TestTest_PrismPreferred(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Drop a fake prism on a tempdir-then-prepend-to-PATH.
	binDir := t.TempDir()
	marker := filepath.Join(dir, "prism-args.txt")
	prismScript := "#!/bin/sh\necho \"$@\" > " + marker + "\n"
	prismPath := filepath.Join(binDir, "prism")
	if err := os.WriteFile(prismPath, []byte(prismScript), 0o755); err != nil {
		t.Fatalf("write fake prism: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	// Need a minimal go.mod so the test doesn't crash on something
	// unrelated if prism happens to do anything fancy. prism
	// shouldn't actually exec go test when invoked as a fake
	// (we're just checking argv).
	if err := os.WriteFile("go.mod", []byte("module testfallback\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	_ = Test()

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if !strings.Contains(string(got), "go test ./...") {
		t.Errorf("expected prism to be called with 'go test ./...'; got: %q", got)
	}
}

// TestTest_PrismSkippedForOldGo exercises the version-gate path:
// if the runtime Go is < 1.24, prism is NOT consulted even if it's
// on $PATH. We can't easily lower runtime.Version(), so we test
// the underlying goVersionLessThan helper directly with a table
// that includes "go1.23.0" and "go1.24.0".
//
// (Testing the full Test() gating would require a separate
// Go binary; the helper test is the precise unit boundary.)
func TestGoVersionLessThan(t *testing.T) {
	cases := []struct {
		current, want string
		less          bool
	}{
		{"go1.23.0", "1.24", true},
		{"go1.24.0", "1.24", false},
		{"go1.25.0", "1.24", false},
		{"go1.24.5", "1.24", false},
		{"go1.23.9", "1.24", true},
	}
	for _, c := range cases {
		t.Run(c.current, func(t *testing.T) {
			got := compareVersionStrings(c.current, c.want)
			if got != c.less {
				t.Errorf("compareVersionStrings(%q, %q) = %v, want %v",
					c.current, c.want, got, c.less)
			}
		})
	}
}

// compareVersionStrings is a thin wrapper around goVersionLessThan
// that accepts the full "go1.23.0" form. The production helper
// strips "go" first, so the test exercises both the strip and the
// comparison in one go.
func compareVersionStrings(current, want string) bool {
	return goVersionLessThanWithVersion(current, want)
}
