// Package cmd wires the spin cobra root command.
//
// rootCmd is a package-level variable so that subcommand files can attach
// themselves via init(). RootCmd() is the constructor accessor for callers
// (main, tests) and returns the same singleton.
package cmd

import (
	"github.com/spf13/cobra"
)

// version is the hardcoded spin version for the Walking Skeleton.
// Plan 02 will wire this to internal/version via ldflags so `go install`
// emits the correct version string at build time.
const version = "0.1.0"

// rootCmd is the cobra root command for spin. Subcommand files in this
// package attach themselves to rootCmd via init().
var rootCmd = &cobra.Command{
	Use:   "spin",
	Short: "Scaffold a charmbracelet v2 Go project",
	Long: "spin is a Go project scaffolder for the charmbracelet v2 ecosystem. " +
		"It generates ready-to-run Go projects pre-wired with charmbracelet " +
		"libraries, modern Go tooling, and the prism test runner. " +
		"One command produces a project that builds, tests, and runs without extra setup.",
	Version:                    version,
	SilenceUsage:               true,
	SilenceErrors:              true,
	SuggestionsMinimumDistance: 2,
}

// RootCmd returns the spin cobra root command with all subcommands attached.
// Tests use this to construct a fresh tree; main uses this as the entry point
// passed to fang.Execute.
func RootCmd() *cobra.Command {
	return rootCmd
}
