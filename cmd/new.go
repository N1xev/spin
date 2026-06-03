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

	// Project variant matrix (--tui / --cli / --all):
	//   --tui   is the default; scaffolds a single bubbletea program.
	//           --bubbletea is auto-enabled.
	//   --cli   scaffolds a cobra+fang CLI. --cobra and --fang are
	//           auto-enabled (CR-002).
	//   --all   scaffolds a single binary with both halves. --bubbletea,
	//           --cobra, and --fang are all auto-enabled (CR-003).
	//
	// Library flags (--bubbletea, --bubbles, --lipgloss, --cobra, --fang,
	// --viper, --huh, --glamour, --glow, --wish, --log, --harmonica)
	// layer in extra charm v2 deps on top of the chosen variant.
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

	// External template repo (TMPL-03). When set, replaces the embedded
	// template tree with a depth-1 git clone of <url>. The cloned repo
	// must contain a _base/ subdir (the spin overlay engine's required
	// entry point). The clone lives in a tempdir; pass --keep-template-cache
	// to retain it for inspection.
	pf.String("template-repo", "", "override the embedded template with an external git repo (depth-1 clone to a tempdir; the repo must have a _base/ directory)")
	pf.Bool("keep-template-cache", false, "retain the cloned template repo on disk after scaffolding (useful for debugging external templates)")

	// Behavior flags.
	pf.Bool("force", false, "overwrite existing ./<name>/ directory")
	pf.Bool("no-git", false, "skip git init + initial commit")
	pf.Bool("no-verify", false, "skip post-scaffold go build ./... smoke test")
	pf.Bool("quiet", false, "minimal scaffolder output")
	// Phase 3: --no-interactive disables the prompt layer entirely. The
	// alias spellings --yes and --batch are separate flags that all
	// bind to the same p.NoInteractive field (UI-SPEC Locked Decision
	// #5). pflag v1.0.6 does not support multi-char flag aliases (only
	// single-letter Shorthand), so the three CLI spellings are wired
	// individually in resolve.go. ResolveFlags reads "no-interactive"
	// as the canonical name; --yes / --batch are bound to the same
	// field for the alias UX.
	pf.Bool("no-interactive", false, "disable interactive prompts (alias: --yes, --batch)")
	pf.Bool("yes", false, "alias for --no-interactive")
	pf.Bool("batch", false, "alias for --no-interactive")

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
// ResolveFlags, validates the project (name regex, license whitelist,
// dir conflict — all enforced by p.Validate per validate.go), clones
// the external template repo if --template-repo is set, and hands off
// to scaffold.New for rendering + emit + smoke test.
//
// Validation contract: runNew owns the Validate() call so we fail
// fast before any FS write. scaffold.New does NOT re-validate (WR-003
// removed the duplicate call). Any other entry point that calls
// scaffold.New must validate first.
//
// External template lifecycle (TMPL-03): when --template-repo is set,
// we clone to a fresh tempdir (CloneTemplateRepo) BEFORE scaffolding.
// On success, the tempdir is either removed on return (default) or
// retained for inspection (--keep-template-cache). The path is logged
// at Info level so users with --keep-template-cache can find it.
func runNew(cmd *cobra.Command, args []string) error {
	p, err := scaffold.ResolveFlags(cmd, args)
	if err != nil {
		return err
	}
	// Phase 3: prompt layer. Fill is a no-op in Plan 01; Plans 02/03
	// wire the huh and gum backends. The chokepoint is established
	// here so downstream code can wire against it without churn.
	// p.NoInteractive is read inside Fill as a final guard: if the
	// user passed --no-interactive / --yes / --batch, skip the call
	// even when the env/TTY/CI guard would otherwise let prompts fire.
	if !p.NoInteractive {
		if err := prompt.Fill(p); err != nil {
			return err
		}
	}
	if err := p.Validate(); err != nil {
		return err
	}

	// External template repo (TMPL-03). Clone BEFORE scaffold.New so a
	// failed clone produces no FS writes in the target dir. The clone
	// also pre-validates that the repo has a _base/ subdir.
	if p.TemplateRepo != "" {
		dir, err := scaffold.CloneTemplateRepo(context.Background(), p.TemplateRepo)
		if err != nil {
			return err
		}
		p.ExternalDir = dir
		log.Info("external template cloned", "path", dir, "url", p.TemplateRepo)
		if !p.KeepTemplateCache {
			// Schedule cleanup. Deferring here means even a panic in
			// scaffold.New still cleans up the tempdir.
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
