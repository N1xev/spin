package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/N1xev/spin/internal/registry"
)

var addCmd = &cobra.Command{
	Use:   "add <spec>",
	Short: "Pin a template locally for offline use",
	Long:  "Pin a template (local path or git URL) so `spin new` can use it offline. Pinned templates are stored in ~/.config/spin/pinned.json and cached under ~/.config/spin/templates/.",
	Example: `  # Pin from a local path (no network)
  spin add ~/code/templates/go-cli

  # Pin from a git URL (shallow-cloned, GIT_TERMINAL_PROMPT=0)
  spin add https://github.com/me/go-cli-template.git

  # List pinned templates
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

	// `<alias>/<id>` shorthand: resolve via the registry index,
	// then pin the resolved source as if the user had typed it
	// directly. The pin's Name stays the template's id (so
	// `spin new <name>` keeps working) and the Source stores the
	// resolved git URL or local path.
	if registry.IsShorthand(spec) {
		mgr := registry.NewManager()
		resolved, err := mgr.ResolveShorthand(spec)
		if err != nil {
			return fmt.Errorf("spin add: %w", err)
		}
		pinned, err := client.Add(resolved.Source)
		if err != nil {
			return err
		}
		pinned.Name = resolved.ID
		pinned.PinnedAt = time.Now().UTC().Format(time.RFC3339)
		if err := client.Pin(*pinned); err != nil {
			return err
		}
		kind := "cloned from"
		if pinned.Version == "local" {
			kind = "local at"
		}
		printSuccess("added %s (%s %s, resolved from %s)", pinned.Name, kind, pinned.LocalPath, spec)
		return nil
	}

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
	kind := "cloned to"
	if pinned.Version == "local" {
		kind = "local at"
	}
	printSuccess("added %s (%s %s)", pinned.Name, kind, pinned.LocalPath)
	return nil
}
