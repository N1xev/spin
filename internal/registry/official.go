package registry

// OfficialAlias is the alias the built-in official registry is
// registered under on first run.
const OfficialAlias = "official"

// DefaultRegistryURL is the git source of the built-in official
// registry, bootstrapped on first run. It is a variable, not a
// constant, so tests can redirect it to a local fixture.
var DefaultRegistryURL = "https://github.com/spin-templates/registry"
