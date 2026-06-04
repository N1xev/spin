// Package scaffold: input validation.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModuleSegmentRegex matches an acceptable Go module path segment:
// lowercase letters, digits, hyphens, underscores, dots; must start
// and end with a letter or digit; 2-62 chars total.
var ModuleSegmentRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,61}[a-z0-9]$`)

// reservedGoWords collides with stdlib packages or common tooling dirs.
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

// validLicenses is the supported --license whitelist.
var validLicenses = map[string]bool{
	"mit":        true,
	"apache-2.0": true,
	"none":       true,
}

// IsValidLicense reports whether s is a supported --license value.
// Comparison is case-insensitive.
func IsValidLicense(s string) bool {
	return validLicenses[strings.ToLower(s)]
}

// IsValidTemplateRepo reports whether s is an acceptable
// --template-repo URL. The check is permissive; git itself is the
// real choke point and surfaces meaningful errors for bad URLs.
func IsValidTemplateRepo(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "git@") {
		return true
	}
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
	// Reject URLs whose first path segment starts with `-`; e.g.
	// "https://x.com/-evil". Git's `--` separator in CloneTemplateRepo
	// is the primary mitigation.
	rest := strings.TrimPrefix(s, scheme)
	rest = strings.TrimLeft(rest, "/")
	if rest == "" {
		return false
	}
	pathStart := strings.IndexAny(rest, "/?#")
	if pathStart < 0 {
		return true
	}
	if pathStart+1 < len(rest) && rest[pathStart+1] == '-' {
		return false
	}
	return true
}

// IsValidGoModuleSegment reports whether s is acceptable as a project
// name: 2-62 chars, matches ModuleSegmentRegex, no `..`, not a
// reserved word.
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

// Validate checks the project name, license, and target directory.
// Returns a descriptive error suitable for surfacing to the user.
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
	// Normalize case so --license MIT or --license Apache-2.0 succeed.
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
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check directory %q: %w", target, err)
	}

	return nil
}
