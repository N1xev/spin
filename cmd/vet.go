package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/wrap"
)

var vetCmd = &cobra.Command{
	Use:           "vet",
	Short:         "Run go vet ./...",
	Args:          cobra.NoArgs,
	RunE:          func(cmd *cobra.Command, args []string) error { return wrap.Vet() },
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(vetCmd)
}
