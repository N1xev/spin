package params

import (
	"fmt"
	"strconv"

	"charm.land/huh/v2"
)

// NumberParam has no dedicated huh type; we use NewInput + a Validate that
// parses the input as int and applies Min/Max bounds. The string backing
// field (valueStr) is what huh writes into via Value(&p.valueStr); the
// numeric value is derived on demand from Value() and from SetDefaults.
type NumberParam struct {
	name     string
	prompt   string
	def      int
	min      *int
	max      *int
	value    int
	valueStr string
}

func NewNumber(name, prompt string, def int, min, max *int) *NumberParam {
	return &NumberParam{
		name:     name,
		prompt:   prompt,
		def:      def,
		min:      min,
		max:      max,
		valueStr: fmt.Sprintf("%d", def),
	}
}

func (p *NumberParam) Name() string   { return p.name }
func (p *NumberParam) Type() Type     { return TypeNumber }
func (p *NumberParam) Prompt() string { return p.prompt }
func (p *NumberParam) Default() any   { return p.def }
func (p *NumberParam) Apply(v Value)  { p.value = v.Int; p.valueStr = fmt.Sprintf("%d", v.Int) }
func (p *NumberParam) Value() Value {
	if p.valueStr != "" {
		if n, err := strconv.Atoi(p.valueStr); err == nil {
			p.value = n
		}
	}
	return Value{Kind: TypeNumber, Int: p.value}
}
func (p *NumberParam) Hmm() string { return p.String() }

func (p *NumberParam) HuhField() huh.Field {
	return huh.NewInput().
		Key(p.name).
		Title(orPrompt(p.name, p.prompt)).
		Placeholder(fmt.Sprintf("%d", p.def)).
		Value(&p.valueStr).
		Validate(func(s string) error {
			n, err := strconv.Atoi(s)
			if err != nil {
				return fmt.Errorf("not a number: %q", s)
			}
			if p.min != nil && n < *p.min {
				return fmt.Errorf("must be >= %d", *p.min)
			}
			if p.max != nil && n > *p.max {
				return fmt.Errorf("must be <= %d", *p.max)
			}
			return nil
		})
}

func (p *NumberParam) String() string {
	return fmt.Sprintf("%s (number, default %d)", p.name, p.def)
}
