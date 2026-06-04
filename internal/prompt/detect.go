// Package prompt: TTY and CI environment detection.
//
// p.NoInteractive is not consulted here; cmd/new.go reads that flag
// before calling Fill, so the env/TTY/CI check stays independent of
// cobra flag plumbing.
package prompt

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsInteractive reports whether the current invocation should run
// interactive prompts. Returns true iff SPIN_NO_INTERACTIVE is unset,
// os.Stdin is a TTY, and no CI env var is set.
func IsInteractive() bool {
	if os.Getenv("SPIN_NO_INTERACTIVE") != "" {
		return false
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return false
	}
	if ciEnv() {
		return false
	}
	return true
}

// ciEnv reports whether any well-known CI env var is set.
func ciEnv() bool {
	return os.Getenv("CI") != "" ||
		os.Getenv("GITHUB_ACTIONS") != "" ||
		os.Getenv("GITLAB_CI") != "" ||
		os.Getenv("BUILDKITE") != "" ||
		os.Getenv("CIRCLECI") != ""
}
