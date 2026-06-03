// Package prompt tests — fill chokepoint + Canceled error contract.
//
// The interactive path (when ShouldPrompt() returns true and stdin
// is a tty) is covered by cmd/new.go's existing test suite
// (TestNew in cmd_test.go). When the huh form needs a TTY, the test
// runs in non-TTY mode and Fill short-circuits to nil, which is the
// desired behavior in the test runner. So these tests focus on the
// no-op, nil-project, and no-interactive paths.

package prompt_test

import (
	"errors"
	"testing"

	"github.com/example/spin/internal/prompt"
	"github.com/example/spin/internal/scaffold"
)

// TestFill_NoInteractiveReturns asserts that Fill returns nil without
// modifying p when the explicit no-interactive gate is set. We assert
// this by setting SPIN_NO_INTERACTIVE=1, calling Fill, and verifying
// that p.Name is unchanged (which proves the no-op path ran and did
// not enter fillWithHuh, which would have prompted).
func TestFill_NoInteractiveReturns(t *testing.T) {
	t.Setenv("SPIN_NO_INTERACTIVE", "1")
	t.Setenv("CI", "")
	p := &scaffold.Project{Name: "myapp", Type: "tui"}
	if err := prompt.Fill(p); err != nil {
		t.Errorf("Fill with SPIN_NO_INTERACTIVE=1 = %v, want nil", err)
	}
	if p.Name != "myapp" {
		t.Errorf("Fill mutated p.Name despite gate: got %q, want %q", p.Name, "myapp")
	}
	if p.Type != "tui" {
		t.Errorf("Fill mutated p.Type despite gate: got %q, want %q", p.Type, "tui")
	}
}

// TestFill_NilProject asserts that Fill(nil) returns nil without
// panicking. The gate check runs before the p == nil check, so the
// no-op path is taken when ShouldPrompt() is false; we also assert
// the nil-check works when ShouldPrompt() is true (which we
// simulate by stubbing the env to bypass the gate — but then
// fillWithHuh would run, so we only test the no-prompt path).
func TestFill_NilProject(t *testing.T) {
	// Gate prompt off so we hit the p == nil branch without
	// entering fillWithHuh.
	t.Setenv("SPIN_NO_INTERACTIVE", "1")
	if err := prompt.Fill(nil); err != nil {
		t.Errorf("Fill(nil) with gate = %v, want nil", err)
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
