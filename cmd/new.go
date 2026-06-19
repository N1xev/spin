// Package cmd wires the spin cobra root command.
//
// new.go is the v2 scaffolding entry point: `spin new <name>
// --template <spec>`. It loads an external template (git URL, local
// path, or future registry reference) via internal/template, prompts
// the user for any required params, renders the _base/ tree against
// the resolved values, and runs the template's [[post]] steps.
//
// The v2 ecosystem path (Ecosystem interface, defaultRegistry,
// dispatchV2) was archived in v2.x; templates are the only
// extension surface now.
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
	Use:   "new <name>",
	Short: "Scaffold a new project from a template",
	Long:  "Scaffold a new project from an external template. A template is a git repo, local path, or pinned spec that contains a spin.toml manifest and a _base/ tree of overlay files.",
	Example: `  # From a pinned template (recommended; works offline)
  spin new myapp --template my-template

  # From a local path
  spin new myapp --template ~/code/templates/go-cli

  # From a git URL
  spin new myapp --template https://github.com/me/go-cli-template.git

  # Non-interactive (CI / scripts): pre-set every param
  spin new myapp --template go-cli --param port=8080 --param api_key=... --param features=ci,release

  # Preview the template's params without scaffolding
  spin new myapp --template <spec> --print-params

  # Preview the rendered tree without writing files
  spin new myapp --template <spec> --dry-run`,
	Args:          cobra.ExactArgs(1),
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
	newCmd.Flags().StringVarP(&newTemplate, "template", "t", "", "template spec: user/repo, git URL, or local path (required)")
	newCmd.Flags().StringVarP(&newDest, "dest", "d", "", "destination directory (default: ./<name>)")
	newCmd.Flags().BoolVar(&newPrintParams, "print-params", false, "print the template's params as JSON and exit (no files written)")
	newCmd.Flags().BoolVar(&newDryRun, "dry-run", false, "render to a temp dir, print the file list, and clean up (no project written)")
	newCmd.Flags().StringArrayVar(&newParams, "param", nil, "set a template param as key=value (repeatable); skips the interactive form. Use --print-params to discover valid keys.")
	rootCmd.AddCommand(newCmd)
}

// runNew is the RunE for `spin new`. Loads a v2 template, resolves
// its params (interactive or default), renders the project tree,
// and runs the template's [[post]] steps.
//
// Two preview flags short-circuit the write:
//   --print-params: print the resolved params map as JSON and exit
//   --dry-run:      render to a temp dir, print the file list, exit
func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	if newTemplate == "" {
		return fmt.Errorf("spin new: --template is required (use `spin search <query>` to find one, or `spin list` for pinned)")
	}
	dest, err := resolveDest(newDest, name)
	if err != nil {
		return err
	}

	loader := template.NewLoader("")
	// Wire the keep/remove prompt for invalid pinned templates and
	// the reuse/pin/wipe prompt for an existing clone at the
	// destination. Only attach in interactive mode -- non-interactive
	// runs (CI, scripts) get the wipe-and-reclone fallback.
	if isInteractive() {
		loader.PromptInvalidPinned = promptInvalidPinned
		loader.PromptExistingDest = promptExistingDest
	}
	tpl, err := loader.Load(newTemplate)
	if err != nil {
		return fmt.Errorf("spin new: load template: %w", err)
	}

	values := map[string]any{
		"name":         name,
		"project_name": name,
	}
	// --param values layer on top of the name map, then any
	// template-defaults are applied by ResolveForm. Validation
	// against the param spec happens here so the user gets a
	// clear error before the (skipped) form or render.
	if len(newParams) > 0 {
		parsed, err := applyParamFlags(tpl, newParams)
		if err != nil {
			return fmt.Errorf("spin new: %w", err)
		}
		maps.Copy(values, parsed)
	}
	// --print-params and --dry-run should never open a form: the
	// caller wants the values as-is (defaults) without typing.
	// --param also forces non-interactive: the user already
	// supplied answers, so opening a form is hostile.
	interactive := isInteractive() && !newPrintParams && !newDryRun && len(newParams) == 0
	resolved, err := tpl.ResolveForm(values, interactive)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			// Bypass fang's error print (it would render "Aborted."
			// and duplicate our friendly message). Exit 130 = SIGINT.
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

	// After a successful scaffold from a remote source, offer to pin
	// it so future runs work offline. Skipped for local paths
	// (already on disk) and already-pinned templates.
	if isInteractive() && tpl.Repo != "" {
		promptPinAfterSuccess(name, tpl)
	}
	return nil
}

// printResolvedParams dumps the resolved param values as JSON so
// the caller can pipe them into a script. Honors --print-params.
//
// The output groups all template metadata under a `template` key
// (so user params named "description", "version", etc. don't
// collide with template-level fields), and the resolved values
// under a `values` key.
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

// dryRunRender walks the template's _base/ tree, lists the files
// that WOULD be written (with the rendered content for .tmpl files),
// then cleans up. No project is left on disk. Honors --dry-run.
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

// isInteractive reports whether stdin is a TTY. Used to decide
// whether to show the huh form or apply defaults silently.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptInvalidPinned is the Loader.PromptInvalidPinned callback.
// Drives a huh confirm: "Keep the broken clone, or remove it?".
// Short on purpose -- the user already saw the path in the error
// line, so the form just needs the question and the two actions.
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

// promptExistingDest is the Loader.PromptExistingDest callback.
// Asks the user what to do when a previous clone already lives at
// the destination. Three actions + cancel. Pin and Reuse are
// identical in the loader (both call Detect on the existing
// clone) -- the difference is the post-scaffold prompt also
// persists the pin, so the user gets `spin new --template <name>`
// working offline going forward. Wipe nukes the existing clone
// and falls through to a fresh `git clone`. Cancel aborts.
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

// promptPinAfterSuccess offers to pin a freshly-used template so
// subsequent `spin new` calls can use it offline. Fires only when
// the source was a remote URL (Repo != "") and the user is on a
// TTY. Local-path and pinned-name sources don't need this.
func promptPinAfterSuccess(_ string, tpl *template.Template) {
	client := registry.New()
	// Skip silently if it's already pinned (e.g. user re-ran on a
	// pin they had).
	for _, p := range pinnedSnapshot(client) {
		if p.LocalPath == tpl.Source {
			return
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

// pinnedSnapshot is a tiny helper that swallows errors from
// ListPinned -- the snapshot is best-effort; an unreadable
// pinned.json is "no pins", not a fatal condition.
func pinnedSnapshot(client *registry.Client) []registry.Pinned {
	pinned, err := client.ListPinned()
	if err != nil {
		return nil
	}
	return pinned
}

// resolveDest returns the absolute destination path for `spin new`.
// Precedence: explicit --dest (expanded and made absolute) > name
// (resolved against cwd). A leading "~" or "~/" is expanded via
// the user's home directory; everything else is made absolute via
// filepath.Abs so the user always sees a full path in the success
// line, not a relative path that depends on the shell's CWD.
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

// applyParamFlags turns a slice of `key=value` strings (from the
// repeated --param flag) into a typed map[string]any suitable for
// layering on top of ResolveForm's values. Coerces each value
// according to the param's declared Type (number -> int, bool ->
// bool, multiselect -> []string via comma split) so the rendered
// template sees the right primitive. Unknown keys, malformed
// `key=value` syntax, or out-of-range numbers produce clear errors
// that name the offending flag.
//
// The returned map only contains the keys the caller passed; the
// template's own defaults are still applied later by ResolveForm
// (which respects our explicit overrides).
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

// splitParamEntry parses one --param argument. Accepts
//   key=value
// but rejects empty key, empty value (since `key=` is almost
// always a typo and easy to surface), or missing `=`.
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
// the primitive the param type expects. The set of supported
// coercions mirrors what `huh` writes back via Apply/Value.
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
		// or whitespace doesn't produce phantom options.
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

// parseLooseBool accepts the obvious truthy/falsy spellings so
// `spin new --param verbose=true` and `--param verbose=1` both
// work. Anything else errors out with a helpful message.
func parseLooseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	}
	return false, fmt.Errorf("expected bool, got %q (use true/false, 1/0, yes/no)", s)
}

// joinKnownParams renders the param spec keys in a stable order
// for the unknown-key error. Used only on the error path.
func joinKnownParams(specs map[string]params.Spec) string {
	names := make([]string, 0, len(specs))
	for k := range specs {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
