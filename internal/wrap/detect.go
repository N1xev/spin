// Package wrap contains the spin CLI's "wrapper" subcommands: thin shims
// that wrap the Go toolchain (and friends) with a uniform `spin run` /
// `spin build` / `spin test` / `spin vet` / `spin fmt` surface. Every
// wrapper follows the same pattern:
//
//  1. Look up the preferred tool on $PATH via exec.LookPath.
//  2. If present, exec it with the right args/env.
//  3. If missing, print a one-line "hint: install with: ..." message
//     to stderr and fall back to a stock Go equivalent.
//
// The single helper for that pattern is ToolSpec + RunWithFallback
// (this file). Each wrapper (run.go, build.go, test.go, vet.go, fmt.go)
// composes a ToolSpec and calls RunWithFallback. The runTool function
// is unexported because it's an implementation detail — wrappers never
// call it directly; they always go through RunWithFallback (or, in
// fmt.go's case, LookPath+runTool directly for chained fallbacks).
//
// Anti-patterns deliberately NOT followed:
//   - We do NOT block on user input. If a tool is missing, we fall
//     back to stock Go; auto-install would be too magical for v1.
//   - We do NOT call os.Exit from inside this package. Errors are
//     returned to the cobra subcommand (cmd/run.go, etc.), which
//     fang.Execute turns into a styled error and exit code.
package wrap

import (
	"fmt"
	"os"
	"os/exec"
)

// ToolSpec describes one tool to run. Compose two ToolSpecs (preferred
// + fallback) and pass them to RunWithFallback.
//
// Name is the bare command name (e.g. "air", "prism", "gofumpt") and
// is resolved via exec.LookPath against $PATH. Args is the argv
// (excluding the program name). ExtraEnv is appended to the inherited
// environment — e.g. {"CGO_ENABLED=0"} to force a static build.
// InstallHint is the "go install ..." string printed when Name is
// missing; empty string suppresses the hint.
type ToolSpec struct {
	Name        string
	Args        []string
	ExtraEnv    []string
	InstallHint string
}

// RunWithFallback looks up spec.Name on $PATH. If found, it runs spec
// directly. If not, it prints a one-line install hint to stderr and
// runs the fallback. Returns whatever exec.Cmd.Run returns from the
// chosen tool — non-nil on a non-zero exit code (the cobra RunE
// propagates that to fang as an error and exits non-zero).
//
// The function is the single helper for all 5 wrappers (run, build,
// test, vet, fmt). New wrappers should compose a ToolSpec and call
// this; the unexported runTool is not part of the public API.
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
// process. The user sees the wrapped tool's output, not a silent
// wrapper. ExtraEnv (if non-empty) is appended to os.Environ() so the
// inherited env is preserved (PATH, HOME, GOPATH, etc. all survive).
//
// This is unexported on purpose: it is the only place we touch
// exec.Cmd, and exposing it would let callers bypass the
// LookPath-then-run pattern that all wrappers share.
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
