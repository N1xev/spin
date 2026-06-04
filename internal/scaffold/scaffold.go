// Package scaffold implements the spin scaffolder.
//
// The Walking Skeleton shipped the minimum: New() accepts a *Project,
// renders the embedded template tree, writes files to ./<name>/,
// and runs a post-scaffold `go build ./...` smoke test with
// CGO_ENABLED=0. The overlay engine (p.renderToMap in template.go)
// composes _base → variant_<type> → lib/<name>/ in last-write-wins
// order.
package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// `all:` is required (RESEARCH §4.1) so hidden files like .air.toml
// and .gitignore are included; a `*` glob silently skips them.
//
//go:embed all:templates
var FS embed.FS

// InitLogger configures the charm/log v2 default logger (stderr,
// InfoLevel). Moved from a package-level init() so importing the
// scaffold package has no side effects (WR-002).
func InitLogger() {
	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{Level: log.InfoLevel}))
}

// New is the main scaffolder entrypoint. Caller must call
// p.Validate() BEFORE New — New does not re-validate.
//
// Steps:
//  0. Configure the default logger.
//  1. renderToMap walks the embed FS in overlay order.
//  2. emit writes the files to ./<name>/.
//  3. VerifyBuild runs `go build ./...` with CGO_ENABLED=0.
//  4. GitInit makes the initial commit (skipped with --no-git).
func New(p *Project) error {
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
	// Post-scaffold smoke test FIRST — a failing build must never
	// be committed (otherwise a broken scaffold would be the user's
	// first commit on a brand-new project).
	if err := p.VerifyBuild(); err != nil {
		return fmt.Errorf("scaffold: verify: %w", err)
	}
	if err := p.GitInit(); err != nil {
		return fmt.Errorf("scaffold: git: %w", err)
	}
	return nil
}

// emit writes the rendered files to ./<name>/ preserving relative
// paths. All files are 0644.
//
// Path-traversal guard: every rendered rel path is resolved against
// the project root and verified to remain inside it. A template that
// renders `{{.Name}}` to `../../etc/passwd` is rejected before any
// filesystem write. cleanRoot carries the trailing separator so a
// candidate path equal to cleanRoot fails the prefix check.
func emit(p *Project, files map[string][]byte) error {
	root := filepath.Join(".", p.Name)
	cleanRoot := filepath.Clean(root) + string(filepath.Separator)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", root, err)
	}

	for rel, content := range files {
		full := filepath.Join(root, rel)
		cleanFull := filepath.Clean(full)
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
