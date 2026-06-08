package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// renderFile renders a single .tmpl file with the given values.
func renderFile(path string, values map[string]any) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(path)).Funcs(funcMap()).Parse(string(b))
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
	return template.FuncMap{
		// Useful in templates
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
		"trim":  strings.TrimSpace,
		"join":  strings.Join,
		"default": func(d, v any) any {
			if v == nil || v == "" {
				return d
			}
			return v
		},
	}
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
