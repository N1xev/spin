// Package scaffold: ResolveFlags binds cobra command flags to a *Project.
package scaffold

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/version"
)

// ResolveFlags constructs a *Project from the cobra command's flags
// and positional args. It does not validate; call p.Validate afterwards.
func ResolveFlags(cmd *cobra.Command, args []string) (*Project, error) {
	if len(args) < 1 {
		return nil, &ArgError{Message: "missing project name (positional argument)"}
	}

	p := &Project{Name: args[0]}

	if v, err := mustString(cmd, "module"); err != nil {
		return nil, err
	} else {
		p.Module = v
	}
	// Lowercase the license so "MIT" and "Apache-2.0" match the
	// whitelist in validate.go.
	if v, err := mustString(cmd, "license"); err != nil {
		return nil, err
	} else {
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
	if p.TemplateRepo != "" && !IsValidTemplateRepo(p.TemplateRepo) {
		return nil, &ArgError{
			Message: "--template-repo " + p.TemplateRepo +
				": must start with https://, http://, git://, file://, or git@ (ssh-agent), " +
				"and the first path segment must not start with '-'",
		}
	}
	if p.TemplateRepo == "" && cmd.Flags().Changed("template-repo") {
		return nil, &ArgError{
			Message: "--template-repo must not be empty (omit the flag to use the embedded templates)",
		}
	}

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
	if v, err := mustBool(cmd, "no-interactive"); err != nil {
		return nil, err
	} else {
		p.NoInteractive = v
	}
	// pflag v1.0.6 has no long-form alias API, so --yes and --batch
	// are registered separately and OR'd into p.NoInteractive.
	if v, err := mustBool(cmd, "yes"); err != nil {
		return nil, err
	} else if v {
		p.NoInteractive = true
	}
	if v, err := mustBool(cmd, "batch"); err != nil {
		return nil, err
	} else if v {
		p.NoInteractive = true
	}
	if v, err := mustBool(cmd, "keep-template-cache"); err != nil {
		return nil, err
	} else {
		p.KeepTemplateCache = v
	}

	cli, _ := cmd.Flags().GetBool("cli")
	all, _ := cmd.Flags().GetBool("all")
	tui, _ := cmd.Flags().GetBool("tui")
	switch {
	case all:
		p.Type = "all"
	case cli:
		p.Type = "cli"
	default:
		p.Type = "tui"
		if tui {
			p.Type = "tui"
		}
	}

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
	// --bubbles implies --bubbletea. --tui implies --bubbletea: a
	// TUI variant without bubbletea would import a module the go.mod
	// does not require.
	if containsString(libs, "bubbles") && !containsString(libs, "bubbletea") {
		libs = append(libs, "bubbletea")
	}
	if p.Type == "tui" && !containsString(libs, "bubbletea") {
		libs = append(libs, "bubbletea")
	}
	sort.Strings(libs)
	libs = dedupStrings(libs)
	p.Libs = libs

	for _, b := range []struct {
		flag  string
		field *bool
	}{
		{"cobra", &p.Cobra},
		{"fang", &p.Fang},
		{"viper", &p.Viper},
		{"huh", &p.Huh},
		{"glamour", &p.Glamour},
		{"wish", &p.Wish},
		{"log", &p.Log},
		{"harmonica", &p.Harmonica},
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
	// --agents is a long-form alias for --ai.
	if v, err := mustBool(cmd, "agents"); err != nil {
		return nil, err
	} else if v {
		p.AI = true
	}

	// Variant auto-defaults: --cli implies --cobra + --fang (the
	// variant_cli template always wraps a cobra rootCmd); --all
	// implies --bubbletea + --cobra + --fang. Without these, a user
	// running `spin new myapp --cli` gets a project that imports
	// cobra+fang in main.go but does not list them in go.mod, and
	// `go build` fails.
	//
	// Explicit user negation via --cobra=false is honored through
	// cmd.Flags().Changed(), so users can still opt out.
	if p.Type == "cli" || p.Type == "all" {
		if !cmd.Flags().Changed("cobra") {
			p.Cobra = true
		}
		if !cmd.Flags().Changed("fang") {
			p.Fang = true
		}
	}
	if p.Type == "all" {
		if !containsString(p.Libs, "bubbletea") &&
			!cmd.Flags().Changed("bubbletea") &&
			!cmd.Flags().Changed("bubbles") {
			p.Libs = append(p.Libs, "bubbletea")
			sort.Strings(p.Libs)
			p.Libs = dedupStrings(p.Libs)
		}
	}

	if p.Module == "" {
		p.Module = p.Name
	}

	p.Year = time.Now().Year()
	p.SpinVer = version.Version

	return p, nil
}

// mustString returns the value of a string flag, or an error if the
// flag is not registered.
func mustString(cmd *cobra.Command, name string) (string, error) {
	if cmd.Flags().Lookup(name) == nil && cmd.PersistentFlags().Lookup(name) == nil {
		return "", &FlagError{Flag: name, Message: "not registered"}
	}
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		v, err = cmd.PersistentFlags().GetString(name)
	}
	return v, err
}

// mustBool returns the value of a bool flag, or an error if the flag
// is not registered.
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

func containsString(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// ArgError reports a problem with the positional arguments.
type ArgError struct {
	Message string
}

func (e *ArgError) Error() string { return "scaffold: " + e.Message }

// FlagError reports a problem with a specific flag.
type FlagError struct {
	Flag    string
	Message string
}

func (e *FlagError) Error() string {
	return "scaffold: flag --" + e.Flag + ": " + e.Message
}
