package cmd

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/N1xev/spin/internal/registry"
)

// TestNew_PositionalForm covers `spin new <name> <template>`
// without --template.
func TestNew_PositionalForm(t *testing.T) {
	dir := t.TempDir()
	tplParent := t.TempDir()
	initOut, initExit := runSpinWithDir(t, tplParent, "init", "go-cli")
	if initExit != 0 {
		t.Fatalf("spin init go-cli failed: exit=%d\n%s", initExit, initOut)
	}
	tplPath := filepath.Join(tplParent, "go-cli")

	out, exitCode := runSpinWithDir(t, dir, "new", "myapp", tplPath, "--print-params")
	if exitCode != 0 {
		t.Fatalf("expected exit 0; got %d\n%s", exitCode, out)
	}
	if !bytes.Contains(out, []byte(`"name": "myapp"`)) {
		t.Errorf("expected resolved params to include name=myapp; got:\n%s", out)
	}
	if !bytes.Contains(out, []byte("go-cli")) {
		t.Errorf("expected output to mention template name go-cli; got:\n%s", out)
	}
}

// TestNew_FlagStillWorks covers backward compat with --template.
func TestNew_FlagStillWorks(t *testing.T) {
	dir := t.TempDir()
	tplParent := t.TempDir()
	initOut, initExit := runSpinWithDir(t, tplParent, "init", "go-cli")
	if initExit != 0 {
		t.Fatalf("spin init go-cli failed: exit=%d\n%s", initExit, initOut)
	}
	tplPath := filepath.Join(tplParent, "go-cli")

	out, exitCode := runSpinWithDir(t, dir, "new", "myapp", "--template", tplPath, "--print-params")
	if exitCode != 0 {
		t.Fatalf("expected exit 0; got %d\n%s", exitCode, out)
	}
	if !bytes.Contains(out, []byte(`"name": "myapp"`)) {
		t.Errorf("expected resolved params to include name=myapp; got:\n%s", out)
	}
}

// TestNew_PositionalPlusFlagConflicts covers the validator
// rejecting positional <template> + --template together.
func TestNew_PositionalPlusFlagConflicts(t *testing.T) {
	out, exitCode := runSpinExit(t, "new", "myapp", "go-cli", "--template", "other")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("cannot pass <template> both positionally")) {
		t.Errorf("error should mention the conflict; got:\n%s", out)
	}
}

// TestNew_TooManyArgsRejected covers the validator rejecting >2
// positionals.
func TestNew_TooManyArgsRejected(t *testing.T) {
	out, exitCode := runSpinExit(t, "new", "a", "b", "c")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("at most 2 positional args")) {
		t.Errorf("error should mention the limit; got:\n%s", out)
	}
}

// TestNew_SinglePositionalTemplateSpec covers `spin new <template>`
// where the lone positional arg is a local path. It should be treated
// as the template, not the project name, so non-interactive mode errors
// on the missing name (not on a missing template picker).
func TestNew_SinglePositionalTemplateSpec(t *testing.T) {
	tplParent := t.TempDir()
	initOut, initExit := runSpinWithDir(t, tplParent, "init", "go-cli")
	if initExit != 0 {
		t.Fatalf("spin init go-cli failed: exit=%d\n%s", initExit, initOut)
	}
	tplPath := filepath.Join(tplParent, "go-cli")

	out, exitCode := runSpinClosedStdin(t, "new", tplPath, "--print-params")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("<name> is required in non-interactive mode")) {
		t.Errorf("error should name missing <name>, not prompt for template; got:\n%s", out)
	}
}

// a pipe (ModeNamedPipe, not ModeCharDevice) so isInteractive()
// returns false. runSpinExit inherits the test runner's TTY and
// would trigger the huh form.
func runSpinClosedStdin(t *testing.T, args ...string) ([]byte, int) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "spin-closed-stdin")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	run := exec.Command(bin, args...)
	run.Stdin = strings.NewReader("")
	out, err := run.CombinedOutput()
	if err == nil {
		return out, 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out, exitErr.ExitCode()
	}
	return out, 1
}

// TestNew_MissingNameNonInteractive covers the resolver's
// non-interactive error when <name> is missing.
func TestNew_MissingNameNonInteractive(t *testing.T) {
	out, exitCode := runSpinClosedStdin(t, "new")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("<name> is required in non-interactive mode")) {
		t.Errorf("error should name <name> as the missing arg; got:\n%s", out)
	}
}

// TestNew_MissingTemplateNonInteractive covers the same shape for
// missing <template>.
func TestNew_MissingTemplateNonInteractive(t *testing.T) {
	withEmptyPinned(t)
	out, exitCode := runSpinClosedStdin(t, "new", "myapp")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("<template> is required in non-interactive mode")) {
		t.Errorf("error should name <template> as the missing arg; got:\n%s", out)
	}
	if !bytes.Contains(out, []byte("spin search")) {
		t.Errorf("error should hint at `spin search`; got:\n%s", out)
	}
}

// TestNew_InvalidPinnedPathErrors covers a pinned template whose
// LocalPath is gone.
func TestNew_InvalidPinnedPathErrors(t *testing.T) {
	cache := withEmptyPinned(t)
	seedPinned(t, cache, registry.Pinned{
		Name:      "ghost",
		Source:    "/nonexistent/path/that/does/not/exist",
		LocalPath: "/nonexistent/path/that/does/not/exist",
		Version:   "local",
	})
	out, exitCode := runSpinExit(t, "new", "myapp", "ghost", "--print-params")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("ghost")) {
		t.Errorf("error should mention the template name; got:\n%s", out)
	}
}
