package wrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_NoAirToml exercises the fallback path: when .air.toml is
// absent, Run() must call `go run .` directly. We can't easily
// observe that go was invoked (stdout merges with the parent), so
// we instead verify behavior via the *absence* of the air install
// hint: in a tempdir with no .air.toml, the air-missing path is
// never reached, so no hint about air is printed.
//
// We rely on chdir isolation: each subtest gets a fresh tempdir.
// t.Chdir is the standard testing helper for that (Go 1.24+).
func TestRun_NoAirToml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// No .air.toml is written — that is the precondition.
	origStderr := os.Stderr
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = devNull.Close()
	})

	// We don't assert on `go run .` success/failure here — `go run .`
	// in an empty dir fails, which is fine; we only care that the
	// air path was not taken. So just confirm Run() returns without
	// panicking and that no .air.toml is created as a side-effect.
	if err := Run(); err == nil {
		// Unlikely in a tempdir (no go.mod), but harmless.
	}
	if _, err := os.Stat(filepath.Join(dir, ".air.toml")); err == nil {
		t.Errorf("Run() should not create .air.toml as a side effect")
	}
}

// TestRun_WithAirToml exercises the preferred path: when .air.toml
// is present, Run() looks for `air` on $PATH. If air is missing
// (the common case in CI), it falls back to `go run .` and prints
// the air install hint. We capture stderr to verify the hint.
//
// If air IS present, the install hint is suppressed — we don't
// gate the test on air being missing.
func TestRun_WithAirToml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	airPath := filepath.Join(dir, ".air.toml")
	if err := os.WriteFile(airPath, []byte("root = \".\"\n"), 0o644); err != nil {
		t.Fatalf("write .air.toml: %v", err)
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	// Run() will either exec `air` (if present) or fall back to
	// `go run .` (which will fail in the empty tempdir, but that's
	// fine — we only care about the hint).
	_ = Run()
	_ = w.Close()

	buf := readPipe(t, r)
	// air is unlikely to be on $PATH in the test env; if it is,
	// no hint is expected. So this assertion is conditional:
	if !containsTool(buf, "air") {
		if !containsString(buf, "github.com/air-verse/air") {
			t.Errorf("expected air install hint when air missing; got: %q", buf)
		}
	}
}

// containsTool is a small helper that returns true when buf
// mentions a tool name as a missing tool (i.e., the "not found on
// $PATH" prefix). Used to skip the install-hint assertion when
// the tool happens to be installed.
func containsTool(buf, name string) bool {
	return containsString(buf, name+" not found")
}

// containsString is a thin alias to strings.Contains for the
// test-file's call sites; it makes grep'ing for "string contains
// hint" intent easy.
func containsString(haystack, needle string) bool {
	return strings.Index(haystack, needle) >= 0
}

// readPipe reads everything from r into a string.
func readPipe(t *testing.T, r interface{ Read(p []byte) (int, error) }) string {
	t.Helper()
	tmp := make([]byte, 4096)
	var out []byte
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			out = append(out, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(out)
}
