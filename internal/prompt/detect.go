// Package prompt: TTY and CI environment detection for the three-layer
// prompt guard (UI-SPEC §"TTY guard").
//
// The three layers, evaluated in order:
//  1. SPIN_NO_INTERACTIVE env var wins (any non-empty value disables).
//  2. isatty.IsTerminal(os.Stdin) must report true.
//  3. None of the CI env vars ($CI, $GITHUB_ACTIONS, $GITLAB_CI,
//     $BUILDKITE, $CIRCLECI) may be set to a non-empty value.
//
// If ANY layer is false, prompting is disabled. This is the single
// chokepoint per INT-03 — no gum or huh call site is allowed to invoke
// a prompt without first consulting ShouldPrompt (which calls
// IsInteractive).
//
// p.NoInteractive is NOT consulted here; that flag is read in
// cmd/new.go before calling Fill, and Fill skips the call entirely
// when the user passed --no-interactive / --yes / --batch. The split
// keeps the env/TTY/CI check independent of the cobra flag plumbing.
package prompt

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsInteractive reports whether the current invocation should run
// interactive prompts. Returns true iff ALL three layers pass:
//
//  1. SPIN_NO_INTERACTIVE env var is empty (set to any non-empty value
//     to disable, matching the de-facto standard for "non-interactive
//     override" env vars in POSIX tooling).
//  2. os.Stdin is a terminal (isatty.IsTerminal on the file descriptor).
//  3. None of the five CI env vars are set ($CI, $GITHUB_ACTIONS,
//     $GITLAB_CI, $BUILDKITE, $CIRCLECI).
//
// On any false return, prompt UI must NOT be shown. Callers handle
// missing-required-field errors via the existing ArgError/FlagError
// path (the Phase 2 contract — see RESEARCH §Pitfall 1).
func IsInteractive() bool {
	// Layer 1: explicit env-var override. Checked first so a CI
	// environment with an attached pty (some debuggers / kubectl
	// exec into a pod) still respects the user's intent.
	if os.Getenv("SPIN_NO_INTERACTIVE") != "" {
		return false
	}
	// Layer 2: TTY check. isatty.IsTerminal handles Windows, Cygwin,
	// MSYS2, and the various Unix flavors; we don't roll our own
	// stat/ioctl here (UI-SPEC §"TTY guard"; RESEARCH §"Don't
	// Hand-Roll"). The function takes a uintptr fd, not *os.File,
	// so we call .Fd() to extract the descriptor.
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return false
	}
	// Layer 3: CI env detection. Per RESEARCH §"Pitfall 1 / env-vs-tty",
	// we treat CI as non-interactive even when stdin is a tty — defense
	// in depth.
	if ciEnv() {
		return false
	}
	return true
}

// ciEnv reports whether any of the well-known CI env vars are set to a
// non-empty value. We use os.Getenv (not LookupEnv) because both
// "unset" and "empty string" are equivalent for our purposes — both
// mean "the user did not declare themselves to be in CI".
//
// Env vars checked:
//   - $CI              (generic, set by most modern CI systems)
//   - $GITHUB_ACTIONS  (GitHub Actions)
//   - $GITLAB_CI       (GitLab CI)
//   - $BUILDKITE       (Buildkite)
//   - $CIRCLECI        (CircleCI)
func ciEnv() bool {
	return os.Getenv("CI") != "" ||
		os.Getenv("GITHUB_ACTIONS") != "" ||
		os.Getenv("GITLAB_CI") != "" ||
		os.Getenv("BUILDKITE") != "" ||
		os.Getenv("CIRCLECI") != ""
}
