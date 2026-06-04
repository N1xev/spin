// Package scaffold: CharmPins holds the verified charmbracelet v2 version
// pins used by the template engine. Exact versions (not @latest) for
// reproducible builds.
//
// Split into two structs intentionally:
//   - CharmPins — libraries on charm.land/<lib>/v2 (canonical v2 paths).
//   - LegacyCharmPins — libraries still on github.com/charmbracelet/
//     (harmonica), so the per-library v1-leak grep suite can disambiguate.
package scaffold

// CharmPins is the single source of truth for charm v2 versions emitted
// in the generated go.mod. Verified 2026-06-03 against go list -m -versions
// + Context7 upgrade guides.
type CharmPins struct {
	// Bubbletea is the v2 TUI framework.
	Bubbletea string
	// Lipgloss is the v2 styling library.
	Lipgloss string
	// Bubbles is the v2 TUI components library. Requires Go 1.25.0+,
	// which drives the unconditional `go 1.25.0` directive in _base/go.mod.tmpl.
	Bubbles string
	// Huh is the v2 accessible forms library.
	Huh string
	// Glamour is the v2 markdown renderer.
	Glamour string
	// Wish is the v2 SSH server framework.
	Wish string
	// Log is the v2 structured logger.
	Log string
	// Fang is the v2 styled cobra wrapper. Used by the --cli variant.
	Fang string
	// Viper is the optional config-file library (github.com/spf13/viper).
	// Not a charm v2 lib, but lives on the same struct so charmPin("viper")
	// resolves from here. Per CLAUDE.md, Viper is opt-in (--viper).
	Viper string
}

// LegacyCharmPins tracks libraries that have NOT migrated to charm.land/.
// These are the current paths, NOT v1 leaks — the CI grep suite must
// allow them (per scripts/check-v1-leaks.sh).
type LegacyCharmPins struct {
	// Harmonica is the spring animation library. Still on
	// github.com/charmbracelet/harmonica; mature and not expected to migrate.
	Harmonica string
}

// DefaultPins is the package-level pin set used by the template engine.
// Update these (and re-run `spin update`) when new v2 versions ship.
var DefaultPins = CharmPins{
	Bubbletea: "v2.0.7",
	Lipgloss:  "v2.0.3",
	Bubbles:   "v2.1.0",
	Huh:       "v2.0.3",
	Glamour:   "v2.0.0",
	Wish:      "v2.0.1",
	Log:       "v2.0.0",
	Fang:      "v2.0.1",
	Viper:     "v1.20.1",
}

// DefaultLegacyPins is the package-level pin set for the
// github.com/charmbracelet/ libs that have not migrated.
var DefaultLegacyPins = LegacyCharmPins{
	Harmonica: "v0.2.0",
}
