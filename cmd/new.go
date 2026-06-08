package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/prompt"
	"github.com/example/spin/internal/scaffold"
)

// deprecationPrinted guards the one-time deprecation notice for the
// legacy `spin new <name>` form (no ecosystem). It is per-process.
var deprecationPrinted bool

// printDeprecationNotice prints the v1->v2 deprecation hint to stderr
// exactly once per process. Subsequent calls in the same process are
// no-ops.
func printDeprecationNotice() {
	if deprecationPrinted {
		return
	}
	deprecationPrinted = true
	fmt.Fprintf(os.Stderr,
		"WARN  `spin new <name>` is deprecated; use `spin new charm <name>` "+
			"(or `spin new rust <name>` for cargo projects). "+
			"The legacy form will be removed in v3.0.\n")
}

// isKnownEcosystem returns true if the given name matches a registered
// ecosystem (case-insensitive). Lookup is via the single source of
// truth: cmd/ecosystem.go's defaultRegistry.
func isKnownEcosystem(name string) bool {
	r := defaultRegistry()
	for _, n := range r.Names() {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold a new charmbracelet project",
	// v2.0 accepts up to 2 positionals: [<ecosystem>] <name>.
	// runNew performs the actual validation (known vs unknown
	// ecosystem, missing name, etc.) and returns a clear error
	// when the input is wrong.
	Args:          cobra.MaximumNArgs(2),
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
//
// v2.0 ecosystem dispatch: when the first positional arg matches a
// known ecosystem name (case-insensitive), the call is delegated to
// dispatchV2. When no positional arg is given, OR the first positional
// is NOT a known ecosystem, the legacy charm path runs (with a
// one-time deprecation notice). When the first positional IS a name
// but is NOT a known ecosystem AND there are >=2 args, the call is
// rejected with an error listing known ecosystems.
func runNew(cmd *cobra.Command, args []string) error {
	// No args: legacy path with deprecation notice.
	if len(args) == 0 {
		printDeprecationNotice()
		return runLegacy(cmd, args)
	}

	first := args[0]
	if isKnownEcosystem(first) {
		// Known ecosystem: dispatch to v2 flow. We do not print
		// the deprecation notice in this case (user is using the
		// new form).
		if len(args) < 2 {
			return fmt.Errorf("spin new %s: missing <name> argument", first)
		}
		return dispatchV2(args, cmd)
	}

	// Unknown name with >=2 args: treat as an unknown ecosystem
	// attempt and error out clearly.
	if len(args) >= 2 {
		r := defaultRegistry()
		return fmt.Errorf("spin new: unknown ecosystem %q (available: %v)",
			first, r.Names())
	}

	// Single unknown arg: legacy path with deprecation notice.
	printDeprecationNotice()
	return runLegacy(cmd, args)
}

// runLegacy is the v1.0 charm scaffolder path. It is unchanged from
// the pre-v2.0 behaviour; the only addition in v2.0 is the
// deprecation notice printed by the caller.
func runLegacy(cmd *cobra.Command, args []string) error {
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

// dispatchV2 looks up the named ecosystem in defaultRegistry and runs
// the v2 flow (Validate -> Render -> PostScaffold). Flags from the
// cobra command are collected into Context.Flags. Used by both the
// legacy shim and (in Task 2) the --template binding in
// cmd/new_extras.go.
func dispatchV2(args []string, cmd *cobra.Command) error {
	ecoName := args[0]
	name := args[1]

	r := defaultRegistry()
	eco, err := r.Get(ecoName)
	if err != nil {
		return err
	}

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
		Name:    name,
		Year:    time.Now().Year(),
		SpinVer: spinVersion(),
		Flags:   flags,
	}

	if err := eco.Validate(ctx); err != nil {
		return err
	}
	files, err := eco.Render(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rendered %d files for %s\n", len(files), name)
	return eco.PostScaffold(ctx, name)
}
