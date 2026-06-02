// Package scaffold: CharmPins holds the verified charmbracelet v2 version
// pins used by the template engine. These are the EXACT versions the
// generated go.mod will reference, so they must be hand-pinned (not
// @latest) for reproducible builds. RESEARCH §2.1 cites the verified versions.
//
// The pins are split into two structs intentionally:
//
//   - CharmPins — libraries that migrated to charm.land/<lib>/v2. These
//     are the canonical v2 paths and are what the generated go.mod emits.
//   - LegacyCharmPins — libraries still on github.com/charmbracelet/
//     because they pre-date the migration (harmonica) or are binaries
//     served from the github.com path (glow). Kept on a separate struct
//     so the per-library v1-leak grep suite can disambiguate.
package scaffold

// CharmPins is the single source of truth for charm v2 versions emitted
// in the generated go.mod. Pin exact versions (no @latest) so two scaffolds
// a month apart produce identical go.mod files.
//
// Verified 2026-06-03 against go list -m -versions + Context7 upgrade guides.
type CharmPins struct {
	// Bubbletea is the v2 TUI framework. Latest stable: v2.0.7.
	Bubbletea string

	// Lipgloss is the v2 styling library. Latest stable: v2.0.3
	// (the v2.0.0-beta.2 module-path mismatch was the MEMORY.md entry
	// flagged for pre-Phase-2 bump; resolved here).
	Lipgloss string

	// Bubbles is the v2 TUI components library. Latest stable: v2.1.0.
	// Requires Go 1.25.0+ (this is the floor that drove the unconditional
	// `go 1.25.0` directive in _base/go.mod.tmpl).
	Bubbles string

	// Huh is the v2 accessible forms library. Latest stable: v2.0.3.
	Huh string

	// Glamour is the v2 markdown renderer. Latest stable: v2.0.0.
	Glamour string

	// Wish is the v2 SSH server framework. Latest stable: v2.0.1.
	Wish string

	// Log is the v2 structured logger. Latest stable: v2.0.0.
	Log string

	// Fang is the v2 styled cobra wrapper. Latest stable: v2.0.1.
	// Used by the --cli variant for root-cobra execution.
	Fang string

	// Viper is the optional config-file library (github.com/spf13/viper).
	// Latest stable: v1.20.1. Not a charm v2 lib, but lives on the same
	// struct so the template engine can resolve it from charmPin("viper").
	// Per CLAUDE.md, Viper is opt-in (--viper).
	Viper string
}

// LegacyCharmPins tracks libraries that have NOT migrated to charm.land/
// and remain on github.com/charmbracelet/. These are the current paths
// for those libraries, NOT v1 leaks — the CI grep suite must allow them
// (per the refined deny-list in scripts/check-v1-leaks.sh).
type LegacyCharmPins struct {
	// Harmonica is the spring animation library. Still on
	// github.com/charmbracelet/harmonica. Latest: v0.2.0.
	// v0.2.0 pre-dates the charm.land migration; the lib is mature
	// and is not expected to migrate.
	Harmonica string

	// Glow is the markdown reader CLI. The Go module path is
	// github.com/charmbracelet/glow/v2; the binary is installed via
	// `go install github.com/charmbracelet/glow/v2@latest`. This is
	// the user-facing binary dependency for the --glow flag; no Go
	// import is required in the generated project's go.mod.
	Glow string
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
	Glow:      "v2.1.2",
}
