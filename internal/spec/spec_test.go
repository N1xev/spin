package spec

import "testing"

func TestIsLocalPath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"/foo", true},
		{"./foo", true},
		{"~foo", true},
		{"https://github.com/foo/bar", false},
		{"git@github.com:foo/bar", false},
		{"foo/bar", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsLocalPath(tc.in); got != tc.want {
			t.Errorf("IsLocalPath(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsGitURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://github.com/foo/bar", true},
		{"http://github.com/foo/bar", true},
		{"git@github.com:foo/bar", true},
		{"git://github.com/foo/bar", true},
		{"ssh://git@github.com/foo/bar", true},
		{"/local/path", false},
		{"./local", false},
		{"foo/bar", false},
	}
	for _, tc := range cases {
		if got := IsGitURL(tc.in); got != tc.want {
			t.Errorf("IsGitURL(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsShorthand(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"user/repo", true},
		{"official/go-cli", true},
		{"foo/bar/baz", false},
		{"/foo/bar", false},
		{"https://github.com/foo/bar", false},
		{"git@github.com:foo/bar", false},
		{"noslash", false},
		{"/foo", false},
		{"foo/", false},
		{"/bar", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsShorthand(tc.in); got != tc.want {
			t.Errorf("IsShorthand(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
