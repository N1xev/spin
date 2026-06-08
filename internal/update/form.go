package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"charm.land/huh/v2"
	"github.com/mattn/go-isatty"
)

// isTerminalFunc is the seam tests use to force a non-TTY outcome
// without resorting to os.Pipe + syscall.Dup2 (which leaks the
// original stdin across tests). Production wires it to
// isatty.IsTerminal.
var isTerminalFunc = isatty.IsTerminal

// Action keys for the form's per-dep Select. "skip" leaves Old alone
// (Apply no-ops it). "stable" sets Target to NewStable. "latest" sets
// Target to NewLatest. The strings are also what huh displays as the
// option value in its results map.
const (
	actionSkip   = "skip"
	actionStable = "stable"
	actionLatest = "latest"
)

// ErrCanceled is returned when the user exits the form before
// submitting. Cobra + fang render it as a styled message and the
// process exits non-zero. We define it locally (rather than depending
// on prompt.ErrCanceled) to keep the dependency graph one-way:
// update is a leaf package; prompt already depends on scaffold.
var ErrCanceled = errors.New("update: user canceled")

// UpdateChoice captures one form result: the resolved Dep plus the
// user's pick for it.
type UpdateChoice struct {
	Dep    Dep
	Action string
}

// PromptOptions configures PromptForUpdate. Production code can pass
// the zero value; tests inject stubs.
type PromptOptions struct {
	GoModPath      string
	IncludeIndirect bool
	Log            io.Writer
	Proxy          ModuleProxy
	In             io.Reader
	Out            io.Writer
}

func (o *PromptOptions) applyDefaults() {
	if o.Log == nil {
		o.Log = io.Discard
	}
	if o.Proxy == nil {
		o.Proxy = &HTTPMirror{}
	}
	if o.In == nil {
		o.In = os.Stdin
	}
	if o.Out == nil {
		o.Out = os.Stdout
	}
}

// PromptForUpdate is the user-facing entry point. It lists deps,
// resolves upgrade candidates, renders the huh form (or the non-TTY
// table in CI), and applies the chosen versions via Apply.
//
// D-07: direct-only by default; IncludeIndirect widens to // indirect.
// D-09: one Select per dep, options [Skip, newStable, newLatest],
// default newStable. D-10: apply = go get + go mod tidy + CGO=0
// build; never go test.
func PromptForUpdate(ctx context.Context, opts PromptOptions) error {
	opts.applyDefaults()

	deps, err := ListDeps(opts.GoModPath, opts.IncludeIndirect)
	if err != nil {
		return err
	}
	if len(deps) == 0 {
		fmt.Fprintln(opts.Log, "no deps to update")
		return nil
	}

	resolved, err := (&Resolver{Proxy: opts.Proxy}).Resolve(ctx, deps)
	if err != nil {
		return err
	}

	// Per CONTEXT INT-03 / threat T-04-22: a huh form reading from
	// a non-TTY stdin blocks forever. Guard the build with isatty
	// on the real os.Stdin fd so piped CI invocations print the
	// table and exit cleanly.
	if !isTerminalFunc(os.Stdin.Fd()) {
		if err := printNonTTYTable(resolved, opts.Out); err != nil {
			return err
		}
		return fmt.Errorf("update: stdin is not a TTY; re-run interactively or pass --batch (not yet implemented)")
	}

	selects, choices := buildSelects(resolved)
	err = formFromSelects(selects).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCanceled
	}
	if err != nil {
		return fmt.Errorf("update: form: %w", err)
	}

	return applyChoices(resolved, choices, filepath.Dir(opts.GoModPath), opts.Log)
}

// buildSelects returns one huh.Select[string] per dep alongside the
// pointer map PromptForUpdate reads after the form runs. Default
// value is actionStable per D-09. The function is exported (via
// the form_test.go) so tests can assert the slice length matches
// the dep count without running the form (huh needs a TTY).
func buildSelects(deps []Dep) ([]*huh.Select[string], map[string]*string) {
	selects := make([]*huh.Select[string], 0, len(deps))
	choices := make(map[string]*string, len(deps))
	for _, d := range deps {
		choice := actionStable
		sel := huh.NewSelect[string]().
			Title(fmt.Sprintf("%s  (%s -> stable %s, latest %s)",
				d.Module, d.Old, d.NewStable, d.NewLatest)).
			Options(
				huh.NewOption("Skip", actionSkip),
				huh.NewOption("newStable: "+d.NewStable, actionStable),
				huh.NewOption("newLatest: "+d.NewLatest, actionLatest),
			).
			Value(&choice)
		selects = append(selects, sel)
		choices[d.Module] = &choice
	}
	return selects, choices
}

// formFromSelects wraps the Select list in a single huh Form. One
// group per dep so each Select gets its own page. Exported for the
// buildForm test (TestPromptForUpdate_BuildsFormForEachDep).
func formFromSelects(selects []*huh.Select[string]) *huh.Form {
	groups := make([]*huh.Group, 0, len(selects))
	for _, sel := range selects {
		groups = append(groups, huh.NewGroup(sel))
	}
	return huh.NewForm(groups...)
}

// applyChoices translates the form's per-dep action into Dep.Target
// values and hands them to Apply. Skip drops the dep from the
// apply list entirely.
func applyChoices(deps []Dep, choices map[string]*string, gomodDir string, log io.Writer) error {
	toApply := make([]Dep, 0, len(deps))
	for _, d := range deps {
		choice, ok := choices[d.Module]
		if !ok || choice == nil {
			continue
		}
		switch *choice {
		case actionStable:
			d.Target = d.NewStable
		case actionLatest:
			d.Target = d.NewLatest
		case actionSkip:
			continue
		default:
			continue
		}
		toApply = append(toApply, d)
	}
	return Apply(toApply, gomodDir, log)
}

// printNonTTYTable renders the version table for a CI / non-TTY
// invocation. Returns the sentinel "not a TTY" error so the caller
// can produce a clear exit message.
func printNonTTYTable(deps []Dep, w io.Writer) error {
	if _, err := fmt.Fprintln(w, "update: stdin is not a TTY; printing version table only"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "MODULE\tOLD\tSTABLE\tLATEST"); err != nil {
		return err
	}
	for _, d := range deps {
		stable := d.NewStable
		if stable == d.Old {
			stable = "(current)"
		}
		latest := d.NewLatest
		if latest == d.Old {
			latest = "(current)"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			truncate(d.Module, 50), d.Old, stable, latest); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// truncate returns s with an ellipsis suffix if it exceeds max.
// Used by printNonTTYTable to keep wide module paths from breaking
// terminal wrapping in CI logs.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + strings.Repeat(".", 3)
}
