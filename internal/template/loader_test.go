package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoader_Load_LocalPath verifies Load with a local dir
// returns a non-nil *Template with the correct BaseDir. This is
// the happy path for `spin new <name> --template /path/to/tpl`.
func TestLoader_Load_LocalPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.toml"), []byte("name = \"tpl\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "_base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_base", "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(t.TempDir())
	tpl, err := l.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tpl == nil {
		t.Fatal("Load returned nil Template")
	}
	if tpl.BaseDir == "" {
		t.Errorf("Template.BaseDir is empty")
	}
}

// TestLoader_Load_LocalPath_MissingSpinToml verifies Load fails
// (with a clear "spin.toml not found" error) when the local dir
// has no spin.toml. The error message is part of the v2.0
// contract — if it changes, the CLI's user-facing error
// degrades silently.
func TestLoader_Load_LocalPath_MissingSpinToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "_base"), 0o755); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(t.TempDir())
	_, err := l.Load(dir)
	if err == nil {
		t.Fatal("Load should fail when spin.toml is missing")
	}
	if !strings.Contains(err.Error(), "spin.toml") {
		t.Errorf("error should mention spin.toml, got: %q", err.Error())
	}
}

// TestLoader_Load_LocalPath_MissingBase verifies Load fails when
// the local dir has spin.toml but no _base/ directory. The
// _base/ tree is the source of files-to-render; without it, the
// template produces nothing, so we fail fast.
func TestLoader_Load_LocalPath_MissingBase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.toml"), []byte("name = \"tpl\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(t.TempDir())
	_, err := l.Load(dir)
	if err == nil {
		t.Fatal("Load should fail when _base/ is missing")
	}
	if !strings.Contains(err.Error(), "_base") {
		t.Errorf("error should mention _base/, got: %q", err.Error())
	}
}

// TestLoader_IsLocalPath verifies the heuristic that distinguishes
// a local path from a git URL. Used by Load to dispatch.
func TestLoader_IsLocalPath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"/foo", true},
		{"./foo", true},
		{"~foo", true},
		{"https://github.com/foo/bar", false},
		{"git@github.com:foo/bar", false},
		{"foo/bar", false}, // ambiguous; treated as a registry shorthand, not a local path
	}
	for _, tc := range cases {
		if got := isLocalPath(tc.in); got != tc.want {
			t.Errorf("isLocalPath(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestLoader_IsGitURL verifies the heuristic for git URL detection.
func TestLoader_IsGitURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://github.com/foo/bar", true},
		{"http://github.com/foo/bar", true},
		{"git@github.com:foo/bar", true},
		{"git://github.com/foo/bar", true},
		{"ssh://git@github.com/foo/bar", true},
		{"/local/path", false},
		{"./local", false},
	}
	for _, tc := range cases {
		if got := isGitURL(tc.in); got != tc.want {
			t.Errorf("isGitURL(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestRender_PathTraversal verifies that rendering a template
// with a key like "../escape.txt" is rejected by writeFiles.
// This is the v2.0 security guard against malicious templates
// escaping the destination dir.
func TestRender_PathTraversal(t *testing.T) {
	// Build a path-traversal file map by hand and call
	// WriteFiles directly. We don't need a real Template for
	// this test — the security guard is in writeFiles, which
	// is the same code path RenderTo uses.
	dest := t.TempDir()
	err := WriteFiles(dest, map[string][]byte{
		"../escape.txt": []byte("evil"),
	})
	if err == nil {
		t.Fatal("WriteFiles should reject path-traversal key")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention 'path traversal', got: %q", err.Error())
	}
}

// TestRender_DeletesSpinToml verifies that RenderToWithPost
// removes spin.toml from the destination dir as the last step.
// TPL-16: "spin.toml is deleted from the output after a
// successful render". The deletion is a defensive walk in case
// the template's _base/ accidentally includes a spin.toml.
func TestRender_DeletesSpinToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.toml"), []byte("name = \"tpl\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "_base"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Include a spin.toml in _base/ as if the template author
	// was sloppy.
	if err := os.WriteFile(filepath.Join(dir, "_base", "spin.toml"), []byte("name = \"stray\""), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-spin.toml file alongside it (so we have something
	// to render).
	if err := os.WriteFile(filepath.Join(dir, "_base", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	dest := t.TempDir()
	if err := tpl.RenderToWithPost(dest, map[string]any{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	// spin.toml at top level was never rendered (it's not in
	// _base), so it doesn't exist at dest/spin.toml. The
	// _base/spin.toml IS rendered, so it appears at
	// dest/spin.toml — and the defensive walk must remove it.
	if _, err := os.Stat(filepath.Join(dest, "spin.toml")); !os.IsNotExist(err) {
		t.Errorf("dest/spin.toml should NOT exist (TPL-16), but stat says: %v", err)
	}
	// The other file is still there.
	if _, err := os.Stat(filepath.Join(dest, "main.go")); err != nil {
		t.Errorf("dest/main.go should exist, got: %v", err)
	}
}

// TestRunPostHook_RunsShellCommand verifies RunPostHook executes
// the [post] hook command via `sh -c` in the given dir, with
// the supplied values available as template variables. The hook
// runs AFTER files are written and BEFORE spin.toml deletion.
func TestRunPostHook_RunsShellCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.toml"), []byte("name = \"tpl\"\n[post]\nrun = \"echo {{.name}} > post-out.txt && touch post-ran.txt\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "_base"), 0o755); err != nil {
		t.Fatal(err)
	}
	tpl, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if err := RunPostHook(tpl, map[string]any{"name": "test-proj"}, dir); err != nil {
		t.Fatalf("RunPostHook: %v", err)
	}
	// post-ran.txt is the touch side-effect; it must exist.
	if _, err := os.Stat(filepath.Join(dir, "post-ran.txt")); err != nil {
		t.Errorf("post-ran.txt should exist (touch side-effect), got: %v", err)
	}
	// post-out.txt is the echo output; it should contain the
	// interpolated name.
	b, err := os.ReadFile(filepath.Join(dir, "post-out.txt"))
	if err != nil {
		t.Fatalf("ReadFile post-out.txt: %v", err)
	}
	if !strings.Contains(string(b), "test-proj") {
		t.Errorf("post-out.txt = %q, want it to contain %q", string(b), "test-proj")
	}
}

// TestLoader_Load_GitURL_Mock verifies Load dispatches to the
// git-clone branch for git URLs. We can't actually clone (no
// network in tests, and a fake host hangs on DNS), so we
// exercise only the dispatcher: the URL must not be treated as
// a local path (isLocalPath returns false) and must be
// recognised as a git URL (isGitURL returns true). The actual
// cloneGit failure mode is covered by the integration
// verification in the plan (it requires a real git server).
func TestLoader_Load_GitURL_Mock(t *testing.T) {
	spec := "https://github.com/foo/bar.git"
	if isLocalPath(spec) {
		t.Errorf("isLocalPath(%q) = true, want false (git URLs are not local)", spec)
	}
	if !isGitURL(spec) {
		t.Errorf("isGitURL(%q) = false, want true", spec)
	}
}
