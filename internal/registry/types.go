// Package registry is the client for the public template/ecosystem
// registry. The registry server itself is a separate project; this
// package only handles the CLI side: search, add (pin a template
// locally), list (show pinned), and remove.
package registry

import "fmt"

// DefaultIndexURL is the default registry endpoint. Overridable via
// SPIN_REGISTRY_URL env var (SPIN_REGISTRY is honored as a fallback
// for backward compatibility). The .invalid TLD is reserved by RFC
// 2606 and guaranteed never to resolve, so the friendly
// "not yet deployed" message is shown until the real server ships.
const DefaultIndexURL = "https://registry.spin.invalid/v1"

// ErrNotDeployed is returned by Client methods when the registry
// server is unreachable (DNS failure, connection refused, timeout, or
// HTTP 404). The CLI surfaces this as a friendly message — never as
// a stack trace. The name reflects v2.0: the server is not yet
// deployed, so this is a transient condition, not a missing
// implementation.
var ErrNotDeployed = fmt.Errorf("registry: public index not yet deployed; see spin registry roadmap")

// ErrNotImplemented is the v2.0-skeleton name. Kept as an alias for
// backward compatibility with code that pre-dates the v2.0 rename.
// New code should use ErrNotDeployed.
var ErrNotImplemented = ErrNotDeployed

// Entry is a single record in the registry index.
type Entry struct {
	Name        string   `json:"name"`        // "vercel/nextjs-tailwind"
	Description string   `json:"description"` // one-liner
	Tags        []string `json:"tags"`        // ["nextjs", "tailwind", "vercel"]
	Language    string   `json:"language"`    // "typescript"
	Type        string   `json:"type"`        // "tui" | "cli" | "lib" | ...
	Version     string   `json:"version"`     // semver
	Downloads   int      `json:"downloads"`   // for sort-by-popularity
	Source      string   `json:"source"`      // git URL
	UpdatedAt   string   `json:"updated_at"`  // ISO 8601
}

// SearchResult is the response from `GET /v1/search?q=...`.
type SearchResult struct {
	Query   string  `json:"query"`
	Total   int     `json:"total"`
	Entries []Entry `json:"entries"`
}

// Pinned is a locally-pinned template (the result of `spin add user/repo`).
// LocalPath is the on-disk location of the cloned/copied template,
// resolved at pin time. Older pin files (pre-v2.0) may have an empty
// LocalPath; the consumer should fall back to CacheDir/templates/<name>.
type Pinned struct {
	Name      string `json:"name"`        // "vercel/nextjs-tailwind"
	Source    string `json:"source"`      // git URL or local path
	PinnedAt  string `json:"pinned_at"`   // ISO 8601
	Version   string `json:"version"`     // last-seen registry version
	LocalPath string `json:"local_path"`  // absolute path on disk (v2.0+)
}
