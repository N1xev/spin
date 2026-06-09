package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/spin/internal/ecosystem"
	"github.com/example/spin/internal/ecosystems/rust"
	"github.com/example/spin/internal/runner"
)

// TestEcosystemTasks_RustBeatsFallback is the end-to-end test for the
// source-precedence chain. It builds a Runner with both the
// ecosystemTasks source (rust) and the hardcoded fallback in a
// tempdir containing a Cargo.toml, and asserts:
//
//  1. The `build` task comes from `ecosystem:rust` (Order=5), not
//     from `fallback` (Order=0). The merge function picks the
//     higher-Order entry on conflict, so this proves the chain
//     wires the ecosystem source above the fallback.
//  2. The `clippy` and `fmt` tasks exist — these are NOT in the
//     hardcoded fallback, so their presence proves the ecosystem
//     source is the authority for language-specific tasks.
//
// This is the load-bearing verification for Phase 5 success
// criterion 4: "spin run build in a generated rust project invokes
// cargo build". The rust ecosystem's Tasks() supplies cargo build;
// the runner's merge selects it over the hardcoded fallback.
//
// This test file lives in the `sources` package (not in
// `internal/ecosystem`) so it can import the concrete
// `internal/ecosystems/rust` package without an import cycle. The
// ecosystem package itself imports this package's parent, which
// would create the cycle if we tried to import ecosystem.Ecosystem
// from internal/ecosystem's own test file.
func TestEcosystemTasks_RustBeatsFallback(t *testing.T) {
	dir := t.TempDir()
	// Empty Cargo.toml is sufficient — the rust ecosystem's Matches()
	// only checks file presence, not content.
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname=\"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build the source list with both the rust ecosystem AND the
	// hardcoded fallback. The runner's merge function should pick
	// the higher-Order entry (ecosystem=5) on conflict.
	ecos := []ecosystem.Ecosystem{rust.New()}

	r := runner.New(dir)
	r.Sources = []runner.TaskSource{
		NewFallback(),
		NewEcosystemTasks(ecos),
	}

	all, err := r.All()
	if err != nil {
		t.Fatalf("All(): %v", err)
	}

	// 1) build must come from ecosystem:rust
	var build *runner.Task
	for i := range all {
		if all[i].Name == "build" {
			build = &all[i]
			break
		}
	}
	if build == nil {
		t.Fatalf("no `build` task found; got %d tasks: %v", len(all), taskNames(all))
	}
	if !strings.Contains(build.Source, "ecosystem:rust") {
		t.Errorf("build.Source = %q, want substring %q (the rust ecosystem should win over fallback)", build.Source, "ecosystem:rust")
	}
	if build.Command != "cargo build" {
		t.Errorf("build.Command = %q, want %q", build.Command, "cargo build")
	}

	// 2) clippy and fmt must be present (NOT in the hardcoded fallback)
	if !hasTask(all, "clippy") {
		t.Errorf("clippy task missing; got %v", taskNames(all))
	}
	if !hasTask(all, "fmt") {
		t.Errorf("fmt task missing; got %v", taskNames(all))
	}
}

// TestEcosystemTasks_Detect_NoMatch verifies Detect returns false
// when no wrapped ecosystem's Detector.Matches returns true.
func TestEcosystemTasks_Detect_NoMatch(t *testing.T) {
	dir := t.TempDir()
	src := NewEcosystemTasks([]ecosystem.Ecosystem{rust.New()})
	if src.Detect(dir) {
		t.Errorf("Detect(%q) = true, want false (no Cargo.toml)", dir)
	}
}

// TestEcosystemTasks_OrderIsFive pins the Order() value at 5. The
// runner's merge function picks higher-Order on conflict, so this
// value is load-bearing for the chain: 5 is above the hardcoded
// fallback (0) but below scripts/ (20), package.json (30), Makefile
// (40), Taskfile (60), and spin.config.toml (100). Changing it
// silently would re-order the chain.
func TestEcosystemTasks_OrderIsFive(t *testing.T) {
	src := NewEcosystemTasks([]ecosystem.Ecosystem{rust.New()})
	if got := src.Order(); got != 5 {
		t.Errorf("Order() = %d, want 5", got)
	}
}

// TestEcosystemTasks_Name is a trivial assertion that the source
// surfaces under the canonical "ecosystem" name so --list output
// is stable across runs.
func TestEcosystemTasks_Name(t *testing.T) {
	src := NewEcosystemTasks([]ecosystem.Ecosystem{rust.New()})
	if got := src.Name(); got != "ecosystem" {
		t.Errorf("Name() = %q, want %q", got, "ecosystem")
	}
}

func hasTask(ts []runner.Task, name string) bool {
	for _, t := range ts {
		if t.Name == name {
			return true
		}
	}
	return false
}

func taskNames(ts []runner.Task) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}
