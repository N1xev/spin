package ecosystem

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// stubEco is an inline test stub that implements the Ecosystem
// interface. It exists in the test file (not the concrete
// internal/ecosystems/* packages) to avoid an import cycle: those
// packages import this one, so importing them here would break
// compilation. Only Matches/Name/FriendlyName are exercised; the
// other methods are stubs that satisfy the interface.
type stubEco struct {
	name     string
	marker   string // file that, when present in a dir, makes Matches true
	friendly string
}

func (s *stubEco) Name() string                       { return s.name }
func (s *stubEco) Description() string                { return "" }
func (s *stubEco) Version() string                    { return "test" }
func (s *stubEco) Flags() []Flag                      { return nil }
func (s *stubEco) Matches(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, s.marker))
	return err == nil
}
func (s *stubEco) FriendlyName() string            { return s.friendly }
func (s *stubEco) Validate(ctx Context) error       { return nil }
func (s *stubEco) Render(ctx Context) (map[string][]byte, error) {
	return nil, nil
}
func (s *stubEco) PostScaffold(ctx Context, dir string) error {
	return nil
}
func (s *stubEco) Tasks() map[string]string { return nil }

// Compile-time assertion that stubEco satisfies the Ecosystem
// interface. If the interface gains a new method, this line fails
// to compile and the stub must be updated.
var _ Ecosystem = (*stubEco)(nil)

// TestRegistry_Get_UnknownEcosystem verifies the error message
// for an unknown ecosystem name includes the supplied name AND the
// list of available ecosystems.
func TestRegistry_Get_UnknownEcosystem(t *testing.T) {
	r := NewRegistry()
	r.RegisterBuiltin(&stubEco{name: "alpha", friendly: "Alpha"})
	r.RegisterBuiltin(&stubEco{name: "beta", friendly: "Beta"})

	_, err := r.Get("madeup")
	if err == nil {
		t.Fatal("Get(madeup) should return an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "madeup") {
		t.Errorf("error should mention the missing name %q, got: %q", "madeup", msg)
	}
	if !strings.Contains(msg, "alpha") {
		t.Errorf("error should list available ecosystem %q, got: %q", "alpha", msg)
	}
	if !strings.Contains(msg, "beta") {
		t.Errorf("error should list available ecosystem %q, got: %q", "beta", msg)
	}
}

// TestRegistry_Get_KnownEcosystem verifies a registered ecosystem
// is returned by name.
func TestRegistry_Get_KnownEcosystem(t *testing.T) {
	r := NewRegistry()
	want := &stubEco{name: "alpha", friendly: "Alpha"}
	r.RegisterBuiltin(want)

	got, err := r.Get("alpha")
	if err != nil {
		t.Fatalf("Get(alpha): %v", err)
	}
	if got != want {
		t.Errorf("Get(alpha) = %v, want %v", got, want)
	}
}

// TestRegistry_Names_StableOrder verifies Names() returns the
// registered names in sorted (alphabetical) order, regardless of
// the order in which they were registered.
func TestRegistry_Names_StableOrder(t *testing.T) {
	r := NewRegistry()
	r.RegisterBuiltin(&stubEco{name: "zulu", friendly: "Zulu"})
	r.RegisterBuiltin(&stubEco{name: "alpha", friendly: "Alpha"})
	r.RegisterBuiltin(&stubEco{name: "mike", friendly: "Mike"})

	got := r.Names()
	want := []string{"alpha", "mike", "zulu"}
	if !equalStrings(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	// And the returned slice should already be sorted.
	if !sort.StringsAreSorted(got) {
		t.Errorf("Names() = %v is not sorted", got)
	}
}

// TestRegistry_Detect_MarkerBased verifies Detect() returns the
// first registered ecosystem whose Matches(dir) returns true.
// Uses two stub ecosystems with different file markers, then
// creates a tempdir containing one marker's file and asserts the
// matching stub is returned.
func TestRegistry_Detect_MarkerBased(t *testing.T) {
	alpha := &stubEco{name: "alpha", marker: "alpha.txt", friendly: "Alpha"}
	beta := &stubEco{name: "beta", marker: "beta.txt", friendly: "Beta"}
	r := NewRegistry()
	// Register beta FIRST so Detect has to walk past it; this
	// proves the matcher is consulted rather than a registration
	// order shortcut.
	r.RegisterBuiltin(beta)
	r.RegisterBuiltin(alpha)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("marker"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := r.Detect(dir)
	if !ok {
		t.Fatal("Detect should find beta (the dir contains beta.txt)")
	}
	if got.Name() != "beta" {
		t.Errorf("Detect returned %q, want %q", got.Name(), "beta")
	}
}

// TestRegistry_Detect_NoMatch verifies Detect returns ok=false when
// no ecosystem matches the directory.
func TestRegistry_Detect_NoMatch(t *testing.T) {
	r := NewRegistry()
	r.RegisterBuiltin(&stubEco{name: "alpha", marker: "alpha.txt"})

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := r.Detect(dir)
	if ok {
		t.Errorf("Detect should not find a match, got %v", got)
	}
}

// TestRegistry_All verifies All() returns every registered
// ecosystem in registration order (built-ins first, externals
// after).
func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	alpha := &stubEco{name: "alpha"}
	beta := &stubEco{name: "beta"}
	r.RegisterBuiltin(alpha)
	r.RegisterExternal(beta)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d, want 2", len(all))
	}
	if all[0].Name() != "alpha" || all[1].Name() != "beta" {
		t.Errorf("All() = [%s, %s], want [alpha, beta]", all[0].Name(), all[1].Name())
	}
}

// equalStrings is a small helper so the tests don't depend on
// reflect.DeepEqual (slices are not comparable with ==).
func equalStrings(a, b []string) bool {
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
