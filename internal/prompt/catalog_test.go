// Package prompt tests for the library catalog.
//
// These tests assert the catalog invariants:
//   - LibCatalog has no duplicate Name values
//   - LibCatalog is sorted alphabetically by Name (deterministic order)
//   - LibsForType returns the correct defaults for each variant
//   - DefaultLibsFor matches LibsForType

package prompt_test

import (
	"sort"
	"testing"

	"github.com/example/spin/internal/prompt"
)

// TestLibCatalog_UniqueNames asserts that no two catalog entries share
// the same Name. A duplicate would corrupt the multi-select prompt
// (huh requires unique option values) and the AGENTS.md template
// (which iterates over the catalog).
func TestLibCatalog_UniqueNames(t *testing.T) {
	seen := make(map[string]bool, len(prompt.LibCatalog))
	for _, lib := range prompt.LibCatalog {
		if seen[lib.Name] {
			t.Errorf("LibCatalog contains duplicate Name %q", lib.Name)
		}
		seen[lib.Name] = true
	}
}

// TestLibCatalog_Sorted asserts that LibCatalog is sorted alphabetically
// by Name. The sort is required for the deterministic-output contract:
// the multi-select option order and the AGENTS.md library section
// order must both be stable across runs.
func TestLibCatalog_Sorted(t *testing.T) {
	names := make([]string, len(prompt.LibCatalog))
	for i, lib := range prompt.LibCatalog {
		names[i] = lib.Name
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	for i := range names {
		if names[i] != sorted[i] {
			t.Errorf("LibCatalog not sorted at index %d: got %q, want %q (full: %v)",
				i, names[i], sorted[i], names)
		}
	}
}

// TestLibsForType asserts the variant-specific default library list:
//
//   - tui: bubbletea
//   - cli: cobra, fang
//   - all: all three (the union of tui and cli defaults)
//
// The "all" case is the special case in LibsForType: a lib whose
// DefaultFor is "tui" or "cli" is also returned for "all".
func TestLibsForType(t *testing.T) {
	cases := []struct {
		variant string
		want    []string
	}{
		{"tui", []string{"bubbletea"}},
		{"cli", []string{"cobra", "fang"}},
		{"all", []string{"bubbletea", "cobra", "fang"}},
		// Unknown variant returns no defaults — the user must have
		// passed an explicit flag set for the multi-select to have
		// anything to pre-select.
		{"", nil},
		{"unknown", nil},
	}
	for _, c := range cases {
		t.Run(c.variant, func(t *testing.T) {
			got := prompt.LibsForType(c.variant)
			// Compare as sets: order matches catalog, but the test
			// should not be brittle to internal sort changes. Use
			// sort.Strings on both sides for a stable comparison.
			gSorted := append([]string(nil), got...)
			sort.Strings(gSorted)
			wSorted := append([]string(nil), c.want...)
			sort.Strings(wSorted)
			if !stringSlicesEqual(gSorted, wSorted) {
				t.Errorf("LibsForType(%q) = %v, want %v", c.variant, got, c.want)
			}
		})
	}
}

// TestDefaultLibsFor_MatchesLibsForType asserts the convenience alias
// returns the same result as LibsForType. The two are kept as
// separate functions for readability at call sites; the alias must
// not diverge from the canonical implementation.
func TestDefaultLibsFor_MatchesLibsForType(t *testing.T) {
	for _, variant := range []string{"tui", "cli", "all", "", "unknown"} {
		got := prompt.DefaultLibsFor(variant)
		want := prompt.LibsForType(variant)
		if !stringSlicesEqual(got, want) {
			t.Errorf("DefaultLibsFor(%q) = %v, LibsForType(%q) = %v (must match)",
				variant, got, variant, want)
		}
	}
}

// stringSlicesEqual reports whether two string slices are equal
// (same length, same elements in the same order).
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
