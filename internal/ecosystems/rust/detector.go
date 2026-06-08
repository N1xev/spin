package rust

import (
	"os"
	"path/filepath"
)

// Detector identifies a Rust project by the presence of a Cargo.toml
// in the project root. We do the stat inline (rather than using
// ecosystem.FileDetector) to keep the import surface minimal.
func (e *Ecosystem) Matches(dir string) bool {
	if dir == "" {
		dir = "."
	}
	_, err := os.Stat(filepath.Join(dir, "Cargo.toml"))
	return err == nil
}

// FriendlyName is the human name shown to the user.
func (e *Ecosystem) FriendlyName() string { return "Rust (Cargo)" }
