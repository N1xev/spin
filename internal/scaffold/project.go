// Package scaffold renders and writes a new Go project tree.
package scaffold

import "sort"

// Project captures the inputs and options for a scaffold run.
type Project struct {
	Name string
	Module string
	Type string
	Libs []string
	License string
	Template string
	// ExternalDir is the on-disk path of a cloned template repo, or
	// empty to use the embedded templates. Set by runNew after a
	// successful clone.
	ExternalDir string
	// KeepTemplateCache retains the cloned template repo on disk
	// after scaffolding completes.
	KeepTemplateCache bool
	// TemplateRepo is the --template-repo flag value, validated by
	// IsValidTemplateRepo before any clone.
	TemplateRepo string
	Force bool
	NoGit bool
	NoVerify bool
	Quiet bool
	// NoInteractive disables interactive prompts. Read in cmd/new.go
	// before calling prompt.Fill; Fill itself does not consult it.
	NoInteractive bool
	Year int
	// SpinVer is the spin version emitted in generated file headers.
	SpinVer string

	Cobra bool
	Fang  bool
	Viper bool

	// Huh, Glamour, Wish, Log, Harmonica are charm library flags.
	Huh       bool
	Glamour   bool
	Wish      bool
	Log       bool
	Harmonica bool

	// Ansi, Runewidth are charmbracelet/x subpackage flags.
	Ansi      bool
	Runewidth bool

	// AI enables the AGENTS.md file in the generated project.
	AI bool
}

// AllLibs returns the project's full library set as a sorted,
// deduplicated slice: p.Libs unioned with the bool-flag libraries
// (Cobra, Fang, Viper, Huh, Glamour, Wish, Log, Harmonica). The
// result is empty (not nil) for a zero *Project.
func (p *Project) AllLibs() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, lib := range p.Libs {
		if !seen[lib] {
			seen[lib] = true
			out = append(out, lib)
		}
	}
	for lib, active := range p.libBoolMap() {
		if active && !seen[lib] {
			seen[lib] = true
			out = append(out, lib)
		}
	}
	sort.Strings(out)
	return out
}

// libBoolMap returns the bool flag → library name mapping for the
// libraries that do not have their own lib/<name>/ overlay.
func (p *Project) libBoolMap() map[string]bool {
	return map[string]bool{
		"cobra":     p.Cobra,
		"fang":      p.Fang,
		"viper":     p.Viper,
		"huh":       p.Huh,
		"glamour":   p.Glamour,
		"wish":      p.Wish,
		"log":       p.Log,
		"harmonica": p.Harmonica,
	}
}
