package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestCurrentFS_EmbeddedWhenEmpty asserts that currentFS("") returns
// the package-level embed.FS (FS). The trivial branch: no external
// override in effect, so the walker reads from the embed.
//
// The assertion uses interface equality: templateFS is an interface,
// and both the empty-string and external-dir branches must return a
// value assignable to that interface.
func TestCurrentFS_EmbeddedWhenEmpty(t *testing.T) {
	got := currentFS("")
	if got == nil {
		t.Fatal("currentFS(\"\") = nil, want non-nil")
	}
	// Read a known embedded file (the Walking Skeleton's go.mod.tmpl).
	// If currentFS("") is correctly returning the embed, this should
	// succeed; if it accidentally returns an empty FS, ReadFile errors.
	_, err := fs.ReadFile(got, "templates/_base/go.mod.tmpl")
	if err != nil {
		t.Errorf("ReadFile(templates/_base/go.mod.tmpl) via currentFS(\"\") = %v, want nil", err)
	}
}

// TestCurrentFS_DirFSWhenSet asserts the external-dir branch:
// currentFS(<some tempdir>) returns an fsys that reads from that
// tempdir via os.DirFS. The test creates a tempdir, writes a file,
// and reads it back through the returned templateFS.
func TestCurrentFS_DirFSWhenSet(t *testing.T) {
	tmp := t.TempDir()

	// Write a marker file so we can confirm the read path is the
	// tempdir, not the embed.
	marker := "external-marker.txt"
	want := []byte("hello from external template\n")
	if err := os.WriteFile(filepath.Join(tmp, marker), want, 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	got := currentFS(tmp)
	if got == nil {
		t.Fatal("currentFS(tmp) = nil, want non-nil")
	}

	got2, err := fs.ReadFile(got, marker)
	if err != nil {
		t.Fatalf("ReadFile(%q) = %v, want nil", marker, err)
	}
	if string(got2) != string(want) {
		t.Errorf("ReadFile(%q) = %q, want %q", marker, got2, want)
	}
}
