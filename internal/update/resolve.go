package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/example/spin/internal/version"
)

// ModuleProxy is the seam between Resolver and the network. The
// default implementation is HTTPMirror, which talks to the public
// Go module proxy; tests inject a fake to avoid hitting the network.
type ModuleProxy interface {
	// ListVersions returns every known version of module, in
	// whatever order the proxy serves them. The resolver sorts
	// and filters. Returning a 404-shaped error (errors.Is(err,
	// ErrModuleNotFound)) signals "this module isn't on the
	// proxy" — the resolver degrades to "no upgrade available"
	// for that module without failing the whole batch.
	ListVersions(ctx context.Context, module string) ([]string, error)
}

// ErrModuleNotFound is returned by ModuleProxy implementations when
// the module is local-only (replace directive, unpublished, etc).
// Resolver matches it to keep the current version in both columns.
var ErrModuleNotFound = errors.New("update: module not found on proxy")

// HTTPMirror fetches version lists from https://proxy.golang.org.
// The URL is built per module and the response body is split on
// newlines, with each line trimmed. A 10s timeout caps the worst
// case for a single module (threat T-04-20).
type HTTPMirror struct {
	// Client overrides the default http.Client. nil → use a client
	// with a 10s timeout. The override exists for tests that want
	// to point at httptest.NewServer.
	Client *http.Client
	// BaseURL overrides the proxy URL for tests. nil → use
	// "https://proxy.golang.org".
	BaseURL string
}

// ListVersions fetches the version list for module and returns the
// parsed lines. Returns ErrModuleNotFound on a 404 so Resolver can
// degrade the per-dep result without failing the whole batch.
func (h *HTTPMirror) ListVersions(ctx context.Context, module string) ([]string, error) {
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	base := h.BaseURL
	if base == "" {
		base = "https://proxy.golang.org"
	}

	endpoint := base + "/" + url.PathEscape(module) + "/@v/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("User-Agent", "spin/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch %s: %w", endpoint, err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModuleNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: %s: HTTP %d", endpoint, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("update: read %s: %w", endpoint, err)
	}

	lines := strings.Split(string(body), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		v := strings.TrimSpace(line)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// Resolver enriches parsed Dep rows with upgrade candidates. Use
// NewResolver in non-test code; the zero value is also valid (the
// zero ModuleProxy field is replaced with HTTPMirror on first use).
type Resolver struct {
	Proxy ModuleProxy
	// Logger is an optional io.Writer for per-module diagnostics.
	// nil → io.Discard. A real CLI passes os.Stderr.
	Logger io.Writer
}

// NewResolver returns a Resolver with HTTPMirror wired up. Tests
// construct Resolver{Proxy: fake} directly to inject fakes.
func NewResolver() *Resolver {
	return &Resolver{Proxy: &HTTPMirror{}}
}

// Resolve fetches the version list for each dep and fills in
// NewStable (highest semver without a pre-release suffix) and
// NewLatest (highest semver including pre-releases), per CONTEXT
// D-08. A 404 from the proxy leaves both columns equal to Old
// (the "no upgrade available" degraded state). Any other fetch
// error is surfaced as the function's return error and the
// partially-enriched slice.
func (r *Resolver) Resolve(ctx context.Context, deps []Dep) ([]Dep, error) {
	proxy := r.Proxy
	if proxy == nil {
		proxy = &HTTPMirror{}
	}
	log := r.Logger
	if log == nil {
		log = io.Discard
	}

	out := make([]Dep, len(deps))
	for i, d := range deps {
		out[i] = d
		versions, err := proxy.ListVersions(ctx, d.Module)
		if err != nil {
			if errors.Is(err, ErrModuleNotFound) {
				fmt.Fprintf(log, "update: %s: not on proxy, keeping %s\n", d.Module, d.Old)
				out[i].NewStable = d.Old
				out[i].NewLatest = d.Old
				continue
			}
			return out, fmt.Errorf("update: list %s: %w", d.Module, err)
		}

		stable, latest := pickHighest(versions)
		if stable == "" {
			stable = d.Old
		}
		if latest == "" {
			latest = d.Old
		}
		out[i].NewStable = stable
		out[i].NewLatest = latest
	}

	return out, nil
}

// pickHighest returns the highest stable (no pre-release) and the
// highest overall version from the input list, using semver.Compare.
// Pre-release versions are excluded from the "stable" set per
// CONTEXT D-08: a `v2.0.0-beta.1` never wins the stable slot, even
// if it would otherwise be the latest. Empty/invalid versions are
// skipped.
func pickHighest(versions []string) (newStable, newLatest string) {
	var bestStable, bestLatest string
	for _, raw := range versions {
		v := semver.Canonical(raw)
		if !semver.IsValid(v) {
			continue
		}
		if bestLatest == "" || semver.Compare(v, bestLatest) > 0 {
			bestLatest = v
		}
		if semver.Prerelease(v) == "" {
			if bestStable == "" || semver.Compare(v, bestStable) > 0 {
				bestStable = v
			}
		}
	}
	return bestStable, bestLatest
}
