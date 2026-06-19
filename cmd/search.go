package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/N1xev/spin/internal/registry"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the public template registry",
	Long:  "Search the public template registry for templates matching the given query. When the registry server is unreachable, a friendly message is shown -- never a stack trace. Override the endpoint via SPIN_REGISTRY_URL.",
	Example: `  spin search "go cli"
  spin search rust cli
  spin search tauri --limit 10 --json`,
	Args:          cobra.MinimumNArgs(1),
	RunE:          runSearch,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var (
	searchLimit int
	searchJSON  bool
)

func init() {
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "max results to show")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "machine-readable output")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	client := registry.New()
	res, err := client.SearchWithLimit(args[0], searchLimit)
	if err != nil {
		// All "registry not yet deployed" cases -- DNS failure,
		// connection refused, HTTP 404, etc -- collapse into a
		// single friendly message. Never a stack trace.
		if errors.Is(err, registry.ErrNotDeployed) {
			printInfo("the public registry is not yet deployed")
			printHint("in the meantime, use `spin new <name> --template <git-url>` to scaffold from a git repo,")
			printHint("or `spin add <path-or-url>` to pin a template locally for offline use")
			return nil
		}
		return err
	}
	if searchJSON {
		enc := json.NewEncoder(os.Stdout)
		// Field names stay lower-camel to match the wire format the
		// public registry ships; SearchResult is tagged for json in
		// internal/registry/types.go.
		if err := enc.Encode(res); err != nil {
			return fmt.Errorf("encode search result: %w", err)
		}
		return nil
	}
	fmt.Print(registry.FormatSearch(res, false))
	return nil
}
