package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/registry"
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Pin a template locally so you can scaffold it without a network",
	Long: `Pin a template from the registry (or a local path or git URL) so
` + "`spin new`" + ` can use it offline. Pinned templates are stored in
~/.config/spin/pinned.json.

When the spec is a local path (starts with /, ., or ~) it is added
without any network call. When the spec is a git URL (http://,
https://, git@, git://, ssh://) it is shallow-cloned into
~/.config/spin/templates/. The "user/repo" registry shorthand is
not yet supported (the public registry is not deployed) — use a
full git URL or a local path.

Examples:
  spin add /path/to/my-template
  spin add https://github.com/charmbracelet/spin-charm-api.git
  spin add --list`,
	Args:          cobra.MinimumNArgs(0),
	RunE:          runAdd,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var addListFlag bool

func init() {
	addCmd.Flags().BoolVar(&addListFlag, "list", false, "show pinned templates and exit")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	client := registry.New()

	// `--list` (or no args) prints the pinned list and exits.
	if addListFlag || len(args) == 0 {
		return execList(cmd, nil)
	}

	spec := strings.TrimSpace(args[0])
	pinned, err := client.Add(spec)
	if err != nil {
		return err
	}

	// Persist. PinnedAt is the only field Add() does not set.
	pinned.PinnedAt = time.Now().UTC().Format(time.RFC3339)
	if err := client.Pin(*pinned); err != nil {
		return err
	}

	// Print a human-friendly confirmation that mentions the on-disk
	// location and the kind of source (cloned git repo vs local).
	kind := "cloned"
	if pinned.Version == "local" {
		kind = "local at"
	}
	fmt.Printf("✓ added %s (%s %s)\n", pinned.Name, kind, pinned.LocalPath)
	return nil
}
