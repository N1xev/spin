// Package scaffold renders and writes a new Go project tree.
package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// FS holds the embedded template tree. The `all:` prefix is required
// so hidden files like .air.toml and .gitignore are included.
//
//go:embed all:templates
var FS embed.FS

// InitLogger configures the default charm/log v2 logger. It is
// exported because the package uses init() in the past, which is no
// longer desired (so importing scaffold has no side effects).
func InitLogger() {
	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{Level: log.InfoLevel}))
}

// New scaffolds the project described by p into ./p.Name/, runs the
// post-scaffold build verification, and (unless --no-git) makes the
// initial commit. The caller must call p.Validate() before New.
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
	// Build first so a broken scaffold never lands as the user's
	// first commit on a brand-new project.
	if err := p.VerifyBuild(); err != nil {
		return fmt.Errorf("scaffold: verify: %w", err)
	}
	if err := p.GitInit(); err != nil {
		return fmt.Errorf("scaffold: git: %w", err)
	}
	return nil
}

// emit writes the rendered files to ./p.Name/, refusing any rel path
// that resolves outside the project root. cleanRoot carries a trailing
// separator so a candidate equal to cleanRoot fails the prefix check.
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
