package registry

import (
	"fmt"
	"sort"
	"strings"
)

// FormatSearch writes a human-readable search result table to w.
func FormatSearch(r *SearchResult, plain bool) string {
	if r == nil || len(r.Entries) == 0 {
		return fmt.Sprintf("No templates matched %q.\n", r.Query)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d template(s) for %q:\n\n", r.Total, r.Query)
	fmt.Fprintf(&sb, "%-30s  %-10s  %-12s  %s\n", "NAME", "LANG", "TYPE", "DESCRIPTION")
	for _, e := range r.Entries {
		fmt.Fprintf(&sb, "%-30s  %-10s  %-12s  %s\n",
			e.Name, e.Language, e.Type, truncate(e.Description, 50))
	}
	if !plain {
		fmt.Fprintf(&sb, "\nAdd a template:  spin add <name>\n")
	}
	return sb.String()
}

// SortByPopularity sorts entries by download count (desc).
func SortByPopularity(es []Entry) []Entry {
	out := append([]Entry{}, es...)
	sort.Slice(out, func(i, j int) bool { return out[i].Downloads > out[j].Downloads })
	return out
}

func truncate(s string, n int) string {
	if n <= 3 || len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], " ") + "…"
}
