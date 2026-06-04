// Package prompt: TTY and CI environment detection for the three-layer
// prompt guard. ShouldPrompt is the single chokepoint per INT-03 — no
// gum or huh call site may prompt without consulting it.
//
// p.NoInteractive is NOT consulted here; cmd/new.go reads that flag
// before calling Fill, which keeps the env/TTY/CI check independent of
// cobra flag plumbing.
package prompt

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsInteractive reports whether the current invocation should run
// interactive prompts. Returns true iff ALL three layers pass:
// SPIN_NO_INTERACTIVE unset, os.Stdin is a TTY, and no CI env var is set.
func IsInteractive() bool {
	// Env-var override first so a CI env with an attached pty
	// (kubectl exec into a pod, debuggers) still respects intent.
	if os.Getenv("SPIN_NO_INTERACTIVE") != "" {
		return false
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return false
	}
	// Treat CI as non-interactive even when stdin is a tty — defense
	// in depth (RESEARCH §Pitfall 1 / env-vs-tty).
	if ciEnv() {
		return false
	}
	return true
}

func ciEnv() bool {
	return os.Getenv("CI") != "" ||
		os.Getenv("GITHUB_ACTIONS") != "" ||
		os.Getenv("GITLAB_CI") != "" ||
		os.Getenv("BUILDKITE") != "" ||
		os.Getenv("CIRCLECI") != ""
}
