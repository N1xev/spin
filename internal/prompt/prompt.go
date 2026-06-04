// Package prompt owns the interactive prompt layer for `spin new`.
//
// Fill is the single chokepoint: it populates unset required fields
// on *scaffold.Project by asking the user. ShouldPrompt is the
// three-layer guard (env var, TTY, CI env vars) every prompt UI call
// site must consult. Canceled is a typed error that main.go maps to
// exit 130 (UI-SPEC §Cancellation / cleanup).
//
// Testability: os/exec seams live on the Deps struct (passed through
// fillWithDeps / resolveBackend), not as package-level mutable
// globals. Tests build a Deps with stubs and call the internal
// entry points directly — no shared state, so t.Parallel() is safe.
package prompt

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/example/spin/internal/scaffold"
)

// ErrCanceled matches via errors.Is(err, prompt.ErrCanceled). Use
// errors.As for the typed *Canceled when you need the Reason.
var ErrCanceled = errors.New("prompt canceled by user")

type Canceled struct {
	Reason string
}

func (c *Canceled) Error() string { return "spin: " + c.Reason }

// Is makes the type matchable with both errors.Is and errors.As.
func (c *Canceled) Is(target error) bool { return target == ErrCanceled }

type backend int

// backendNone is reserved as the zero value; resolveBackend never
// returns it. The Fill switch's default branch panics on it to
// catch a future regression adding a third backend.
const (
	backendNone backend = iota
	backendGum
	backendHuh
)

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

// Deps is the bag of injectable seams. The production values come
// from DefaultDeps(); tests build a Deps with stubs and pass it to
// the internal entry points.
type Deps struct {
	LookPath     func(string) (string, error)
	VersionCheck func(string) error
	Runner       func(context.Context, ...string) (string, error)
}

// DefaultDeps returns the production Deps: real exec.LookPath, a
// 2-second-timeout `gum --version` probe, and the real gumRunCapture.
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

// Resolution order (UI-SPEC §gum vs huh decision + RESEARCH Pitfall 3):
//  1. SPIN_USE_HUH=1 → backendHuh (escape hatch)
//  2. LookPath + VersionCheck pass → backendGum
//  3. Otherwise → backendHuh (always built in)
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

// ShouldPrompt is the three-layer guard. p.NoInteractive is NOT
// consulted here — cmd/new.go reads p.NoInteractive after
// ResolveFlags and skips Fill entirely when --no-interactive is set.
func ShouldPrompt() bool {
	return IsInteractive()
}

func Fill(p *scaffold.Project) error {
	return fillWithDeps(p, DefaultDeps())
}

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
		panic("spin: prompt: unreachable backendNone in Fill")
	}
}
