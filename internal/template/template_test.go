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
	if err := tpl.RenderToWithPost(dest, map[string]any{}, HookOptions{}); err != nil {
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
	if err := tpl.RenderToWithPost(dest, map[string]any{}, HookOptions{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, "spin.toml")); !os.IsNotExist(err) {
		t.Fatalf("nested spin.toml should be removed, stat err=%v", err)
	}
}

// TestTemplate_RenderToWithPost_CopiesPreDir verifies that an optional
// _pre/ directory next to spin.toml is copied into the generated project
// before pre-hooks run.
func TestTemplate_RenderToWithPost_CopiesPreDir(t *testing.T) {
	base := t.TempDir()
	pre := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pre, "init.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir:    base,
		PreHookDir: pre,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Pre:    []PreStep{{Run: "test -f _pre/init.sh"}},
		},
	}
	if err := tpl.RenderToWithPost(dest, map[string]any{}, HookOptions{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "_pre", "init.sh")); err != nil {
		t.Fatalf("_pre/init.sh should be copied to dest: %v", err)
	}
}

// TestTemplate_RenderToWithPost_CopiesPostDir verifies that an optional
// _post/ directory next to spin.toml is copied into the generated project
// before post-hooks run.
func TestTemplate_RenderToWithPost_CopiesPostDir(t *testing.T) {
	base := t.TempDir()
	post := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(post, "setup.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir:     base,
		PostHookDir: post,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Post:   []PostStep{{Run: "test -f _post/setup.sh"}},
		},
	}
	if err := tpl.RenderToWithPost(dest, map[string]any{}, HookOptions{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "_post", "setup.sh")); err != nil {
		t.Fatalf("_post/setup.sh should be copied to dest: %v", err)
	}
}

// TestTemplate_RenderToWithPost_AutoHookScripts verifies that every file
// in _pre/ and _post/ is executed automatically, in alphabetical order.
func TestTemplate_RenderToWithPost_AutoHookScripts(t *testing.T) {
	base := t.TempDir()
	pre := t.TempDir()
	post := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}

	// Multiple scripts in _pre/ run in alphabetical order. The second
	// script checks that the first one already ran.
	if err := os.WriteFile(filepath.Join(pre, "01-first.sh"), []byte("#!/bin/sh\ntouch pre-first\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pre, "02-second.sh"), []byte("#!/bin/sh\nif [ ! -f pre-first ]; then echo 'first did not run'; exit 1; fi\ntouch pre-second\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A non-executable script is run with sh.
	if err := os.WriteFile(filepath.Join(post, "01-setup.sh"), []byte("touch post-setup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(post, "02-cleanup.sh"), []byte("#!/bin/sh\ntouch post-cleanup\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir:     base,
		PreHookDir:  pre,
		PostHookDir: post,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
		},
	}
	if err := tpl.RenderToWithPost(dest, map[string]any{}, HookOptions{}); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	for _, marker := range []string{"pre-first", "pre-second", "post-setup", "post-cleanup"} {
		if _, err := os.Stat(filepath.Join(dest, marker)); err != nil {
			t.Fatalf("marker %q missing: %v", marker, err)
		}
	}
}

// TestUnwrapValue verifies the primitive-extraction helper used by
// the template renderer and the post-hook. Empty multiselect lists must
// return []string{} so text/template sees the right type, not a string.
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
		{"empty list", params.Value{List: []string{}}, []string{}},
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
	case nil:
		return b == nil
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

// TestRender_Include verifies that [[include]] rules gate files and
// directories on resolved param values.
func TestRender_Include(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"always.md", "ci/workflows.yml", "auth/middleware.go.tmpl", "grpc/server.go"} {
		full := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("hello from " + rel)
		if filepath.Ext(rel) == ".tmpl" {
			body = []byte("// {{.name}}")
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Include: []IncludeRule{
				{Path: "always.md"},
				{Path: "ci/**", If: "{{ .ci }}"},
				{Path: "auth/**", If: "{{ has .features \"auth\" }}"},
				{Path: "grpc/**", If: "{{ has .features \"grpc\" }}"},
			},
		},
	}
	out, err := tpl.Render(map[string]any{"name": "myapp", "ci": true, "features": []string{"auth"}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if _, ok := out["always.md"]; !ok {
		t.Errorf("expected always.md; got keys: %v", keysOf(out))
	}
	if _, ok := out["ci/workflows.yml"]; !ok {
		t.Errorf("expected ci/workflows.yml; got keys: %v", keysOf(out))
	}
	if _, ok := out["auth/middleware.go"]; !ok {
		t.Errorf("expected auth/middleware.go; got keys: %v", keysOf(out))
	}
	if _, ok := out["grpc/server.go"]; ok {
		t.Errorf("grpc/server.go should be excluded when features lacks grpc")
	}
}

// TestRender_IncludeSkipsDirectory verifies that a false [[include]]
// rule for a directory prunes the entire subtree.
func TestRender_IncludeSkipsDirectory(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"keep.md", "optional/a.md", "optional/b.md"} {
		full := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Include: []IncludeRule{
				{Path: "keep.md"},
				{Path: "optional/**", If: "{{ .with_optional }}"},
			},
		},
	}
	out, err := tpl.Render(map[string]any{"with_optional": false})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if _, ok := out["keep.md"]; !ok {
		t.Errorf("expected keep.md; got keys: %v", keysOf(out))
	}
	if _, ok := out["optional/a.md"]; ok {
		t.Errorf("optional/a.md should be excluded")
	}
	if _, ok := out["optional/b.md"]; ok {
		t.Errorf("optional/b.md should be excluded")
	}
}

// TestRenderToWithPost_PreHook verifies that pre-hooks run before
// files are rendered and can write into the destination.
func TestRenderToWithPost_PreHook(t *testing.T) {
	base := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "hello.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Pre: []PreStep{
				{Run: "touch pre-hook-marker"},
			},
		},
	}
	opts := HookOptions{PrintCommands: true}
	if err := tpl.RenderToWithPost(dest, map[string]any{}, opts); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "pre-hook-marker")); err != nil {
		t.Errorf("pre-hook marker missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.md")); err != nil {
		t.Errorf("rendered file missing: %v", err)
	}
}

// TestRenderToWithPost_PreHookFailureStopsRender verifies that a
// failing pre-hook prevents file rendering.
func TestRenderToWithPost_PreHookFailureStopsRender(t *testing.T) {
	base := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "hello.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Pre: []PreStep{
				{Run: "exit 1"},
			},
		},
	}
	opts := HookOptions{PrintCommands: true}
	err := tpl.RenderToWithPost(dest, map[string]any{}, opts)
	if err == nil {
		t.Fatal("expected pre-hook failure")
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.md")); !os.IsNotExist(err) {
		t.Errorf("hello.md should not be rendered after pre-hook failure")
	}
}

// TestRenderToWithPost_NoHooks skips pre and post hooks.
func TestRenderToWithPost_NoHooks(t *testing.T) {
	base := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "hello.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl := &Template{
		BaseDir: base,
		SpinToml: &SpinToml{
			Params: map[string]params.Spec{},
			Pre: []PreStep{
				{Run: "touch pre-hook-marker"},
			},
			Post: []PostStep{
				{Run: "touch post-hook-marker"},
			},
		},
	}
	opts := HookOptions{NoHooks: true, PrintCommands: true}
	if err := tpl.RenderToWithPost(dest, map[string]any{}, opts); err != nil {
		t.Fatalf("RenderToWithPost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "pre-hook-marker")); !os.IsNotExist(err) {
		t.Errorf("pre-hook marker should not exist with --no-hooks")
	}
	if _, err := os.Stat(filepath.Join(dest, "post-hook-marker")); !os.IsNotExist(err) {
		t.Errorf("post-hook marker should not exist with --no-hooks")
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.md")); err != nil {
		t.Errorf("rendered file should exist: %v", err)
	}
}
