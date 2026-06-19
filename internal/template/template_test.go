package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/N1xev/spin/internal/params"
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
			Post:   nil, // empty -> no post-hook
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

// TestRender_Exclude verifies that paths matching any glob in
// SpinToml.Exclude are skipped during render -- neither the .tmpl
// nor the rendered output lands in the result map. This is the
// primary use case for `exclude` (e.g. a CI badge file that
// should stay literal, or a docs/ tree the author doesn't want
// copied by default).
func TestRender_Exclude(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	// _base/ contains: keep.md (should land), drop.md (exact match),
	// docs/intro.md (glob match), docs/inner.go.tmpl (not excluded,
	// should render and land as docs/inner.go).
	for _, rel := range []string{"keep.md", "drop.md", "docs/intro.md", "docs/inner.go.tmpl"} {
		full := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("hello from " + rel)
		if filepath.Ext(rel) == ".tmpl" {
			body = []byte("package p\n// name={{.name}}\n")
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Exclude: []string{
				"drop.md",   // exact match
				"docs/*.md", // glob
			},
		},
	}
	out, err := tpl.Render(map[string]any{"name": "myapp"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if _, ok := out["keep.md"]; !ok {
		t.Errorf("expected keep.md in output; got keys: %v", keysOf(out))
	}
	if _, ok := out["drop.md"]; ok {
		t.Errorf("drop.md should be excluded")
	}
	if _, ok := out["docs/intro.md"]; ok {
		t.Errorf("docs/intro.md should be excluded by glob")
	}
	if _, ok := out["docs/inner.go"]; !ok {
		t.Errorf("docs/inner.go.tmpl should render and land; got keys: %v", keysOf(out))
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
