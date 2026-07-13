package cmd

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/N1xev/spin/internal/log"
)

// Style for the table rendered by printTable.
var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	styleCell   = lipgloss.NewStyle()
)

// printSuccess logs a success message to stdout. The charmbracelet/log
// library's INFO level (green) is the user-facing indicator — no
// custom icons needed.
func printSuccess(format string, args ...any) {
	log.Stdout.Info(fmt.Sprintf(format, args...))
}

// printInfo logs an informational message to stdout via the log
// library's INFO level.
func printInfo(format string, args ...any) {
	log.Stdout.Info(fmt.Sprintf(format, args...))
}

// printWarn logs a warning to stderr via the log library's WARN
// level (yellow prefix).
func printWarn(format string, args ...any) {
	log.Warn(fmt.Sprintf(format, args...))
}

// printHint prints a hint to stdout with no level prefix. Used after
// an error or info line to suggest the next step.
func printHint(format string, args ...any) {
	log.Stdout.Print(fmt.Sprintf(format, args...))
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
	_, _ = fmt.Fprintln(w, t.Render())
}

// truncate is a tiny string helper for table cells. We keep it
// here next to printTable so cmd/list.go doesn't need its own copy.
func truncate(s string, n int) string {
	if n <= 1 || len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n-1], " ") + "…"
}
