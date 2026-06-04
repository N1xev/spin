// Package prompt: library catalog.
package prompt

// Library describes one entry in the charm-library catalog.
type Library struct {
	Name    string
	Display string
	// DefaultFor is the variant this lib auto-on for: "tui" | "cli" | "all" | "".
	DefaultFor string
	// AlwaysOn means the lib is forced on for its DefaultFor variant.
	AlwaysOn bool
}

// LibCatalog is the canonical list of charm libraries spin scaffolds
// into a project. Order is alphabetical for deterministic AGENTS.md output.
var LibCatalog = []Library{
	{Name: "bubbles", Display: "Bubbles", DefaultFor: "", AlwaysOn: false},
	{Name: "bubbletea", Display: "Bubble Tea", DefaultFor: "tui", AlwaysOn: true},
	{Name: "cobra", Display: "Cobra", DefaultFor: "cli", AlwaysOn: true},
	{Name: "fang", Display: "Fang", DefaultFor: "cli", AlwaysOn: true},
	{Name: "glamour", Display: "Glamour", DefaultFor: "", AlwaysOn: false},
	{Name: "harmonica", Display: "Harmonica", DefaultFor: "", AlwaysOn: false},
	{Name: "huh", Display: "Huh", DefaultFor: "", AlwaysOn: false},
	{Name: "lipgloss", Display: "Lip Gloss", DefaultFor: "", AlwaysOn: false},
	{Name: "log", Display: "Log", DefaultFor: "", AlwaysOn: false},
	{Name: "viper", Display: "Viper", DefaultFor: "", AlwaysOn: false},
	{Name: "wish", Display: "Wish", DefaultFor: "", AlwaysOn: false},
}

// LibsForType returns the default library Names for the given project
// variant. "all" returns the union of tui and cli defaults. The
// result is in catalog order; callers may mutate the returned slice.
func LibsForType(typ string) []string {
	var out []string
	for _, lib := range LibCatalog {
		if !lib.AlwaysOn && lib.DefaultFor == "" {
			continue
		}
		if lib.DefaultFor == typ {
			out = append(out, lib.Name)
			continue
		}
		if typ == "all" && (lib.DefaultFor == "tui" || lib.DefaultFor == "cli") {
			out = append(out, lib.Name)
		}
	}
	return out
}

// DefaultLibsFor is a convenience alias for LibsForType.
func DefaultLibsFor(typ string) []string {
	return LibsForType(typ)
}

// libBoolMirror maps catalog Name → Go field name on *scaffold.Project.
// Libraries without a per-lib bool (bubbletea, bubbles, lipgloss) are
// absent; they live in p.Libs only.
var libBoolMirror = map[string]string{
	"cobra":     "Cobra",
	"fang":      "Fang",
	"viper":     "Viper",
	"huh":       "Huh",
	"glamour":   "Glamour",
	"wish":      "Wish",
	"log":       "Log",
	"harmonica": "Harmonica",
}
