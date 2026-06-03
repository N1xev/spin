// Command spin is a Go project scaffolder for the charmbracelet v2 ecosystem.
//
// One command produces a ready-to-run Go project pre-wired with charmbracelet
// libraries, modern Go tooling, and the prism test runner.
package main

import (
	"context"
	"errors"
	"os"

	"charm.land/fang/v2"

	"github.com/example/spin/cmd"
	"github.com/example/spin/internal/prompt"
	"github.com/example/spin/internal/version"
)

func main() {
	err := fang.Execute(
		context.Background(),
		cmd.RootCmd(),
		fang.WithVersion(version.Version),
	)
	if err == nil {
		return
	}
	// Phase 3: map *prompt.Canceled to exit 130. The typed error
	// carries a Reason (see internal/prompt/prompt.go); the Is() method
	// matches ErrCanceled, so errors.As can extract the struct for any
	// future logging in main. fang has already styled and printed the
	// error to stderr; we just need to pick the right exit code.
	// errors.As needs a non-nil pointer to *prompt.Canceled (not
	// *prompt.Canceled value — the error type is the pointer).
	var canceled *prompt.Canceled
	if errors.As(err, &canceled) {
		_ = canceled // Reason is preserved for future logging per T-03.01-R
		os.Exit(130)
	}
	os.Exit(1)
}
