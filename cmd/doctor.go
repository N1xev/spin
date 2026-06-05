package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/example/spin/internal/doctor"
)

var doctorCmd = &cobra.Command{
	Use:           "doctor",
	Short:         "Audit a Go project for build/lint/tool health",
	Long:          "doctor runs a small set of universal Go project health checks " +
		"(Go toolchain version, tool presence, go.mod hygiene, CGO_ENABLED=0 build) " +
		"on the current directory. Pass --deep to add a golangci-lint pass, --fix to " +
		"apply safe repairs (go mod tidy; go install for missing tools), --strict to " +
		"promote warnings to a non-zero exit, and --format json for a stable CI schema.",
	Args:          cobra.NoArgs,
	RunE:          runDoctor,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().String("format", "human", `output format: "human" (default, lipgloss-styled) or "json" (stable schema)`)
	doctorCmd.Flags().Bool("strict", false, "treat warnings as failures (exit 1 instead of 0)")
	doctorCmd.Flags().Bool("deep", false, "include the golangci-lint check (runs `golangci-lint run ./...`)")
	doctorCmd.Flags().Bool("fix", false, "apply safe repairs: `go mod tidy` and `go install` for missing tools")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return err
	}
	deep, err := cmd.Flags().GetBool("deep")
	if err != nil {
		return err
	}
	fix, err := cmd.Flags().GetBool("fix")
	if err != nil {
		return err
	}

	opts := doctor.RunOptions{
		Format: format,
		Strict: strict,
		Deep:   deep,
		Fix:    fix,
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	results, exitCode := doctor.Run(ctx, opts)
	if renderErr := doctor.FormatSelector(opts.Format, results, os.Stdout); renderErr != nil {
		return renderErr
	}
	if exitCode == 0 {
		return nil
	}
	failures := 0
	for _, r := range results {
		if r.Status == doctor.StatusFail {
			failures++
		}
	}
	return fmt.Errorf("doctor: %d check(s) failed", failures)
}
