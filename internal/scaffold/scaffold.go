// Package scaffold implements the spin scaffolder.
//
// The Walking Skeleton (Task 1 / Task 2) ships the minimum: New() accepts a
// *Project, renders the embedded template tree, writes files to ./<name>/,
// and runs a post-scaffold `go build ./...` smoke test with CGO_ENABLED=0.
//
// Plan 03 expands this with the proper overlay engine (overlayOrder),
// FuncMap helpers, and the full lib overlay set.
package scaffold

import (
	"embed"
	"fmt"
	"os"
)

// FS is the embedded template tree rooted at templates/.
//
// The all: prefix is required (RESEARCH §4.1) so that hidden files like
// .air.toml and .gitignore are included in the embed — a `*` glob would
// silently skip them.
var FS embed.FS

// New is the main scaffolder entrypoint.
//
// Walking Skeleton (Task 1): accepts a *Project and returns a "not yet
// implemented" error. Task 2 replaces this with the full render/emit/verify
// pipeline. Until then, this stub exists so cmd/new.go can call it and the
// binary builds end-to-end — the whole point of the Walking Skeleton is to
// prove the CLI plumbing works before the scaffolder logic lands.
func New(p *Project) error {
	if p == nil || p.Name == "" {
		return fmt.Errorf("scaffold: project name is required")
	}
	if _, err := os.Stat(p.Name); err == nil {
		return fmt.Errorf("scaffold: directory %q already exists", p.Name)
	}
	return fmt.Errorf("scaffold: full implementation lands in Task 2 (Project=%+v)", p)
}
