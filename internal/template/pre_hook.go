package template

import (
	"context"
	"fmt"
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
			log.Stdout.Print(fmt.Sprintf("→ %s-hook: %s", kind, rendered))
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		c := exec.CommandContext(ctx, "sh", "-c", rendered)
		c.Dir = dir
		if opts.Verbose {
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
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
		scriptPath := filepath.Join(dirName, e.Name())
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode()&0o111 != 0 {
			scripts = append(scripts, "./"+scriptPath)
		} else {
			scripts = append(scripts, "sh "+scriptPath)
		}
	}
	slices.Sort(scripts)
	return scripts, nil
}
