package wrap

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
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
// than want. It uses lexicographic string comparison on the
// semver portion of runtime.Version() (the leading "go" prefix is
// stripped), which is correct for "1.24" < "1.25" etc.
//
// This is intentionally simple — a full semver parser is overkill
// for "is this at least 1.24?". If the comparison ever needs to
// handle pre-release tags like "1.24rc1", swap in golang.org/x/mod/semver.
func goVersionLessThan(want string) bool {
	v := strings.TrimPrefix(runtime.Version(), "go")
	return v < want
}
