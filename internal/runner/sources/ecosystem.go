// ecosystem.go — wraps every registered ecosystem's Tasks() as a
// runner.TaskSource. The language-specific fallbacks (cargo, go, etc.)
// live with the ecosystems themselves; the runner pulls them into the
// source chain at Order=5, just above the hardcoded fallback at 0.
//
// Why this exists: the v2.0 universal-scaffolder model says that each
// ecosystem owns its own task list. The rust ecosystem's Tasks() returns
// the 5 cargo fallbacks (build/test/run/clippy/fmt). The charm
// ecosystem's Tasks() returns the legacy v1.0 toolchain wrappers (air
// for dev, prism for test, gofumpt for fmt, ...). All of them are
// merged into the source chain here, so `spin run build` in a Cargo
// project invokes `cargo build` (not the hardcoded fallback), and
// `spin run dev` in a charm project invokes `air`.
//
// The hardcoded fallback in fallback.go stays in the chain for
// resilience: if no ecosystem is registered (or a project uses a
// marker we don't recognise), the fallback fills the gap. Ecosystem
// tasks always win on conflict because Order=5 > Order=0.

package sources

import (
	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/runner"
)

// NewEcosystemTasks wraps a list of ecosystems as a runner.TaskSource.
// The returned source contributes the union of every wrapped
// ecosystem's Tasks() map, tagged with source=ecosystem:<name> so
// `spin run --list` can show which ecosystem supplied each task.
func NewEcosystemTasks(ecos []ecosystem.Ecosystem) runner.TaskSource {
	return &ecosystemTasks{ecos: ecos}
}

type ecosystemTasks struct {
	ecos []ecosystem.Ecosystem
}

func (e *ecosystemTasks) Name() string { return "ecosystem" }

// Order is 5: above the hardcoded fallback (0) so ecosystem tasks win
// on conflict, but below the user-facing sources (scripts:20,
// package.json:30, makefile:40, taskfile:60, spinconfig:100) so a
// user's spin.config.toml still beats a project's default cargo
// fallback.
func (e *ecosystemTasks) Order() int { return 5 }

// Detect returns true when ANY wrapped ecosystem's Detector.Matches
// returns true. This is the union semantics: the source "applies"
// whenever at least one ecosystem recognises the directory. The
// individual task contributions are then filtered in Tasks().
func (e *ecosystemTasks) Detect(dir string) bool {
	for _, eco := range e.ecos {
		if eco.Matches(dir) {
			return true
		}
	}
	return false
}

// Tasks returns the union of every matching ecosystem's Tasks() map.
// Each task is tagged with `ecosystem:<name>` so `spin run --list`
// shows the contributing ecosystem unambiguously. The Source field of
// the returned Task struct is set to the ecosystem name (e.g.
// "rust"); the runner's All() will prefix the source name as
// `<Source.Name()>:<Task.Source>` (giving "ecosystem:rust"), so the
// flag here is just the bare ecosystem name.
func (e *ecosystemTasks) Tasks(dir string) ([]runner.Task, error) {
	out := []runner.Task{}
	for _, eco := range e.ecos {
		if !eco.Matches(dir) {
			continue
		}
		for name, cmd := range eco.Tasks() {
			out = append(out, runner.Task{
				Name:    name,
				Command: cmd,
				Source:  eco.Name(),
			})
		}
	}
	return out, nil
}
