package registry

import (
	"context"
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
var ErrUnresolved = errors.New("shorthand unresolved")

// SplitAliasID splits a `<alias>/<id>` shorthand into its parts.
func SplitAliasID(spec string) (alias, id string) {
	i := strings.IndexByte(spec, '/')
	return spec[:i], spec[i+1:]
}

// splitAliasID is the unexported alias for internal use.
func splitAliasID(spec string) (alias, id string) {
	return SplitAliasID(spec)
}

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
func (m Manager) ResolveShorthand(ctx context.Context, spec string) (Resolved, error) {
	if err := ctx.Err(); err != nil {
		return Resolved{}, err
	}
	return m.resolveShorthandDepth(ctx, spec, 0)
}

// resolveShorthandDepth is the internal recursive helper. depth=0 is
// the user-typed spec; depth>0 means a previous resolution aliased
// to another shorthand and we're following the chain.
func (m Manager) resolveShorthandDepth(ctx context.Context, spec string, depth int) (Resolved, error) {
	if depth > 1 {
		return Resolved{}, fmt.Errorf("shorthand chain too deep (cycle?): %s", spec)
	}
	if !IsShorthand(spec) {
		return Resolved{}, fmt.Errorf("shorthand %q is not an <alias>/<id>", spec)
	}
	alias, id := splitAliasID(spec)
	reg, ok := m.Get(ctx, alias)
	if !ok {
		return Resolved{}, fmt.Errorf("alias %q not registered", alias)
	}
	tplPath := filepath.Join(reg.Path, "templates", id+".toml")
	if _, err := os.Stat(tplPath); err != nil {
		return Resolved{}, fmt.Errorf("template %q not in registry %q", id, alias)
	}
	var tpl TemplateMetadata
	if _, err := toml.DecodeFile(tplPath, &tpl); err != nil {
		return Resolved{}, fmt.Errorf("parse %s: %w", tplPath, err)
	}
	if tpl.Source == "" {
		return Resolved{}, fmt.Errorf("template %q has empty source", id)
	}
	// If the source is itself a shorthand, follow the chain.
	if IsShorthand(tpl.Source) {
		return m.resolveShorthandDepth(ctx, tpl.Source, depth+1)
	}
	return Resolved{
		Alias:  alias,
		ID:     id,
		Source: tpl.Source,
		Kind:   reg.Kind,
	}, nil
}
