// Package prompt — Library catalog (INT-05 / Plan 02).
//
// Single source of truth for the 11 charm libraries `spin new` can
// scaffold. Both huh v2 and gum backends consume it to populate the
// multi-select, pre-select variant defaults (LibsForType), and
// mirror the multi-select answer into p.Libs + per-lib bool fields.
//
// Order is alphabetical so iteration is deterministic across runs
// (UI-SPEC §"Surface B / Determinism contract").
//
// ansi and runewidth are excluded: scaffolder-only, not user-facing
// in the generated project.
package prompt

// Library describes one entry in the charm-library catalog. The fields
// are the canonical contract for both prompt backends and the AGENTS.md
// template renderer.
type Library struct {
	Name string
	Display string
	// DefaultFor is the variant this lib auto-on for: "tui" | "cli" | "all" | "".
	// "all" is handled specially by LibsForType: any lib whose DefaultFor
	// is "tui" or "cli" is also returned for "all".
	DefaultFor string
	// AlwaysOn means the user cannot un-select the lib (forced-on for some
	// variant). Strict enforcement is deferred; huh form uses this as a
	// "cannot be un-selected" hint only.
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
// variant. "all" returns the union of tui and cli defaults so a combined
// TUI+CLI project gets both halves' forced libs. Result follows catalog
// order; callers may mutate the returned slice.
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

// DefaultLibsFor is a convenience alias for LibsForType. Exists for
// call-site readability ("DefaultLibsFor(tui)" reads more naturally
// when the question is "what are the defaults?").
func DefaultLibsFor(typ string) []string {
	return LibsForType(typ)
}

// libBoolMirror maps each catalog Name to the *bool field on
// *scaffold.Project that the multi-select answer must mirror. Single
// source of truth for "update BOTH p.Libs and the bool field" write-back
// (Pitfall 6 in 03-RESEARCH.md). Libs without a per-lib bool (bubbletea,
// bubbles, lipgloss) are deliberately absent: those live in p.Libs only.
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
