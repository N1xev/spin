package scaffold

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestOverlayOrder_TUI asserts the simplest overlay order: --tui --bubbletea
// produces 3 layers in _base -> variant_tui -> lib/bubbletea order.
func TestOverlayOrder_TUI(t *testing.T) {
	p := &Project{Type: "tui", Libs: []string{"bubbletea"}}
	got := p.overlayOrder()
	want := []string{"_base", "variant_tui", "lib/bubbletea"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("overlayOrder = %v, want %v", got, want)
	}
}

// TestOverlayOrder_AllLibs asserts multiple libs produce deterministic,
// sorted layer order. The plan requires ResolveFlags to sort Libs; the test
// passes a pre-sorted slice to assert the engine's behavior.
func TestOverlayOrder_AllLibs(t *testing.T) {
	p := &Project{Type: "tui", Libs: []string{"bubbletea", "bubbles", "lipgloss"}}
	got := p.overlayOrder()
	want := []string{"_base", "variant_tui", "lib/bubbletea", "lib/bubbles", "lib/lipgloss"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("overlayOrder = %v, want %v", got, want)
	}
}

// TestOverlayOrder_NoType asserts that an empty p.Type still yields a
// valid _base + libs overlay (used for the Phase 3 --config-only mode).
func TestOverlayOrder_NoType(t *testing.T) {
	p := &Project{Type: "", Libs: []string{"bubbletea"}}
	got := p.overlayOrder()
	want := []string{"_base", "lib/bubbletea"}
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

// TestFuncMap_CharmPin asserts the verified v2 pins (RESEARCH §3).
func TestFuncMap_CharmPin(t *testing.T) {
	fm := funcMap(&Project{})
	cp := fm["charmPin"].(func(string) string)
	cases := map[string]string{
		"bubbletea": "v2.0.0",
		"lipgloss":  "v2.0.0-beta.2",
		"bubbles":   "v2.0.0",
		"log":       "v2.0.0",
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
// --lipgloss combination and asserts the rendered map contains all expected
// files with all expected content. This is Plan 03's main acceptance test
// for the template engine.
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

	// Required keys.
	for _, name := range []string{
		"go.mod", "main.go", "README.md", ".gitignore",
		".air.toml", "Taskfile.yml", "LICENSE",
		"internal/ui/styles.go",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing rendered file %q; got keys: %v", name, keysOf(files))
		}
	}

	// go.mod — conditional Go version + all 3 charm imports.
	goMod := files["go.mod"]
	for _, want := range []string{
		"module github.com/example/myapp",
		"go 1.25.0", // because --bubbles
		"charm.land/bubbletea/v2 v2.0.0",
		"charm.land/bubbles/v2 v2.0.0",
		"charm.land/lipgloss/v2 v2.0.0-beta.2",
	} {
		if !bytes.Contains(goMod, []byte(want)) {
			t.Errorf("go.mod missing %q; got:\n%s", want, goMod)
		}
	}

	// main.go — bubbletea v2 API.
	mainGo := files["main.go"]
	for _, want := range []string{
		"package main",
		"tea.NewProgram",
		"charm.land/bubbletea/v2",
	} {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("main.go missing %q; got:\n%s", want, mainGo)
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

// TestRenderToMap_GoVersion asserts the conditional go directive.
func TestRenderToMap_GoVersion(t *testing.T) {
	// --bubbletea only: 1.23 floor.
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea"}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	if !bytes.Contains(files["go.mod"], []byte("go 1.23")) {
		t.Errorf("expected go 1.23 without bubbles; got:\n%s", files["go.mod"])
	}

	// --bubbletea --bubbles: 1.25.0 floor.
	p.Libs = []string{"bubbletea", "bubbles"}
	files, err = p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	if !bytes.Contains(files["go.mod"], []byte("go 1.25.0")) {
		t.Errorf("expected go 1.25.0 with bubbles; got:\n%s", files["go.mod"])
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

// TestRenderToMap_NoLipgloss_NoStylesFile asserts styles.go is the no-op
// base when --lipgloss is not passed.
func TestRenderToMap_NoLipgloss_NoStylesFile(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "tui", Libs: []string{"bubbletea"}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	files, err := p.renderToMap()
	if err != nil {
		t.Fatalf("renderToMap: %v", err)
	}
	styles, ok := files["internal/ui/styles.go"]
	if !ok {
		t.Fatal("internal/ui/styles.go missing")
	}
	// No-op base contains a sentinel comment.
	if !bytes.Contains(styles, []byte("no-op default")) {
		t.Errorf("expected no-op styles.go (no --lipgloss); got:\n%s", styles)
	}
	if bytes.Contains(styles, []byte("lipgloss.NewStyle")) {
		t.Errorf("styles.go should not contain real lipgloss styles when --lipgloss absent; got:\n%s", styles)
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

// TestRenderToMap_TypeCLIRejected asserts --type=cli returns a Phase 2 error.
func TestRenderToMap_TypeCLIRejected(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "cli", Libs: []string{}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	_, err := p.renderToMap()
	if err == nil {
		t.Fatal("expected error for Type=cli")
	}
	if !strings.Contains(err.Error(), "Phase 2") {
		t.Errorf("expected 'Phase 2' in error; got: %v", err)
	}
}

// TestRenderToMap_TypeAllRejected asserts --type=all returns a Phase 2 error.
func TestRenderToMap_TypeAllRejected(t *testing.T) {
	p := &Project{Name: "x", Module: "x", Type: "all", Libs: []string{}, License: "none", Year: 2026, SpinVer: "0.1.0"}
	_, err := p.renderToMap()
	if err == nil {
		t.Fatal("expected error for Type=all")
	}
	if !strings.Contains(err.Error(), "Phase 2") {
		t.Errorf("expected 'Phase 2' in error; got: %v", err)
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

// TestRenderToMap_FullTUI_BuildsAndCompiles is the Plan 03 acceptance test:
// scaffold a project to a temp dir with all 3 libs, then `go build ./...`
// and `go test ./...` must exit 0. Skipped on -short.
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
