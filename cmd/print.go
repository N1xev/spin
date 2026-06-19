// Package cmd: shared output helpers.
//
// Every spin command that prints a success line, info line, error
// line, warning, or table goes through this file. We deliberately
// reuse the same fang color scheme the error path uses, so the
// whole CLI reads as one cohesive surface (success = green like
// the ANSI flag color; info = blue like the title; warning =
// yellow/orange; hint = dimmed base). The marks ("✓", "ℹ", "⚠",
// "✗") are unicode dingbats -- they look at home next to fang's
// fang styles and stay one codepoint per call site.
//
// If you find yourself reaching for fmt.Printf or fmt.Fprintln
// directly in another cmd file, add a helper here instead.
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Styles. Built once at package init; lipgloss v2 styles are safe
// to share across goroutines. The colour values mirror fang's
// DefaultColorScheme so success/info/warn match the fang error
// header visually.
var (
	styleSuccessMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	styleInfoMark    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	styleWarnMark    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	styleErrorMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Bold(true)
	styleDim         = lipgloss.NewStyle().Faint(true)
	styleHeader      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	styleCell        = lipgloss.NewStyle()
)

// printSuccess writes "✓ <msg>" to stdout, fang-flag-green. Use
// for actions the user just performed: "added", "created", "pinned".
func printSuccess(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "%s %s\n",
		styleSuccessMark.Render("✓"),
		fmt.Sprintf(format, args...))
}

// printInfo writes "ℹ <msg>" to stdout, fang-title-blue. Use for
// neutral confirmations and "no items" messages.
func printInfo(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "%s %s\n",
		styleInfoMark.Render("ℹ"),
		fmt.Sprintf(format, args...))
}

// printWarn writes "⚠ <msg>" to stderr, amber. Use for conditions
// the user should know about but which are not fatal (e.g. a
// template that requires a newer spin).
func printWarn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n",
		styleWarnMark.Render("⚠"),
		fmt.Sprintf(format, args...))
}

// printHint writes a dimmed hint to stderr. Used after an error or
// an "info" line to suggest the next step (e.g. "Use `spin new
// --template <user/repo>` to scaffold from a git repo.").
func printHint(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s\n",
		styleDim.Render(fmt.Sprintf(format, args...)))
}

// printTable renders a styled table to w. headers is the column
// names; rows is the data (each row same length as headers).
// Columns are sized to the widest cell. Used by `spin list` and any
// future tabular output.
//
// Padding is applied via a 2-space left pad in StyleFunc. We don't
// use BorderColumn(true) because that draws box-drawing characters
// that look heavy in a pipelined/non-TTY context; plain padding
// reads cleanly in both.
func printTable(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	pad := lipgloss.NewStyle().PaddingLeft(2)
	headerStyle := pad.Inherit(styleHeader)
	cellStyle := pad.Inherit(styleCell)
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	fmt.Fprintln(w, t.Render())
}

// truncate is a tiny string helper for table cells. We keep it
// here next to printTable so cmd/list.go doesn't need its own copy.
func truncate(s string, n int) string {
	if n <= 1 || len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], " ") + "…"
}
