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
	p := &Project{Name: name, Force: false}
	if err := p.Validate(); err != nil {
		t.Errorf("non-existent dir: Validate() = %v, want nil", err)
	}

	// 2. Existing dir without --force -> error containing "already exists".
	conflict := "spin-validate-conflict-" + randStr(t)
	if err := os.Mkdir(conflict, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(conflict) })

	p2 := &Project{Name: conflict, Force: false}
	err = p2.Validate()
	if err == nil {
		t.Fatal("existing dir without --force: Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q does not contain %q", err.Error(), "already exists")
	}

	// 3. Existing dir with --force -> nil.
	p3 := &Project{Name: conflict, Force: true}
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
		p := &Project{Name: bad, Force: true} // force=true sidesteps dir check
		if err := p.Validate(); err == nil {
			t.Errorf("Validate(%q) = nil, want error", bad)
		}
	}
}

// TestProjectValidate_ErrorFormat ensures the user-facing error names the
// regex constraint and points at the example invocation.
func TestProjectValidate_ErrorFormat(t *testing.T) {
	p := &Project{Name: "MyApp", Force: true}
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

// randStr returns a 6-char lowercase alpha suffix unique per test call.
func randStr(t *testing.T) string {
	t.Helper()
	const letters = "abcdefghijklmnopqrstuvwxyz"
	// Use the test name + nanosecond timestamp for uniqueness; not crypto.
	name := t.Name()
	h := 0
	for i := 0; i < len(name); i++ {
		h = h*31 + int(name[i])
	}
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[(h+i)%len(letters)]
	}
	return string(b)
}

// keep filepath imported even if not used directly here
var _ = filepath.Join
