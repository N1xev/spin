package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	srcspec "github.com/N1xev/spin/internal/spec"
)

// ErrUnresolved is returned when a `<alias>/<id>` shorthand cannot
// be matched to a registered template.
var ErrUnresolved = errors.New("registry: shorthand unresolved")

// Resolved is the output of ResolveShorthand: the template's
// upstream source plus the kind (local or git) for downstream
// consumers like the template Loader. Alias and ID are the original
// parts the user typed.
type Resolved struct {
	Alias  string
	ID     string
	Source string
	Kind   RegistryKind
}

// IsShorthand reports whether spec is a `<alias>/<id>` shorthand.
// URLs and filesystem paths fall through to the local/git paths in
// template.Loader.
func IsShorthand(spec string) bool {
	return srcspec.IsShorthand(spec)
}

// ResolveShorthand looks up `<alias>/<id>` against the manager's
// registries. Returns ErrUnresolved when the alias is not
// registered, the id is not present, or the template's source is
// missing. Recurses once if the resolved source is itself a
// shorthand (max depth 2); cycles are rejected.
func (m Manager) ResolveShorthand(spec string) (Resolved, error) {
	return m.resolveShorthandDepth(spec, 0)
}

// resolveShorthandDepth is the internal recursive helper. depth=0 is
// the user-typed spec; depth>0 means a previous resolution aliased
// to another shorthand and we're following the chain.
func (m Manager) resolveShorthandDepth(spec string, depth int) (Resolved, error) {
	if depth > 1 {
		return Resolved{}, fmt.Errorf("registry: shorthand chain too deep (cycle?): %s", spec)
	}
	if !IsShorthand(spec) {
		return Resolved{}, fmt.Errorf("%w: %q is not a <alias>/<id>", ErrUnresolved, spec)
	}
	alias, id := splitAliasID(spec)
	reg, ok := m.Get(alias)
	if !ok {
		return Resolved{}, fmt.Errorf("%w: alias %q not registered", ErrUnresolved, alias)
	}
	tplPath := filepath.Join(reg.Path, "templates", id+".toml")
	if _, err := os.Stat(tplPath); err != nil {
		return Resolved{}, fmt.Errorf("%w: template %q not in registry %q", ErrUnresolved, id, alias)
	}
	var tpl TemplateMetadata
	if _, err := toml.DecodeFile(tplPath, &tpl); err != nil {
		return Resolved{}, fmt.Errorf("registry: parse %s: %w", tplPath, err)
	}
	if tpl.Source == "" {
		return Resolved{}, fmt.Errorf("registry: template %q has empty source", id)
	}
	// If the source is itself a shorthand, follow the chain.
	if IsShorthand(tpl.Source) {
		return m.resolveShorthandDepth(tpl.Source, depth+1)
	}
	return Resolved{
		Alias:  alias,
		ID:     id,
		Source: tpl.Source,
		Kind:   reg.Kind,
	}, nil
}

// splitAliasID returns alias and id from a `<alias>/<id>` string.
// Caller must have already verified IsShorthand.
func splitAliasID(spec string) (alias, id string) {
	i := strings.IndexByte(spec, '/')
	return spec[:i], spec[i+1:]
}
