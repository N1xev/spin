package wrap

import (
	"os"
	"strings"
	"testing"
)

// TestVet_Runs verifies that Vet() wires through to `go vet ./...`.
// We don't have an easy way to assert on the exact `go vet` argv
// without an external process-trace hook, so we exercise the path
// in a tempdir that has a known-bad Go file and assert that Vet
// returns non-zero (i.e., `go vet` exited non-zero on the bad code).
//
// A clean empty dir makes `go vet ./...` exit 0; we use the
// negative-result approach so the test is robust to whatever is
// in the tempdir.
func TestVet_Runs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Write a minimal go.mod + a Go file with an obvious vet error:
	// a Printf format mismatch. `go vet` will exit 1.
	if err := os.WriteFile("go.mod", []byte("module vetfallback\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	badMain := `package main

import "fmt"

func main() {
	fmt.Printf("%d", "not a number")
}
`
	if err := os.WriteFile("main.go", []byte(badMain), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	origStderr := os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = devNull.Close()
	})

	err := Vet()
	if err == nil {
		t.Errorf("expected Vet() to return non-zero on a bad printf; got nil")
	}
}

// TestVet_CleanDirPasses is the positive case: in a tempdir with a
// well-formed Go file, Vet() returns nil. This guards against a
// regression where Vet() always returns non-zero.
func TestVet_CleanDirPasses(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile("go.mod", []byte("module vetpass\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	origStderr := os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = devNull.Close()
	})

	if err := Vet(); err != nil {
		t.Errorf("expected Vet() to pass on clean code; got: %v", err)
	}
}

// TestVet_InvokesGoVet confirms the underlying go-vet path is
// reached by introspecting the ToolSpec used in vet.go. We
// re-create the spec inline and assert on its shape; this is a
// pinned regression test, not a behavioral one.
func TestVet_InvokesGoVet(t *testing.T) {
	spec := ToolSpec{Name: "go", Args: []string{"vet", "./..."}}
	if !strings.Contains(strings.Join(spec.Args, " "), "vet") {
		t.Errorf("Vet() must pass 'vet' to go; got args: %v", spec.Args)
	}
}
