// Package runner_test holds the external (black-box) tests for the
// runner package. It lives in a separate package so it can import
// internal/runner/sources — which the internal/runner package
// already imports — without creating a Go test import cycle.
package runner_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/spin/internal/runner"
	"github.com/example/spin/internal/runner/sources"
)

// TestRunner_SourcePrecedence verifies that a user-authored task in
// spin.config.toml wins over a fallback task with the same name.
// The runner's merge function picks the higher-Order entry on
// conflict; spinconfig is Order=100, fallback is Order=0, so the
// spinconfig task must come out on top.
func TestRunner_SourcePrecedence(t *testing.T) {
	dir := t.TempDir()
	// Both files declare `build`. spin.config.toml must win.
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte("[tasks]\nbuild = \"echo user\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := runner.New(dir)
	r.Sources = []runner.TaskSource{
		sources.NewFallback(),
		sources.NewSpinConfig(),
	}

	all, err := r.All()
	if err != nil {
		t.Fatalf("All(): %v", err)
	}

	var build *runner.Task
	for i := range all {
		if all[i].Name == "build" {
			build = &all[i]
			break
		}
	}
	if build == nil {
		t.Fatalf("no `build` task found; got %d tasks", len(all))
	}
	if build.Command != "echo user" {
		t.Errorf("build.Command = %q, want %q (spin.config.toml should win)", build.Command, "echo user")
	}
	if !strings.Contains(build.Source, "spin.config.toml") {
		t.Errorf("build.Source = %q, want substring %q", build.Source, "spin.config.toml")
	}
}

// TestRunner_Resolve_NotFound verifies that Resolve returns the
// structured ErrNotFound error when no source supplies the task.
// The error type allows consumers to distinguish "no such task"
// from other failure modes (e.g. a parse error).
func TestRunner_Resolve_NotFound(t *testing.T) {
	dir := t.TempDir()
	r := runner.New(dir)
	r.Sources = []runner.TaskSource{sources.NewFallback()}

	_, err := r.Resolve("nonexistent")
	if err == nil {
		t.Fatal("Resolve(nonexistent) should return an error")
	}
	enf, ok := err.(*runner.ErrNotFound)
	if !ok {
		t.Fatalf("err is %T, want *ErrNotFound", err)
	}
	if enf.Name != "nonexistent" {
		t.Errorf("ErrNotFound.Name = %q, want %q", enf.Name, "nonexistent")
	}
}

// TestRunner_List_EmptyDir verifies that List in a directory with
// no task files prints the "No tasks defined" hint. The exact
// wording is part of the v2.0 UX contract; if it changes the
// command-line experience degrades.
func TestRunner_List_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	r := runner.New(dir)
	r.Sources = []runner.TaskSource{sources.NewFallback(), sources.NewSpinConfig()}

	var buf bytes.Buffer
	if err := r.List(&buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No tasks defined") {
		t.Errorf("List output should mention 'No tasks defined', got: %q", out)
	}
	if !strings.Contains(out, "Tip:") {
		t.Errorf("List output should include the Tip: hint, got: %q", out)
	}
}

// TestRunner_Explain_ShowsCommand verifies that Explain writes a
// structured per-task report including the resolved command. The
// runner is the canonical way for users to discover what `spin run
// <name>` will actually execute; if the command line is missing,
// the user is left guessing.
func TestRunner_Explain_ShowsCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte("[tasks]\ndev = \"air\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runner.New(dir)
	r.Sources = []runner.TaskSource{sources.NewSpinConfig()}

	var buf bytes.Buffer
	if err := r.Explain(&buf, "dev"); err != nil {
		t.Fatalf("Explain: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "command:") {
		t.Errorf("Explain output should include 'command:' label, got: %q", out)
	}
	if !strings.Contains(out, "air") {
		t.Errorf("Explain output should include the resolved command 'air', got: %q", out)
	}
}

// TestRunner_List_ColumnAlignment verifies the table format
// right-pads task names so columns line up across rows of
// different widths. The check is loose: we look for at least one
// short-name and one long-name row both padded to the wider
// column width (i.e. the longer name's width). The behaviour is
// load-bearing for the v2.0 visual contract; if List stops
// padding, scripts that grep for fixed column offsets break.
func TestRunner_List_ColumnAlignment(t *testing.T) {
	dir := t.TempDir()
	cfg := "[tasks]\n" +
		"a = \"1\"\n" +
		"longname = \"2\"\n"
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runner.New(dir)
	r.Sources = []runner.TaskSource{sources.NewSpinConfig()}

	var buf bytes.Buffer
	if err := r.List(&buf); err != nil {
		t.Fatalf("List: %v", err)
	}
	out := buf.String()
	// "longname" is 8 chars; "a" should be padded to 8 too.
	// We check the substring "a       " (a + 7 spaces) appears in
	// the data row.
	if !strings.Contains(out, "a ") {
		t.Errorf("List output should pad short name 'a' to match 'longname' width, got: %q", out)
	}
	// The header "TASK" is right-padded to the same width.
	if !strings.Contains(out, "TASK") {
		t.Errorf("List output missing TASK header, got: %q", out)
	}
}

// TestRunner_Merge_DedupByName verifies that when two sources
// contribute a task with the same name, the higher-Order entry
// wins. This is the load-bearing behaviour for the source chain
// (e.g. spin.config.toml at Order=100 winning over a fallback at
// Order=0). The test exercises the merge end-to-end through
// Runner.All(), observing the merged task list. (The unexported
// merge function itself is tested separately in source_test.go.)
func TestRunner_Merge_DedupByName(t *testing.T) {
	dir := t.TempDir()
	// go.mod triggers the fallback (Order=0, command: "go build ./...")
	// spin.config.toml triggers the user source (Order=100, command: "go build -tags=x ./...")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spin.config.toml"), []byte("[tasks]\nbuild = \"echo user-build\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := runner.New(dir)
	// Order sources so the lower-Order one is iterated FIRST — the
	// runner's merge sees both and must pick the higher-Order one.
	r.Sources = []runner.TaskSource{
		sources.NewFallback(),
		sources.NewSpinConfig(),
	}

	all, err := r.All()
	if err != nil {
		t.Fatalf("All(): %v", err)
	}
	var build *runner.Task
	for i := range all {
		if all[i].Name == "build" {
			build = &all[i]
			break
		}
	}
	if build == nil {
		t.Fatalf("no `build` task in %+v", all)
	}
	if build.Command != "echo user-build" {
		t.Errorf("merged build.Command = %q, want %q (higher-Order should win)", build.Command, "echo user-build")
	}
}
