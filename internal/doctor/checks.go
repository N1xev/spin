package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// toolSpec is one entry in the tool-presence list. Name is what
// exec.LookPath probes; InstallHint is the `go install ...` string
// shown when the tool is missing. The four tools are the same set
// scaffolded projects tell users to install (see internal/scaffold/
// templates/_base/Taskfile.yml.tmpl).
type toolSpec struct {
	Name        string
	InstallHint string
}

// defaultTools is the set probed by ToolPresenceCheck. Keep in sync
// with the install commands in the generated Taskfile.yml.
var defaultTools = []toolSpec{
	{"air", "go install github.com/air-verse/air@latest"},
	{"prism", "go install go.dalton.dog/prism@latest"},
	{"gofumpt", "go install mvdan.cc/gofumpt@latest"},
	{"goimports", "go install golang.org/x/tools/cmd/goimports@latest"},
}

// GoVersionCheck verifies that `go version` reports a Go toolchain
// new enough to build spin and the projects it scaffolds. spin itself
// pins go 1.23 in its go.mod; we accept >= 1.23 as pass, 1.21-1.22 as
// warn (oldest supported tier per the Go release policy at the time
// of writing), and < 1.21 as fail.
type GoVersionCheck struct{}

// Name implements Check.
func (GoVersionCheck) Name() string { return "go-version" }

// Run implements Check.
func (c GoVersionCheck) Run(ctx context.Context) (CheckResult, error) {
	out, err := exec.CommandContext(ctx, "go", "version").Output()
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "go not on $PATH: " + err.Error(),
			Hint:    "install Go from https://go.dev/dl/"}, nil
	}
	raw := strings.TrimSpace(string(out))
	// "go version go1.26.2 linux/amd64" -> "1.26.2".
	fields := strings.Fields(raw)
	var v string
	for _, f := range fields {
		if strings.HasPrefix(f, "go") && len(f) > 2 {
			if f[2] >= '0' && f[2] <= '9' {
				v = f[2:]
				break
			}
		}
	}
	if v == "" {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "could not parse go version: " + raw}, nil
	}
	// semver.IsValid wants a leading 'v'.
	if !semver.IsValid("v"+v) {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "could not parse go version: " + raw}, nil
	}
	normalized := "v" + v
	switch {
	case semver.Compare(normalized, "v1.23.0") >= 0:
		return CheckResult{Name: c.Name(), Status: StatusPass,
			Message: "go " + v + " (>= 1.23)"}, nil
	case semver.Compare(normalized, "v1.21.0") >= 0:
		return CheckResult{Name: c.Name(), Status: StatusWarn,
			Message: "go " + v + " is older than 1.23; some charm v2 libs require 1.25+",
			Hint:    "upgrade: https://go.dev/dl/"}, nil
	default:
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "go " + v + " is too old (< 1.21)",
			Hint:    "upgrade: https://go.dev/dl/"}, nil
	}
}

// ToolPresenceCheck probes $PATH for each tool in defaultTools. Found
// is a pass; missing is a warn with an install hint. --fix may
// install the missing ones.
type ToolPresenceCheck struct{}

// Name implements Check.
func (ToolPresenceCheck) Name() string { return "tool-presence" }

// Run implements Check.
func (c ToolPresenceCheck) Run(ctx context.Context) (CheckResult, error) {
	_ = ctx
	var found, missing []toolSpec
	for _, t := range defaultTools {
		if _, err := exec.LookPath(t.Name); err == nil {
			found = append(found, t)
		} else {
			missing = append(missing, t)
		}
	}
	switch {
	case len(missing) == 0:
		names := make([]string, len(found))
		for i, t := range found {
			names[i] = t.Name
		}
		return CheckResult{Name: c.Name(), Status: StatusPass,
			Message: "all present: " + strings.Join(names, ", ")}, nil
	case len(found) == 0:
		mnames := make([]string, len(missing))
		hints := make([]string, len(missing))
		for i, t := range missing {
			mnames[i] = t.Name
			hints[i] = t.InstallHint
		}
		return CheckResult{Name: c.Name(), Status: StatusWarn,
			Message: "no optional tools on $PATH (missing: " + strings.Join(mnames, ", ") + ")",
			Hint:    strings.Join(hints, "; ")}, nil
	default:
		mnames := make([]string, len(missing))
		for i, t := range missing {
			mnames[i] = t.Name
		}
		first := missing[0]
		return CheckResult{Name: c.Name(), Status: StatusWarn,
			Message: "missing: " + strings.Join(mnames, ", "),
			Hint:    "install one with: " + first.InstallHint}, nil
	}
}

// Fix implements Fixer. Runs `go install` for each missing tool.
// Tolerant of single failures: a tool that fails to install is
// surfaced as a multi-line hint; the remaining tools still get a
// chance to install.
func (c ToolPresenceCheck) Fix(ctx context.Context) error {
	var failed []string
	for _, t := range defaultTools {
		if _, err := exec.LookPath(t.Name); err == nil {
			continue
		}
		// 5-minute ceiling per tool: go install can pull a large
		// module graph the first time.
		cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		cmd := exec.CommandContext(cctx, "go", "install", strings.TrimPrefix(t.InstallHint, "go install "))
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v (%s)", t.Name, err, bytes.TrimSpace(out)))
		}
		cancel()
	}
	if len(failed) > 0 {
		return errors.New("could not install: " + strings.Join(failed, "; "))
	}
	return nil
}

// GoModHygieneCheck verifies the cwd contains a go.mod with the
// canonical `go` and `module` directives. It also flags a real-but-
// rare hygiene issue: a `// indirect` require that is also listed
// directly (which suggests a stale go.sum or a manual edit).
type GoModHygieneCheck struct{}

// Name implements Check.
func (GoModHygieneCheck) Name() string { return "go-mod" }

// Run implements Check.
func (c GoModHygieneCheck) Run(ctx context.Context) (CheckResult, error) {
	_ = ctx
	dir, err := os.Getwd()
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "could not get cwd: " + err.Error()}, nil
	}
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "no go.mod in " + dir,
			Hint:    "run from a Go module root, or `go mod init <module>`"}, nil
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "go.mod parse error: " + err.Error(),
			Hint:    "fix the syntax error or run `go mod tidy`"}, nil
	}
	if mf.Go == nil || mf.Go.Version == "" {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "go.mod is missing the `go` directive",
			Hint:    "add `go 1.23` (or newer) and re-run"}, nil
	}
	if mf.Module == nil || mf.Module.Mod.Path == "" {
		return CheckResult{Name: c.Name(), Status: StatusFail,
			Message: "go.mod is missing the `module` directive",
			Hint:    "add `module <path>` and re-run"}, nil
	}
	// Detect an indirect require that is also required directly.
	direct := make(map[string]bool, len(mf.Require))
	for _, r := range mf.Require {
		if !r.Indirect {
			direct[r.Mod.Path] = true
		}
	}
	for _, r := range mf.Require {
		if r.Indirect && direct[r.Mod.Path] {
			return CheckResult{Name: c.Name(), Status: StatusWarn,
				Message: r.Mod.Path + " is listed as both direct and // indirect",
				Hint:    "run `go mod tidy`"}, nil
		}
	}
	return CheckResult{Name: c.Name(), Status: StatusPass,
		Message: fmt.Sprintf("module %s, go %s", mf.Module.Mod.Path, mf.Go.Version)}, nil
}

// Fix implements Fixer. Runs `go mod tidy` with a 60s ceiling.
func (c GoModHygieneCheck) Fix(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "go", "mod", "tidy")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("`go mod tidy`: %w (%s)", err, bytes.TrimSpace(out))
	}
	return nil
}

// CGOBuildCheck runs `CGO_ENABLED=0 go build ./...` in the cwd with a
// 60s timeout. The check is universal: it works on any Go module
// regardless of how it was scaffolded.
type CGOBuildCheck struct{}

// Name implements Check.
func (CGOBuildCheck) Name() string { return "cgo-build" }

// Run implements Check.
func (c CGOBuildCheck) Run(ctx context.Context) (CheckResult, error) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "go", "build", "./...")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return CheckResult{Name: c.Name(), Status: StatusPass,
			Message: "CGO_ENABLED=0 go build ./... ok"}, nil
	}
	truncated := string(out)
	if len(truncated) > 1024 {
		truncated = truncated[:1024] + "\n... (truncated)"
	}
	return CheckResult{Name: c.Name(), Status: StatusFail,
		Message: "build failed: " + strings.TrimSpace(err.Error()) + "\n" + truncated}, nil
}

// DeepLintCheck runs `golangci-lint run ./...` if golangci-lint is
// available. Missing is a warn (per D-04: the base doctor stays fast
// and lint is opt-in). This check is only registered when
// RunOptions.Deep is true.
type DeepLintCheck struct{}

// Name implements Check.
func (DeepLintCheck) Name() string { return "lint" }

// Run implements Check.
func (c DeepLintCheck) Run(ctx context.Context) (CheckResult, error) {
	path, err := exec.LookPath("golangci-lint")
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusWarn,
			Message: "golangci-lint not on $PATH",
			Hint:    "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, path, "run", "./...")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return CheckResult{Name: c.Name(), Status: StatusPass,
			Message: "golangci-lint run ./... ok"}, nil
	}
	truncated := string(out)
	if len(truncated) > 1024 {
		truncated = truncated[:1024] + "\n... (truncated)"
	}
	return CheckResult{Name: c.Name(), Status: StatusFail,
		Message: "lint failed: " + strings.TrimSpace(err.Error()) + "\n" + truncated}, nil
}
