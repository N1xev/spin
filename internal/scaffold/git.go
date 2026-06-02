// Package scaffold: post-scaffold git init hook.
//
// GitInit runs `git init -b main`, `git add .`, and `git commit` in the
// generated project. All three invocations set GIT_TERMINAL_PROMPT=0 plus
// the four GIT_AUTHOR_*/GIT_COMMITTER_* env vars so the scaffolder never
// blocks on credentials and never fails when the user has no global git
// identity configured.
//
// SCAF-04 (generated project has git init + initial commit) is enforced
// here.
package scaffold

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// gitEnv is the env-guard set for every git invocation. RESEARCH §12.3:
//   - GIT_TERMINAL_PROMPT=0 prevents git from blocking on credentials
//   - GIT_AUTHOR_NAME/EMAIL and GIT_COMMITTER_NAME/EMAIL override any
//     missing global identity (common in fresh CI containers)
var gitEnv = []string{
	"GIT_TERMINAL_PROMPT=0",
	"GIT_AUTHOR_NAME=spin",
	"GIT_AUTHOR_EMAIL=spin@localhost",
	"GIT_COMMITTER_NAME=spin",
	"GIT_COMMITTER_EMAIL=spin@localhost",
}

// GitInit initializes a git repo in ./<Project.Name>/ and creates one
// initial commit with the message "scaffold <name> with spin <SpinVer>".
//
// Behavior:
//   - p.NoGit == true -> return nil immediately, no exec
//   - git not on $PATH -> log warning, return nil (don't fail the scaffold)
//   - any git command exits non-zero -> return a wrapped error with stderr
func (p *Project) GitInit() error {
	if p.NoGit {
		return nil
	}

	root := filepath.Join(".", p.Name)
	log.Info("initializing git", "path", root)

	// 1. git init -b main. If git is < 2.28 (which predates -b), the
	// unknown-flag error surfaces; we catch it and fall back to plain
	// `git init` + `git symbolic-ref HEAD refs/heads/main`. We don't try
	// to be clever about the version — the symbolic-ref fallback always
	// works.
	if out, err := runCmd(root, gitEnv, "git", "init", "-b", "main"); err != nil {
		if !isUnknownFlagErr(out, "b") {
			// git itself missing (e.g. not on $PATH): log warning, not fatal.
			if isNotFoundErr(err) {
				log.Warn("git not on $PATH; skipping git init", "hint", "install git or pass --no-git")
				return nil
			}
			return fmt.Errorf("git init failed in %s:\n%s", root, string(out))
		}
		// Fallback: plain `git init` + symbolic-ref.
		if out, err := runCmd(root, gitEnv, "git", "init"); err != nil {
			return fmt.Errorf("git init failed in %s:\n%s", root, string(out))
		}
		if out, err := runCmd(root, gitEnv, "git", "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
			return fmt.Errorf("git symbolic-ref failed in %s:\n%s", root, string(out))
		}
	}

	// 2. git add .
	if out, err := runCmd(root, gitEnv, "git", "add", "."); err != nil {
		return fmt.Errorf("git add failed in %s:\n%s", root, string(out))
	}

	// 3. git commit -m "scaffold <name> with spin <SpinVer>".
	msg := fmt.Sprintf("scaffold %s with spin %s", p.Name, p.SpinVer)
	if out, err := runCmd(root, gitEnv, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit failed in %s:\n%s", root, string(out))
	}

	return nil
}

// isUnknownFlagErr returns true if the git error output indicates that
// the -b flag is not recognized (git < 2.28). The check is conservative
// (looks for "unknown" or "unrecognized" + "b") so false positives are
// rare; we still fall back gracefully.
func isUnknownFlagErr(out []byte, shortFlag string) bool {
	s := string(out)
	return strings.Contains(s, "unknown option") && strings.Contains(s, "-"+shortFlag) ||
		strings.Contains(s, "unrecognized") && strings.Contains(s, "-"+shortFlag)
}

// isNotFoundErr returns true if err indicates "executable file not found
// in $PATH" (exec.ErrNotFound) or similar. We use errors.Is for safety.
func isNotFoundErr(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
