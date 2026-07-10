package params

import (
	"fmt"

	"charm.land/huh/v2"
)

// Type is the wire name used in spin.toml (e.g. `type = "text"`).
type Type string

const (
	TypeText        Type = "text"
	TypeTextarea    Type = "textarea"
	TypeNumber      Type = "number"
	TypeSelect      Type = "select"
	TypeMultiSelect Type = "multiselect"
	TypeBool        Type = "bool"
	TypePath        Type = "path"
	TypeSecret      Type = "secret"
)

// Value is the resolved value of a Param after the form has run.
// Only one of the data fields is populated, depending on Kind.
type Value struct {
	Kind   Type
	String string
	Int    int
	Bool   bool
	List   []string
	Path   string
}

// Param is the interface implemented by every typed parameter.
type Param interface {
	Name() string
	Type() Type
	Prompt() string
	Default() any
	Hmm() string // human-readable summary
	Apply(v Value)
	Value() Value
	// HuhField builds the huh field for this param. The form runner
	// writes the result back via Apply().
	HuhField() huh.Field
	// String returns a one-line summary, used by `spin new --print-params`.
	String() string
}

// Spec is the raw shape we accept from a parsed spin.toml block.
// The parse step turns a Spec into a concrete Param.
type Spec struct {
	Type    Type     `toml:"type"`
	Prompt  string   `toml:"prompt"`
	Default any      `toml:"default"`
	Min     *int     `toml:"min"`
	Max     *int     `toml:"max"`
	Options []string `toml:"options"`
}

// ErrUnknownType is returned when a param declares a type we don't recognise.
type ErrUnknownType struct {
	Name string
	Type Type
}

func (e ErrUnknownType) Error() string {
	return fmt.Sprintf("param %q: unknown type %q (want text, textarea, number, select, multiselect, bool, path, secret)", e.Name, e.Type)
}

// orPrompt returns p if set, else the name. Keeps huh titles sensible when
// a template author forgets to set a prompt.
func orPrompt(name, p string) string {
	if p != "" {
		return p
	}
	return name
}


