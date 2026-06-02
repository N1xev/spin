// Package scaffold: CharmPins holds the verified charmbracelet v2 version
// pins used by the template engine. These are the EXACT versions the
// generated go.mod will reference, so they must be hand-pinned (not
// @latest) for reproducible builds. RESEARCH §3 cites the verified versions.
package scaffold

// CharmPins is the single source of truth for charm v2 versions emitted
// in the generated go.mod. Pin exact versions (no @latest) so two scaffolds
// a month apart produce identical go.mod files.
type CharmPins struct {
	// Bubbletea is the v2 TUI framework. Stable: v2.0.0.
	Bubbletea string

	// Lipgloss is the v2 styling library. v2 stable line is v2.0.0-beta.2.
	Lipgloss string

	// Bubbles is the v2 TUI components library. Requires Go 1.25.0+.
	Bubbles string

	// Log is the v2 structured logger. v2.0.0 stable.
	Log string
}

// DefaultPins is the package-level pin set used by the template engine.
// Update these (and re-run `spin update`) when new v2 versions ship.
var DefaultPins = CharmPins{
	Bubbletea: "v2.0.0",
	Lipgloss:  "v2.0.0-beta.2",
	Bubbles:   "v2.0.0",
	Log:       "v2.0.0",
}
