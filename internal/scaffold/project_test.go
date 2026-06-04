// Tests for the AllLibs method.
//
// AllLibs unifies p.Libs and the per-lib bools (Cobra, Fang, Viper,
// Huh, Glamour, Wish, Log, Harmonica) into a single sorted,
// deduplicated slice. It is consumed by the prompt layer
// (askLibs) and the AGENTS.md template.

package scaffold

import (
	"reflect"
	"testing"
)

// TestProject_AllLibs_OnlyLibsSet covers the case where the user
// only passed --bubbletea and --lipgloss (no per-lib bools). The
// method must return those two names, sorted alphabetically.
func TestProject_AllLibs_OnlyLibsSet(t *testing.T) {
	p := &Project{Libs: []string{"bubbletea", "lipgloss"}}
	got := p.AllLibs()
	want := []string{"bubbletea", "lipgloss"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllLibs() = %v, want %v", got, want)
	}
}

// TestProject_AllLibs_OnlyBoolsSet covers the case where the user
// only passed --cobra and --fang (no --cobra / --fang in p.Libs).
// The method must derive the union from the bool flags and return
// them sorted.
func TestProject_AllLibs_OnlyBoolsSet(t *testing.T) {
	p := &Project{Cobra: true, Fang: true}
	got := p.AllLibs()
	want := []string{"cobra", "fang"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllLibs() = %v, want %v", got, want)
	}
}

// TestProject_AllLibs_Mixed covers the union-of-both case: a user
// who passed --huh (a bool) AND --bubbletea (a string flag) should
// see both in the result, deduped, sorted.
func TestProject_AllLibs_Mixed(t *testing.T) {
	p := &Project{
		Libs:  []string{"bubbletea", "lipgloss"},
		Cobra: true,
		Huh:   true,
	}
	got := p.AllLibs()
	want := []string{"bubbletea", "cobra", "huh", "lipgloss"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllLibs() = %v, want %v", got, want)
	}
}

// TestProject_AllLibs_Dedup covers the dedup invariant: a lib that
// appears in BOTH p.Libs and a per-lib bool (e.g., a user who did
// `spin new foo --tui --cobra` — cobra is auto-on for --cli but the
// user explicitly added it, and the boolFlagOverlayMap returns it
// for the --cobra=true case) must appear only once in the result.
//
// We construct the overlap by setting Cobra=true (bool field) AND
// "cobra" in p.Libs.
func TestProject_AllLibs_Dedup(t *testing.T) {
	p := &Project{
		Libs:  []string{"bubbletea", "cobra"},
		Cobra: true,
	}
	got := p.AllLibs()
	want := []string{"bubbletea", "cobra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllLibs() = %v, want %v", got, want)
	}
}

// TestProject_AllLibs_Empty covers the zero-value case: a fresh
// *Project{} has no libs and no bools. The method must return an
// empty slice (not nil) so callers can range without nil-checking.
func TestProject_AllLibs_Empty(t *testing.T) {
	p := &Project{}
	got := p.AllLibs()
	if got == nil {
		t.Error("AllLibs() = nil, want empty (non-nil) slice")
	}
	if len(got) != 0 {
		t.Errorf("AllLibs() length = %d, want 0", len(got))
	}
}

// TestProject_AllLibs_AllBoolsSet covers the case where every
// per-lib bool is set: the result must contain all 8 bool-mapped
// libs, sorted alphabetically. This is the upper-bound coverage for
// the bool-flag derivation path.
func TestProject_AllLibs_AllBoolsSet(t *testing.T) {
	p := &Project{
		Cobra:     true,
		Fang:      true,
		Viper:     true,
		Huh:       true,
		Glamour:   true,
		Wish:      true,
		Log:       true,
		Harmonica: true,
	}
	got := p.AllLibs()
	want := []string{"cobra", "fang", "glamour", "harmonica", "huh", "log", "viper", "wish"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllLibs() = %v, want %v", got, want)
	}
}
