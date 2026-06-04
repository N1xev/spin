// Package scaffold: post-scaffold git init hook.
package scaffold

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// gitEnv disables credential prompts and supplies a default identity
// for any git invocation. The values are sufficient for the local
// initial commit and prevent failures in fresh CI containers that
// have no global git identity.
var gitEnv = []string{
	"GIT_TERMINAL_PROMPT=0",
	"GIT_AUTHOR_NAME=spin",
	"GIT_AUTHOR_EMAIL=spin@localhost",
	"GIT_COMMITTER_NAME=spin",
	"GIT_COMMITTER_EMAIL=spin@localhost",
}

// GitInit initializes a git repo in ./p.Name/ and creates one initial
// commit. It is a no-op when p.NoGit is set or git is not on $PATH.
func (p *Project) GitInit() error {
	if p.NoGit {
		return nil
	}

	root := filepath.Join(".", p.Name)
	log.Info("initializing git", "path", root)

	// git < 2.28 does not understand -b; fall back to plain init +
	// symbolic-ref. The fallback always works regardless of version.
	if out, err := runCmd(root, gitEnv, "git", "init", "-b", "main"); err != nil {
		if !isUnknownFlagErr(out, "b") {
			if isNotFoundErr(err) {
				log.Warn("git not on $PATH; skipping git init", "hint", "install git or pass --no-git")
				return nil
			}
			return fmt.Errorf("git init failed in %s:\n%s", root, string(out))
		}
		if out, err := runCmd(root, gitEnv, "git", "init"); err != nil {
			return fmt.Errorf("git init failed in %s:\n%s", root, string(out))
		}
		if out, err := runCmd(root, gitEnv, "git", "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
			return fmt.Errorf("git symbolic-ref failed in %s:\n%s", root, string(out))
		}
	}

	if out, err := runCmd(root, gitEnv, "git", "add", "."); err != nil {
		return fmt.Errorf("git add failed in %s:\n%s", root, string(out))
	}

	msg := fmt.Sprintf("scaffold %s with spin %s", p.Name, p.SpinVer)
	if out, err := runCmd(root, gitEnv, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit failed in %s:\n%s", root, string(out))
	}

	return nil
}

// isUnknownFlagErr reports whether out indicates that the given short
// flag is not recognized. Older git versions and some distro patches
// use "unknown switch" or "unrecognized switch" instead of the
// canonical "unknown option" / "unrecognized option".
func isUnknownFlagErr(out []byte, shortFlag string) bool {
	s := string(out)
	hasFlag := strings.Contains(s, "-"+shortFlag) || strings.Contains(s, "`"+shortFlag)
	if !hasFlag {
		return false
	}
	clauses := []string{
		"unknown option",
		"unknown switch",
		"unrecognized option",
		"unrecognized switch",
	}
	for _, c := range clauses {
		if strings.Contains(s, c) {
			return true
		}
	}
	return false
}

// isNotFoundErr reports whether err indicates the executable was
// not found on $PATH.
func isNotFoundErr(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
