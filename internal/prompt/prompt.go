// Package prompt owns the interactive prompt layer for `spin new`.
//
// This is the single chokepoint through which every prompt UI call must
// pass. The contract is:
//
//   - Fill(p *scaffold.Project) error populates any unset required fields
//     on p by asking the user. It writes back into p in place.
//   - ShouldPrompt() bool is the three-layer guard (env var, TTY, CI env
//     vars) that every prompt UI call site must consult before showing
//     a widget. When it returns false, Fill is a no-op.
//   - Canceled is a typed error that propagates to main.go for exit-code
//     mapping (exit 130 on cancellation, per UI-SPEC §"Cancellation /
//     cleanup").
//
// In Plan 01 this package ships the contract surface only — Fill is a
// documented no-op, and ShouldPrompt delegates to IsInteractive (in
// detect.go). Plans 02 and 03 wire the huh v2 and gum backends into Fill
// respectively.
package prompt

import (
	"errors"

	"github.com/example/spin/internal/scaffold"
)

// ErrCanceled is the sentinel that *Canceled.Is matches against. Callers
// that want to detect a user cancellation independent of the typed error
// (e.g., log statements) can use errors.Is(err, prompt.ErrCanceled).
var ErrCanceled = errors.New("prompt canceled by user")

// Canceled is the typed error returned by Fill when the user cancels
// (Ctrl-C, Esc, or any other abort path). The Reason field is a
// human-readable explanation suitable for logging.
//
// The main boundary in main.go matches this with errors.As to map it
// to exit code 130 (UI-SPEC §"Cancellation / cleanup").
type Canceled struct {
	Reason string
}

// Error returns "spin: <Reason>". The "spin:" prefix matches the rest
// of the scaffolder's user-facing error messages for grep-friendly logs.
func (c *Canceled) Error() string {
	return "spin: " + c.Reason
}

// Is reports whether target is ErrCanceled. This makes the type matchable
// with both errors.Is(err, ErrCanceled) and errors.As(err, &c).
func (c *Canceled) Is(target error) bool {
	return target == ErrCanceled
}

// ShouldPrompt is the public chokepoint that every prompt UI call site
// must consult. It delegates to IsInteractive (in detect.go) for the
// three-layer guard (env var → TTY → CI env vars).
//
// Note: p.NoInteractive is NOT consulted here. The contract is that
// cmd/new.go reads p.NoInteractive AFTER ResolveFlags and skips the
// Fill call entirely when the user passed --no-interactive / --yes /
// --batch. The split keeps ShouldPrompt's three-layer check independent
// of the flag plumbing.
func ShouldPrompt() bool {
	return IsInteractive()
}

// Fill populates any unset required fields on p by asking the user. It
// writes back into p in place. If ShouldPrompt() returns false, Fill
// returns nil without modifying p and any missing fields will surface
// in the subsequent p.Validate() call.
//
// If the user cancels (Ctrl-C, Esc, empty-after-retry), Fill returns a
// *Canceled error; the caller exits 130.
//
// Plan 02 wires the huh backend; Plan 03 wires the gum backend. In
// Plan 01 the body is a documented no-op that returns nil when prompting
// is allowed — the chokepoint is established so the rest of the system
// can wire against it without churning when the backends land.
func Fill(p *scaffold.Project) error {
	if !ShouldPrompt() {
		return nil
	}
	// Plan 02 wires the huh backend; Plan 03 wires the gum backend.
	// For Plan 01 the chokepoint is established but no prompts fire.
	_ = p
	return nil
}
