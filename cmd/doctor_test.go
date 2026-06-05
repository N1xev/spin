package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestDoctorCmd_Registered asserts the `doctor` subcommand is on
// rootCmd after init() runs. The pointer identity check is the
// strongest signal: the very same doctorCmd variable declared in
// doctor.go must be the one attached to the tree.
func TestDoctorCmd_Registered(t *testing.T) {
	found := false
	for _, c := range RootCmd().Commands() {
		if c.Name() == "doctor" && c == doctorCmd {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, 8)
		for _, c := range RootCmd().Commands() {
			names = append(names, c.Name())
		}
		t.Errorf("doctorCmd not registered on rootCmd; commands: %v", names)
	}
}

// TestDoctorCmd_HasAllFlags asserts all 4 flags are registered with
// the expected default values. A regression that drops a flag from
// init() breaks the user-visible --format/--strict/--deep/--fix
// surface and the runDoctor handler would silently no-op.
func TestDoctorCmd_HasAllFlags(t *testing.T) {
	cases := []struct {
		name    string
		defVal  string
		isBool  bool
		boolDef bool
	}{
		{"format", "human", false, false},
		{"strict", "", true, false},
		{"deep", "", true, false},
		{"fix", "", true, false},
	}
	for _, tc := range cases {
		fl := doctorCmd.Flags().Lookup(tc.name)
		if fl == nil {
			t.Errorf("flag --%s not registered", tc.name)
			continue
		}
		if tc.isBool {
			if fl.DefValue != "false" {
				t.Errorf("flag --%s default = %q, want \"false\"", tc.name, fl.DefValue)
			}
		} else {
			if fl.DefValue != tc.defVal {
				t.Errorf("flag --%s default = %q, want %q", tc.name, fl.DefValue, tc.defVal)
			}
		}
	}
}

// TestDoctorCmd_Help renders the subcommand's help to a buffer and
// asserts the output mentions the command name and every flag we
// expect users to be able to discover through `spin doctor --help`.
// cobra's --help text is plain (not fang-styled) for subcommands
// without a fang wrapper; we just need the strings to be present.
func TestDoctorCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	doctorCmd.SetErr(&buf)
	if err := doctorCmd.Help(); err != nil {
		t.Fatalf("doctorCmd.Help: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"doctor", "--deep", "--format", "--strict", "--fix"} {
		if !strings.Contains(out, want) {
			t.Errorf("`doctor --help` output missing %q; got:\n%s", want, out)
		}
	}
}

// keep cobra import used in case future tests need it
var _ = (*cobra.Command)(nil)
