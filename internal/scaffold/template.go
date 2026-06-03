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
//
// In addition to the entries in p.Libs, the function also walks the
// lib/<name>/ overlay for every active bool flag (Cobra, Fang, Viper,
// Huh, Glamour, Glow, Wish, Log, Harmonica). Plan 02-03 introduced
// these as bools so the scaffolder didn't have to know the
// forward-compat flag set at ResolveFlags time, but the overlay
// engine still needs to walk their lib/<name>/ directories when
// the corresponding flag is set (e.g. lib/viper/internal/config/config.go.tmpl
// for --viper).
func (p *Project) overlayOrder() []string {
	layers := []string{"_base"}
	if p.Type != "" {
		layers = append(layers, "variant_"+p.Type)
	}
	seen := map[string]bool{}
	for _, lib := range p.Libs {
		layers = append(layers, "lib/"+lib)
		seen[lib] = true
	}
	// Walk the lib/<name>/ overlay for every active bool flag. Skip
	// entries that were already added via p.Libs (the bool may be
	// derived from the same flag).
	for lib, active := range p.boolFlagOverlayMap() {
		if !active || seen[lib] {
			continue
		}
		layers = append(layers, "lib/"+lib)
		seen[lib] = true
	}
	return layers
}

// boolFlagOverlayMap returns the set of lib overlay names that should
// be walked when the corresponding bool flag is set. Keys match the
// templates/lib/<name>/ directory names; values are the bool fields on
// Project. Kept in declaration order so the overlay walk is stable.
func (p *Project) boolFlagOverlayMap() map[string]bool {
	return map[string]bool{
		"cobra":     p.Cobra,
		"fang":      p.Fang,
		"viper":     p.Viper,
		"huh":       p.Huh,
		"glamour":   p.Glamour,
		"glow":      p.Glow,
		"wish":      p.Wish,
		"log":       p.Log,
		"harmonica": p.Harmonica,
	}
}

// renderToMap walks the embed FS in overlay order, renders every .tmpl
// against p (with the FuncMap), and returns a map of relative output path
// to rendered bytes. The .tmpl extension is stripped from output keys.
// Last-write-wins on identical relative output paths.
//
// Output-path placeholder substitution: after the .tmpl suffix is
// stripped, every occurrence of `_name_` in the relative output path is
// replaced with p.Name. This lets templates live at paths like
// `cmd/_name_/main.go.tmpl` and render to `cmd/myapp/main.go`. The
// underscore form is preferred over `<name>` because `<>` are valid in
// filenames but harder to type in editors; the walker convention is
// documented here so template authors can find it.
//
// License gating: LICENSE-<X>.tmpl files render only when License matches
// X. License="none" suppresses all LICENSE files.
func (p *Project) renderToMap() (map[string][]byte, error) {
	out := map[string][]byte{}
	fm := funcMap(p)
	fsys := currentFS(p.ExternalDir)

	// The embedded FS (//go:embed all:templates) starts at "templates/";
	// an external repo (os.DirFS(externalDir)) starts at the repo root.
	// The overlay layer names ("_base", "variant_tui", "lib/<lib>") are
	// the same in both cases — only the root prefix differs.
	rootPrefix := "templates/"
	if p.ExternalDir != "" {
		rootPrefix = ""
	}

	for _, layer := range p.overlayOrder() {
		root := rootPrefix + layer
		err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				// Missing layer directory is not fatal — variant_tui may be
				// absent for an empty project, etc. The Walking Skeleton
				// was the same way. Return nil to continue.
				//
				// The error wording differs by FS source: embed.FS uses
				// "file does not exist", os.DirFS results use the OS stat
				// wording ("no such file or directory"). Match both so
				// external repos (which typically have only _base/ and
				// no variant_tui) don't blow up.
				msg := walkErr.Error()
				if strings.Contains(msg, "file does not exist") ||
					strings.Contains(msg, "no such file or directory") {
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

			// Substitute the `_name_` placeholder in the output path with
			// p.Name. This is a PATH-level substitution (not a template
			// substitution) so the walker can locate templates at paths
			// like `cmd/_name_/main.go.tmpl` and emit `cmd/myapp/main.go`.
			// See the function comment for the rationale and the `_name_`
			// vs `<name>` choice.
			if p.Name != "" {
				outKey = strings.ReplaceAll(outKey, "_name_", p.Name)
			}

			// The active LICENSE file maps to the literal key "LICENSE" in
			// the output (not "LICENSE-mit" or "LICENSE-Apache-2.0"). The
			// file's stem encodes the license kind, but the user-facing
			// name in the scaffolded project is just "LICENSE".
			if strings.HasPrefix(name, "LICENSE-") {
				outKey = "LICENSE"
			}

			raw, err := fs.ReadFile(fsys, path)
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
		"hasHuh":       func(p2 *Project) bool { return p2.Huh },
		"hasGlamour":   func(p2 *Project) bool { return p2.Glamour },
		"hasGlow":      func(p2 *Project) bool { return p2.Glow },
		"hasWish":      func(p2 *Project) bool { return p2.Wish },
		"hasLog":       func(p2 *Project) bool { return p2.Log },
		"hasHarmonica": func(p2 *Project) bool { return p2.Harmonica },
		"hasViper":     func(p2 *Project) bool { return p2.Viper },
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
			case "viper":
				return DefaultPins.Viper
			case "harmonica":
				return DefaultLegacyPins.Harmonica
			case "glow":
				return DefaultLegacyPins.Glow
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
