// Package scaffold: external template repo cloning.
//
// CloneTemplateRepo clones a user-supplied git URL to a fresh tempdir
// and validates that the resulting tree contains a `_base/` subdir
// (the spin overlay engine's required entry point). The caller owns
// the returned path; pass it to os.RemoveAll on completion, or set
// p.KeepTemplateCache to retain it for debugging.
//
// Lifecycle:
//   - ctx     is plumbed for future cancellation; Plan 02-02 calls
//             with context.Background().
//   - The tempdir is auto-removed on clone failure (non-zero exit
//     from `git clone` OR missing _base/ subdir) so a failed attempt
//     does not leak dirs into the OS temp area.
//   - On success, the caller MUST call os.RemoveAll(dir) when done.
//             Or set Project.KeepTemplateCache=true to retain it.
//
// Security:
//   - GIT_TERMINAL_PROMPT=0 prevents git from blocking on credentials
//             (RESEARCH §5.1). Users with private repos configure
//             GIT_SSH_COMMAND or ssh-agent separately.
//   - The four GIT_AUTHOR_*/GIT_COMMITTER_* vars match the values
//             used by GitInit (git.go) so a fresh CI container with
//             no global git identity can still clone.
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

// CloneTemplateRepoTimeout caps how long a `git clone` may take before
// the scaffolder gives up. CR-005: a slow or dead remote used to freeze
// the scaffolder indefinitely; a 60s ceiling is generous for a `git
// clone --depth 1` of a typical template repo while still being a fast
// failure for unreachable hosts.
//
// Declared as a `var` (not const) so the timeout test in repo_test.go
// can lower it to a sub-second value without hanging the suite.
var CloneTemplateRepoTimeout = 60 * time.Second

// CloneTemplateRepo clones url to a fresh tempdir and validates that
// the result has a `_base/` subdir (the spin overlay engine entry
// point). Returns the tempdir path on success.
//
// On any failure (git clone non-zero exit, missing _base/), the
// tempdir is removed and the error is wrapped with the underlying
// git stderr (when available) so the user can see what went wrong.
//
// The clone is wrapped in a 60-second timeout (CloneTemplateRepoTimeout)
// so a slow or dead remote cannot freeze the scaffolder. On timeout
// the returned error names the duration so the user can distinguish
// "network too slow" from "wrong URL".
//
// Requires `git` on $PATH; if missing, returns an exec.ErrNotFound-
// wrapped error (callers can use errors.Is(err, exec.ErrNotFound)
// to surface a "git not installed" message).
func CloneTemplateRepo(ctx context.Context, url string) (string, error) {
	tmp, err := os.MkdirTemp("", "spin-template-*")
	if err != nil {
		return "", fmt.Errorf("mkdir tempdir: %w", err)
	}

	// CR-005: wrap ctx with a 60s timeout so a slow remote cannot
	// freeze the scaffolder. The derived context is passed to
	// exec.CommandContext, which sends SIGKILL on expiry.
	cloneCtx, cancel := context.WithTimeout(ctx, CloneTemplateRepoTimeout)
	defer cancel()

	// Run `git clone --depth 1 -- <url> <tmp>`. We pass --depth 1 to keep
	// the clone cheap; spin does not need history, only HEAD. The `--`
	// separator is defense-in-depth (CR-004): without it, a `url` like
	// "-upload-pack=evil" would be interpreted by git as a flag rather
	// than a positional. The validator (IsValidTemplateRepo) is the
	// primary gate; the `--` is the belt to its suspenders.
	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--", url, tmp)
	cmd.Env = append(os.Environ(), gitEnv...)
	// CR-005 (timeout propagation): when the context expires, force
	// the I/O pipes closed so CombinedOutput's drainer goroutines
	// return instead of blocking on Read. Without this, a SIGKILL
	// that git ignores (e.g. blocked on a network read) leaves the
	// pipes open and CombinedOutput hangs forever. cmd.Cancel was
	// added in Go 1.20 and is the standard way to handle this.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Best-effort cleanup; ignore the remove error since we already
		// have a more interesting error to report.
		_ = os.RemoveAll(tmp)
		// CR-005: surface a clear timeout error. We use errors.Is
		// against context.DeadlineExceeded because exec.CommandContext
		// kills the child on ctx cancellation and propagates the ctx
		// error as the cmd error.
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf(
				"git clone timed out after %s; check the URL or your network",
				CloneTemplateRepoTimeout,
			)
		}
		return "", fmt.Errorf("git clone %s failed:\n%s", url, strings.TrimSpace(string(out)))
	}

	// Validate the cloned tree has the required entry point. spin's
	// overlay engine starts at templates/_base/ in the embedded FS;
	// an external repo must follow the same convention or the walker
	// finds no files to render.
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
