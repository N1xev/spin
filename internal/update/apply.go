package update

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Apply runs the upgrade plan produced by ListDeps+Resolve+the user
// (Plan 04-04's huh form). For every dep where Target differs from
// Old it shells out to `go get module@version`; then it runs
// `go mod tidy` once, then `CGO_ENABLED=0 go build ./...` as the
// smoke test (CONTEXT D-10). On any non-zero exit, Apply returns
// immediately with the wrapped error and the combined stdout+stderr
// in the message.
//
// Apply does NOT run `go test ./...` — the user's test suite is
// their concern; update is conservative (D-10). The D-10 contract
// is enforced by TestApply_DoesNotRunGoTest.
//
// Callers set Dep.Target to the version they want applied (typically
// NewStable or NewLatest from Resolver.Resolve, or the empty string
// for "skip"). Apply ignores Dep.NewStable / Dep.NewLatest on
// purpose; it only knows "go get module@version".
func Apply(deps []Dep, gomodDir string, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}

	runner := &execRunner{}

	for _, d := range deps {
		if d.Target == "" || d.Target == d.Old {
			continue
		}
		out, err := runner.run(gomodDir, nil, "go", "get", d.Module+"@"+d.Target)
		if err != nil {
			return fmt.Errorf("update: go get %s@%s: %w\n%s",
				d.Module, d.Target, err, string(out))
		}
	}

	if out, err := runner.run(gomodDir, nil, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("update: go mod tidy: %w\n%s", err, string(out))
	}

	if out, err := runner.run(gomodDir, []string{"CGO_ENABLED=0"}, "go", "build", "./..."); err != nil {
		return fmt.Errorf("update: CGO_ENABLED=0 go build ./...: %w\n%s", err, string(out))
	}

	fmt.Fprintf(log, "update: applied %d upgrade(s); build passed\n", countTargeted(deps))
	return nil
}

// countTargeted returns how many Deps would actually trigger a
// `go get`. Used only for the success log line.
func countTargeted(deps []Dep) int {
	n := 0
	for _, d := range deps {
		if d.Target != "" && d.Target != d.Old {
			n++
		}
	}
	return n
}

// execRunner is the default CommandRunner. It shells out to the
// real `go` toolchain. Apply constructs one in non-test code; tests
// inject a fakeRunner via ApplyWithRunner.
type execRunner struct{}

func (e *execRunner) run(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}
	return cmd.CombinedOutput()
}

// ApplyWithRunner is the test-friendly entry point. The production
// Apply function delegates to this with a real execRunner; tests
// pass a fakeRunner that records argv and returns canned output.
//
// The split exists so Apply's contract is unambiguous for callers
// ("go get → go mod tidy → go build, no go test") while tests can
// exercise every branch (success, single-upgrade, build failure,
// multi-upgrade batching, the no-go-test guard).
func ApplyWithRunner(runner CommandRunner, deps []Dep, gomodDir string, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}

	for _, d := range deps {
		if d.Target == "" || d.Target == d.Old {
			continue
		}
		out, err := runner.Run(gomodDir, nil, "go", "get", d.Module+"@"+d.Target)
		if err != nil {
			return fmt.Errorf("update: go get %s@%s: %w\n%s",
				d.Module, d.Target, err, string(out))
		}
	}

	if out, err := runner.Run(gomodDir, nil, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("update: go mod tidy: %w\n%s", err, string(out))
	}

	if out, err := runner.Run(gomodDir, []string{"CGO_ENABLED=0"}, "go", "build", "./..."); err != nil {
		return fmt.Errorf("update: CGO_ENABLED=0 go build ./...: %w\n%s", err, string(out))
	}

	fmt.Fprintf(log, "update: applied %d upgrade(s); build passed\n", countTargeted(deps))
	return nil
}

// CommandRunner is the seam between Apply and os/exec. The real
// implementation lives in execRunner above; tests pass a fake
// that records argv per call so they can assert ordering, count
// of `go mod tidy`, and the absence of `go test`.
type CommandRunner interface {
	Run(dir string, env []string, args ...string) ([]byte, error)
}
