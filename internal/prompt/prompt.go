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
//
// Testability: the package's os/exec seams (LookPath, gum --version probe,
// gum subprocess invocation) are collected on the Deps struct, not as
// package-level mutable globals. Tests construct a Deps with stub
// functions and pass it to the internal fillWithDeps / resolveBackend
// entry points. This eliminates the race-condition footgun of the
// previous design (gumLookPath / gumVersionCheck / gumRunner / gumCtx
// were all package globals; a future test using t.Parallel() would have
// raced on them).
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
	// backendNone is reserved as the zero value; resolveBackend never
	// returns it (it always returns backendGum or backendHuh). The
	// Fill switch's default branch panics on this value to catch a
	// future regression that adds a third backend without updating
	// the dispatch.
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

// Deps is the bag of injectable seams for the prompt package. The
// production values come from DefaultDeps(); tests construct a Deps
// with stub functions and pass it to the internal fillWithDeps /
// resolveBackend entry points. Keeping the seams on a value type
// (passed through call sites) — rather than as package-level mutable
// globals — eliminates the race-condition footgun of the previous
// design and lets tests run with t.Parallel() safely.
type Deps struct {
	// LookPath is exec.LookPath; the resolveBackend probe uses it to
	// decide whether gum is on $PATH. Tests stub this to simulate gum
	// being present or absent.
	LookPath func(string) (string, error)

	// VersionCheck runs `gum --version` against the path LookPath
	// returned; nil means "healthy install". Tests stub this to
	// simulate a healthy or broken install (RESEARCH §Pitfall 3: a
	// corrupt install must not break the scaffolder — it falls
	// back to backendHuh).
	VersionCheck func(string) error

	// Runner is the gum subprocess invocation used by every widget
	// wrapper (gumChoose, gumInput, gumMultiSelect, gumConfirm). The
	// production value is gumRunCapture. Tests stub this to record
	// the arg list and return canned answers.
	Runner func(context.Context, ...string) (string, error)
}

// DefaultDeps returns the production Deps: real exec.LookPath, a
// 2-second-timeout `gum --version` probe, and the real gumRunCapture
// as the runner. Called by Fill on entry; tests bypass this and
// build their own Deps.
func DefaultDeps() Deps {
	return Deps{
		LookPath: exec.LookPath,
		VersionCheck: func(path string) error {
			verCtx, verCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer verCancel()
			return exec.CommandContext(verCtx, path, "--version").Run()
		},
		Runner: gumRunCapture,
	}
}

// resolveBackend picks the prompt backend for a single Fill call.
// The result is locked for the duration of the call; we never switch
// backends mid-flow (per UI-SPEC §"gum vs huh decision").
//
// Resolution order (per UI-SPEC §"gum vs huh decision" + RESEARCH
// Pattern 1 + Pitfall 3):
//
//  1. SPIN_USE_HUH=1 forces backendHuh. Escape hatch for users who
//     want the in-process backend for some reason (debugging, no
//     charmbracelet feel wanted, etc.). Documented in the install hint.
//  2. deps.LookPath("gum") finds the binary on $PATH AND deps.VersionCheck
//     exits 0 within 2s → backendGum. A non-zero exit (corrupt install,
//     missing shared lib, weird permission) falls through to backendHuh
//     per RESEARCH §Pitfall 3.
//  3. Otherwise → backendHuh (the in-process fallback is always
//     available; the only thing missing is the TTY, which ShouldPrompt
//     already verified).
func resolveBackend(deps Deps) backend {
	if os.Getenv("SPIN_USE_HUH") == "1" {
		return backendHuh
	}
	path, err := deps.LookPath("gum")
	if err != nil {
		return backendHuh
	}
	if err := deps.VersionCheck(path); err != nil {
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
	return fillWithDeps(p, DefaultDeps())
}

// fillWithDeps is the internal entry point used by tests. It takes the
// injectable Deps explicitly so tests don't have to mutate any
// package-level state.
func fillWithDeps(p *scaffold.Project, deps Deps) error {
	if !ShouldPrompt() {
		return nil
	}
	if p == nil {
		return nil
	}
	be := resolveBackend(deps)
	logBackend(be.String())
	switch be {
	case backendGum:
		return fillWithGumDeps(p, deps)
	case backendHuh:
		return fillWithHuh(p)
	default:
		// Unreachable: resolveBackend() only returns backendGum or
		// backendHuh. A future regression adding a third backend
		// without updating this switch is a bug — panic loudly so
		// the regression is caught immediately rather than masked
		// by a fake cancellation.
		panic("spin: prompt: unreachable backendNone in Fill")
	}
}
