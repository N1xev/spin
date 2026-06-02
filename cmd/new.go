package cmd

import (
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/scaffold"
)

var newCmd = &cobra.Command{
	Use:           "new <name>",
	Short:         "Scaffold a new charmbracelet project",
	Args:          cobra.ExactArgs(1),
	RunE:          runNew,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(newCmd)

	pf := newCmd.PersistentFlags()

	// Top-level project-type flags (one of --tui / --cli / --all).
	// --tui is the default; --cli and --all are forward-compat (Phase 2 templates).
	pf.Bool("tui", false, "TUI project variant (default if no --cli)")
	pf.Bool("cli", false, "CLI project variant (Phase 2 templates)")
	pf.Bool("all", false, "TUI + CLI combo variant (Phase 2 templates)")

	// Phase 1 active charm v2 libs.
	pf.Bool("bubbletea", false, "add charm.land/bubbletea/v2")
	pf.Bool("bubbles", false, "add charm.land/bubbles/v2 (implies --bubbletea; bumps go.mod to 1.25.0)")
	pf.Bool("lipgloss", false, "add charm.land/lipgloss/v2")

	// Module + license + template (FLAG-16, FLAG-17, TMPL-01).
	pf.String("module", "", "override default module path (e.g. github.com/foo/myapp)")
	pf.String("license", "mit", "license type: mit, apache-2.0, none")
	pf.String("template", "tui-bubbletea", "template name (default: tui-bubbletea)")

	// Behavior flags.
	pf.Bool("force", false, "overwrite existing ./<name>/ directory")
	pf.Bool("no-git", false, "skip git init + initial commit")
	pf.Bool("no-verify", false, "skip post-scaffold go build ./... smoke test")
	pf.Bool("quiet", false, "minimal scaffolder output")

	// Forward-compat flags (Phase 2). Flag binding only; template content
	// lands in the corresponding phase. See Project struct field comments.
	pf.Bool("cobra", false, "add cobra (Phase 2)")
	pf.Bool("fang", false, "add fang (Phase 2)")
	pf.Bool("viper", false, "add viper (Phase 2)")
	pf.Bool("huh", false, "add huh v2 (Phase 2)")
	pf.Bool("glamour", false, "add glamour v2 (Phase 2)")
	pf.Bool("glow", false, "add glow binary (Phase 2)")
	pf.Bool("wish", false, "add wish v2 (Phase 2)")
	pf.Bool("log", false, "add charm log v2 (Phase 2)")
	pf.Bool("harmonica", false, "add harmonica v2 (Phase 2)")
	pf.Bool("modifiers", false, "add x/modifiers (Phase 2)")
	pf.Bool("ansi", false, "add x/ansi (Phase 2)")
	pf.Bool("runewidth", false, "add go-runewidth (Phase 2)")
	pf.Bool("ai", false, "opt in to AGENTS.md (Phase 3)")
}

// runNew is the `spin new` RunE. It binds CLI flags to a *Project via
// ResolveFlags, validates the project (name regex, dir conflict), and
// hands off to scaffold.New for rendering + emit + smoke test.
func runNew(cmd *cobra.Command, args []string) error {
	p, err := scaffold.ResolveFlags(cmd, args)
	if err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	return scaffold.New(p)
}
