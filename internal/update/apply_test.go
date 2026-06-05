package update

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeRunner is the test CommandRunner. It records every call and
// returns a canned (output, error) per subcommand family. Tests
// toggle behavior by setting hooks on the relevant subcommand.
type fakeRunner struct {
	mu sync.Mutex

	// calls is the ordered list of argv arrays we were asked to
	// execute. Tests assert on the shape (length, prefix, exact
	// value) of this slice.
	calls [][]string

	// getResponses maps "module@version" to (output, err). A miss
	// falls back to defaultGet (success, empty output).
	getResponses map[string]fakeResp

	// buildErr is the error returned by `go build ./...`. nil →
	// build succeeds. Tests that exercise the failure path set
	// this directly.
	buildErr error
	buildOut []byte

	// testMarkerPath, if set, is created on disk if `go test ./...`
	// is ever called. TestApply_DoesNotRunGoTest asserts the file
	// does not exist after Apply completes. This is the D-10
	// contract: update never runs the user's tests.
	testMarkerPath string
}

type fakeResp struct {
	out []byte
	err error
}

func (f *fakeRunner) Run(_ string, _ []string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Copy the args slice so later mutations to caller's argv
	// cannot retroactively rewrite history.
	recorded := append([]string(nil), args...)
	f.calls = append(f.calls, recorded)

	switch {
	case len(args) >= 1 && args[0] == "go" && len(args) >= 2 && args[1] == "get":
		moduleVersion := args[len(args)-1]
		if resp, ok := f.getResponses[moduleVersion]; ok {
			return resp.out, resp.err
		}
		return nil, nil

	case len(args) >= 3 && args[0] == "go" && args[1] == "mod" && args[2] == "tidy":
		return nil, nil

	case len(args) >= 3 && args[0] == "go" && args[1] == "build" && args[2] == "./...":
		return f.buildOut, f.buildErr

	case len(args) >= 3 && args[0] == "go" && args[1] == "test" && args[2] == "./...":
		if f.testMarkerPath != "" {
			_ = os.WriteFile(f.testMarkerPath, []byte("TESTS RAN"), 0o644)
		}
		// Returning success here would mask the regression;
		// return an error so Apply cannot accidentally succeed
		// if a future bug ever routes a `go test` through this
		// fake.
		return []byte("D-10 VIOLATION: go test should not be in the apply path"), errors.New("go test ran")
	}

	return nil, fmt.Errorf("fakeRunner: unhandled argv: %v", args)
}

// findCall returns the first recorded call whose args join starts
// with prefix, or -1. Used by the build-failure test to assert the
// fake was invoked in the right order without scanning the full
// call list manually.
func (f *fakeRunner) findCall(prefix ...string) int {
	for i, c := range f.calls {
		if len(c) < len(prefix) {
			continue
		}
		match := true
		for j, p := range prefix {
			if c[j] != p {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// writeRealGoMod drops a real, valid go.mod + main.go into a
// tempdir so the no-op Apply test can call the real Apply path
// against a real (or fake) `go`. The fixture builds cleanly.
func writeRealGoMod(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gomod := `module example.com/test

go 1.23
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	main := `package main

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return dir
}

func TestApply_NoOp_EmptyDeps(t *testing.T) {
	// A no-op Apply over a real, minimal go.mod with no deps to
	// upgrade must succeed and leave go.mod untouched. We point
	// the runner at /bin/true (or the platform equivalent) for
	// `go mod tidy` and `go build` by shimming PATH... actually,
	// simpler: assert that ApplyWithRunner is the no-op path
	// and use a fake to confirm no shell-out happened.
	fake := &fakeRunner{}
	dir := writeRealGoMod(t)
	var log bytes.Buffer

	err := ApplyWithRunner(fake, nil, dir, &log)
	if err != nil {
		t.Fatalf("ApplyWithRunner: %v", err)
	}
	if len(fake.calls) != 2 {
		t.Errorf("expected 2 calls (tidy + build), got %d: %v", len(fake.calls), fake.calls)
	}
	if !strings.Contains(log.String(), "applied 0 upgrade(s)") {
		t.Errorf("log should report 0 upgrades; got %q", log.String())
	}
}

func TestApply_SingleUpgrade_FakeGo(t *testing.T) {
	fake := &fakeRunner{
		getResponses: map[string]fakeResp{
			"example.com/dep@v1.1.0": {},
		},
	}
	dir := t.TempDir()
	gomod := "module example.com/test\n\ngo 1.23\n\nrequire example.com/dep v1.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	deps := []Dep{{
		Module: "example.com/dep",
		Old:    "v1.0.0",
		Target: "v1.1.0",
	}}
	if err := ApplyWithRunner(fake, deps, dir, io.Discard); err != nil {
		t.Fatalf("ApplyWithRunner: %v", err)
	}

	// Expect 3 calls in this order: go get, go mod tidy, go build.
	if len(fake.calls) != 3 {
		t.Fatalf("expected 3 calls (get + tidy + build), got %d: %v", len(fake.calls), fake.calls)
	}

	wantGet := []string{"go", "get", "example.com/dep@v1.1.0"}
	if !equalStringSlices(fake.calls[0], wantGet) {
		t.Errorf("call 0 = %v, want %v", fake.calls[0], wantGet)
	}
	wantTidy := []string{"go", "mod", "tidy"}
	if !equalStringSlices(fake.calls[1], wantTidy) {
		t.Errorf("call 1 = %v, want %v", fake.calls[1], wantTidy)
	}
	wantBuild := []string{"go", "build", "./..."}
	if !equalStringSlices(fake.calls[2], wantBuild) {
		t.Errorf("call 2 = %v, want %v", fake.calls[2], wantBuild)
	}
}

func TestApply_BuildFailure_ReturnsError(t *testing.T) {
	fake := &fakeRunner{
		buildErr: errors.New("exit 1"),
		buildOut: []byte("compile error: cannot find foo"),
	}
	dir := writeRealGoMod(t)

	err := ApplyWithRunner(fake, nil, dir, io.Discard)
	if err == nil {
		t.Fatal("expected error from failed build, got nil")
	}
	if !strings.Contains(err.Error(), "compile error: cannot find foo") {
		t.Errorf("error should contain build output; got: %v", err)
	}
	if !strings.Contains(err.Error(), "go build") {
		t.Errorf("error should mention go build; got: %v", err)
	}
}

func TestApply_DoesNotRunGoTest(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	fake := &fakeRunner{
		testMarkerPath: marker,
	}
	dir := writeRealGoMod(t)

	_ = ApplyWithRunner(fake, nil, dir, io.Discard)

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("D-10 VIOLATION: Apply invoked `go test` (marker file was created)")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat marker: %v", err)
	}

	// Belt-and-braces: scan recorded calls for the forbidden
	// `go test` argv. Catches a future regression that routes
	// through the fake's fallback branch (which would create
	// the marker file before returning an error).
	for _, c := range fake.calls {
		if len(c) >= 2 && c[0] == "go" && c[1] == "test" {
			t.Errorf("D-10 VIOLATION: recorded call includes go test: %v", c)
		}
	}
}

func TestApply_MultipleUpgrades_BatchedTidy(t *testing.T) {
	fake := &fakeRunner{}
	dir := writeRealGoMod(t)

	deps := []Dep{
		{Module: "example.com/a", Old: "v1.0.0", Target: "v1.1.0"},
		{Module: "example.com/b", Old: "v2.0.0", Target: "v2.0.1"},
		{Module: "example.com/c", Old: "v3.0.0", Target: "v3.0.1"},
	}
	if err := ApplyWithRunner(fake, deps, dir, io.Discard); err != nil {
		t.Fatalf("ApplyWithRunner: %v", err)
	}

	// Expect: 3 go get + 1 go mod tidy + 1 go build = 5 calls.
	if len(fake.calls) != 5 {
		t.Fatalf("expected 5 calls, got %d: %v", len(fake.calls), fake.calls)
	}

	tidyCount := 0
	for _, c := range fake.calls {
		if len(c) >= 3 && c[0] == "go" && c[1] == "mod" && c[2] == "tidy" {
			tidyCount++
		}
	}
	if tidyCount != 1 {
		t.Errorf("go mod tidy ran %d times, want exactly 1 (batched, not per dep)", tidyCount)
	}

	// And `go build` was the last call.
	last := fake.calls[len(fake.calls)-1]
	if !equalStringSlices(last, []string{"go", "build", "./..."}) {
		t.Errorf("last call = %v, want [go build ./...]", last)
	}
}

// TestApply_TargetEqualOld_SkipsGoGet makes sure we don't shell
// out a `go get module@sameversion` for a dep the user left
// untouched. The CGO=0 build still runs.
func TestApply_TargetEqualOld_SkipsGoGet(t *testing.T) {
	fake := &fakeRunner{}
	dir := writeRealGoMod(t)

	deps := []Dep{
		{Module: "example.com/a", Old: "v1.0.0", Target: "v1.0.0"},
	}
	if err := ApplyWithRunner(fake, deps, dir, io.Discard); err != nil {
		t.Fatalf("ApplyWithRunner: %v", err)
	}
	if len(fake.calls) != 2 {
		t.Errorf("expected 2 calls (tidy + build), got %d: %v", len(fake.calls), fake.calls)
	}
	if fake.findCall("go", "get") >= 0 {
		t.Error("go get should not run when Target == Old")
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
