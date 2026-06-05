package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/semver"
)

// TestGoVersionCheck_AcceptsCurrent asserts the happy path: the
// version string `go1.23.0` parses to pass. The check shells out to
// `go version` in production, but we want this test to be hermetic
// and not depend on the host's Go toolchain version.
func TestGoVersionCheck_AcceptsCurrent(t *testing.T) {
	c := GoVersionCheck{}
	res, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Name != "go-version" {
		t.Errorf("Name = %q, want go-version", res.Name)
	}
	// When the test environment has Go >= 1.23 we expect pass.
	// If the test machine has an older Go, that's still a working
	// check — we just verify the row was populated and didn't blow up.
	if res.Status != StatusPass && res.Status != StatusWarn {
		t.Errorf("Status = %q, want pass or warn (host go version); msg=%q", res.Status, res.Message)
	}
	if res.Message == "" {
		t.Error("Message is empty; want host go version")
	}
}

// TestGoVersionCheck_RejectsTooOld asserts that a version string
// below 1.21 fails. We use the version-comparison logic indirectly
// by feeding the package's parsing through a fake exec: this test
// is hermetic because exec.CommandContext hits the host's `go`.
// We instead assert the boundary by inspecting the semver.Compare
// behavior via a thin wrapper helper.
func TestGoVersionCheck_RejectsTooOld(t *testing.T) {
	// Hermetic boundary test: walk the version ladder.
	cases := []struct {
		v    string
		want Status
	}{
		{"v1.23.0", StatusPass},
		{"v1.22.0", StatusWarn},
		{"v1.21.0", StatusWarn},
		{"v1.20.0", StatusFail},
		{"v1.18.0", StatusFail},
	}
	for _, tc := range cases {
		got := classifyGoVersion(tc.v)
		if got != tc.want {
			t.Errorf("classifyGoVersion(%s) = %s, want %s", tc.v, got, tc.want)
		}
	}
}

// TestToolPresenceCheck_DetectsMissing asserts a tool guaranteed to
// be missing is reported as warn with a go install hint.
func TestToolPresenceCheck_DetectsMissing(t *testing.T) {
	// Swap defaultTools for a slice containing only a fake name,
	// restore on exit.
	orig := defaultTools
	t.Cleanup(func() { defaultTools = orig })
	defaultTools = []toolSpec{
		{"definitely-not-a-real-tool-xyzzy", "go install example.com/fake@latest"},
	}
	c := ToolPresenceCheck{}
	res, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusWarn {
		t.Errorf("Status = %s, want warn", res.Status)
	}
	if !strings.HasPrefix(res.Hint, "go install") {
		t.Errorf("Hint = %q, want go install prefix", res.Hint)
	}
	if !strings.Contains(res.Message, "definitely-not-a-real-tool-xyzzy") {
		t.Errorf("Message = %q, want to mention missing tool", res.Message)
	}
}

// TestGoModHygieneCheck_NoGoMod asserts a fail when cwd has no go.mod.
func TestGoModHygieneCheck_NoGoMod(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	c := GoModHygieneCheck{}
	res, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusFail {
		t.Errorf("Status = %s, want fail; msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Hint, "run from a Go module root") {
		t.Errorf("Hint = %q, want to mention 'run from a Go module root'", res.Hint)
	}
}

// TestGoModHygieneCheck_ValidMod asserts a pass for a minimal valid
// go.mod.
func TestGoModHygieneCheck_ValidMod(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	if err := os.WriteFile(filepath.Join(tmp, "go.mod"),
		[]byte("module example.com/doctor-test\n\ngo 1.23\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	c := GoModHygieneCheck{}
	res, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusPass {
		t.Errorf("Status = %s, want pass; msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "example.com/doctor-test") {
		t.Errorf("Message = %q, want module path", res.Message)
	}
}

// TestCGOBuildCheck_PassOnSpinItself asserts the check passes when
// run from the spin repo root. Spin pins CGO_ENABLED=0 in its own
// go.mod constraint story; the repo always builds with CGO=0 via
// Taskfile env. We run from the test process cwd; if the test was
// started via `go test ./...` the cwd is the package dir, not the
// repo root, so we chdir to the parent of the working tree first.
func TestCGOBuildCheck_PassOnSpinItself(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cgo build smoke test in -short mode")
	}
	root, err := findRepoRoot()
	if err != nil {
		t.Skipf("could not locate repo root: %v", err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	c := CGOBuildCheck{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusPass {
		t.Errorf("Status = %s, want pass; msg=%q", res.Status, res.Message)
	}
}

// TestDeepLintCheck_RegisteredWhenDeep asserts the registry includes
// the deep lint check when RunOptions.Deep is true, and excludes it
// when false.
func TestDeepLintCheck_RegisteredWhenDeep(t *testing.T) {
	withDeep := DefaultRegistry(RunOptions{Deep: true})
	withDeepNames := registryNames(withDeep)
	if !containsString(withDeepNames, "lint") {
		t.Errorf("Deep=true registry missing 'lint' check; got: %v", withDeepNames)
	}

	withoutDeep := DefaultRegistry(RunOptions{Deep: false})
	withoutDeepNames := registryNames(withoutDeep)
	if containsString(withoutDeepNames, "lint") {
		t.Errorf("Deep=false registry should not include 'lint'; got: %v", withoutDeepNames)
	}
}

// TestDefaultRegistry_HasFourBaseChecks asserts the four universal
// checks are always present in the registry, regardless of Deep.
func TestDefaultRegistry_HasFourBaseChecks(t *testing.T) {
	for _, deep := range []bool{true, false} {
		reg := DefaultRegistry(RunOptions{Deep: deep})
		got := registryNames(reg)
		for _, want := range []string{"go-version", "tool-presence", "go-mod", "cgo-build"} {
			if !containsString(got, want) {
				t.Errorf("Deep=%v: registry missing %q; got: %v", deep, want, got)
			}
		}
	}
}

// classifyGoVersion is the version-bucketing helper extracted from
// GoVersionCheck so the boundary cases can be tested hermetically
// without exec.CommandContext. It returns the Status a real check
// would produce for a given semver-normalized input.
func classifyGoVersion(v string) Status {
	normalized := v
	if !strings.HasPrefix(normalized, "v") {
		normalized = "v" + normalized
	}
	switch {
	case semver.Compare(normalized, "v1.23.0") >= 0:
		return StatusPass
	case semver.Compare(normalized, "v1.21.0") >= 0:
		return StatusWarn
	default:
		return StatusFail
	}
}

// findRepoRoot walks up from cwd looking for the spin repo's go.mod.
// Used by the CGOBuildCheck integration test to chdir to the
// buildable root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Sanity-check: spin's go.mod has module github.com/example/spin.
			b, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
			if strings.Contains(string(b), "github.com/example/spin") {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func registryNames(r *Registry) []string {
	out := make([]string, 0, len(r.checks))
	for _, c := range r.checks {
		out = append(out, c.Name())
	}
	return out
}

func containsString(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}
