// Package scaffold: template-FS abstraction.
//
// The template walker (template.go renderToMap) historically read from a
// package-level embed.FS (FS in scaffold.go). Plan 02-02 needs the walker
// to transparently switch between the embedded tree and an external
// directory cloned from a user-supplied git repo (--template-repo).
//
// templateFS is the minimum-interface subset that both embed.FS (since
// Go 1.16) and os.DirFS results satisfy:
//
//   - Open(name) -> (fs.File, error)         // inherited via fs.FS
//   - ReadDir(name) -> ([]fs.DirEntry, error) // fs.ReadDirFS
//   - ReadFile(name) -> ([]byte, error)      // fs.ReadFileFS
//
// Why an explicit interface? Go's type system cannot statically prove
// that os.DirFS's return value (an unexported *os.dirFS) satisfies an
// interface that contains those three methods, because the methods
// are inherited from stdlib interfaces. The currentFS helper does a
// type assertion to templateFS at runtime; if a future Go release ever
// drops one of the three methods, the assertion fails the build here
// (panic) instead of surfacing as a confusing nil-method panic deep
// inside fs.WalkDir.
package scaffold

import (
	"io/fs"
	"os"
)

// templateFS is the minimum-method subset required by the template
// walker. See package comment for the rationale.
type templateFS interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
}

// currentFS returns the FS the walker should read from. When externalDir
// is empty, returns the package-level embedded FS (FS in scaffold.go).
// When externalDir is set, returns os.DirFS(externalDir) (an on-disk
// directory the caller has cloned from a git repo).
//
// The type assertion to templateFS is a compile-time guard: if
// os.DirFS's result ever stops satisfying one of the three interface
// methods, the panic at startup surfaces the problem here rather than
// as a nil-method call deep in fs.WalkDir.
func currentFS(externalDir string) templateFS {
	if externalDir == "" {
		// FS is declared in scaffold.go as `//go:embed all:templates`.
		// Its concrete type is embed.FS, which satisfies templateFS
		// (and has since Go 1.16) — but embed.FS is a struct, not an
		// interface, so the conversion to templateFS is a static check
		// enforced at compile time.
		return any(FS).(templateFS)
	}
	// os.DirFS returns an unexported *os.dirFS; we type-assert to
	// templateFS so the interface check fires here, not inside the
	// walker.
	return os.DirFS(externalDir).(templateFS)
}
