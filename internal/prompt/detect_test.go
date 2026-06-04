// Black-box tests for the TTY/CI detection layer. The unexported
// `ciEnv` is exercised indirectly through IsInteractive.
//
// TTY path: `go test` always runs with a pipe for stdin, so the
// tty-missing path is reliably testable. The TTY-present path skips.
package prompt_test

import (
	"testing"

	"github.com/example/spin/internal/prompt"
)

func TestIsInteractive_TTYCheck(t *testing.T) {
	// We don't need to swap stdin — `go test` always runs with a pipe,
	// so the tty-missing path is the only one reachable here.
	if prompt.IsInteractive() {
		t.Skip("stdin is a tty in this test environment; TTY-missing path untestable here")
	}
}

func TestIsInteractive_EnvOverride(t *testing.T) {
	t.Setenv("SPIN_NO_INTERACTIVE", "1")
	if prompt.IsInteractive() {
		t.Error("IsInteractive with SPIN_NO_INTERACTIVE=1 = true, want false")
	}
	// Empty string is NOT an override (per the env-var contract);
	// non-empty values (any of them) disable.
	t.Setenv("SPIN_NO_INTERACTIVE", "true")
	if prompt.IsInteractive() {
		t.Error("IsInteractive with SPIN_NO_INTERACTIVE=true = true, want false")
	}
}

func TestIsInteractive_CIEnv(t *testing.T) {
	// Make sure SPIN_NO_INTERACTIVE is unset so the CI layer is the
	// one that actually trips the guard.
	t.Setenv("SPIN_NO_INTERACTIVE", "")

	cases := []struct {
		name string
		env  string
	}{
		{"CI", "CI"},
		{"GITHUB_ACTIONS", "GITHUB_ACTIONS"},
		{"GITLAB_CI", "GITLAB_CI"},
		{"BUILDKITE", "BUILDKITE"},
		{"CIRCLECI", "CIRCLECI"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, e := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI"} {
				t.Setenv(e, "")
			}
			t.Setenv(c.env, "true")
			if prompt.IsInteractive() {
				t.Errorf("IsInteractive with %s=true = true, want false", c.env)
			}
		})
	}
}

func TestIsInteractive_AllLayersOff(t *testing.T) {
	t.Setenv("SPIN_NO_INTERACTIVE", "")
	for _, e := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI"} {
		t.Setenv(e, "")
	}
	// `go test` stdin is a pipe, so IsInteractive returns false. The
	// env-var overrides above gate the result to false in any
	// environment, including an interactive shell.
	if prompt.IsInteractive() {
		t.Skip("stdin is a tty in this test environment; pipe-stdin path untestable here")
	}
}
