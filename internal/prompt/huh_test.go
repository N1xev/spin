// White-box tests for the huh v2 form backend.
//
// These tests assert the form construction (which options, which
// pre-selection, which skip predicate), not the .Run() output.
// .Run()-level tests are TTY-only and out of scope for the unit
// suite. The tests use `package prompt` (white-box) to call the
// unexported helpers (preSelectedLibs, templateOptionsFor) directly.

package prompt

import (
	"reflect"
	"sort"
	"testing"

	"github.com/example/spin/internal/scaffold"
)

// TestAskType_SkipsWhenTypeSet asserts that askType returns nil
// without running the form when p.Type is already set (flag path).
// We can't easily test the .Run() path, but the skip-when-set path
// is testable because it doesn't touch the form.
func TestAskType_SkipsWhenTypeSet(t *testing.T) {
	p := &scaffold.Project{Type: "tui"}
	if err := askType(p); err != nil {
		t.Errorf("askType(p.Type=tui) = %v, want nil", err)
	}
	if p.Type != "tui" {
		t.Errorf("askType mutated p.Type: got %q, want %q", p.Type, "tui")
	}
}

// TestAskName_SkipsWhenNameSet asserts the same skip-when-set
// behavior for askName.
func TestAskName_SkipsWhenNameSet(t *testing.T) {
	p := &scaffold.Project{Name: "myapp"}
	if err := askName(p); err != nil {
		t.Errorf("askName(p.Name=myapp) = %v, want nil", err)
	}
	if p.Name != "myapp" {
		t.Errorf("askName mutated p.Name: got %q, want %q", p.Name, "myapp")
	}
}

// TestAskModule_SkipsWhenModuleSet asserts askModule skips when
// p.Module is already set to a non-default value.
func TestAskModule_SkipsWhenModuleSet(t *testing.T) {
	p := &scaffold.Project{Name: "myapp", Module: "github.com/foo/myapp"}
	if err := askModule(p); err != nil {
		t.Errorf("askModule(p.Module=set) = %v, want nil", err)
	}
	if p.Module != "github.com/foo/myapp" {
		t.Errorf("askModule mutated p.Module: got %q", p.Module)
	}
}

// TestAskLicense_SkipsWhenNonMitSet asserts askLicense skips when
// p.License is set to a non-default value (apache-2.0 or none).
func TestAskLicense_SkipsWhenNonMitSet(t *testing.T) {
	for _, lic := range []string{"apache-2.0", "none"} {
		p := &scaffold.Project{License: lic}
		if err := askLicense(p); err != nil {
			t.Errorf("askLicense(p.License=%q) = %v, want nil", lic, err)
		}
		if p.License != lic {
			t.Errorf("askLicense mutated p.License: got %q, want %q", p.License, lic)
		}
	}
}

// TestAskLicense_Options asserts askLicenseOptions returns the
// canonical three options in the documented order: MIT, Apache-2.0,
// None. askLicense consumes this builder so a regression in the
// option set (a typo, a missing value) would silently surface as a
// confusing huh form error; this test pins the contract.
func TestAskLicense_Options(t *testing.T) {
	opts := askLicenseOptions()
	if len(opts) != 3 {
		t.Fatalf("askLicenseOptions len = %d, want 3", len(opts))
	}
	wantValues := []string{"mit", "apache-2.0", "none"}
	for i, want := range wantValues {
		if opts[i].Value != want {
			t.Errorf("askLicenseOptions[%d].Value = %q, want %q", i, opts[i].Value, want)
		}
	}
}

// TestPreSelectLicense_MitPicksIndexZero asserts that
// preSelectLicense marks the mit option (index 0) as selected
// when called with license="mit". The huh.Option Selected method
// is a setter (returns a new Option with the flag flipped), so we
// verify the contract by comparing the option identity at the
// matched index against the pre-built "selected" option — the
// underlying value should differ from the unselected baseline.
func TestPreSelectLicense_MitPicksIndexZero(t *testing.T) {
	base := askLicenseOptions()
	sel := preSelectLicense(askLicenseOptions(), "mit")
	if base[0] == sel[0] {
		t.Error("preSelectLicense(mit): options[0] unchanged, want Selected(true) applied")
	}
	if base[1] != sel[1] {
		t.Error("preSelectLicense(mit): options[1] mutated, want untouched")
	}
	if base[2] != sel[2] {
		t.Error("preSelectLicense(mit): options[2] mutated, want untouched")
	}
}

// TestPreSelectLicense_UnknownLeavesAllUnchanged asserts that a
// license value with no matching option (e.g. "bsd-3-clause" if
// it ever snuck in) does not crash and leaves every option in
// its baseline state.
func TestPreSelectLicense_UnknownLeavesAllUnchanged(t *testing.T) {
	base := askLicenseOptions()
	sel := preSelectLicense(askLicenseOptions(), "bsd-3-clause")
	for i := range base {
		if base[i] != sel[i] {
			t.Errorf("preSelectLicense(unknown): options[%d] mutated, want untouched", i)
		}
	}
}

// TestAskTemplate_SkipsWhenNonDefaultSet asserts askTemplate skips
// when p.Template is set to a non-default value. The default is
// "tui-bubbletea" so we use a different value to exercise the skip
// path.
func TestAskTemplate_SkipsWhenNonDefaultSet(t *testing.T) {
	p := &scaffold.Project{Type: "cli", Template: "cli-cobra-fang"}
	if err := askTemplate(p); err != nil {
		t.Errorf("askTemplate(p.Template=cli-cobra-fang) = %v, want nil", err)
	}
}

// TestAskTemplateRepo_SkipsWhenSet asserts askTemplateRepo skips
// when p.TemplateRepo is already set.
func TestAskTemplateRepo_SkipsWhenSet(t *testing.T) {
	p := &scaffold.Project{TemplateRepo: "https://github.com/foo/bar"}
	if err := askTemplateRepo(p); err != nil {
		t.Errorf("askTemplateRepo(p.TemplateRepo=set) = %v, want nil", err)
	}
}

// TestPreSelectedLibs_TuiVariant asserts that the multi-select
// pre-selection for a TUI project is exactly the TUI defaults
// (bubbletea).
func TestPreSelectedLibs_TuiVariant(t *testing.T) {
	p := &scaffold.Project{Type: "tui"}
	got := preSelectedLibs(p)
	want := []string{"bubbletea"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preSelectedLibs(tui) = %v, want %v", got, want)
	}
}

// TestPreSelectedLibs_CliVariant asserts the pre-selection for a
// CLI project is the CLI defaults (cobra, fang).
func TestPreSelectedLibs_CliVariant(t *testing.T) {
	p := &scaffold.Project{Type: "cli"}
	got := preSelectedLibs(p)
	want := []string{"cobra", "fang"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preSelectedLibs(cli) = %v, want %v", got, want)
	}
}

// TestPreSelectedLibs_AllVariant asserts the pre-selection for
// "all" is the union of tui and cli defaults.
func TestPreSelectedLibs_AllVariant(t *testing.T) {
	p := &scaffold.Project{Type: "all"}
	got := preSelectedLibs(p)
	want := []string{"bubbletea", "cobra", "fang"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preSelectedLibs(all) = %v, want %v", got, want)
	}
}

// TestPreSelectedLibs_FlagSet asserts the pre-selection includes
// user-passed flags that aren't in the variant default. e.g.,
// a --huh flag on a tui project adds "huh" to the pre-selection.
func TestPreSelectedLibs_FlagSet(t *testing.T) {
	p := &scaffold.Project{
		Type: "tui",
		Libs: []string{"lipgloss"},
		Huh:  true,
	}
	got := preSelectedLibs(p)
	want := []string{"bubbletea", "huh", "lipgloss"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preSelectedLibs(tui+huh+lipgloss) = %v, want %v", got, want)
	}
}

// TestPreSelectedLibs_EmptyProject asserts the pre-selection is
// empty (not nil) for a zero *Project.
func TestPreSelectedLibs_EmptyProject(t *testing.T) {
	p := &scaffold.Project{}
	got := preSelectedLibs(p)
	if got == nil {
		t.Error("preSelectedLibs(zero) = nil, want empty (non-nil) slice")
	}
	if len(got) != 0 {
		t.Errorf("preSelectedLibs(zero) length = %d, want 0", len(got))
	}
}

// TestTemplateOptionsFor_Variants asserts the template options
// match the expected set for each variant.
func TestTemplateOptionsFor_Variants(t *testing.T) {
	cases := []struct {
		variant string
		want    []string // expected values
	}{
		{"tui", []string{"tui-bubbletea"}},
		{"cli", []string{"cli-cobra-fang"}},
		{"all", []string{"tui-bubbletea", "cli-cobra-fang"}},
		{"", nil},
		{"unknown", nil},
	}
	for _, c := range cases {
		t.Run(c.variant, func(t *testing.T) {
			opts := templateOptionsFor(c.variant)
			got := make([]string, len(opts))
			for i, o := range opts {
				got[i] = o.Value
			}
			gSorted := append([]string(nil), got...)
			sort.Strings(gSorted)
			wSorted := append([]string(nil), c.want...)
			sort.Strings(wSorted)
			if !reflect.DeepEqual(gSorted, wSorted) {
				t.Errorf("templateOptionsFor(%q) values = %v, want %v", c.variant, got, c.want)
			}
		})
	}
}

// TestSetBoolFieldByName asserts the small reflection-avoidance
// switch in askLibs correctly maps field names to p.<Field>.
// We test all libBoolMirror entries.
func TestSetBoolFieldByName(t *testing.T) {
	for _, fieldName := range libBoolMirror {
		p := &scaffold.Project{}
		setBoolFieldByName(p, fieldName, true)
		if !boolFieldByName(p, fieldName) {
			t.Errorf("setBoolFieldByName(p, %q, true) did not set the field", fieldName)
		}
		setBoolFieldByName(p, fieldName, false)
		if boolFieldByName(p, fieldName) {
			t.Errorf("setBoolFieldByName(p, %q, false) did not clear the field", fieldName)
		}
	}
}

// boolFieldByName is a small reader companion to setBoolFieldByName.
// It exists only for the test in this file.
func boolFieldByName(p *scaffold.Project, fieldName string) bool {
	switch fieldName {
	case "Cobra":
		return p.Cobra
	case "Fang":
		return p.Fang
	case "Viper":
		return p.Viper
	case "Huh":
		return p.Huh
	case "Glamour":
		return p.Glamour
	case "Wish":
		return p.Wish
	case "Log":
		return p.Log
	case "Harmonica":
		return p.Harmonica
	}
	return false
}
