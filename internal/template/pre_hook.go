package template

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/N1xev/spin/internal/log"
	"github.com/N1xev/spin/internal/params"
)

// RunPreHook executes the template's [[pre]] steps (if any) after
// params are resolved but before files are rendered. Each step's `run`
// is rendered against the resolved param + flag values, then run via
// `sh -c` in dir. Steps run in order; the hook stops on the first
// failure and returns that error.
//
// An empty or missing pre section is a no-op.
func RunPreHook(ctx context.Context, t *Template, values map[string]any, dir string, opts HookOptions) error {
	if t == nil || t.SpinToml == nil {
		return nil
	}
	steps := make([]hookStep, 0, len(t.SpinToml.Pre))
	for _, s := range t.SpinToml.Pre {
		steps = append(steps, hookStep(s))
	}
	scripts, err := autoHookScripts(dir, "_pre")
	if err != nil {
		return fmt.Errorf("pre-hook: list scripts: %w", err)
	}
	for _, cmd := range scripts {
		steps = append(steps, hookStep{Run: cmd})
	}
	if len(steps) == 0 {
		return nil
	}
	return runHooks(ctx, "pre", steps, values, dir, opts)
}

// HasHooks reports whether the template would run any shell commands:
// [[pre]]/[[post]] steps, or non-hidden files in _pre/ or _post/.
func HasHooks(t *Template) bool {
	if t == nil || t.SpinToml == nil {
		return false
	}
	if len(t.SpinToml.Pre) > 0 || len(t.SpinToml.Post) > 0 {
		return true
	}
	for _, dir := range []string{t.PreHookDir, t.PostHookDir} {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				return true
			}
		}
	}
	return false
}

// HookOptions controls how hooks are reported and whether they run.
type HookOptions struct {
	// NoHooks skips execution entirely. Commands are still printed if
	// PrintCommands is true.
	NoHooks bool
	// PrintCommands prints each rendered command before running it.
	PrintCommands bool
	// Verbose streams hook output to the caller. When false, output is
	// captured and only returned on failure.
	Verbose bool
	// Output, when set, receives the echoed command lines and, when
	// Verbose is true, the live command output of each step. It is used
	// by the interactive TUI to stream hook execution into a viewport.
	// When Output is nil, PrintCommands falls back to the package logger.
	Output io.Writer
	// StepStart, when set, is called before each step runs so the caller
	// can print a styled header. Only used when PrintCommands is true;
	// falls back to a plain log line otherwise.
	StepStart func(kind, cmd string)
}

// hookStep is the common shape of PreStep and PostStep.
type hookStep struct {
	Run string
}

func runHooks(ctx context.Context, kind string, steps []hookStep, values map[string]any, dir string, opts HookOptions) error {
	resolved := make(map[string]any, len(values))
	for k, v := range values {
		if pv, ok := v.(params.Value); ok {
			resolved[k] = UnwrapValue(pv)
		} else {
			resolved[k] = v
		}
	}
	for i, step := range steps {
		if step.Run == "" {
			continue
		}
		rendered, err := renderHook(step.Run, resolved)
		if err != nil {
			return fmt.Errorf("%s-hook step %d: render: %w", kind, i+1, err)
		}
		if opts.NoHooks {
			continue
		}
		if opts.PrintCommands {
			if opts.StepStart != nil {
				opts.StepStart(kind, rendered)
			} else if opts.Output != nil {
				fmt.Fprintf(opts.Output, "→ %s-hook: %s\n", kind, rendered)
			} else {
				log.Stdout.Print(fmt.Sprintf("→ %s-hook: %s", kind, rendered))
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		c := exec.CommandContext(ctx, "sh", "-c", rendered)
		c.Dir = dir
		if opts.Verbose {
			if opts.Output != nil {
				c.Stdout = opts.Output
				c.Stderr = opts.Output
			} else {
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
			}
			if err := c.Run(); err != nil {
				flushWriter(opts.Output) // best-effort; run error takes priority
				return fmt.Errorf("%s-hook step %d %q failed: %w", kind, i+1, rendered, err)
			}
			if err := flushWriter(opts.Output); err != nil {
				return fmt.Errorf("%s-hook step %d %q: flush: %w", kind, i+1, rendered, err)
			}
			continue
		}
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s-hook step %d %q failed: %s: %w", kind, i+1, rendered, string(out), err)
		}
	}
	return nil
}

// flusher is implemented by callers that buffer streamed hook output and
// need to emit a trailing partial line once a command finishes.
type flusher interface{ Flush() error }

// flushWriter flushes w if it buffers output (e.g. cmd's tree writer).
// Writers that stream directly (os.Stdout) have no Flush and are ignored.
func flushWriter(w io.Writer) error {
	if f, ok := w.(flusher); ok {
		return f.Flush()
	}
	return nil
}

// autoHookScripts returns shell commands for every file in
// dir/<dirName>/, sorted alphabetically. Hidden files and subdirectories
// are skipped. Executable files are run as ./<dirName>/<file>; otherwise
// they are run with `sh`. The directory itself is allowed to be missing.
func autoHookScripts(dir, dirName string) ([]string, error) {
	fullDir := filepath.Join(dir, dirName)
	entries, err := os.ReadDir(fullDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	scripts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		cmd, err := scriptCommand(fullDir, dirName, e.Name())
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, cmd)
	}
	slices.Sort(scripts)
	return scripts, nil
}

// scriptCommand returns the shell command used to run a single hook
// script file: ./<dirName>/<name> when the file is executable, otherwise
// `sh <dirName>/<name>`. The path component is single-quoted so filenames
// with spaces or shell metacharacters are safe.
func scriptCommand(fullDir, dirName, name string) (string, error) {
	info, err := os.Stat(filepath.Join(fullDir, name))
	if err != nil {
		return "", err
	}
	scriptPath := filepath.Join(dirName, name)
	scriptPath = shellQuote(scriptPath)
	if info.Mode()&0o111 != 0 {
		return "./" + scriptPath, nil
	}
	return "sh " + scriptPath, nil
}

// shellQuote wraps s in single quotes, escaping embedded single quotes
// with the '"'"' trick so the result is safe for sh -c.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
