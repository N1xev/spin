package template

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

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

// writeFiles writes a rel-path → bytes map to a destination directory.
// Rejects any path that resolves outside dest (path-traversal guard).
func writeFiles(ctx context.Context, dest string, files map[string][]byte) error {
	cleanDest := filepath.Clean(dest) + string(filepath.Separator)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dest, err)
	}
	for rel, content := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
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
