package template

import (
	"fmt"

	"charm.land/huh/v2"

	"github.com/N1xev/spin/internal/params"
)

// BuildForm constructs a huh.Form from the template's spin.toml params.
// The user fills the form; the resolved values are written back into
// the supplied map.
func (t *Template) BuildForm(values map[string]any) (*huh.Form, error) {
	ps, err := params.Parse(t.SpinToml.Params)
	if err != nil {
		return nil, err
	}
	// Pre-fill with the supplied values, so the user sees existing defaults
	// (e.g. when re-running with --no-interactive).
	for _, p := range ps {
		if v, ok := values[p.Name()]; ok {
			p.Apply(toParamValue(v))
		}
	}
	form := params.Form(ps)
	// After Run, walk the params and copy their Values into values.
	// We attach a callback by using huh's Key() -- but huh doesn't have
	// a generic "post-run" hook. So we expose a separate helper.
	return form, nil
}

// ResolveForm runs the form (or applies defaults in non-interactive
// mode) and returns the resolved values ready for Render().
//
// Returned values are unwrapped to raw Go primitives (string, int,
// bool, []string) so text/template rendering produces sensible
// output (e.g. `{{.project_name}}` interpolates as the name, not
// the params.Value struct dump).
//
// Order of operations is significant: defaults are applied first,
// THEN any caller-supplied values are layered on top. This ensures
// explicit values from the CLI or pre-apply map win over the
// template's own defaults.
func (t *Template) ResolveForm(values map[string]any, interactive bool) (map[string]any, error) {
	ps, err := params.Parse(t.SpinToml.Params)
	if err != nil {
		return nil, err
	}
	if !interactive {
		params.SetDefaults(ps)
	} else {
		if err := params.Run(ps); err != nil {
			return nil, err
		}
	}
	// Apply caller-supplied values AFTER defaults so explicit
	// overrides win.
	for _, p := range ps {
		if v, ok := values[p.Name()]; ok {
			p.Apply(toParamValue(v))
		}
	}
	out := map[string]any{}
	for _, p := range ps {
		// Unwrap params.Value to its underlying primitive so
		// text/template sees {{.project_name}} as a string, not
		// the Value struct dump.
		out[p.Name()] = unwrapValue(p.Value())
	}
	// also copy through any caller-supplied keys that aren't params
	for k, v := range values {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out, nil
}

func toParamValue(v any) params.Value {
	switch x := v.(type) {
	case string:
		return params.Value{String: x}
	case int:
		return params.Value{Int: x}
	case bool:
		return params.Value{Bool: x}
	case []string:
		return params.Value{List: x}
	}
	return params.Value{}
}

// UnwrapValue returns the underlying primitive held by a
// params.Value. The text/template engine wants raw Go types
// (string, int, bool, []string), not the multi-field struct.
// Exported because post_hook.go also needs it.
func UnwrapValue(v params.Value) any {
	switch {
	case v.String != "":
		return v.String
	case v.Int != 0:
		return v.Int
	case v.Bool:
		return true
	case v.Path != "":
		return v.Path
	case len(v.List) > 0:
		return v.List
	}
	return ""
}

func unwrapValue(v params.Value) any { return UnwrapValue(v) }

// Hints returns a one-line-per-param summary, used by
// `spin new <template> --print-params` and the template README.
func (t *Template) Hints() []string {
	out := []string{}
	for name, spec := range t.SpinToml.Params {
		out = append(out, fmt.Sprintf("  %-20s %s", name, spec.Type))
	}
	return out
}
