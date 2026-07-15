package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/N1xev/spin/internal/template"
)

// keyPress builds a tea.KeyPressMsg for a single key. "enter" maps to
// the carriage-return code; any other single character uses its rune
// as both Code and Text so String() echoes it back.
func keyPress(s string) tea.KeyPressMsg {
	if s == "enter" {
		return tea.KeyPressMsg{Code: '\r'}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// writeFixtureTemplate creates a minimal template on disk with an inline
// pre-hook, a _pre script file, and an inline post-hook, plus one _base
// file, then loads it.
func writeFixtureTemplate(t *testing.T) *template.Template {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spin.toml"), []byte(`
name = "fixture"
[params]
name = { type = "text", default = "world" }
[[pre]]
run = "echo preparing to scaffold"
[[post]]
run = "echo done post"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "_base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "hello.txt"), []byte("hi {{.name}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	pre := filepath.Join(dir, "_pre")
	if err := os.MkdirAll(pre, 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-executable so the runner uses `sh _pre/hi.sh`.
	if err := os.WriteFile(filepath.Join(pre, "hi.sh"), []byte("echo from-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := template.NewLoader("")
	tpl, err := loader.LoadContext(context.Background(), dir)
	if err != nil {
		t.Fatalf("load fixture template: %v", err)
	}
	return tpl
}

// pump drains streamed output messages until the run completes.
func pump(t *testing.T, m hooksModel, cmd tea.Cmd) hooksModel {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		switch x := msg.(type) {
		case runLineMsg:
			m, cmd = m.update(x)
		case runDoneMsg:
			m, cmd = m.update(x)
			cmd = nil
		default:
			t.Fatalf("unexpected message type %T", msg)
		}
	}
	return m
}

// TestHooksTUI_EnterRunsSelectedHook verifies that pressing Enter on a
// hook runs just that hook, streaming its command + output into the
// right pane, without scaffolding the project or setting didRun.
func TestHooksTUI_EnterRunsSelectedHook(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	dest := t.TempDir()
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), dest, "myapp", false, false, false)

	// Enter runs the selected hook (index 0: inline pre).
	m, cmd := m.update(keyPress("enter"))
	if !m.running {
		t.Fatal("expected running after enter")
	}
	if m.runningAll {
		t.Fatal("single hook run must not be flagged runningAll")
	}
	if m.didRun {
		t.Fatal("single hook run must not set didRun")
	}
	m = pump(t, m, cmd)
	if m.running {
		t.Fatal("expected running false after completion")
	}
	if !strings.Contains(m.output, "pre-hook") {
		t.Fatalf("expected streamed output to include pre-hook lines, got:\n%s", m.output)
	}
	// A single hook run does not render _base files.
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); !os.IsNotExist(err) {
		t.Error("single hook run must not scaffold _base files")
	}
}

// TestHooksTUI_EnterRunsFileHook verifies that a _pre/_post script file
// hook is executed in place, with its asset copied into dest first.
func TestHooksTUI_EnterRunsFileHook(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	dest := t.TempDir()
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), dest, "myapp", false, false, true)

	// Hook order: [inline pre(0), _pre/hi.sh(1), inline post(2)].
	const fileIdx = 1
	// Select the file hook in the list.
	m.list.Select(fileIdx)
	if m.list.Index() != fileIdx {
		t.Fatalf("expected list index %d, got %d", fileIdx, m.list.Index())
	}
	m, cmd := m.update(keyPress("enter"))
	m = pump(t, m, cmd)
	if !strings.Contains(m.output, "from-file") {
		t.Fatalf("expected file hook output, got:\n%s", m.output)
	}
	if _, err := os.Stat(filepath.Join(dest, "_pre", "hi.sh")); err != nil {
		t.Errorf("expected _pre/hi.sh copied into dest: %v", err)
	}
}

// TestHooksTUI_EnterRunsFileHookNoVerbose verifies that without
// --verbose only the echoed command is streamed, not the command's
// stdout. The script prints "from-file"; in non-verbose mode that must
// not appear in the pane.
func TestHooksTUI_EnterRunsFileHookNoVerbose(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	dest := t.TempDir()
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), dest, "myapp", false, false, false)

	const fileIdx = 1
	m.list.Select(fileIdx)
	m, cmd := m.update(keyPress("enter"))
	m = pump(t, m, cmd)
	if !strings.Contains(m.output, "→ pre-hook: sh _pre/hi.sh") {
		t.Fatalf("expected echoed command in pane, got:\n%s", m.output)
	}
	if strings.Contains(m.output, "from-file") {
		t.Fatalf("expected command output suppressed without --verbose, got:\n%s", m.output)
	}
}

// TestHooksTUI_SelectedHookContent verifies that while reviewing
// (before running) the right pane shows the SELECTED hook's content:
// the inline command for [[pre]]/[[post]] steps, or the file
// contents of a _pre/_post script. Switching the list selection
// switches the pane content.
func TestHooksTUI_SelectedHookContent(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), t.TempDir(), "myapp", false, false, false)

	// Hook order: [inline pre(0), _pre/hi.sh(1), inline post(2)].
	// Index 0 is an inline pre command.
	m.list.Select(0)
	inline := m.selectedHookContent(m.list.Index())
	if !strings.Contains(inline, "echo preparing to scaffold") {
		t.Fatalf("expected inline command in pane, got:\n%s", inline)
	}
	if strings.Contains(inline, "from-file") {
		t.Fatalf("inline pane must not show the file's body, got:\n%s", inline)
	}

	// Index 1 is the _pre/hi.sh script file; the pane shows its body.
	m.list.Select(1)
	file := m.selectedHookContent(m.list.Index())
	if !strings.Contains(file, "from-file") {
		t.Fatalf("expected script file body in pane, got:\n%s", file)
	}
	if !strings.Contains(file, "_pre/hi.sh") {
		t.Fatalf("expected script path in pane, got:\n%s", file)
	}

	// Navigating the list updates the pane (back to inline post).
	m.list.Select(2)
	post := m.selectedHookContent(m.list.Index())
	if !strings.Contains(post, "echo done post") {
		t.Fatalf("expected inline post command in pane, got:\n%s", post)
	}
}

// TestHooksTUI_ModalStateMachine verifies the run-all modal (opened with
// R) toggles and closes without running.
func TestHooksTUI_ModalStateMachine(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), t.TempDir(), "myapp", false, false, false)

	if m.modalOpen {
		t.Fatal("modal should start closed")
	}
	// R opens the run-all modal.
	m, _ = m.update(keyPress("R"))
	if !m.modalOpen {
		t.Fatal("expected modal to open on R")
	}
	if m.modalChoice != 0 {
		t.Fatalf("expected default choice Run (0), got %d", m.modalChoice)
	}
	m, _ = m.update(keyPress("l"))
	if m.modalChoice != 1 {
		t.Fatalf("expected Skip (1) after 'l', got %d", m.modalChoice)
	}
	m, _ = m.update(keyPress("h"))
	if m.modalChoice != 0 {
		t.Fatalf("expected Run (0) after 'h', got %d", m.modalChoice)
	}
	m, _ = m.update(keyPress("esc"))
	if m.modalOpen {
		t.Fatal("expected modal to close on esc")
	}
	if m.didRun {
		t.Fatal("esc must not trigger a run")
	}
}

// TestHooksTUI_RunAllScaffolds verifies that submitting the modal runs
// the full scaffold and sets didRun so runNew skips its own render.
func TestHooksTUI_RunAllScaffolds(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	dest := t.TempDir()
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), dest, "myapp", false, false, false)

	// R opens modal, enter submits (Run) -> full scaffold.
	m, _ = m.update(keyPress("R"))
	m, cmd := m.update(keyPress("enter"))
	if !m.didRun {
		t.Fatal("expected didRun after submit")
	}
	if !m.runningAll {
		t.Fatal("expected runningAll for full scaffold")
	}
	m = pump(t, m, cmd)
	if m.running {
		t.Fatal("expected running false after completion")
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt rendered by full scaffold: %v", err)
	}
	// After a full scaffold the right pane mirrors the success
	// summary (same two lines runNew prints after q), and the model
	// stays open for the user to quit manually.
	if !strings.Contains(m.output, "done.") {
		t.Fatalf("expected done. in pane, got:\n%s", m.output)
	}
	if !strings.Contains(m.output, "INFO created") {
		t.Fatalf("expected INFO created line in pane, got:\n%s", m.output)
	}
	if !strings.Contains(m.output, "cd "+dest) {
		t.Fatalf("expected cd hint in pane, got:\n%s", m.output)
	}
}

// TestHooksTUI_ModalRendersCanvas verifies the Run/Skip modal renders
// (via canvas + layered composition) without panicking and shows the
// template name, the hook summary, and the Run/Skip choices.
func TestHooksTUI_ModalRendersCanvas(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), t.TempDir(), "fixture", false, false, false)

	m, _ = m.update(keyPress("R"))
	if !m.modalOpen {
		t.Fatal("expected modal open")
	}
	out := m.view().Content
	for _, want := range []string{"fixture", "hooks?", "Run", "Skip", "echo preparing to scaffold"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected modal to contain %q, got:\n%s", want, out)
		}
	}
}

// TestHooksTUI_SkipRunsScaffoldWithoutHooks verifies that choosing Skip
// still scaffolds the project but disables hook execution.
func TestHooksTUI_SkipRunsScaffoldWithoutHooks(t *testing.T) {
	tpl := writeFixtureTemplate(t)
	dest := t.TempDir()
	resolved := map[string]any{"name": "myapp", "project_name": "myapp"}
	m := newHooksModel(tpl, newTUIStyles(), 100, 30, resolved, context.Background(), dest, "myapp", false, false, false)

	m, _ = m.update(keyPress("R"))
	m, cmd := m.update(keyPress("n")) // choose Skip in modal
	if m.modalChoice != 1 {
		t.Fatalf("expected Skip (1) after 'n', got %d", m.modalChoice)
	}
	if !m.didRun {
		t.Fatal("expected didRun after skip submit")
	}
	m = pump(t, m, cmd)
	if strings.Contains(m.output, "pre-hook") {
		t.Fatalf("skip must not run hooks, but output contained pre-hook:\n%s", m.output)
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt rendered even when skipping hooks: %v", err)
	}
}
