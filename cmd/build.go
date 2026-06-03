package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/wrap"
)

var buildCmd = &cobra.Command{
	Use:           "build",
	Short:         "Build the project (produces bin/<name>, CGO disabled)",
	Args:          cobra.NoArgs,
	RunE:          func(cmd *cobra.Command, args []string) error { return wrap.Build() },
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
