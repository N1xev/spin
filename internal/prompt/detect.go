// Package prompt: TTY and CI environment detection for the three-layer
// prompt guard.
//
// This file is a Plan 01 stub. Plan 02 (Task 2 of 03-01) replaces the
// IsInteractive body with the full three-layer guard:
//
//  1. SPIN_NO_INTERACTIVE env var wins (set to any non-empty value).
//  2. isatty.IsTerminal(os.Stdin) must be true.
//  3. None of the CI env vars ($CI, $GITHUB_ACTIONS, $GITLAB_CI,
//     $BUILDKITE, $CIRCLECI) may be set.
//
// For Plan 01 we ship the function with a permissive body (always
// false) so the package compiles and the ShouldPrompt → IsInteractive
// delegation chain is exercisable in tests. Plan 02's tests will assert
// the actual guard behavior.
package prompt

// IsInteractive reports whether the current invocation should run
// interactive prompts. Plan 02 replaces this stub with the full
// three-layer guard (see file header).
func IsInteractive() bool {
	return false
}
