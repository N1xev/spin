package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// listTask is the JSON-friendly view of a Task. We don't marshal Task
// directly because that would expose the Source/Order/Notes fields
// (the first two are derived state, the third is a free-form note
// best surfaced under a different name in JSON).
type listTask struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Command string `json:"command"`
	Notes   string `json:"notes,omitempty"`
}

// List writes a human-readable table of all available tasks to w.
// The output format is:
//
//	TASKS    SOURCE                       COMMAND
//	dev      spin.config.toml:8           air
//	test     spin.config.toml:10          prism   (fallback: go test ./...)
//	bench    Taskfile.yml:14              go test -bench=.
//	...
//
// When the merged list is empty, prints a hint pointing the user at
// either `spin new` (to scaffold) or one of the task files.
func (r *Runner) List(w io.Writer) error {
	all, err := r.All()
	if err != nil {
		return err
	}

	// Column widths
	wName, wSrc, wCmd := 8, 30, 30
	for _, t := range all {
		if len(t.Name) > wName {
			wName = len(t.Name)
		}
		if len(t.Source) > wSrc {
			wSrc = len(t.Source)
		}
		if len(t.Command) > wCmd {
			wCmd = len(t.Command)
		}
	}
	wSrc = min(wSrc, 35)
	wCmd = min(wCmd, 60)

	fmt.Fprintf(w, "%-*s  %-*s  %s\n", wName, "TASK", wSrc, "SOURCE", "COMMAND")
	for _, t := range all {
		fmt.Fprintf(w, "%-*s  %-*s  %s\n", wName, t.Name, wSrc, truncate(t.Source, wSrc), truncate(t.Command, wCmd))
	}

	if len(all) == 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "No tasks defined.")
		fmt.Fprintln(w, "Tip: run `spin new <name> --type=<tui|cli|lib>` to scaffold a project,")
		fmt.Fprintln(w, "or create a Taskfile.yml / Makefile / package.json with scripts.")
	}

	return nil
}

// ListJSON writes the task list as a JSON array. The output is a
// stable, machine-readable format suitable for shell pipelines
// (`./spin run --list --json | jq '.[] | select(.name == "build")'`).
// Each element is a listTask with name, source, command, and (when
// present) notes.
func (r *Runner) ListJSON(w io.Writer) error {
	all, err := r.All()
	if err != nil {
		return err
	}
	out := make([]listTask, 0, len(all))
	for _, t := range all {
		out = append(out, listTask{
			Name:    t.Name,
			Source:  t.Source,
			Command: t.Command,
			Notes:   t.Notes,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func truncate(s string, n int) string {
	if n <= 3 || len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], " ") + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
