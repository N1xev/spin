package wrap

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

// minGoVersionForPrism is the lowest Go version on which the prism
// test runner can be `go install`-ed. Below this version, we
// unconditionally fall back to `go test` even if prism is on $PATH
// (prism itself won't build).
//
// Per Prism's README: requires Go 1.24+.
const minGoVersionForPrism = "1.24"

// Test executes `spin test` in the current working directory.
//
// The preferred path is `prism go test ./...` (prism wraps go test
// with parallel/colored output). prism requires Go 1.24+, so the
// detector checks both `$PATH` and the runtime Go version before
// selecting prism. If either check fails, we fall back to
// `go test ./...` and print a one-line hint explaining why.
//
// This is the only wrapper with unique logic beyond the
// RunWithFallback pattern: the version gate means the standard
// helper cannot be used as-is. The function composes a ToolSpec
// directly and prints its own hint so the user knows whether
// prism is missing or whether the Go version is too old.
func Test() error {
	if !goVersionLessThan(minGoVersionForPrism) {
		if path, err := exec.LookPath("prism"); err == nil {
			spec := ToolSpec{
				Name: path,
				Args: []string{"go", "test", "./..."},
			}
			return runTool(spec.Name, spec.Args, nil)
		}
		fmt.Fprintln(os.Stderr,
			"hint: prism not found on $PATH; install with: go install go.dalton.dog/prism@latest\n"+
				"falling back to: go test ./...")
	} else {
		fmt.Fprintf(os.Stderr,
			"hint: prism requires Go %s+; you are on %s\n"+
				"falling back to: go test ./...\n",
			minGoVersionForPrism, runtime.Version())
	}
	spec := ToolSpec{
		Name: "go",
		Args: []string{"test", "./..."},
	}
	return runTool(spec.Name, spec.Args, nil)
}

// goVersionLessThan reports whether the current Go version is less
// than want. The comparison uses golang.org/x/mod/semver.Compare
// (via the IsValid + Canonical + Compare pipeline) so multi-digit
// minors sort correctly — "1.9" < "1.10" returns true. The leading
// "go" prefix on runtime.Version() is stripped before normalization.
//
// Pre-release tags (e.g. "1.24rc1") are treated as less than the
// corresponding release (semver semantics).
func goVersionLessThan(want string) bool {
	return goVersionLessThanWithVersion(runtime.Version(), want)
}

// goVersionLessThanWithVersion is the parameterized form used by
// tests: callers pass a full "goX.Y.Z" string so the comparison
// can be exercised without actually rebuilding with a different
// toolchain.
//
// WR-001: the previous implementation used a bare lexicographic `v < want`
// compare, which returned the wrong answer for multi-digit minors
// ("1.9" < "1.10" is false lexically but true semantically). This now
// uses semver.Compare.
func goVersionLessThanWithVersion(current, want string) bool {
	v := strings.TrimPrefix(current, "go")
	// semver.Canonical requires a leading "v".
	vc := semver.Canonical("v" + v)
	wc := semver.Canonical("v" + want)
	// semver.IsValid rejects pre-release / build-metadata tagged
	// versions; treat those as unknown and fall back to a
	// conservative "not less than" so prism is not chosen in that
	// edge case.
	if !semver.IsValid(vc) || !semver.IsValid(wc) {
		return false
	}
	return semver.Compare(vc, wc) < 0
}
