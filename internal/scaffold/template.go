// Template engine. Composes three overlay layers in last-write-wins
// order:
//
//  1. templates/_base/         — every project gets these
//  2. templates/variant_<type>/ — variant (tui/cli/all) main.go etc.
//  3. templates/lib/<name>/     — per-library overlays (only lib/ai/
//     survives after the 260604-7jt glow/modifiers removal)
//
// Walking the embed with fs.WalkDir (not one-level recursion) is
// required for lib overlays that nest (lipgloss, ai). License
// gating is by filename: LICENSE-<active>.tmpl renders for the
// active license only; License="none" emits no LICENSE file.
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

// charmLibInfoField dispatches on the field name. Unknown fields
// return "" (matches charmPin contract).
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

// overlayOrder returns layer paths in walk order (lowest precedence
// first, last-write-wins last). Each bool-flag's lib/<name>/ is also
// walked when the flag is set (e.g. lib/ai/ for --ai).
func (p *Project) overlayOrder() []string {
	layers := []string{"_base"}
	if p.Type != "" {
		layers = append(layers, "variant_"+p.Type)
	}
	seen := map[string]bool{}
	// p.Libs still contains names like "bubbletea" (set by
	// ResolveFlags for the templates' has* predicates), but those
	// no longer have lib/<name>/ overlay directories in the
	// restructured tree. Filter against the overlay-walk set so we
	// don't add `lib/bubbletea` and silently skip it.
	overlayLibs := p.boolFlagOverlayMap()
	for _, lib := range p.Libs {
		if !overlayLibs[lib] {
			continue
		}
		layers = append(layers, "lib/"+lib)
		seen[lib] = true
	}
	for lib, active := range overlayLibs {
		if !active || seen[lib] {
			continue
		}
		layers = append(layers, "lib/"+lib)
		seen[lib] = true
	}
	return layers
}

// boolFlagOverlayMap returns the set of lib overlay names walked
// when the corresponding bool flag is set. Keys match
// templates/lib/<name>/ directory names. After the 260604-7jt
// removal only "ai" survives. All other charm wiring is inlined in
// the variant_*/internal/{app,cmd,ui,config}/*.go.tmpl files as
// `if has<Lib> .` blocks. For the parallel "is this lib selected?"
// predicate that includes non-overlay bools, see libBoolMap in
// project.go — AllLibs() uses that one.
func (p *Project) boolFlagOverlayMap() map[string]bool {
	return map[string]bool{
		"ai": p.AI,
	}
}

// renderToMap walks the embed FS in overlay order, renders every
// .tmpl against p, and returns rel-path → rendered bytes. Last-write-
// wins on identical output paths.
//
// `_name_` substitution: after the .tmpl suffix is stripped, every
// `_name_` in the rel path is replaced with p.Name. This lets
// `cmd/_name_/main.go.tmpl` render to `cmd/myapp/main.go`.
//
// License gating: LICENSE-<active>.tmpl renders only when License
// matches. License="none" suppresses all LICENSE files.
func (p *Project) renderToMap() (map[string][]byte, error) {
	out := map[string][]byte{}
	fm := funcMap(p)
	fsys := currentFS(p.ExternalDir)

	// embed.FS starts at "templates/"; os.DirFS(externalDir) starts
	// at the repo root. Layer names are the same in both cases;
	// only the root prefix differs.
	rootPrefix := "templates/"
	if p.ExternalDir != "" {
		rootPrefix = ""
	}

	for _, layer := range p.overlayOrder() {
		root := rootPrefix + layer
		err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				// Missing layer directory is not fatal (variant_tui
				// may be absent for an empty project). Match both
				// "file does not exist" (embed) and "no such file or
				// directory" (os.DirFS) so external repos with only
				// _base/ don't blow up.
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
			if strings.HasPrefix(name, "LICENSE-") {
				want := "LICENSE-" + p.License + ".tmpl"
				if !strings.EqualFold(name, want) {
					return nil
				}
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("rel %s: %w", path, err)
			}
			outKey := strings.TrimSuffix(filepath.ToSlash(rel), ".tmpl")
			if p.Name != "" {
				outKey = strings.ReplaceAll(outKey, "_name_", p.Name)
			}
			// Active LICENSE file maps to "LICENSE" in the output,
			// not "LICENSE-mit" / "LICENSE-Apache-2.0".
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

// funcMap wires p-bound helpers so templates can do
// `{{hasBubbles .}}` and `{{charmPin "bubbletea"}}` without passing
// p through every call site.
func funcMap(p *Project) template.FuncMap {
	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"upper": strings.ToUpper,
		"join":  func(parts []string, sep string) string { return strings.Join(parts, sep) },
		"quote": strconv.Quote,
		"has":   func(v string) bool { return slices.Contains(p.Libs, v) },
		"hasBubbles":   func(p2 *Project) bool { return slices.Contains(p2.Libs, "bubbles") },
		"hasBubbletea": func(p2 *Project) bool { return slices.Contains(p2.Libs, "bubbletea") },
		"hasLipgloss":  func(p2 *Project) bool { return slices.Contains(p2.Libs, "lipgloss") },
		"hasCobra":     func(p2 *Project) bool { return p2.Cobra },
		"hasFang":      func(p2 *Project) bool { return p2.Fang },
		"hasHuh":       func(p2 *Project) bool { return p2.Huh },
		"hasGlamour":   func(p2 *Project) bool { return p2.Glamour },
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
			}
			return ""
		},
		// charmLibInfo returns metadata for a library: "display",
		// "module", "purpose", "extending", or "example". The 13 keys
		// cover every library in UI-SPEC §Surface B "Library lookup
		// table (canonical)". Module paths are pinned in code (not
		// looked up from versions.go) because they are the long-
		// lived canonical paths; the version pin is what charmPin
		// resolves.
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
		// allLibs returns the project's full library set (p.Libs
		// union the bool-flag libs, sorted) so the AGENTS.md
		// template iterates in the same order the prompt backend
		// used.
		"allLibs": func(p2 *Project) []string { return p2.AllLibs() },
		"requiresImport": func(p2 *Project, lib string) bool {
			if slices.Contains(p2.Libs, lib) {
				return true
			}
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
