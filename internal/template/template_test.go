package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/example/spin/internal/params"
)

// TestTemplate_RenderToWithPost_DeletesSpinToml verifies that
// RenderToWithPost removes spin.toml from the rendered output
// (TPL-16). The test writes a Template with an empty _base/ to
// avoid needing a real on-disk template.
func TestTemplate_RenderToWithPost_DeletesSpinToml(t *testing.T) {
	base := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a fake spin.toml into the destination (as if a
	// template's _base/ had included it).
	if err := os.WriteFile(filepath.Join(dest, "spin.toml"), []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Post:   PostHook{}, // empty -> no post-hook
		},
	}
	if err := tpl.RenderToWithPost(dest, map[string]any{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "spin.toml")); !os.IsNotExist(err) {
		t.Fatalf("spin.toml should be removed from output, stat err=%v", err)
	}
}

// TestTemplate_RenderToWithPost_NestedSpinToml verifies the
// defensive walk also removes spin.toml files in subdirectories.
func TestTemplate_RenderToWithPost_NestedSpinToml(t *testing.T) {
	base := t.TempDir()
	dest := t.TempDir()
	sub := filepath.Join(dest, "sub")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "spin.toml"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
		},
	}
	if err := tpl.RenderToWithPost(dest, map[string]any{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, "spin.toml")); !os.IsNotExist(err) {
		t.Fatalf("nested spin.toml should be removed, stat err=%v", err)
	}
}

// TestUnwrapValue verifies the primitive-extraction helper used by
// the template renderer and the post-hook.
func TestUnwrapValue(t *testing.T) {
	cases := []struct {
		name string
		in   params.Value
		want any
	}{
		{"string", params.Value{String: "hello"}, "hello"},
		{"int", params.Value{Int: 42}, 42},
		{"bool", params.Value{Bool: true}, true},
		{"list", params.Value{List: []string{"a", "b"}}, []string{"a", "b"}},
		{"empty", params.Value{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnwrapValue(c.in)
			// Use a type-aware comparison so []string matches
			// by content, not by identity (slices aren't
			// comparable with ==).
			if !equalAny(got, c.want) {
				t.Errorf("UnwrapValue(%+v) = %v (%T), want %v (%T)", c.in, got, got, c.want, c.want)
			}
		})
	}
}

func equalAny(a, b any) bool {
	switch ax := a.(type) {
	case string:
		bx, ok := b.(string)
		return ok && ax == bx
	case int:
		bx, ok := b.(int)
		return ok && ax == bx
	case bool:
		bx, ok := b.(bool)
		return ok && ax == bx
	case []string:
		bx, ok := b.([]string)
		if !ok || len(ax) != len(bx) {
			return false
		}
		for i := range ax {
			if ax[i] != bx[i] {
				return false
			}
		}
		return true
	}
	return a == b
}

// TestDefaultCacheDir_PrefersXDG verifies the loader's default
// cache directory is the XDG config dir (not the legacy .cache).
func TestDefaultCacheDir_PrefersXDG(t *testing.T) {
	got := defaultCacheDir()
	if !filepath.IsAbs(got) {
		t.Fatalf("defaultCacheDir should be absolute, got %q", got)
	}
	// Should contain "spin/templates" suffix.
	if filepath.Base(got) != "templates" {
		t.Errorf("defaultCacheDir suffix: got %q, want basename=templates", got)
	}
}
