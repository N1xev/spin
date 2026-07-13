package template

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
)

// RunPostHook executes the template's [[post]] steps (if any) after
// the files have been written to disk. Each step's `run` is rendered
// against the resolved param + flag values (so `{{.project_name}}`
// interpolates correctly), then run via `sh -c` in dir. Steps run
// in order; the hook stops on the first failure and returns that
// error (with the failing command and its combined output).
//
// An empty or missing post section is a no-op.
//
// The post-hook runs AFTER files are written, BEFORE the spin.toml
// is removed from the output directory. This ordering lets the hook
// observe the full scaffolded state (including any spin.toml that
// might have been included in _base/) but ensures the project that
// the user sees has spin.toml deleted by the time the scaffolder
// returns.
func RunPostHook(ctx context.Context, t *Template, values map[string]any, dir string, opts HookOptions) error {
	if t == nil || t.SpinToml == nil {
		return nil
	}
	steps := make([]hookStep, 0, len(t.SpinToml.Post))
	for _, s := range t.SpinToml.Post {
		steps = append(steps, hookStep(s))
	}
	scripts, err := autoHookScripts(dir, "_post")
	if err != nil {
		return fmt.Errorf("post-hook: list scripts: %w", err)
	}
	for _, cmd := range scripts {
		steps = append(steps, hookStep{Run: cmd})
	}
	if len(steps) == 0 {
		return nil
	}
	return runHooks(ctx, "post", steps, values, dir, opts)
}

// renderHook parses the post-hook command as a text/template and
// executes it against the resolved values. We deliberately use a
// fresh FuncMap (no template helpers like `title`/`upper`) so the
// post-hook is a thin shell wrapper, not a full templating pass.
func renderHook(cmd string, values map[string]any) (string, error) {
	t, err := template.New("post").Parse(cmd)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}
