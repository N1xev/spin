package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/N1xev/spin/internal/version"
)

var versionCmd = &cobra.Command{
	Use:           "version",
	Short:         "Print the spin version",
	Args:          cobra.NoArgs,
	Run:           func(cmd *cobra.Command, args []string) { fmt.Println(version.Version) },
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
