// Package scaffold: post-scaffold hooks.
//
// VerifyBuild runs `go mod tidy` (populates go.sum), then `go build ./...`
// with CGO_ENABLED=0, then `go test ./...` in the generated project.
// Failures surface the go command's stderr verbatim so the user can see
// exactly which template produced the bad output.
//
// SCAF-05 (generated project builds), SCAF-06 (smoke test runs + reports
// clearly), and TOOL-05 (smoke test passes end-to-end) are enforced here.
package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
)

// runCmd is the shared exec helper used by VerifyBuild and GitInit.
// It sets cmd.Dir and appends extra env to os.Environ().
//
// Returns the combined stdout+stderr output and a non-nil error if the
// command exited non-zero.
func runCmd(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

// VerifyBuild runs the post-scaffold smoke test on the generated project.
//
// Order: `go mod tidy` (populates go.sum for fresh scaffolds) -> `go build
// ./...` with CGO_ENABLED=0 -> `go test ./...`. The tidy step is needed
// because a fresh scaffold has no go.sum, and `go build` would fail with
// "missing go.sum entry" before any of the actual v1-leak or wrong-pin
// checks could fire. (See Walking Skeleton deviation #4.)
//
// If p.NoVerify is set, returns nil immediately without exec'ing anything.
func (p *Project) VerifyBuild() error {
	if p.NoVerify {
		log.Info("skipping verify (--no-verify)", "path", p.Name)
		return nil
	}

	root := filepath.Join(".", p.Name)
	log.Info("verifying build", "path", root)

	// Step 1: go mod tidy (no CGO override; tidy doesn't compile).
	tidyArgs := []string{"go", "mod", "tidy"}
	if out, err := runCmd(root, nil, tidyArgs...); err != nil {
		return fmt.Errorf("`%s` failed in %s:\n%s",
			strings.Join(tidyArgs, " "), root, string(out))
	}

	// Step 2: go build ./... with CGO_ENABLED=0.
	buildArgs := []string{"go", "build", "./..."}
	if out, err := runCmd(root, []string{"CGO_ENABLED=0"}, buildArgs...); err != nil {
		log.Error("smoke test failed", "step", "build", "output", string(out))
		return fmt.Errorf("`%s` failed in %s:\n%s",
			strings.Join(buildArgs, " "), root, string(out))
	}

	// Step 3: go test ./... (no env override; tests can use whatever they need).
	testArgs := []string{"go", "test", "./..."}
	if out, err := runCmd(root, nil, testArgs...); err != nil {
		log.Error("smoke test failed", "step", "test", "output", string(out))
		return fmt.Errorf("`%s` failed in %s:\n%s",
			strings.Join(testArgs, " "), root, string(out))
	}

	log.Info("smoke test passed", "path", root)
	return nil
}
