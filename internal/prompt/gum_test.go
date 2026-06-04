// White-box tests for the gum shell-out backend.
//
// These tests verify the gum widget wrappers pass the correct
// arguments to deps.Runner. The runner is supplied via the Deps
// struct (no package-level mutable seam), so no real os/exec call
// is made. The test runner has no TTY and no gum binary; Deps is
// the only way the package's subprocess machinery gets exercised.
//
// Tests build a Deps with a stub Runner and call fillWithGumDeps /
// gumChoose / gumMultiSelect / gumInput / gumConfirm directly. The
// Runner stub records the arg list and returns a caller-supplied
// canned answer. No global state is touched, so t.Parallel() is
// safe (these tests don't use it today, but the structure permits it).

package prompt

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/example/spin/internal/scaffold"
)

// gumCall is one recorded invocation of the Runner stub. `name`
// is the first arg (the gum subcommand: "choose", "input",
// "confirm"); `args` is the full arg list including the name.
type gumCall struct {
	name string
	args []string
}

// captureRunner returns a Deps with a Runner that records every
// call's args into *calls and returns the supplied answer/err.
// The returned Deps has production-equivalent LookPath and
// VersionCheck so tests that exercise the full fillWithGumDeps
// path don't trip on a nil LookPath.
func captureRunner(calls *[]gumCall, answer string, err error) Deps {
	return Deps{
		LookPath:     func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error { return nil },
		Runner: func(ctx context.Context, args ...string) (string, error) {
			if len(args) == 0 {
				// Match the production gumRunCapture guard.
				return "", errors.New("gum: no subcommand")
			}
			// Defensive copy so the test's expectation isn't affected
			// by the stub mutating the args slice.
			cp := append([]string(nil), args...)
			*calls = append(*calls, gumCall{name: args[0], args: cp})
			return answer, err
		},
	}
}

// TestGumChoose_Args asserts gumChoose builds the canonical arg
// list: `choose --header H --selected N a b c` where N is 1-based
// (gum's documented convention; the wrapper passes defaultIdx+1).
func TestGumChoose_Args(t *testing.T) {
	var calls []gumCall
	deps := captureRunner(&calls, "stub", nil)
	if _, err := gumChoose(context.Background(), deps, "Pick one", []string{"a", "b", "c"}, 0); err != nil {
		t.Fatalf("gumChoose: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("Runner called %d times, want 1", len(calls))
	}
	want := []string{"choose", "--header", "Pick one", "--selected", "1", "a", "b", "c"}
	if !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("gumChoose args = %v, want %v", calls[0].args, want)
	}
}

// TestGumChoose_DefaultIndex asserts gumChoose translates the
// 0-based Go defaultIdx to gum's 1-based --selected.
func TestGumChoose_DefaultIndex(t *testing.T) {
	var calls []gumCall
	deps := captureRunner(&calls, "stub", nil)
	if _, err := gumChoose(context.Background(), deps, "Pick one", []string{"a", "b", "c"}, 1); err != nil {
		t.Fatalf("gumChoose: %v", err)
	}
	want := []string{"choose", "--header", "Pick one", "--selected", "2", "a", "b", "c"}
	if !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("gumChoose args = %v, want %v (--selected must be 1-based)", calls[0].args, want)
	}
}

// TestGumMultiSelect_Args asserts gumMultiSelect builds the
// canonical arg list: `choose --no-limit --header H a b c` with
// the options as positional args after the header.
func TestGumMultiSelect_Args(t *testing.T) {
	var calls []gumCall
	// Stub returns "a\nb" so the wrapper splits and returns ["a","b"].
	deps := captureRunner(&calls, "a\nb", nil)
	got, err := gumMultiSelect(context.Background(), deps, "Pick libs", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("gumMultiSelect: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("gumMultiSelect result = %v, want [a b]", got)
	}
	want := []string{"choose", "--no-limit", "--header", "Pick libs", "a", "b", "c"}
	if !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("gumMultiSelect args = %v, want %v", calls[0].args, want)
	}
}

// TestGumMultiSelect_EmptyReturnsNil asserts that an empty stdout
// (the user confirmed with no selection) returns nil, not an empty
// slice.
func TestGumMultiSelect_EmptyReturnsNil(t *testing.T) {
	var calls []gumCall
	deps := captureRunner(&calls, "", nil)
	got, err := gumMultiSelect(context.Background(), deps, "Pick libs", []string{"a", "b"})
	if err != nil {
		t.Fatalf("gumMultiSelect: %v", err)
	}
	if got != nil {
		t.Errorf("gumMultiSelect(empty) = %v, want nil", got)
	}
}

// TestGumInput_Args asserts gumInput builds the canonical arg
// list: `input --header H --placeholder P [--value V]`.
func TestGumInput_Args(t *testing.T) {
	cases := []struct {
		name         string
		header       string
		placeholder  string
		defaultValue string
		want         []string
	}{
		{
			name:         "with default",
			header:       "Project name",
			placeholder:  "myapp",
			defaultValue: "preset",
			want:         []string{"input", "--header", "Project name", "--placeholder", "myapp", "--value", "preset"},
		},
		{
			name:         "no default",
			header:       "Project name",
			placeholder:  "myapp",
			defaultValue: "",
			want:         []string{"input", "--header", "Project name", "--placeholder", "myapp"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls []gumCall
			deps := captureRunner(&calls, "stub", nil)
			if _, err := gumInput(context.Background(), deps, c.header, c.placeholder, c.defaultValue); err != nil {
				t.Fatalf("gumInput: %v", err)
			}
			if !reflect.DeepEqual(calls[0].args, c.want) {
				t.Errorf("gumInput args = %v, want %v", calls[0].args, c.want)
			}
		})
	}
}

// TestGumConfirm_Args asserts gumConfirm builds the canonical arg
// list: `confirm --default=<bool> <prompt>`. Also asserts the bool
// return maps "Yes" → true, "No" → false.
func TestGumConfirm_Args(t *testing.T) {
	cases := []struct {
		name       string
		defaultYes bool
		prompt     string
		answer     string
		wantArg    string
		wantBool   bool
	}{
		{
			name:       "default true / answer Yes",
			defaultYes: true,
			prompt:     "Deploy?",
			answer:     "Yes",
			wantArg:    "--default=true",
			wantBool:   true,
		},
		{
			name:       "default false / answer No",
			defaultYes: false,
			prompt:     "Deploy?",
			answer:     "No",
			wantArg:    "--default=false",
			wantBool:   false,
		},
		{
			name:       "default true / answer No",
			defaultYes: true,
			prompt:     "Deploy?",
			answer:     "No",
			wantArg:    "--default=true",
			wantBool:   false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls []gumCall
			deps := captureRunner(&calls, c.answer, nil)
			got, err := gumConfirm(context.Background(), deps, c.prompt, c.defaultYes)
			if err != nil {
				t.Fatalf("gumConfirm: %v", err)
			}
			if got != c.wantBool {
				t.Errorf("gumConfirm bool = %v, want %v (answer=%q)", got, c.wantBool, c.answer)
			}
			// First arg is the subcommand; second is the --default flag.
			if len(calls[0].args) < 2 || calls[0].args[0] != "confirm" || calls[0].args[1] != c.wantArg {
				t.Errorf("gumConfirm args[0:2] = %v, want [confirm %s]", calls[0].args[:2], c.wantArg)
			}
			if calls[0].args[len(calls[0].args)-1] != c.prompt {
				t.Errorf("gumConfirm last arg = %q, want %q", calls[0].args[len(calls[0].args)-1], c.prompt)
			}
		})
	}
}

// TestGumConfirm_CanceledPropagation asserts that a *Canceled
// error from Runner propagates through gumConfirm unchanged so
// the per-step askGumXxx wrappers can wrap it with a specific
// "user canceled at <step>" reason.
func TestGumConfirm_CanceledPropagation(t *testing.T) {
	var calls []gumCall
	want := &Canceled{Reason: "test cancel"}
	deps := captureRunner(&calls, "", want)
	_, err := gumConfirm(context.Background(), deps, "Deploy?", true)
	if err != want {
		t.Errorf("gumConfirm err = %v, want %v (same *Canceled instance)", err, want)
	}
}

// sequenceDeps returns a Deps with a Runner that returns successive
// answers from the supplied slice. After the slice is exhausted,
// the Runner fails the test (a clear signal that the call count
// didn't match the plan). The Runner also records every call's
// args so the test can assert arg construction if desired.
func sequenceDeps(t *testing.T, answers []string) (Deps, *[]gumCall) {
	t.Helper()
	var calls []gumCall
	var i int
	deps := Deps{
		LookPath:     func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error { return nil },
		Runner: func(ctx context.Context, args ...string) (string, error) {
			if i >= len(answers) {
				return "", fmt.Errorf("Runner called %d times, only %d answers stubbed", i+1, len(answers))
			}
			cp := append([]string(nil), args...)
			calls = append(calls, gumCall{name: args[0], args: cp})
			ans := answers[i]
			i++
			return ans, nil
		},
	}
	return deps, &calls
}

// TestFillWithGum_WritesBackToProject is the load-bearing test for
// fillWithGum: it stubs the Runner to return the strings
// fillWithGum expects for each of the 8 steps and asserts the
// resulting *scaffold.Project has the correct field values. This
// proves the write-back is correct end-to-end without spawning
// a single real gum subprocess.
func TestFillWithGum_WritesBackToProject(t *testing.T) {
	// 8 answers, one per step, in order. Each answer is the gum
	// "user-facing" string the wrapper maps back to the machine key.
	answers := []string{
		"TUI — terminal app with bubbletea", // askType → "tui"
		"myapp",                             // askName → "myapp"
		"github.com/example/myapp",          // askModule → "github.com/example/myapp"
		"Bubble Tea\nBubbles",               // askLibs → ["bubbletea","bubbles"]
		"MIT",                               // askLicense → "mit"
		"Bubble Tea hello world",            // askTemplate → "tui-bubbletea"
		"",                                  // askTemplateRepo → skip (empty)
		"Yes",                               // askAI → true
	}
	deps, _ := sequenceDeps(t, answers)

	p := &scaffold.Project{}
	if err := fillWithGumDeps(p, deps); err != nil {
		t.Fatalf("fillWithGumDeps: %v", err)
	}
	if p.Type != "tui" {
		t.Errorf("p.Type = %q, want %q", p.Type, "tui")
	}
	if p.Name != "myapp" {
		t.Errorf("p.Name = %q, want %q", p.Name, "myapp")
	}
	if p.Module != "github.com/example/myapp" {
		t.Errorf("p.Module = %q, want %q", p.Module, "github.com/example/myapp")
	}
	wantLibs := []string{"bubbles", "bubbletea"} // sorted by askGumLibs
	if !reflect.DeepEqual(p.Libs, wantLibs) {
		t.Errorf("p.Libs = %v, want %v", p.Libs, wantLibs)
	}
	if p.License != "mit" {
		t.Errorf("p.License = %q, want %q", p.License, "mit")
	}
	if p.Template != "tui-bubbletea" {
		t.Errorf("p.Template = %q, want %q", p.Template, "tui-bubbletea")
	}
	if p.TemplateRepo != "" {
		t.Errorf("p.TemplateRepo = %q, want empty (skipped)", p.TemplateRepo)
	}
	if !p.AI {
		t.Error("p.AI = false, want true (answer was 'Yes')")
	}
}

// TestFillWithGum_SkipsSetFields asserts that fillWithGum respects
// the "field already set by flag" skip predicate for each step.
// A pre-populated Project must come out of fillWithGum unchanged
// (no Runner calls) — the flags were the source of truth.
func TestFillWithGum_SkipsSetFields(t *testing.T) {
	var calls int
	deps := Deps{
		LookPath:     func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error { return nil },
		Runner: func(ctx context.Context, args ...string) (string, error) {
			calls++
			return "", nil
		},
	}

	// Template is set to a NON-default value so askGumTemplate's
	// skip predicate (`Template != "tui-bubbletea"`) fires.
	//
	// License is set to "apache-2.0" (non-default) so askGumLicense's
	// skip predicate (`License != "mit"`) fires. Setting License="mit"
	// re-asks (the user must be able to confirm the default), so the
	// test would have an extra Runner call.
	p := &scaffold.Project{
		Type:         "tui",
		Name:         "myapp",
		Module:       "github.com/example/myapp",
		License:      "apache-2.0",
		Template:     "cli-cobra-fang", // non-default → askGumTemplate skips
		TemplateRepo: "https://github.com/example/template",
		// AI: false (always asked)
		// Libs: nil (always asked)
	}
	if err := fillWithGumDeps(p, deps); err != nil {
		t.Fatalf("fillWithGumDeps: %v", err)
	}
	// The skip predicate covers Type, Name, Module, License, Template,
	// TemplateRepo. The always-asked steps are Libs and AI. So we
	// expect exactly 2 Runner calls.
	if calls != 2 {
		t.Errorf("Runner called %d times, want 2 (only Libs + AI should fire)", calls)
	}
	// Verify the un-set fields are still at their zero values
	// (we only stubbed the Runner to return ""; the dispatcher
	// would interpret "" as a user cancellation or empty pick,
	// but here we expect the skip predicates to short-circuit
	// all but 2 steps).
	if p.Type != "tui" {
		t.Errorf("p.Type mutated to %q", p.Type)
	}
	if p.Name != "myapp" {
		t.Errorf("p.Name mutated to %q", p.Name)
	}
	if p.Module != "github.com/example/myapp" {
		t.Errorf("p.Module mutated to %q", p.Module)
	}
	if p.License != "apache-2.0" {
		t.Errorf("p.License mutated to %q", p.License)
	}
	if p.Template != "cli-cobra-fang" {
		t.Errorf("p.Template mutated to %q", p.Template)
	}
	if p.TemplateRepo != "https://github.com/example/template" {
		t.Errorf("p.TemplateRepo mutated to %q", p.TemplateRepo)
	}
}

// TestFillWithGum_CancelPropagates asserts that a *Canceled error
// from a stubbed Runner propagates up to fillWithGumDeps wrapped
// with a step-specific reason. The caller's errors.As(*Canceled)
// check can still match it via the Is method and map to exit 130.
func TestFillWithGum_CancelPropagates(t *testing.T) {
	inner := &Canceled{Reason: "inner test cancel"}
	deps := Deps{
		LookPath:     func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error { return nil },
		Runner: func(ctx context.Context, args ...string) (string, error) {
			return "", inner
		},
	}

	p := &scaffold.Project{}
	err := fillWithGumDeps(p, deps)
	if err == nil {
		t.Fatal("fillWithGumDeps err = nil, want *Canceled")
	}
	var c *Canceled
	if !errors.As(err, &c) {
		t.Errorf("fillWithGumDeps err = %v, want *Canceled (matchable via errors.As)", err)
	}
	if c != nil && c.Reason == "" {
		t.Errorf("fillWithGumDeps *Canceled Reason = empty, want non-empty (step-specific)")
	}
}

// TestTemplateOptionsForType_Variants asserts the template options
// for each variant.
func TestTemplateOptionsForType_Variants(t *testing.T) {
	cases := []struct {
		variant string
		want    []string // expected keys
	}{
		{"tui", []string{"tui-bubbletea"}},
		{"cli", []string{"cli-cobra-fang"}},
		{"all", []string{"tui-bubbletea", "cli-cobra-fang"}},
		{"", nil},
		{"unknown", nil},
	}
	for _, c := range cases {
		t.Run(c.variant, func(t *testing.T) {
			opts := templateOptionsForType(c.variant)
			got := make([]string, len(opts))
			for i, o := range opts {
				got[i] = o.key
			}
			gSorted := append([]string(nil), got...)
			sort.Strings(gSorted)
			wSorted := append([]string(nil), c.want...)
			sort.Strings(wSorted)
			if !reflect.DeepEqual(gSorted, wSorted) {
				t.Errorf("templateOptionsForType(%q) keys = %v, want %v", c.variant, got, c.want)
			}
		})
	}
}

// TestTypeDisplayToKey_AllLabels asserts the reverse-lookup map
// covers all three labels exactly. A regression in the labels
// (a typo, a missing entry) would silently default to "" in
// askGumType and surface as a confusing "ask type: unexpected
// answer" error.
func TestTypeDisplayToKey_AllLabels(t *testing.T) {
	wantKeys := map[string]string{
		"TUI — terminal app with bubbletea":          "tui",
		"CLI — command-line tool with cobra + fang":  "cli",
		"TUI + CLI — single binary with both halves": "all",
	}
	for label, wantKey := range wantKeys {
		gotKey, ok := typeDisplayToKey[label]
		if !ok {
			t.Errorf("typeDisplayToKey missing label %q", label)
			continue
		}
		if gotKey != wantKey {
			t.Errorf("typeDisplayToKey[%q] = %q, want %q", label, gotKey, wantKey)
		}
	}
	if len(typeDisplayToKey) != len(wantKeys) {
		t.Errorf("typeDisplayToKey has %d entries, want %d (no stale labels allowed)", len(typeDisplayToKey), len(wantKeys))
	}
}

// TestIsCanceled_AffectsOnlyCanceledErrors is a sanity check on
// the isCanceled helper used by every askGumXxx wrapper.
func TestIsCanceled_AffectsOnlyCanceledErrors(t *testing.T) {
	if isCanceled(nil) {
		t.Error("isCanceled(nil) = true, want false")
	}
	if !isCanceled(&Canceled{Reason: "x"}) {
		t.Error("isCanceled(*Canceled{}) = false, want true")
	}
	if isCanceled(errors.New("plain error")) {
		t.Error("isCanceled(plain error) = true, want false")
	}
}
