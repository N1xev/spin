package wrap

import (
	"os"
)

// Run executes `spin run` in the current working directory.
//
// If a .air.toml file is present, Run prefers the `air` hot-reload
// tool (via RunWithFallback — if air is missing, it falls back to
// `go run .` and prints the install hint). If .air.toml is absent,
// there is nothing for air to do, so we run `go run .` directly
// without going through the fallback dance.
//
// The .air.toml stat check is the only unique logic in this wrapper;
// everything else is composed via ToolSpec.
func Run() error {
	if _, err := os.Stat(".air.toml"); err != nil {
		// No .air.toml: air would have nothing to drive. Run go directly.
		return runTool("go", []string{"run", "."}, nil)
	}
	spec := ToolSpec{
		Name:        "air",
		Args:        []string{},
		InstallHint: "go install github.com/air-verse/air@latest",
	}
	fallback := ToolSpec{
		Name: "go",
		Args: []string{"run", "."},
	}
	return RunWithFallback(spec, fallback)
}
