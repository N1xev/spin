package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp changes the working directory to a fresh t.TempDir() and
// returns a cleanup func that restores the original CWD. Required for
// hooks tests because scaffold.New writes to ./<name>/ (relative paths).
func chdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return tmp
}

// minimalScaffold creates a Project + ./<name>/ directory structure
// sufficient for VerifyBuild to run `go build` and `go test`. The dir
// contains a minimal go.mod + main.go that compiles cleanly.
func minimalScaffold(t *testing.T, tmp string, name string) *Project {
	t.Helper()
	root := filepath.Join(tmp, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", root, err)
	}
	gomod := "module " + name + "\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	mainGo := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return &Project{Name: name, Module: name, Type: "tui", Year: 2026, SpinVer: "0.1.0"}
}

// brokenScaffold creates a Project whose go.mod references a non-existent
// dependency, so `go mod tidy` fails and VerifyBuild returns an error
// containing the go command's output.
func brokenScaffold(t *testing.T, tmp string, name string) *Project {
	t.Helper()
	root := filepath.Join(tmp, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", root, err)
	}
	gomod := "module " + name + "\n\ngo 1.23\n\nrequire nonexistent.invalid/nope v0.0.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	mainGo := "package main\n\nimport \"nonexistent.invalid/nope\"\n\nfunc main() { _ = nope.X }\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return &Project{Name: name, Module: name, Type: "tui", Year: 2026, SpinVer: "0.1.0"}
}

func TestVerifyBuild_Passing(t *testing.T) {
	tmp := chdirTemp(t)
	name := "spin-hook-passing-" + randStr(t)
	p := minimalScaffold(t, tmp, name)

	if err := p.VerifyBuild(); err != nil {
		t.Fatalf("VerifyBuild on minimal project: %v", err)
	}
}

func TestVerifyBuild_Failing(t *testing.T) {
	tmp := chdirTemp(t)
	name := "spin-hook-failing-" + randStr(t)
	p := brokenScaffold(t, tmp, name)

	err := p.VerifyBuild()
	if err == nil {
		t.Fatal("VerifyBuild on broken project: got nil, want error")
	}
	// The error must surface the go command's output.
	if !strings.Contains(err.Error(), name) {
		t.Errorf("error %q does not name the project %q", err.Error(), name)
	}
}

func TestVerifyBuild_Skipped(t *testing.T) {
	tmp := chdirTemp(t)
	name := "spin-hook-skip-" + randStr(t)
	root := filepath.Join(tmp, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Note: do NOT create a go.mod. If VerifyBuild runs, it would fail.
	// --no-verify must short-circuit BEFORE any exec.
	p := &Project{Name: name, Module: name, NoVerify: true}

	if err := p.VerifyBuild(); err != nil {
		t.Fatalf("VerifyBuild with NoVerify=true: %v", err)
	}
	// Sanity: no go.mod was created, confirming no exec happened.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		t.Error("go.mod was created despite --no-verify; VerifyBuild ran")
	}
}

func TestGitInit_NoGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on $PATH")
	}
	tmp := chdirTemp(t)
	name := "spin-nogit-" + randStr(t)
	root := filepath.Join(tmp, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := &Project{Name: name, Module: name, NoGit: true, SpinVer: "0.1.0"}

	if err := p.GitInit(); err != nil {
		t.Fatalf("GitInit with NoGit=true: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Error(".git/ was created despite --no-git; GitInit ran")
	}
}

func TestGitInit_Success(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on $PATH")
	}
	tmp := chdirTemp(t)
	name := "spin-gitok-" + randStr(t)
	root := filepath.Join(tmp, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Drop a file so the commit has something to add.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	p := &Project{Name: name, Module: name, SpinVer: "0.1.0"}

	if err := p.GitInit(); err != nil {
		t.Fatalf("GitInit: %v", err)
	}

	// Verify .git/ exists.
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Errorf(".git/ not created: %v", err)
	}

	// Verify 1 commit was made.
	log := exec.Command("git", "log", "--oneline")
	log.Dir = root
	out, err := log.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 commit; got %d:\n%s", len(lines), out)
	}
	// The commit message should mention the project name and spin version.
	if !strings.Contains(string(out), name) || !strings.Contains(string(out), "spin") {
		t.Errorf("commit message missing project name or spin marker:\n%s", out)
	}
}
