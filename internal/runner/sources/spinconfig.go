// Package sources holds the per-format task discovery used by
// internal/runner. Each source is self-contained: Detect() inspects
// the directory, Tasks() parses the relevant file and returns tasks.
package sources

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/spin/internal/runner"
)

// ─────────────────────────────────────────────────────────────────────
// spin.config.toml
//
// The v2.0 spin.config.toml format is intentionally minimal:
//
//	[tasks]
//	build = "go build ./..."
//	test  = "prism -v ./..."
//	dev   = { command = "air", description = "hot reload", env = ["DEBUG=1"] }
//
// Strings are shorthand for `{ command = "..." }`. The `description`
// key is surfaced via Task.Notes (used by --list and --explain).
// The `env` key is a list of "KEY=value" strings set before running
// the task (used by --explain; not yet honoured by execute.go, but
// stored here so the data flows end-to-end).
// ─────────────────────────────────────────────────────────────────────

type spinConfig struct{ name string }

func NewSpinConfig() runner.TaskSource { return &spinConfig{name: "spin.config.toml"} }

func (s *spinConfig) Name() string { return s.name }
func (s *spinConfig) Order() int   { return 100 } // highest precedence

func (s *spinConfig) Detect(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "spin.config.toml"))
	return err == nil
}

func (s *spinConfig) Tasks(dir string) ([]runner.Task, error) {
	path := filepath.Join(dir, "spin.config.toml")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Hand-rolled mini-parser: walks lines, collects task entries
	// under [tasks] until the next section header. Strings are
	// accepted as shorthand for { command = "..." }; inline tables
	// may carry description + env alongside command.
	out := []runner.Task{}
	inTasks := false
	lineno := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lineno++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inTasks = (line == "[tasks]")
			continue
		}
		if !inTasks {
			continue
		}
		// `name = value` (value may be "cmd" or { ... })
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(line[:eq])
		raw := strings.TrimSpace(line[eq+1:])
		if name == "" {
			continue
		}
		t := runner.Task{Name: name, Source: fmt.Sprintf("%d", lineno)}
		if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
			cmd, desc, env, err := parseTaskInlineTable(raw)
			if err != nil {
				// skip malformed entries; don't fail the whole parse
				continue
			}
			t.Command = cmd
			t.Notes = desc
			t.Env = env
		} else {
			// shorthand: name = "command"
			t.Command = strings.Trim(raw, "\"'")
		}
		if t.Command == "" {
			continue
		}
		out = append(out, t)
	}
	return out, sc.Err()
}

// parseTaskInlineTable decodes the `{ command = "...", description =
// "...", env = ["K=V", ...] }` form. Returns (command, description,
// env, err). The error is returned only on structural problems
// (e.g. unbalanced braces); individual unknown keys are silently
// skipped so a future schema extension doesn't break the parser.
func parseTaskInlineTable(raw string) (cmd, desc string, env []string, err error) {
	body := strings.TrimSpace(raw)
	body = strings.TrimPrefix(body, "{")
	body = strings.TrimSuffix(body, "}")
	parts := splitTopLevel(body, ',')
	for _, p := range parts {
		p = strings.TrimSpace(p)
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		switch k {
		case "command":
			cmd = strings.Trim(v, "\"'")
		case "description":
			desc = strings.Trim(v, "\"'")
		case "env":
			if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
				inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]"))
				for _, e := range splitTopLevel(inner, ',') {
					e = strings.TrimSpace(e)
					e = strings.Trim(e, "\"'")
					if e != "" {
						env = append(env, e)
					}
				}
			}
		}
	}
	if cmd == "" {
		return "", "", nil, fmt.Errorf("task: missing command")
	}
	return cmd, desc, env, nil
}

// splitTopLevel splits s on sep, but ignores separators that appear
// inside balanced brackets (e.g. for an env = ["A=1, B=2"] list, the
// inner comma must not split the outer list).
func splitTopLevel(s string, sep byte) []string {
	out := []string{}
	depth := 0
	last := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, s[last:i])
				last = i + 1
			}
		}
	}
	out = append(out, s[last:])
	return out
}
