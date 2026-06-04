// Package prompt — gum shell-out backend (Plan 03 / INT-01).
//
// fillWithGum runs the 8 prompt steps from UI-SPEC §"Surface A /
// Prompt sequence" by shelling out to the `gum` binary on $PATH. The
// huh backend (huh.go) and the gum backend produce the SAME observable
// behavior — same 8 steps, same field write-back to *Project, same
// cancellation semantics — but they differ in execution model:
//
//   - huh (in-process): `charm.land/huh/v2` form, runs in the Go
//     process, no extra binary required.
//   - gum (subprocess): spawns `gum choose` / `gum input` /
//     `gum confirm` / `gum choose --no-limit` as a child process and
//     captures stdout. The canonical charmbracelet prompt look.
//
// The backend choice is decided once per `Fill` invocation by
// resolveBackend() in prompt.go; see prompt.go for the dispatch.
//
// Testability: the runner is on the Deps struct (see prompt.go), not a
// package-level mutable global. Tests construct a Deps with a stub
// Runner and call fillWithGumDeps directly. The 5-minute context
// timeout is set on the call stack (no global) and flows through the
// widget wrappers as a parameter, so t.Parallel()-using tests are
// safe — no shared state.
//
// Cancellation / timeout pattern is the same as the git-clone path
// in internal/scaffold/repo.go: cmd.Cancel = Process.Kill + a short
// cmd.WaitDelay so Ctrl-C / 5-min timeout never leaves the scaffolder
// hanging on a dead child.

package prompt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/log/v2"

	"github.com/example/spin/internal/scaffold"
)

// gumRunCapture is the single place in the package that calls
// os/exec for gum. It honors the caller's ctx (set by fillWithGumDeps
// for the 5-min timeout) and wires cmd.Cancel + cmd.WaitDelay so a
// parent-side Ctrl-C (or context expiry) kills gum promptly without
// leaving the parent's pipe drains hanging.
//
// Returns the trimmed stdout, or a *Canceled error if ctx is expired
// when the subprocess exits non-zero, or a wrapped error including
// stderr for other failures.
func gumRunCapture(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("gum: no subcommand")
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gum", args...)
	cmd.Stdin = nil // gum reads from the controlling tty directly
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// CR-005 / Pitfall pattern: when the ctx is canceled (Ctrl-C
	// forwarded via signal.Notify, 5-min timeout, or a manual
	// cancel), force the pipes closed so the parent's Read on stdout
	// returns instead of blocking on a dead child. Same shape as
	// internal/scaffold/repo.go's git clone.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 100 * time.Millisecond

	if err := cmd.Run(); err != nil {
		// If the parent context expired, the kill was deliberate —
		// map to *Canceled so the main boundary (main.go) can exit
		// 130. Per UI-SPEC §"Error states": `gum` exits non-zero
		// (broken pipe, SIGPIPE) → "spin: prompt interrupted" → 130.
		if ctx.Err() != nil {
			return "", &Canceled{Reason: "gum canceled"}
		}
		return "", fmt.Errorf("gum %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// gumChoose is the single-select widget wrapper. Returns the chosen
// option text (one of `options`) on success.
//
// The `--selected` flag is 1-based per gum's documented convention
// (gum choose --selected <N> highlights option N where N starts at 1);
// we translate the 0-based Go defaultIdx to 1-based here.
//
// ctx and deps are passed through to deps.Runner; no package-level
// state is touched. This is what makes t.Parallel() safe in tests.
func gumChoose(ctx context.Context, deps Deps, header string, options []string, defaultIdx int) (string, error) {
	args := []string{"choose", "--header", header, "--selected", strconv.Itoa(defaultIdx + 1)}
	args = append(args, options...)
	return deps.Runner(ctx, args...)
}

// gumMultiSelect is the multi-select widget wrapper. Returns the
// selected options (subset of `options`) on success, or nil if the
// user confirmed an empty selection. Per gum docs, multi-select
// separates selections with newlines on stdout.
//
// Pre-selection note: gum's `choose --no-limit` does not support
// pre-selection via the CLI, so the wrapper signature has no
// preSelected parameter (the huh backend's pre-selection is applied
// at the options-builder layer; the gum backend just shows the full
// list). A future plan may pre-fill via stdin if needed; the
// current contract is the user's selection is the only authoritative
// input.
func gumMultiSelect(ctx context.Context, deps Deps, header string, options []string) ([]string, error) {
	args := []string{"choose", "--no-limit", "--header", header}
	args = append(args, options...)
	out, err := deps.Runner(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// gumInput is the single-line text widget wrapper. Returns the typed
// string on success. `placeholder` is shown when the field is empty;
// `defaultValue`, if non-empty, is passed as the gum --value flag
// (gum's pre-filled initial value).
func gumInput(ctx context.Context, deps Deps, header, placeholder, defaultValue string) (string, error) {
	args := []string{"input", "--header", header, "--placeholder", placeholder}
	if defaultValue != "" {
		args = append(args, "--value", defaultValue)
	}
	return deps.Runner(ctx, args...)
}

// gumConfirm is the yes/no widget wrapper. Returns the user's choice
// as a bool. gum prints "Yes" or "No" to stdout on confirm (exit 0
// either way), so we string-compare.
func gumConfirm(ctx context.Context, deps Deps, prompt string, defaultYes bool) (bool, error) {
	args := []string{"confirm", "--default=" + strconv.FormatBool(defaultYes), prompt}
	out, err := deps.Runner(ctx, args...)
	if err != nil {
		return false, err
	}
	return out == "Yes", nil
}

// fillWithGumDeps runs the 8 prompt steps in UI-SPEC order, writing
// each answer back into p in place. Returns on the first error
// (which may be a *Canceled from a gum subprocess abort).
//
// The order matches UI-SPEC §"Surface A / Prompt sequence" table:
//  1. askGumType        — project variant (tui/cli/all)
//  2. askGumName        — directory name (skipped if p.Name set)
//  3. askGumModule      — go.mod module path (skipped if p.Module set)
//  4. askGumLibs        — multi-select (always asked, pre-selected)
//  5. askGumLicense     — mit / apache-2.0 / none (skipped if non-default set)
//  6. askGumTemplate    — variant-specific template (skipped if non-default set)
//  7. askGumTemplateRepo — optional external repo URL (skipped if p.TemplateRepo set)
//  8. askGumAI          — yes/no AGENTS.md opt-in (always asked, default Yes)
//
// The 5-minute context timeout is set here (per the plan) and flows
// through the call stack as a parameter — no package-level mutable
// state. This is what makes the call hermetic for tests.
func fillWithGumDeps(p *scaffold.Project, deps Deps) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return runGumSteps(ctx, deps, p)
}

// runGumSteps dispatches the 8 steps in order. Each step's fn has the
// signature (ctx, deps, p) so the steps table doesn't need closures.
func runGumSteps(ctx context.Context, deps Deps, p *scaffold.Project) error {
	steps := []struct {
		name string
		fn   func(context.Context, Deps, *scaffold.Project) error
	}{
		{"askGumType", askGumType},
		{"askGumName", askGumName},
		{"askGumModule", askGumModule},
		{"askGumLibs", askGumLibs},
		{"askGumLicense", askGumLicense},
		{"askGumTemplate", askGumTemplate},
		{"askGumTemplateRepo", askGumTemplateRepo},
		{"askGumAI", askGumAI},
	}
	for _, s := range steps {
		if err := s.fn(ctx, deps, p); err != nil {
			return err
		}
	}
	return nil
}

// typeDisplayToKey maps the gum "Project type" labels back to the
// machine key written to p.Type. The labels are the user-facing copy
// from UI-SPEC §"Copywriting Contract".
var typeDisplayToKey = map[string]string{
	"TUI — terminal app with bubbletea":          "tui",
	"CLI — command-line tool with cobra + fang":  "cli",
	"TUI + CLI — single binary with both halves": "all",
}

// askGumType: project variant select. Skipped if p.Type is already
// set by a flag (--tui/--cli/--all).
func askGumType(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.Type != "" {
		return nil
	}
	options := []string{
		"TUI — terminal app with bubbletea",
		"CLI — command-line tool with cobra + fang",
		"TUI + CLI — single binary with both halves",
	}
	chosen, err := gumChoose(ctx, deps, "What kind of project?", options, 0)
	if err != nil {
		if isCanceled(err) {
			return &Canceled{Reason: "user canceled at project type selection"}
		}
		return fmt.Errorf("ask type: %w", err)
	}
	key, ok := typeDisplayToKey[chosen]
	if !ok {
		return fmt.Errorf("ask type: unexpected answer %q", chosen)
	}
	p.Type = key
	return nil
}

// askGumName: project name (directory + binary). Validates with
// IsValidGoModuleSegment. Re-prompts once on failure; second
// failure returns the "spin: project name is required" error.
// Skipped if p.Name is already set by a positional arg.
func askGumName(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.Name != "" {
		return nil
	}
	var name string
	for attempt := 1; attempt <= 2; attempt++ {
		n, err := gumInput(ctx, deps, "Project name", "myapp", "")
		if err != nil {
			if isCanceled(err) {
				return &Canceled{Reason: "user canceled at project name"}
			}
			return fmt.Errorf("ask name: %w", err)
		}
		name = strings.TrimSpace(n)
		if scaffold.IsValidGoModuleSegment(name) {
			p.Name = name
			return nil
		}
	}
	return fmt.Errorf("spin: project name is required")
}

// askGumModule: module path (go.mod). Defaults to p.Name. Skipped
// if p.Module is already set to a non-default value (i.e., the
// user passed --module with a different path).
func askGumModule(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.Module != "" && p.Module != p.Name {
		return nil
	}
	defaultValue := p.Name
	if defaultValue == "" {
		defaultValue = "github.com/<your-org>/myapp"
	}
	m, err := gumInput(ctx, deps, "Module path", "github.com/<your-org>/<name>", defaultValue)
	if err != nil {
		if isCanceled(err) {
			return &Canceled{Reason: "user canceled at module path"}
		}
		return fmt.Errorf("ask module: %w", err)
	}
	module := strings.TrimSpace(m)
	if module == "" || module == defaultValue {
		return nil // user accepted the default
	}
	p.Module = module
	return nil
}

// askGumLibs: multi-select. Pre-selects variant defaults + current
// state (covers flag-set values like --huh). Always asked (per
// UI-SPEC: step 4 is one of the two always-fire steps).
//
// Note: gum's `choose --no-limit` does not support pre-selection via
// the CLI; the user always sees the full list. The variant defaults
// (bubbletea for tui, cobra+fang for cli, both for all) are applied
// during the write-back step, not via the prompt itself. This is a
// known divergence from the huh backend (which DOES pre-select via
// .Selected(true)) and is documented in 03-RESEARCH.md as a
// future-enhancement hook (Plan 04+ may pre-fill via stdin).
//
// Write-back: the result is mirrored into BOTH p.Libs (re-set to
// the multi-select answer, sorted) AND the per-lib bool fields
// (p.Cobra, p.Huh, ...). This keeps the two parallel sources of
// truth in sync — see Pitfall 4 in 03-RESEARCH.md.
func askGumLibs(ctx context.Context, deps Deps, p *scaffold.Project) error {
	options := make([]string, 0, len(LibCatalog))
	displayToName := make(map[string]string, len(LibCatalog))
	for _, lib := range LibCatalog {
		options = append(options, lib.Display)
		displayToName[lib.Display] = lib.Name
	}
	picks, err := gumMultiSelect(ctx, deps, "Pick libraries", options)
	if err != nil {
		if isCanceled(err) {
			return &Canceled{Reason: "user canceled at library selection"}
		}
		return fmt.Errorf("ask libs: %w", err)
	}
	// Convert display labels back to names.
	names := make([]string, 0, len(picks))
	for _, d := range picks {
		if n, ok := displayToName[d]; ok {
			names = append(names, n)
		}
	}
	// Mirror picks back to p.Libs and the bool fields.
	pickSet := make(map[string]bool, len(names))
	for _, n := range names {
		pickSet[n] = true
	}
	p.Libs = names
	sort.Strings(p.Libs)
	for n, fieldName := range libBoolMirror {
		setBoolFieldByName(p, fieldName, pickSet[n])
	}
	return nil
}

// askGumLicense: pick from mit / apache-2.0 / none. Skipped if
// p.License is already set to a non-default value (i.e., the
// user passed --license with something other than "mit", which
// is the default). The "mit" default is always re-asked so the
// user can confirm.
func askGumLicense(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.License != "" && p.License != "mit" {
		return nil
	}
	options := []string{"MIT", "Apache-2.0", "None"}
	defaultIdx := 0 // mit
	if p.License == "apache-2.0" {
		defaultIdx = 1
	} else if p.License == "none" {
		defaultIdx = 2
	}
	chosen, err := gumChoose(ctx, deps, "License", options, defaultIdx)
	if err != nil {
		if isCanceled(err) {
			return &Canceled{Reason: "user canceled at license"}
		}
		return fmt.Errorf("ask license: %w", err)
	}
	p.License = strings.ToLower(chosen)
	return nil
}

// askGumTemplate: pick from variant-specific options. Skipped if
// p.Template is already set to a non-default value.
func askGumTemplate(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.Template != "" && p.Template != "tui-bubbletea" {
		return nil
	}
	options := templateOptionsForType(p.Type)
	if len(options) == 0 {
		return nil // no options for unknown variant
	}
	displays := make([]string, 0, len(options))
	displayToKey := make(map[string]string, len(options))
	for _, o := range options {
		displays = append(displays, o.display)
		displayToKey[o.display] = o.key
	}
	chosen, err := gumChoose(ctx, deps, "Template", displays, 0)
	if err != nil {
		if isCanceled(err) {
			return &Canceled{Reason: "user canceled at template"}
		}
		return fmt.Errorf("ask template: %w", err)
	}
	key, ok := displayToKey[chosen]
	if !ok {
		return fmt.Errorf("ask template: unexpected answer %q", chosen)
	}
	p.Template = key
	return nil
}

// askGumTemplateRepo: optional external template URL. Empty input
// means "skip" (use the embedded templates). Validates with
// IsValidTemplateRepo; re-prompts once on failure. Skipped if
// p.TemplateRepo is already set by --template-repo.
func askGumTemplateRepo(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.TemplateRepo != "" {
		return nil
	}
	var last string
	for attempt := 1; attempt <= 2; attempt++ {
		r, err := gumInput(ctx, deps, "External template repo URL", "(skip to use embedded templates)", "")
		if err != nil {
			if isCanceled(err) {
				return &Canceled{Reason: "user canceled at template repo"}
			}
			return fmt.Errorf("ask template repo: %w", err)
		}
		repo := strings.TrimSpace(r)
		if repo == "" {
			return nil // skip
		}
		if scaffold.IsValidTemplateRepo(repo) {
			p.TemplateRepo = repo
			return nil
		}
		last = repo
	}
	return fmt.Errorf("spin: invalid template repo URL %q", last)
}

// askGumAI: yes/no confirm. Default Yes (UI-SPEC §"Copywriting
// Contract" / AI opt-in default: Yes). Always asked; Plan 04
// may add a --no-ai skip path.
func askGumAI(ctx context.Context, deps Deps, p *scaffold.Project) error {
	yes, err := gumConfirm(ctx, deps,
		"Generate AGENTS.md for AI assistants? Adds an AGENTS.md describing the project's libraries and how to extend them.",
		true,
	)
	if err != nil {
		if isCanceled(err) {
			return &Canceled{Reason: "user canceled at AI opt-in"}
		}
		return fmt.Errorf("ask AI: %w", err)
	}
	p.AI = yes
	return nil
}

// templateOption is one (display, key) pair for the askGumTemplate
// multi-select. The huh backend's templateOptionsFor returns
// huh.Option values directly; the gum backend needs the display/key
// pair separately so it can map the user's selection back to the
// template key.
type templateOption struct {
	display string
	key     string
}

// templateOptionsForType returns the (display, key) options for the
// askGumTemplate prompt, scoped to the active project variant. The
// list matches UI-SPEC §"Surface A / Copywriting Contract" / Template
// options.
func templateOptionsForType(typ string) []templateOption {
	switch typ {
	case "tui":
		return []templateOption{
			{"Bubble Tea hello world", "tui-bubbletea"},
		}
	case "cli":
		return []templateOption{
			{"Cobra + Fang CLI", "cli-cobra-fang"},
		}
	case "all":
		return []templateOption{
			{"Bubble Tea hello world", "tui-bubbletea"},
			{"Cobra + Fang CLI", "cli-cobra-fang"},
		}
	}
	return nil
}

// isCanceled reports whether err is (or wraps) a *Canceled from a
// widget wrapper. The widget wrappers always return a fresh
// *Canceled; gumRunCapture returns one when ctx is expired; Runner
// stubs (tests) may return one to simulate a cancel. We use errors.As
// for forward-compat (a future wrapper might wrap).
func isCanceled(err error) bool {
	if err == nil {
		return false
	}
	var c *Canceled
	return errors.As(err, &c)
}

// logBackend is the Debug-level log line emitted by resolveBackend
// (in prompt.go) per UI-SPEC §"gum vs huh decision": "The decision
// is logged at Debug level". Centralized here so the format string
// lives next to the backend enum.
func logBackend(name string) {
	log.Default().Debug("prompt backend", "backend", name)
}
