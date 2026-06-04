// Package scaffold: external template repo cloning.
//
// CloneTemplateRepo clones a user-supplied git URL to a fresh tempdir
// and validates that the tree has a `_base/` subdir (the spin overlay
// engine's entry point). The caller owns the returned path; pass it to
// os.RemoveAll on completion, or set p.KeepTemplateCache to retain.
package scaffold

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CloneTemplateRepoTimeout caps how long `git clone` may take. Declared
// as a `var` (not const) so repo_test.go can lower it to a sub-second
// value without hanging the suite.
var CloneTemplateRepoTimeout = 60 * time.Second

// CloneTemplateRepo clones url to a fresh tempdir and validates that
// the result has a `_base/` subdir. On any failure the tempdir is
// removed and the error wraps the underlying git stderr. The clone is
// wrapped in a 60s timeout (CR-005) so a slow remote cannot freeze
// the scaffolder.
//
// Requires `git` on $PATH; if missing, returns an exec.ErrNotFound-
// wrapped error (callers use errors.Is to surface a "git not installed"
// message).
func CloneTemplateRepo(ctx context.Context, url string) (string, error) {
	tmp, err := os.MkdirTemp("", "spin-template-*")
	if err != nil {
		return "", fmt.Errorf("mkdir tempdir: %w", err)
	}

	cloneCtx, cancel := context.WithTimeout(ctx, CloneTemplateRepoTimeout)
	defer cancel()

	// --depth 1 keeps the clone cheap; spin only needs HEAD. The `--`
	// separator is defense-in-depth (CR-004): without it, a `url` like
	// "-upload-pack=evil" would be interpreted as a flag. The
	// validator (IsValidTemplateRepo) is the primary gate; the `--`
	// is the belt to its suspenders.
	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--", url, tmp)
	cmd.Env = append(os.Environ(), gitEnv...)
	// CR-005: when the context expires, force the I/O pipes closed
	// so CombinedOutput's drainer goroutines return instead of
	// blocking on Read. cmd.Cancel (Go 1.20+) is the standard way.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(tmp)
		// CR-005: surface a clear timeout error via errors.Is against
		// context.DeadlineExceeded (exec.CommandContext propagates the
		// ctx error as the cmd error on cancellation).
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf(
				"git clone timed out after %s; check the URL or your network",
				CloneTemplateRepoTimeout,
			)
		}
		return "", fmt.Errorf("git clone %s failed:\n%s", url, strings.TrimSpace(string(out)))
	}

	// Validate the cloned tree has the required entry point.
	baseDir := filepath.Join(tmp, "_base")
	info, err := os.Stat(baseDir)
	if err != nil {
		_ = os.RemoveAll(tmp)
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"template repo %s: missing _base/ directory (required by spin's overlay engine)",
				url,
			)
		}
		return "", fmt.Errorf("stat _base/ in %s: %w", tmp, err)
	}
	if !info.IsDir() {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf(
			"template repo %s: _base exists but is not a directory (required by spin's overlay engine)",
			url,
		)
	}

	return tmp, nil
}
