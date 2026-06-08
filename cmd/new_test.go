package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// withStderr captures stderr for the duration of fn and returns the
// bytes that were written to it. Used to assert what the
// deprecation-notice helper printed.
func withStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}

// TestPrintDeprecationNotice_OncePerProcess verifies the deprecation
// helper prints the notice exactly once even when called multiple
// times in the same process. The flag is process-global; we reset it
// at the start of the test to ensure isolation.
func TestPrintDeprecationNotice_OncePerProcess(t *testing.T) {
	deprecationPrinted = false

	out := withStderr(t, func() {
		printDeprecationNotice()
		printDeprecationNotice()
		printDeprecationNotice()
	})

	// Count occurrences of the WARN prefix. Should be exactly 1.
	if c := strings.Count(out, "WARN"); c != 1 {
		t.Fatalf("expected 1 deprecation notice, got %d (output: %q)", c, out)
	}
	// And the message must point at the new form.
	if !strings.Contains(out, "spin new charm <name>") {
		t.Fatalf("deprecation notice should mention the new form, got: %q", out)
	}
}

// TestIsKnownEcosystem verifies the case-insensitive ecosystem lookup
// against the default registry (charm + rust).
func TestIsKnownEcosystem(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"charm", true},
		{"CHARM", true},
		{"Charm", true},
		{"rust", true},
		{"Rust", true},
		{"madeup", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.in), func(t *testing.T) {
			if got := isKnownEcosystem(c.in); got != c.want {
				t.Fatalf("isKnownEcosystem(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
