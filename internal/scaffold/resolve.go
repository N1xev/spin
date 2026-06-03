// Package scaffold: ResolveFlags binds cobra command flags to a *Project.
//
// This is the single place where CLI flag strings become Project struct
// fields. It enforces the cross-field invariants documented in
// RESEARCH §5.2 (e.g. --bubbles implies --bubbletea) and populates the
// derived fields (Year, SpinVer, Module defaulting to Name).
//
// ResolveFlags does NOT validate. Call p.Validate() after ResolveFlags to
// enforce name-regex and existing-directory checks.
package scaffold

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/version"
)

// ResolveFlags reads every registered flag from cmd and populates a fresh
// *Project. It also computes the derived fields:
//
//   - Module defaults to Name when --module is empty
//   - Libs is sorted and deduped; --bubbles implies --bubbletea
//   - Type is "tui" by default, "cli" with --cli, "all" with --all
//   - Year is the current year
//   - SpinVer is the spin version (ldflags-overridable)
//
// Returns the populated *Project or an error if any flag read fails.
func ResolveFlags(cmd *cobra.Command, args []string) (*Project, error) {
	if len(args) < 1 {
		return nil, &ArgError{Message: "missing project name (positional argument)"}
	}

	p := &Project{Name: args[0]}

	// String flags
	if v, err := mustString(cmd, "module"); err != nil {
		return nil, err
	} else {
		p.Module = v
	}
	if v, err := mustString(cmd, "license"); err != nil {
		return nil, err
	} else {
		// Normalize --license to lowercase so callers can pass "MIT" or
		// "Apache-2.0" and still match the whitelist in validate.go. CR-002.
		p.License = strings.ToLower(v)
	}
	if v, err := mustString(cmd, "template"); err != nil {
		return nil, err
	} else {
		p.Template = v
	}
	if v, err := mustString(cmd, "template-repo"); err != nil {
		return nil, err
	} else {
		p.TemplateRepo = v
	}
	// TMPL-03: reject obviously-invalid --template-repo values before
	// the git clone attempt. The check is permissive (any of http(s)://,
	// git://, file://, git@); git itself returns the real error for
	// unreachable URLs.
	//
	// WR-010: an explicit empty string (--template-repo "") is rejected
	// here even though IsValidTemplateRepo("") already returns false —
	// the explicit guard ensures a user passing "" gets a clear "must
	// not be empty" message instead of a generic "invalid" one. The
	// default (no --template-repo at all) sets TemplateRepo="" too, but
	// we only hit this branch when the user actually passed the flag.
	//
	// We can't tell apart "default" from "explicitly empty" through
	// cobra's GetString — both produce "". So we apply the check
	// unconditionally: if TemplateRepo is empty, it's a no-op (the
	// default path), and the empty-after-explicit case is unreachable
	// in practice. If a user really wants to pass "" they can simply
	// omit the flag.
	if p.TemplateRepo != "" && !IsValidTemplateRepo(p.TemplateRepo) {
		return nil, &ArgError{
			Message: "--template-repo " + p.TemplateRepo +
				": must start with https://, http://, git://, file://, or git@ (ssh-agent), " +
				"and the first path segment must not start with '-' (CR-004)",
		}
	}
	// WR-010: distinguish "user passed --template-repo" (Changed=true)
	// from "default empty value" (Changed=false). An explicit "" is a
	// user error — they intended to point at a repo and pointed at
	// nothing. Reject with a clear message.
	if p.TemplateRepo == "" && cmd.Flags().Changed("template-repo") {
		return nil, &ArgError{
			Message: "--template-repo must not be empty (omit the flag to use the embedded templates)",
		}
	}

	// Bool flags — behavior flags
	if v, err := mustBool(cmd, "force"); err != nil {
		return nil, err
	} else {
		p.Force = v
	}
	if v, err := mustBool(cmd, "no-git"); err != nil {
		return nil, err
	} else {
		p.NoGit = v
	}
	if v, err := mustBool(cmd, "no-verify"); err != nil {
		return nil, err
	} else {
		p.NoVerify = v
	}
	if v, err := mustBool(cmd, "quiet"); err != nil {
		return nil, err
	} else {
		p.Quiet = v
	}
	if v, err := mustBool(cmd, "keep-template-cache"); err != nil {
		return nil, err
	} else {
		p.KeepTemplateCache = v
	}

	// Type resolution (mutually-exclusive project variants).
	cli, _ := cmd.Flags().GetBool("cli")
	all, _ := cmd.Flags().GetBool("all")
	tui, _ := cmd.Flags().GetBool("tui")
	switch {
	case all:
		p.Type = "all"
	case cli:
		p.Type = "cli"
	default:
		// --tui is the default if no --cli/--all; matches the Walking Skeleton.
		p.Type = "tui"
		if tui {
			// explicit --tui is the same as default; left as a hook for future
			// behavior toggles.
			p.Type = "tui"
		}
	}

	// Libs — accumulate from --bubbletea, --bubbles, --lipgloss (the Phase 1
	// TUI libs). Sort + dedupe for determinism.
	libs := []string{}
	if b, _ := cmd.Flags().GetBool("bubbletea"); b {
		libs = append(libs, "bubbletea")
	}
	if b, _ := cmd.Flags().GetBool("bubbles"); b {
		libs = append(libs, "bubbles")
	}
	if b, _ := cmd.Flags().GetBool("lipgloss"); b {
		libs = append(libs, "lipgloss")
	}
	// --bubbles implies --bubbletea because bubbles is a layer on top of
	// bubbletea. RESEARCH §5.2.
	if containsString(libs, "bubbles") && !containsString(libs, "bubbletea") {
		libs = append(libs, "bubbletea")
	}
	// --tui implies --bubbletea: a TUI project without bubbletea has no
	// program loop, so variant_tui/main.go.tmpl (which always wraps a
	// bubbletea Model + tea.NewProgram) would emit an import for a module
	// that go.mod does not require. CR-001.
	if p.Type == "tui" && !containsString(libs, "bubbletea") {
		libs = append(libs, "bubbletea")
	}
	sort.Strings(libs)
	libs = dedupStrings(libs)
	p.Libs = libs

	// Forward-compat bool flags (Phase 2/3/4). Flag binding only — template
	// content is added by the corresponding phase. See the struct comment.
	for _, b := range []struct {
		flag  string
		field *bool
	}{
		{"cobra", &p.Cobra},
		{"fang", &p.Fang},
		{"viper", &p.Viper},
		{"huh", &p.Huh},
		{"glamour", &p.Glamour},
		{"glow", &p.Glow},
		{"wish", &p.Wish},
		{"log", &p.Log},
		{"harmonica", &p.Harmonica},
		{"modifiers", &p.Modifiers},
		{"ansi", &p.Ansi},
		{"runewidth", &p.Runewidth},
		{"ai", &p.AI},
	} {
		v, err := mustBool(cmd, b.flag)
		if err != nil {
			return nil, err
		}
		*b.field = v
	}

	// Variant auto-defaults. WR-003 / CR-002 / CR-003:
	//
	//   --tui   → --bubbletea (Phase 1 invariant; already applied above)
	//   --cli   → --cobra + --fang (matches the Phase 1 pattern; a CLI
	//             project without cobra+fang is unbuildable because the
	//             variant_cli/main.go.tmpl always wraps a cobra rootCmd)
	//   --all   → --bubbletea + --cobra + --fang (same reason; the
	//             variant_all template composes both halves)
	//
	// Without this block, a user running `spin new myapp --cli` gets a
	// project that imports cobra+fang in main.go but does not list them
	// in go.mod — `go build` fails. This is the cluster of CR-002,
	// CR-003, and WR-003.
	//
	// NOTE: this block MUST run AFTER the bool-flag binding loop above
	// so it overrides the bound (false) values from --cobra / --fang when
	// the user did not pass those flags explicitly.
	if p.Type == "cli" || p.Type == "all" {
		p.Cobra = true
		p.Fang = true
	}
	if p.Type == "all" {
		// --all also implies --bubbletea; we already added "bubbletea"
		// to p.Libs above for the tui path, but for the --all path we
		// need to add it here because the tui-specific block did not fire.
		if !containsString(p.Libs, "bubbletea") {
			p.Libs = append(p.Libs, "bubbletea")
			sort.Strings(p.Libs)
			p.Libs = dedupStrings(p.Libs)
		}
	}

	// Default Module = Name when --module is empty.
	if p.Module == "" {
		p.Module = p.Name
	}

	// Derived fields.
	p.Year = time.Now().Year()
	p.SpinVer = version.Version

	return p, nil
}

// mustString returns the value of a string flag or an error if the flag
// is not registered. All Plan 02 flags are registered in init() so this
// should never error in production; the error path exists for tests that
// build a partial command.
func mustString(cmd *cobra.Command, name string) (string, error) {
	if cmd.Flags().Lookup(name) == nil && cmd.PersistentFlags().Lookup(name) == nil {
		return "", &FlagError{Flag: name, Message: "not registered"}
	}
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		// Try persistent flags too.
		v, err = cmd.PersistentFlags().GetString(name)
	}
	return v, err
}

// mustBool returns the value of a bool flag or an error if the flag
// is not registered. See mustString for rationale.
func mustBool(cmd *cobra.Command, name string) (bool, error) {
	if cmd.Flags().Lookup(name) == nil && cmd.PersistentFlags().Lookup(name) == nil {
		return false, &FlagError{Flag: name, Message: "not registered"}
	}
	v, err := cmd.Flags().GetBool(name)
	if err != nil {
		v, err = cmd.PersistentFlags().GetBool(name)
	}
	return v, err
}

// dedupStrings returns a new slice with duplicates removed, preserving
// the order of first occurrence.
func dedupStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// containsString reports whether s contains v.
func containsString(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// ArgError is returned by ResolveFlags when args are missing.
type ArgError struct {
	Message string
}

func (e *ArgError) Error() string { return "scaffold: " + e.Message }

// FlagError is returned by mustString/mustBool when a flag is missing.
type FlagError struct {
	Flag    string
	Message string
}

func (e *FlagError) Error() string {
	return "scaffold: flag --" + e.Flag + ": " + e.Message
}
