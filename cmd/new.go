package cmd

import (
	"context"
	"os"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/example/spin/internal/prompt"
	"github.com/example/spin/internal/scaffold"
)

var newCmd = &cobra.Command{
	Use:           "new <name>",
	Short:         "Scaffold a new charmbracelet project",
	Args:          cobra.MaximumNArgs(1),
	RunE:          runNew,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(newCmd)

	pf := newCmd.PersistentFlags()

	// Variant flags. --tui is the default; --cli implies --cobra+--fang;
	// --all implies --bubbletea+--cobra+--fang.
	pf.Bool("tui", false, "TUI project variant (default if no --cli)")
	pf.Bool("cli", false, "CLI project variant")
	pf.Bool("all", false, "TUI + CLI combo variant")

	pf.Bool("bubbletea", false, "add charm.land/bubbletea/v2")
	pf.Bool("bubbles", false, "add charm.land/bubbles/v2 (implies --bubbletea; bumps go.mod to 1.25.0)")
	pf.Bool("lipgloss", false, "add charm.land/lipgloss/v2")

	pf.String("module", "", "override default module path (e.g. github.com/foo/myapp)")
	pf.String("license", "mit", "license type: mit, apache-2.0, none")
	pf.String("template", "tui-bubbletea", "template name")

	// External template repo overrides the embedded tree with a depth-1
	// git clone of <url>. The cloned repo must contain a _base/ subdir.
	pf.String("template-repo", "", "override the embedded template with an external git repo (depth-1 clone to a tempdir; the repo must have a _base/ directory)")
	pf.Bool("keep-template-cache", false, "retain the cloned template repo on disk after scaffolding")

	pf.Bool("force", false, "overwrite existing ./<name>/ directory")
	pf.Bool("no-git", false, "skip git init + initial commit")
	pf.Bool("no-verify", false, "skip post-scaffold go build ./... smoke test")
	pf.Bool("quiet", false, "minimal scaffolder output")
	pf.Bool("no-interactive", false, "disable interactive prompts (alias: --yes, --batch)")
	pf.Bool("yes", false, "alias for --no-interactive")
	pf.Bool("batch", false, "alias for --no-interactive")

	pf.Bool("cobra", false, "add cobra")
	pf.Bool("fang", false, "add fang")
	pf.Bool("viper", false, "add viper")
	pf.Bool("huh", false, "add huh v2")
	pf.Bool("glamour", false, "add glamour v2")
	pf.Bool("wish", false, "add wish v2")
	pf.Bool("log", false, "add charm log v2")
	pf.Bool("harmonica", false, "add harmonica v2")
	pf.Bool("ansi", false, "add x/ansi")
	pf.Bool("runewidth", false, "add go-runewidth")
	pf.Bool("ai", false, "opt in to AGENTS.md (alias: --agents)")
	pf.Bool("agents", false, "alias for --ai")
}

// runNew is the `spin new` RunE. It binds CLI flags to a *Project,
// validates, optionally clones an external template repo, then hands
// off to scaffold.New for rendering, emit, smoke test, and git init.
func runNew(cmd *cobra.Command, args []string) error {
	p, err := scaffold.ResolveFlags(cmd, args)
	if err != nil {
		return err
	}
	if !p.NoInteractive {
		if err := prompt.Fill(p); err != nil {
			return err
		}
	}
	if err := p.Validate(); err != nil {
		return err
	}

	if p.TemplateRepo != "" {
		dir, err := scaffold.CloneTemplateRepo(context.Background(), p.TemplateRepo)
		if err != nil {
			return err
		}
		p.ExternalDir = dir
		log.Info("external template cloned", "path", dir, "url", p.TemplateRepo)
		if !p.KeepTemplateCache {
			defer func() {
				if rmErr := os.RemoveAll(dir); rmErr != nil {
					log.Warn("failed to remove template cache", "path", dir, "err", rmErr.Error())
				}
			}()
		} else {
			log.Info("template cache retained (--keep-template-cache)", "path", dir)
		}
	}

	return scaffold.New(p)
}
