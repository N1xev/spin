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

func init() {
	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{Level: log.InfoLevel}))
}

// New is the main scaffolder entrypoint.
//
// Steps:
//  1. Validate the Project (name regex + existing-dir check, with --force).
//  2. Call p.renderToMap() to walk the embed FS in overlay order.
//  3. Call emit(p, files) to write the files to ./<name>/.
//  4. Call p.VerifyBuild() to run `go build ./...` with CGO_ENABLED=0.
//  5. Call p.GitInit() to make the initial commit (skipped with --no-git).
func New(p *Project) error {
	if p == nil || p.Name == "" {
		return fmt.Errorf("scaffold: project name is required")
	}
	if err := p.Validate(); err != nil {
		return err
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
func emit(p *Project, files map[string][]byte) error {
	root := filepath.Join(".", p.Name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", root, err)
	}

	for rel, content := range files {
		full := filepath.Join(root, rel)
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
