package params

import (
	"fmt"

	"charm.land/huh/v2"
)

type TextParam struct {
	name   string
	prompt string
	def    string
	value  string
}

func NewText(name, prompt, def string) *TextParam {
	return &TextParam{name: name, prompt: prompt, def: def}
}

func (p *TextParam) Name() string   { return p.name }
func (p *TextParam) Type() Type     { return TypeText }
func (p *TextParam) Prompt() string { return p.prompt }
func (p *TextParam) Default() any   { return p.def }
func (p *TextParam) Apply(v Value)  { p.value = v.String }
func (p *TextParam) Value() Value   { return Value{String: p.value} }
func (p *TextParam) Hmm() string    { return p.String() }

func (p *TextParam) HuhField() huh.Field {
	f := huh.NewInput().
		Key(p.name).
		Title(orPrompt(p.name, p.prompt)).
		Value(&p.value)
	if p.def != "" {
		f = f.Placeholder(p.def)
	}
	return f
}

func (p *TextParam) String() string {
	return fmt.Sprintf("%s (text, default %q)", p.name, p.def)
}
