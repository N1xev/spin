// Package cmd wires the spin cobra root command.
//
// rootCmd is a package-level variable so that subcommand files can attach
// themselves via init(). RootCmd() is the constructor accessor for callers
// (main, tests) and returns the same singleton.
//
// FLAG-18 (unknown flag → suggestion) is enforced by rootCmd.FlagErrorFunc:
// when pflag returns "unknown flag: --X", we Levenshtein-match X against
// every registered flag on the command. cobra's built-in
// SuggestionsMinimumDistance only handles command suggestions, not flag
// suggestions, so this is implemented here.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/example/spin/internal/version"
)

// rootCmd is the cobra root command for spin. Subcommand files in this
// package attach themselves to rootCmd via init().
var rootCmd = &cobra.Command{
	Use:   "spin",
	Short: "Scaffold a charmbracelet v2 Go project",
	Long: "spin is a Go project scaffolder for the charmbracelet v2 ecosystem.\n" +
		"It generates ready-to-run Go projects — TUI apps, CLI tools, or both — " +
		"pre-wired with the right charmbracelet libraries, modern Go tooling " +
		"(cobra, fang, gum), hot reload (air), and the prism test runner.\n\n" +
		"One command produces a project that builds, tests, and runs without extra setup.\n\n" +
		"Example:\n" +
		"  spin new myapp --tui --bubbletea --bubbles --lipgloss\n\n" +
		"See `spin new --help` for available flags.",
	Version:                    version.Version,
	SilenceUsage:               true,
	SilenceErrors:              true,
	SuggestionsMinimumDistance: 2,
}

func init() {
	// FlagErrorFunc must be set via the setter (cobra doesn't expose the
	// field directly). Sets the custom flag-error handler that adds
	// "Did you mean --X?" suggestions for unknown flags (FLAG-18).
	rootCmd.SetFlagErrorFunc(flagErrorFuncWithSuggestion)
}

// RootCmd returns the spin cobra root command with all subcommands attached.
// Tests use this to construct a fresh tree; main uses this as the entry point
// passed to fang.Execute.
func RootCmd() *cobra.Command {
	return rootCmd
}

// flagErrorFuncWithSuggestion wraps cobra's default flag-error behavior
// with a Levenshtein-based "Did you mean --X?" suggestion. Returns the
// original error unchanged if no close-enough flag is found.
//
// pflag emits "unknown flag: --X" for unknown flags; we parse the name
// out of that string, compare against every registered flag (on the
// current command and its parents), and append a suggestion if the
// best match is within SuggestionsMinimumDistance.
func flagErrorFuncWithSuggestion(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	// Only augment "unknown flag" errors. Other parse errors (e.g. "flag
	// needs an argument") are returned as-is.
	const unknownFlagPrefix = "unknown flag: --"
	if len(err.Error()) < len(unknownFlagPrefix) || err.Error()[:len(unknownFlagPrefix)] != unknownFlagPrefix {
		return err
	}
	badName := err.Error()[len(unknownFlagPrefix):]
	// Trim any trailing description.
	for i, r := range badName {
		if r == ' ' || r == '=' || r == '\'' || r == '"' {
			badName = badName[:i]
			break
		}
	}

	// Cobra's SuggestionsMinimumDistance is auto-set to 2 only for command
	// suggestions (see findSuggestions in cobra/command.go). For flag
	// suggestions we use a fixed threshold of 2 — close enough to catch
	// most typos without flooding the user with garbage suggestions.
	const maxFlagDist = 2
	suggestion := closestFlag(cmd, badName, maxFlagDist)
	if suggestion == "" {
		return err
	}
	return &FlagSuggestionError{
		Original:   err,
		Suggestion: suggestion,
	}
}

// FlagSuggestionError wraps the original pflag error with a "did you mean"
// suggestion. The Error() string is what fang styles.
type FlagSuggestionError struct {
	Original   error
	Suggestion string
}

func (e *FlagSuggestionError) Error() string {
	return e.Original.Error() + "\nDid you mean --" + e.Suggestion + "?"
}

// Unwrap exposes the original error for errors.Is/As.
func (e *FlagSuggestionError) Unwrap() error { return e.Original }

// closestFlag returns the registered flag whose name is closest to bad
// (Levenshtein distance) within maxDist. Returns "" if no match.
func closestFlag(cmd *cobra.Command, bad string, maxDist int) string {
	best := ""
	bestDist := maxDist + 1

	// Walk this command and all parents (so subcommand flags match too).
	for c := cmd; c != nil; c = c.Parent() {
		c.Flags().VisitAll(func(f *pflag.Flag) {
			d := levenshtein(bad, f.Name, true)
			if d < bestDist {
				bestDist = d
				best = f.Name
			}
		})
	}
	if bestDist > maxDist {
		return ""
	}
	return best
}

// levenshtein computes the edit distance between a and b. If icase is
// true, the comparison is case-insensitive. Standard DP implementation;
// O(len(a)*len(b)) which is fine for short flag names.
func levenshtein(a, b string, icase bool) int {
	if icase {
		a = toLower(a)
		b = toLower(b)
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
