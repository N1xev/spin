package params

import (
	"fmt"

	"charm.land/huh/v2"
)

// SecretParam is a huh.Input with EchoModePassword.
type SecretParam struct {
	name   string
	prompt string
	def    string
	value  string
}

func NewSecret(name, prompt string) *SecretParam {
	return &SecretParam{name: name, prompt: prompt}
}

func (p *SecretParam) Name() string   { return p.name }
func (p *SecretParam) Type() Type     { return TypeSecret }
func (p *SecretParam) Prompt() string { return p.prompt }
func (p *SecretParam) Default() any   { return p.def }
func (p *SecretParam) SetDefault()    { p.Apply(Value{Kind: TypeSecret, String: p.def}) }
func (p *SecretParam) Apply(v Value)  { p.value = v.String }
func (p *SecretParam) Value() Value   { return Value{Kind: TypeSecret, String: p.value} }
func (p *SecretParam) HuhField() huh.Field {
	return huh.NewInput().
		Key(p.name).
		Title(orPrompt(p.name, p.prompt)).
		EchoMode(huh.EchoModePassword).
		Value(&p.value)
}

func (p *SecretParam) String() string {
	return fmt.Sprintf("%s (secret, hidden)", p.name)
}
