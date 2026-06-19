package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClient_New_EnvOverride verifies that New() honors the
// SPIN_REGISTRY_URL env var and falls back to SPIN_REGISTRY, then to
// DefaultIndexURL. This is REG-08.
func TestClient_New_EnvOverride(t *testing.T) {
	// Primary: SPIN_REGISTRY_URL wins.
	t.Setenv("SPIN_REGISTRY_URL", "https://example.com/v1")
	t.Setenv("SPIN_REGISTRY", "https://fallback.example/v1")
	c := New()
	if c.IndexURL != "https://example.com/v1" {
		t.Errorf("IndexURL=%q, want %q", c.IndexURL, "https://example.com/v1")
	}

	// Fallback to SPIN_REGISTRY when SPIN_REGISTRY_URL is unset.
	t.Setenv("SPIN_REGISTRY_URL", "")
	c = New()
	if c.IndexURL != "https://fallback.example/v1" {
		t.Errorf("IndexURL=%q, want SPIN_REGISTRY fallback", c.IndexURL)
	}

	// Final fallback to DefaultIndexURL.
	t.Setenv("SPIN_REGISTRY", "")
	c = New()
	if c.IndexURL != DefaultIndexURL {
		t.Errorf("IndexURL=%q, want DefaultIndexURL=%q", c.IndexURL, DefaultIndexURL)
	}
}

// TestClient_Search_FriendlyFailure verifies that Search() returns
// ErrNotDeployed (not a raw connection error) when the index URL is
// the default .invalid host. This is REG-05: never a stack trace.
func TestClient_Search_FriendlyFailure(t *testing.T) {
	// Make sure no env override leaks into this test.
	t.Setenv("SPIN_REGISTRY_URL", "")
	t.Setenv("SPIN_REGISTRY", "")
	// Build a Client manually with a 1s timeout so this test does
	// not block on the .invalid DNS resolution for the default 15s
	// production timeout. The behavior under test (error mapping to
	// ErrNotDeployed) is the same.
	c := &Client{IndexURL: DefaultIndexURL, HTTP: newShortTimeoutClient()}
	_, err := c.Search("foo")
	if !errors.Is(err, ErrNotDeployed) {
		t.Fatalf("Search() err = %v, want ErrNotDeployed", err)
	}
}

// newShortTimeoutClient returns an *http.Client with a 1-second
// timeout. Used in tests that intentionally hit unreachable hosts
// (like the .invalid default) so the suite does not pay the
// production 15s timeout on every run.
func newShortTimeoutClient() *http.Client {
	return &http.Client{Timeout: 1_000_000_000} // 1s
}

// TestClient_Search_HTTPError verifies that an HTTP 404 from the
// server collapses to ErrNotDeployed. This matches the "not deployed
// yet" semantics: 404 is the response the real registry would give
// for an unrecognised route, so we treat it the same as connection
// refused for UX purposes.
func TestClient_Search_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := &Client{IndexURL: srv.URL, HTTP: srv.Client()}
	_, err := c.Search("foo")
	if !errors.Is(err, ErrNotDeployed) {
		t.Fatalf("Search() err = %v, want ErrNotDeployed", err)
	}
}

// TestClient_Search_OK verifies that a 200 response with valid JSON
// is parsed and returned.
func TestClient_Search_OK(t *testing.T) {
	want := SearchResult{
		Query: "foo",
		Total: 2,
		Entries: []Entry{
			{Name: "foo/bar", Language: "go", Type: "tui", Description: "a test"},
			{Name: "foo/baz", Language: "go", Type: "cli", Description: "another"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q != "foo" {
			t.Errorf("query=%q, want %q", q, "foo")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	c := &Client{IndexURL: srv.URL, HTTP: srv.Client()}
	got, err := c.Search("foo")
	if err != nil {
		t.Fatalf("Search() err = %v", err)
	}
	if got.Total != want.Total {
		t.Errorf("Total=%d, want %d", got.Total, want.Total)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("len(Entries)=%d, want 2", len(got.Entries))
	}
	if got.Entries[0].Name != "foo/bar" {
		t.Errorf("Entries[0].Name=%q, want %q", got.Entries[0].Name, "foo/bar")
	}
}

// TestClient_Search_Limit verifies that SearchWithLimit caps the
// number of returned entries but does not modify the underlying
// server response.
func TestClient_Search_Limit(t *testing.T) {
	want := SearchResult{
		Query: "foo",
		Total: 5,
		Entries: []Entry{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}, {Name: "e"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	c := &Client{IndexURL: srv.URL, HTTP: srv.Client()}
	got, err := c.SearchWithLimit("foo", 2)
	if err != nil {
		t.Fatalf("SearchWithLimit() err = %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("len(Entries)=%d, want 2", len(got.Entries))
	}
	if got.Total != 5 {
		t.Errorf("Total=%d, want 5 (uncapped)", got.Total)
	}
}

// TestClient_Pin_And_List_RoundTrip writes a pin to a temp dir and
// reads it back, verifying JSON marshalling, the LocalPath default,
// and de-duplication.
func TestClient_Pin_And_List_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := &Client{
		IndexURL: "https://example.invalid/v1",
		HTTP:     http.DefaultClient,
		CacheDir: dir,
	}

	p := Pinned{
		Name:      "foo/bar",
		Source:    "https://github.com/foo/bar",
		PinnedAt:  "2026-06-08T00:00:00Z",
		Version:   "v1.0.0",
		LocalPath: filepath.Join(dir, "templates", "bar"),
	}
	if err := c.Pin(p); err != nil {
		t.Fatalf("Pin() err = %v", err)
	}

	// File should exist and be valid JSON.
	b, err := os.ReadFile(c.PinnedPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(b), "foo/bar") {
		t.Errorf("pinned.json missing name; got: %s", string(b))
	}
	if !strings.Contains(string(b), "local_path") {
		t.Errorf("pinned.json missing local_path; got: %s", string(b))
	}

	// ListPinned should round-trip.
	got, err := c.ListPinned()
	if err != nil {
		t.Fatalf("ListPinned() err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(pinned)=%d, want 1", len(got))
	}
	if got[0].Name != p.Name {
		t.Errorf("Name=%q, want %q", got[0].Name, p.Name)
	}
	if got[0].LocalPath != p.LocalPath {
		t.Errorf("LocalPath=%q, want %q", got[0].LocalPath, p.LocalPath)
	}

	// Unpin then re-list: should be empty.
	if err := c.Unpin(p.Name); err != nil {
		t.Fatalf("Unpin() err = %v", err)
	}
	got, err = c.ListPinned()
	if err != nil {
		t.Fatalf("ListPinned() err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(pinned)=%d, want 0 after Unpin", len(got))
	}
}

// TestClient_Pin_DeDupeByName verifies that pinning twice with the
// same Name REPLACES the existing record (no duplicate entries).
func TestClient_Pin_DeDupeByName(t *testing.T) {
	dir := t.TempDir()
	c := &Client{
		IndexURL: "https://example.invalid/v1",
		HTTP:     http.DefaultClient,
		CacheDir: dir,
	}

	first := Pinned{
		Name:      "foo/bar",
		Source:    "https://github.com/foo/bar",
		Version:   "v1.0.0",
		LocalPath: filepath.Join(dir, "templates", "bar-v1"),
	}
	second := Pinned{
		Name:      "foo/bar",
		Source:    "https://github.com/foo/bar",
		Version:   "v2.0.0",
		LocalPath: filepath.Join(dir, "templates", "bar-v2"),
	}

	if err := c.Pin(first); err != nil {
		t.Fatalf("Pin(first) err = %v", err)
	}
	if err := c.Pin(second); err != nil {
		t.Fatalf("Pin(second) err = %v", err)
	}

	got, err := c.ListPinned()
	if err != nil {
		t.Fatalf("ListPinned() err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(pinned)=%d, want 1 (de-duped)", len(got))
	}
	if got[0].Version != "v2.0.0" {
		t.Errorf("Version=%q, want v2.0.0 (replaced)", got[0].Version)
	}
	if got[0].LocalPath != second.LocalPath {
		t.Errorf("LocalPath=%q, want %q (replaced)", got[0].LocalPath, second.LocalPath)
	}
}

// TestClient_Add_LocalPath verifies that Add() handles a local
// directory: it symlinks (or copies) the source into
// CacheDir/templates/<basename> and returns a Pinned with the
// resolved LocalPath.
func TestClient_Add_LocalPath(t *testing.T) {
	dir := t.TempDir()
	cache := t.TempDir()

	src := filepath.Join(dir, "mytmpl")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "spin.toml"), []byte("name = \"mytmpl\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &Client{
		IndexURL: "https://example.invalid/v1",
		HTTP:     http.DefaultClient,
		CacheDir: cache,
	}
	p, err := c.Add(src)
	if err != nil {
		t.Fatalf("Add() err = %v", err)
	}
	if p.Name != "mytmpl" {
		t.Errorf("Name=%q, want mytmpl", p.Name)
	}
	if p.LocalPath == "" {
		t.Fatal("LocalPath is empty")
	}
	// Resolved LocalPath should be a child of the cache templates dir.
	wantPrefix := filepath.Join(cache, "templates") + string(filepath.Separator)
	if !strings.HasPrefix(p.LocalPath, wantPrefix) {
		t.Errorf("LocalPath=%q, want prefix %q", p.LocalPath, wantPrefix)
	}
	// spin.toml should be readable from the resolved LocalPath
	// (either via symlink resolution or direct copy).
	if _, err := os.Stat(filepath.Join(p.LocalPath, "spin.toml")); err != nil {
		t.Errorf("spin.toml not visible at LocalPath: %v", err)
	}
}

// TestClient_Add_ShorthandExpands verifies that a "user/repo"
// shorthand is transparently expanded to https://github.com/
// user/repo.git before being treated as a git URL. We stub
// exec.Command via PATH manipulation: by pointing PATH at an
// empty tempdir, `git clone` fails, and we assert the error
// mentions the expanded URL.
func TestClient_Add_ShorthandExpands(t *testing.T) {
	emptyBin := t.TempDir()
	t.Setenv("PATH", emptyBin)

	c := &Client{
		IndexURL: "https://example.invalid/v1",
		HTTP:     http.DefaultClient,
		CacheDir: t.TempDir(),
	}
	_, err := c.Add("vercel/nextjs-tailwind")
	if err == nil {
		t.Fatal("Add() err = nil, want git-failure error for shorthand")
	}
	want := "https://github.com/vercel/nextjs-tailwind.git"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err=%v, want it to mention expanded URL %q", err, want)
	}
}

// TestClient_Search_URLEscape verifies the query string is properly
// URL-encoded (spaces, special chars).
func TestClient_Search_URLEscape(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		_ = json.NewEncoder(w).Encode(SearchResult{})
	}))
	defer srv.Close()

	c := &Client{IndexURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Search("go api server"); err != nil {
		t.Fatalf("Search() err = %v", err)
	}
	u, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Query().Get("q") != "go api server" {
		t.Errorf("q=%q, want %q", u.Query().Get("q"), "go api server")
	}
}

// TestErrNotImplementedAlias is a guard against accidentally breaking
// the v2.0-skeleton alias. The skeleton referenced ErrNotImplemented
// in some paths; the v2.0 canonical name is ErrNotDeployed.
func TestErrNotImplementedAlias(t *testing.T) {
	if ErrNotImplemented != ErrNotDeployed {
		t.Errorf("ErrNotImplemented=%v should equal ErrNotDeployed=%v", ErrNotImplemented, ErrNotDeployed)
	}
	// Just to keep the imports tidy.
	_ = fmt.Sprint(ErrNotImplemented)
}
