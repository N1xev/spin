// Package runner is the universal task-runner engine for spin.
//
// It resolves task names to commands across multiple sources, with a
// declared precedence order (lowest Order() wins; spin.config.toml is
// the highest-precedence source). The same Task struct is used for
// listing, explaining, and executing.
package runner

import (
	"sort"
	"strings"
)

// Task is the resolved description of a runnable task.
type Task struct {
	Name    string   // "dev", "test", "build"
	Command string   // shell command, may be multi-statement ("a && b")
	Source  string   // human-friendly: "spin.config.toml:8"
	Order   int      // source precedence
	Watch   string   // optional watch-mode command
	Notes   string   // free-form: "uses prism if available"
	Env     []string // optional env vars ("FOO=bar") set before running
}

// TaskSource discovers tasks in a particular kind of file (spin config,
// Taskfile, Makefile, package.json, scripts/, fallback).
type TaskSource interface {
	// Name is the display name used in --list output.
	Name() string
	// Order is the precedence (lower wins).
	Order() int
	// Detect returns true if this source applies to the given directory.
	Detect(dir string) bool
	// Tasks returns every task this source contributes.
	Tasks(dir string) ([]Task, error)
}

// Runner is the top-level orchestrator. Holds the working directory
// and the list of sources to consult.
type Runner struct {
	Dir     string
	Sources []TaskSource
}

// New constructs a Runner with no sources wired. Callers (typically
// the CLI) populate Sources explicitly. The default chain is built
// in cmd/run.go via DefaultSources().
func New(dir string) *Runner {
	return &Runner{Dir: dir}
}

// DefaultSources returns the canonical source chain. Callers wire
// this into Runner.Sources. Lives in this package to keep the
// runner/sources dependency edge one-way (sources → runner).
func DefaultSources() []TaskSource {
	return nil // populated by cmd/run.go to avoid an import cycle here
}

// All returns every task found across every applicable source, with
// duplicates resolved by precedence (higher Order wins).
func (r *Runner) All() ([]Task, error) {
	all := []Task{}
	for _, s := range r.Sources {
		if !s.Detect(r.Dir) {
			continue
		}
		ts, err := s.Tasks(r.Dir)
		if err != nil {
			return nil, err
		}
		for _, t := range ts {
			t.Order = s.Order()
			t.Source = s.Name() + ":" + t.Source
			all = append(all, t)
		}
	}
	merged := merge(all)
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Name != merged[j].Name {
			return merged[i].Name < merged[j].Name
		}
		return merged[i].Order > merged[j].Order
	})
	return merged, nil
}

// Resolve returns the winning task for a name, or an error if no source
// supplies it.
func (r *Runner) Resolve(name string) (Task, error) {
	all, err := r.All()
	if err != nil {
		return Task{}, err
	}
	for _, t := range all {
		if t.Name == name {
			return t, nil
		}
		// support the "source:task" form for explicit disambiguation
		if strings.HasPrefix(t.Name, name+":") {
			return t, nil
		}
	}
	return Task{}, &ErrNotFound{Name: name}
}

// merge collapses duplicates by name. Higher-Order (precedence) wins.
func merge(ts []Task) []Task {
	by := map[string]Task{}
	for _, t := range ts {
		cur, ok := by[t.Name]
		if !ok || t.Order > cur.Order {
			by[t.Name] = t
		}
	}
	out := make([]Task, 0, len(by))
	for _, t := range by {
		out = append(out, t)
	}
	return out
}

// ErrNotFound is returned by Resolve when no source supplies the task.
type ErrNotFound struct{ Name string }

func (e ErrNotFound) Error() string {
	return "spin run: no task named " + strconvQuote(e.Name)
}

func strconvQuote(s string) string { return `"` + s + `"` }
