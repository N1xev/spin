package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/wrap"
)

var testCmd = &cobra.Command{
	Use:           "test",
	Short:         "Run tests (uses prism if Go 1.24+ and on $PATH, else go test)",
	Args:          cobra.NoArgs,
	RunE:          func(cmd *cobra.Command, args []string) error { return wrap.Test() },
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(testCmd)
}
