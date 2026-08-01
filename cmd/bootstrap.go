package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/N1xev/spin/internal/registry"
)

// maybeBootstrapOfficial registers the built-in official registry on
// first run, when registries.json does not yet exist, and reports it
// to the user. It is a no-op once the file is present, including when
// the user removed the official registry on purpose. Failures (e.g.
// no network) are surfaced as a warning and the caller proceeds; the
// user can retry on a later command.
func maybeBootstrapOfficial(ctx context.Context, mgr *registry.Manager) {
	did, err := mgr.Bootstrap(ctx)
	switch {
	case err != nil:
		printWarn("could not bootstrap official registry: %v", err)
	case did:
		printInfo("bootstrapped official registry")
	}
}

// annotateShorthandError enriches a shorthand resolution failure with
// a re-add hint when the missing alias is the built-in official
// registry. The registry package only reports the structured
// AliasNotRegisteredError; the hint is CLI presentation, so it lives
// here. Returns the enriched error and true, or the original error
// and false when nothing applies (nil, other failure kinds, or a
// non-official alias with no known source).
func annotateShorthandError(err error) (error, bool) {
	var notRegistered registry.AliasNotRegisteredError
	if errors.As(err, &notRegistered) && notRegistered.Alias == registry.OfficialAlias {
		return fmt.Errorf("%w (re-add it with: spin registry add %s %s)", err, registry.OfficialAlias, registry.DefaultRegistryURL), true
	}
	return err, false
}
