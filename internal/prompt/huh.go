// Package prompt — huh v2 in-process form backend (Plan 02 / INT-04).
//
// fillWithHuh runs the 8 prompt steps from UI-SPEC §"Surface A /
// Prompt sequence" using `charm.land/huh/v2` in-process forms. Each
// step is a standalone function (askType, askName, ...) that
// builds and runs a single huh form and writes the result back
// into *scaffold.Project. Cancellation (`huh.ErrUserAborted`)
// is mapped to *Canceled; the main boundary (main.go) maps
// *Canceled to exit code 130.
//
// The gum backend (Plan 03) implements the same observable behavior
// (same 8 steps, same field write-back to *Project, same
// cancellation semantics) by shelling out to `gum` subprocesses.
// Tests assert against *Project after Fill, not against widget
// internals — see prompt_test.go and huh_test.go.

package prompt

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"charm.land/huh/v2"

	"github.com/example/spin/internal/scaffold"
)

// fillWithHuh runs the 8 prompt steps in UI-SPEC order, writing
// each answer back into p in place. Returns on the first error
// (which may be a *Canceled from a huh form abort).
//
// The order matches UI-SPEC §"Surface A / Prompt sequence" table:
//  1. askType        — project variant (tui/cli/all)
//  2. askName        — directory name (skipped if p.Name set)
//  3. askModule      — go.mod module path (skipped if p.Module set)
//  4. askLibs        — multi-select (always asked, pre-selected)
//  5. askLicense     — mit / apache-2.0 / none (skipped if non-default set)
//  6. askTemplate    — variant-specific template (skipped if non-default set)
//  7. askTemplateRepo — optional external repo URL (skipped if p.TemplateRepo set)
//  8. askAI          — yes/no AGENTS.md opt-in (always asked, default Yes)
//
// Per UI-SPEC, steps 4 and 8 always fire (with variant-specific
// defaults pre-applied); the others are gap-fillers.
func fillWithHuh(p *scaffold.Project) error {
	steps := []struct {
		name string
		fn   func(*scaffold.Project) error
	}{
		{"askType", askType},
		{"askName", askName},
		{"askModule", askModule},
		{"askLibs", askLibs},
		{"askLicense", askLicense},
		{"askTemplate", askTemplate},
		{"askTemplateRepo", askTemplateRepo},
		{"askAI", askAI},
	}
	for _, s := range steps {
		if err := s.fn(p); err != nil {
			return err
		}
	}
	return nil
}

// askType: project variant select. Skipped if p.Type is already
// set by a flag (--tui/--cli/--all).
func askType(p *scaffold.Project) error {
	if p.Type != "" {
		return nil
	}
	var t string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What kind of project?").
				Options(
					huh.NewOption("TUI — terminal app with bubbletea", "tui"),
					huh.NewOption("CLI — command-line tool with cobra + fang", "cli"),
					huh.NewOption("TUI + CLI — single binary with both halves", "all"),
				).
				Value(&t),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return &Canceled{Reason: "user canceled at project type selection"}
	}
	if err != nil {
		return fmt.Errorf("ask type: %w", err)
	}
	p.Type = t
	return nil
}

// askName: project name (directory + binary). Validates with
// IsValidGoModuleSegment. Re-prompts once on failure; second
// failure returns the "spin: project name is required" error.
// Skipped if p.Name is already set by a positional arg.
func askName(p *scaffold.Project) error {
	if p.Name != "" {
		return nil
	}
	var name string
	for attempt := 1; attempt <= 2; attempt++ {
		n := "myapp"
		if attempt == 2 && name != "" {
			n = name // pre-fill with previous attempt's value
		}
		err := huh.NewInput().
			Title("Project name").
			Description("Used as the directory name and default module path.").
			Placeholder("myapp").
			Value(&n).
			Run()
		if errors.Is(err, huh.ErrUserAborted) {
			return &Canceled{Reason: "user canceled at project name"}
		}
		if err != nil {
			return fmt.Errorf("ask name: %w", err)
		}
		name = n
		if scaffold.IsValidGoModuleSegment(name) {
			p.Name = name
			return nil
		}
	}
	return fmt.Errorf("spin: project name is required")
}

// askModule: module path (go.mod). Defaults to p.Name. Skipped
// if p.Module is already set to a non-default value (i.e., the
// user passed --module with a different path).
func askModule(p *scaffold.Project) error {
	if p.Module != "" && p.Module != p.Name {
		return nil
	}
	initial := p.Module
	if initial == "" || initial == p.Name {
		initial = p.Name
		if initial == "" {
			initial = "github.com/<your-org>/myapp"
		}
	}
	var m string = initial
	err := huh.NewInput().
		Title("Module path").
		Description("Used in go.mod. Press Enter to accept the default.").
		Placeholder("github.com/<your-org>/<name>").
		Value(&m).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return &Canceled{Reason: "user canceled at module path"}
	}
	if err != nil {
		return fmt.Errorf("ask module: %w", err)
	}
	module := strings.TrimSpace(m)
	if module == "" {
		return nil // user accepted the default
	}
	p.Module = module
	return nil
}

// askLibs: multi-select. Pre-selects variant defaults + current
// state (covers flag-set values like --huh). Always asked (per
// UI-SPEC: step 4 is one of the two always-fire steps).
//
// Write-back: the result is mirrored into BOTH p.Libs (re-set to
// the multi-select answer, sorted) AND the per-lib bool fields
// (p.Cobra, p.Huh, ...). This keeps the two parallel sources of
// truth in sync — see Pitfall 4 in 03-RESEARCH.md.
func askLibs(p *scaffold.Project) error {
	pre := preSelectedLibs(p)
	preSet := make(map[string]bool, len(pre))
	for _, n := range pre {
		preSet[n] = true
	}
	var picks []string
	options := make([]huh.Option[string], 0, len(LibCatalog))
	for _, lib := range LibCatalog {
		options = append(options,
			huh.NewOption(lib.Display, lib.Name).Selected(preSet[lib.Name]))
	}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Pick libraries").
				Description("Space to toggle, Enter to confirm. Defaults are pre-selected.").
				Options(options...).
				Value(&picks),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return &Canceled{Reason: "user canceled at library selection"}
	}
	if err != nil {
		return fmt.Errorf("ask libs: %w", err)
	}
	// Mirror picks back to p.Libs and the bool fields.
	pickSet := make(map[string]bool, len(picks))
	for _, n := range picks {
		pickSet[n] = true
	}
	p.Libs = make([]string, 0, len(picks))
	for _, n := range picks {
		p.Libs = append(p.Libs, n)
	}
	sort.Strings(p.Libs)
	for n, fieldName := range libBoolMirror {
		setBoolFieldByName(p, fieldName, pickSet[n])
	}
	return nil
}

// askLicenseOptions returns the three license options shown by
// askLicense. Extracted as a package-private helper so tests can
// assert against the canonical option list (Values: "mit",
// "apache-2.0", "none") without needing a TTY to drive the form.
func askLicenseOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("MIT", "mit"),
		huh.NewOption("Apache-2.0", "apache-2.0"),
		huh.NewOption("None", "none"),
	}
}

// preSelectLicense returns options with the entry whose Value
// matches license marked Selected(true). Extracted as a helper so
// tests can verify the pre-select behavior without a TTY. The
// huh.Option type's Selected method is a setter (returns a new
// Option with the flag flipped), so the only way to detect the
// mutation is to compare the returned slice's identity at the
// matched index against the input — which is what the test does.
func preSelectLicense(options []huh.Option[string], license string) []huh.Option[string] {
	for i := range options {
		if options[i].Value == license {
			options[i] = options[i].Selected(true)
		}
	}
	return options
}

// preSelectedLibs returns the lib Names that should be pre-selected
// in the askLibs multi-select prompt. The result is the union of:
//
//   - LibsForType(p.Type) — the variant defaults
//   - p.AllLibs() — the current state (covers flag-set values like
//     --huh that aren't in the variant default)
//
// Deduplicated and sorted alphabetically. This is the single
// "pre-select" decision point — both prompt backends (huh in this
// plan, gum in Plan 03) consume the same function so the defaults
// are consistent across implementations.
func preSelectedLibs(p *scaffold.Project) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, n := range LibsForType(p.Type) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range p.AllLibs() {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// setBoolFieldByName sets p.<fieldName> = val. Field names are
// the Go field names listed in libBoolMirror (e.g., "Cobra",
// "Huh"). A small switch avoids reflect on every multi-select
// answer (Pitfall 6 in 03-RESEARCH.md).
func setBoolFieldByName(p *scaffold.Project, fieldName string, val bool) {
	switch fieldName {
	case "Cobra":
		p.Cobra = val
	case "Fang":
		p.Fang = val
	case "Viper":
		p.Viper = val
	case "Huh":
		p.Huh = val
	case "Glamour":
		p.Glamour = val
	case "Wish":
		p.Wish = val
	case "Log":
		p.Log = val
	case "Harmonica":
		p.Harmonica = val
	}
}

// askLicense: pick from mit / apache-2.0 / none. Skipped if
// p.License is already set to a non-default value (i.e., the
// user passed --license with something other than "mit", which
// is the default). The "mit" default is always re-asked so the
// user can confirm.
func askLicense(p *scaffold.Project) error {
	if p.License != "" && p.License != "mit" {
		return nil
	}
	options := preSelectLicense(askLicenseOptions(), p.License)
	var lic string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("License").
				Options(options...).
				Value(&lic),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return &Canceled{Reason: "user canceled at license"}
	}
	if err != nil {
		return fmt.Errorf("ask license: %w", err)
	}
	p.License = lic
	return nil
}

// askTemplate: pick from variant-specific options. Skipped if
// p.Template is already set to a non-default value.
func askTemplate(p *scaffold.Project) error {
	if p.Template != "" && p.Template != "tui-bubbletea" {
		return nil
	}
	options := templateOptionsFor(p.Type)
	if len(options) == 0 {
		return nil // no options for unknown variant
	}
	var tmpl string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Template").
				Options(options...).
				Value(&tmpl),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return &Canceled{Reason: "user canceled at template"}
	}
	if err != nil {
		return fmt.Errorf("ask template: %w", err)
	}
	p.Template = tmpl
	return nil
}

// templateOptionsFor returns the huh Options for the askTemplate
// prompt, scoped to the active project variant. The list matches
// UI-SPEC §"Surface A / Copywriting Contract" / Template options.
func templateOptionsFor(typ string) []huh.Option[string] {
	switch typ {
	case "tui":
		return []huh.Option[string]{
			huh.NewOption("Bubble Tea hello world", "tui-bubbletea"),
		}
	case "cli":
		return []huh.Option[string]{
			huh.NewOption("Cobra + Fang CLI", "cli-cobra-fang"),
		}
	case "all":
		return []huh.Option[string]{
			huh.NewOption("Bubble Tea hello world", "tui-bubbletea"),
			huh.NewOption("Cobra + Fang CLI", "cli-cobra-fang"),
		}
	}
	return nil
}

// askTemplateRepo: optional external template URL. Empty input
// means "skip" (use the embedded templates). Validates with
// IsValidTemplateRepo; re-prompts once on failure. Skipped if
// p.TemplateRepo is already set by --template-repo.
func askTemplateRepo(p *scaffold.Project) error {
	if p.TemplateRepo != "" {
		return nil
	}
	var repo, last string
	for attempt := 1; attempt <= 2; attempt++ {
		r := repo
		err := huh.NewInput().
			Title("External template repo URL").
			Description("(skip to use embedded templates)").
			Placeholder("(skip to use embedded templates)").
			Value(&r).
			Run()
		if errors.Is(err, huh.ErrUserAborted) {
			return &Canceled{Reason: "user canceled at template repo"}
		}
		if err != nil {
			return fmt.Errorf("ask template repo: %w", err)
		}
		repo = strings.TrimSpace(r)
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

// askAI: yes/no confirm. Default Yes (UI-SPEC §"Copywriting
// Contract" / AI opt-in default: Yes). Always asked; Plan 04
// may add a --no-ai skip path.
func askAI(p *scaffold.Project) error {
	var ai bool = true
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate AGENTS.md for AI assistants?").
				Description("Adds an AGENTS.md describing the project's libraries and how to extend them.").
				Value(&ai),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return &Canceled{Reason: "user canceled at AI opt-in"}
	}
	if err != nil {
		return fmt.Errorf("ask AI: %w", err)
	}
	p.AI = ai
	return nil
}
