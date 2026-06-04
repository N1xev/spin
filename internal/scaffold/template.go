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

// charmLibInfoField is the field selector used by the charmLibInfo
// FuncMap helper. It dispatches on the `field` argument and returns the
// matching piece of the library metadata. The four metadata fields
// (display, module, purpose, extending, example) are passed positionally
// so the call site is a single switch case rather than a 5-way nested
// switch. Returns "" for unknown fields, matching the contract of
// charmPin (unknown names / fields return empty strings).
//
// The string fields are kept short and copy-pasteable: they appear
// verbatim in the generated AGENTS.md, and a long extending block
// would make the file noisy.
func charmLibInfoField(display, module, purpose, extending, example, field string) string {
	switch field {
	case "display":
		return display
	case "module":
		return module
	case "purpose":
		return purpose
	case "extending":
		return extending
	case "example":
		return example
	}
	return ""
}

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
	// p.Libs still contains names like "bubbletea", "bubbles", "lipgloss"
	// (set by ResolveFlags for the templates' has* predicates to read),
	// but those no longer have lib/<name>/ overlay directories in the
	// restructured tree. Filter against the overlay-walk set so we don't
	// add `lib/bubbletea` to the layer list and then silently skip it
	// when fs.WalkDir returns "file does not exist" — that mismatch made
	// TestOverlayOrder_TUI fail.
	overlayLibs := p.boolFlagOverlayMap()
	for _, lib := range p.Libs {
		if !overlayLibs[lib] {
			continue
		}
		layers = append(layers, "lib/"+lib)
		seen[lib] = true
	}
	// Walk the lib/<name>/ overlay for every active bool flag. Skip
	// entries that were already added via p.Libs (the bool may be
	// derived from the same flag).
	for lib, active := range overlayLibs {
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
//
// Plan 02-05 reduced this map to a single entry (glow). Plan 03-04
// adds the second surviving overlay: lib/ai/ (the AGENTS.md template
// gated on --ai). All other charm library wiring is inlined in the
// variant_*/internal/{app,cmd,ui,config}/*.go.tmpl files as
// `if has<Lib> .` blocks, so no lib/* overlay directory is needed for
// huh, wish, glamour, harmonica, bubbles, bubbletea, cobra, fang, log,
// lipgloss, viper, ansi, modifiers, or runewidth.
//
// For the parallel "is this lib selected?" predicate that includes
// non-overlay bools (Cobra, Fang, Viper, Huh, Log, etc.) see
// libBoolMap in project.go — AllLibs() uses that one.
func (p *Project) boolFlagOverlayMap() map[string]bool {
	return map[string]bool{
		"glow": p.Glow,
		"ai":   p.AI,
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
		// charmLibInfo returns a metadata string for a given library name
		// (one of the 15 keys in the UI-SPEC §"Library lookup table" — see
		// the template at templates/lib/ai/AGENTS.md.tmpl). The second
		// argument selects which field: "display", "module", "purpose",
		// "extending", or "example". Returns "" for unknown names or
		// fields, matching the charmPin contract.
		//
		// Module paths are pinned in code (not looked up from versions.go)
		// because they are the long-lived canonical paths documented in
		// CLAUDE.md and the AGENTS.md must remain stable across spin
		// versions. The version pin itself is what charmPin resolves; the
		// module path is a separate, more stable identifier.
		//
		// The 15 keys cover every library listed in UI-SPEC Surface B §
		// "Library lookup table (canonical)": bubbletea, bubbles,
		// lipgloss, huh, glamour, glow, wish, log, harmonica, cobra,
		// fang, viper, modifiers, ansi, runewidth.
		"charmLibInfo": func(name, field string) string {
			switch name {
			case "bubbletea":
				return charmLibInfoField("Bubble Tea", "charm.land/bubbletea/v2",
					"TUI framework, MVU runtime",
					"Add new messages to the model `Update` switch; v2 uses `View() tea.View` not `View() string`.",
					"m := model{}\np := tea.NewProgram(m)\nif _, err := p.Run(); err != nil {\n    fmt.Println(err)\n}", field)
			case "bubbles":
				return charmLibInfoField("Bubbles", "charm.land/bubbles/v2",
					"Pre-built TUI components",
					"Components are `tea.Model` implementations; compose with `lipgloss` for layout.",
					"ti := textinput.New()\nti.Placeholder = \"name\"\nti.Focus()", field)
			case "lipgloss":
				return charmLibInfoField("Lip Gloss", "charm.land/lipgloss/v2",
					"CSS-like terminal styling",
					"Use subpackages for tables/lists/trees; v2 is the supported line.",
					"style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(\"5\"))\nfmt.Println(style.Render(\"hi\"))", field)
			case "huh":
				return charmLibInfoField("Huh", "charm.land/huh/v2",
					"Accessible forms/prompts",
					"Forms compose Fields into Groups; submit via `form.Run()`.",
					"form := huh.NewForm(huh.NewGroup(huh.NewInput().Title(\"Name\")))\n_ = form.Run()", field)
			case "glamour":
				return charmLibInfoField("Glamour", "charm.land/glamour/v2",
					"Stylesheet-based markdown renderer",
					"Use `NewTermRenderer` for terminal, `Render` for plain string.",
					"r, _ := glamour.NewTermRenderer(glamour.WithStylePath(\"dark\"))\nout, _ := r.Render(md)", field)
			case "glow":
				return charmLibInfoField("Glow", "github.com/charmbracelet/glow/v2",
					"Markdown reader CLI",
					"Binary-only; shell out via `os/exec`.",
					"out, _ := exec.Command(\"glow\", \"README.md\").Output()\nfmt.Println(string(out))", field)
			case "wish":
				return charmLibInfoField("Wish", "charm.land/wish/v2",
					"SSH server framework",
					"Subpackages: `bubbletea`, `logging`, `activeterm`; wire as middleware.",
					"s, _ := wish.NewServer(wish.WithMiddleware(\n    bubbletea.Middleware(teaHandler),\n))", field)
			case "log":
				return charmLibInfoField("Log", "charm.land/log/v2",
					"Minimal colorful leveled logging",
					"`log.Default()` + `log.SetDefault(log.New(os.Stderr, log.Options{...}))`.",
					"log.Info(\"hi\", \"k\", \"v\")\nlog.Error(\"oops\", \"err\", err)", field)
			case "harmonica":
				return charmLibInfoField("Harmonica", "github.com/charmbracelet/harmonica",
					"Spring animations for the terminal",
					"Use `NewSpring` + `Update` per frame in `tea.Tick`.",
					"sp := harmonica.NewSpring(harmonica.FPS(60), 8.0, 0.5)\n_, _, _ = sp.Update(0.016, 0)", field)
			case "cobra":
				return charmLibInfoField("Cobra", "github.com/spf13/cobra",
					"CLI subcommand/flag framework",
					"Pair with fang for styled help; pin v1.9+.",
					"var rootCmd = &cobra.Command{Use: \"app\"}\nrootCmd.AddCommand(&cobra.Command{Use: \"hello\"})", field)
			case "fang":
				return charmLibInfoField("Fang", "charm.land/fang/v2",
					"Styled help + errors for Cobra",
					"Drop-in `fang.Execute(ctx, rootCmd)`; replaces cobra's default renderer.",
					"if err := fang.Execute(ctx, rootCmd); err != nil {\n    os.Exit(1)\n}", field)
			case "viper":
				return charmLibInfoField("Viper", "github.com/spf13/viper",
					"Config-file support",
					"Use `mapstructure` v2 fork; bind flags with `viper.BindPFlag`.",
					"viper.SetDefault(\"port\", 8080)\nport := viper.GetInt(\"port\")", field)
			case "modifiers":
				return charmLibInfoField("x/modifiers", "github.com/charmbracelet/x/modifiers",
					"Inert UI modifiers (e.g. inert)",
					"Use inside `tea.Update` to short-circuit input handling.",
					"if _, ok := msg.(modifiers.Inert); ok {\n    return m, nil\n}", field)
			case "ansi":
				return charmLibInfoField("x/ansi", "github.com/charmbracelet/x/ansi",
					"Low-level ANSI parser/generator",
					"Use for escape sequences lipgloss v2 doesn't cover.",
					"link := ansi.Hyperlink(\"https://example.com\", \"example\")\nfmt.Println(link)", field)
			case "runewidth":
				return charmLibInfoField("go-runewidth", "github.com/mattn/go-runewidth",
					"East-Asian-aware display width",
					"Set `RUNEWIDTH_EASTASIAN=true` env var for CJK terminals.",
					"w := runewidth.StringWidth(\"こんにちは\")\nfmt.Println(w) // 10", field)
			}
			return ""
		},
		// allLibs returns the project's full library set (p.Libs union
		// the bool-flag libs, sorted) so the AGENTS.md template can
		// iterate in a stable order. This is the same iteration order
		// the gum/huh prompt backend uses, so the AGENTS.md matches the
		// "what we asked the user to confirm" sequence. See
		// Project.AllLibs() in project.go for the implementation.
		"allLibs": func(p2 *Project) []string { return p2.AllLibs() },
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
