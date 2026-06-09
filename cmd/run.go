package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/runner"
	"github.com/example/spin/internal/runner/sources"
)

var runCmd = &cobra.Command{
	Use:   "run [task] [-- args...]",
	Short: "Run a project task (declared in spin.config.toml, Taskfile, Makefile, package.json, or scripts/)",
	Long: `spin run is the universal task runner. It resolves a task name to a
command across multiple sources (in precedence order):

  1. spin.config.toml  (highest)
  2. Taskfile.yml
  3. Makefile
  4. package.json scripts
  5. scripts/ directory
  6. language-specific fallback (go.mod, Cargo.toml, ...)

Examples:
  spin run                # run the default task (usually "dev")
  spin run test           # run the "test" task
  spin run --list         # show every discovered task
  spin run --explain test # show the resolved command + source for "test"
  spin run test -- -v     # pass -v through to the underlying command
  spin run dev --watch    # run in watch mode (where supported)`,
	Args:               cobra.ArbitraryArgs,
	RunE:               runRun,
	SilenceUsage:       true,
	SilenceErrors:      true,
}

var (
	runList    bool
	runExplain string
	runWatch   bool
	runAuto    bool
	runJSON    bool
)

func init() {
	runCmd.Flags().BoolVar(&runList, "list", false, "list all available tasks")
	runCmd.Flags().StringVar(&runExplain, "explain", "", "explain a single task: show source + command")
	runCmd.Flags().BoolVar(&runWatch, "watch", false, "watch-mode for the task (where supported)")
	runCmd.Flags().BoolVar(&runAuto, "auto", false, "ignore spin.config.toml; use auto-detection only")
	runCmd.Flags().BoolVar(&runJSON, "json", false, "machine-readable output (with --list/--explain)")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	r := runner.New(dir)
	r.Sources = defaultSourceChain(runAuto)

	switch {
	case runList:
		return r.List(os.Stdout)
	case runExplain != "":
		return r.Explain(os.Stdout, runExplain)
	}

	if len(args) == 0 {
		// No task + no --list/--explain: run the default task.
		args = []string{"dev"}
	}
	taskName := args[0]
	extra := args[1:]

	// Split on "--" to honour pass-through.
	for i, a := range extra {
		if a == "--" {
			extra = extra[i+1:]
			break
		}
	}

	return r.Run(context.Background(), taskName, extra, runWatch)
}

// defaultSourceChain returns the source chain. When auto is true,
// spin.config.toml is dropped so only auto-detected sources apply.
//
// The chain's effective precedence (lowest Order → highest Order) is:
//
//	fallback     Order=0   hardcoded go/cargo/pytest/deno (resilience)
//	ecosystem    Order=5   registered ecosystems' Tasks() (rust, charm, ...)
//	scripts/     Order=20  every executable in scripts/
//	package.json Order=30  npm: prefixed
//	Makefile     Order=40
//	Taskfile.yml Order=60
//	spin.config.toml  Order=100  highest — user always wins
//
// The slice order here does not matter; the runner's merge function
// uses Order() to pick the winner. The ecosystem source is the
// AUTHORITY for language-specific fallbacks (cargo build, cargo
// clippy, charm's `dev` → air, ...); the hardcoded fallback is the
// last-resort floor in case no ecosystem is registered.
func defaultSourceChain(auto bool) []runner.TaskSource {
	chain := []runner.TaskSource{
		sources.NewFallback(),
		sources.NewEcosystemTasks(defaultRegistry().All()),
		sources.NewScriptsDir(),
		sources.NewPackageJSON(),
		sources.NewMakefile(),
		sources.NewTaskfile(),
	}
	if !auto {
		chain = append(chain, sources.NewSpinConfig())
	}
	return chain
}

// helper used by the init bodies below to find the project root.
// walks up from cwd looking for go.mod / Cargo.toml / etc.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		for _, marker := range []string{"go.mod", "Cargo.toml", "package.json", "pyproject.toml", "spin.config.toml", "Taskfile.yml", "Makefile"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no project root found (no go.mod, Cargo.toml, etc.)")
		}
		dir = parent
	}
}
