package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitClone performs a shallow git clone with a 2-minute timeout.
func GitClone(ctx context.Context, url, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", url, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone %s: %s: %w", url, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// GitFetch performs a shallow git fetch with a 2-minute timeout.
func GitFetch(ctx context.Context, repoDir string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "fetch", "--depth=1", "origin")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cannot fetch %s: %s", repoDir, strings.TrimSpace(string(out)))
	}
	return nil
}

// GitReset performs a git reset --hard to origin/HEAD, falling back to FETCH_HEAD.
func GitReset(ctx context.Context, repoDir string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "reset", "--hard", "origin/HEAD")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		cmd2 := exec.CommandContext(ctx, "git", "-C", repoDir, "reset", "--hard", "FETCH_HEAD")
		cmd2.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("cannot reset %s: %s (also tried FETCH_HEAD: %s)", repoDir, strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
	}
	return nil
}
