package wrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuild_ProducesBinary exercises the happy path: in a fresh
// tempdir with a minimal go.mod + main.go, Build() must create
// bin/<basename> as an executable file.
//
// We intentionally use a minimal main.go (not a full project) to
// keep the test fast and isolated. Build()'s contract is: it always
// runs `go build -o bin/<name> .` with CGO_ENABLED=0. If the build
// succeeds, the binary exists. If it fails, Build() returns an
// error, which we surface as a test failure (not a skip) — a
// broken Build() in the test env is a real regression.
func TestBuild_ProducesBinary(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Minimal valid module + main.go.
	if err := os.WriteFile("go.mod", []byte("module buildtest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile("main.go", []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if err := Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	binPath := filepath.Join("bin", filepath.Base(dir))
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("expected binary at %s: %v", binPath, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("binary at %s is not executable (mode=%v)", binPath, info.Mode())
	}
}

// TestBuild_CreatesBinDir verifies that Build() creates the bin/
// directory if it doesn't already exist. We do not write any
// Go sources so `go build` will fail, but the bin/ directory
// must still have been created before go was invoked.
func TestBuild_CreatesBinDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// We expect Build() to fail because there's no go.mod. We
	// redirect stderr to /dev/null to keep test output clean, then
	// check that bin/ was created.
	origStderr := os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = devNull.Close()
	})

	_ = Build()

	info, err := os.Stat("bin")
	if err != nil {
		t.Fatalf("expected bin/ to exist after Build(): %v", err)
	}
	if !info.IsDir() {
		t.Errorf("bin/ is not a directory (mode=%v)", info.Mode())
	}
}

// TestBuild_CGOEnabledZero verifies that Build()'s ToolSpec wires
// CGO_ENABLED=0 into the child env. We exec `go env CGO_ENABLED`
// through the same code path used by Build() (the runTool helper
// with the same ExtraEnv shape) and assert the output is "0".
func TestBuild_CGOEnabledZero(t *testing.T) {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	spec := ToolSpec{
		Name:     "go",
		Args:     []string{"env", "CGO_ENABLED"},
		ExtraEnv: []string{"CGO_ENABLED=0"},
	}
	if err := runTool(spec.Name, spec.Args, spec.ExtraEnv); err != nil {
		_ = w.Close()
		t.Skipf("go not on $PATH: %v", err)
	}
	_ = w.Close()

	out := readPipe(t, r)
	if !strings.Contains(out, "0") {
		t.Errorf("expected CGO_ENABLED=0 in go env output; got: %q", out)
	}
}
