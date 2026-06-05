package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/wrap"
)

var lintCmd = &cobra.Command{
	Use:   "lint [golangci-lint args]",
	Short: "Run golangci-lint (passes all args through)",
	Long: "Run golangci-lint against the current module. All arguments are forwarded to " +
		"golangci-lint, so `spin lint run`, `spin lint cache clean`, `spin lint version`, etc. " +
		"all work. If golangci-lint is not on $PATH, spin prints an install hint and exits " +
		"non-zero. Install with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest",
	Args:          cobra.ArbitraryArgs,
	RunE:          runLint,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(lintCmd)
}

func runLint(cmd *cobra.Command, args []string) error {
	return wrap.Lint(args)
}
