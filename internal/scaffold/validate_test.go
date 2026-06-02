package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsValidGoModuleSegment covers the 14 cases from RESEARCH §6:
//
//	valid:    myapp, my-app, my_app, my.app, a1, go-app, 62-char-string
//	invalid:  MyApp, -myapp, myapp-, test, internal, .., .hidden, a, "", myapp/../etc, 64-char, _test
func TestIsValidGoModuleSegment(t *testing.T) {
	// Build a 62-char valid string (a + 60 'b's + 'a').
	chars62 := "a" + strings.Repeat("b", 60) + "a"
	if len(chars62) != 62 {
		t.Fatalf("test fixture wrong: 62-char string is %d chars", len(chars62))
	}
	// 64-char invalid string.
	chars64 := "a" + strings.Repeat("b", 62) + "a"
	if len(chars64) != 64 {
		t.Fatalf("test fixture wrong: 64-char string is %d chars", len(chars64))
	}

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		// valid
		{"simple lowercase", "myapp", true},
		{"hyphen middle", "my-app", true},
		{"underscore middle", "my_app", true},
		{"dot middle", "my.app", true},
		{"2-char minimum", "a1", true},
		{"starts with go-", "go-app", true},
		{"62-char maximum", chars62, true},

		// invalid
		{"uppercase", "MyApp", false},
		{"leading dash", "-myapp", false},
		{"trailing dash", "myapp-", false},
		{"reserved test", "test", false},
		{"reserved internal", "internal", false},
		{"path traversal ..", "..", false},
		{"leading dot", ".hidden", false},
		{"too short", "a", false},
		{"empty string", "", false},
		{"contains slash", "myapp/../etc", false},
		{"too long 64", chars64, false},
		{"reserved _test", "_test", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsValidGoModuleSegment(c.input)
			if got != c.want {
				t.Errorf("IsValidGoModuleSegment(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

// TestProjectValidate_DirConflict covers the three branches of the
// existing-directory check in Project.Validate:
//
//	non-existent dir  -> nil
//	existing dir, no  --force -> error containing "already exists"
//	existing dir, with --force -> nil
func TestProjectValidate_DirConflict(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// 1. Non-existent dir -> nil.
	name := "spin-validate-ok-" + randStr(t)
	p := &Project{Name: name, License: "mit", Force: false}
	if err := p.Validate(); err != nil {
		t.Errorf("non-existent dir: Validate() = %v, want nil", err)
	}

	// 2. Existing dir without --force -> error containing "already exists".
	conflict := "spin-validate-conflict-" + randStr(t)
	if err := os.Mkdir(conflict, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(conflict) })

	p2 := &Project{Name: conflict, License: "mit", Force: false}
	err = p2.Validate()
	if err == nil {
		t.Fatal("existing dir without --force: Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q does not contain %q", err.Error(), "already exists")
	}

	// 3. Existing dir with --force -> nil.
	p3 := &Project{Name: conflict, License: "mit", Force: true}
	if err := p3.Validate(); err != nil {
		t.Errorf("existing dir with --force: Validate() = %v, want nil", err)
	}
}

// TestProjectValidate_NameRegex confirms that Validate rejects names
// matching the same cases as IsValidGoModuleSegment (covering the union
// of the two checks; the directory check is satisfied by the
// non-existent-name path).
func TestProjectValidate_NameRegex(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	for _, bad := range []string{"MyApp", "test", "..", "a", "internal"} {
		p := &Project{Name: bad, License: "mit", Force: true} // force=true sidesteps dir check
		if err := p.Validate(); err == nil {
			t.Errorf("Validate(%q) = nil, want error", bad)
		}
	}
}

// TestProjectValidate_ErrorFormat ensures the user-facing error names the
// regex constraint and points at the example invocation.
func TestProjectValidate_ErrorFormat(t *testing.T) {
	p := &Project{Name: "MyApp", License: "mit", Force: true}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for uppercase name")
	}
	msg := err.Error()
	for _, want := range []string{"MyApp", "lowercase", "spin new myapp --tui --bubbletea"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

// TestIsValidLicense covers the CR-002 whitelist: mit / apache-2.0 / none
// are accepted, everything else is rejected.
func TestIsValidLicense(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		// valid (already normalized)
		{"mit", true},
		{"apache-2.0", true},
		{"none", true},

		// valid after case normalization
		{"MIT", true},
		{"Apache-2.0", true},
		{"NONE", true},

		// invalid (typos, unsupported values)
		{"gpl", false},
		{"mt", false},     // typo for mit
		{"bsd", false},
		{"unlicense", false},
		{"", false},
		{" mit", false},   // whitespace — must not silently match
		{"mit ", false},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			if got := IsValidLicense(c.input); got != c.want {
				t.Errorf("IsValidLicense(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

// TestProjectValidate_LicenseField covers the CR-002 license validation
// branch in Project.Validate:
//   - empty / default (mit) succeeds
//   - case-insensitive normalization in resolve.go produces "mit"
//   - explicitly invalid values produce a descriptive error
func TestProjectValidate_LicenseField(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	valid := []string{"mit", "apache-2.0", "none"}
	for _, lic := range valid {
		t.Run("valid/"+lic, func(t *testing.T) {
			name := "spin-license-ok-" + randStr(t)
			p := &Project{Name: name, License: lic}
			if err := p.Validate(); err != nil {
				t.Errorf("Validate(license=%q) = %v, want nil", lic, err)
			}
		})
	}

	invalid := []string{"gpl", "mt", "bsd", "unlicense"}
	for _, lic := range invalid {
		t.Run("invalid/"+lic, func(t *testing.T) {
			name := "spin-license-bad-" + randStr(t)
			p := &Project{Name: name, License: lic}
			err := p.Validate()
			if err == nil {
				t.Fatalf("Validate(license=%q) = nil, want error", lic)
			}
			for _, want := range []string{lic, "mit", "apache-2.0", "none"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q missing %q", err.Error(), want)
				}
			}
		})
	}
}

// TestResolveFlags_LicenseNormalization covers the resolve.go side of
// CR-002: --license MIT (uppercase) must be normalized to "mit" before
// being stored on the Project.
func TestResolveFlags_LicenseNormalization(t *testing.T) {
	cases := []struct {
		flag string
		want string
	}{
		{"MIT", "mit"},
		{"Apache-2.0", "apache-2.0"},
		{"NONE", "none"},
		{"mit", "mit"},
	}
	for _, c := range cases {
		t.Run(c.flag, func(t *testing.T) {
			p := runResolveCmd(t, "myapp", "--tui", "--bubbletea", "--license", c.flag)
			if p.License != c.want {
				t.Errorf("p.License = %q, want %q (after --license %s)", p.License, c.want, c.flag)
			}
		})
	}
}

// TestIsValidTemplateRepo covers TMPL-03's URL format gate. The
// validation is permissive (any git-supported protocol accepted) but
// rejects obviously-invalid input (empty, no scheme, ftp://). The git
// binary is the real choke point for invalid URLs that pass this
// check.
func TestIsValidTemplateRepo(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		// valid — supported schemes
		{"https", "https://github.com/me/spin-templates", true},
		{"http", "http://example.com/repo.git", true},
		{"git scheme", "git://github.com/me/repo.git", true},
		{"ssh-agent", "git@github.com:me/spin-templates.git", true},
		{"file scheme (local dev + smoke tests)", "file:///tmp/repo", true},

		// invalid
		{"empty", "", false},
		{"no scheme", "not-a-url", false},
		{"ftp rejected", "ftp://example.com/repo.git", false},
		{"relative path rejected", "github.com/me/repo", false},
		{"whitespace rejected", " https://example.com/repo.git", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsValidTemplateRepo(c.url); got != c.want {
				t.Errorf("IsValidTemplateRepo(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

// randStr returns a 6-char lowercase alpha suffix unique per test call.
// Uses uint32 to avoid negative int overflow from non-ASCII test names
// (e.g. "TestX/case_with_unicode").
func randStr(t *testing.T) string {
	t.Helper()
	const letters = "abcdefghijklmnopqrstuvwxyz"
	name := t.Name()
	var h uint32
	for i := 0; i < len(name); i++ {
		h = h*31 + uint32(name[i])
	}
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[h%uint32(len(letters))]
		h = h*7 + 1
	}
	return string(b)
}

// keep filepath imported even if not used directly here
var _ = filepath.Join
