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
		echo := func(line string) {
			if opts.Output != nil {
				fmt.Fprintln(opts.Output, line)
			} else if opts.PrintCommands {
				log.Stdout.Print(line)
			}
		}
		echo(fmt.Sprintf("→ %s-hook: %s", kind, rendered))
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
				return fmt.Errorf("%s-hook step %d %q failed: %w", kind, i+1, rendered, err)
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
// `sh <dirName>/<name>`.
func scriptCommand(fullDir, dirName, name string) (string, error) {
	info, err := os.Stat(filepath.Join(fullDir, name))
	if err != nil {
		return "", err
	}
	scriptPath := filepath.Join(dirName, name)
	if info.Mode()&0o111 != 0 {
		return "./" + scriptPath, nil
	}
	return "sh " + scriptPath, nil
}

// hookAssetDir returns the template's _pre or _post script directory
// for the given phase.
func hookAssetDir(t *Template, phase string) string {
	if phase == "post" {
		return t.PostHookDir
	}
	return t.PreHookDir
}

// RunSingleHook executes one hook entry (an inline [[pre]]/[[post]]
// command or a _pre/_post script file) in isolation, streaming its
// echoed command and combined output through opts.Output. It is used by
// the interactive TUI to preview a single hook: selecting a hook and
// running it shows exactly the command that would run and its output in
// the review pane. The relevant hook-asset directory is copied into dest
// first so script files can be found, but the project's _base files are
// not rendered (that only happens on a full run).
func RunSingleHook(ctx context.Context, t *Template, values map[string]any, dest string, h HookView, opts HookOptions) error {
	if t == nil || t.SpinToml == nil {
		return nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dest, err)
	}
	var step hookStep
	switch {
	case h.IsFile:
		if err := copyHookAssets(ctx, hookAssetDir(t, h.Phase), filepath.Join(dest, "_"+h.Phase)); err != nil {
			return err
		}
		cmd, err := scriptCommand(hookAssetDir(t, h.Phase), "_"+h.Phase, filepath.Base(h.File))
		if err != nil {
			return err
		}
		step = hookStep{Run: cmd}
	default:
		step = hookStep{Run: h.Run}
	}
	if opts.NoHooks || step.Run == "" {
		return nil
	}
	return runHooks(ctx, h.Phase, []hookStep{step}, values, dest, opts)
}
