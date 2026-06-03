// Package cmd wires spin's cobra subcommands. The wrapper subcommands
// in this directory (run, build, test, vet, fmt) are SCAFFOLDER-SIDE —
// they wrap the Go toolchain for the project the user is in, NOT for
// the spin repo itself. The scaffolder side (cmd/new.go) generates a
// project; the wrapper side (this file + the rest) operates on the
// generated project.
//
// Each wrapper is a thin cobra adapter: ~15 lines that compose a
// ToolSpec (in internal/wrap) and call into it. The shared init()
// pattern attaches the subcommand to rootCmd.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/wrap"
)

var runCmd = &cobra.Command{
	Use:           "run",
	Short:         "Run the project (uses air if .air.toml present, else go run .)",
	Args:          cobra.NoArgs,
	RunE:          func(cmd *cobra.Command, args []string) error { return wrap.Run() },
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(runCmd)
}
