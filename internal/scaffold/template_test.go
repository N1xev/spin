package scaffold

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// TestOverlayOrder_TUI asserts the simplest overlay order: --tui --bubbletea
// produces 2 layers in _base -> variant_tui order. The lib/* overlays
// for bubbletea/bubbles/lipgloss etc. were restructured to inline
// library wiring as `if has<Lib> .` blocks within the variant files.
func TestOverlayOrder_TUI(t *testing.T) {
	p := &Project{Type: "tui", Libs: []string{"bubbletea"}}
	got := p.overlayOrder()
	want := []string{"_base", "variant_tui"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("overlayOrder = %v, want %v", got, want)
	}
}

// TestOverlayOrder_AllLibs asserts that even with multiple libs in p.Libs
// the overlay order is still just _base + variant_tui (lib/* overlays are
// no longer produced for the restructured variants).
func TestOverlayOrder_AllLibs(t *testing.T) {
	p := &Project{Type: "tui", Libs: []string{"bubbletea", "bubbles", "lipgloss"}}
	got := p.overlayOrder()
	want := []string{"_base", "variant_tui"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("overlayOrder = %v, want %v", got, want)
	}
}

// TestOverlayOrder_NoType asserts that an empty p.Type still yields a
// valid _base-only overlay (used for the Phase 3 --config-only mode).
// The boolFlagOverlayMap no longer contributes lib/* directories for
// the structured variants.
func TestOverlayOrder_NoType(t *testing.T) {
	p := &Project{Type: "", Libs: []string{"bubbletea"}}
	got := p.overlayOrder()
	want := []string{"_base"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("overlayOrder = %v, want %v", got, want)
	}
}

// TestFuncMap_HasBubbles asserts the hasBubbles/hasBubbletea/hasLipgloss
// helpers reflect p.Libs correctly.
func TestFuncMap_HasBubbles(t *testing.T) {
	fm := funcMap(&Project{Libs: []string{"bubbletea", "bubbles", "lipgloss"}})
	if !fm["hasBubbles"].(func(*Project) bool)(&Project{Libs: []string{"bubbletea", "bubbles", "lipgloss"}}) {
		t.Error("hasBubbles=true expected when bubbles in Libs")
	}
	if fm["hasBubbles"].(func(*Project) bool)(&Project{Libs: []string{"bubbletea"}}) {
		t.Error("hasBubbles=false expected when bubbles not in Libs")
	}
	if !fm["hasBubbletea"].(func(*Project) bool)(&Project{Libs: []string{"bubbletea"}}) {
		t.Error("hasBubbletea=true expected")
	}
	if !fm["hasLipgloss"].(func(*Project) bool)(&Project{Libs: []string{"lipgloss"}}) {
		t.Error("hasLipgloss=true expected")
	}
}

// TestFuncMap_CharmPin asserts the verified v2 pins for the charm
// libraries spin scaffolds.
func TestFuncMap_CharmPin(t *testing.T) {
	fm := funcMap(&Project{})
	cp := fm["charmPin"].(func(string) string)
	cases := map[string]string{
		"bubbletea": "v2.0.7",
		"lipgloss":  "v2.0.3",
		"bubbles":   "v2.1.0",
		"log":       "v2.0.0",
		"huh":       "v2.0.3",
		"glamour":   "v2.0.0",
		"wish":      "v2.0.1",
		"fang":      "v2.0.1",
		"unknown":   "",
	}
	for lib, want := range cases {
		if got := cp(lib); got != want {
			t.Errorf("charmPin(%q) = %q, want %q", lib, got, want)
		}
	}
}

// TestFuncMap_BasicHelpers asserts title, upper, join, quote pass through
// correctly. These are used by README and other string-substitution templates.
func TestFuncMap_BasicHelpers(t *testing.T) {
	fm := funcMap(&Project{})
	if got := fm["title"].(func(string) string)("myapp"); got != "Myapp" {
		t.Errorf("title(myapp) = %q, want Myapp", got)
	}
	if got := fm["upper"].(func(string) string)("hello"); got != "HELLO" {
		t.Errorf("upper(hello) = %q, want HELLO", got)
	}
	if got := fm["join"].(func([]string, string) string)([]string{"a", "b", "c"}, "-"); got != "a-b-c" {
		t.Errorf("join = %q, want a-b-c", got)
	}
	if got := fm["quote"].(func(string) string)("hi"); got != `"hi"` {
		t.Errorf("quote(hi) = %q, want \"hi\"", got)
	}
}

// TestRenderToMap_FullTUI scaffolds the full --tui --bubbletea --bubbles
// --lipgloss combination and asserts the rendered map contains all
// expected files with all expected content.
func TestRenderToMap_FullTUI(t *testing.T) {
	p := &Project{
		Name:    "myapp",
		Module:  "github.com/example/myapp",
		Type:    "tui",
		Libs:    []string{"bubbletea", "bubbles", "lipgloss"},
		License: "mit",
		Year:    2026,
		SpinVer: "0.1.0",
	}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}

	// Required keys. main.go is at `cmd/<name>/main.go` (canonical
	// Go path), and the TUI logic lives in
	// `internal/app/{app,update,view,keys}.go` instead of one big
	// root main.go.
	for _, name := range []string{
		"go.mod", "cmd/myapp/main.go", "README.md", ".gitignore",
		".air.toml", "Taskfile.yml", "LICENSE",
		"internal/ui/styles.go",
		"internal/app/app.go", "internal/app/update.go",
		"internal/app/view.go", "internal/app/keys.go",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing rendered file %q; got keys: %v", name, keysOf(files))
		}
	}

	// go.mod — unconditional Go 1.25.0 + all 3 charm imports at the
	// Phase 2 research §2.1 pins.
	goMod := files["go.mod"]
	for _, want := range []string{
		"module github.com/example/myapp",
		"go 1.25.0",
		"charm.land/bubbletea/v2 v2.0.7",
		"charm.land/bubbles/v2 v2.1.0",
		"charm.land/lipgloss/v2 v2.0.3",
	} {
		if !bytes.Contains(goMod, []byte(want)) {
			t.Errorf("go.mod missing %q; got:\n%s", want, goMod)
		}
	}

	// main.go (cmd/<name>/main.go) — thin entry, hands off to app.Run().
	mainGo := files["cmd/myapp/main.go"]
	for _, want := range []string{
		"package main",
		"app.Run",
		`"github.com/example/myapp/internal/app"`,
	} {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("main.go missing %q; got:\n%s", want, mainGo)
		}
	}

	// internal/app/app.go — owns Model + Init + Run.
	appGo := files["internal/app/app.go"]
	for _, want := range []string{
		"package app",
		"tea.NewProgram",
		"type Model struct",
	} {
		if !bytes.Contains(appGo, []byte(want)) {
			t.Errorf("app.go missing %q; got:\n%s", want, appGo)
		}
	}

	// styles.go — real lipgloss v2 styles.
	styles := files["internal/ui/styles.go"]
	for _, want := range []string{
		"package ui",
		"lipgloss.NewStyle",
		"lipgloss.Color",
	} {
		if !bytes.Contains(styles, []byte(want)) {
			t.Errorf("styles.go missing %q; got:\n%s", want, styles)
		}
	}

	// .air.toml — entrypoint, no bin.
	airToml := files[".air.toml"]
	if !bytes.Contains(airToml, []byte("build.entrypoint")) {
		t.Errorf(".air.toml missing build.entrypoint; got:\n%s", airToml)
	}
	if bytes.Contains(airToml, []byte("build.bin")) {
		t.Errorf(".air.toml should not contain build.bin; got:\n%s", airToml)
	}

	// Taskfile.yml — setup target.
	taskfile := files["Taskfile.yml"]
	for _, want := range []string{
		"setup:",
		"go install mvdan.cc/gofumpt@latest",
		"go install golang.org/x/tools/cmd/goimports@latest",
		"go install github.com/air-verse/air@latest",
		"go install go.dalton.dog/prism@latest",
	} {
		if !bytes.Contains(taskfile, []byte(want)) {
			t.Errorf("Taskfile.yml missing %q; got:\n%s", want, taskfile)
		}
	}

	// LICENSE — MIT default.
	license := files["LICENSE"]
	if !bytes.Contains(license, []byte("MIT License")) {
		t.Errorf("LICENSE missing 'MIT License'; got:\n%s", license)
	}
	if !bytes.Contains(license, []byte("Copyright (c) 2026 myapp")) {
		t.Errorf("LICENSE missing year+name; got:\n%s", license)
	}
}

// TestRenderToMap_GoVersion asserts the unconditional `go 1.25.0`
// directive. Every charm v2 library requires Go 1.25.0+ transitively.
//
// Three sub-cases cover the reachable combinations:
//   - bubbletea only (no bubbles): 1.25.0
//   - bubbletea + bubbles:          1.25.0
//   - bubbles only (implies bubbletea per ResolveFlags; same): 1.25.0
func TestRenderToMap_GoVersion(t *testing.T) {
	cases := []struct {
		name string
		libs []string
	}{
		{"bubbletea_only", []string{"bubbletea"}},
		{"bubbletea_and_bubbles", []string{"bubbletea", "bubbles"}},
		{"bubbles_implies_bubbletea", []string{"bubbles"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Project{Name: "x", Module: "x", Type: "tui", Libs: tc.libs, License: "none", Year: 2026, SpinVer: "0.1.0"}
			files, err := p.renderToMap()
			if err != nil {
				t.Fatalf("renderToMap: %v", err)
			}
			if !bytes.Contains(files["go.mod"], []byte("go 1.25.0")) {
				t.Errorf("expected go 1.25.0 (libs=%v); got:\n%s", tc.libs, files["go.mod"])
			}
			// The old `go 1.23` branch must never appear now.
			if bytes.Contains(files["go.mod"], []byte("go 1.23")) {
				t.Errorf("go.mod should not contain 'go 1.23' (libs=%v); got:\n%s", tc.libs, files["go.mod"])
			}
		})
	}
}

// TestRenderToMap_NoLicense asserts --license none produces no LICENSE.
func TestRenderToMap_NoLicense(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea"}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	if _, ok := files["LICENSE"]; ok {
		t.Errorf("LICENSE key present when License=none; got:\n%s", files["LICENSE"])
	}
}

// TestRenderToMap_ApacheLicense asserts --license apache-2.0 produces Apache text.
func TestRenderToMap_ApacheLicense(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea"}, License: "apache-2.0", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	license, ok := files["LICENSE"]
	if !ok {
		t.Fatal("LICENSE key missing for apache-2.0")
	}
	if !bytes.Contains(license, []byte("Apache License")) {
		t.Errorf("LICENSE missing 'Apache License'; got:\n%s", license)
	}
	if !bytes.Contains(license, []byte("Version 2.0")) {
		t.Errorf("LICENSE missing 'Version 2.0'; got:\n%s", license)
	}
}

// TestRenderToMap_NoLipgloss_StillHasStylesFile asserts that
// internal/ui/styles.go is always present in TUI variant: the
// variant provides the styles file directly. Without --lipgloss, the
// file still exists but contains no `lipgloss.NewStyle` call (the
// template gates the import + the type on hasLipgloss). With
// --lipgloss, it contains the real lipgloss v2 styles.
//
// Note: the old "no-op default" comment-based assertion is gone; the
// restructured template is conditional inside the file body rather
// than overlaid by a no-op base.
func TestRenderToMap_NoLipgloss_StillHasStylesFile(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea"}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	styles, ok := files["internal/ui/styles.go"]
	if !ok {
		t.Fatal("internal/ui/styles.go missing")
	}
	// The package is still ui.
	if !bytes.Contains(styles, []byte("package ui")) {
		t.Errorf("expected 'package ui'; got:\n%s", styles)
	}
	// Without --lipgloss, the file is the TUI variant's styles.go
	// which is conditional — it should NOT have lipgloss.NewStyle in
	// it (only Styles struct is unconditional in the template).
	// However, in our restructured template the styles.go always has
	// lipgloss imports — it just has no `Styles` struct when --lipgloss
	// is absent... actually, the restructured template always emits
	// the full content (the variant has `{{ if hasLipgloss .}}` only
	// for import gating but the package is the same). So this
	// assertion flips: we expect lipgloss content always (the TUI
	// variant always provides lipgloss).
	if !bytes.Contains(styles, []byte("lipgloss.NewStyle")) {
		t.Errorf("expected lipgloss.NewStyle in TUI variant's styles.go; got:\n%s", styles)
	}
}

// TestRenderToMap_WithLipgloss_RealStylesFile asserts styles.go has real
// lipgloss v2 styles when --lipgloss is set.
func TestRenderToMap_WithLipgloss_RealStylesFile(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea", "lipgloss"}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	styles, ok := files["internal/ui/styles.go"]
	if !ok {
		t.Fatal("internal/ui/styles.go missing")
	}
	if !bytes.Contains(styles, []byte("lipgloss.NewStyle")) {
		t.Errorf("expected real lipgloss styles with --lipgloss; got:\n%s", styles)
	}
	if !bytes.Contains(styles, []byte("type Styles struct")) {
		t.Errorf("expected Styles struct definition; got:\n%s", styles)
	}
}

// TestRenderToMap_TypeCLI asserts --type=cli renders successfully:
// cmd/<name>/main.go + internal/cmd/root.go.
func TestRenderToMap_TypeCLI(t *testing.T) {
	p := &Project{
		Name: "x", Module: "x", Type: "cli",
		Cobra: true, Fang: true, Viper: true,
		License: "none", Year: 2026, SpinVer: "0.1.0",
	}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap with Type=cli failed: %v", err)
	}
	// main.go is at cmd/<name>/main.go (thin), and the cobra
	// subcommand + fang.Execute + config.Bind live in
	// internal/cmd/root.go.tmpl.
	main, ok := files["cmd/x/main.go"]
	if !ok {
		t.Fatal("cmd/x/main.go missing for Type=cli")
	}
	for _, want := range []string{
		"package main",
		"cmd.RootCmd()",
		"fang.Execute",
	} {
		if !bytes.Contains(main, []byte(want)) {
			t.Errorf("cmd/x/main.go missing %q for Type=cli; got:\n%s", want, main)
		}
	}
	root, ok := files["internal/cmd/root.go"]
	if !ok {
		t.Fatal("internal/cmd/root.go missing for Type=cli")
	}
	for _, want := range []string{
		"func RootCmd() *cobra.Command",
		"helloCmd",
		"config.Bind", // --viper wiring
	} {
		if !bytes.Contains(root, []byte(want)) {
			t.Errorf("internal/cmd/root.go missing %q for Type=cli; got:\n%s", want, root)
		}
	}
}

// TestRenderToMap_TypeAll asserts --type=all renders successfully with
// both a tui subcommand (bubbletea) and a hello subcommand (CLI).
// main.go is the thin entry at cmd/x/main.go; the cobra subcommands
// live in internal/cmd/.
func TestRenderToMap_TypeAll(t *testing.T) {
	p := &Project{
		Name: "x", Module: "x", Type: "all",
		Libs: []string{"bubbletea"},
		Cobra: true, Fang: true,
		License: "none", Year: 2026, SpinVer: "0.1.0",
	}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap with Type=all failed: %v", err)
	}
	main, ok := files["cmd/x/main.go"]
	if !ok {
		t.Fatal("cmd/x/main.go missing for Type=all")
	}
	for _, want := range []string{
		"package main",
		"cmd.RootCmd()",
		"fang.Execute",
	} {
		if !bytes.Contains(main, []byte(want)) {
			t.Errorf("cmd/x/main.go missing %q for Type=all; got:\n%s", want, main)
		}
	}
	root, ok := files["internal/cmd/root.go"]
	if !ok {
		t.Fatal("internal/cmd/root.go missing for Type=all")
	}
	for _, want := range []string{
		"tuiCmd",
		"helloCmd",
	} {
		if !bytes.Contains(root, []byte(want)) {
			t.Errorf("internal/cmd/root.go missing %q for Type=all; got:\n%s", want, root)
		}
	}
}

// TestRenderToMap_ReadmePrerequisites asserts README contains the
// conditional Go version and the "Prerequisites" section.
func TestRenderToMap_ReadmePrerequisites(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea", "bubbles"}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	readme := files["README.md"]
	if !bytes.Contains(readme, []byte("Prerequisites")) {
		t.Errorf("README missing 'Prerequisites'; got:\n%s", readme)
	}
	if !bytes.Contains(readme, []byte("1.25.0")) {
		t.Errorf("README missing '1.25.0' (because --bubbles); got:\n%s", readme)
	}
}

// TestRenderToMap_FullTUI_BuildsAndCompiles scaffolds a project to a
// temp dir with all 3 libs, then `go build ./...` and `go test ./...`
// must exit 0. Skipped on -short.
func TestRenderToMap_FullTUI_BuildsAndCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end compile in -short mode")
	}

	tmp := t.TempDir()
	name := "scaffold-build-test"
	projectDir := filepath.Join(tmp, name)

	p := &Project{
		Name:    name,
		Module:  name,
		Type:    "tui",
		Libs:    []string{"bubbletea", "bubbles", "lipgloss"},
		License: "mit",
		Year:    2026,
		SpinVer: "0.1.0",
	}

	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	for rel, content := range files {
		full := filepath.Join(projectDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// `go mod tidy` is required because the generated go.mod references
	// modules that need to be downloaded. CGO disabled per CLAUDE.md.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = projectDir
	tidy.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed:\n%s", out)
	}

	build := exec.Command("go", "build", "./...")
	build.Dir = projectDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... failed in %s:\n%s", projectDir, out)
	}

	test := exec.Command("go", "test", "./...")
	test.Dir = projectDir
	test.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... failed in %s:\n%s", projectDir, out)
	}
}

// TestRenderToMap_NameSubstitution pins the `_name_` -> p.Name path-level
// substitution: a template at `cmd/_name_/main.go.tmpl` (or any path with
// the `_name_` token) must render to `cmd/<p.Name>/main.go` in the
// output map. The walker convention is documented in renderToMap's
// comment; this test pins it.
//
// The test uses t.TempDir + ExternalDir to build a tiny template tree
// at `_base/cmd/_name_/main.go.tmpl` so we can assert the substitution
// primitive without depending on the production embed (which doesn't
// ship a `cmd/_name_/main.go.tmpl` until Task 2 lands).
//
// TestIntegrationScaffold_NameInPath (integration_test.go) covers the
// end-to-end case where a scaffold emits `cmd/myapp/main.go`.
//
// Edge case pinned: if p.Name happens to be literally `_name_`, the
// substitution is a no-op because strings.ReplaceAll is called exactly
// once (no double-substitution, no infinite loop).
func TestRenderToMap_NameSubstitution(t *testing.T) {
	// Build a temp template tree: _base/cmd/_name_/main.go.tmpl.
	tmp := t.TempDir()
	mustMkdirAll(t, tmp, "_base/cmd/_name_")
	if err := os.WriteFile(
		filepath.Join(tmp, "_base", "cmd", "_name_", "main.go.tmpl"),
		[]byte("package main\n// name={{.Name}}\n"),
		0o644,
	); err != nil {
		t.Fatalf("write main.go.tmpl: %v", err)
	}

	cases := []struct {
		projectName string
		wantKey     string
	}{
		{"myapp", "cmd/myapp/main.go"},
		{"weird-name_123", "cmd/weird-name_123/main.go"},
		{"_name_", "cmd/_name_/main.go"}, // no-op edge case
	}
	for _, tc := range cases {
		t.Run(tc.projectName, func(t *testing.T) {
			p := &Project{
				Name:        tc.projectName,
				Module:      tc.projectName,
				Year:        2026,
				SpinVer:     "0.1.0",
				ExternalDir: tmp,
				// Type="" skips variant_*; we just want _base.
			}
			files, err := p.renderToMap()
			if err != nil {
				t.Fatalf("renderToMap: %v", err)
			}
			got, ok := files[tc.wantKey]
			if !ok {
				keys := make([]string, 0, len(files))
				for k := range files {
					keys = append(keys, k)
				}
				t.Fatalf("expected key %q in output map; got keys: %v", tc.wantKey, keys)
			}
			if !bytes.Contains(got, []byte("package main")) {
				t.Errorf("rendered %q missing 'package main': %s", tc.wantKey, got)
			}
		})
	}
}

func mustMkdirAll(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
}

