package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/ecosystems/charm"
	"github.com/example/spin/internal/template"
)

var newCharmCmd = &cobra.Command{
	Use:   "charm <name>",
	Short: "Scaffold a charmbracelet v2 Go project (TUI/CLI/lib)",
	Long: `Scaffold a Go project wired with the charmbracelet v2 stack.

This is the v2.0 form of the scaffolder. The legacy form
` + "`spin new <name> --tui --bubbletea`" + ` still works for now and
will be removed in v3.0.

Examples:
  spin new charm myapp --type=tui --bubbletea --bubbles
  spin new charm mycli --type=cli --cobra --fang --ai
  spin new charm mylib --type=lib --module github.com/foo/mylib
  spin new charm myapi --template charmbracelet/spin-charm-api`,
	Args:          cobra.ExactArgs(1),
	RunE:          runNewCharm,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Bind every charm ecosystem flag as a cobra flag. Note: the
	// charm ecosystem already declares its own `template` flag
	// (bundled variant name). For v2 external templates, the value
	// of `template` is interpreted as a git ref (URL or user/repo)
	// when it looks like one (see looksLikeV2Template in
	// new_extras.go for the same heuristic).
	for _, f := range charm.Flags() {
		switch f.Type {
		case ecosystem.FlagTypeBool:
			def, _ := f.Default.(bool)
			newCharmCmd.Flags().Bool(f.Name, def, f.Help)
		case ecosystem.FlagTypeString, ecosystem.FlagTypeChoice:
			def, _ := f.Default.(string)
			help := f.Help
			if f.Type == ecosystem.FlagTypeChoice {
				help = f.Help + " (choices: " + joinChoices(f.Choices) + ")"
			}
			newCharmCmd.Flags().String(f.Name, def, help)
		}
	}
	newCmd.AddCommand(newCharmCmd)
}

func joinChoices(cs []string) string {
	out := ""
	for i, c := range cs {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// mergeMaps merges two rel-path → bytes maps. Last-write-wins: any
// key in b overrides the value in a. Used to overlay template files
// on top of ecosystem files in `spin new <eco> <name> --template`.
func mergeMaps(a, b map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func runNewCharm(cmd *cobra.Command, args []string) error {
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

	ctx := ecosystem.Context{
		Name:    args[0],
		Year:    time.Now().Year(),
		SpinVer: spinVersion(),
		Flags:   flags,
	}

	eco := charm.New()
	if err := eco.Validate(ctx); err != nil {
		return err
	}

	// --template flow: the charm ecosystem already declares a
	// `template` flag (bundled variant name). When the value looks
	// like a v2 git spec (URL or user/repo), the v2 loader takes
	// over: clone, parse spin.toml, run the huh form (or apply
	// defaults in non-TTY), merge with ecosystem files, run the
	// post-hook, and delete spin.toml from the output (TPL-16).
	if templateRef, _ := cmd.Flags().GetString("template"); looksLikeV2Template(templateRef) {
		tpl, err := template.NewLoader("").Load(templateRef)
		if err != nil {
			return err
		}
		values, err := tpl.ResolveForm(map[string]any{
			"project_name": ctx.Name,
			"name":         ctx.Name,
			"module":       ctx.Module,
			"year":         ctx.Year,
			"type":         ctx.GetString("type"),
		}, isTerminalCmd())
		if err != nil {
			return err
		}
		ecoFiles, err := eco.Render(ctx)
		if err != nil {
			return err
		}
		tplFiles, err := tpl.Render(values)
		if err != nil {
			return err
		}
		merged := mergeMaps(ecoFiles, tplFiles)
		// Write the merged file map (path-traversal safe via
		// template.WriteFiles), then run the post-hook + delete
		// spin.toml.
		if err := template.WriteFiles(ctx.Name, merged); err != nil {
			return err
		}
		if err := template.RunPostHook(tpl, values, ctx.Name); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "✓ rendered %d files for %s (with --template %s)\n",
			len(merged), ctx.Name, templateRef)
		return eco.PostScaffold(ctx, ctx.Name)
	}

	// Default flow: no template. Render the ecosystem and post-scaffold.
	files, err := eco.Render(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ rendered %d files for %s\n", len(files), ctx.Name)
	return eco.PostScaffold(ctx, ctx.Name)
}

// isTerminalCmd is the cmd-package variant of isTerminal. It uses
// the standard isatty package's IsTerminal against os.Stdin.
func isTerminalCmd() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}

func spinVersion() string {
	return "2.0.0"
}
