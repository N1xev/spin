package update

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// stubProxy satisfies ModuleProxy with a fixed body and is used by
// the non-TTY and empty-deps tests.
type stubProxy struct {
	body []string
}

func (s *stubProxy) ListVersions(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), s.body...), nil
}

// withNonTTYForced swaps the package's isTerminalFunc for a stub
// that always returns false, restoring the original on test exit.
// The override is what makes the non-TTY path unit-testable without
// the syscall.Dup2 gymnastics that would leak across tests.
func withNonTTYForced(t *testing.T) {
	t.Helper()
	orig := isTerminalFunc
	isTerminalFunc = func(_ uintptr) bool { return false }
	t.Cleanup(func() { isTerminalFunc = orig })
}

// writeGoModAt writes content to dir/go.mod and returns the path.
// (parse_test.go's writeGoMod uses its own t.TempDir; here we need
// a caller-chosen directory so the test owns the cleanup path.)
func writeGoModAt(t *testing.T, dir, content string) string {
	t.Helper()
	path := dir + "/go.mod"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	return path
}

func TestPromptForUpdate_NonTTY_PrintsTable_NoHang(t *testing.T) {
	withNonTTYForced(t)

	dir := t.TempDir()
	writeGoModAt(t, dir, fixtureGoMod)

	ctx, cancel := context.WithTimeout(context.Background(), 1_000_000_000) // 1s
	defer cancel()

	var out, log bytes.Buffer
	err := PromptForUpdate(ctx, PromptOptions{
		GoModPath: dir + "/go.mod",
		Proxy:     &stubProxy{body: []string{"v1.0.0", "v1.1.0", "v1.2.0"}},
		Out:       &out,
		Log:       &log,
	})
	if err == nil {
		t.Fatal("expected error from non-TTY prompt, got nil")
	}
	if !strings.Contains(err.Error(), "not a TTY") {
		t.Errorf("error should mention not a TTY; got: %v", err)
	}
	for _, want := range []string{"MODULE", "OLD", "STABLE", "LATEST",
		"charm.land/bubbletea/v2",
		"charm.land/lipgloss/v2",
		"github.com/spf13/cobra"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output table missing %q; got:\n%s", want, out.String())
		}
	}
}

func TestPromptForUpdate_EmptyDeps_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	emptyMod := "module example.com/empty\n\ngo 1.23\n"
	writeGoModAt(t, dir, emptyMod)

	var log bytes.Buffer
	err := PromptForUpdate(context.Background(), PromptOptions{
		GoModPath: dir + "/go.mod",
		Proxy:     &stubProxy{},
		Log:       &log,
	})
	if err != nil {
		t.Fatalf("expected nil for empty deps, got: %v", err)
	}
	if !strings.Contains(log.String(), "no deps to update") {
		t.Errorf("log should mention 'no deps to update'; got: %q", log.String())
	}
}

func TestPromptForUpdate_BuildsFormForEachDep(t *testing.T) {
	deps := []Dep{
		{Module: "example.com/a", Old: "v1.0.0", NewStable: "v1.1.0", NewLatest: "v1.2.0-beta.1"},
		{Module: "example.com/b", Old: "v0.5.0", NewStable: "v0.5.0", NewLatest: "v0.5.0"},
		{Module: "example.com/c", Old: "v2.0.0", NewStable: "v2.0.0", NewLatest: "v2.0.0"},
	}
	selects, choices := buildSelects(deps)
	if len(selects) != len(deps) {
		t.Errorf("len(selects) = %d, want %d", len(selects), len(deps))
	}
	if len(choices) != len(deps) {
		t.Errorf("len(choices) = %d, want %d", len(choices), len(deps))
	}
	for _, d := range deps {
		if _, ok := choices[d.Module]; !ok {
			t.Errorf("choices missing key %q", d.Module)
		}
	}
	form := formFromSelects(selects)
	if form == nil {
		t.Fatal("formFromSelects returned nil")
	}
}

func TestPrintNonTTYTable_FormatsCurrentAsCurrent(t *testing.T) {
	deps := []Dep{
		{Module: "example.com/unchanged", Old: "v1.0.0", NewStable: "v1.0.0", NewLatest: "v1.0.0"},
	}
	var buf bytes.Buffer
	if err := printNonTTYTable(deps, &buf); err != nil {
		t.Fatalf("printNonTTYTable: %v", err)
	}
	out := buf.String()
	if c := strings.Count(out, "(current)"); c < 2 {
		t.Errorf("expected at least 2 occurrences of '(current)'; got %d in:\n%s", c, out)
	}
	if !strings.Contains(out, "MODULE") || !strings.Contains(out, "STABLE") {
		t.Errorf("table header missing; got:\n%s", out)
	}
}

func TestPrintNonTTYTable_TruncatesLongModuleNames(t *testing.T) {
	longName := strings.Repeat("a", 80)
	deps := []Dep{
		{Module: longName, Old: "v1.0.0", NewStable: "v1.0.0", NewLatest: "v1.0.0"},
	}
	var buf bytes.Buffer
	if err := printNonTTYTable(deps, &buf); err != nil {
		t.Fatalf("printNonTTYTable: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines; got %d:\n%s", len(lines), buf.String())
	}
	row := lines[len(lines)-1]
	if len(row) > 110 {
		t.Errorf("row width = %d, expected <= 110; got %q", len(row), row)
	}
	if !strings.Contains(row, "...") {
		t.Errorf("expected truncated module to contain '...'; got %q", row)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 50, "short"},
		{strings.Repeat("x", 60), 50, strings.Repeat("x", 49) + "..."},
		{"a", 1, "a"},
		{"ab", 1, "a"},
	}
	for _, tc := range cases {
		got := truncate(tc.in, tc.max)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}
