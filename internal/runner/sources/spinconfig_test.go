package sources

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestSpinConfig_Parse verifies the parser handles a typical
// spin.config.toml with three tasks. The shorthand form
// (`name = "command"`) is the most common case.
func TestSpinConfig_Parse(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tasks]
build = "go build ./..."
test  = "prism -v ./..."
dev   = "air"
`
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewSpinConfig()
	tasks, err := src.Tasks(dir)
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3: %+v", len(tasks), tasks)
	}
	byName := map[string]string{}
	for _, task := range tasks {
		byName[task.Name] = task.Command
	}
	want := map[string]string{
		"build": "go build ./...",
		"test":  "prism -v ./...",
		"dev":   "air",
	}
	if !reflect.DeepEqual(byName, want) {
		t.Errorf("got %v, want %v", byName, want)
	}
}

// TestSpinConfig_Parse_Empty verifies the parser returns an empty
// (but not nil) slice and no error for an empty file. This is the
// "no tasks defined, that's fine" path.
func TestSpinConfig_Parse_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewSpinConfig()
	tasks, err := src.Tasks(dir)
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("got %d tasks, want 0: %+v", len(tasks), tasks)
	}
}

// TestSpinConfig_Parse_Description verifies the inline-table form
// (`{ command = "...", description = "...", env = [...] }`) is
// parsed correctly. The description lands in Task.Notes; env
// lands in Task.Env. This is RUN-14: the spin.config.toml schema
// supports an optional `description` and `env` alongside `command`.
func TestSpinConfig_Parse_Description(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tasks]
dev = { command = "air", description = "hot reload", env = ["DEBUG=1", "LOG=trace"] }
`
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewSpinConfig()
	tasks, err := src.Tasks(dir)
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %+v", len(tasks), tasks)
	}
	task := tasks[0]
	if task.Name != "dev" {
		t.Errorf("Name = %q, want %q", task.Name, "dev")
	}
	if task.Command != "air" {
		t.Errorf("Command = %q, want %q", task.Command, "air")
	}
	if task.Notes != "hot reload" {
		t.Errorf("Notes = %q, want %q", task.Notes, "hot reload")
	}
	wantEnv := []string{"DEBUG=1", "LOG=trace"}
	sort.Strings(task.Env)
	sort.Strings(wantEnv)
	if !reflect.DeepEqual(task.Env, wantEnv) {
		t.Errorf("Env = %v, want %v", task.Env, wantEnv)
	}
}

// TestSpinConfig_Parse_OutOfSection verifies that keys outside
// the [tasks] block are ignored. The parser is intentionally
// strict about section scoping: it does not invent tasks from
// top-level keys like `name = "foo"` (which the loader
// interprets as something else).
func TestSpinConfig_Parse_OutOfSection(t *testing.T) {
	dir := t.TempDir()
	cfg := `name = "my-project"
version = "0.1.0"

[tasks]
build = "go build ./..."

[env]
DEBUG = "1"
`
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewSpinConfig()
	tasks, err := src.Tasks(dir)
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	// Only the [tasks] block should contribute.
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %+v", len(tasks), tasks)
	}
	if tasks[0].Name != "build" {
		t.Errorf("Name = %q, want %q", tasks[0].Name, "build")
	}
	if tasks[0].Command != "go build ./..." {
		t.Errorf("Command = %q, want %q", tasks[0].Command, "go build ./...")
	}
}

// TestSpinConfig_Parse_ShorthandOnly verifies the parser
// supports a task whose value is just a string (the
// most-common form). This is the regression test for the
// hand-rolled parser: the inline-table branch must NOT
// intercept the shorthand form.
func TestSpinConfig_Parse_ShorthandOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tasks]
build = "go build ./..."
`
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewSpinConfig()
	tasks, err := src.Tasks(dir)
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %+v", len(tasks), tasks)
	}
	if tasks[0].Command != "go build ./..." {
		t.Errorf("Command = %q, want %q", tasks[0].Command, "go build ./...")
	}
	if tasks[0].Notes != "" {
		t.Errorf("Notes = %q, want empty (shorthand form has no description)", tasks[0].Notes)
	}
	if len(tasks[0].Env) != 0 {
		t.Errorf("Env = %v, want empty (shorthand form has no env)", tasks[0].Env)
	}
}
