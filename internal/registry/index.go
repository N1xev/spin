package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Index is a snapshot of every valid template under every registered
// registry. Build scans once and the result is immutable for Search
// calls -- a registry update is the caller's signal to rebuild.
//
// Index is built lazily on demand; it does not cache itself across
// process runs. Each `spin search` is fast enough (sub-millisecond
// per TOML file) that caching would add invalidation complexity for
// no perceivable win.
type Index struct {
	entries []TemplateEntry
}

// Build scans every registry's templates/*.toml, parses and
// validates each file, and returns the resulting index. Invalid
// files are skipped; the per-registry error counts are returned in
// `errors` so the CLI can surface them in the update summary.
func (m Manager) Build() (*Index, map[string]int, error) {
	cfg, err := m.Load()
	if err != nil {
		return nil, nil, err
	}
	idx := &Index{}
	skipCounts := make(map[string]int)
	for _, reg := range cfg.Registries {
		tplDir := filepath.Join(reg.Path, "templates")
		entries, err := os.ReadDir(tplDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // not yet populated -- treat as zero
			}
			skipCounts[reg.Alias]++
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
				continue
			}
			var tpl TemplateMetadata
			if _, err := toml.DecodeFile(filepath.Join(tplDir, e.Name()), &tpl); err != nil {
				skipCounts[reg.Alias]++
				continue
			}
			if !validTemplate(&tpl, e.Name()) {
				skipCounts[reg.Alias]++
				continue
			}
			idx.entries = append(idx.entries, TemplateEntry{
				Alias:       reg.Alias,
				ID:          tpl.ID,
				Name:        tpl.Name,
				Description: tpl.Description,
				Source:      tpl.Source,
				Tags:        tpl.Tags,
				Type:        tpl.Type,
				Language:    tpl.Language,
				Version:     tpl.Version,
			})
		}
	}
	sort.Slice(idx.entries, func(i, j int) bool {
		return idx.entries[i].Alias+"/"+idx.entries[i].ID < idx.entries[j].Alias+"/"+idx.entries[j].ID
	})
	return idx, skipCounts, nil
}

// validTemplate enforces the registry template contract: id must be
// present and match the file basename, source must be non-empty.
// Other fields are advisory.
func validTemplate(tpl *TemplateMetadata, fileName string) bool {
	if tpl.ID == "" || tpl.Name == "" || tpl.Source == "" {
		return false
	}
	// The id field must match the file basename (sans .toml). This
	// prevents `<alias>/foo` from accidentally resolving to a file
	// named `bar.toml`.
	want := strings.TrimSuffix(fileName, ".toml")
	return tpl.ID == want
}

// Search returns the entries whose alias, id, name, description, or
// tags contain query as a substring, sorted by relevance:
// exact id match > id substring > name > description > tag.
// Empty query returns every entry in id-ascending order.
func (idx *Index) Search(query string, limit int) []TemplateEntry {
	scored := make([]scoredEntry, 0, len(idx.entries))
	q := strings.ToLower(query)
	for _, e := range idx.entries {
		s := score(e, q)
		if query != "" && s == 0 {
			continue
		}
		scored = append(scored, scoredEntry{entry: e, score: s})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		// Tie-break on id (then alias) so a query like "go" surfaces
		// go-api ahead of go-tui regardless of registry order.
		if scored[i].entry.ID != scored[j].entry.ID {
			return scored[i].entry.ID < scored[j].entry.ID
		}
		return scored[i].entry.Alias < scored[j].entry.Alias
	})
	out := make([]TemplateEntry, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.entry)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// score returns a numeric relevance score. Higher is better. 0 means
// "no match" (caller should filter).
func score(e TemplateEntry, q string) int {
	if q == "" {
		return 1
	}
	id := strings.ToLower(e.Alias + "/" + e.ID)
	if id == q {
		return 100
	}
	if strings.Contains(id, q) {
		return 50
	}
	if strings.Contains(strings.ToLower(e.Name), q) {
		return 30
	}
	if strings.Contains(strings.ToLower(e.Description), q) {
		return 20
	}
	for _, t := range e.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return 10
		}
	}
	return 0
}

type scoredEntry struct {
	entry TemplateEntry
	score int
}

// Validate scans a single registry's metadata for completeness.
// Returns a slice of error messages for the registry.toml (id, name)
// and any templates/*.toml that fail to parse or fail validTemplate.
// Empty slice means the registry is fully valid.
func (m Manager) Validate(alias string) []string {
	reg, ok := m.Get(alias)
	if !ok {
		return []string{fmt.Sprintf("%s: not registered", alias)}
	}
	var out []string
	// Registry-level
	var rm RegistryMetadata
	if _, err := toml.DecodeFile(filepath.Join(reg.Path, "registry.toml"), &rm); err != nil {
		out = append(out, fmt.Sprintf("%s: registry.toml: %v", alias, err))
	} else {
		if rm.ID == "" {
			out = append(out, fmt.Sprintf("%s: registry.toml: missing id", alias))
		}
		if rm.Name == "" {
			out = append(out, fmt.Sprintf("%s: registry.toml: missing name", alias))
		}
	}
	tplDir := filepath.Join(reg.Path, "templates")
	entries, err := os.ReadDir(tplDir)
	if err != nil {
		out = append(out, fmt.Sprintf("%s: templates/: %v", alias, err))
		return out
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
			continue
		}
		var tpl TemplateMetadata
		if _, err := toml.DecodeFile(filepath.Join(tplDir, e.Name()), &tpl); err != nil {
			out = append(out, fmt.Sprintf("%s/templates/%s: %v", alias, e.Name(), err))
			continue
		}
		if !validTemplate(&tpl, e.Name()) {
			out = append(out, fmt.Sprintf("%s/templates/%s: missing id/name/source or id mismatch", alias, e.Name()))
		}
	}
	return out
}