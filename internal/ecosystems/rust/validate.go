package rust

import (
	"fmt"
	"strings"

	"github.com/example/spin/internal/ecosystem"
)

// Validate runs cross-flag rules that depend on multiple values.
func (e *Ecosystem) Validate(ctx ecosystem.Context) error {
	// Defensive: type must be bin, lib, or example.
	// (ChoiceFlag should already enforce this, but double-check.)
	switch t := ctx.GetString("type"); t {
	case "bin", "lib", "example":
		// ok
	case "":
		return ecosystem.NewValidationError(e.Name(),
			"project --type is required (bin, lib, or example)")
	default:
		return ecosystem.NewValidationError(e.Name(),
			fmt.Sprintf("invalid --type=%q (must be bin, lib, or example)", t))
	}

	// Edition must be a known cargo edition.
	switch ed := ctx.GetString("edition"); ed {
	case "2015", "2018", "2021", "2024":
		// ok
	case "":
		return ecosystem.NewValidationError(e.Name(),
			"--edition is required (2015, 2018, 2021, or 2024)")
	default:
		return ecosystem.NewValidationError(e.Name(),
			fmt.Sprintf("invalid --edition=%q (must be 2015, 2018, 2021, or 2024)", ed))
	}

	// rust-version (MSRV) must be non-empty.
	if strings.TrimSpace(ctx.GetString("rust-version")) == "" {
		return ecosystem.NewValidationError(e.Name(),
			"--rust-version is required (e.g. 1.75)")
	}

	return nil
}
