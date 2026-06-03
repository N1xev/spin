package wrap

import (
	"os"
	"path/filepath"
)

// Build executes `spin build` in the current working directory.
//
// The output path is bin/<basename-of-cwd>, with CGO_ENABLED=0
// appended to the environment so the resulting binary is static and
// cross-compile-friendly (per the project's CLAUDE.md "No CGO"
// constraint on scaffolded projects).
//
// Build has no upgrade path beyond `go build` itself, so there is no
// fallback — we run go build directly via runTool rather than going
// through RunWithFallback. The wrapper exists to give the user a
// uniform `spin build` command and to wire the CGO_ENABLED=0 env
// automatically.
func Build() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name := filepath.Base(cwd)

	if err := os.MkdirAll("bin", 0o755); err != nil {
		return err
	}

	spec := ToolSpec{
		Name:     "go",
		Args:     []string{"build", "-o", filepath.Join("bin", name), "."},
		ExtraEnv: []string{"CGO_ENABLED=0"},
	}
	// No fallback: go build is the only path. Call runTool directly
	// so we don't print a misleading "falling back to: go" hint.
	return runTool(spec.Name, spec.Args, spec.ExtraEnv)
}
