package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// renderFile renders a single .tmpl file with the given values and
// funcs. Callers build funcs once per render pass and reuse it.
func renderFile(path string, values map[string]any, funcs template.FuncMap) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(path)).Funcs(funcs).Parse(string(b))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, values); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func funcMap() template.FuncMap {
	titleCaser := cases.Title(language.English)
	return template.FuncMap{
		// Useful in templates
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": titleCaser.String,
		"trim":  strings.TrimSpace,
		"join":  strings.Join,
		"default": func(d, v any) any {
			if v == nil || v == "" {
				return d
			}
			return v
		},
		// snake_case: "MyProject" -> "my_project".
		// Splits on case boundaries and word boundaries, joins
		// with underscores, lowercases.
		"snake_case": snakeCase,
		// kebab-case: "MyProject" -> "my-project". Go's text/template
		// requires function names to be valid identifiers, so we use
		// `kebab` (called as `{{ kebab "X" }}`).
		"kebab": func(s string) string {
			return strings.ReplaceAll(snakeCase(s), "_", "-")
		},
		// quote: shell-escapes s for use inside a `[[post]] run = "..."`.
		// Uses single-quote wrapping with the standard "'='"'" trick
		// so embedded single quotes are escaped correctly.
		"quote": shellQuote,
		// now: current time, formatted. No-arg -> RFC3339. With a
		// layout string (e.g. "2006") -> that layout.
		"now": func(layout string) string {
			if layout == "" {
				layout = time.RFC3339
			}
			return time.Now().UTC().Format(layout)
		},
		// contains: substring check. Useful in templates that want
		// to gate on a value (e.g. `{{ if contains .tags "rust" }}`).
		"contains": strings.Contains,
		// has: report whether a []string contains a value. Used by
		// [[include]] rules and conditional templates.
		"has": slices.Contains[[]string, string],
		// not_has: inverse of has.
		"not_has": func(list []string, item string) bool {
			return !slices.Contains(list, item)
		},
		// one_of: report whether a value equals any of the given strings.
		"one_of": func(v string, items ...string) bool {
			return slices.Contains(items, v)
		},
	}
}

var nonWordSplitter = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func snakeCase(s string) string {
	if s == "" {
		return ""
	}
	// Insert an underscore at every case boundary: "MyProject" -> "My_Project".
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && isUpper(r) && !isUpper(runes[i-1]) {
			b.WriteByte('_')
		}
		if i > 0 && isUpper(r) && i+1 < len(runes) && isLower(runes[i+1]) && isLower(runes[i-1]) {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(nonWordSplitter.ReplaceAllString(b.String(), "_"))
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }

func shellQuote(s string) string {
	// 'foo' -> 'foo'. Embedded single quotes become '"'"'.
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// WriteFiles writes a rel-path → bytes map to a destination directory.
// Rejects any path that resolves outside dest (path-traversal guard).
// Exported for callers (e.g. cmd/new_charm.go) that merge template
// files with ecosystem files before writing.
func WriteFiles(dest string, files map[string][]byte) error {
	return writeFiles(dest, files)
}

// writeFiles writes a rel-path → bytes map to a destination directory.
// Rejects any path that resolves outside dest (path-traversal guard).
func writeFiles(dest string, files map[string][]byte) error {
	cleanDest := filepath.Clean(dest) + string(filepath.Separator)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dest, err)
	}
	for rel, content := range files {
		full := filepath.Join(dest, rel)
		cleanFull := filepath.Clean(full)
		if !strings.HasPrefix(cleanFull+string(filepath.Separator), cleanDest) {
			return fmt.Errorf("path traversal: %q resolves outside %q", rel, dest)
		}
		if err := os.MkdirAll(filepath.Dir(cleanFull), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(cleanFull, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
