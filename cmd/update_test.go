package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestUpdateCmd_Registered(t *testing.T) {
	found := false
	for _, c := range RootCmd().Commands() {
		if c.Name() == "update" && c == updateCmd {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, 8)
		for _, c := range RootCmd().Commands() {
			names = append(names, c.Name())
		}
		t.Errorf("updateCmd not registered on rootCmd; commands: %v", names)
	}
}

func TestUpdateCmd_HasAllFlag(t *testing.T) {
	fl := updateCmd.Flags().Lookup("all")
	if fl == nil {
		t.Fatal("flag --all not registered on updateCmd")
	}
	if fl.DefValue != "false" {
		t.Errorf("--all default = %q, want \"false\"", fl.DefValue)
	}
}

func TestUpdateCmd_LongMentionsForm(t *testing.T) {
	for _, want := range []string{"Skip", "newStable", "newLatest", "huh v2"} {
		if !strings.Contains(updateCmd.Long, want) {
			t.Errorf("updateCmd.Long missing %q; got: %q", want, updateCmd.Long)
		}
	}
}

func TestUpdateCmd_LongMentionsNoTest(t *testing.T) {
	if !strings.Contains(updateCmd.Long, "Does not run `go test`") {
		t.Errorf("updateCmd.Long should mention the no-go-test policy; got: %q", updateCmd.Long)
	}
}

// TestRunUpdate_NoGoMod_ReturnsError asserts the find-up behavior:
// chdir into an empty tempdir, runUpdate must fail with a wrapped
// "no go.mod found" error before ever calling PromptForUpdate.
func TestRunUpdate_NoGoMod_ReturnsError(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("chdir back: %v", err)
		}
	})

	empty := t.TempDir()
	if err := os.Chdir(empty); err != nil {
		t.Fatalf("chdir to tempdir: %v", err)
	}

	err = runUpdate(updateCmd, nil)
	if err == nil {
		t.Fatal("expected error when no go.mod exists, got nil")
	}
	if !strings.Contains(err.Error(), "no go.mod found") {
		t.Errorf("expected error to mention 'no go.mod found'; got: %v", err)
	}
}
