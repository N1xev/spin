// Package prompt: huh v2 in-process form backend.
//
// fillWithHuh runs the 8 prompt steps and writes each answer back
// into *scaffold.Project. Cancellation is mapped to *Canceled. The
// gum backend (gum.go) implements the same observable behavior by
// shelling out to `gum` subprocesses.
package prompt

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"charm.land/huh/v2"

	"github.com/example/spin/internal/scaffold"
)

// fillWithHuh runs the prompt steps in order. Each step is a
// standalone function that builds a single huh form and writes the
// answer back into p.
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

// askType asks for the project variant. Skipped if p.Type is already
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

// askName asks for the project name (directory + binary). Re-prompts
// once on validation failure; second failure returns an error.
func askName(p *scaffold.Project) error {
	if p.Name != "" {
		return nil
	}
	var name string
	for attempt := 1; attempt <= 2; attempt++ {
		n := "myapp"
		if attempt == 2 && name != "" {
			n = name
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

// askModule asks for the go.mod module path. Skipped if p.Module is
// already set to a non-default value.
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
	m := initial
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
		return nil
	}
	p.Module = module
	return nil
}

// askLibs runs the multi-select. Pre-selects variant defaults plus
// any flag-set values. The result is mirrored into p.Libs and the
// per-lib bool fields.
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

// askLicenseOptions returns the license options shown by askLicense.
func askLicenseOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("MIT", "mit"),
		huh.NewOption("Apache-2.0", "apache-2.0"),
		huh.NewOption("None", "none"),
	}
}

// preSelectLicense marks the option whose Value matches license as selected.
func preSelectLicense(options []huh.Option[string], license string) []huh.Option[string] {
	for i := range options {
		if options[i].Value == license {
			options[i] = options[i].Selected(true)
		}
	}
	return options
}

// preSelectedLibs returns the lib Names pre-selected in the askLibs
// multi-select: variant defaults unioned with the project's current
// library set, deduplicated and sorted.
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

// setBoolFieldByName sets p.<fieldName> = val. A small switch avoids
// reflect on every multi-select answer.
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

// askLicense asks for the license. Skipped if p.License is already
// set to a non-default value.
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

// askTemplate asks for the template, scoped to the active variant.
// Skipped if p.Template is already set to a non-default value.
func askTemplate(p *scaffold.Project) error {
	if p.Template != "" && p.Template != "tui-bubbletea" {
		return nil
	}
	options := templateOptionsFor(p.Type)
	if len(options) == 0 {
		return nil
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

// templateOptionsFor returns the huh Options for askTemplate, scoped
// to the active project variant.
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

// askTemplateRepo asks for the optional external template URL. Empty
// input means "skip". Re-prompts once on validation failure.
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
			return nil
		}
		if scaffold.IsValidTemplateRepo(repo) {
			p.TemplateRepo = repo
			return nil
		}
		last = repo
	}
	return fmt.Errorf("spin: invalid template repo URL %q", last)
}

// askAI asks for the AGENTS.md opt-in. Default Yes.
func askAI(p *scaffold.Project) error {
	ai := true
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
