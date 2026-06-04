// gum shell-out backend. Shells out to `gum` subprocesses for
// choose / input / confirm / multi-select. huh backend (huh.go) is
// the in-process fallback; the two produce identical observable
// behavior (same 8 steps, same write-back, same cancel semantics).
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

// gumRunCapture is the single os/exec call site for gum. ctx is set
// by fillWithGumDeps (5-min timeout); cancel forces the pipes closed
// so the parent Read returns instead of blocking on a dead child
// (same shape as internal/scaffold/repo.go git clone).
func gumRunCapture(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("gum: no subcommand")
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gum", args...)
	cmd.Stdin = nil
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 100 * time.Millisecond

	if err := cmd.Run(); err != nil {
		// Parent ctx expired → deliberate kill → exit 130.
		if ctx.Err() != nil {
			return "", &Canceled{Reason: "gum canceled"}
		}
		return "", fmt.Errorf("gum %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// gum --selected is 1-based; we translate 0-based Go defaultIdx.
func gumChoose(ctx context.Context, deps Deps, header string, options []string, defaultIdx int) (string, error) {
	args := []string{"choose", "--header", header, "--selected", strconv.Itoa(defaultIdx + 1)}
	args = append(args, options...)
	return deps.Runner(ctx, args...)
}

// gum's `choose --no-limit` has no pre-selection CLI flag, so the
// wrapper signature has no preSelected parameter. Pre-selection in
// huh is done at the options-builder layer; gum just shows the
// full list and the user's selection is authoritative.
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

func gumInput(ctx context.Context, deps Deps, header, placeholder, defaultValue string) (string, error) {
	args := []string{"input", "--header", header, "--placeholder", placeholder}
	if defaultValue != "" {
		args = append(args, "--value", defaultValue)
	}
	return deps.Runner(ctx, args...)
}

// gum prints "Yes"/"No" on stdout; exit 0 either way.
func gumConfirm(ctx context.Context, deps Deps, prompt string, defaultYes bool) (bool, error) {
	args := []string{"confirm", "--default=" + strconv.FormatBool(defaultYes), prompt}
	out, err := deps.Runner(ctx, args...)
	if err != nil {
		return false, err
	}
	return out == "Yes", nil
}

func fillWithGumDeps(p *scaffold.Project, deps Deps) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return runGumSteps(ctx, deps, p)
}

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

// User-facing labels from UI-SPEC §Copywriting Contract.
var typeDisplayToKey = map[string]string{
	"TUI — terminal app with bubbletea":          "tui",
	"CLI — command-line tool with cobra + fang":  "cli",
	"TUI + CLI — single binary with both halves": "all",
}

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
		return nil
	}
	p.Module = module
	return nil
}

// Mirror picks into p.Libs (sorted) and the per-lib bool fields.
// Two parallel sources of truth must stay in sync — see Pitfall 4
// in 03-RESEARCH.md.
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
	names := make([]string, 0, len(picks))
	for _, d := range picks {
		if n, ok := displayToName[d]; ok {
			names = append(names, n)
		}
	}
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

func askGumLicense(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.License != "" && p.License != "mit" {
		return nil
	}
	options := []string{"MIT", "Apache-2.0", "None"}
	defaultIdx := 0
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

func askGumTemplate(ctx context.Context, deps Deps, p *scaffold.Project) error {
	if p.Template != "" && p.Template != "tui-bubbletea" {
		return nil
	}
	options := templateOptionsForType(p.Type)
	if len(options) == 0 {
		return nil
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

// Track the user's last invalid input in `last` so the final error
// reports what they typed, not the empty p.TemplateRepo.
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

type templateOption struct {
	display string
	key     string
}

func templateOptionsForType(typ string) []templateOption {
	switch typ {
	case "tui":
		return []templateOption{{"Bubble Tea hello world", "tui-bubbletea"}}
	case "cli":
		return []templateOption{{"Cobra + Fang CLI", "cli-cobra-fang"}}
	case "all":
		return []templateOption{
			{"Bubble Tea hello world", "tui-bubbletea"},
			{"Cobra + Fang CLI", "cli-cobra-fang"},
		}
	}
	return nil
}

func isCanceled(err error) bool {
	if err == nil {
		return false
	}
	var c *Canceled
	return errors.As(err, &c)
}

func logBackend(name string) {
	log.Default().Debug("prompt backend", "backend", name)
}
