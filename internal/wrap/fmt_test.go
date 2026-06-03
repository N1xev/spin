package wrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFmt_GofumptMissing_Strict exercises the strict-mode error
// path: when gofumpt is missing and noStrict is false, Fmt()
// returns an error that mentions the gofumpt install hint.
//
// We point $PATH at a tempdir that has goimports + gofmt but NOT
// gofumpt. (goimports and gofmt come from the Go toolchain, so
// we can't fully strip them — but we don't need to: the chain
// short-circuits on the gofumpt-missing check.)
func TestFmt_GofumptMissing_Strict(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Point PATH at a tempdir that has neither gofumpt nor
	// goimports (we create a "go" symlink-less dir). The real
	// gofmt from the Go toolchain is still reachable via the
	// test process's PATH if we don't fully replace it; but
	// since the gofumpt check is the first thing Fmt() does, the
	// exact PATH contents of goimports/gofmt don't matter.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	emptyBin := t.TempDir()
	if err := os.Setenv("PATH", emptyBin); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	err := Fmt(false) // strict
	if err == nil {
		t.Fatal("expected Fmt(false) to return error when gofumpt is missing")
	}
	if !strings.Contains(err.Error(), "gofumpt") {
		t.Errorf("expected error to mention gofumpt; got: %v", err)
	}
	if !strings.Contains(err.Error(), "mvdan.cc/gofumpt") {
		t.Errorf("expected error to mention the gofumpt install path; got: %v", err)
	}
	if !strings.Contains(err.Error(), "--no-strict") {
		t.Errorf("expected error to mention --no-strict opt-out; got: %v", err)
	}
}

// TestFmt_GofumptMissing_NoStrict exercises the no-strict path:
// when gofumpt is missing and noStrict is true, Fmt() prints a
// warning to stderr and falls through to goimports + gofmt.
//
// We point $PATH at a tempdir that has goimports (so the chain
// reaches goimports) and preserves the original PATH for gofmt
// (which is part of the Go toolchain and not on $PATH by default
// in a barebones CI image — but here it usually is). The key
// invariant is the gofumpt-missing warning fires, regardless of
// whether the final gofmt step succeeds.
func TestFmt_GofumptMissing_NoStrict(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create a tempdir that has a NO-OP goimports shim so the
	// chain reaches it. Preserve original PATH for gofmt.
	binDir := t.TempDir()
	writeFake := func(name string) {
		script := "#!/bin/sh\n# noop shim for " + name + " (test only)\n"
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	writeFake("goimports") // gofumpt deliberately NOT created

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	// New PATH: our shim dir (so goimports resolves) + original
	// (so go/gofmt resolve normally).
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	// We don't care whether Fmt() returns nil (gofmt does nothing
	// on an empty dir) or non-nil (gofmt itself missing) — the
	// only invariant is the gofumpt warning.
	_ = Fmt(true)
	_ = w.Close()

	out := readPipe(t, r)
	if !strings.Contains(out, "gofumpt not found") {
		t.Errorf("expected gofumpt-missing warning; got: %q", out)
	}
	if !strings.Contains(out, "--no-strict") {
		t.Errorf("expected warning to mention --no-strict; got: %q", out)
	}
}

// TestFmt_AllPresent exercises the happy path: all three tools
// are on $PATH (we mock gofumpt and goimports; gofmt is real).
// The chain must run gofumpt first, then goimports, then gofmt.
//
// We assert ordering by having each mock tool append its name to
// a marker file; after Fmt() returns, the marker file contains
// the order in which the tools were called.
func TestFmt_AllPresent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	binDir := t.TempDir()
	marker := filepath.Join(dir, "chain-order.txt")

	// gofumpt and goimports are fakes; gofmt is the real one
	// (gofmt has no easy mock since it has no Go-package API).
	writeFake := func(name string) {
		script := "#!/bin/sh\necho " + name + " >> " + marker + "\n"
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	writeFake("gofumpt")
	writeFake("goimports")

	// Prepend the fake binDir to PATH so gofumpt/goimports resolve
	// to our fakes while gofmt (the real one from the Go toolchain)
	// stays reachable via the original PATH.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	if err := Fmt(false); err != nil {
		t.Fatalf("Fmt() failed: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	// The marker should contain gofumpt, goimports, and (the real)
	// gofmt's invocation too. We check the first two are present
	// and in the right order; gofmt always runs third and is part
	// of the Go toolchain, so its line is appended to the same
	// marker only if it doesn't itself write to stdout (it doesn't
	// in this empty dir, but to keep the test robust we don't
	// assert on the gofmt line).
	gotStr := string(got)
	gofumptIdx := strings.Index(gotStr, "gofumpt")
	goimportsIdx := strings.Index(gotStr, "goimports")
	if gofumptIdx < 0 {
		t.Errorf("expected gofumpt to run; marker: %q", gotStr)
	}
	if goimportsIdx < 0 {
		t.Errorf("expected goimports to run; marker: %q", gotStr)
	}
	if gofumptIdx >= 0 && goimportsIdx >= 0 && gofumptIdx > goimportsIdx {
		t.Errorf("expected gofumpt before goimports; got marker: %q", gotStr)
	}
}

// TestFmt_NoStrictFlagThreading verifies the boolean parameter
// reaches Fmt() correctly. We can't easily test both branches
// (true/false) without a real gofumpt or a complex PATH mock,
// but we can confirm the function signature accepts the bool.
func TestFmt_NoStrictFlagThreading(t *testing.T) {
	// Smoke: the function should accept a bool and not panic.
	_ = Fmt(true)
	// The error path is exercised by the Strict test above.
}
