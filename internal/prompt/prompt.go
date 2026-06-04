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
// Plan 01 shipped the contract surface only. Plan 02 wired the huh v2
// in-process backend (fillWithHuh in huh.go). Plan 03 adds the gum
// shell-out backend (fillWithGum in gum.go) and a resolveBackend() call
// in Fill that picks backendGum when gum is on $PATH and a `gum --version`
// sanity check exits 0; otherwise backendHuh.
package prompt

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

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

// backend is the resolved prompt backend for a single Fill call. The
// choice is made once at Fill entry (in resolveBackend) and never
// re-evaluated; per the plan, the backend never switches mid-flow.
type backend int

const (
	// backendNone is the unreachable-in-practice default returned by
	// resolveBackend when both gum and huh are unavailable. Defensive:
	// only fires if ShouldPrompt() returned true (TTY + not CI + not
	// --no-interactive) AND resolveBackend cannot find gum, which
	// should never happen on a real terminal because the huh backend
	// is always built in.
	backendNone backend = iota
	backendGum
	backendHuh
)

// String returns the lowercase name used in the Debug log line per
// UI-SPEC §"gum vs huh decision": `"prompt backend" backend=gum` or
// `backend=huh`.
func (b backend) String() string {
	switch b {
	case backendGum:
		return "gum"
	case backendHuh:
		return "huh"
	default:
		return "none"
	}
}

// gumLookPath is the test seam for exec.LookPath. Default is the
// real exec.LookPath; tests in prompt_backend_test.go override it
// to simulate gum being present or absent on $PATH.
var gumLookPath = exec.LookPath

// gumVersionCheck is the test seam for the `gum --version` sanity
// probe. Default runs the real `gum --version` with a 2-second
// timeout; tests override it to simulate a broken gum install
// (RESEARCH §Pitfall 3). The probe contract: return nil if gum is
// healthy, non-nil if it should be rejected.
var gumVersionCheck = func(path string) error {
	verCtx, verCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer verCancel()
	return exec.CommandContext(verCtx, path, "--version").Run()
}

// resolveBackend decides once which prompt backend to use for this
// Fill invocation. The result is locked for the duration of the call;
// we never switch backends mid-flow (per UI-SPEC §"gum vs huh decision").
//
// Resolution order (per UI-SPEC §"gum vs huh decision" + RESEARCH
// Pattern 1 + Pitfall 3):
//
//  1. SPIN_USE_HUH=1 forces backendHuh. Escape hatch for users who
//     want the in-process backend for some reason (debugging, no
//     charmbracelet feel wanted, etc.). Documented in the install hint.
//  2. exec.LookPath("gum") finds the binary on $PATH AND a `gum --version`
//     sanity check exits 0 within 2s → backendGum. A non-zero exit
//     (corrupt install, missing shared lib, weird permission) falls
//     through to backendHuh per RESEARCH §Pitfall 3.
//  3. Otherwise → backendHuh (the in-process fallback is always
//     available; the only thing missing is the TTY, which ShouldPrompt
//     already verified).
func resolveBackend() backend {
	if os.Getenv("SPIN_USE_HUH") == "1" {
		return backendHuh
	}
	path, err := gumLookPath("gum")
	if err != nil {
		return backendHuh
	}
	// Sanity check: `gum --version` must exit 0 within 2s. A non-zero
	// exit (or a hang) means a broken install; fall back to huh rather
	// than failing the whole scaffolder (RESEARCH §Pitfall 3).
	if err := gumVersionCheck(path); err != nil {
		return backendHuh
	}
	return backendGum
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
// The backend (gum or huh) is decided once via resolveBackend at
// Fill entry; the decision is logged at Debug level per UI-SPEC
// §"gum vs huh decision".
func Fill(p *scaffold.Project) error {
	if !ShouldPrompt() {
		return nil
	}
	if p == nil {
		return nil
	}
	be := resolveBackend()
	logBackend(be.String())
	switch be {
	case backendGum:
		return fillWithGum(p)
	case backendHuh:
		return fillWithHuh(p)
	default:
		// Defensive: ShouldPrompt() returned true (TTY + not CI +
		// not --no-interactive) but resolveBackend() returned
		// backendNone. In practice this is unreachable because
		// backendHuh is always built in. If we ever land here, the
		// TTY disappeared between the two checks (e.g., user
		// detached); treat as cancel per RESEARCH Pattern 1.
		return &Canceled{Reason: "no TTY available for prompts"}
	}
}
