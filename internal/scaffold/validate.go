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
// URL (TMPL-03). Permissive: we accept any of https://, http://, git://,
// file://, and git@ (for ssh-agent URLs). git itself is the real choke
// point — an unreachable URL or a non-git path returns a meaningful
// error from the clone step.
//
// Rejected:
//
//   - empty string
//   - schemes that git does not support (e.g. ftp://)
//   - strings that don't look like a URL at all (no scheme prefix and
//     no git@ prefix)
//   - URLs whose first non-scheme path segment starts with `-` (CR-004):
//     even with the `--` separator in CloneTemplateRepo, a leading-dash
//     path is almost certainly a typo or an attack and is rejected at
//     the validator too as defense-in-depth.
//
// file:// is accepted because it's a standard git protocol (useful for
// local development and the smoke tests). The recommended workflow for
// distributed templates is still to push to a remote and clone via
// https://, but we don't force that.
func IsValidTemplateRepo(s string) bool {
	if s == "" {
		return false
	}
	// SSH-agent style: git@github.com:user/repo.git
	if strings.HasPrefix(s, "git@") {
		return true
	}
	// Standard schemes git supports. We use prefix matching instead
	// of url.Parse because url.Parse accepts relative paths and we
	// want to reject those. The scheme check is enough to catch the
	// common typos (--template-repo not-a-url, --template-repo foo/bar).
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
	//
	// Logic: find the host (up to the first `/`, `?`, `#`, or end of
	// string), then look at the first character of the path. A path
	// starting with `-` is almost certainly a typo or an attack.
	rest := strings.TrimPrefix(s, scheme)
	// Skip leading slashes (file:///path -> path).
	rest = strings.TrimLeft(rest, "/")
	if rest == "" {
		// Scheme with no host is malformed.
		return false
	}
	// Find the path separator (end of host).
	pathStart := strings.IndexAny(rest, "/?#")
	if pathStart < 0 {
		// No path — just a host. Nothing to check on the path side.
		return true
	}
	// First char of the path must not be `-`. (Empty path is fine.)
	if pathStart+1 < len(rest) && rest[pathStart+1] == '-' {
		return false
	}
	return true
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
