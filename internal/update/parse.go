// Package update implements the engine of `spin update`: a universal
// Go dependency updater. It reads any project's go.mod, resolves the
// latest available versions from proxy.golang.org, and applies the
// chosen upgrades with `go get` + `go mod tidy` + `CGO=0 go build`.
//
// Plan 04-04 will wire this engine to a huh v2 form and a cobra
// subcommand; this package is the engine only.
package update

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/mod/modfile"
)

// Dep is one entry in a go.mod require block plus the upgrade
// candidates that resolve.go attaches to it.
//
// Module and Old come straight from go.mod. NewStable and NewLatest
// are filled in by Resolver.Resolve; a caller that skips Resolve (or
// that hit a 404) leaves them equal to Old. Target is the version
// the user has chosen to apply — Plan 04-04 sets it before calling
// Apply, after running the huh form.
type Dep struct {
	Module     string
	Old        string
	NewStable  string
	NewLatest  string
	Target     string
	Indirect   bool
}

// ListDeps reads gomodPath and returns every require, sorted
// alphabetically by Module path. When includeIndirect is false,
// entries marked with a `// indirect` comment are filtered out (per
// CONTEXT D-07: direct deps by default, --all in Plan 04-04 widens
// the set).
//
// A missing file is reported via an error that wraps os.ErrNotExist
// so callers can branch with errors.Is(err, os.ErrNotExist).
func ListDeps(gomodPath string, includeIndirect bool) ([]Dep, error) {
	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return nil, fmt.Errorf("update: %s: %w", gomodPath, err)
	}

	f, err := modfile.Parse(gomodPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("update: parse %s: %w", gomodPath, err)
	}

	deps := make([]Dep, 0, len(f.Require))
	for _, r := range f.Require {
		if !includeIndirect && r.Indirect {
			continue
		}
		deps = append(deps, Dep{
			Module:   r.Mod.Path,
			Old:      r.Mod.Version,
			Indirect: r.Indirect,
		})
	}

	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Module < deps[j].Module
	})

	return deps, nil
}

// FindGoMod walks up from startDir until it finds a go.mod, returning
// its absolute path. If the search reaches the filesystem root
// without finding one, an error is returned. The walk is bounded by
// the parent == self check so a misnamed root cannot loop forever.
func FindGoMod(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("update: resolve %s: %w", startDir, err)
	}

	for {
		candidate := filepath.Join(dir, "go.mod")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("update: no go.mod found in %s or any parent directory", startDir)
		}
		dir = parent
	}
}
