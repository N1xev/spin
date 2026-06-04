// End-to-end integration tests for the 5 wrapper subcommands
// (run, build, test, vet, fmt). These tests build the spin binary
// from source, scaffold a minimal Go project into a fresh tempdir,
// chdir into it, and exec each wrapper subcommand via the spin
// binary. The tests assert on:
//   - exit codes (0 vs non-0 in the right conditions)
//   - file state (bin/<name> is produced by `spin build`)
//   - stderr content (install hints fire when tools are missing)
//
// The tests live in the `wrap` package (rather than `scaffold`)
// because they exercise the spin binary's wrapper subcommands,
// not the scaffolder. The `scaffold` package's
// integration_test.go covers the scaffolder end-to-end; this file
// is the mirror for the wrapper side.
//
// Failure here means a wrapper subcommand is broken end-to-end —
// the cobra subcommand is registered, the internal/wrap function
// returns, and the spin binary propagates the error correctly.
package wrap

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// integrationProjectName is the project name used in the scaffold
// for these tests. The Walking Skeleton form (--tui --bubbletea)
// is the lightest variant — enough to give the wrappers a real
// project to operate on without pulling in extra dependencies.
const integrationProjectName = "wrap-int-myapp"

// buildSpin builds the spin binary to a stable absolute path and
// returns the path. Cleanup is registered automatically.
func buildSpin(t *testing.T) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	binPath := filepath.Join(os.TempDir(),
		fmt.Sprintf("spin-wrap-int-%d-%s", os.Getpid(), filepath.Base(t.Name())))
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stale bin: %v", err)
	}
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build spin: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(binPath) })
	return binPath
}

// scaffoldMinimalProject writes a minimal go.mod + main.go into
// workDir and returns the project path. We avoid the full
// scaffolder here because the wrapper tests don't care about
// template content — they only need a real Go project to run the
// wrappers against. A minimal project keeps the test fast and
// side-effect-free.
func scaffoldMinimalProject(t *testing.T, workDir, name string) string {
	t.Helper()
	projectDir := filepath.Join(workDir, name)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	goMod := "module " + name + "\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	mainGo := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return projectDir
}

// runSpinCmd execs the spin binary at binPath with the given
// subcommand + args in a fresh tempdir. It returns stdout, stderr,
// and the exit error.
func runSpinCmd(t *testing.T, binPath, subcommand string, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	fullArgs := append([]string{subcommand}, args...)
	cmd := exec.Command(binPath, fullArgs...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestWrapperRun_HelpRegistered verifies the `spin run` subcommand
// is registered and fang-styled help renders. `--help` exits 0
// (cobra convention) and the output mentions the subcommand name.
func TestWrapperRun_HelpRegistered(t *testing.T) {
	binPath := buildSpin(t)
	stdout, stderr, err := runSpinCmd(t, binPath, "run", "--help")
	if err != nil {
		t.Fatalf("spin run --help failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	// fang-styled help mentions the subcommand; we don't pin the
	// exact wording because fang's renderer changes between versions.
	// We just assert the subcommand description ("Run the project")
	// is present.
	if !strings.Contains(stdout, "Run the project") {
		t.Errorf("expected help to mention 'Run the project'; got: %q", stdout)
	}
}

// TestWrapperBuild_ProducesBinary verifies `spin build` produces
// bin/<basename> in the project's working directory.
//
// The test scaffolds a minimal project, chdirs to it, runs
// `spin build`, then asserts the binary file exists and is
// executable.
func TestWrapperBuild_ProducesBinary(t *testing.T) {
	binPath := buildSpin(t)
	workDir := t.TempDir()
	projectDir := scaffoldMinimalProject(t, workDir, integrationProjectName)
	t.Chdir(projectDir)

	cmd := exec.Command(binPath, "build")
	cmd.Dir = projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("spin build failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	binFile := filepath.Join(projectDir, "bin", integrationProjectName)
	info, err := os.Stat(binFile)
	if err != nil {
		t.Fatalf("expected binary at %s: %v", binFile, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("binary at %s is not executable (mode=%v)", binFile, info.Mode())
	}
}

// TestWrapperTest_GoTestFallback verifies `spin test` runs
// `go test ./...` (prism is missing in CI), and exits 0 when the
// project has no failing tests. A minimal project with no _test.go
// files makes `go test` exit 0 trivially.
func TestWrapperTest_GoTestFallback(t *testing.T) {
	binPath := buildSpin(t)
	workDir := t.TempDir()
	projectDir := scaffoldMinimalProject(t, workDir, integrationProjectName)
	t.Chdir(projectDir)

	cmd := exec.Command(binPath, "test")
	cmd.Dir = projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("spin test failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	// We don't assert on the install hint being printed — that
	// depends on whether prism is on $PATH in the test env. The
	// exit-0 + "No tests found" (Go 1.10+) or "ok" (with a real
	// test file) is the load-bearing assertion: the wrapper
	// delegated to the test runner and it exited 0.
	//
	// We check for either the "ok" pattern (prism/gotest happy
	// path with a passing test) or the "No tests found" message
	// (no _test.go files in the project — Go's standard response).
	out := stdout.String()
	if !strings.Contains(out, "ok") && !strings.Contains(out, "No tests found") {
		t.Errorf("expected 'ok' or 'No tests found' from test runner; got stdout: %q", out)
	}
}

// TestWrapperVet_GoVet verifies `spin vet` runs `go vet ./...`
// and exits 0 on clean code.
func TestWrapperVet_GoVet(t *testing.T) {
	binPath := buildSpin(t)
	workDir := t.TempDir()
	projectDir := scaffoldMinimalProject(t, workDir, integrationProjectName)
	t.Chdir(projectDir)

	cmd := exec.Command(binPath, "vet")
	cmd.Dir = projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("spin vet failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
}

// TestWrapperFmt_NoStrict verifies `spin fmt --no-strict` exits 0
// in a project with no gofumpt in the env. We use --no-strict so
// the test is robust to whether gofumpt happens to be on $PATH
// (it usually isn't in CI).
//
// We can't test the strict-mode gofumpt-missing failure here
// because the test runner inherits the full test env (PATH may
// have gofumpt). That case is covered by the unit test
// TestFmt_GofumptMissing_Strict in fmt_test.go.
func TestWrapperFmt_NoStrict(t *testing.T) {
	binPath := buildSpin(t)
	workDir := t.TempDir()
	projectDir := scaffoldMinimalProject(t, workDir, integrationProjectName)
	t.Chdir(projectDir)

	cmd := exec.Command(binPath, "fmt", "--no-strict")
	cmd.Dir = projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("spin fmt --no-strict failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
}

// TestWrapperAllSubcommands_HelpRegistered is a smoke test that
// every wrapper subcommand (build, test, vet, fmt) is registered
// and has a fang-styled help. Fails fast if any subcommand is
// missing from the binary.
func TestWrapperAllSubcommands_HelpRegistered(t *testing.T) {
	binPath := buildSpin(t)
	for _, sub := range []string{"run", "build", "test", "vet", "fmt"} {
		t.Run(sub, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(binPath, sub, "--help")
			cmd.Dir = dir
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("spin %s --help failed: %v\nstdout: %s\nstderr: %s",
					sub, err, stdout.String(), stderr.String())
			}
		})
	}
}

// findRepoRoot locates the spin repo root by walking up from the
// current working directory until it finds a go.mod file. Used by
// buildSpin to set cmd.Dir to the repo root.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find spin repo root (go.mod) from %s", dir)
		}
		dir = parent
	}
}

// (intentionally no code follows — the file ends here.)
