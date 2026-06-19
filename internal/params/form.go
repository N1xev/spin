package params

import (
	"charm.land/huh/v2"
)

// PageSize is the number of params grouped per "page" in the huh
// form. huh renders each Group as a separate page (user navigates
// with Next/Prev), so this cap is also the visible pagination
// granularity. 4 keeps the page scannable on a 24-line terminal.
const PageSize = 4

// Form builds a huh.Form from a slice of params. Params are
// grouped PageSize-at-a-time into huh.Groups; each Group becomes
// one form page the user can step through with Next/Prev. A single
// page is rendered when len(ps) <= PageSize.
func Form(ps []Param) *huh.Form {
	if len(ps) == 0 {
		return huh.NewForm()
	}
	groups := make([]*huh.Group, 0, (len(ps)+PageSize-1)/PageSize)
	for i := 0; i < len(ps); i += PageSize {
		end := min(i+PageSize, len(ps))
		fields := make([]huh.Field, 0, end-i)
		for _, p := range ps[i:end] {
			fields = append(fields, p.HuhField())
		}
		groups = append(groups, huh.NewGroup(fields...))
	}
	return huh.NewForm(groups...)
}

// Run executes the form on the given params. The form populates each
// param's value in place.
func Run(ps []Param) error {
	return Form(ps).Run()
}

// SetDefaults applies each param's Default to its current value.
// Useful when --no-interactive is set.
func SetDefaults(ps []Param) {
	for _, p := range ps {
		switch p.Type() {
		case TypeText, TypeTextarea, TypePath, TypeSecret:
			if s, ok := p.Default().(string); ok {
				p.Apply(Value{String: s})
			}
		case TypeNumber:
			if i, ok := p.Default().(int); ok {
				p.Apply(Value{Int: i})
			}
		case TypeSelect:
			if s, ok := p.Default().(string); ok {
				p.Apply(Value{String: s})
			}
		case TypeMultiSelect:
			if ss, ok := p.Default().([]string); ok {
				p.Apply(Value{List: ss})
			}
		case TypeBool:
			if b, ok := p.Default().(bool); ok {
				p.Apply(Value{Bool: b})
			}
		}
	}
}
