package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/registry"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List pinned templates",
	Long: `List every template pinned locally via ` + "`spin add`" + `. Pinned
templates are stored in ~/.config/spin/pinned.json and cloned (or
copied) into ~/.config/spin/templates/.`,
	Args:          cobra.NoArgs,
	RunE:          execList,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

// execList prints the pinned templates as a human-readable table.
// `spin list` and `spin add --list` both call this; the latter
// reaches it through runAdd with args=nil.
//
// Each row includes the resolved LocalPath (under
// ~/.config/spin/templates/) so the user can inspect or hand-edit
// the on-disk template.
func execList(cmd *cobra.Command, args []string) error {
	client := registry.New()
	pinned, err := client.ListPinned()
	if err != nil {
		return err
	}
	if len(pinned) == 0 {
		fmt.Println("No pinned templates.")
		fmt.Println("Use `spin search <query>` to find templates in the registry,")
		fmt.Println("or `spin add <name>` to pin one.")
		return nil
	}

	// Compute the cache root so we can show short, relative local
	// paths in the table (e.g. "templates/foo/bar") instead of the
	// full "/home/user/.config/spin/templates/foo/bar".
	cacheRoot := client.CacheDir

	fmt.Printf("%-30s  %-10s  %-12s  %s\n", "NAME", "VERSION", "PINNED", "LOCAL PATH")
	for _, p := range pinned {
		short := shortenLocal(p.LocalPath, cacheRoot)
		fmt.Printf("%-30s  %-10s  %-12s  %s\n",
			truncate(p.Name, 30),
			truncate(p.Version, 10),
			truncate(p.PinnedAt, 12),
			short,
		)
	}
	fmt.Println()
	fmt.Println("Run `spin new <name>` to scaffold from a pinned template.")
	fmt.Println("Run `spin new <name> --refresh` to re-clone (future).")
	return nil
}

// shortenLocal renders p as a path relative to the cache root when
// possible (e.g. "templates/foo/bar"), so the table stays readable.
// Falls back to the absolute path when the path is not under the
// cache root (older pin files that pre-date LocalPath).
func shortenLocal(p, cacheRoot string) string {
	if p == "" {
		return "(unknown)"
	}
	if cacheRoot == "" {
		return p
	}
	if rel, err := filepath.Rel(cacheRoot, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}

// truncate is a tiny helper for table column widths. The skeleton
// re-uses similar truncation in the registry formatting code; we
// keep a local copy here so cmd/list.go does not depend on the
// internal/registry internals.
func truncate(s string, n int) string {
	if n <= 1 || len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], " ") + "…"
}
