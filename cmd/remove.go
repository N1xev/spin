// Package cmd: spin remove.
//
// `spin remove <name>` (alias: `rm`) drops a pin from
// pinned.json. The on-disk cache is left alone by default; the
// user can opt into deleting the cache with `--purge`. The reason
// for the opt-in: a separate `spin add <name>` re-uses the cache
// when it's still there, so a user who removes a pin to "start
// over" is surprised when the next `spin add` skips the network
// round-trip and reuses their stale cache. `--purge` makes the
// intent explicit.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/N1xev/spin/internal/registry"
)

var removeCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a pinned template",
	Long:    "Remove a template from the pinned list. The on-disk cache is left alone unless --purge is passed.",
	Example: `  # Remove a pin (cache kept on disk for reuse)
  spin remove go-cli

  # Remove a pin AND delete its on-disk cache
  spin remove go-cli --purge`,
	Args:          cobra.ExactArgs(1),
	RunE:          runRemove,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var removePurgeFlag bool

func init() {
	removeCmd.Flags().BoolVar(&removePurgeFlag, "purge", false, "also delete the on-disk cache (default: keep it for future `spin add`)")
	rootCmd.AddCommand(removeCmd)
}

// runRemove is the RunE for `spin remove`. Reads pinned.json,
// finds the named entry, removes it, and (optionally) purges the
// on-disk cache. Reports "not found" as an error so a typo
// doesn't silently no-op.
func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	client := registry.New()
	all, err := client.ListPinned()
	if err != nil {
		return fmt.Errorf("spin remove: read pinned: %w", err)
	}
	var match *registry.Pinned
	for i := range all {
		if all[i].Name == name {
			match = &all[i]
			break
		}
	}
	if match == nil {
		return fmt.Errorf("spin remove: no pinned template named %q (run `spin list`)", name)
	}
	if err := client.Unpin(name); err != nil {
		return fmt.Errorf("spin remove: write pinned.json: %w", err)
	}
	if removePurgeFlag && match.LocalPath != "" {
		if err := os.RemoveAll(match.LocalPath); err != nil {
			// Non-fatal: pin is already gone, but tell the user
			// the cache wipe failed so they can clean up by hand.
			printWarn("removed pin %q but failed to delete cache at %s: %v", name, match.LocalPath, err)
			return nil
		}
		printSuccess("removed %q and deleted cache at %s", name, match.LocalPath)
		return nil
	}
	if removePurgeFlag && match.LocalPath == "" {
		printSuccess("removed %q (no on-disk cache to purge)", name)
		return nil
	}
	printSuccess("removed %q (cache kept at %s; use --purge to delete)", name, match.LocalPath)
	return nil
}
