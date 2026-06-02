// Package scaffold: template engine.
//
// The engine composes three overlay layers in last-write-wins order:
//
//	1. templates/_base/         — scaffolding files that every project gets
//	                                 (go.mod, README, .air.toml, Taskfile.yml, LICENSE-*, .gitignore)
//	2. templates/variant_<type>/ — the project variant (tui, cli, all) main.go
//	3. templates/lib/<name>/     — per-library overlays (bubbletea, bubbles, lipgloss, ...)
//
// Each lib overlay is a DIRECTORY. Its contents map directly to the output
// tree (path-prefix `templates/lib/<name>/` stripped, `.tmpl` suffix
// stripped). This convention lets `templates/lib/lipgloss/internal/ui/styles.go.tmpl`
// overwrite the no-op `templates/_base/internal/ui/styles.go.tmpl` while
// staying in its proper subdirectory.
//
// Walking the embed with fs.WalkDir (not the one-level recursion the Walking
// Skeleton used) is required for lib overlays that nest (lipgloss).
//
// License gating is done by filename in the walker: files matching
// `LICENSE-<license>.tmpl` render for the active license only. With
// `License="none"`, no LICENSE is emitted at all.
package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"
)

// overlayOrder returns the layer paths in walk order (lowest precedence
// first, last-write-wins last). The result is independent of which .tmpl
// files exist — missing layers are silently skipped by the walker.
func (p *Project) overlayOrder() []string {
	layers := []string{"_base"}
	if p.Type != "" {
		layers = append(layers, "variant_"+p.Type)
	}
	for _, lib := range p.Libs {
		layers = append(layers, "lib/"+lib)
	}
	return layers
}

// renderToMap walks the embed FS in overlay order, renders every .tmpl
// against p (with the FuncMap), and returns a map of relative output path
// to rendered bytes. The .tmpl extension is stripped from output keys.
// Last-write-wins on identical relative output paths.
//
// Type validation: --type=cli and --type=all return a "Phase 2" error
// because their variant templates are placeholders that don't compile yet.
// License gating: LICENSE-<X>.tmpl files render only when License matches
// X. License="none" suppresses all LICENSE files.
func (p *Project) renderToMap() (map[string][]byte, error) {
	// Type validation per open decision §15.6. Phase 2 replaces this branch
	// with real template content for --cli and --all variants.
	if p.Type == "cli" || p.Type == "all" {
		return nil, fmt.Errorf(
			"--type=%s: this variant ships in Phase 2; use --tui --bubbletea (and optionally --bubbles, --lipgloss) instead",
			p.Type,
		)
	}

	out := map[string][]byte{}
	fm := funcMap(p)

	for _, layer := range p.overlayOrder() {
		root := "templates/" + layer
		err := fs.WalkDir(FS, root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				// Missing layer directory is not fatal — variant_tui may be
				// absent for an empty project, etc. The Walking Skeleton
				// was the same way. Return nil to continue.
				if strings.Contains(walkErr.Error(), "file does not exist") {
					return nil
				}
				return fmt.Errorf("walk %s: %w", path, walkErr)
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".tmpl") {
				return nil
			}

			// License gating: only render LICENSE-<active>.tmpl.
			// Comparison is case-insensitive so templates can be named
			// LICENSE-MIT.tmpl while p.License is the lowercase "mit".
			if strings.HasPrefix(name, "LICENSE-") {
				want := "LICENSE-" + p.License + ".tmpl"
				if !strings.EqualFold(name, want) {
					return nil
				}
			}

			// Compute the output key: strip the layer prefix and .tmpl suffix.
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("rel %s: %w", path, err)
			}
			outKey := strings.TrimSuffix(filepath.ToSlash(rel), ".tmpl")

			// The active LICENSE file maps to the literal key "LICENSE" in
			// the output (not "LICENSE-mit" or "LICENSE-Apache-2.0"). The
			// file's stem encodes the license kind, but the user-facing
			// name in the scaffolded project is just "LICENSE".
			if strings.HasPrefix(name, "LICENSE-") {
				outKey = "LICENSE"
			}

			raw, err := fs.ReadFile(FS, path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}

			// FuncMap must be registered before Parse (RESEARCH §4.2).
			t, err := template.New(filepath.Base(path)).Funcs(fm).Option("missingkey=error").Parse(string(raw))
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}

			var buf bytes.Buffer
			if err := t.Execute(&buf, p); err != nil {
				return fmt.Errorf("execute %s: %w", path, err)
			}
			out[outKey] = buf.Bytes()
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no templates rendered; did you write the template files?")
	}
	return out, nil
}

// funcMap returns the text/template FuncMap wired to p. Most helpers close
// over p so templates can do `{{hasBubbles .}}` and `{{charmPin "bubbletea"}}`
// without passing p through every call site. Plan 03's set: title, upper,
// join, quote, has* predicates, charmPin, requiresImport.
func funcMap(p *Project) template.FuncMap {
	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"upper":  strings.ToUpper,
		"join":   func(parts []string, sep string) string { return strings.Join(parts, sep) },
		"quote":  strconv.Quote,
		"has":    func(v string) bool { return slices.Contains(p.Libs, v) },
		"hasBubbles": func(p2 *Project) bool {
			return slices.Contains(p2.Libs, "bubbles")
		},
		"hasBubbletea": func(p2 *Project) bool {
			return slices.Contains(p2.Libs, "bubbletea")
		},
		"hasLipgloss": func(p2 *Project) bool {
			return slices.Contains(p2.Libs, "lipgloss")
		},
		"hasCobra": func(p2 *Project) bool { return p2.Cobra },
		"hasFang":  func(p2 *Project) bool { return p2.Fang },
		"charmPin": func(lib string) string {
			switch lib {
			case "bubbletea":
				return DefaultPins.Bubbletea
			case "lipgloss":
				return DefaultPins.Lipgloss
			case "bubbles":
				return DefaultPins.Bubbles
			case "log":
				return DefaultPins.Log
			case "huh":
				return DefaultPins.Huh
			case "glamour":
				return DefaultPins.Glamour
			case "wish":
				return DefaultPins.Wish
			case "fang":
				return DefaultPins.Fang
			default:
				return ""
			}
		},
		"requiresImport": func(p2 *Project, lib string) bool {
			if slices.Contains(p2.Libs, lib) {
				return true
			}
			// Forward-compat for Phase 2 bools.
			switch lib {
			case "cobra":
				return p2.Cobra
			case "fang":
				return p2.Fang
			case "viper":
				return p2.Viper
			case "huh":
				return p2.Huh
			case "log":
				return p2.Log
			}
			return false
		},
	}
}
