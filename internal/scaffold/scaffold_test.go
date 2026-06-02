package scaffold

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderToMapWalkingSkeleton builds a *Project representing the
// --tui --bubbletea Walking Skeleton combination and asserts the rendered
// file map contains the expected files with the expected content.
//
// The test exercises the embed -> walk -> overlay -> text/template pipeline
// without touching the filesystem. Plan 02/03 expand the Project fields
// (License, Template, Force, NoGit, Quiet, ...) and the overlay engine.
//
// The test runs in <1s and requires no Go toolchain beyond the test
// framework itself, proving the embed + template pipeline produces the
// expected files for the Walking Skeleton flag combination.
func TestRenderToMapWalkingSkeleton(t *testing.T) {
	p := &Project{
		Name:    "myapp",
		Module:  "myapp",
		Type:    "tui",
		Libs:    []string{"bubbletea"},
		Year:    2026,
		SpinVer: "0.1.0",
	}

	files, err := renderToMap(p)
	if err != nil {
		t.Fatalf("renderToMap failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("renderToMap returned empty map; embed not walking templates?")
	}

	// Required files in the Walking Skeleton output.
	required := []string{"go.mod", "main.go", "README.md", ".gitignore"}
	for _, name := range required {
		if _, ok := files[name]; !ok {
			t.Errorf("missing required rendered file %q; got keys: %v", name, keysOf(files))
		}
	}

	// go.mod assertions.
	goMod, ok := files["go.mod"]
	if !ok {
		t.Fatal("go.mod missing from rendered map")
	}
	for _, want := range []string{
		"module myapp",
		"go 1.23",
		"charm.land/bubbletea/v2",
	} {
		if !bytes.Contains(goMod, []byte(want)) {
			t.Errorf("go.mod missing %q; got:\n%s", want, goMod)
		}
	}

	// main.go assertions.
	mainGo, ok := files["main.go"]
	if !ok {
		t.Fatal("main.go missing from rendered map")
	}
	for _, want := range []string{
		"package main",
		"tea.NewProgram",
	} {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("main.go missing %q; got:\n%s", want, mainGo)
		}
	}
}

// TestNewEndToEndWalkingSkeleton is the full pipeline test: render, emit
// to a temp directory, and run `go build ./...` with CGO_ENABLED=0 in the
// generated project. Requires the templates to exist (Task 3) and a Go
// toolchain on the test runner (we're already running `go test`).
//
// Skipped if templates have not been written yet (Task 3) so Task 2
// (engine only) and Task 3 (templates) can land independently.
func TestNewEndToEndWalkingSkeleton(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end in -short mode")
	}
	// Pre-flight: if templates haven't been written yet, skip.
	// We probe by trying a one-line render — if it errors with "no
	// templates", skip; otherwise proceed.
	if _, err := renderToMap(&Project{Name: "probe", Module: "probe", Type: "tui", Libs: []string{"bubbletea"}, Year: 2026, SpinVer: "0.1.0"}); err != nil {
		if strings.Contains(err.Error(), "no templates") || strings.Contains(err.Error(), "embed") {
			t.Skipf("templates not yet written (deferred to Task 3): %v", err)
		}
	}

	tmp := t.TempDir()
	name := "scaffold-test-myapp"
	projectDir := filepath.Join(tmp, name)

	p := &Project{
		Name:    name,
		Module:  name,
		Type:    "tui",
		Libs:    []string{"bubbletea"},
		Year:    2026,
		SpinVer: "0.1.0",
	}

	// Change working directory to tmp so the scaffold emits ./<name>/ here.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := New(p); err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Assert the four required files exist on disk.
	for _, fname := range []string{"go.mod", "main.go", "README.md", ".gitignore"} {
		path := filepath.Join(projectDir, fname)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %q not on disk: %v", path, err)
		}
	}

	// Smoke test: go build ./... with CGO disabled.
	build := exec.Command("go", "build", "./...")
	build.Dir = projectDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... failed in %s:\n%s", projectDir, out)
	}
}

func keysOf(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
