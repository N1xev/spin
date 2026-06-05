package wrap

import (
	"fmt"
	"os"
	"os/exec"
)

// golangciLintInstallHint is the canonical install command for
// golangci-lint, kept in sync with the hint printed by
// internal/doctor/checks.go's DeepLintCheck. Duplicated (rather
// than shared) to avoid a cross-package import cycle: the wrap
// package must stay importable by both the cobra subcommand layer
// and the doctor package without any coupling.
const golangciLintInstallHint = "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"

// Lint wraps `golangci-lint` for the `spin lint` subcommand. All
// args are forwarded verbatim so subcommands like `cache clean`
// and flags like `--fix` work transparently. When golangci-lint is
// missing on $PATH, a one-line install hint is printed to stderr
// and a non-nil error is returned so fang reports a non-zero exit.
//
// Lint does not use RunWithFallback because there is no sensible
// fallback for a linter — silently running `go vet` would
// downgrade the user's lint signal without their consent.
func Lint(args []string) error {
	if path, err := exec.LookPath("golangci-lint"); err == nil {
		return runTool(path, args, nil)
	}
	fmt.Fprintf(os.Stderr,
		"hint: golangci-lint not found on $PATH; install with: %s\n",
		golangciLintInstallHint)
	return fmt.Errorf("golangci-lint not found on $PATH")
}
