// Package prompt owns the interactive prompt layer for `spin new`.
//
// Fill populates unset required fields on *scaffold.Project by asking
// the user. IsInteractive is the three-layer guard (env var, TTY, CI)
// every prompt UI call site must consult. ErrCanceled is a typed
// error that callers map to exit code 130.
package prompt

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/example/spin/internal/scaffold"
)

// ErrCanceled is matched by errors.Is; the typed *Canceled carries Reason.
var ErrCanceled = errors.New("prompt canceled by user")

// Canceled describes a user-initiated cancellation.
type Canceled struct {
	Reason string
}

func (c *Canceled) Error() string { return "spin: " + c.Reason }

// Is makes *Canceled matchable with both errors.Is and errors.As.
func (c *Canceled) Is(target error) bool { return target == ErrCanceled }

type backend int

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

// Deps is the bag of injectable seams. Production values come from
// DefaultDeps; tests build a Deps with stubs and pass it to the
// internal entry points.
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

// resolveBackend returns the active prompt backend. SPIN_USE_HUH=1
// forces the huh backend; otherwise gum is used when present and
// responsive. The huh backend is the final fallback.
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

// ShouldPrompt reports whether the current invocation should run
// interactive prompts. p.NoInteractive is not consulted here;
// cmd/new.go reads that flag before calling Fill.
func ShouldPrompt() bool {
	return IsInteractive()
}

// Fill populates p's required fields by asking the user.
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
