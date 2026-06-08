// Package rust is the built-in "rust" ecosystem: Cargo-based Rust projects
// (binary, library, or example). It registers alongside the charm ecosystem
// and provides cargo fallback tasks for `spin run`.
package rust

import "github.com/example/spin/internal/ecosystem"

// Ecosystem is the exported singleton. Internal/cmd imports it via
// New() so that the registry can take an interface.
type Ecosystem struct{}

func New() *Ecosystem { return &Ecosystem{} }

func (e *Ecosystem) Name() string { return "rust" }
func (e *Ecosystem) Description() string {
	return "Cargo-based Rust projects (binary, library, or example). Spawns `spin run` cargo fallbacks out of the box."
}
func (e *Ecosystem) Version() string { return "2.0.0" }

func (e *Ecosystem) Flags() []ecosystem.Flag { return Flags() }

// Render, PostScaffold, and Tasks are implemented in render.go / post.go /
// tasks.go (Task 2). The method bodies declared here are placeholders that
// keep `go build` green; Task 2 will overwrite them by defining Render,
// PostScaffold, and Tasks in their own files.
func (e *Ecosystem) Render(ctx ecosystem.Context) (map[string][]byte, error) {
	return nil, nil
}
func (e *Ecosystem) PostScaffold(ctx ecosystem.Context, dir string) error {
	return nil
}
func (e *Ecosystem) Tasks() map[string]string { return nil }

// Compile-time check that *Ecosystem satisfies the interface.
var _ ecosystem.Ecosystem = (*Ecosystem)(nil)
