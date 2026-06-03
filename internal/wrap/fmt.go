package wrap

import (
	"fmt"
	"os"
	"os/exec"
)

// Fmt executes `spin fmt` in the current working directory.
//
// The chain is: gofumpt → goimports → gofmt. The order matters:
//
//   - gofumpt is a stricter gofmt, so running it first sets the
//     "what is canonical" baseline.
//   - goimports adds missing imports and removes unused ones —
//     neither gofumpt nor gofmt does this.
//   - gofmt is the final idempotent pass (gofumpt's output is
//     already gofmt-clean, so this is a no-op when gofumpt ran).
//
// When noStrict is true, a missing gofumpt is a warning + fall
// through to goimports; otherwise it is a hard error so the user
// notices they are missing the strict formatter (per
// CLAUDE.md: gofumpt is the primary formatter, goimports and gofmt
// are the fallbacks).
//
// Fmt does not use RunWithFallback because the three tools form a
// chain, not a preferred/fallback pair.
func Fmt(noStrict bool) error {
	gofumptPath, gofumptErr := exec.LookPath("gofumpt")
	if gofumptErr == nil {
		if err := runTool(gofumptPath, []string{"-l", "-w", "."}, nil); err != nil {
			return err
		}
	} else {
		if !noStrict {
			return fmt.Errorf("gofumpt not found on $PATH; install with: go install mvdan.cc/gofumpt@latest (or pass --no-strict): %w", gofumptErr)
		}
		fmt.Fprintln(os.Stderr,
			"warn: gofumpt not found on $PATH; falling back to goimports + gofmt (--no-strict)")
	}

	goimportsPath, goimportsErr := exec.LookPath("goimports")
	if goimportsErr == nil {
		if err := runTool(goimportsPath, []string{"-w", "."}, nil); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stderr,
			"warn: goimports not found on $PATH; skipping (install with: go install golang.org/x/tools/cmd/goimports@latest)")
	}

	// gofmt is part of the Go toolchain itself; if it's missing, the
	// toolchain isn't usable, and runTool will surface that as a
	// non-zero exit (which fang will style as an error).
	return runTool("gofmt", []string{"-l", "-w", "."}, nil)
}
