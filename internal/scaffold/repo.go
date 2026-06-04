// Package scaffold: external template repo cloning.
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

// CloneTemplateRepoTimeout caps how long a `git clone` may take. It
// is a var (not const) so tests can lower it without hanging the suite.
var CloneTemplateRepoTimeout = 60 * time.Second

// CloneTemplateRepo clones url into a fresh tempdir and verifies the
// tree contains a `_base/` subdirectory. On any failure the tempdir
// is removed and the returned error wraps the underlying git stderr.
// The clone is wrapped in CloneTemplateRepoTimeout so a slow remote
// cannot freeze the scaffolder.
func CloneTemplateRepo(ctx context.Context, url string) (string, error) {
	tmp, err := os.MkdirTemp("", "spin-template-*")
	if err != nil {
		return "", fmt.Errorf("mkdir tempdir: %w", err)
	}

	cloneCtx, cancel := context.WithTimeout(ctx, CloneTemplateRepoTimeout)
	defer cancel()

	// --depth 1 keeps the clone cheap; spin only needs HEAD. The `--`
	// separator prevents a url beginning with `-` from being parsed
	// as a git flag.
	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--", url, tmp)
	cmd.Env = append(os.Environ(), gitEnv...)
	// On context expiry, force the I/O pipes closed so CombinedOutput's
	// drainer returns instead of blocking on Read.
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
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf(
				"git clone timed out after %s; check the URL or your network",
				CloneTemplateRepoTimeout,
			)
		}
		return "", fmt.Errorf("git clone %s failed:\n%s", url, strings.TrimSpace(string(out)))
	}

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
