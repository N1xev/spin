// Package scaffold implements the spin scaffolder.
//
// The Walking Skeleton (Task 1 / Task 2) shipped the minimum: New() accepts
// a *Project, renders the embedded template tree, writes files to ./<name>/,
// and runs a post-scaffold `go build ./...` smoke test with CGO_ENABLED=0.
//
// Plan 03 expands this with the proper overlay engine (p.renderToMap in
// template.go), the FuncMap helpers, license gating, and the full lib
// overlay set. The Walking Skeleton's hardcoded free-function renderToMap
// is now a *Project method; the scaffolding pipeline is otherwise the same.
package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// FS is the embedded template tree rooted at templates/.
//
// The all: prefix is required (RESEARCH §4.1) so that hidden files like
// .air.toml and .gitignore are included in the embed — a `*` glob would
// silently skip them.
//
//go:embed all:templates
var FS embed.FS

// InitLogger configures the charm/log v2 default logger for scaffolder
// output: writes to stderr at InfoLevel. Callers may invoke this
// directly if they want scaffold-style logging without going through
// New (e.g. from a custom entrypoint). New itself calls InitLogger on
// entry, so most callers don't need to. WR-002 moved this from a
// package-level init() so importing the scaffold package has no
// side effects.
func InitLogger() {
	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{Level: log.InfoLevel}))
}

// New is the main scaffolder entrypoint.
//
// Contract: the caller is responsible for calling p.Validate() BEFORE
// calling New. New does not re-validate because runNew does the
// fail-fast check before any FS write, and the only other direct
// caller (scaffold_test.go's TestNewEndToEndWalkingSkeleton) builds
// its Project struct manually with valid fields. WR-003 removed the
// duplicate call.
//
// Steps:
//  0. Configure the default logger (stderr, InfoLevel). This is the
//     first thing New() does so importing this package has no side
//     effects — WR-002 moved this out of a package-level init() so
//     tests that import scaffold do not silently override the global
//     log default.
//  1. Call p.renderToMap() to walk the embed FS in overlay order.
//  2. Call emit(p, files) to write the files to ./<name>/.
//  3. Call p.VerifyBuild() to run `go build ./...` with CGO_ENABLED=0.
//  4. Call p.GitInit() to make the initial commit (skipped with --no-git).
func New(p *Project) error {
	// Configure the default logger on entry, not at package init. This
	// keeps the scaffold package side-effect-free on import. Tests and
	// other tools that import scaffold retain their own log default
	// unless they call New (or InitLogger explicitly).
	InitLogger()

	if p == nil || p.Name == "" {
		return fmt.Errorf("scaffold: project name is required")
	}

	files, err := p.renderToMap()
	if err != nil {
		return fmt.Errorf("scaffold: render: %w", err)
	}

	if err := emit(p, files); err != nil {
		return fmt.Errorf("scaffold: emit: %w", err)
	}

	// Post-scaffold smoke test FIRST. A failing build must never be
	// committed to git (otherwise a broken scaffold would be the user's
	// first commit on a brand-new project).
	if err := p.VerifyBuild(); err != nil {
		return fmt.Errorf("scaffold: verify: %w", err)
	}

	// Git init + initial commit AFTER verify. Skip on --no-git.
	if err := p.GitInit(); err != nil {
		return fmt.Errorf("scaffold: git: %w", err)
	}

	return nil
}

// emit writes the rendered files to ./<name>/ preserving relative paths.
// All files are written with 0644 perms. Plan 02 may add +x for shell
// scripts in Taskfile hooks.
//
// Path-traversal guard: every rendered relative path is resolved against
// the project root and verified to remain inside it. A template that
// renders `{{.Name}}` to `../../etc/passwd` (or any other escape) is
// rejected with a descriptive error before any filesystem write happens.
// The check uses filepath.Clean and a string-prefix comparison against
// the cleaned root, both computed with filepath.Separator so it is
// portable across POSIX and Windows.
func emit(p *Project, files map[string][]byte) error {
	root := filepath.Join(".", p.Name)
	cleanRoot := filepath.Clean(root) + string(filepath.Separator)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", root, err)
	}

	for rel, content := range files {
		full := filepath.Join(root, rel)
		// Defense-in-depth: a malicious or buggy template that renders a
		// relative path with `..` segments must not be allowed to write
		// outside the project root. filepath.Clean collapses the path
		// so the prefix check is unambiguous.
		cleanFull := filepath.Clean(full)
		// cleanRoot carries the trailing separator, so a candidate path
		// equal to cleanRoot fails the prefix check (cleanFull+sep has
		// an extra sep suffix) and is correctly rejected. No second
		// clause needed.
		if !strings.HasPrefix(cleanFull+string(filepath.Separator), cleanRoot) {
			return fmt.Errorf(
				"path traversal: rendered %q resolves to %q which is outside project root %q",
				rel, cleanFull, cleanRoot,
			)
		}
		dir := filepath.Dir(full)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", dir, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", full, err)
		}
	}
	return nil
}
