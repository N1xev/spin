package doctor

import (
	"encoding/json"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
)

// glyphs used by the human renderer. Kept as package-level consts so
// tests can match them without depending on lipgloss output bytes.
const (
	glyphPass = "✓"
	glyphWarn = "!"
	glyphFail = "✗"
)

// statusStyles returns the lipgloss style used to color the icon
// glyph for each status. We style the icon only (not the whole line)
// so the user's terminal width is not consumed by ANSI codes and
// the lines stay easy to grep.
func statusStyles() (pass, warn, fail lipgloss.Style) {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
}

// RenderHuman writes the human-readable report to w. One line per
// check, status-colored icon glyph + name + message, optional hint
// on a second indented line, and a trailing summary line.
//
// Errors are returned only when w fails to accept the bytes. Output
// is always one line per check plus the summary; never partial.
func RenderHuman(w io.Writer, results []CheckResult) error {
	passS, warnS, failS := statusStyles()
	var pass, warnC, fail int
	for i, r := range results {
		var icon string
		switch r.Status {
		case StatusPass:
			icon = passS.Render(glyphPass)
			pass++
		case StatusWarn:
			icon = warnS.Render(glyphWarn)
			warnC++
		case StatusFail:
			icon = failS.Render(glyphFail)
			fail++
		default:
			icon = "?"
		}
		if _, err := fmt.Fprintf(w, "  %s %s  %s\n", icon, r.Name, r.Message); err != nil {
			return err
		}
		if r.Hint != "" {
			if _, err := fmt.Fprintf(w, "      hint: %s\n", r.Hint); err != nil {
				return err
			}
		}
		_ = i
	}
	if _, err := fmt.Fprintf(w, "\n%d passed, %d warned, %d failed\n", pass, warnC, fail); err != nil {
		return err
	}
	return nil
}

// jsonCheck is the on-the-wire view of a CheckResult. The fields are
// kept separate from CheckResult so the public type stays free of
// json tags (the human renderer doesn't need them) and the schema
// is locked in one place.
type jsonCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

// jsonReport is the top-level JSON document. The "checks" key is
// stable; renaming it is a breaking change.
type jsonReport struct {
	Checks []jsonCheck `json:"checks"`
}

// RenderJSON writes the stable JSON schema to w: a single object with
// a "checks" array of {name, status, message, hint} objects. Uses
// json.Encoder (not json.Marshal) so an io.Writer failure surfaces.
func RenderJSON(w io.Writer, results []CheckResult) error {
	rep := jsonReport{Checks: make([]jsonCheck, 0, len(results))}
	for _, r := range results {
		rep.Checks = append(rep.Checks, jsonCheck{
			Name:    r.Name,
			Status:  string(r.Status),
			Message: r.Message,
			Hint:    r.Hint,
		})
	}
	return json.NewEncoder(w).Encode(rep)
}

// FormatSelector dispatches to RenderHuman or RenderJSON based on
// format. Empty format defaults to human (the CLI's --format default
// is "human" too, but we tolerate "" so the call sites don't have to
// normalize). Unknown formats return an error so the CLI can surface
// a useful message via cobra/fang.
func FormatSelector(format string, results []CheckResult, w io.Writer) error {
	switch format {
	case "", "human":
		return RenderHuman(w, results)
	case "json":
		return RenderJSON(w, results)
	default:
		return fmt.Errorf("doctor: unknown --format %q (want human or json)", format)
	}
}
