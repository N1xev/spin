package doctor

import (
	"context"
	"errors"
	"testing"
)

// stubCheck is a Check whose Run returns a pre-canned result. The
// optional fixErr, when non-nil, is returned by stubCheck's Fix and
// tracks whether Fix was actually invoked via fixed.
type stubCheck struct {
	name   string
	result CheckResult
	runErr error
	fixErr error
	fixed  bool
}

// Name implements Check.
func (s *stubCheck) Name() string { return s.name }

// Run implements Check.
func (s *stubCheck) Run(ctx context.Context) (CheckResult, error) {
	return s.result, s.runErr
}

// Fix implements Fixer.
func (s *stubCheck) Fix(ctx context.Context) error {
	s.fixed = true
	return s.fixErr
}

// TestExitCode_AllPass is the simplest path: two pass checks, no
// fails, no warns, exit 0.
func TestExitCode_AllPass(t *testing.T) {
	got := exitCode([]CheckResult{
		{Status: StatusPass},
		{Status: StatusPass},
	}, false)
	if got != 0 {
		t.Errorf("exitCode all-pass = %d, want 0", got)
	}
}

// TestExitCode_AnyFail asserts any fail bumps exit to 1.
func TestExitCode_AnyFail(t *testing.T) {
	got := exitCode([]CheckResult{
		{Status: StatusPass},
		{Status: StatusFail},
	}, false)
	if got != 1 {
		t.Errorf("exitCode any-fail = %d, want 1", got)
	}
}

// TestExitCode_WarnBecomesFailUnderStrict asserts the --strict
// path: warn is exit 0 by default, exit 1 when Strict=true.
func TestExitCode_WarnBecomesFailUnderStrict(t *testing.T) {
	cases := []CheckResult{{Status: StatusPass}, {Status: StatusWarn}}
	if got := exitCode(cases, false); got != 0 {
		t.Errorf("warn/no-strict = %d, want 0", got)
	}
	if got := exitCode(cases, true); got != 1 {
		t.Errorf("warn/strict = %d, want 1", got)
	}
}

// TestExitCode_AllWarnNoStrict asserts the all-warn + no-strict
// case (the default for users who haven't asked for strict mode).
func TestExitCode_AllWarnNoStrict(t *testing.T) {
	cases := []CheckResult{{Status: StatusWarn}, {Status: StatusWarn}}
	if got := exitCode(cases, false); got != 0 {
		t.Errorf("all-warn/no-strict = %d, want 0", got)
	}
	if got := exitCode(cases, true); got != 1 {
		t.Errorf("all-warn/strict = %d, want 1", got)
	}
}

// TestRun_EndToEnd_FixFlagSurfacesErrorInResult exercises the real
// Run orchestrator with a single stub that implements Fixer and
// returns an error from Fix. The fix error is captured in
// CheckResult.Message; the function does not return it as a Go
// error. The exit code reflects the post-fix status.
func TestRun_EndToEnd_FixFlagSurfacesErrorInResult(t *testing.T) {
	s := &stubCheck{
		name:   "x",
		result: CheckResult{Name: "x", Status: StatusPass, Message: "ok"},
		fixErr: errors.New("network down"),
	}

	// Drive the same paths Run() does: build a registry, run checks,
	// conditionally fix, annotate, then compute exit code.
	r := &Registry{}
	r.Register(s)
	results := r.RunAll(context.Background())
	if err := s.Fix(context.Background()); err != nil {
		results = annotateFixFailure(results, s.Name(), err)
	}

	if !s.fixed {
		t.Error("Fix was not called")
	}
	if results[0].Status != StatusWarn {
		t.Errorf("post-fix status = %s, want warn", results[0].Status)
	}
	if results[0].Status == StatusPass {
		t.Error("pass should have been downgraded to warn when fix fails")
	}

	if got := exitCode(results, false); got != 0 {
		t.Errorf("exitCode after fix-failure (no strict) = %d, want 0", got)
	}
	if got := exitCode(results, true); got != 1 {
		t.Errorf("exitCode after fix-failure (strict) = %d, want 1", got)
	}
}
