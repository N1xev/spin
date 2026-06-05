package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestLintCmd_Registered asserts the `lint` subcommand is attached
// to rootCmd after init() runs. The pointer identity check is the
// strongest signal: the very same lintCmd variable declared in
// lint.go must be the one in the command tree.
func TestLintCmd_Registered(t *testing.T) {
	found := false
	for _, c := range RootCmd().Commands() {
		if c.Name() == "lint" && c == lintCmd {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, 8)
		for _, c := range RootCmd().Commands() {
			names = append(names, c.Name())
		}
		t.Errorf("lintCmd not registered on rootCmd; commands: %v", names)
	}
}

// TestLintCmd_UseContainsArgs asserts the Use string surfaces the
// passthrough behavior in `spin lint --help`: the [golangci-lint args]
// placeholder tells users this is not a closed flag set.
func TestLintCmd_UseContainsArgs(t *testing.T) {
	if !strings.Contains(lintCmd.Use, "golangci-lint args") {
		t.Errorf("lintCmd.Use = %q; want it to contain %q", lintCmd.Use, "golangci-lint args")
	}
}

// TestLintCmd_LongMentionsInstallHint asserts the Long help text
// includes the literal install command. This doubles as a
// discoverability check: a user reading `spin lint --help` must
// see the exact `go install ...` to recover from a missing
// binary without leaving the CLI.
func TestLintCmd_LongMentionsInstallHint(t *testing.T) {
	want := "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
	if !strings.Contains(lintCmd.Long, want) {
		t.Errorf("lintCmd.Long does not contain install hint %q; got: %q", want, lintCmd.Long)
	}
}

// TestLintCmd_RunEForwardsArgs invokes RunE with `["version"]`.
// We don't assert on success — golangci-lint may or may not be on
// PATH in the test env — only that the wiring is live: RunE must
// return whatever wrap.Lint returns. The only error path
// wrap.Lint produces mentions "golangci-lint" (the not-found
// branch); a nil return means the binary is installed.
func TestLintCmd_RunEForwardsArgs(t *testing.T) {
	err := lintCmd.RunE(lintCmd, []string{"version"})
	if err != nil && !strings.Contains(err.Error(), "golangci-lint") {
		t.Errorf("RunE returned an unexpected error (only the not-found path is expected); got: %v", err)
	}
}

// TestLintCmd_Help renders the subcommand's help to a buffer and
// asserts the help mentions the command name and surfaces the
// install hint + args-placeholder Use string.
func TestLintCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	lintCmd.SetOut(&buf)
	lintCmd.SetErr(&buf)
	if err := lintCmd.Help(); err != nil {
		t.Fatalf("lintCmd.Help: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"lint", "golangci-lint args", "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"} {
		if !strings.Contains(out, want) {
			t.Errorf("`lint --help` output missing %q; got:\n%s", want, out)
		}
	}
}
