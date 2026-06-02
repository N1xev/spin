package scaffold

import (
	"sort"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// newResolveCmd returns a fresh cobra command with the full Phase 1 + forward-
// compat flag set registered. It mirrors what cmd/new.go does in production;
// the two must stay in sync. ResolveFlags reads flag values from this command.
func newResolveCmd() *cobra.Command {
	c := &cobra.Command{Use: "new", Args: cobra.ExactArgs(1), RunE: func(*cobra.Command, []string) error { return nil }}
	pf := c.PersistentFlags()

	// Phase 1 active
	pf.Bool("tui", false, "TUI variant")
	pf.Bool("cli", false, "CLI variant (Phase 2)")
	pf.Bool("all", false, "TUI + CLI combo (Phase 2)")
	pf.Bool("bubbletea", false, "add bubbletea v2")
	pf.Bool("bubbles", false, "add bubbles v2")
	pf.Bool("lipgloss", false, "add lipgloss v2")
	pf.String("module", "", "module path override")
	pf.String("license", "mit", "license type: mit, apache-2.0, none")
	pf.String("template", "tui-bubbletea", "template name")
	pf.Bool("force", false, "overwrite existing directory")
	pf.Bool("no-git", false, "skip git init")
	pf.Bool("no-verify", false, "skip post-scaffold go build")
	pf.Bool("quiet", false, "minimal output")

	// Forward-compat (Phase 2/3/4) — flag binding only; no template content yet.
	pf.Bool("cobra", false, "add cobra (Phase 2)")
	pf.Bool("fang", false, "add fang (Phase 2)")
	pf.Bool("viper", false, "add viper (Phase 2)")
	pf.Bool("huh", false, "add huh v2 (Phase 2)")
	pf.Bool("glamour", false, "add glamour v2 (Phase 2)")
	pf.Bool("glow", false, "add glow binary (Phase 2)")
	pf.Bool("wish", false, "add wish v2 (Phase 2)")
	pf.Bool("log", false, "add charm log v2 (Phase 2)")
	pf.Bool("harmonica", false, "add harmonica v2 (Phase 2)")
	pf.Bool("modifiers", false, "add x/modifiers (Phase 2)")
	pf.Bool("ansi", false, "add x/ansi (Phase 2)")
	pf.Bool("runewidth", false, "add go-runewidth (Phase 2)")
	pf.Bool("ai", false, "opt in to AGENTS.md (Phase 3)")

	return c
}

// runResolveCmd is a test helper that simulates cobra flag parsing by setting
// args and executing. Returns the *Project produced by ResolveFlags.
func runResolveCmd(t *testing.T, name string, flags ...string) *Project {
	t.Helper()
	c := newResolveCmd()
	c.SetArgs(append([]string{name}, flags...))
	if err := c.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}
	p, err := ResolveFlags(c, []string{name})
	if err != nil {
		t.Fatalf("ResolveFlags: %v", err)
	}
	return p
}

func TestResolveFlags_Default(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--tui", "--bubbletea")

	if p.Name != "myapp" {
		t.Errorf("Name = %q, want %q", p.Name, "myapp")
	}
	if p.Module != "myapp" {
		t.Errorf("Module = %q, want %q (default = Name)", p.Module, "myapp")
	}
	if p.Type != "tui" {
		t.Errorf("Type = %q, want %q", p.Type, "tui")
	}
	if len(p.Libs) != 1 || p.Libs[0] != "bubbletea" {
		t.Errorf("Libs = %v, want [bubbletea]", p.Libs)
	}
	if p.License != "mit" {
		t.Errorf("License = %q, want %q (default)", p.License, "mit")
	}
	if p.Template != "tui-bubbletea" {
		t.Errorf("Template = %q, want %q (default)", p.Template, "tui-bubbletea")
	}
	year := time.Now().Year()
	if p.Year != year {
		t.Errorf("Year = %d, want %d (current year)", p.Year, year)
	}
	if p.SpinVer == "" {
		t.Error("SpinVer is empty; ResolveFlags must populate it from version.Version()")
	}
}

func TestResolveFlags_BubblesImpliesBubbletea(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--tui", "--bubbles")

	// --bubbles alone must produce Libs containing BOTH "bubbles" and "bubbletea".
	if !containsString(p.Libs, "bubbles") {
		t.Errorf("Libs = %v, missing %q", p.Libs, "bubbles")
	}
	if !containsString(p.Libs, "bubbletea") {
		t.Errorf("Libs = %v, missing %q (--bubbles implies --bubbletea)", p.Libs, "bubbletea")
	}
}

func TestResolveFlags_LibsSortedAndDeduped(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--tui", "--lipgloss", "--bubbletea", "--bubbles")

	want := []string{"bubbletea", "bubbles", "lipgloss"}
	if !equalSorted(p.Libs, want) {
		t.Errorf("Libs = %v, want sorted %v", p.Libs, want)
	}
}

func TestResolveFlags_ModuleOverride(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--tui", "--bubbletea", "--module", "github.com/foo/myapp")

	if p.Module != "github.com/foo/myapp" {
		t.Errorf("Module = %q, want %q", p.Module, "github.com/foo/myapp")
	}
	if p.Name != "myapp" {
		t.Errorf("Name = %q, want %q (Name must come from positional, not --module)", p.Name, "myapp")
	}
}

func TestResolveFlags_CLIVariant(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--cli", "--cobra", "--fang")

	if p.Type != "cli" {
		t.Errorf("Type = %q, want %q", p.Type, "cli")
	}
	if !p.Cobra {
		t.Error("Cobra = false, want true")
	}
	if !p.Fang {
		t.Error("Fang = false, want true")
	}
	if len(p.Libs) != 0 {
		t.Errorf("Libs = %v, want [] (no TUI libs with --cli)", p.Libs)
	}
}

func TestResolveFlags_LicenseOverride(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--tui", "--bubbletea", "--license", "apache-2.0")
	if p.License != "apache-2.0" {
		t.Errorf("License = %q, want %q", p.License, "apache-2.0")
	}
}

func TestResolveFlags_AllBoolsBind(t *testing.T) {
	flags := []string{
		"--tui", "--bubbletea",
		"--force", "--no-git", "--no-verify", "--quiet",
		"--cobra", "--fang", "--viper", "--huh", "--glamour", "--glow",
		"--wish", "--log", "--harmonica", "--modifiers", "--ansi",
		"--runewidth", "--ai",
	}
	p := runResolveCmd(t, "myapp", flags...)

	checks := []struct {
		name string
		got  bool
	}{
		{"Force", p.Force},
		{"NoGit", p.NoGit},
		{"NoVerify", p.NoVerify},
		{"Quiet", p.Quiet},
		{"Cobra", p.Cobra},
		{"Fang", p.Fang},
		{"Viper", p.Viper},
		{"Huh", p.Huh},
		{"Glamour", p.Glamour},
		{"Glow", p.Glow},
		{"Wish", p.Wish},
		{"Log", p.Log},
		{"Harmonica", p.Harmonica},
		{"Modifiers", p.Modifiers},
		{"Ansi", p.Ansi},
		{"Runewidth", p.Runewidth},
		{"AI", p.AI},
	}
	for _, c := range checks {
		if !c.got {
			t.Errorf("%s = false, want true (flag must bind to struct field)", c.name)
		}
	}
}

func TestResolveFlags_TemplateDefault(t *testing.T) {
	p := runResolveCmd(t, "myapp", "--tui", "--bubbletea")
	if p.Template != "tui-bubbletea" {
		t.Errorf("Template = %q, want %q", p.Template, "tui-bubbletea")
	}

	// Explicit --template override.
	p2 := runResolveCmd(t, "myapp", "--tui", "--bubbletea", "--template", "custom")
	if p2.Template != "custom" {
		t.Errorf("Template = %q, want %q", p2.Template, "custom")
	}
}

// equalSorted compares two slices after sorting both. Empty and nil are
// treated as equal so callers don't have to worry about slice init state.
func equalSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
