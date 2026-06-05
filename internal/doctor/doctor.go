// Package doctor audits a Go project for build/lint/tool health.
//
// doctor is universal: it operates on the current working directory and
// the surrounding module. It is not aware of spin-scaffolded layout
// beyond what a vanilla `go.mod` gives it. The default registry runs
// four checks: Go version, tool presence (air, prism, gofumpt,
// goimports), go.mod hygiene, and a CGO_ENABLED=0 build smoke test.
// `RunOptions.Deep` adds a fifth check that shells out to
// golangci-lint; `RunOptions.Fix` lets the registered Fixer
// implementations apply safe repairs (go mod tidy, go install for
// missing tools).
//
// Exit codes follow the go vet / golangci-lint convention: 0 = all
// pass or all warn with --strict unset, 1 = any fail OR (--strict and
// any warn). The CLI subcommand (cmd/doctor.go) wraps the exit code
// into a cobra error so fang renders a styled failure.
package doctor

import (
	"context"
	"fmt"
)

// Status is the result of a single check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// CheckResult is one row in the doctor report. Stable JSON shape; the
// keys in render.go's jsonCheck mirror these field names.
type CheckResult struct {
	Name    string `json:"-"`
	Status  Status `json:"-"`
	Message string `json:"-"`
	Hint    string `json:"-"`
}

// Check is the interface every check implements. Name is the stable
// identifier used in the JSON output; Run performs the check and
// returns a result plus an optional error. An error is treated as
// Status=fail with err.Error() appended to the result's Message.
type Check interface {
	Name() string
	Run(ctx context.Context) (CheckResult, error)
}

// Fixer is an optional interface a Check may implement to support
// `spin doctor --fix`. Only GoModHygieneCheck and ToolPresenceCheck
// implement it; GoVersionCheck and CGOBuildCheck have nothing safe to
// repair automatically.
type Fixer interface {
	Fix(ctx context.Context) error
}

// Registry accumulates the checks to run. Doctor populates it from
// RunOptions; tests can register stubs directly.
type Registry struct {
	checks []Check
}

// Register appends a check to the registry.
func (r *Registry) Register(c Check) {
	if c == nil {
		return
	}
	r.checks = append(r.checks, c)
}

// RunAll executes every registered check, in registration order, and
// returns the result list. Errors from Check.Run are converted to
// Status=fail with the error message included in CheckResult.Message;
// RunAll itself never returns an error.
func (r *Registry) RunAll(ctx context.Context) []CheckResult {
	out := make([]CheckResult, 0, len(r.checks))
	for _, c := range r.checks {
		res, err := c.Run(ctx)
		if err != nil {
			res.Name = c.Name()
			if res.Status == "" {
				res.Status = StatusFail
			}
			if res.Message == "" {
				res.Message = err.Error()
			} else {
				res.Message = res.Message + ": " + err.Error()
			}
		}
		out = append(out, res)
	}
	return out
}

// RunOptions controls which checks run and how results are interpreted.
type RunOptions struct {
	Strict bool   // warnings become exit 1
	Fix    bool   // call Fixer.Fix on registered checks
	Deep   bool   // include golangci-lint check
	Format string // "human" or "json" (default "human"); consumed by the renderer
}

// DefaultRegistry returns the four universal checks plus DeepLintCheck
// when opts.Deep is true. The Deep check is always constructed (cheap)
// and skipped by the registry runner when opts.Deep is false; that
// keeps the construction logic in one place.
func DefaultRegistry(opts RunOptions) *Registry {
	r := &Registry{}
	r.Register(&GoVersionCheck{})
	r.Register(&ToolPresenceCheck{})
	r.Register(&GoModHygieneCheck{})
	r.Register(&CGOBuildCheck{})
	if opts.Deep {
		r.Register(&DeepLintCheck{})
	}
	return r
}

// Run is the top-level orchestrator. It runs the default registry,
// optionally invokes Fix on checks that implement Fixer, then computes
// the exit code. The exit code is the only contract the CLI cares
// about: 0 = ok, 1 = any failure or warn-under-strict.
//
// Fix errors are surfaced in CheckResult.Message (with a short prefix)
// rather than returned as a function error, so the caller can keep
// rendering the report even when one repair failed.
func Run(ctx context.Context, opts RunOptions) ([]CheckResult, int) {
	reg := DefaultRegistry(opts)
	results := reg.RunAll(ctx)

	if opts.Fix {
		for _, c := range reg.checks {
			f, ok := c.(Fixer)
			if !ok {
				continue
			}
			if err := f.Fix(ctx); err != nil {
				results = annotateFixFailure(results, c.Name(), err)
			}
		}
	}

	return results, exitCode(results, opts.Strict)
}

// annotateFixFailure finds the row for name and tacks the fix error
// onto its Message. If the row is missing (it shouldn't be — Fix only
// runs on registered checks) it appends a new fail row so the user
// sees the failure in the report.
func annotateFixFailure(results []CheckResult, name string, err error) []CheckResult {
	for i := range results {
		if results[i].Name != name {
			continue
		}
		results[i].Message = fmt.Sprintf("fix failed: %s (prior: %s)", err, results[i].Message)
		if results[i].Status == StatusPass {
			results[i].Status = StatusWarn
		}
		return results
	}
	return append(results, CheckResult{
		Name:    name,
		Status:  StatusFail,
		Message: "fix failed: " + err.Error(),
	})
}

// exitCode implements D-03: 0 when all pass or all warn (Strict=false),
// 1 when any fail OR (Strict=true and any warn).
func exitCode(results []CheckResult, strict bool) int {
	for _, r := range results {
		if r.Status == StatusFail {
			return 1
		}
	}
	if strict {
		for _, r := range results {
			if r.Status == StatusWarn {
				return 1
			}
		}
	}
	return 0
}
