// Package version exposes the spin version string.
//
// Version is set at build time via -ldflags. The default is "0.1.0" for
// dev builds. Example:
//
//	go build -ldflags="-X github.com/example/spin/internal/version.Version=1.2.3" .
//
// cobra/fang read Version directly as a string and pass it to their
// themed --version output. There's no need for a function form here; the
// var alone is the single source of truth.
package version

// Version is the current spin release. Override via -ldflags at build time.
var Version = "0.1.0"
