package template

import "testing"

// TestMatchGlob_LeadingDoubleStar tests matchGlob with patterns that
// start with **, matching any leading path depth.
func TestMatchGlob_LeadingDoubleStar(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"**/*.go", "src/main.go", true},
		{"**/*.go", "a/b/c.go", true},
		{"**/*.go", "README.md", false},
		{"**", "anything/at/all", true},
	}
	for _, c := range cases {
		t.Run(c.pattern+"/"+c.name, func(t *testing.T) {
			got, err := matchGlob(c.pattern, c.name)
			if err != nil {
				t.Fatalf("matchGlob(%q, %q): %v", c.pattern, c.name, err)
			}
			if got != c.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
			}
		})
	}
}

// TestMatchGlob_MiddleDoubleStar tests matchGlob with patterns that
// have ** in the middle, matching any number of intermediate directories.
func TestMatchGlob_MiddleDoubleStar(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"src/**/test.go", "src/test.go", true},
		{"src/**/test.go", "src/a/b/test.go", true},
		{"src/**/test.go", "lib/test.go", false},
	}
	for _, c := range cases {
		t.Run(c.pattern+"/"+c.name, func(t *testing.T) {
			got, err := matchGlob(c.pattern, c.name)
			if err != nil {
				t.Fatalf("matchGlob(%q, %q): %v", c.pattern, c.name, err)
			}
			if got != c.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
			}
		})
	}
}

// TestMatchGlob_TrailingDoubleStar tests matchGlob with patterns that
// end with **, matching any path under the given prefix.
func TestMatchGlob_TrailingDoubleStar(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"docs/**", "docs/README.md", true},
		{"docs/**", "docs/a/b/c", true},
		{"docs/**", "src/docs/file", false},
	}
	for _, c := range cases {
		t.Run(c.pattern+"/"+c.name, func(t *testing.T) {
			got, err := matchGlob(c.pattern, c.name)
			if err != nil {
				t.Fatalf("matchGlob(%q, %q): %v", c.pattern, c.name, err)
			}
			if got != c.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
			}
		})
	}
}

// TestMatchGlob_NoDoubleStar tests matchGlob with plain glob patterns
// (no **) that fall back to filepath.Match.
func TestMatchGlob_NoDoubleStar(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", false},
	}
	for _, c := range cases {
		t.Run(c.pattern+"/"+c.name, func(t *testing.T) {
			got, err := matchGlob(c.pattern, c.name)
			if err != nil {
				t.Fatalf("matchGlob(%q, %q): %v", c.pattern, c.name, err)
			}
			if got != c.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
			}
		})
	}
}

// TestRenderBool verifies the renderBool helper that evaluates a Go
// template string and returns whether the result is truthy.
func TestRenderBool(t *testing.T) {
	cases := []struct {
		name string
		tpl  string
		vals map[string]any
		want bool
	}{
		{"true_literal", "true", nil, true},
		{"TRUE_literal", "TRUE", nil, true},
		{"one_literal", "1", nil, true},
		{"y_literal", "y", nil, true},
		{"Y_literal", "Y", nil, true},
		{"false_literal", "false", nil, false},
		{"zero_literal", "0", nil, false},
		{"empty_string", "", nil, false},
		{"random_string", "anything", nil, false},
		{"template_non_truthy", "{{.name}}", map[string]any{"name": "other"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := renderBool(c.tpl, c.vals, nil)
			if err != nil {
				t.Fatalf("renderBool(%q): %v", c.tpl, err)
			}
			if got != c.want {
				t.Errorf("renderBool(%q, %v) = %v, want %v", c.tpl, c.vals, got, c.want)
			}
		})
	}
}
