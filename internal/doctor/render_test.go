package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestRenderHuman_AllPass feeds three pass results to RenderHuman and
// asserts the summary line and the green check glyph appear in the
// output. We do NOT assert on raw ANSI bytes (lipgloss may emit
// different escape sequences across versions); the substring
// assertions are stable across lipgloss versions.
func TestRenderHuman_AllPass(t *testing.T) {
	var buf bytes.Buffer
	results := []CheckResult{
		{Name: "go-version", Status: StatusPass, Message: "go 1.25 ok"},
		{Name: "tool-presence", Status: StatusPass, Message: "all present"},
		{Name: "go-mod", Status: StatusPass, Message: "ok"},
	}
	if err := RenderHuman(&buf, results); err != nil {
		t.Fatalf("RenderHuman: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "3 passed, 0 warned, 0 failed") {
		t.Errorf("missing summary line; got:\n%s", out)
	}
	if !strings.Contains(out, glyphPass) {
		t.Errorf("missing pass glyph %q; got:\n%s", glyphPass, out)
	}
	if !strings.Contains(out, "go-version") || !strings.Contains(out, "go-mod") {
		t.Errorf("missing check names; got:\n%s", out)
	}
}

// TestRenderHuman_FailWithHint asserts the fail row prints a hint
// line on the row that has one. The summary must also report 1 failed.
func TestRenderHuman_FailWithHint(t *testing.T) {
	var buf bytes.Buffer
	results := []CheckResult{
		{Name: "go-version", Status: StatusFail, Message: "no go", Hint: "go install foo@latest"},
	}
	if err := RenderHuman(&buf, results); err != nil {
		t.Fatalf("RenderHuman: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0 passed, 0 warned, 1 failed") {
		t.Errorf("missing summary '0 passed, 0 warned, 1 failed'; got:\n%s", out)
	}
	if !strings.Contains(out, "hint: go install foo@latest") {
		t.Errorf("missing hint line; got:\n%s", out)
	}
	if !strings.Contains(out, glyphFail) {
		t.Errorf("missing fail glyph %q; got:\n%s", glyphFail, out)
	}
}

// TestRenderJSON_Schema asserts the JSON output decodes into the
// documented shape: a top-level "checks" array of objects with the
// four stable keys.
func TestRenderJSON_Schema(t *testing.T) {
	var buf bytes.Buffer
	results := []CheckResult{
		{Name: "go-version", Status: StatusPass, Message: "go 1.25 ok"},
		{Name: "tool-presence", Status: StatusWarn, Message: "missing air", Hint: "go install air@latest"},
		{Name: "go-mod", Status: StatusFail, Message: "no go.mod"},
	}
	if err := RenderJSON(&buf, results); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	var got struct {
		Checks []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(got.Checks) != 3 {
		t.Fatalf("expected 3 checks; got %d", len(got.Checks))
	}
	if got.Checks[0].Name != "go-version" || got.Checks[0].Status != "pass" {
		t.Errorf("first check wrong: %+v", got.Checks[0])
	}
	if got.Checks[1].Hint != "go install air@latest" {
		t.Errorf("second check hint wrong: %+v", got.Checks[1])
	}
	if got.Checks[2].Status != "fail" {
		t.Errorf("third check status wrong: %+v", got.Checks[2])
	}
}

// TestFormatSelector_RejectsUnknown asserts an unknown --format
// returns an error that names the bad value and the allowed set.
func TestFormatSelector_RejectsUnknown(t *testing.T) {
	var buf bytes.Buffer
	err := FormatSelector("yaml", nil, &buf)
	if err == nil {
		t.Fatal("expected error for unknown format; got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "yaml") {
		t.Errorf("error %q does not mention 'yaml'", msg)
	}
	if !strings.Contains(msg, "human or json") {
		t.Errorf("error %q does not mention allowed formats", msg)
	}
}

// TestFormatSelector_DefaultsToHuman asserts the empty format string
// dispatches to the human renderer.
func TestFormatSelector_DefaultsToHuman(t *testing.T) {
	var buf bytes.Buffer
	results := []CheckResult{{Name: "x", Status: StatusPass, Message: "ok"}}
	if err := FormatSelector("", results, &buf); err != nil {
		t.Fatalf("FormatSelector empty: %v", err)
	}
	if !strings.Contains(buf.String(), "1 passed, 0 warned, 0 failed") {
		t.Errorf("expected human summary; got:\n%s", buf.String())
	}
}
