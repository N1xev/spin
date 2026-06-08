package rust

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/example/spin/internal/ecosystem"
)

// PostScaffold writes the rendered file map to disk and runs the
// post-scaffold hooks (git init, initial commit). We deliberately
// skip `cargo build` here — build verification is the user's
// responsibility, or a future `spin doctor` check.
func (e *Ecosystem) PostScaffold(ctx ecosystem.Context, dir string) error {
	if dir == "" {
		dir = ctx.Name
	}
	// If dir is relative, anchor it at cwd. This matches what the
	// v1 scaffold package does in its emit() helper.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(".", dir)
	}

	files, err := e.Render(ctx)
	if err != nil {
		return fmt.Errorf("rust: render: %w", err)
	}
	if err := writeFiles(dir, files); err != nil {
		return fmt.Errorf("rust: write files: %w", err)
	}

	// Skip git init if --no-git.
	if ctx.GetBool("no-git") {
		return nil
	}

	// Skip if git is not on $PATH. Log a one-line warning and continue
	// rather than failing the whole scaffold.
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git not found on $PATH; skipping git init for %s\n", dir)
		return nil
	}

	init := exec.Command("git", "init")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		return fmt.Errorf("rust: git init: %w (%s)", err, string(out))
	}

	add := exec.Command("git", "add", "-A")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("rust: git add: %w (%s)", err, string(out))
	}

	commitMsg := fmt.Sprintf("chore: scaffold %s with spin v%s", ctx.Name, ctx.SpinVer)
	commit := exec.Command("git", "commit", "-m", commitMsg)
	commit.Dir = dir
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("rust: git commit: %w (%s)", err, string(out))
	}

	return nil
}
