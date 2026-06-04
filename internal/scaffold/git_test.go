// Tests for the post-scaffold git init hook.
//
// The interesting branch is isUnknownFlagErr, which decides whether
// `git init -b main` failed because the local git is too old (< 2.28)
// — in which case we fall back to `git init` + `git symbolic-ref HEAD
// refs/heads/main` — or for some other reason, in which case we
// surface the raw git error. The test table covers every wording
// the matcher must recognize.
package scaffold

import "testing"

// TestIsUnknownFlagErr exercises every wording of "git doesn't know
// about the flag" that any released git version is known to emit.
// The test is hermetic — no git binary required, just the matcher.
func TestIsUnknownFlagErr(t *testing.T) {
	cases := []struct {
		name      string
		stderr    string
		shortFlag string
		want      bool
	}{
		// True positives: every wording + the matching flag.
		// The flag is sometimes quoted bare (`b') and sometimes with
		// a leading dash (`-b'). Both forms must match.
		{"unknown option + bare flag", "error: unknown option `b'\nusage: git init [-q] [--bare] [-b <branch-name>]", "b", true},
		{"unknown switch + bare flag", "error: unknown switch `b'\nusage: git init [-q] [--bare]", "b", true},
		{"unknown option + dashed flag", "error: unknown option `-b'\nusage: git init", "b", true},
		{"unknown switch + dashed flag", "error: unknown switch `-b'", "b", true},
		{"unrecognized option + bare flag", "fatal: unrecognized option `b'", "b", true},
		{"unrecognized switch + bare flag", "git: unrecognized switch `b'", "b", true},

		// True negatives: errors that look similar but aren't about -b.
		{"unknown branch-name (unrelated)", "error: unknown branch name 'main'", "b", false},
		{"unrelated error", "fatal: not a git repository", "b", false},
		{"empty stderr", "", "b", false},

		// Specific-flag check: a "unknown option" with the wrong flag
		// must not match (regression guard for the wording-only match).
		{"unknown option but flag -q, looking for -b", "error: unknown option `q'", "b", false},
		{"unknown switch but flag -q, looking for -b", "error: unknown switch `q'", "b", false},

		// Different short flag.
		{"looking for -q, error mentions -b", "error: unknown option `b'", "q", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isUnknownFlagErr([]byte(c.stderr), c.shortFlag)
			if got != c.want {
				t.Errorf("isUnknownFlagErr(%q, %q) = %v, want %v",
					c.stderr, c.shortFlag, got, c.want)
			}
		})
	}
}
