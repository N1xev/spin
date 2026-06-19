package params

import (
	"fmt"

	"charm.land/huh/v2"
)

type BoolParam struct {
	name   string
	prompt string
	def    bool
	value  bool
}

func NewBool(name, prompt string, def bool) *BoolParam {
	return &BoolParam{name: name, prompt: prompt, def: def}
}

func (p *BoolParam) Name() string   { return p.name }
func (p *BoolParam) Type() Type     { return TypeBool }
func (p *BoolParam) Prompt() string { return p.prompt }
func (p *BoolParam) Default() any   { return p.def }
func (p *BoolParam) Apply(v Value)  { p.value = v.Bool }
func (p *BoolParam) Value() Value   { return Value{Bool: p.value} }
func (p *BoolParam) Hmm() string    { return p.String() }

func (p *BoolParam) HuhField() huh.Field {
	return huh.NewConfirm().
		Key(p.name).
		Title(orPrompt(p.name, p.prompt)).
		Affirmative("Yes").
		Negative("No").
		Value(&p.value)
}

func (p *BoolParam) String() string {
	return fmt.Sprintf("%s (bool, default %t)", p.name, p.def)
}
