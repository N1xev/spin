package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/ecosystems/charm"
	"github.com/example/spin/internal/ecosystems/rust"
)

var ecosystemCmd = &cobra.Command{
	Use:   "ecosystem",
	Short: "Manage ecosystems (built-in + external)",
	Long: `List, add, remove, and inspect ecosystems. Built-in ecosystems
ship with the binary; external ecosystems are loaded from git repos
or the public registry (planned).

Examples:
  spin ecosystem list
  spin ecosystem info charm`,
}

var ecosystemListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available ecosystems",
	Args:  cobra.NoArgs,
	RunE:  runEcosystemList,
}

var ecosystemInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show details for an ecosystem (flags, description, version)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runEcosystemInfo,
}

var ecosystemAddCmd = &cobra.Command{
	Use:   "add <git-url>",
	Short: "Pin an external ecosystem (v2.x)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runEcosystemAdd,
}

var ecosystemRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a pinned ecosystem",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runEcosystemRemove,
}

func init() {
	ecosystemCmd.AddCommand(ecosystemListCmd, ecosystemInfoCmd, ecosystemAddCmd, ecosystemRemoveCmd)
	rootCmd.AddCommand(ecosystemCmd)
}

// ─── handlers ──────────────────────────────────────────────────────

func runEcosystemList(cmd *cobra.Command, args []string) error {
	r := defaultRegistry()
	fmt.Printf("%-15s  %-8s  %s\n", "NAME", "VERSION", "DESCRIPTION")
	for _, e := range r.All() {
		fmt.Printf("%-15s  %-8s  %s\n", e.Name(), e.Version(), e.Description())
	}
	return nil
}

func runEcosystemInfo(cmd *cobra.Command, args []string) error {
	r := defaultRegistry()
	e, err := r.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("%s — %s\n", e.Name(), e.Description())
	fmt.Printf("version: %s\n", e.Version())
	fmt.Println()
	fmt.Println("Flags:")
	for _, f := range ecosystem.SortedFlags(e) {
		def := ""
		if f.Default != nil {
			def = fmt.Sprintf(" (default %v)", f.Default)
		}
		fmt.Printf("  --%-15s %s%s\n", f.Name, f.Prompt, def)
	}
	return nil
}

func runEcosystemAdd(cmd *cobra.Command, args []string) error {
	fmt.Println("External ecosystem loading is planned for v2.x; see MIGRATION.md.")
	fmt.Printf("(stub: would have added %s)\n", args[0])
	return nil
}

func runEcosystemRemove(cmd *cobra.Command, args []string) error {
	fmt.Printf("(stub: would have removed %s)\n", args[0])
	return nil
}

// defaultRegistry returns the registry seeded with built-in ecosystems.
func defaultRegistry() *ecosystem.Registry {
	r := ecosystem.NewRegistry()
	r.RegisterBuiltin(charm.New())
	r.RegisterBuiltin(rust.New())
	return r
}
