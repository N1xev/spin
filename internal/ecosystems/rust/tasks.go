package rust

// Tasks returns the default [tasks] block for the generated
// spin.config.toml. These are the cargo fallbacks merged into
// the runner's source-precedence chain at the language-fallback
// level (per RUN-13). Users can override any of them in their
// own spin.config.toml.
func (e *Ecosystem) Tasks() map[string]string {
	return map[string]string{
		"build":  "cargo build",
		"test":   "cargo test",
		"run":    "cargo run",
		"clippy": "cargo clippy",
		"fmt":    "cargo fmt",
	}
}
