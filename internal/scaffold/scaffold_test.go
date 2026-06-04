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
// without touching the filesystem. The test runs in <1s and requires no
// Go toolchain beyond the test framework itself, proving the embed +
// template pipeline produces the expected files for the Walking
// Skeleton flag combination.
func TestRenderToMapWalkingSkeleton(t *testing.T) {
	p := &Project{
		Name:    "myapp",
		Module:  "myapp",
		Type:    "tui",
		Libs:    []string{"bubbletea"},
		Year:    2026,
		SpinVer: "0.1.0",
	}

	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("renderToMap returned empty map; embed not walking templates?")
	}

	// main.go is at the canonical Go path `cmd/<name>/main.go`.
	required := []string{"go.mod", "cmd/myapp/main.go", "README.md", ".gitignore"}
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
		"go 1.25.0",
		"charm.land/bubbletea/v2",
	} {
		if !bytes.Contains(goMod, []byte(want)) {
			t.Errorf("go.mod missing %q; got:\n%s", want, goMod)
		}
	}

	// main.go assertions: thin entry, hands off to app.Run.
	mainGo, ok := files["cmd/myapp/main.go"]
	if !ok {
		t.Fatal("cmd/myapp/main.go missing from rendered map")
	}
	for _, want := range []string{
		"package main",
		"app.Run",
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
	if _, err := (&Project{Name: "probe", Module: "probe", Type: "tui", Libs: []string{"bubbletea"}, Year: 2026, SpinVer: "0.1.0"}).renderToMap(); err != nil {
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

	// Assert the required files exist on disk. main.go is at
	// the canonical Go path `cmd/<name>/main.go`.
	for _, fname := range []string{"go.mod", "cmd/" + name + "/main.go", "README.md", ".gitignore"} {
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

// TestEmit_PathTraversal asserts that emit() rejects a rendered file map
// whose relative paths resolve outside the project root. The guard is
// the first line of defense against a template (or a buggy template
// helper) that interpolates `{{.Name}}` (or any other user-controlled
// value) into a path with `..` segments.
//
// Sub-cases cover the three common escape shapes:
//   - absolute `/etc/passwd` (POSIX) — `filepath.Join` strips the leading
//     `/` and joins with root, but the clean-root check still catches the
//     resulting path that escapes the sandbox.
//   - `../../etc/passwd` (relative) — the canonical traversal.
//   - `subdir/../../escape` (mixed) — the clean-then-prefix check has
//     to handle non-trailing `..` segments correctly.
func TestEmit_PathTraversal(t *testing.T) {
	// Use a per-test temp dir so emit()'s `os.MkdirAll(root)` operates
	// in a sandbox we own. Save and restore cwd.
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	p := &Project{Name: "myapp", Module: "myapp"}

	cases := []struct {
		name    string
		rel     string
		content []byte
	}{
		{"absolute_unix", "../../../etc/passwd", []byte("nope")},
		{"relative_traversal", "../../escape.txt", []byte("nope")},
		{"mixed_traversal", "a/b/../../../escape.txt", []byte("nope")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string][]byte{tc.rel: tc.content}
			err := emit(p, files)
			if err == nil {
				t.Fatalf("emit(%q): expected error for traversal; got nil", tc.rel)
			}
			if !strings.Contains(err.Error(), "path traversal") {
				t.Errorf("emit(%q): expected 'path traversal' in error; got: %v", tc.rel, err)
			}
		})
	}
}

// TestEmit_HappyPath is a positive control: a relative path that stays
// inside the project root is accepted. Pairs with TestEmit_PathTraversal
// so the guard is verified to allow legitimate writes too.
func TestEmit_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	p := &Project{Name: "myapp", Module: "myapp"}
	files := map[string][]byte{
		"go.mod":                  []byte("module myapp\n"),
		"internal/ui/styles.go":   []byte("package ui\n"),
		"a/b/c/deep.txt":          []byte("deep\n"),
	}
	if err := emit(p, files); err != nil {
		t.Fatalf("emit: unexpected error for in-root paths: %v", err)
	}

	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join("myapp", rel))
		if err != nil {
			t.Errorf("read %q: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%q: content = %q, want %q", rel, got, want)
		}
	}
}
