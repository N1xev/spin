// Package scaffold: pinned library versions for the generated go.mod.
//
// CharmPins is for libraries on charm.land/<lib>/v2; LegacyCharmPins
// is for libraries that have not migrated and remain on
// github.com/charmbracelet/. Versions are exact (not @latest) for
// reproducible builds.
package scaffold

// CharmPins holds the verified charmbracelet v2 version pins.
type CharmPins struct {
	Bubbletea string
	Lipgloss  string
	// Bubbles requires Go 1.25.0+.
	Bubbles string
	Huh       string
	Glamour   string
	Wish      string
	Log       string
	Fang      string
	Viper     string
}

// LegacyCharmPins holds pins for libraries still on github.com/charmbracelet/.
type LegacyCharmPins struct {
	Harmonica string
}

// DefaultPins is the package-level pin set used by the template engine.
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

// DefaultLegacyPins is the package-level pin set for github.com/charmbracelet/ libs.
var DefaultLegacyPins = LegacyCharmPins{
	Harmonica: "v0.2.0",
}
