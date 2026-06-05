package update

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

const fixtureGoMod = `module example.com/dummy

go 1.23

require (
	charm.land/bubbletea/v2 v2.0.7
	charm.land/lipgloss/v2 v2.0.3
	github.com/spf13/cobra v1.9.1
	github.com/mattn/go-runewidth v0.0.23 // indirect
	charm.land/bubbles/v2 v2.1.0 // indirect
)
`

// writeGoMod drops content into a tempdir go.mod and returns the
// path. Used by every ListDeps test so they share a stable fixture.
func writeGoMod(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	return path
}

// modulesOf extracts the Module field in order, for stable equality
// checks. Sorting the result defensively (in case future code
// stops pre-sorting) keeps these tests honest.
func modulesOf(deps []Dep) []string {
	out := make([]string, len(deps))
	for i, d := range deps {
		out[i] = d.Module
	}
	sort.Strings(out)
	return out
}

func TestListDeps_DirectOnly(t *testing.T) {
	path := writeGoMod(t, fixtureGoMod)

	deps, err := ListDeps(path, false)
	if err != nil {
		t.Fatalf("ListDeps: %v", err)
	}

	if len(deps) != 3 {
		t.Fatalf("len(deps) = %d, want 3; got %v", len(deps), deps)
	}
	for _, d := range deps {
		if d.Indirect {
			t.Errorf("dep %q marked Indirect in direct-only result", d.Module)
		}
	}

	want := []string{
		"charm.land/bubbletea/v2",
		"charm.land/lipgloss/v2",
		"github.com/spf13/cobra",
	}
	got := modulesOf(deps)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("modules = %v, want %v", got, want)
	}

	// Alphabetical assertion: result is already sorted by Module
	// inside ListDeps, but verify the in-place order directly so a
	// future sort regression in the function does not silently hide.
	if deps[0].Module != "charm.land/bubbletea/v2" ||
		deps[1].Module != "charm.land/lipgloss/v2" ||
		deps[2].Module != "github.com/spf13/cobra" {
		t.Errorf("deps not sorted alphabetically: %+v", deps)
	}
}

func TestListDeps_IncludeIndirect(t *testing.T) {
	path := writeGoMod(t, fixtureGoMod)

	deps, err := ListDeps(path, true)
	if err != nil {
		t.Fatalf("ListDeps: %v", err)
	}

	if len(deps) != 5 {
		t.Fatalf("len(deps) = %d, want 5; got %v", len(deps), deps)
	}

	indirect := map[string]bool{}
	for _, d := range deps {
		if d.Indirect {
			indirect[d.Module] = true
		}
	}
	if !indirect["charm.land/bubbles/v2"] {
		t.Error("expected charm.land/bubbles/v2 to be Indirect")
	}
	if !indirect["github.com/mattn/go-runewidth"] {
		t.Error("expected github.com/mattn/go-runewidth to be Indirect")
	}
}

func TestListDeps_MissingFile(t *testing.T) {
	_, err := ListDeps(filepath.Join(t.TempDir(), "go.mod"), false)
	if err == nil {
		t.Fatal("expected error for missing go.mod, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist; got: %v", err)
	}
}

func TestListDeps_Deterministic(t *testing.T) {
	path := writeGoMod(t, fixtureGoMod)

	first, err := ListDeps(path, true)
	if err != nil {
		t.Fatalf("first ListDeps: %v", err)
	}
	second, err := ListDeps(path, true)
	if err != nil {
		t.Fatalf("second ListDeps: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Errorf("ListDeps is not deterministic across calls\nfirst:  %+v\nsecond: %+v", first, second)
	}

	// Byte-equal module ordering. Catches a future regression that
	// iterates a Go map and feeds unsorted output to the huh form.
	for i := range first {
		if first[i].Module != second[i].Module {
			t.Errorf("position %d: first=%q second=%q", i, first[i].Module, second[i].Module)
		}
	}
}

func TestFindGoMod_FromSubdir(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "go.mod")
	if err := os.WriteFile(want, []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}

	sub := filepath.Join(root, "internal", "pkg", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got, err := FindGoMod(sub)
	if err != nil {
		t.Fatalf("FindGoMod: %v", err)
	}
	if got != want {
		t.Errorf("FindGoMod = %q, want %q", got, want)
	}
}

func TestFindGoMod_NoModAnywhere(t *testing.T) {
	empty := t.TempDir()
	_, err := FindGoMod(empty)
	if err == nil {
		t.Fatal("expected error when no go.mod exists, got nil")
	}
	if !contains(err.Error(), "no go.mod found") {
		t.Errorf("expected error to mention 'no go.mod found'; got: %v", err)
	}
}

// contains is a tiny substring helper so this file does not pull in
// strings just for one assertion.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
