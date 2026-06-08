package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/ecosystems/rust"
)

var newRustCmd = &cobra.Command{
	Use:   "rust <name>",
	Short: "Scaffold a Rust project (binary, library, or example)",
	Long: `Scaffold a Cargo-based Rust project.

This is the v2.0 universal-scaffolder form. The three project types:
  - bin:     a binary crate (src/main.rs)
  - lib:     a library crate (src/lib.rs, with a unit test)
  - example: a single-file example (examples/<name>.rs)

Examples:
  spin new rust myapp --bin
  spin new rust mylib --lib
  spin new rust mydemo --example`,
	Args:          cobra.ExactArgs(1),
	RunE:          runNewRust,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Bind every rust ecosystem flag as a cobra flag.
	for _, f := range rust.Flags() {
		switch f.Type {
		case ecosystem.FlagTypeBool:
			def, _ := f.Default.(bool)
			newRustCmd.Flags().Bool(f.Name, def, f.Help)
		case ecosystem.FlagTypeString, ecosystem.FlagTypeChoice:
			def, _ := f.Default.(string)
			help := f.Help
			if f.Type == ecosystem.FlagTypeChoice {
				help = f.Help + " (choices: " + joinChoices(f.Choices) + ")"
			}
			newRustCmd.Flags().String(f.Name, def, help)
		}
		// For a ChoiceFlag with aliases (e.g. --bin, --lib, --example for
		// the rust "type" flag), register each alias as a bool flag that
		// short-circuits the canonical type. Translation happens in
		// runNewRust so the choice value flows into the renderer context
		// unchanged.
		if f.Type == ecosystem.FlagTypeChoice {
			for _, alias := range f.Aliases {
				newRustCmd.Flags().Bool(alias, false, "Alias for --type="+alias)
			}
		}
	}
	newCmd.AddCommand(newRustCmd)
}

func runNewRust(cmd *cobra.Command, args []string) error {
	flags := map[string]any{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		switch f.Value.Type() {
		case "bool":
			b, _ := cmd.Flags().GetBool(f.Name)
			flags[f.Name] = b
		default:
			s, _ := cmd.Flags().GetString(f.Name)
			flags[f.Name] = s
		}
	})

	// Resolve ChoiceFlag aliases (e.g. --bin) into the canonical --type
	// value. The alias that is true wins; later aliases in declaration
	// order override earlier ones (so --lib --bin => bin, the last flag
	// on the command line).
	for _, f := range rust.Flags() {
		if f.Type != ecosystem.FlagTypeChoice {
			continue
		}
		for _, alias := range f.Aliases {
			if b, ok := flags[alias].(bool); ok && b {
				flags[f.Name] = alias
			}
		}
	}

	ctx := ecosystem.Context{
		Name:    args[0],
		Year:    time.Now().Year(),
		SpinVer: spinVersion(),
		Flags:   flags,
	}

	eco := rust.New()
	if err := eco.Validate(ctx); err != nil {
		return err
	}

	files, err := eco.Render(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ rendered %d files for %s\n", len(files), ctx.Name)
	return eco.PostScaffold(ctx, ctx.Name)
}
