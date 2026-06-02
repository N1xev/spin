// Package scaffold implements the spin scaffolder.
//
// The Walking Skeleton (Task 1 / Task 2) ships the minimum: New() accepts a
// *Project, renders the embedded template tree, writes files to ./<name>/,
// and runs a post-scaffold `go build ./...` smoke test with CGO_ENABLED=0.
//
// Plan 03 expands this with the proper overlay engine (overlayOrder),
// FuncMap helpers, and the full lib overlay set.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"charm.land/log/v2"
)

// FS is the embedded template tree rooted at templates/.
//
// The all: prefix is required (RESEARCH §4.1) so that hidden files like
// .air.toml and .gitignore are included in the embed — a `*` glob would
// silently skip them.
//
// Walking Skeleton: the embed compiles even when the templates/ tree is
// empty (FS = empty embed.FS), so Task 2 (engine) and Task 3 (templates)
// can land independently. renderToMap returns an error if no templates
// are found at walk time.
//
//go:embed all:templates
var FS embed.FS

func init() {
	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{Level: log.InfoLevel}))
}

// New is the main scaffolder entrypoint for the Walking Skeleton.
//
// Steps:
//  1. Verify ./<name>/ does not exist (no --force in Walking Skeleton).
//  2. Call renderToMap(p) to render every .tmpl into a map[string][]byte.
//  3. Call emit(p, files) to write the files to ./<name>/.
//  4. Call verifyBuild(p) to run `go build ./...` with CGO_ENABLED=0.
//
// The walking/rendering logic is hardcoded for the Walking Skeleton's
// three-layer overlay (_base + variant_tui + lib/bubbletea). Plan 03
// replaces this with the proper overlayOrder() + FuncMap.
func New(p *Project) error {
	if p == nil || p.Name == "" {
		return fmt.Errorf("scaffold: project name is required")
	}

	// The existing-directory check moved to Project.Validate (Task 2 of
	// Plan 01-02). The cmd/new.go runNew now runs Validate before New, and
	// direct callers of New (tests, future internal callers) are expected
	// to run Validate first too. Keeping the check out of New avoids a
	// second, --force-blind error path.

	files, err := renderToMap(p)
	if err != nil {
		return fmt.Errorf("scaffold: render: %w", err)
	}

	if err := emit(p, files); err != nil {
		return fmt.Errorf("scaffold: emit: %w", err)
	}

	// Post-scaffold smoke test FIRST. A failing build must never be
	// committed to git (otherwise a broken scaffold would be the user's
	// first commit on a brand-new project).
	if err := p.VerifyBuild(); err != nil {
		return fmt.Errorf("scaffold: verify: %w", err)
	}

	// Git init + initial commit AFTER verify. Skip on --no-git.
	if err := p.GitInit(); err != nil {
		return fmt.Errorf("scaffold: git: %w", err)
	}

	return nil
}

// renderToMap walks the embedded template tree in three layers
// (_base, variant_<type>, lib/<name>) and renders every .tmpl file with
// text/template, returning a map of relative output path -> rendered bytes.
//
// The .tmpl extension is stripped from the output key. Last-write-wins on
// relative path: variant_tui overlays _base, lib overlays variant_tui.
//
// This helper is exported-via-test so scaffold_test.go can verify the
// render pipeline without touching the filesystem.
func renderToMap(p *Project) (map[string][]byte, error) {
	layers := overlayOrder(p)
	files := make(map[string][]byte)

	// Read each layer's directory entries from the embed FS.
	for _, layer := range layers {
		entries, err := fs.ReadDir(FS, layer)
		if err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "file does not exist") {
				// Layer directory not present in the embed (e.g. lib/foo
				// missing for a non-bubbletea project). Skip silently.
				continue
			}
			return nil, fmt.Errorf("read layer %q: %w", layer, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				// Recurse one level for subdirectories like internal/.
				subEntries, err := fs.ReadDir(FS, filepath.Join(layer, entry.Name()))
				if err != nil {
					return nil, fmt.Errorf("read subdir %q in layer %q: %w", entry.Name(), layer, err)
				}
				for _, sub := range subEntries {
					if sub.IsDir() {
						continue
					}
					name := sub.Name()
					if !strings.HasSuffix(name, ".tmpl") {
						continue
					}
					full := filepath.Join(layer, entry.Name(), name)
					out := filepath.ToSlash(filepath.Join(entry.Name(), strings.TrimSuffix(name, ".tmpl")))
					if err := renderOne(p, full, out, files); err != nil {
						return nil, err
					}
				}
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".tmpl") {
				continue
			}
			full := filepath.Join(layer, name)
			out := strings.TrimSuffix(name, ".tmpl")
			if err := renderOne(p, full, out, files); err != nil {
				return nil, err
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no templates found in embed; did you write the template files (Task 3)?")
	}

	return files, nil
}

// renderOne reads a single .tmpl from the embed FS, renders it with
// text/template using p as data, and writes the result to files[out].
func renderOne(p *Project, embedPath, out string, files map[string][]byte) error {
	raw, err := fs.ReadFile(FS, embedPath)
	if err != nil {
		return fmt.Errorf("read template %q: %w", embedPath, err)
	}

	t, err := template.New(filepath.Base(embedPath)).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return fmt.Errorf("parse template %q: %w", embedPath, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, p); err != nil {
		return fmt.Errorf("execute template %q: %w", embedPath, err)
	}

	files[out] = buf.Bytes()
	return nil
}

// overlayOrder returns the embed-relative layer paths in walk order
// (lowest precedence first, last-write-wins last).
//
// Walking Skeleton: hardcoded to [_base, variant_tui, lib/bubbletea].
// Plan 03 replaces with p.Type / p.Libs-driven dynamic ordering.
func overlayOrder(p *Project) []string {
	layers := []string{"templates/_base"}
	if p.Type != "" {
		layers = append(layers, "templates/variant_"+p.Type)
	}
	for _, lib := range p.Libs {
		layers = append(layers, "templates/lib/"+lib)
	}
	return layers
}

// emit writes the rendered files to ./<name>/ preserving relative paths.
// All files are written with 0644 perms. Plan 02 may add +x for shell
// scripts in Taskfile hooks.
func emit(p *Project, files map[string][]byte) error {
	root := filepath.Join(".", p.Name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", root, err)
	}

	for rel, content := range files {
		full := filepath.Join(root, rel)
		dir := filepath.Dir(full)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", dir, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", full, err)
		}
	}
	return nil
}
