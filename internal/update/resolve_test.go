package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

// fakeProxy is the test ModuleProxy. requested records the order
// the resolver hits modules in; body is the canned version list
// returned for every module; err forces a synthetic 404 or other
// error response.
type fakeProxy struct {
	body      []string
	err       error
	requested []string
}

func (f *fakeProxy) ListVersions(_ context.Context, module string) ([]string, error) {
	f.requested = append(f.requested, module)
	if f.err != nil {
		return nil, f.err
	}
	return append([]string(nil), f.body...), nil
}

func newDeps(modules ...string) []Dep {
	out := make([]Dep, len(modules))
	for i, m := range modules {
		out[i] = Dep{Module: m, Old: "v1.0.0"}
	}
	return out
}

func TestResolver_FakeProxy_Stable(t *testing.T) {
	proxy := &fakeProxy{body: []string{
		"v1.0.0", "v1.1.0", "v2.0.0-beta.1", "v1.2.0",
	}}
	r := &Resolver{Proxy: proxy}

	got, err := r.Resolve(context.Background(), newDeps("example.com/foo"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got[0].NewStable != "v1.2.0" {
		t.Errorf("NewStable = %q, want v1.2.0", got[0].NewStable)
	}
	if got[0].NewLatest != "v2.0.0-beta.1" {
		t.Errorf("NewLatest = %q, want v2.0.0-beta.1 (highest overall)", got[0].NewLatest)
	}
	// Sanity: stable must NOT be the pre-release.
	if got[0].NewStable == got[0].NewLatest {
		t.Errorf("NewStable should differ from NewLatest; both = %q", got[0].NewStable)
	}
}

func TestResolver_FakeProxy_OnlyPreReleases(t *testing.T) {
	proxy := &fakeProxy{body: []string{
		"v1.0.0-alpha.1", "v1.0.0-beta.2",
	}}
	r := &Resolver{Proxy: proxy}

	got, err := r.Resolve(context.Background(), newDeps("example.com/foo"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got[0].NewStable != "v1.0.0" {
		t.Errorf("NewStable = %q, want v1.0.0 (no stable available, keep Old)", got[0].NewStable)
	}
	if got[0].NewLatest != "v1.0.0-beta.2" {
		t.Errorf("NewLatest = %q, want v1.0.0-beta.2 (highest overall)", got[0].NewLatest)
	}
}

func TestResolver_FakeProxy_404_KeepsOld(t *testing.T) {
	proxy := &fakeProxy{err: ErrModuleNotFound}
	r := &Resolver{Proxy: proxy}

	got, err := r.Resolve(context.Background(), newDeps("example.com/local"))
	if err != nil {
		t.Fatalf("Resolve should not error on ErrModuleNotFound; got %v", err)
	}
	if got[0].NewStable != "v1.0.0" {
		t.Errorf("NewStable = %q, want v1.0.0", got[0].NewStable)
	}
	if got[0].NewLatest != "v1.0.0" {
		t.Errorf("NewLatest = %q, want v1.0.0", got[0].NewLatest)
	}
}

func TestResolver_FakeProxy_Empty(t *testing.T) {
	proxy := &fakeProxy{body: nil}
	r := &Resolver{Proxy: proxy}

	got, err := r.Resolve(context.Background(), newDeps("example.com/foo"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got[0].NewStable != "v1.0.0" {
		t.Errorf("NewStable = %q, want v1.0.0", got[0].NewStable)
	}
	if got[0].NewLatest != "v1.0.0" {
		t.Errorf("NewLatest = %q, want v1.0.0", got[0].NewLatest)
	}
}

func TestResolver_PicksHigherThanOld(t *testing.T) {
	proxy := &fakeProxy{body: []string{"v0.9.0", "v1.0.0", "v1.0.1"}}
	r := &Resolver{Proxy: proxy}

	deps := []Dep{{Module: "example.com/foo", Old: "v0.9.0"}}
	got, err := r.Resolve(context.Background(), deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got[0].NewStable != "v1.0.1" {
		t.Errorf("NewStable = %q, want v1.0.1 (strictly higher than Old)", got[0].NewStable)
	}
	if semverCmp(got[0].NewStable, got[0].Old) <= 0 {
		t.Errorf("NewStable %q should be higher than Old %q", got[0].NewStable, got[0].Old)
	}
}

func TestResolver_MultipleDeps_PerModuleFetch(t *testing.T) {
	proxy := &fakeProxy{body: []string{"v1.0.0", "v1.1.0"}}
	r := &Resolver{Proxy: proxy}

	_, err := r.Resolve(context.Background(), newDeps("a.example/x", "b.example/y", "c.example/z"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(proxy.requested) != 3 {
		t.Fatalf("fake saw %d requests, want 3; got %v", len(proxy.requested), proxy.requested)
	}
	got := append([]string(nil), proxy.requested...)
	sort.Strings(got)
	want := []string{"a.example/x", "b.example/y", "c.example/z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requested = %v, want %v", got, want)
	}
}

func TestHTTPMirror_BuildsCorrectURL(t *testing.T) {
	var observedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath = r.URL.Path
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header missing on proxy request")
		}
		_, _ = fmt.Fprintln(w, "v1.0.0")
		_, _ = fmt.Fprintln(w, "v1.1.0")
	}))
	defer srv.Close()

	mirror := &HTTPMirror{BaseURL: srv.URL}
	versions, err := mirror.ListVersions(context.Background(), "example.com/foo/bar")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if observedPath != "/example.com/foo/bar/@v/list" {
		t.Errorf("URL path = %q, want /example.com/foo/bar/@v/list", observedPath)
	}
	if len(versions) != 2 {
		t.Errorf("versions = %v, want 2 entries", versions)
	}
}

// TestHTTPMirror_NotFound_ReturnsSentinel proves the 404 path
// returns ErrModuleNotFound so the resolver can degrade per
// module without failing the batch.
func TestHTTPMirror_NotFound_ReturnsSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	mirror := &HTTPMirror{BaseURL: srv.URL}
	_, err := mirror.ListVersions(context.Background(), "example.com/local")
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}
}

// semverCmp wraps semver.Compare so this file does not import
// golang.org/x/mod/semver directly (the resolver's contract is
// the only place that should care about the package).
func semverCmp(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
