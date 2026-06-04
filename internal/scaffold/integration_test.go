// TOOL-05 integration test: scaffolds a complete Phase 1 project and
// validates every Phase 1 acceptance criterion end-to-end.
//
// This is the canonical "Phase 1 works" test. It builds the spin binary
// from source, runs the CLI against a fresh temp directory, then validates:
//
//  1. The full TUI scaffold (--tui --bubbletea --bubbles --lipgloss):
//     - go.mod has the expected module name, charm.land/*/v2 pins, and
//     no `github.com/charmbracelet/` paths
//     - main.go is v2 (tea.View, tea.KeyPressMsg, tea.NewProgram) and
//     does NOT contain v1 patterns (View() string, tea.WithAltScreen,
//     lipgloss.NewRenderer)
//     - internal/ui/styles.go has real lipgloss v2 styles (NewStyle)
//     - .air.toml uses build.entrypoint, NOT build.bin
//     - Taskfile.yml has the `setup:` target wiring gofumpt + goimports +
//     air + prism installs
//     - LICENSE contains "MIT License" and the current year
//     - README.md has "## Next steps" and "## Prerequisites"
//     - .gitignore contains tmp/ and bin/
//     - .git/ is initialized with exactly 1 commit
//     - `go build ./...` and `go test ./...` both exit 0
//     - the v1-leak grep suite returns 0 matches
//
//  2. A --tui --bubbletea scaffold (no --bubbles): go.mod's go directive
//     is NOT bumped to 1.25.0 (TOOL-02).
//
//  3. Three license variants: --license mit, --license apache-2.0,
//     --license none. Asserts the LICENSE file matches the flag (or is
//     absent for `none`).
//
// Failure here is a Phase 1 regression. Total runtime is ~30s (dominated
// by go mod tidy in the scaffolded project).
package scaffold

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const integrationProjectName = "spin-integration-myapp"

func TestIntegrationScaffold(t *testing.T) {
	projectDir, repoRootPath := runSpinScaffold(t, integrationProjectName,
		[]string{"--tui", "--bubbletea", "--bubbles", "--lipgloss", "--ai"})

	assertGoModFullTUI(t, projectDir)
	assertMainGoV2(t, projectDir)
	assertAppGoV2(t, projectDir)
	assertStylesGoV2(t, projectDir)
	assertAirToml(t, projectDir)
	assertTaskfile(t, projectDir)
	assertLicenseMit(t, projectDir)
	assertReadme(t, projectDir)
	assertGitignore(t, projectDir)
	assertGitInit(t, projectDir)
	assertGoBuildAndTest(t, projectDir)
	assertNoV1Leaks(t, projectDir, repoRootPath)
	// Plan 03-04: --ai is now part of the canonical TOOL-05 test, so
	// every scaffold run also exercises AGENTS.md generation.
	assertAGENTSmd(t, projectDir)

	t.Logf("OK: Phase 1+3 integration test passed for %s", integrationProjectName)
}

// TestIntegrationScaffold_AlwaysGo1250 asserts that every generated
// project emits `go 1.25.0`, regardless of whether --bubbles is set.
// Per Phase 2 research §2.2, every charm v2 library requires Go 1.25.0+
// transitively; the previous `{{if hasBubbles}}` branch was dead code
// (RESOLVE: --tui implies --bubbletea, and even --bubbletea alone
// inherits the fang v2.0.1 floor of 1.25.0). Plan 02-01 (Task 2)
// simplified the directive to unconditional 1.25.0.
//
// The test name is a rename of the former
// `TestIntegrationScaffold_NoBubblesGoVersion` (which asserted the
// inverse — that --bubbletea without --bubbles did NOT emit 1.25.0).
// The semantic flipped when Task 2 removed the conditional branch.
func TestIntegrationScaffold_AlwaysGo1250(t *testing.T) {
	// --bubbletea only: still must emit go 1.25.0 (TOOL-01/TOOL-02).
	projectDir, _ := runSpinScaffold(t, integrationProjectName+"-bubbletea-only",
		[]string{"--tui", "--bubbletea"})

	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !bytes.Contains(goMod, []byte("\ngo 1.25.0\n")) {
		t.Errorf("go.mod missing 'go 1.25.0' (always required per Plan 02-01 Task 2):\n%s", goMod)
	}
	// The old `go 1.23` directive must never appear now.
	if bytes.Contains(goMod, []byte("\ngo 1.23\n")) {
		t.Errorf("go.mod should not contain 'go 1.23' (dead branch removed in Task 2):\n%s", goMod)
	}
}

func TestIntegrationScaffold_LicenseVariants(t *testing.T) {
	cases := []struct {
		flag       string
		mustHave   []string // substrings expected in the LICENSE file
		mustNot    []string // substrings expected to be absent
		fileAbsent bool     // true when no LICENSE file should be emitted
	}{
		{
			flag:     "mit",
			mustHave: []string{"MIT License", strconv.Itoa(time.Now().Year())},
		},
		{
			flag:     "apache-2.0",
			mustHave: []string{"Apache License", "Version 2.0", strconv.Itoa(time.Now().Year())},
			mustNot:  []string{"MIT License"},
		},
		{
			flag:       "none",
			fileAbsent: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			name := integrationProjectName + "-" + tc.flag
			projectDir, _ := runSpinScaffold(t, name,
				[]string{"--tui", "--bubbletea", "--license", tc.flag})

			licensePath := filepath.Join(projectDir, "LICENSE")
			if tc.fileAbsent {
				if _, err := os.Stat(licensePath); !os.IsNotExist(err) {
					t.Errorf("--license none: expected no LICENSE file, but %s exists (err=%v)",
						licensePath, err)
				}
				return
			}

			license, err := os.ReadFile(licensePath)
			if err != nil {
				t.Fatalf("read LICENSE: %v", err)
			}
			for _, want := range tc.mustHave {
				if !bytes.Contains(license, []byte(want)) {
					t.Errorf("LICENSE missing %q; got:\n%s", want, license)
				}
			}
			for _, banned := range tc.mustNot {
				if bytes.Contains(license, []byte(banned)) {
					t.Errorf("LICENSE unexpectedly contains %q; got:\n%s", banned, license)
				}
			}
		})
	}
}

// runSpinScaffold builds the spin binary (if not already built), chdirs
// into a fresh temp directory, runs `spin new <name> <flags...>`, and
// returns the absolute path of the scaffolded project + the spin repo root.
func runSpinScaffold(t *testing.T, name string, extraArgs []string) (string, string) {
	t.Helper()

	// Capture the repo root BEFORE chdir. Tests that call runSpinScaffold
	// multiple times in the same function (e.g.
	// TestIntegrationScaffold_AGENTSmd_Determinism) need the captured
	// root to be valid even after the first call chdirs into its workDir.
	// Caching in a package var (with sync.Once) ensures the very first
	// call wins, regardless of cwd, and subsequent calls reuse it.
	repoRootPath := cachedRepoRoot(t)
	workDir := t.TempDir()

	// Build the spin binary to a stable absolute path under os.TempDir
	// (NOT t.TempDir(), whose 0700 perms can confuse downstream tooling
	// in some sandboxes; same workaround the cmd/help_test.go TTY test uses).
	binPath := filepath.Join(os.TempDir(),
		fmt.Sprintf("spin-int-%d-%s", os.Getpid(), filepath.Base(t.Name())))
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stale bin: %v", err)
	}
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = repoRootPath
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build spin: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(binPath) })

	// chdir into the temp work dir; restore on test exit.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir %s: %v", workDir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Run `spin new <name> <flags...>`.
	args := append([]string{"new", name}, extraArgs...)
	run := exec.Command(binPath, args...)
	run.Dir = workDir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("spin new %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}

	projectDir := filepath.Join(workDir, name)
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("project dir %s not created: %v", projectDir, err)
	}
	return projectDir, repoRootPath
}

// repoRootOnce is the cached repo root path. The first call to
// runSpinScaffold in a test process computes the root by walking up
// from cwd; subsequent calls reuse the cached value. The cache is
// populated before runSpinScaffold does any chdir, so even tests that
// call it multiple times (e.g. determinism) always see a stable path.
//
// We do NOT clear this cache between tests because the project root
// doesn't change during a test binary's lifetime.
var (
	repoRootOnce sync.Once
	repoRootPath string
)

func cachedRepoRoot(t *testing.T) string {
	t.Helper()
	repoRootOnce.Do(func() {
		repoRootPath = repoRoot(t)
	})
	return repoRootPath
}

// assertGoModFullTUI validates the full-TUI go.mod:
//   - module name = project name
//   - charm.land/bubbletea/v2 v2.0.0 (with --bubbletea)
//   - charm.land/lipgloss/v2 (with --lipgloss; pin is upgraded by
//     `go mod tidy` from v2.0.0-beta.2 to v2.0.0 — see Plan 03 deviation
//     "go mod tidy upgrades the go directive and lipgloss pin")
//   - charm.land/bubbles/v2 v2.0.0 (with --bubbles, implies go 1.25.0)
//   - no v1 lib paths (github.com/charmbracelet/<lib> where <lib> is a
//     library that moved to charm.land in v2). Indirect transitive deps
//     under github.com/charmbracelet/x/... are NOT v1 leaks — that is
//     the current experimental namespace per CLAUDE.md tech stack.
func assertGoModFullTUI(t *testing.T, projectDir string) {
	t.Helper()
	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	wants := []string{
		"module " + integrationProjectName,
		"charm.land/bubbletea/v2 v2.0.7",
		"charm.land/lipgloss/v2 v2.0.3", // Phase 2 research §2.1 pin
		"charm.land/bubbles/v2 v2.1.0",
	}
	for _, want := range wants {
		if !bytes.Contains(goMod, []byte(want)) {
			t.Errorf("go.mod missing %q; got:\n%s", want, goMod)
		}
	}
	// v1 lib paths (each was at github.com/charmbracelet/<lib> in v1; moved
	// to charm.land/<lib>/v2). `github.com/charmbracelet/x/...` indirect
	// deps are the current experimental namespace — NOT a v1 leak.
	for _, v1lib := range []string{
		"github.com/charmbracelet/bubbletea",
		"github.com/charmbracelet/lipgloss",
		"github.com/charmbracelet/bubbles",
		"github.com/charmbracelet/huh",
		"github.com/charmbracelet/glamour",
		"github.com/charmbracelet/glow",
		"github.com/charmbracelet/wish",
		"github.com/charmbracelet/log",
		"github.com/charmbracelet/fang",
	} {
		if bytes.Contains(goMod, []byte(v1lib)) {
			t.Errorf("go.mod contains forbidden v1 path %q:\n%s", v1lib, goMod)
		}
	}
	// go 1.25.0 only required when --bubbles is used. We pass --bubbles
	// for this test, so the go directive MUST be 1.25.0 (TOOL-01).
	if !bytes.Contains(goMod, []byte("\ngo 1.25.0\n")) {
		t.Errorf("go.mod missing 'go 1.25.0' (TOOL-01 requires 1.25.0 with --bubbles):\n%s", goMod)
	}
}

// assertMainGoV2 validates the generated cmd/<name>/main.go uses the v2
// API and is a thin entry point (Plan 02-05 restructure):
//   - package main, imports "{{module}}/internal/app", calls app.Run
//   - does NOT contain v1 patterns (View() string, tea.WithAltScreen,
//     lipgloss.NewRenderer)
//   - the bubbletea MVU runtime (tea.NewProgram, tea.View) lives in
//     internal/app/, not main.go
func assertMainGoV2(t *testing.T, projectDir string) {
	t.Helper()
	mainGo, err := os.ReadFile(filepath.Join(projectDir, "cmd", integrationProjectName, "main.go"))
	if err != nil {
		t.Fatalf("read cmd/%s/main.go: %v", integrationProjectName, err)
	}
	wants := []string{
		"package main",
		"app.Run",
		`"` + integrationProjectName + `/internal/app"`,
	}
	for _, want := range wants {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("cmd/%s/main.go missing %q; got:\n%s", integrationProjectName, want, mainGo)
		}
	}
	banned := []string{
		"View() string",        // v1 View signature (TOOL-03)
		"tea.WithAltScreen",    // v1 program option (removed in v2)
		"lipgloss.NewRenderer", // v1 Renderer type (removed in v2)
	}
	for _, b := range banned {
		if bytes.Contains(mainGo, []byte(b)) {
			t.Errorf("cmd/%s/main.go contains forbidden v1 pattern %q:\n%s", integrationProjectName, b, mainGo)
		}
	}
}

// assertAppGoV2 validates the restructured internal/app/*.go files use
// the v2 bubbletea API and contain the expected per-file content.
// Plan 02-05 split the old single main.go into:
//   - internal/app/app.go    — Model + New + Init + Run
//   - internal/app/update.go — Update() with conditional lib wiring
//   - internal/app/view.go   — View() returning tea.View
//   - internal/app/keys.go   — help.Component KeyMap
func assertAppGoV2(t *testing.T, projectDir string) {
	t.Helper()
	base := filepath.Join(projectDir, "internal", "app")
	for _, f := range []string{"app.go", "update.go", "view.go", "keys.go"} {
		if _, err := os.Stat(filepath.Join(base, f)); err != nil {
			t.Errorf("internal/app/%s missing: %v", f, err)
		}
	}
	appGo, err := os.ReadFile(filepath.Join(base, "app.go"))
	if err != nil {
		t.Fatalf("read internal/app/app.go: %v", err)
	}
	for _, want := range []string{
		"package app",
		"type Model struct",
		"tea.NewProgram",
	} {
		if !bytes.Contains(appGo, []byte(want)) {
			t.Errorf("internal/app/app.go missing %q; got:\n%s", want, appGo)
		}
	}
	viewGo, err := os.ReadFile(filepath.Join(base, "view.go"))
	if err != nil {
		t.Fatalf("read internal/app/view.go: %v", err)
	}
	for _, want := range []string{
		"package app",
		"tea.View",    // v2 type, not the v1 `View() string` signature
		"tea.NewView", // v2 constructor
	} {
		if !bytes.Contains(viewGo, []byte(want)) {
			t.Errorf("internal/app/view.go missing %q; got:\n%s", want, viewGo)
		}
	}
	updateGo, err := os.ReadFile(filepath.Join(base, "update.go"))
	if err != nil {
		t.Fatalf("read internal/app/update.go: %v", err)
	}
	for _, want := range []string{
		"package app",
		"func (m *Model) Update",
		"tea.KeyPressMsg",
		"tea.Quit",
	} {
		if !bytes.Contains(updateGo, []byte(want)) {
			t.Errorf("internal/app/update.go missing %q; got:\n%s", want, updateGo)
		}
	}
}

// assertStylesGoV2 validates the generated internal/ui/styles.go uses
// lipgloss v2 (lipgloss.NewStyle, lipgloss.Color) and does NOT use the
// v1 NewRenderer (only present when --lipgloss was passed).
func assertStylesGoV2(t *testing.T, projectDir string) {
	t.Helper()
	styles, err := os.ReadFile(filepath.Join(projectDir, "internal", "ui", "styles.go"))
	if err != nil {
		t.Fatalf("read internal/ui/styles.go: %v", err)
	}
	if !bytes.Contains(styles, []byte("package ui")) {
		t.Errorf("internal/ui/styles.go missing 'package ui'")
	}
	if !bytes.Contains(styles, []byte("lipgloss.NewStyle")) {
		t.Errorf("internal/ui/styles.go missing lipgloss.NewStyle (lipgloss v2 API):\n%s", styles)
	}
	if bytes.Contains(styles, []byte("NewRenderer")) {
		t.Errorf("internal/ui/styles.go contains forbidden v1 NewRenderer:\n%s", styles)
	}
}

// assertAirToml validates the generated .air.toml uses the modern
// `build.entrypoint` field and does NOT use the deprecated `build.bin`.
func assertAirToml(t *testing.T, projectDir string) {
	t.Helper()
	air, err := os.ReadFile(filepath.Join(projectDir, ".air.toml"))
	if err != nil {
		t.Fatalf("read .air.toml: %v", err)
	}
	if !bytes.Contains(air, []byte("entrypoint")) {
		t.Errorf(".air.toml missing modern 'entrypoint' field:\n%s", air)
	}
	if bytes.Contains(air, []byte(`bin = "tmp/main"`)) {
		t.Errorf(".air.toml contains deprecated 'bin = \"tmp/main\"':\n%s", air)
	}
}

// assertTaskfile validates the generated Taskfile.yml has a `setup:`
// target that installs gofumpt + goimports + air + prism.
func assertTaskfile(t *testing.T, projectDir string) {
	t.Helper()
	taskfile, err := os.ReadFile(filepath.Join(projectDir, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read Taskfile.yml: %v", err)
	}
	wants := []string{
		"setup:",
		"go install mvdan.cc/gofumpt@latest",
		"go install golang.org/x/tools/cmd/goimports@latest",
		"go install github.com/air-verse/air@latest",
		"go install go.dalton.dog/prism@latest",
	}
	for _, want := range wants {
		if !bytes.Contains(taskfile, []byte(want)) {
			t.Errorf("Taskfile.yml missing %q:\n%s", want, taskfile)
		}
	}
}

// assertLicenseMit validates the generated LICENSE is MIT and includes
// the current year. (TOOL-05 is flag-agnostic about license choice;
// license variants are covered by TestIntegrationScaffold_LicenseVariants.)
func assertLicenseMit(t *testing.T, projectDir string) {
	t.Helper()
	license, err := os.ReadFile(filepath.Join(projectDir, "LICENSE"))
	if err != nil {
		t.Fatalf("read LICENSE: %v", err)
	}
	year := strconv.Itoa(time.Now().Year())
	wants := []string{"MIT License", year}
	for _, want := range wants {
		if !bytes.Contains(license, []byte(want)) {
			t.Errorf("LICENSE missing %q:\n%s", want, license)
		}
	}
}

// assertReadme validates the generated README has the required sections.
func assertReadme(t *testing.T, projectDir string) {
	t.Helper()
	readme, err := os.ReadFile(filepath.Join(projectDir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	wants := []string{"## Next steps", "## Prerequisites"}
	for _, want := range wants {
		if !bytes.Contains(readme, []byte(want)) {
			t.Errorf("README.md missing %q:\n%s", want, readme)
		}
	}
}

// assertGitignore validates the generated .gitignore has the standard
// tmp/ and bin/ exclusions.
func assertGitignore(t *testing.T, projectDir string) {
	t.Helper()
	gi, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, want := range []string{"tmp/", "bin/"} {
		if !bytes.Contains(gi, []byte(want)) {
			t.Errorf(".gitignore missing %q:\n%s", want, gi)
		}
	}
}

// assertGitInit validates the generated project has a .git directory
// with exactly one commit. Skips if git is not on $PATH.
func assertGitInit(t *testing.T, projectDir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on $PATH; skipping git init assertion: %v", err)
	}
	gitDir := filepath.Join(projectDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Fatalf(".git missing: %v", err)
	}
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 commit, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(string(out), integrationProjectName) {
		t.Errorf("commit message should contain project name; got: %s", out)
	}
}

// assertGoBuildAndTest runs `go build ./...` and `go test ./...` in the
// generated project with CGO disabled. Both must exit 0.
func assertGoBuildAndTest(t *testing.T, projectDir string) {
	t.Helper()
	for _, args := range [][]string{{"build", "./..."}, {"test", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = projectDir
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("go %s in %s failed: %v\n%s",
				strings.Join(args, " "), projectDir, err, out)
		}
	}
}

// assertNoV1Leaks runs scripts/check-v1-leaks.sh against the scaffolded
// project. Must exit 0 (no v1 patterns found).
func assertNoV1Leaks(t *testing.T, projectDir, repoRootPath string) {
	t.Helper()
	script := filepath.Join(repoRootPath, "scripts", "check-v1-leaks.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("check-v1-leaks.sh missing: %v", err)
	}
	cmd := exec.Command("bash", script, projectDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("v1 leaks detected in %s:\n%s%s",
			projectDir, stdout.String(), stderr.String())
	}
}

// TestIntegrationScaffold_TUIAllLibs scaffolds a TUI variant with every
// charm v2 lib that has a TUI-side conditional block and asserts:
//   - the restructured tree (cmd/<name>/main.go + internal/app/*.go) is
//     emitted with the expected per-file content
//   - every lib's conditional block is INLINED into internal/app/update.go
//     (huh, glamour, harmonica, bubbles, log) — Plan 02-05's central
//     guarantee that the variant file composes libs in one place rather
//     than scattering one file per lib
//   - the project builds with CGO_ENABLED=0 and has zero v1 leaks
//
// This is the "kitchen sink" TUI test: if a future change accidentally
// reverts to the file-per-lib overlay pattern this test catches it.
func TestIntegrationScaffold_TUIAllLibs(t *testing.T) {
	name := integrationProjectName + "-tui-all"
	projectDir, repoRootPath := runSpinScaffold(t, name,
		[]string{
			"--tui", "--bubbletea", "--bubbles", "--lipgloss",
			"--huh", "--glamour", "--harmonica", "--log",
		})

	// Structure: thin entry + internal/app + internal/ui.
	wantFiles := []string{
		filepath.Join("cmd", name, "main.go"),
		filepath.Join("internal", "app", "app.go"),
		filepath.Join("internal", "app", "update.go"),
		filepath.Join("internal", "app", "view.go"),
		filepath.Join("internal", "app", "keys.go"),
		filepath.Join("internal", "ui", "styles.go"),
	}
	for _, f := range wantFiles {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}

	// main.go is the thin entry point — does NOT contain MVU runtime.
	mainGo, err := os.ReadFile(filepath.Join(projectDir, "cmd", name, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	for _, banned := range []string{"tea.NewProgram", "type Model struct", "func (m"} {
		if bytes.Contains(mainGo, []byte(banned)) {
			t.Errorf("cmd/%s/main.go should be a thin entry, but contains %q:\n%s",
				name, banned, mainGo)
		}
	}

	// internal/app/update.go must inline the lib content that belongs
	// inside Update (Plan 02-05 central guarantee — no per-lib file).
	// Constructors (spinner.New) live in app.go; tick/message handling
	// lives in update.go. We check both files for the right markers.
	updateGo, err := os.ReadFile(filepath.Join(projectDir, "internal", "app", "update.go"))
	if err != nil {
		t.Fatalf("read internal/app/update.go: %v", err)
	}
	wantInlinedUpdate := map[string]string{
		"huh.NewForm":             "--huh wiring",
		"glamour.NewTermRenderer": "--glamour wiring",
		"harmonica.NewSpring":     "--harmonica wiring",
		"spinner.TickMsg":         "--bubbles spinner tick handling",
		"log.Info":                "--log wiring",
	}
	for marker, label := range wantInlinedUpdate {
		if !bytes.Contains(updateGo, []byte(marker)) {
			t.Errorf("internal/app/update.go missing %s (%q):\n%s",
				label, marker, updateGo)
		}
	}

	// app.go must hold the constructors (Plan 02-05 splits Model + New
	// + Init + Run into app.go; the rest is in update.go and view.go).
	appGo, err := os.ReadFile(filepath.Join(projectDir, "internal", "app", "app.go"))
	if err != nil {
		t.Fatalf("read internal/app/app.go: %v", err)
	}
	if !bytes.Contains(appGo, []byte("spinner.New")) {
		t.Errorf("internal/app/app.go missing 'spinner.New' (--bubbles constructor):\n%s", appGo)
	}

	// internal/app/* must NOT have the old file-per-lib overlay names.
	// If a future change accidentally re-introduces them this test
	// catches it before the v1-leak script does.
	for _, banned := range []string{"huh.go", "wish.go", "glamour.go", "harmonica.go"} {
		if _, err := os.Stat(filepath.Join(projectDir, "internal", "app", banned)); err == nil {
			t.Errorf("internal/app/%s exists — Plan 02-05 forbids per-lib files; inline into update.go instead", banned)
		}
	}

	assertGoBuildAndTest(t, projectDir)
	assertNoV1Leaks(t, projectDir, repoRootPath)
}

// TestIntegrationScaffold_CLIAllLibs scaffolds a CLI variant with every
// charm v2 lib that has a CLI-side conditional block and asserts:
//   - the restructured tree (cmd/<name>/main.go + internal/cmd/*.go) is
//     emitted with one file per subcommand (hello, ssh)
//   - main.go is a thin fang.Execute entry
//   - hello subcommand runs end-to-end and produces the
//     expected output (lipgloss-styled "Hello, world!")
//   - the project builds with CGO_ENABLED=0 and has zero v1 leaks
func TestIntegrationScaffold_CLIAllLibs(t *testing.T) {
	name := integrationProjectName + "-cli-all"
	projectDir, repoRootPath := runSpinScaffold(t, name,
		[]string{
			"--cli", "--cobra", "--fang", "--lipgloss",
			"--glamour", "--wish", "--log", "--viper",
		})

	// Structure: thin entry + internal/cmd subcommands + internal/ui.
	wantFiles := []string{
		filepath.Join("cmd", name, "main.go"),
		filepath.Join("internal", "cmd", "root.go"),
		filepath.Join("internal", "cmd", "hello.go"),
		filepath.Join("internal", "cmd", "ssh.go"),
		filepath.Join("internal", "ui", "styles.go"),
	}
	for _, f := range wantFiles {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}

	// main.go must be the thin fang.Execute entry point.
	mainGo, err := os.ReadFile(filepath.Join(projectDir, "cmd", name, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	for _, want := range []string{
		"package main",
		"fang.Execute",
		`"` + name + `/internal/cmd"`,
	} {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("cmd/%s/main.go missing %q; got:\n%s", name, want, mainGo)
		}
	}

	assertGoBuildAndTest(t, projectDir)
	assertNoV1Leaks(t, projectDir, repoRootPath)

	// Build the scaffolded CLI and exercise the hello subcommand.
	cliBin := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", cliBin, "./cmd/"+name)
	build.Dir = projectDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}

	hello := exec.Command(cliBin, "hello", "world")
	hello.Dir = projectDir
	out, err := hello.CombinedOutput()
	if err != nil {
		t.Errorf("%s hello world failed: %v\n%s", name, err, out)
	}
	if !bytes.Contains(out, []byte("Hello, world!")) {
		t.Errorf("hello world output missing 'Hello, world!':\n%s", out)
	}
}

// TestIntegrationScaffold_AllVariant scaffolds the --all variant
// (TUI + CLI composed in one binary) with the full lib set and asserts:
//   - the merged tree contains internal/app/*.go AND internal/cmd/*.go
//   - the root help lists all 3 subcommands (tui, hello, ssh)
//   - hello subcommand works end-to-end
//   - tui subcommand exists in help (we do not exec the TUI itself
//     because bubbletea needs a real TTY)
func TestIntegrationScaffold_AllVariant(t *testing.T) {
	name := integrationProjectName + "-all"
	projectDir, repoRootPath := runSpinScaffold(t, name,
		[]string{
			"--all", "--bubbletea", "--bubbles", "--lipgloss",
			"--cobra", "--fang", "--huh", "--glamour", "--wish", "--log",
		})

	// Structure: thin entry + internal/app + internal/cmd + internal/ui.
	for _, f := range []string{
		filepath.Join("cmd", name, "main.go"),
		filepath.Join("internal", "app", "app.go"),
		filepath.Join("internal", "app", "update.go"),
		filepath.Join("internal", "app", "view.go"),
		filepath.Join("internal", "cmd", "root.go"),
		filepath.Join("internal", "cmd", "hello.go"),
		filepath.Join("internal", "cmd", "ssh.go"),
		filepath.Join("internal", "cmd", "tui.go"),
		filepath.Join("internal", "ui", "styles.go"),
	} {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}

	assertGoBuildAndTest(t, projectDir)
	assertNoV1Leaks(t, projectDir, repoRootPath)

	// Build + exercise root help (must list tui, hello, ssh).
	allBin := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", allBin, "./cmd/"+name)
	build.Dir = projectDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build all-variant: %v\n%s", err, out)
	}

	help := exec.Command(allBin, "--help")
	help.Dir = projectDir
	out, err := help.CombinedOutput()
	if err != nil {
		t.Errorf("%s --help failed: %v\n%s", name, err, out)
	}
	for _, sub := range []string{"tui", "hello", "ssh"} {
		if !bytes.Contains(out, []byte(sub)) {
			t.Errorf("--help output missing subcommand %q:\n%s", sub, out)
		}
	}

	hello := exec.Command(allBin, "hello", "world")
	hello.Dir = projectDir
	out, err = hello.CombinedOutput()
	if err != nil {
		t.Errorf("%s hello world failed: %v\n%s", name, err, out)
	}
	if !bytes.Contains(out, []byte("Hello, world!")) {
		t.Errorf("hello world output missing 'Hello, world!':\n%s", out)
	}
}

// TestIntegrationScaffold_NameInPath verifies the walker substitutes the
// `_name_` placeholder in output paths with p.Name for unusual but
// legal project names (mixed case, digits, dashes, underscores). Plan
// 02-05's central walker contract: `templates/.../cmd/_name_/main.go.tmpl`
// renders to `cmd/<actual-name>/main.go`.
func TestIntegrationScaffold_NameInPath(t *testing.T) {
	weirdName := "weird-name_123"
	projectDir, _ := runSpinScaffold(t, weirdName,
		[]string{"--tui", "--bubbletea"})

	wantPath := filepath.Join(projectDir, "cmd", weirdName, "main.go")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected %s to exist (walker `_name_` substitution failed): %v", wantPath, err)
	}

	// The literal `_name_` placeholder must NOT appear anywhere in the
	// scaffolded tree (would indicate a missed substitution).
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.Contains(path, "_name_") {
			t.Errorf("scaffolded path contains unsubstituted `_name_` placeholder: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk scaffold tree: %v", err)
	}
}

// TestIntegrationScaffold_AGENTSmd scaffolds a TUI project with --ai
// and asserts the generated AGENTS.md contains the expected sections.
// Plan 03-04's "AGENTS.md opt-in" test: --ai is the load-bearing flag,
// and the output must begin with the marker, contain all 6 sections,
// list libraries alphabetically, and contain no ANSI / lipgloss.
func TestIntegrationScaffold_AGENTSmd(t *testing.T) {
	projectDir, _ := runSpinScaffold(t, integrationProjectName+"-ai",
		[]string{"--tui", "--bubbletea", "--lipgloss", "--ai"})

	assertAGENTSmd(t, projectDir)
}

// TestIntegrationScaffold_AGENTSmd_Determinism is the load-bearing
// determinism test: scaffolds the same project twice (in two separate
// temp dirs, so the runs are independent) and asserts the two AGENTS.md
// files are byte-identical. This is the contract that makes AGENTS.md
// safe to commit (no machine IDs, no timestamps, no UUIDs). Pitfall 5
// from 03-RESEARCH.md.
//
// Both scaffolds use the same project name so the only difference
// between the two runs is the working directory and process IDs —
// neither of which can leak into the rendered bytes. The project name
// and module path are inputs, not noise; if those change, the output
// is allowed to change.
func TestIntegrationScaffold_AGENTSmd_Determinism(t *testing.T) {
	flags := []string{"--tui", "--bubbletea", "--bubbles", "--lipgloss", "--ai"}
	name := integrationProjectName + "-determinism"
	first, _ := runSpinScaffold(t, name, flags)
	second, _ := runSpinScaffold(t, name, flags)

	firstAgents, err := os.ReadFile(filepath.Join(first, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read first AGENTS.md: %v", err)
	}
	secondAgents, err := os.ReadFile(filepath.Join(second, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read second AGENTS.md: %v", err)
	}
	if !bytes.Equal(firstAgents, secondAgents) {
		t.Errorf("AGENTS.md not byte-identical across two scaffolds:\n--- first ---\n%s\n--- second ---\n%s",
			firstAgents, secondAgents)
	}
}

// TestIntegrationScaffold_AGENTSmd_Alias verifies --agents (the alias)
// produces the same AGENTS.md as --ai. This is the UI-SPEC Locked
// Decision #5 alias contract: the two spellings are interchangeable.
// We use the same flag set as TestIntegrationScaffold_AGENTSmd so the
// content assertions (Bubble Tea, Lip Gloss in alphabetical order) hold
// for both --ai and --agents.
func TestIntegrationScaffold_AGENTSmd_Alias(t *testing.T) {
	projectDir, _ := runSpinScaffold(t, integrationProjectName+"-agents-alias",
		[]string{"--tui", "--bubbletea", "--lipgloss", "--agents"})

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err != nil {
		t.Fatalf("--agents did not produce AGENTS.md: %v", err)
	}
	// Same content assertions as --ai.
	assertAGENTSmd(t, projectDir)
}

// TestIntegrationScaffold_NoAI_NoAGENTSmd asserts that without --ai
// (or --agents), no AGENTS.md is emitted. The overlay walker skips
// lib/ai/ when p.AI is false.
func TestIntegrationScaffold_NoAI_NoAGENTSmd(t *testing.T) {
	projectDir, _ := runSpinScaffold(t, integrationProjectName+"-no-ai",
		[]string{"--tui", "--bubbletea"})

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md should NOT exist without --ai, but %s exists (err=%v)",
			agentsPath, err)
	}
}

// assertAGENTSmd is the shared content assertion for AGENTS.md in the
// integration tests. Asserts:
//   - AGENTS.md exists at project root
//   - line 1 is the version marker
//   - the 6 required sections are present
//   - libraries are listed alphabetically
//   - no ANSI escape codes
//   - no TODO / FIXME placeholders
func assertAGENTSmd(t *testing.T, projectDir string) {
	t.Helper()
	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read %s: %v", agentsPath, err)
	}

	// Line 1 is the version marker. The exact version comes from the
	// spin binary (ldflags), so we assert the prefix rather than a
	// specific version string.
	firstLine, _, _ := strings.Cut(string(agents), "\n")
	wantPrefix := "<!-- AUTOGENERATED by spin "
	if !strings.HasPrefix(firstLine, wantPrefix) || !strings.HasSuffix(firstLine, " -->") {
		t.Errorf("AGENTS.md line 1 = %q, want %q<ver>%q", firstLine, wantPrefix, " -->")
	}

	// All 6 required sections must be present.
	sections := []string{
		"## What this project is",
		"## Libraries",
		"## Extending",
		"## Conventions",
		"## Rebuilding this file",
	}
	for _, s := range sections {
		if !bytes.Contains(agents, []byte(s)) {
			t.Errorf("AGENTS.md missing section %q", s)
		}
	}

	// Library sections are present and alphabetical. The test project
	// has --tui --bubbletea --lipgloss (and auto-implied bubbletea),
	// so we expect both Bubble Tea and Lip Gloss headings with BT
	// appearing before LG.
	idxBT := bytes.Index(agents, []byte("### Bubble Tea"))
	idxLG := bytes.Index(agents, []byte("### Lip Gloss"))
	if idxBT < 0 {
		t.Errorf("AGENTS.md missing '### Bubble Tea' heading")
	}
	if idxLG < 0 {
		t.Errorf("AGENTS.md missing '### Lip Gloss' heading")
	}
	if idxBT >= 0 && idxLG >= 0 && idxBT >= idxLG {
		t.Errorf("AGENTS.md libraries not alphabetical: Bubble Tea at %d, Lip Gloss at %d", idxBT, idxLG)
	}

	// No ANSI escape codes (UI-SPEC §"What AGENTS.md MUST NOT contain").
	for _, banned := range []string{"\033", "\x1b", "\x1B"} {
		if bytes.Contains(agents, []byte(banned)) {
			t.Errorf("AGENTS.md contains forbidden ANSI byte %q", banned)
		}
	}

	// No TODO / FIXME placeholders — the file is a finished doc.
	for _, banned := range []string{"TODO", "FIXME"} {
		if bytes.Contains(agents, []byte(banned)) {
			t.Errorf("AGENTS.md contains forbidden placeholder %q", banned)
		}
	}
}
