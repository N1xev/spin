package wrap

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunWithFallback_PreferredExists exercises the happy path:
// LookPath finds the preferred tool, and runTool is invoked on it
// (not the fallback). We use `/bin/echo` as the preferred tool —
// it's always present on Linux/macOS test environments — and
// configure the fallback as a fake command that would always fail
// if it were accidentally called.
func TestRunWithFallback_PreferredExists(t *testing.T) {
	spec := ToolSpec{
		Name: "echo",
		Args: []string{"preferred-was-used"},
	}
	fallback := ToolSpec{
		Name: "this-command-does-not-exist-zzzz",
		Args: []string{"fallback-was-used"},
	}

	// Capture stderr so we can assert the hint did NOT fire.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	if err := RunWithFallback(spec, fallback); err != nil {
		// echo always exits 0; if we got an error, the tool didn't
		// actually run (e.g., missing on $PATH in the test env).
		_ = w.Close()
		t.Skipf("echo not on $PATH in test env: %v", err)
	}
	_ = w.Close()

	var buf strings.Builder
	_ = readAll(r, &buf)
	if strings.Contains(buf.String(), "fallback") {
		t.Errorf("expected no fallback message; got: %q", buf.String())
	}
	if strings.Contains(buf.String(), "not found") {
		t.Errorf("expected no install hint; got: %q", buf.String())
	}
}

// TestRunWithFallback_PreferredMissing exercises the fallback path:
// LookPath returns an error, the install hint is printed, and the
// fallback tool is invoked. We use a fake name that we know doesn't
// exist on $PATH, and a real fallback (`echo`).
func TestRunWithFallback_PreferredMissing(t *testing.T) {
	spec := ToolSpec{
		Name:        "this-tool-does-not-exist-xyzzy",
		InstallHint: "go install example.com/fake@latest",
	}
	fallback := ToolSpec{
		Name: "echo",
		Args: []string{"fallback-was-used"},
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	if err := RunWithFallback(spec, fallback); err != nil {
		_ = w.Close()
		t.Skipf("echo fallback not on $PATH: %v", err)
	}
	_ = w.Close()

	var buf strings.Builder
	_ = readAll(r, &buf)
	if !strings.Contains(buf.String(), "this-tool-does-not-exist-xyzzy") {
		t.Errorf("expected stderr to mention the missing tool; got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "go install example.com/fake@latest") {
		t.Errorf("expected stderr to mention the install hint; got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "falling back") {
		t.Errorf("expected stderr to mention fallback; got: %q", buf.String())
	}
}

// TestRunWithFallback_NoHintWhenInstallHintEmpty verifies that
// callers can opt out of the install hint by leaving InstallHint
// empty. Used by cmd/build.go's path where the fallback would be
// misleading ("falling back to: go" when go is the only option).
func TestRunWithFallback_NoHintWhenInstallHintEmpty(t *testing.T) {
	spec := ToolSpec{
		Name:        "this-tool-does-not-exist-xyzzy",
		InstallHint: "", // explicitly suppressed
	}
	fallback := ToolSpec{
		Name: "echo",
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	_ = RunWithFallback(spec, fallback)
	_ = w.Close()

	var buf strings.Builder
	_ = readAll(r, &buf)
	if strings.Contains(buf.String(), "install with") {
		t.Errorf("expected no install hint when InstallHint empty; got: %q", buf.String())
	}
}

// readAll is a tiny helper: read everything from r into buf. We
// avoid io.ReadAll to keep the import set minimal.
func readAll(r interface{ Read(p []byte) (int, error) }, buf *strings.Builder) error {
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// TestRunTool_RespectsExtraEnv verifies that the ExtraEnv slice is
// appended to os.Environ() and that the child process sees the
// added variable. We use `/bin/sh -c 'echo $SPIN_TEST_EXTRA_ENV'`
// to echo the env var back.
func TestRunTool_RespectsExtraEnv(t *testing.T) {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	// Unique value per run so concurrent tests don't collide.
	value := "spin-test-extra-env-" + filepath.Base(t.Name())
	if err := runTool("/bin/sh", []string{"-c", "echo $SPIN_TEST_EXTRA_ENV"},
		[]string{"SPIN_TEST_EXTRA_ENV=" + value}); err != nil {
		_ = w.Close()
		t.Skipf("sh not on $PATH: %v", err)
	}
	_ = w.Close()

	var buf strings.Builder
	_ = readAll(r, &buf)
	if !strings.Contains(buf.String(), value) {
		t.Errorf("expected stdout to contain %q; got: %q", value, buf.String())
	}
}
