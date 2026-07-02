package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"charm.land/huh/v2"

	"github.com/N1xev/spin/internal/params"
	"github.com/N1xev/spin/internal/registry"
	"github.com/N1xev/spin/internal/template"
)

var newCmd = &cobra.Command{
	Use:   "new <name> [<template>]",
	Short: "Scaffold a new project from a template",
	Long:  "Scaffold a new project from an external template. A template is a git repo, local path, or pinned spec that contains a spin.toml manifest and a _base/ tree of overlay files. If <name> or <template> is omitted, spin prompts for it when running interactively.",
	Example: `  # Positional name + template (no flag needed)
  spin new myapp my-template
  spin new demoapp https://github.com/me/go-cli-template.git

  # Interactive: spin asks for the name (and template) when missing
  spin new

  # From a pinned template (recommended; works offline)
  spin new myapp --template my-template

  # From a local path
  spin new myapp --template ~/code/templates/go-cli

  # Non-interactive (CI / scripts): pre-set every param
  spin new myapp go-cli --param port=8080 --param api_key=... --param features=ci,release

  # Preview the template's params without scaffolding
  spin new myapp <spec> --print-params

  # Preview the rendered tree without writing files
  spin new myapp <spec> --dry-run`,
	Args:          validateNewArgs,
	RunE:          runNew,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var (
	newTemplate    string
	newDest        string
	newPrintParams bool
	newDryRun      bool
	newParams      []string
)

func init() {
	newCmd.Flags().StringVarP(&newTemplate, "template", "t", "", "template spec: user/repo, git URL, or local path (or pass positionally as the second argument)")
	newCmd.Flags().StringVarP(&newDest, "dest", "d", "", "destination directory (default: ./<name>)")
	newCmd.Flags().BoolVar(&newPrintParams, "print-params", false, "print the template's params as JSON and exit (no files written)")
	newCmd.Flags().BoolVar(&newDryRun, "dry-run", false, "render to a temp dir, print the file list, and clean up (no project written)")
	newCmd.Flags().StringArrayVar(&newParams, "param", nil, "set a template param as key=value (repeatable); skips the interactive form. Use --print-params to discover valid keys.")
	rootCmd.AddCommand(newCmd)
}

// runNew is the RunE for `spin new`. Resolves name + template
// (positional / flag / interactive prompt), loads the template,
// collects params (interactive or via --param), renders the
// project, and runs [[post]] steps. Honors --print-params and
// --dry-run as preview-only short circuits.
func runNew(cmd *cobra.Command, args []string) error {
	name, tplSpec, err := resolveNameAndTemplate(cmd, args)
	if err != nil {
		return err
	}
	dest, err := resolveDest(newDest, name)
	if err != nil {
		return err
	}

	loader := template.NewLoader("")
	if isInteractive() {
		loader.PromptInvalidPinned = promptInvalidPinned
		loader.PromptExistingDest = promptExistingDest
	}
	tpl, err := loader.Load(tplSpec)
	if err != nil {
		return fmt.Errorf("spin new: %w", err)
	}

	values := map[string]any{
		"name":         name,
		"project_name": name,
	}
	if len(newParams) > 0 {
		parsed, err := applyParamFlags(tpl, newParams)
		if err != nil {
			return fmt.Errorf("spin new: %w", err)
		}
		maps.Copy(values, parsed)
	}
	// --print-params, --dry-run, and --param all force
	// non-interactive so we never reask the user for answers they
	// already supplied.
	interactive := isInteractive() && !newPrintParams && !newDryRun && len(newParams) == 0
	resolved, err := tpl.ResolveForm(values, interactive)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			// Bypass fang's "Aborted." banner. Exit 130 = SIGINT.
			printInfo("cancelled -- no project was created")
			os.Exit(130)
		}
		return fmt.Errorf("spin new: resolve params: %w", err)
	}

	if newPrintParams {
		return printResolvedParams(tpl, resolved)
	}
	if newDryRun {
		return dryRunRender(tpl, resolved, dest)
	}

	if err := tpl.RenderToWithPost(dest, resolved); err != nil {
		return fmt.Errorf("spin new: render: %w", err)
	}

	printSuccess("created %s at %s", name, dest)

	if isInteractive() && tpl.Repo != "" {
		promptPinAfterSuccess(name, tpl)
	}
	return nil
}

// printResolvedParams writes the template metadata + resolved
// values as JSON to stdout. Groups under `template` and `values`
// keys so user params named "description"/"version" don't collide
// with template-level fields.
func printResolvedParams(tpl *template.Template, values map[string]any) error {
	meta := map[string]any{
		"name": tpl.Name,
	}
	if tpl.SpinToml != nil {
		meta["version"] = tpl.SpinToml.Version
		meta["description"] = tpl.SpinToml.Description
		meta["type"] = tpl.SpinToml.Type
		meta["language"] = tpl.SpinToml.Language
	}
	out := map[string]any{
		"template": meta,
		"values":   values,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// dryRunRender renders to a temp dir and lists the file paths
// the project WOULD contain. No files are written to the dest.
func dryRunRender(tpl *template.Template, values map[string]any, dest string) error {
	files, err := tpl.Render(values)
	if err != nil {
		return fmt.Errorf("spin new: render: %w", err)
	}
	printInfo("dry run: would write %d files to %s", len(files), dest)
	for path := range files {
		fmt.Fprintf(os.Stdout, "  %s\n", filepath.Join(dest, path))
	}
	return nil
}

// isInteractive reports whether stdin is a TTY. Drives the
// non-interactive path (--param, --print-params, --dry-run, CI).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptInvalidPinned is the Loader.PromptInvalidPinned hook.
// Asks "keep the broken clone, or remove it?"
func promptInvalidPinned(name, localPath string, detectErr error) (bool, error) {
	var keep bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Template %q is broken -- keep it?", name)).
				Description(fmt.Sprintf("%v", detectErr)).
				Value(&keep).
				Affirmative("Keep").
				Negative("Remove"),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return true, nil // implicit "keep"
		}
		return false, err
	}
	return keep, nil
}

// promptExistingDest is the Loader.PromptExistingDest hook.
// Reuse uses the existing clone as-is; Pin also persists the
// source for offline use; Wipe re-clones; Cancel aborts.
func promptExistingDest(name, localPath string) (template.DestAction, error) {
	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("%q already exists at %s", name, localPath)).
				Description("Reuse the existing clone, pin it for future offline use, wipe and re-clone, or cancel?").
				Options(
					huh.NewOption("Reuse existing clone (no network)", "reuse"),
					huh.NewOption("Pin existing clone (reuse + remember for offline)", "pin"),
					huh.NewOption("Wipe and re-clone", "wipe"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return template.DestCancel, nil
		}
		return template.DestWipe, err
	}
	switch action {
	case "reuse":
		return template.DestReuse, nil
	case "pin":
		return template.DestPin, nil
	case "cancel":
		return template.DestCancel, nil
	default:
		return template.DestWipe, nil
	}
}

// promptPinAfterSuccess offers to pin a freshly-used remote
// template so future runs work offline. Skipped for local paths
// and already-pinned sources.
func promptPinAfterSuccess(_ string, tpl *template.Template) {
	client := registry.New()
	for _, p := range pinnedSnapshot(client) {
		if p.LocalPath == tpl.Source {
			return // already pinned
		}
	}
	var pin bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Pin %q for offline use?", tpl.Name)).
				Description(fmt.Sprintf("Source: %s\nFuture `spin new` calls can use --template %s without a network round-trip.", tpl.Repo, tpl.Name)).
				Value(&pin).
				Affirmative("Pin").
				Negative("Skip"),
		),
	)
	if err := form.Run(); err != nil {
		// User cancelled or huh failed; non-fatal, just skip.
		return
	}
	if !pin {
		return
	}
	pinned, err := client.Add(tpl.Repo)
	if err != nil {
		printHint("could not pin %q: %v", tpl.Repo, err)
		return
	}
	pinned.PinnedAt = time.Now().UTC().Format(time.RFC3339)
	if err := client.Pin(*pinned); err != nil {
		printHint("could not save pin: %v", err)
		return
	}
	printSuccess("pinned %q (cloned to %s)", pinned.Name, pinned.LocalPath)
}

// pinnedSnapshot returns the pin list. Swallows read errors:
// "no pins" is the safe answer for a corrupt pinned.json.
func pinnedSnapshot(client *registry.Client) []registry.Pinned {
	pinned, err := client.ListPinned()
	if err != nil {
		return nil
	}
	return pinned
}

// resolveDest returns the absolute destination path. Expands
// ~ and ~/ via the user's home; everything else via filepath.Abs
// so the success line always shows a full path.
func resolveDest(dest, name string) (string, error) {
	if dest == "" {
		return filepath.Abs(name)
	}
	if dest == "~" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("spin new: resolve ~: %w", err)
		}
		return h, nil
	}
	if strings.HasPrefix(dest, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("spin new: resolve ~/: %w", err)
		}
		return filepath.Abs(filepath.Join(h, dest[2:]))
	}
	return filepath.Abs(dest)
}

// applyParamFlags parses the repeated --param slice into a typed
// map keyed on the template's param spec. Unknown keys, malformed
// key=value, or out-of-range numbers produce clear errors naming
// the offending flag.
func applyParamFlags(tpl *template.Template, raw []string) (map[string]any, error) {
	out := make(map[string]any, len(raw))
	for i, entry := range raw {
		key, value, err := splitParamEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("--param[%d] %q: %w", i, entry, err)
		}
		spec, ok := tpl.SpinToml.Params[key]
		if !ok {
			return nil, fmt.Errorf("--param[%d] %q: unknown key %q (known: %s)", i, entry, key, joinKnownParams(tpl.SpinToml.Params))
		}
		coerced, err := coerceParamValue(spec, value)
		if err != nil {
			return nil, fmt.Errorf("--param[%d] %q: %w", i, entry, err)
		}
		out[key] = coerced
	}
	return out, nil
}

// splitParamEntry parses one "key=value" string. Rejects empty
// key and missing '='.
func splitParamEntry(s string) (key, value string, err error) {
	key, value, ok := strings.Cut(s, "=")
	if !ok {
		return "", "", fmt.Errorf("missing '=' (expected key=value)")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", fmt.Errorf("empty key")
	}
	return key, value, nil
}

// coerceParamValue parses raw (always a string from the CLI) into
// the primitive the param type expects.
func coerceParamValue(spec params.Spec, raw string) (any, error) {
	switch spec.Type {
	case params.TypeNumber:
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", raw)
		}
		if spec.Min != nil && n < *spec.Min {
			return nil, fmt.Errorf("%d is below min %d", n, *spec.Min)
		}
		if spec.Max != nil && n > *spec.Max {
			return nil, fmt.Errorf("%d is above max %d", n, *spec.Max)
		}
		return n, nil
	case params.TypeBool:
		b, err := parseLooseBool(raw)
		if err != nil {
			return nil, err
		}
		return b, nil
	case params.TypeMultiSelect:
		// Comma-split, trim each, drop empties so a trailing comma
		// or stray whitespace doesn't produce phantom options.
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	case params.TypeSelect:
		if len(spec.Options) > 0 && !slices.Contains(spec.Options, raw) {
			return nil, fmt.Errorf("not in options (want one of %v, got %q)", spec.Options, raw)
		}
		return raw, nil
	case params.TypeText, params.TypeTextarea, params.TypePath, params.TypeSecret, "":
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported param type %q", spec.Type)
	}
}

// parseLooseBool accepts truthy/falsy spellings: true/1/yes/y/on
// and false/0/no/n/off.
func parseLooseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	}
	return false, fmt.Errorf("expected bool, got %q (use true/false, 1/0, yes/no)", s)
}

// joinKnownParams renders the param spec keys in stable order
// for the unknown-key error path.
func joinKnownParams(specs map[string]params.Spec) string {
	names := make([]string, 0, len(specs))
	for k := range specs {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// validateNewArgs is the cobra.Args validator. Accepts 0, 1, or 2
// positionals and rejects positional <template> + --template.
func validateNewArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 2 {
		return fmt.Errorf("spin new: accepts at most 2 positional args (<name> [<template>]), got %d", len(args))
	}
	if len(args) == 2 && cmd.Flags().Changed("template") {
		return fmt.Errorf("spin new: cannot pass <template> both positionally and via --template")
	}
	return nil
}

// resolveNameAndTemplate fills name and template from args /
// --template / interactive prompts. Returns precise non-interactive
// errors naming whichever slot is missing.
func resolveNameAndTemplate(cmd *cobra.Command, args []string) (string, string, error) {
	var name, tpl string
	if len(args) >= 1 {
		name = args[0]
	}
	if len(args) >= 2 {
		tpl = args[1]
	}
	// Validator already rejected positional + --template; this is
	// defense in depth -- prefer positional if both slipped through.
	if cmd.Flags().Changed("template") {
		if tpl != "" && newTemplate != "" && tpl != newTemplate {
			printWarn("ignoring --template %q in favor of positional %q", newTemplate, tpl)
		} else if tpl == "" {
			tpl = newTemplate
		}
	}

	if name == "" {
		if !isInteractive() {
			return "", "", fmt.Errorf("spin new: <name> is required in non-interactive mode")
		}
		v, err := promptForName()
		if err != nil {
			return "", "", err
		}
		name = v
	}
	if tpl == "" {
		if !isInteractive() {
			return "", "", fmt.Errorf("spin new: <template> is required in non-interactive mode (use `spin search <query>` to find one, or `spin list` for pinned)")
		}
		v, err := promptForTemplate()
		if err != nil {
			return "", "", err
		}
		tpl = v
	}
	return name, tpl, nil
}

// promptForName asks for the project name via a huh Input. Exits
// 130 on cancel.
func promptForName() (string, error) {
	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Folder name; injected as {{ name }} / {{ project_name }} in the template.").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					return nil
				}),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			printInfo("cancelled -- no project was created")
			os.Exit(130)
		}
		return "", err
	}
	return strings.TrimSpace(name), nil
}

// promptForTemplate asks the user to pick a pinned template via a
// Select. Huh's Select has a built-in `/` filter, so the user
// narrows the list inline instead of answering a separate input.
// Returns a "no pinned templates" error when the cache is empty.
func promptForTemplate() (string, error) {
	client := registry.New()
	pinned, err := client.ListPinned()
	if err != nil {
		return "", fmt.Errorf("spin new: list pinned: %w", err)
	}
	if len(pinned) == 0 {
		return "", fmt.Errorf("spin new: no pinned templates; add a template link or path, or use `spin search` to find one")
	}

	names := make([]string, 0, len(pinned))
	for _, p := range pinned {
		names = append(names, p.Name)
	}
	sort.Strings(names)

	var choice string
	opts := make([]huh.Option[string], 0, len(names))
	for _, n := range names {
		opts = append(opts, huh.NewOption(n, n))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a pinned template").
				Value(&choice).
				Options(opts...),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			printInfo("cancelled -- no project was created")
			os.Exit(130)
		}
		return "", err
	}
	return choice, nil
}