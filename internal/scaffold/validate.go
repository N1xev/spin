// Package scaffold: Validate enforces project name + directory constraints.
//
// ModuleSegmentRegex is the whitelist pattern for a Go module path segment.
// IsValidGoModuleSegment applies the regex + reserved-word + path-traversal
// checks. Project.Validate combines the name check with the existing-
// directory check and the --force escape hatch.
//
// SCAF-02 (reject invalid module path segments) and SCAF-08 (refuse to
// overwrite existing dir without --force) are enforced here.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModuleSegmentRegex is the whitelist pattern for a Go module path segment.
// RESEARCH §6: lowercase letters, digits, hyphens, underscores, dots; must
// start and end with a letter or digit; 2-62 chars total.
var ModuleSegmentRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,61}[a-z0-9]$`)

// reservedGoWords is the set of Go-reserved package names that cannot be
// used as a project name (they collide with stdlib packages, tooling
// directories, or are syntactic keywords that confuse go/build).
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

// validLicenses is the set of supported --license values. CR-002: the
// flag used to be free-form, which meant a typo like --license mt
// silently emitted no LICENSE file. The walker only matches
// LICENSE-<active>.tmpl case-insensitively, so without an explicit
// whitelist a bad value would never reach the error path.
var validLicenses = map[string]bool{
	"mit":        true,
	"apache-2.0": true,
	"none":       true,
}

// IsValidLicense reports whether s is one of the supported --license
// values. The check is case-insensitive after lowercase normalization
// (the walker is already case-insensitive on filenames, so callers may
// pass "MIT" or "Apache-2.0" and get the right result).
func IsValidLicense(s string) bool {
	return validLicenses[strings.ToLower(s)]
}

// IsValidTemplateRepo reports whether s is an acceptable --template-repo
// URL (TMPL-03). Permissive: we accept https://, http://, git://, and
// git@ (for ssh-agent URLs); the actual choke point is git itself, which
// returns its own error if the URL is unreachable or not a git repo.
//
// Rejected:
//
//   - empty string
//   - schemes other than http(s)/git (e.g. ftp://, file://)
//   - strings that don't look like a URL at all (no scheme prefix)
//
// file:// is intentionally rejected because the recommended workflow is
// to push the template repo to a remote and clone via the remote URL.
// Local debugging can use `git clone` manually and then point the
// embedded template's _base/ contents into a local working copy.
func IsValidTemplateRepo(s string) bool {
	if s == "" {
		return false
	}
	// SSH-agent style: git@github.com:user/repo.git
	if strings.HasPrefix(s, "git@") {
		return true
	}
	// Standard schemes. We use prefix matching instead of url.Parse
	// because url.Parse accepts relative paths and we want to reject
	// those. The scheme check is enough to catch the common typos
	// (--template-repo not-a-url, --template-repo foo/bar).
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "git://") {
		return true
	}
	return false
}

// IsValidGoModuleSegment reports whether s is acceptable as a project name
// (i.e. a Go module path segment, the directory name, and the binary name).
//
// Rules:
//   - length 2-62 inclusive
//   - matches ModuleSegmentRegex (lowercase [a-z0-9._-], start/end with
//     letter or digit)
//   - no `..` (path traversal)
//   - not a Go-reserved word (test, internal, cmd, go, golang, etc.)
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

// Validate enforces the SCAF-02 and SCAF-08 constraints on a Project:
//
//  1. Project.Name must satisfy IsValidGoModuleSegment.
//  2. Project.License must be one of the supported values
//     (mit, apache-2.0, none). CR-002.
//  3. ./<Project.Name>/ must not already exist; if it does, --force must
//     be set to proceed.
//
// Returns a descriptive error suitable for surfacing to the user. The
// error message names the constraint and gives an example invocation.
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
		// Directory exists.
		if !p.Force {
			return fmt.Errorf("directory %q already exists; pass --force to overwrite", target)
		}
		// --force: proceed (CWD is the user's responsibility).
	} else if !os.IsNotExist(err) {
		// Stat error other than "not exists" (permissions, I/O, etc.).
		return fmt.Errorf("check directory %q: %w", target, err)
	}

	return nil
}
