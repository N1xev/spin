package rust

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRustPost_NoTraversal guards the writeFiles helper against the
// STRIDE T-05-01 path-traversal class of bug: a file map containing
// a key like "../escape.txt" must be rejected rather than written
// outside of the destination directory.
func TestRustPost_NoTraversal(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"../escape.txt": []byte("should never be written"),
	}
	err := writeFiles(dir, files)
	if err == nil {
		t.Fatal("writeFiles accepted a path-traversal key; expected non-nil error")
	}

	// Verify the escape file was NOT created.
	parent := filepath.Dir(dir)
	escape := filepath.Join(parent, "escape.txt")
	if _, statErr := os.Stat(escape); statErr == nil {
		t.Fatalf("escape file was created at %q; writeFiles failed to guard against path traversal", escape)
	}
}
