// Package scaffold: Validate enforces project name + directory constraints
// (SCAF-02, SCAF-08).
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModuleSegmentRegex is the whitelist pattern for a Go module path segment.
// RESEARCH §6: lowercase letters, digits, hyphens, underscores, dots;
// start and end with a letter or digit; 2-62 chars total.
var ModuleSegmentRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,61}[a-z0-9]$`)

// reservedGoWords collides with stdlib packages, tooling directories,
// or are syntactic keywords that confuse go/build.
var reservedGoWords = map[string]bool{
	"test":    true,
	"tests":   true,
	"_test":   true,
	"vendor":  true,
	"internal": true,
	"cmd":     true,
	"go":      true,
	"golang":  true,
}

// validLicenses is the supported --license whitelist (CR-002). The
// walker only matches LICENSE-<active>.tmpl case-insensitively, so
// without an explicit whitelist a bad value would never reach the
// error path (it would silently emit no LICENSE).
var validLicenses = map[string]bool{
	"mit":        true,
	"apache-2.0": true,
	"none":       true,
}

// IsValidLicense reports whether s is a supported --license value.
// Case-insensitive (the walker is case-insensitive on filenames too,
// so callers may pass "MIT" or "Apache-2.0" and get the right result).
func IsValidLicense(s string) bool {
	return validLicenses[strings.ToLower(s)]
}

// IsValidTemplateRepo reports whether s is an acceptable --template-repo
// URL (TMPL-03). Permissive — git itself is the real choke point; an
// unreachable URL or non-git path returns a meaningful error from the
// clone step. file:// is accepted (standard git protocol, useful for
// local dev and smoke tests).
func IsValidTemplateRepo(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "git@") {
		return true
	}
	// Prefix matching (not url.Parse) so relative paths are rejected.
	// The scheme check is enough to catch common typos.
	var scheme string
	switch {
	case strings.HasPrefix(s, "https://"):
		scheme = "https://"
	case strings.HasPrefix(s, "http://"):
		scheme = "http://"
	case strings.HasPrefix(s, "git://"):
		scheme = "git://"
	case strings.HasPrefix(s, "file://"):
		scheme = "file://"
	default:
		return false
	}
	// CR-004: defense-in-depth — reject URLs whose first path segment
	// starts with `-` (e.g. "https://x.com/-evil"). Git's `--` separator
	// in CloneTemplateRepo is the primary mitigation; this is the
	// validator-side belt to that suspenders.
	rest := strings.TrimPrefix(s, scheme)
	rest = strings.TrimLeft(rest, "/")
	if rest == "" {
		// Scheme with no host is malformed.
		return false
	}
	pathStart := strings.IndexAny(rest, "/?#")
	if pathStart < 0 {
		// No path — just a host. Nothing to check on the path side.
		return true
	}
	if pathStart+1 < len(rest) && rest[pathStart+1] == '-' {
		return false
	}
	return true
}

// IsValidGoModuleSegment reports whether s is acceptable as a project
// name (Go module path segment, directory name, binary name). Length
// 2-62, matches ModuleSegmentRegex, no `..`, not a Go-reserved word.
func IsValidGoModuleSegment(s string) bool {
	if len(s) < 2 || len(s) > 62 {
		return false
	}
	if !ModuleSegmentRegex.MatchString(s) {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	if reservedGoWords[s] {
		return false
	}
	return true
}

// Validate enforces SCAF-02 (name regex) and SCAF-08 (refuse to overwrite
// existing dir without --force) on a Project. CR-002: license must be in
// the whitelist.
func (p *Project) Validate() error {
	if p == nil {
		return fmt.Errorf("scaffold: project is nil")
	}
	if !IsValidGoModuleSegment(p.Name) {
		return fmt.Errorf(
			"invalid project name %q: must be 2-62 chars, lowercase [a-z0-9._-], "+
				"start and end with a letter or digit, and not be a Go-reserved word "+
				"(test, internal, cmd, go, golang, vendor, _test, tests); "+
				"see example: spin new myapp --tui --bubbletea",
			p.Name,
		)
	}
	// CR-002: normalize case before the whitelist check so a user
	// passing --license MIT or --license Apache-2.0 still succeeds.
	if !IsValidLicense(p.License) {
		return fmt.Errorf(
			"--license %q is not supported; valid options: mit, apache-2.0, none",
			p.License,
		)
	}

	target := filepath.Join(".", p.Name)
	if _, err := os.Stat(target); err == nil {
		if !p.Force {
			return fmt.Errorf("directory %q already exists; pass --force to overwrite", target)
		}
		// --force: proceed (CWD is the user's responsibility).
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check directory %q: %w", target, err)
	}

	return nil
}
