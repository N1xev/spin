package template

import (
	"bytes"
	"fmt"
	"os/exec"
	"text/template"

	"github.com/N1xev/spin/internal/params"
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
func RunPostHook(t *Template, values map[string]any, dir string) error {
	if t == nil || t.SpinToml == nil {
		return nil
	}
	steps := t.SpinToml.Post
	if len(steps) == 0 {
		return nil
	}
	resolved := unwrapValues(values)
	for i, step := range steps {
		if step.Run == "" {
			continue
		}
		rendered, err := renderHook(step.Run, resolved)
		if err != nil {
			return fmt.Errorf("post-hook step %d: render: %w", i+1, err)
		}
		c := exec.Command("sh", "-c", rendered)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("post-hook step %d %q failed: %s: %w", i+1, rendered, string(out), err)
		}
	}
	return nil
}

// unwrapValues walks the values map and replaces any params.Value
// with its underlying primitive (String, Int, Bool, List, Path). This
// is what template engines expect: a flat key→primitive map, not
// nested struct values.
func unwrapValues(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = unwrapAny(v)
	}
	return out
}

func unwrapAny(v any) any {
	if v == nil {
		return nil
	}
	if pv, ok := v.(params.Value); ok {
		return UnwrapValue(pv)
	}
	return v
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
