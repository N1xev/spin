package wrap

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

// minGoVersionForPrism is the lowest Go version on which prism can
// be `go install`-ed. Below this, fall back to `go test` unconditionally.
const minGoVersionForPrism = "1.24"

// Test wraps `prism go test ./...` (preferred) or `go test ./...`
// (fallback) for the `spin test` subcommand. prism requires Go 1.24+;
// the version gate means the standard RunWithFallback pattern is not
// used here — this function composes its own ToolSpec and prints its
// own hint so the user knows whether prism is missing or the Go
// version is too old.
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
// than want. The comparison uses semver.Compare so multi-digit minors
// sort correctly ("1.9" < "1.10" returns true). The leading "go"
// prefix on runtime.Version() is stripped before normalization.
func goVersionLessThan(want string) bool {
	return goVersionLessThanWithVersion(runtime.Version(), want)
}

// goVersionLessThanWithVersion is the parameterized form used by
// tests so the comparison can be exercised without rebuilding the
// toolchain.
func goVersionLessThanWithVersion(current, want string) bool {
	v := strings.TrimPrefix(current, "go")
	// semver.Canonical requires a leading "v".
	vc := semver.Canonical("v" + v)
	wc := semver.Canonical("v" + want)
	// WR-001: pre-release / build-metadata versions are treated as
	// unknown — fall back to a conservative "not less than" so prism
	// is not chosen in that edge case.
	if !semver.IsValid(vc) || !semver.IsValid(wc) {
		return false
	}
	return semver.Compare(vc, wc) < 0
}
