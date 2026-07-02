package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/N1xev/spin/internal/registry"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search locally-registered template registries",
	Long:  "Search across every template in every registered registry. Reads ~/.config/spin/registries/*/templates/*.toml directly -- no network call. Use `spin registry add` to register a registry first.",
	Example: `  spin search "go cli"
  spin search rust --limit 5
  spin search tauri --json`,
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

type searchEntry struct {
	Alias       string   `json:"alias"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags,omitempty"`
	Type        string   `json:"type,omitempty"`
	Language    string   `json:"language,omitempty"`
	Version     string   `json:"version,omitempty"`
}

type searchResultView struct {
	Query   string        `json:"query"`
	Total   int           `json:"total"`
	Entries []searchEntry `json:"entries"`
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	mgr := registry.NewManager()
	idx, _, err := mgr.Build()
	if err != nil {
		return fmt.Errorf("spin search: build index: %w", err)
	}
	results := idx.Search(query, searchLimit)

	view := searchResultView{Query: query, Total: len(results), Entries: make([]searchEntry, 0, len(results))}
	for _, e := range results {
		view.Entries = append(view.Entries, searchEntry{
			Alias:       e.Alias,
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Source:      e.Source,
			Tags:        e.Tags,
			Type:        e.Type,
			Language:    e.Language,
			Version:     e.Version,
		})
	}

	if searchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(view)
	}
	if view.Total == 0 {
		printInfo("no templates matched %q", query)
		printHint("run `spin registry add <alias> <git-or-local-source>` to register a registry")
		return nil
	}
	headers := []string{"ALIAS/ID", "NAME", "LANG", "DESCRIPTION"}
	rows := make([][]string, 0, len(results))
	for _, e := range results {
		rows = append(rows, []string{
			e.Alias + "/" + e.ID,
			e.Name,
			e.Language,
			e.Description,
		})
	}
	printTable(os.Stdout, headers, rows)
	fmt.Fprintln(os.Stdout)
	printHint("add a template: spin add <alias>/<id>")
	return nil
}