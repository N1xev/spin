// Package scaffold: post-scaffold build verification.
package scaffold

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/log/v2"
)

// VerifyBuildTimeout caps how long each post-scaffold step may take.
const VerifyBuildTimeout = 2 * time.Minute

// runCmd is the shared exec helper. It sets cmd.Dir and appends env
// to os.Environ(). It returns the combined stdout+stderr and a non-nil
// error if the command exited non-zero.
func runCmd(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}
	return cmd.CombinedOutput()
}

// runCmdTimeout is runCmd with a per-call context timeout. On expiry
// the returned error names the deadline so the user can distinguish
// "command slow" from "command wrong".
func runCmdTimeout(dir string, timeout time.Duration, env []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return out, fmt.Errorf("`%s` timed out after %s",
				strings.Join(args, " "), timeout)
		}
		return out, err
	}
	return out, nil
}

// VerifyBuild runs the post-scaffold smoke test: `go mod tidy`,
// then `go build ./...` with CGO_ENABLED=0, then `go test ./...`.
// The tidy step is required because a fresh scaffold has no go.sum,
// and `go build` would fail with "missing go.sum entry" before the
// actual scaffold errors could surface. Each step is wrapped in
// VerifyBuildTimeout. If p.NoVerify is set, returns nil immediately.
func (p *Project) VerifyBuild() error {
	if p.NoVerify {
		log.Info("skipping verify (--no-verify)", "path", p.Name)
		return nil
	}

	root := filepath.Join(".", p.Name)
	log.Info("verifying build", "path", root)

	tidyArgs := []string{"go", "mod", "tidy"}
	if out, err := runCmdTimeout(root, VerifyBuildTimeout, nil, tidyArgs...); err != nil {
		return fmt.Errorf("`%s` failed in %s:\n%s",
			strings.Join(tidyArgs, " "), root, string(out))
	}

	buildArgs := []string{"go", "build", "./..."}
	if out, err := runCmdTimeout(root, VerifyBuildTimeout, []string{"CGO_ENABLED=0"}, buildArgs...); err != nil {
		log.Error("smoke test failed", "step", "build", "output", string(out))
		return fmt.Errorf("`%s` failed in %s:\n%s",
			strings.Join(buildArgs, " "), root, string(out))
	}

	testArgs := []string{"go", "test", "./..."}
	if out, err := runCmdTimeout(root, VerifyBuildTimeout, nil, testArgs...); err != nil {
		log.Error("smoke test failed", "step", "test", "output", string(out))
		return fmt.Errorf("`%s` failed in %s:\n%s",
			strings.Join(testArgs, " "), root, string(out))
	}

	log.Info("smoke test passed", "path", root)
	return nil
}
