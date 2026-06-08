package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/ecosystems/charm"
)

// charmFlagsForDispatch returns the charm ecosystem's flags without
// re-running newCharmCmd's init. Used by dispatchNewCharmWithTemplate
// to build a standalone cobra command that can drive runNewCharm.
func charmFlagsForDispatch() []ecosystem.Flag {
	return charm.Flags()
}

var (
	newListEcosystems bool
)

func init() {
	// Add the --list-ecosystems flag to the existing newCmd without
	// touching the original init() body in cmd/new.go.
	newCmd.Flags().BoolVar(&newListEcosystems, "list-ecosystems", false,
		"list every available ecosystem and exit")
	// Note: the --template flag is already declared on newCmd in
	// cmd/new.go (it is a v1 template name like "tui-bubbletea").
	// In v2.0, when the value is not a known v1 template name, we
	// treat it as a v2 git URL or user/repo spec. The detection
	// happens in the PreRunE below.
}

// We hook into newCmd's pre-run so --list-ecosystems works alongside
// the existing positional-arg flow.
func init() {
	oldPreRun := newCmd.PreRunE
	newCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if newListEcosystems {
			r := defaultRegistry()
			fmt.Printf("%-15s  %-8s  %s\n", "NAME", "VERSION", "DESCRIPTION")
			for _, e := range r.All() {
				fmt.Printf("%-15s  %-8s  %s\n", e.Name(), e.Version(), e.Description())
			}
			os.Exit(0)
		}
		// If --template is set on the legacy v1 `new` command
		// (NOT a charm/rust subcommand) AND the value looks like
		// a v2 git spec (URL or user/repo), short-circuit to the
		// v2 dispatch by injecting "charm" as the ecosystem. The
		// canonical --template handling on the subcommand lives
		// in cmd/new_charm.go; this branch is the v1->v2 bridge
		// for the legacy form `spin new <name> --template <ref>`.
		if tplVal, _ := cmd.Flags().GetString("template"); looksLikeV2Template(tplVal) {
			if len(args) >= 1 && !hasNewSubcommand() {
				if err := dispatchNewCharmWithTemplate(args, cmd, tplVal); err != nil {
					return err
				}
				os.Exit(0)
			}
		}
		if oldPreRun != nil {
			return oldPreRun(cmd, args)
		}
		return nil
	}
}

// hasNewSubcommand returns true when the current invocation is a
// `spin new <subcommand> ...` call (so we can distinguish it from
// the bare `spin new <name> ...` form). Inspects os.Args since cobra
// does not expose parent-command presence at PreRunE time.
func hasNewSubcommand() bool {
	for i, a := range os.Args {
		if a == "new" && i+1 < len(os.Args) {
			for _, rest := range os.Args[i+1:] {
				if len(rest) > 0 && rest[0] == '-' {
					continue
				}
				// First non-flag arg after `new`. If it
				// is a known subcommand (charm, rust),
				// we're in subcommand mode.
				if rest == "charm" || rest == "rust" {
					return true
				}
				return false
			}
		}
	}
	return false
}

// dispatchNewCharmWithTemplate synthesizes a call to runNewCharm
// with the --template flag set. We cannot use cobra's command
// dispatch (the args would need to be re-parsed); instead we call
// the same underlying function with a freshly-built *cobra.Command
// that has the charm flags bound AND the parent's flag values
// copied over (so --tui, --bubbletea, --module, etc. flow through).
//
// args[0] must be the project name (NOT the ecosystem name —
// runNewCharm is a cobra subcommand with ExactArgs(1)).
func dispatchNewCharmWithTemplate(args []string, parent *cobra.Command, templateRef string) error {
	cmd := &cobra.Command{}
	// Bind the charm ecosystem's flags so runNewCharm has the
	// same flag surface as newCharmCmd.
	for _, f := range charmFlagsForDispatch() {
		switch f.Type {
		case ecosystem.FlagTypeBool:
			def, _ := f.Default.(bool)
			cmd.Flags().Bool(f.Name, def, f.Help)
		case ecosystem.FlagTypeString, ecosystem.FlagTypeChoice:
			def, _ := f.Default.(string)
			cmd.Flags().String(f.Name, def, f.Help)
		}
	}
	// Set the template flag value to the v2 ref.
	if err := cmd.Flags().Set("template", templateRef); err != nil {
		return err
	}
	// Copy the parent's flag values onto cmd. For flags that exist
	// on BOTH (e.g. --tui, --bubbletea are on both the v1
	// newCmd and the charm ecosystem), we OVERRIDE the default
	// with the user's actual value. This is critical: without it,
	// the user's --tui --bubbletea flags are dropped on the way
	// into the charm flow.
	parent.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			return // user didn't touch this flag; leave the default
		}
		switch f.Value.Type() {
		case "bool":
			b, _ := parent.Flags().GetBool(f.Name)
			_ = cmd.Flags().Set(f.Name, boolToString(b))
		default:
			s, _ := parent.Flags().GetString(f.Name)
			_ = cmd.Flags().Set(f.Name, s)
		}
	})
	// args[0] is the project name; runNewCharm expects
	// ExactArgs(1), so we pass it through directly.
	return runNewCharm(cmd, args)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
