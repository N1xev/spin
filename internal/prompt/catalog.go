// Package prompt — Library catalog (INT-05 / Plan 02).
//
// The catalog is the single source of truth for the 11 charm libraries
// that `spin new` can scaffold into a generated project. Both the huh
// v2 backend (this plan) and the gum backend (Plan 03) consume this
// catalog to:
//
//   - populate the multi-select prompt's options (Display as label,
//     Name as value)
//   - pre-select the variant defaults (LibsForType)
//   - re-derive p.Libs and the per-lib bool fields from a user's
//     multi-select answer (mirrorMap at the bottom of this file)
//
// The catalog is intentionally static data (no reflection, no map
// iteration) so the multi-select prompt's option order is stable
// across runs — Plan 03's AGENTS.md template relies on this for the
// deterministic-output contract (UI-SPEC §"Surface B / Determinism
// contract").
//
// ansi and runewidth are excluded per the plan: they are tooling
// libraries used by the scaffolder itself, not user-facing libraries
// the generated project should import directly.
package prompt

// Library describes one entry in the charm-library catalog. The fields
// are the canonical contract for both prompt backends and the
// AGENTS.md template renderer (Plan 04).
type Library struct {
	// Name is the machine key used in p.Libs and on the CLI flag
	// (--bubbletea, --cobra, --huh, ...). It is also the directory
	// name under templates/lib/<name>/.
	Name string

	// Display is the human-readable label used in the multi-select
	// prompt and in AGENTS.md section headers ("Bubble Tea", "Cobra").
	Display string

	// DefaultFor is the project variant this lib is auto-on for:
	// "tui" | "cli" | "all" | "". An empty string means the lib is
	// never default — the user has to opt in via a flag or the
	// multi-select prompt.
	//
	// "all" is handled specially by LibsForType: any lib whose
	// DefaultFor is "tui" or "cli" is also returned for "all" so a
	// combined TUI+CLI project gets both halves' defaults.
	DefaultFor string

	// AlwaysOn means the user cannot un-select the lib in the
	// multi-select prompt. Set this only for libs that are forced
	// on for some variant (cobra+fang for --cli, bubbletea for --tui).
	// Plan 02 only USES this field as a hint for the huh form
	// description ("cannot be un-selected"); the strict enforcement
	// (a separate form-runner) is deferred.
	AlwaysOn bool
}

// LibCatalog is the canonical list of charm libraries spin scaffolds
// into a project. Order is alphabetical by Name so iteration order is
// deterministic across runs (deterministic AGENTS.md output).
//
// The 11 entries are exactly the libraries from UI-SPEC
// §"Library lookup table" minus ansi, runewidth, glow, and modifiers
// (scaffolder-only libraries; not user-facing in the generated project,
// or removed in the glow/modifiers drop quick task).
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

// LibsForType returns the default library Names for the given
// project variant. The result is the union of:
//
//   - every catalog entry with AlwaysOn=true and DefaultFor=typ
//   - when typ=="all", every entry with DefaultFor in {"tui","cli"}
//
// The order follows the catalog's alphabetical order (which is the
// order of the multi-select prompt). The result is a fresh slice;
// callers may mutate it freely.
//
// Used by the multi-select prompt to pre-select the variant defaults
// and by Plan 04's AGENTS.md template to render the default library
// list in the absence of user input.
func LibsForType(typ string) []string {
	var out []string
	for _, lib := range LibCatalog {
		if !lib.AlwaysOn && lib.DefaultFor == "" {
			// Pure opt-in libs are never default for any variant.
			continue
		}
		if lib.DefaultFor == typ {
			out = append(out, lib.Name)
			continue
		}
		// "all" is the union of tui and cli defaults so a combined
		// TUI+CLI project gets both halves' forced libs (bubbletea,
		// cobra, fang).
		if typ == "all" && (lib.DefaultFor == "tui" || lib.DefaultFor == "cli") {
			out = append(out, lib.Name)
		}
	}
	return out
}

// DefaultLibsFor is a convenience alias for LibsForType. It exists
// for readability at call sites ("DefaultLibsFor(tui)" reads more
// naturally than "LibsForType(tui)" when the question being asked is
// "what are the defaults?").
func DefaultLibsFor(typ string) []string {
	return LibsForType(typ)
}

// libBoolMirror maps each catalog Name to the *bool field on
// *scaffold.Project that the multi-select prompt's answer must
// mirror. The map is the single source of truth for the "result of
// the multi-select updates BOTH p.Libs and the bool field" write-back
// logic (Pitfall 6 in 03-RESEARCH.md).
//
// Libs that don't have a per-lib bool (bubbletea, bubbles, lipgloss)
// are deliberately absent: those live in p.Libs only. The three
// "transitively required" libs (bubbletea, cobra, fang) are still
// tracked here when they have a bool (cobra, fang) so the bool is
// kept in sync.
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
