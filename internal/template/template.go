// Package template handles external templates — git repos containing
// a spin.toml manifest, a _base/ tree of file overlays, and an
// optional _post/ hook. Templates are language-agnostic; the CLI
// resolves the user's params (via internal/params) and renders the
// overlays through Go's text/template engine.
package template

import (
	"fmt"
	"os"
	"path/filepath"
)

// Template is a loaded external template, ready to render.
type Template struct {
	Name        string   // dir name, e.g. "rust-cli"
	Source      string   // local path on disk (post-clone)
	Repo        string   // git URL, if any
	SpinToml    *SpinToml // parsed spin.toml
	BaseDir     string   // _base/ inside Source
	PostHookDir string   // _post/ inside Source (optional)
}

// Detect checks whether a directory contains a valid template.
// A valid template has spin.toml and _base/.
func Detect(dir string) (*Template, error) {
	stPath := filepath.Join(dir, "spin.toml")
	if _, err := os.Stat(stPath); err != nil {
		return nil, fmt.Errorf("template: spin.toml not found in %s", dir)
	}
	base := filepath.Join(dir, "_base")
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("template: _base/ not found in %s", dir)
	}
	st, err := ParseSpinToml(stPath)
	if err != nil {
		return nil, err
	}
	return &Template{
		Name:        filepath.Base(dir),
		Source:      dir,
		SpinToml:    st,
		BaseDir:     base,
		PostHookDir: filepath.Join(dir, "_post"),
	}, nil
}

// Render walks the template's _base/ tree, rendering each .tmpl file
// against the supplied values. The output is a rel-path → bytes map.
//
// values is the resolved param + flag map. Keys ending in `.Name` are
// also available as `.Name` for backwards compat with the existing
// scaffold package.
func (t *Template) Render(values map[string]any) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.Walk(t.BaseDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(t.BaseDir, path)
		rel = filepath.ToSlash(rel)
		if filepath.Ext(rel) != ".tmpl" {
			// copy non-templated files verbatim
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = b
			return nil
		}
		rendered, err := renderFile(path, values)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		out[stripTmplExt(rel)] = rendered
		return nil
	})
	return out, err
}

// RenderTo writes the rendered files to dest. Same path-traversal
// guard as scaffold.emit.
func (t *Template) RenderTo(dest string, values map[string]any) error {
	files, err := t.Render(values)
	if err != nil {
		return err
	}
	return writeFiles(dest, files)
}

// RenderToWithPost is the full v2.0 template pipeline:
//  1. Render the template to an in-memory file map.
//  2. Write the files to dest (path-traversal-safe via writeFiles).
//  3. Run the post-hook (if any) in dest.
//  4. Walk dest and delete every spin.toml file found (TPL-16:
//     "spin.toml is deleted from the output after a successful
//     render").
//
// Returns the first non-nil error encountered. The post-hook and
// the spin.toml deletion are best-effort cleanup operations: if
// the post-hook fails, the spin.toml deletion still runs.
func (t *Template) RenderToWithPost(dest string, values map[string]any) error {
	files, err := t.Render(values)
	if err != nil {
		return err
	}
	if err := writeFiles(dest, files); err != nil {
		return err
	}
	// Post-hook: best-effort. Even if it fails, we still attempt
	// to delete spin.toml from the output (TPL-16).
	hookErr := RunPostHook(t, values, dest)
	deleteErr := deleteSpinToml(dest)
	if hookErr != nil {
		return hookErr
	}
	return deleteErr
}

// deleteSpinToml walks dest and removes any file named spin.toml.
// TPL-16 specifies that the manifest must not end up in the user's
// project; a defensive walk handles the case where a template
// accidentally included a spin.toml in _base/ (instead of relying
// on the manifest never being rendered in the first place).
func deleteSpinToml(dest string) error {
	return filepath.Walk(dest, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "spin.toml" {
			return os.Remove(path)
		}
		return nil
	})
}

func stripTmplExt(p string) string {
	if len(p) > 5 && p[len(p)-5:] == ".tmpl" {
		return p[:len(p)-5]
	}
	return p
}
