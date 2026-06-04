// Package cmd wires the spin cobra root command.
//
// Subcommand files attach themselves to rootCmd via init(). RootCmd()
// is the constructor accessor; main and tests use it to obtain the
// fully-populated command tree.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/example/spin/internal/version"
)

// rootCmd is the cobra root command for spin.
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
	rootCmd.SetFlagErrorFunc(flagErrorFuncWithSuggestion)
}

// RootCmd returns the spin cobra root command with all subcommands attached.
func RootCmd() *cobra.Command {
	return rootCmd
}

// flagErrorFuncWithSuggestion augments pflag's "unknown flag" errors
// with a Levenshtein-based "Did you mean --X?" suggestion. Other
// parse errors are returned unchanged.
func flagErrorFuncWithSuggestion(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	const unknownFlagPrefix = "unknown flag: --"
	if len(err.Error()) < len(unknownFlagPrefix) || err.Error()[:len(unknownFlagPrefix)] != unknownFlagPrefix {
		return err
	}
	badName := err.Error()[len(unknownFlagPrefix):]
	for i, r := range badName {
		if r == ' ' || r == '=' || r == '\'' || r == '"' {
			badName = badName[:i]
			break
		}
	}

	suggestion := closestFlag(cmd, badName, 2)
	if suggestion == "" {
		return err
	}
	return &FlagSuggestionError{
		Original:   err,
		Suggestion: suggestion,
	}
}

// FlagSuggestionError wraps the original pflag error with a
// "did you mean" suggestion.
type FlagSuggestionError struct {
	Original   error
	Suggestion string
}

func (e *FlagSuggestionError) Error() string {
	return e.Original.Error() + "\nDid you mean --" + e.Suggestion + "?"
}

// Unwrap exposes the original error for errors.Is/As.
func (e *FlagSuggestionError) Unwrap() error { return e.Original }

// closestFlag returns the registered flag whose name is closest to
// bad within maxDist, or "" if no match.
func closestFlag(cmd *cobra.Command, bad string, maxDist int) string {
	best := ""
	bestDist := maxDist + 1

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

// levenshtein computes the edit distance between a and b. When icase
// is true, the comparison is case-insensitive.
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
			if a[i-1] == b[j-1] || (icase && toLowerByte(a[i-1]) == toLowerByte(b[j-1])) {
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
		b[i] = toLowerByte(s[i])
	}
	return string(b)
}

func toLowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
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
