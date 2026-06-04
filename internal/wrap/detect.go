// Package wrap contains spin's "wrapper" subcommands: thin shims that
// wrap the Go toolchain (and friends) with a uniform `spin run` /
// `spin build` / `spin test` / `spin vet` / `spin fmt` surface.
//
// Each wrapper follows the same pattern: LookPath the preferred tool
// on $PATH; if present, exec it; if missing, print a "hint: install
// with: ..." message and fall back to a stock Go equivalent. The
// single helper is RunWithFallback below.
//
// We do NOT block on user input (auto-install is too magical for v1)
// and do NOT call os.Exit (errors propagate to the cobra subcommand,
// which fang turns into a styled error + non-zero exit).
package wrap

import (
	"fmt"
	"os"
	"os/exec"
)

// ToolSpec describes one tool to run. Compose two ToolSpecs (preferred
// + fallback) and pass them to RunWithFallback.
type ToolSpec struct {
	// Name is the bare command name (e.g. "air", "prism", "gofumpt"),
	// resolved via exec.LookPath against $PATH.
	Name string
	// Args is the argv (excluding the program name).
	Args []string
	// ExtraEnv is appended to the inherited environment — e.g.
	// {"CGO_ENABLED=0"} to force a static build.
	ExtraEnv []string
	// InstallHint is the "go install ..." string printed when Name is
	// missing; empty string suppresses the hint.
	InstallHint string
}

// RunWithFallback looks up spec.Name on $PATH. If found, runs it
// directly. If not, prints a one-line install hint to stderr and runs
// the fallback. Returns whatever exec.Cmd.Run returns from the chosen
// tool (the cobra RunE propagates that to fang as an error).
//
// The unexported runTool is not part of the public API; new wrappers
// should always go through this function.
func RunWithFallback(spec, fallback ToolSpec) error {
	if path, err := exec.LookPath(spec.Name); err == nil {
		return runTool(path, spec.Args, spec.ExtraEnv)
	}
	if spec.InstallHint != "" {
		fmt.Fprintf(os.Stderr,
			"hint: %s not found on $PATH; install with: %s\nfalling back to: %s\n",
			spec.Name, spec.InstallHint, fallback.Name)
	}
	return runTool(fallback.Name, fallback.Args, fallback.ExtraEnv)
}

// runTool execs a tool with stdin/stdout/stderr wired to the parent
// process so the user sees the wrapped tool's output, not a silent
// wrapper. ExtraEnv is appended to os.Environ() so the inherited env
// is preserved (PATH, HOME, GOPATH all survive).
//
// Unexported on purpose: it is the only place we touch exec.Cmd, and
// exposing it would let callers bypass the LookPath-then-run pattern
// all wrappers share.
func runTool(name string, args, extraEnv []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	return cmd.Run()
}
