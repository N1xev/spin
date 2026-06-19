package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/N1xev/spin/internal/registry"
)

// withEmptyPinned sets XDG_CONFIG_HOME to a temp dir for the
// duration of the test, so the registry client reads/writes a
// throwaway pinned.json. Returns the cache dir it picked.
func withEmptyPinned(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "spin")
}

// seedPinned writes pinned records to the test cache and returns
// the path. Used by tests that need a non-empty pinned list.
func seedPinned(t *testing.T, cache string, pins ...registry.Pinned) {
	t.Helper()
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(pins, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "pinned.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRemove_UnknownNameIsError verifies `spin remove foo` errors
// when "foo" isn't pinned. A silent no-op would mask typos and
// leave the user thinking they cleaned up.
func TestRemove_UnknownNameIsError(t *testing.T) {
	withEmptyPinned(t)
	out, exitCode := runSpinExit(t, "remove", "nonexistent")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; got 0\n%s", out)
	}
	if !bytes.Contains(out, []byte("nonexistent")) {
		t.Errorf("error should mention the bad name; got:\n%s", out)
	}
	if !bytes.Contains(out, []byte("spin list")) {
		t.Errorf("error should suggest `spin list`; got:\n%s", out)
	}
}

// TestRemove_DropsFromPinnedJSON verifies the pin record is
// removed from pinned.json, leaving other pins untouched.
func TestRemove_DropsFromPinnedJSON(t *testing.T) {
	cache := withEmptyPinned(t)
	seedPinned(t, cache,
		registry.Pinned{Name: "keepme", LocalPath: "/tmp/keepme"},
		registry.Pinned{Name: "byebye", LocalPath: "/tmp/byebye"},
	)
	out, exitCode := runSpinExit(t, "remove", "byebye")
	if exitCode != 0 {
		t.Fatalf("expected exit 0; got %d\n%s", exitCode, out)
	}
	// pinned.json should now contain only "keepme".
	raw, err := os.ReadFile(filepath.Join(cache, "pinned.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("byebye")) {
		t.Errorf("pinned.json still contains 'byebye' after remove; got:\n%s", raw)
	}
	if !bytes.Contains(raw, []byte("keepme")) {
		t.Errorf("pinned.json lost unrelated pin 'keepme'; got:\n%s", raw)
	}
}

// TestRemove_PurgeDeletesCache verifies --purge removes the
// on-disk cache after the pin is dropped. Without --purge, the
// cache should remain.
func TestRemove_PurgeDeletesCache(t *testing.T) {
	cache := withEmptyPinned(t)
	pinDir := filepath.Join(t.TempDir(), "toremove")
	if err := os.MkdirAll(pinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedPinned(t, cache, registry.Pinned{Name: "toremove", LocalPath: pinDir})

	// First: remove WITHOUT --purge. Cache should remain.
	runSpinExit(t, "remove", "toremove")
	if _, err := os.Stat(pinDir); err != nil {
		t.Errorf("cache should remain after `spin remove` (no --purge); stat err=%v", err)
	}

	// Re-add the pin so we can test --purge.
	seedPinned(t, cache, registry.Pinned{Name: "toremove", LocalPath: pinDir})
	out, exitCode := runSpinExit(t, "remove", "toremove", "--purge")
	if exitCode != 0 {
		t.Fatalf("expected exit 0; got %d\n%s", exitCode, out)
	}
	if _, err := os.Stat(pinDir); !os.IsNotExist(err) {
		t.Errorf("cache should be deleted after `spin remove --purge`; stat err=%v", err)
	}
}

// TestList_JSONOutput verifies --json emits a valid JSON array
// of pinnedRow, one per pin, with no styling/escape codes.
func TestList_JSONOutput(t *testing.T) {
	cache := withEmptyPinned(t)
	pinDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(pinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Full spin.toml (Name is required by the parser).
	if err := os.WriteFile(filepath.Join(pinDir, "spin.toml"),
		[]byte("name = \"demo\"\ndescription = \"a demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedPinned(t, cache, registry.Pinned{
		Name:      "demo",
		Version:   "v1.0.0",
		Source:    "https://example.com/demo.git",
		LocalPath: pinDir,
		PinnedAt:  "2026-06-14T00:00:00Z",
	})

	out := runSpin(t, "list", "--json")
	var rows []pinnedRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Name != "demo" {
		t.Errorf("name = %q, want demo", rows[0].Name)
	}
	if rows[0].Description != "a demo" {
		t.Errorf("description = %q, want 'a demo'", rows[0].Description)
	}
	if rows[0].Version != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", rows[0].Version)
	}
}

// TestList_JSONOutputEmpty verifies the empty-list JSON output is
// a valid empty array, not an error or "no pinned" text. The
// jq/caller contract is "always parseable as []type pinnedRow".
func TestList_JSONOutputEmpty(t *testing.T) {
	withEmptyPinned(t)
	out := runSpin(t, "list", "--json")
	trimmed := strings.TrimSpace(string(out))
	if trimmed != "[]" {
		t.Errorf("empty list --json should be '[]', got: %q", trimmed)
	}
}

// TestList_DefaultIsTable verifies the human-readable path still
// works: --help mentions it, and the table contains the pin's
// name and version.
func TestList_DefaultIsTable(t *testing.T) {
	cache := withEmptyPinned(t)
	seedPinned(t, cache, registry.Pinned{
		Name: "mything", Version: "abc123", LocalPath: "/tmp/mything",
	})
	out := runSpin(t, "list")
	if !bytes.Contains(out, []byte("mything")) {
		t.Errorf("default `spin list` should mention pin name; got:\n%s", out)
	}
	if !bytes.Contains(out, []byte("abc123")) {
		t.Errorf("default `spin list` should mention version; got:\n%s", out)
	}
	// Sanity: no JSON braces in default output.
	if bytes.Contains(out, []byte("[")) && bytes.Contains(out, []byte("{")) {
		t.Errorf("default `spin list` should not emit JSON; got:\n%s", out)
	}
}
