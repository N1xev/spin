package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/wrap"
)

var fmtCmd = &cobra.Command{
	Use:           "fmt [path ...]",
	Short:         "Format with gofumpt → goimports → gofmt (pass --no-strict to skip gofumpt)",
	Args:          cobra.ArbitraryArgs,
	RunE:          runFmt,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(fmtCmd)
	fmtCmd.Flags().Bool("no-strict", false, "skip the gofumpt step (still runs goimports + gofmt)")
}

func runFmt(cmd *cobra.Command, args []string) error {
	noStrict, _ := cmd.Flags().GetBool("no-strict")
	return wrap.Fmt(noStrict)
}
