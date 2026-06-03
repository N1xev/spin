// Package prompt tests — Plan 01 ships the contract surface only, so
// these tests assert the no-op behavior. Plans 02 and 03 extend this
// file with backend-specific assertions (gum + huh).

package prompt_test

import (
	"errors"
	"testing"

	"github.com/example/spin/internal/prompt"
	"github.com/example/spin/internal/scaffold"
)

// TestFillNoop asserts that Fill on a fresh *scaffold.Project returns
// nil when prompting is gated off (CI env or no TTY). The CI test runner
// is non-interactive by default, so ShouldPrompt returns false and Fill
// is a documented no-op.
//
// When the real prompts land in Plans 02/03, this test still passes
// (it only asserts "no error") but new tests will be added to exercise
// the actual fill behavior.
func TestFillNoop(t *testing.T) {
	p := &scaffold.Project{}
	if err := prompt.Fill(p); err != nil {
		t.Errorf("Fill(zero) = %v, want nil (Plan 01 no-op)", err)
	}
}

// TestCanceledErrorIs asserts that *prompt.Canceled matches ErrCanceled
// via errors.Is. This is the contract that main.go depends on for
// exit-code mapping (errors.As also relies on this Unwrap-free
// Is-method match — see main.go's branch in Plan 01 Task 4).
func TestCanceledErrorIs(t *testing.T) {
	c := &prompt.Canceled{Reason: "user pressed Ctrl-C"}
	if got := c.Error(); got != "spin: user pressed Ctrl-C" {
		t.Errorf("Canceled.Error() = %q, want %q", got, "spin: user pressed Ctrl-C")
	}
	if !errors.Is(c, prompt.ErrCanceled) {
		t.Error("errors.Is(Canceled, ErrCanceled) = false, want true")
	}
}
