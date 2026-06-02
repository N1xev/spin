// Command spin is a Go project scaffolder for the charmbracelet v2 ecosystem.
//
// One command produces a ready-to-run Go project pre-wired with charmbracelet
// libraries, modern Go tooling, and the prism test runner.
package main

import (
	"context"
	"os"

	"charm.land/fang/v2"

	"github.com/example/spin/cmd"
	"github.com/example/spin/internal/version"
)

func main() {
	if err := fang.Execute(
		context.Background(),
		cmd.RootCmd(),
		fang.WithVersion(version.Version),
	); err != nil {
		os.Exit(1)
	}
}
