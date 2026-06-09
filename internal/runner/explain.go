package runner

import (
	"encoding/json"
	"fmt"
	"io"
)

// explainTask is the JSON-friendly view of a Task for --explain.
type explainTask struct {
	Name    string   `json:"name"`
	Source  string   `json:"source"`
	Command string   `json:"command"`
	Workdir string   `json:"workdir"`
	Watch   string   `json:"watch,omitempty"`
	Notes   string   `json:"notes,omitempty"`
	Env     []string `json:"env,omitempty"`
}

// Explain writes a detailed one-task description to w. The format is:
//
//	task <name>
//	  source:   <source>
//	  command:  <command>
//	  workdir:  <dir>
//	  watch:    <watch>     (only when set)
//	  env:      K=V ...     (only when set; one per line)
//	  notes:    <notes>     (only when set)
func (r *Runner) Explain(w io.Writer, name string) error {
	t, err := r.Resolve(name)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, t.Name)
	fmt.Fprintf(w, "  source:   %s\n", t.Source)
	fmt.Fprintf(w, "  command:  %s\n", t.Command)
	fmt.Fprintf(w, "  workdir:  %s\n", r.Dir)
	if t.Watch != "" {
		fmt.Fprintf(w, "  watch:    %s\n", t.Watch)
	}
	if len(t.Env) > 0 {
		fmt.Fprintln(w, "  env:")
		for _, e := range t.Env {
			fmt.Fprintf(w, "    - %s\n", e)
		}
	}
	if t.Notes != "" {
		fmt.Fprintf(w, "  notes:    %s\n", t.Notes)
	}
	return nil
}

// ExplainJSON writes the per-task explanation as a single JSON
// object. The fields are a superset of the human output: name,
// source, command, workdir, watch (if set), env (if set, []string),
// notes (if set). Errors with ErrNotFound (typed) are JSON-encoded
// as `{"error": "spin run: no task named \"foo\""}` so the consumer
// can distinguish a real explain output from a "not found" reply
// without parsing strings.
func (r *Runner) ExplainJSON(w io.Writer, name string) error {
	t, err := r.Resolve(name)
	if err != nil {
		// Encode the error as JSON so the consumer can distinguish
		// "no such task" from "transport error".
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]string{"error": err.Error()})
	}
	et := explainTask{
		Name:    t.Name,
		Source:  t.Source,
		Command: t.Command,
		Workdir: r.Dir,
		Watch:   t.Watch,
		Notes:   t.Notes,
		Env:     t.Env,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(et)
}
