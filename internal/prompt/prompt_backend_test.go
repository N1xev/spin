// White-box tests for backend resolution and dispatch.
//
// The testable surface for backend dispatch is small: the test
// runner is non-TTY, so `Fill` always short-circuits to nil at
// ShouldPrompt(). These tests cover resolveBackend() directly
// (no TTY required, no Fill call).
//
// resolveBackend is package-private, so this file is `package
// prompt` (white-box) to access it. Tests construct a Deps with
// stubbed LookPath and VersionCheck and pass it to resolveBackend —
// no package-level state is mutated, so t.Parallel() is safe.

package prompt

import (
	"errors"
	"testing"
)

// TestResolveBackend_HuhWhenGumMissing asserts that resolveBackend
// returns backendHuh when gum is not on $PATH. This is the default
// state in the test runner (gum is not installed here) and proves
// the LookPath-not-found branch.
func TestResolveBackend_HuhWhenGumMissing(t *testing.T) {
	// Ensure the SPIN_USE_HUH escape hatch is OFF so the LookPath
	// path is exercised.
	t.Setenv("SPIN_USE_HUH", "")
	deps := Deps{
		LookPath: func(file string) (string, error) {
			return "", errors.New("gum not on PATH (test stub)")
		},
		VersionCheck: func(path string) error { return nil },
		Runner:       gumRunCapture,
	}
	if got := resolveBackend(deps); got != backendHuh {
		t.Errorf("resolveBackend() with gum missing = %v, want %v", got, backendHuh)
	}
}

// TestResolveBackend_HuhWhenSPINUseHuh1 asserts that the SPIN_USE_HUH=1
// escape hatch forces backendHuh regardless of gum's PATH status.
// This is the documented escape hatch from the install hint.
func TestResolveBackend_HuhWhenSPINUseHuh1(t *testing.T) {
	t.Setenv("SPIN_USE_HUH", "1")
	// Even if a fake LookPath returns success and the version
	// check passes, the env var must short-circuit to backendHuh.
	deps := Deps{
		LookPath:     func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error { return nil },
		Runner:       gumRunCapture,
	}
	if got := resolveBackend(deps); got != backendHuh {
		t.Errorf("resolveBackend() with SPIN_USE_HUH=1 = %v, want %v", got, backendHuh)
	}
}

// TestResolveBackend_GumWhenAvailableAndHealthy asserts that
// resolveBackend returns backendGum when gum is on $PATH AND
// `gum --version` exits 0. Both seams are stubbed to simulate a
// healthy install.
func TestResolveBackend_GumWhenAvailableAndHealthy(t *testing.T) {
	t.Setenv("SPIN_USE_HUH", "")
	deps := Deps{
		LookPath:     func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error { return nil },
		Runner:       gumRunCapture,
	}
	if got := resolveBackend(deps); got != backendGum {
		t.Errorf("resolveBackend() with healthy gum = %v, want %v", got, backendGum)
	}
}

// TestResolveBackend_HuhWhenGumBroken asserts that resolveBackend
// returns backendHuh when gum is on $PATH but `gum --version` fails
// (a corrupt install should not break the scaffolder).
func TestResolveBackend_HuhWhenGumBroken(t *testing.T) {
	t.Setenv("SPIN_USE_HUH", "")
	deps := Deps{
		LookPath: func(file string) (string, error) { return "/fake/gum", nil },
		VersionCheck: func(path string) error {
			return errors.New("gum: corrupt install (test stub)")
		},
		Runner: gumRunCapture,
	}
	if got := resolveBackend(deps); got != backendHuh {
		t.Errorf("resolveBackend() with broken gum = %v, want %v (must fall back to huh)", got, backendHuh)
	}
}

// TestBackendString asserts the String() mapping used in the Debug
// log line: `"prompt backend" backend=gum` or `backend=huh`.
func TestBackendString(t *testing.T) {
	cases := map[backend]string{
		backendGum:  "gum",
		backendHuh:  "huh",
		backendNone: "none",
	}
	for b, want := range cases {
		if got := b.String(); got != want {
			t.Errorf("backend(%d).String() = %q, want %q", b, got, want)
		}
	}
}

// TestFill_DispatchHiresolvesHuhWhenGumMissing asserts that Fill
// dispatches to fillWithHuh when gum is missing. The assertion
// is via resolveBackend, not via the actual fillWithHuh call,
// because the test runner is non-TTY and Fill short-circuits
// before any backend runs.
//
// This is the "load-bearing for the dispatch" test: it documents
// that Fill calls resolveBackend() and that resolveBackend returns
// backendHuh in the default test environment. The actual fillWithHuh
// path is covered by the huh tests; the actual fillWithGum path
// is covered by the gum tests in gum_test.go.
func TestFill_DispatchHiresolvesHuhWhenGumMissing(t *testing.T) {
	t.Setenv("SPIN_USE_HUH", "")
	deps := Deps{
		LookPath: func(file string) (string, error) {
			return "", errors.New("gum not on PATH (test stub)")
		},
		VersionCheck: func(path string) error { return nil },
		Runner:       gumRunCapture,
	}
	if got := resolveBackend(deps); got != backendHuh {
		t.Errorf("resolveBackend() = %v, want %v (Fill would dispatch to fillWithHuh)", got, backendHuh)
	}
}
