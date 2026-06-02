// TOOL-05 integration test: scaffolds a complete Phase 1 project and
// validates every Phase 1 acceptance criterion end-to-end.
//
// This is the canonical "Phase 1 works" test. It builds the spin binary
// from source, runs the CLI against a fresh temp directory, then validates:
//
//  1. The full TUI scaffold (--tui --bubbletea --bubbles --lipgloss):
//     - go.mod has the expected module name, charm.land/*/v2 pins, and
//       no `github.com/charmbracelet/` paths
//     - main.go is v2 (tea.View, tea.KeyPressMsg, tea.NewProgram) and
//       does NOT contain v1 patterns (View() string, tea.WithAltScreen,
//       lipgloss.NewRenderer)
//     - internal/ui/styles.go has real lipgloss v2 styles (NewStyle)
//     - .air.toml uses build.entrypoint, NOT build.bin
//     - Taskfile.yml has the `setup:` target wiring gofumpt + goimports +
//       air + prism installs
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
	"testing"
	"time"
)

const integrationProjectName = "spin-integration-myapp"

func TestIntegrationScaffold(t *testing.T) {
	projectDir, repoRootPath := runSpinScaffold(t, integrationProjectName,
		[]string{"--tui", "--bubbletea", "--bubbles", "--lipgloss"})

	assertGoModFullTUI(t, projectDir)
	assertMainGoV2(t, projectDir)
	assertStylesGoV2(t, projectDir)
	assertAirToml(t, projectDir)
	assertTaskfile(t, projectDir)
	assertLicenseMit(t, projectDir)
	assertReadme(t, projectDir)
	assertGitignore(t, projectDir)
	assertGitInit(t, projectDir)
	assertGoBuildAndTest(t, projectDir)
	assertNoV1Leaks(t, projectDir, repoRootPath)

	t.Logf("OK: Phase 1 integration test passed for %s", integrationProjectName)
}

func TestIntegrationScaffold_NoBubblesGoVersion(t *testing.T) {
	// --bubbletea only (no --bubbles) should NOT bump go directive to 1.25.0.
	// Per RESEARCH §4 (TOOL-01/TOOL-02), only --bubbles requires Go 1.25.0.
	projectDir, _ := runSpinScaffold(t, integrationProjectName+"-nobubbles",
		[]string{"--tui", "--bubbletea"})

	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if bytes.Contains(goMod, []byte("\ngo 1.25.0\n")) {
		t.Errorf("go.mod has go 1.25.0 even though --bubbles was not set:\n%s", goMod)
	}
	// Sanity: go directive is present.
	if !bytes.Contains(goMod, []byte("\ngo ")) {
		t.Errorf("go.mod missing 'go' directive:\n%s", goMod)
	}
}

func TestIntegrationScaffold_LicenseVariants(t *testing.T) {
	cases := []struct {
		flag      string
		mustHave  []string // substrings expected in the LICENSE file
		mustNot   []string // substrings expected to be absent
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

	repoRootPath := repoRoot(t)
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
		"charm.land/bubbletea/v2 v2.0.0",
		"charm.land/lipgloss/v2 v2.0.0", // post-tidy pin (was v2.0.0-beta.2)
		"charm.land/bubbles/v2 v2.0.0",
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

// assertMainGoV2 validates the generated main.go uses the v2 API:
//   - package main, tea.NewProgram, tea.View
//   - does NOT contain v1 patterns (View() string, tea.WithAltScreen,
//     lipgloss.NewRenderer)
func assertMainGoV2(t *testing.T, projectDir string) {
	t.Helper()
	mainGo, err := os.ReadFile(filepath.Join(projectDir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	wants := []string{
		"package main",
		"tea.NewProgram",
		"tea.View", // v2 type, not the v1 `View() string` signature
	}
	for _, want := range wants {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("main.go missing %q; got:\n%s", want, mainGo)
		}
	}
	banned := []string{
		"View() string",       // v1 View signature (TOOL-03)
		"tea.WithAltScreen",   // v1 program option (removed in v2)
		"lipgloss.NewRenderer", // v1 Renderer type (removed in v2)
	}
	for _, b := range banned {
		if bytes.Contains(mainGo, []byte(b)) {
			t.Errorf("main.go contains forbidden v1 pattern %q:\n%s", b, mainGo)
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
