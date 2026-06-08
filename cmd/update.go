package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/update"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Go module deps (huh v2 form confirms before applying)",
	Long: "spin update is a universal Go dependency updater. It reads ./go.mod, fetches " +
		"the latest stable and latest (including pre-release) versions of each direct (or, " +
		"with --all, indirect) dep from proxy.golang.org, and shows a huh v2 form with one " +
		"Select per dep. Each field defaults to newStable; options are Skip, newStable, " +
		"newLatest. Submitting the form runs `go get` for each chosen dep, then `go mod " +
		"tidy`, then `CGO_ENABLED=0 go build ./...` as a smoke test. Does not run `go test`.",
	Args:          cobra.NoArgs,
	RunE:          runUpdate,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().Bool("all", false, "include indirect dependencies in the form (default: direct only)")
}

// runUpdate is the `spin update` RunE. It locates the nearest go.mod
// (walking up from cwd), then hands off to update.PromptForUpdate
// for the list → resolve → form → apply flow. All flag values
// resolve here; PromptForUpdate is the engine.
func runUpdate(cmd *cobra.Command, _ []string) error {
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	gomodPath, err := update.FindGoMod(".")
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	ctx := cmd.Context()
	if err := update.PromptForUpdate(ctx, update.PromptOptions{
		GoModPath:      gomodPath,
		IncludeIndirect: all,
		Log:            os.Stderr,
	}); err != nil {
		return err
	}
	return nil
}
