package registry

// Pinned is a locally-pinned template (the result of `spin add user/repo`).
// LocalPath is the on-disk location of the cloned/copied template,
// resolved at pin time. Older pin files (pre-v2.0) may have an empty
// LocalPath; the consumer should fall back to CacheDir/templates/<name>.
type Pinned struct {
	Name      string `json:"name"`              // "vercel/nextjs-tailwind"
	Source    string `json:"source"`            // git URL or local path
	PinnedAt  string `json:"pinned_at"`         // ISO 8601
	Version   string `json:"version"`           // last-seen registry version
	LocalPath string `json:"local_path"`        // absolute path on disk (v2.0+)
	Removed   bool   `json:"removed,omitempty"` // soft-deleted; cache still on disk until --purge
}

// RegistryKind enumerates how a registry was sourced.
type RegistryKind string

const (
	// KindGit is a registry cloned from a git URL. Refresh does
	// `git fetch + reset` against the upstream.
	KindGit RegistryKind = "git"
	// KindLocal is a registry symlinked (or copied) from a local
	// path. Refresh is a no-op; the user's filesystem is the source
	// of truth.
	KindLocal RegistryKind = "local"
)

// Registry is one registered registry entry. Alias is the user-facing
// shorthand (`spin add <alias>/<id>`); Source is the original spec
// passed to `spin registry add`; Path is where the registry lives on
// disk under CacheDir/registries/<alias>/.
type Registry struct {
	Alias       string       `json:"alias"`
	Source      string       `json:"source"`
	Kind        RegistryKind `json:"kind"`
	Path        string       `json:"path"`
	AddedAt     string       `json:"added_at,omitempty"`
	LastUpdated string       `json:"last_updated,omitempty"`
}

// RegistriesConfig is the on-disk shape of registries.json. It is a
// thin wrapper so future fields (default alias, schema version) can
// be added without rewriting every consumer.
type RegistriesConfig struct {
	Registries []Registry `json:"registries"`
}

// RegistryMetadata is the registry.toml schema. The id/name are
// required; the rest is documentation. See spin-registry.md.
type RegistryMetadata struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Homepage    string `toml:"homepage"`
	Maintainer  string `toml:"maintainer"`
	License     string `toml:"license"`
}

// TemplateMetadata is the per-templates/*.toml schema. Id is the
// short name users type in `<alias>/<id>`; Name is the human label;
// Source is the git URL or local path the existing template loader
// can already consume (or another `<alias>/<id>` to chain).
type TemplateMetadata struct {
	ID          string   `toml:"id"`
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Source      string   `toml:"source"`
	Tags        []string `toml:"tags"`
	Authors     []string `toml:"authors"`
	License     string   `toml:"license"`
	Homepage    string   `toml:"homepage"`
	Type        string   `toml:"type"`
	Language    string   `toml:"language"`
	Version     string   `toml:"version"`
	UpdatedAt   string   `toml:"updated_at"`
}

// TemplateEntry is one template surfaced to `spin search`: the
// registry's alias plus the template metadata, so callers can render
// `<alias>/<id>` directly.
type TemplateEntry struct {
	Alias       string
	ID          string
	Name        string
	Description string
	Source      string
	Tags        []string
	Type        string
	Language    string
	Version     string
}
