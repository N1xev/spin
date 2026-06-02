package cmd

import (
	"time"

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

	// Walking Skeleton flags (Task 1). Plan 02 expands this list with
	// --module, --license, --template, --force, --cobra, --fang, --viper,
	// --bubbles, --lipgloss, --cli, --all, and the other charm v2 libraries.
	pf := newCmd.PersistentFlags()
	pf.Bool("tui", false, "TUI project variant")
	pf.Bool("bubbletea", false, "add bubbletea v2")
}

func runNew(cmd *cobra.Command, args []string) error {
	tui, _ := cmd.Flags().GetBool("tui")
	bubbletea, _ := cmd.Flags().GetBool("bubbletea")

	p := &scaffold.Project{
		Name:    args[0],
		Module:  args[0],
		Type:    "tui",
		Year:    time.Now().Year(),
		SpinVer: version,
	}
	if tui {
		p.Type = "tui"
	}
	if bubbletea {
		p.Libs = append(p.Libs, "bubbletea")
	}

	return scaffold.New(p)
}
