package params

import (
	"fmt"

	"charm.land/huh/v2"
)

// PathParam is a huh.NewFilePicker; default is used as the placeholder
// starting directory (or filename if non-existent).
type PathParam struct {
	name   string
	prompt string
	def    string
	value  string
	dir    bool // true ⇒ directory picker, false ⇒ file picker
}

func NewPath(name, prompt, def string) *PathParam {
	return &PathParam{name: name, prompt: prompt, def: def, dir: false}
}

func NewDir(name, prompt, def string) *PathParam {
	return &PathParam{name: name, prompt: prompt, def: def, dir: true}
}

func (p *PathParam) Name() string   { return p.name }
func (p *PathParam) Type() Type     { return TypePath }
func (p *PathParam) Prompt() string { return p.prompt }
func (p *PathParam) Default() any   { return p.def }
func (p *PathParam) Apply(v Value)  { p.value = v.Path }
func (p *PathParam) Value() Value   { return Value{Kind: TypePath, Path: p.value} }
func (p *PathParam) Hmm() string    { return p.String() }

func (p *PathParam) HuhField() huh.Field {
	f := huh.NewFilePicker().
		Key(p.name).
		Title(orPrompt(p.name, p.prompt))
	if p.dir {
		f = f.FileAllowed(false).DirAllowed(true)
	} else {
		f = f.FileAllowed(true).DirAllowed(false)
	}
	return f.Value(&p.value)
}

func (p *PathParam) String() string {
	kind := "file"
	if p.dir {
		kind = "dir"
	}
	return fmt.Sprintf("%s (path, %s, default %q)", p.name, kind, p.def)
}
