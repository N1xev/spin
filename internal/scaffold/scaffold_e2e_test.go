// This is the Walking Skeleton's end-to-end test. It proves the spin binary
// can be built, can scaffold a runnable bubbletea v2 TUI project from a temp
// directory, and the generated project passes `go build ./...` and
// `go test ./...` with CGO disabled. Failure here means the scaffolder
// pipeline is broken end-to-end.
//
// The test builds spin into os.TempDir(), runs the CLI against a fresh
// t.TempDir(), validates every emitted file, runs go build + go test on
// the generated project, and greps for v1 charmbracelet import leaks
// (TOOL-03).
package scaffold

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EScaffold(t *testing.T) {
	tmpRoot := t.TempDir()

	// 1. Build spin with a stable absolute path.
	spinBin := filepath.Join(os.TempDir(), "spin-e2e-spin")
	build := exec.Command("go", "build", "-o", spinBin, ".")
	build.Dir = repoRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build spin failed:\n%s\n%v", out, err)
	}
	t.Cleanup(func() { _ = os.Remove(spinBin) })

	// 2. chdir into a fresh temp dir for the scaffold.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpRoot); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// 3. Run the spin CLI.
	run := exec.Command(spinBin, "new", "e2e-myapp", "--tui", "--bubbletea")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("spin new failed (exit %v):\n%s", err, out)
	}

	projectDir := filepath.Join(tmpRoot, "e2e-myapp")

	// 4. go.mod assertions.
	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, want := range []string{
		"module e2e-myapp",
		"charm.land/bubbletea/v2 v2.0.7",
	} {
		if !bytes.Contains(goMod, []byte(want)) {
			t.Errorf("go.mod missing %q; got:\n%s", want, goMod)
		}
	}

	// 5. main.go assertions. Plan 02-05: main.go is at cmd/<name>/main.go
	// (thin entry, hands off to app.Run); tea.NewProgram lives in
	// internal/app/app.go.
	mainGo, err := os.ReadFile(filepath.Join(projectDir, "cmd", "e2e-myapp", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/e2e-myapp/main.go: %v", err)
	}
	for _, want := range []string{
		"package main",
		"app.Run",
	} {
		if !bytes.Contains(mainGo, []byte(want)) {
			t.Errorf("main.go missing %q; got:\n%s", want, mainGo)
		}
	}
	// The bubbletea runtime is in internal/app/app.go.
	appGo, err := os.ReadFile(filepath.Join(projectDir, "internal", "app", "app.go"))
	if err != nil {
		t.Fatalf("read internal/app/app.go: %v", err)
	}
	if !bytes.Contains(appGo, []byte("tea.NewProgram")) {
		t.Errorf("internal/app/app.go missing tea.NewProgram; got:\n%s", appGo)
	}

	// 6. .gitignore exists.
	if _, err := os.Stat(filepath.Join(projectDir, ".gitignore")); err != nil {
		t.Errorf(".gitignore not on disk: %v", err)
	}

	// 7. README.md exists and has Next steps.
	readme, err := os.ReadFile(filepath.Join(projectDir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !bytes.Contains(readme, []byte("## Next steps")) {
		t.Errorf("README.md missing '## Next steps' section; got:\n%s", readme)
	}

	// 8. go build ./... and go test ./... in the generated project.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = projectDir
	tidy.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy in %s failed:\n%s", projectDir, out)
	}

	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = projectDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... in %s failed:\n%s", projectDir, out)
	}

	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = projectDir
	testCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... in %s failed:\n%s", projectDir, out)
	}

	// 9. v1-leak grep (TOOL-03).
	grep := exec.Command("grep", "-rE", "github.com/charmbracelet/",
		"--include=*.go", ".")
	grep.Dir = projectDir
	if out, err := grep.CombinedOutput(); err == nil {
		t.Errorf("v1 charmbracelet import path leak detected:\n%s", out)
	} else if !strings.Contains(err.Error(), "exit status 1") {
		// grep returns 1 when no match found (which is what we want).
		t.Errorf("unexpected grep error: %v\n%s", err, out)
	}
}

// repoRoot returns the absolute path of the spin repo root so the E2E
// test can run `go build .` from the right directory regardless of
// process working directory.
//
// Walks up from the current working directory until it finds go.mod.
// This is robust against tests that chdir into temp dirs (e.g. the
// runSpinScaffold helper does `os.Chdir(workDir)` so the scaffolded
// project is the cwd by the time `spin` runs). A previous version
// used a fixed `wd/../..` which broke in exactly that scenario — the
// first call to runSpinScaffold would chdir, the second would
// compute the wrong root.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod starting from %s", wd)
		}
		dir = parent
	}
}
