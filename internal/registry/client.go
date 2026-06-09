package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Client is the registry HTTP client.
type Client struct {
	IndexURL string
	HTTP     *http.Client
	CacheDir string // where Pinned entries are stored; defaults to ~/.config/spin/pinned.json
}

// New builds a Client. The index URL is read from SPIN_REGISTRY_URL
// (preferred) or SPIN_REGISTRY (fallback for v2.0-skeleton callers),
// and defaults to DefaultIndexURL — a known-unreachable host so the
// friendly "not yet deployed" message is shown until the real
// registry server is deployed.
func New() *Client {
	u := os.Getenv("SPIN_REGISTRY_URL")
	if u == "" {
		u = os.Getenv("SPIN_REGISTRY")
	}
	if u == "" {
		u = DefaultIndexURL
	}
	cache, _ := os.UserConfigDir()
	if cache == "" {
		cache = "."
	}
	return &Client{
		IndexURL: u,
		HTTP:     &http.Client{Timeout: 15 * time.Second},
		CacheDir: filepath.Join(cache, "spin"),
	}
}

// Search queries the public index. Returns ErrNotDeployed (never a
// raw connection error) when the server is unreachable (DNS failure,
// connection refused, timeout) or returns HTTP 404. Other HTTP
// failures are returned as wrapped errors that include the status.
func (c *Client) Search(query string) (*SearchResult, error) {
	return c.SearchWithLimit(query, 0)
}

// SearchWithLimit is Search with an optional cap on the returned
// entries. limit <= 0 means "no cap".
func (c *Client) SearchWithLimit(query string, limit int) (*SearchResult, error) {
	u, err := url.Parse(c.IndexURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", c.IndexURL, err)
	}
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	resp, err := c.HTTP.Get(u.String())
	if err != nil {
		if isNetworkError(err) {
			return nil, ErrNotDeployed
		}
		return nil, fmt.Errorf("registry: GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotDeployed
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry: %s: %s", resp.Status, string(body))
	}
	var out SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("registry: decode: %w", err)
	}
	if limit > 0 && len(out.Entries) > limit {
		out.Entries = out.Entries[:limit]
	}
	return &out, nil
}

// isNetworkError reports whether err is a DNS failure, connection
// refused, timeout, or other "could not reach the server" condition
// — as opposed to a protocol-level error after the connection was
// established. Such errors all map to ErrNotDeployed so the CLI
// shows a friendly message instead of a stack trace.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// DNS resolution failure
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// Connection refused / timeout / unreachable host
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	// Fall back to string inspection: the stdlib wraps *net.OpError
	// in *url.Error in many paths, but some low-level errors (e.g.
	// "context deadline exceeded") don't satisfy errors.As above.
	msg := err.Error()
	for _, needle := range []string{
		"connection refused",
		"no such host",
		"i/o timeout",
		"context deadline exceeded",
		"network is unreachable",
		"no route to host",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// ─── Add / Pin / Unpin ────────────────────────────────────────────

// Add resolves a spec (local path, git URL, or — in the future — a
// registry shorthand) into a Pinned template. The clone/copy is
// performed before the Pinned record is returned, so the caller can
// write it to pinned.json only on success.
//
// In v2.0 the registry shorthand ("user/repo") is NOT supported
// because the server isn't deployed. Add returns a clear error
// directing the user to a full git URL or a local path.
func (c *Client) Add(spec string) (*Pinned, error) {
	if spec == "" {
		return nil, fmt.Errorf("registry: add: empty spec")
	}
	switch {
	case isLocalPath(spec):
		return c.addLocal(spec)
	case isGitURL(spec):
		return c.addGit(spec)
	default:
		return nil, fmt.Errorf("registry: shorthand %q is not yet supported; use a full git URL (https://...) or a local path (/path/to/template)", spec)
	}
}

func (c *Client) addLocal(spec string) (*Pinned, error) {
	src, err := expandHome(spec)
	if err != nil {
		return nil, fmt.Errorf("registry: add local: %w", err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("registry: add local: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("registry: add local: %s is not a directory", src)
	}
	templatesDir := filepath.Join(c.CacheDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return nil, fmt.Errorf("registry: add local: mkdir templates: %w", err)
	}
	dest := filepath.Join(templatesDir, filepath.Base(src))

	// Remove any previous pin of this name so the symlink/copy is
	// fresh. (Pin-de-dupe is a separate concern, handled in Pin().)
	if err := os.RemoveAll(dest); err != nil {
		return nil, fmt.Errorf("registry: add local: clear dest: %w", err)
	}

	// Try symlink first (cheap, no copy). On Windows without
	// SeCreateSymbolicLinkPrivilege, or on filesystems that don't
	// support symlinks, fall back to a recursive copy.
	if err := os.Symlink(src, dest); err != nil {
		if copyErr := copyDir(src, dest); copyErr != nil {
			return nil, fmt.Errorf("registry: add local: symlink (%v) and copy (%w) both failed", err, copyErr)
		}
	}

	return &Pinned{
		Name:      filepath.Base(src),
		Source:    src,
		Version:   "local",
		LocalPath: dest,
	}, nil
}

func (c *Client) addGit(spec string) (*Pinned, error) {
	templatesDir := filepath.Join(c.CacheDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return nil, fmt.Errorf("registry: add git: mkdir templates: %w", err)
	}
	dest := filepath.Join(templatesDir, sanitiseRepoName(spec))

	// Remove any previous clone.
	if err := os.RemoveAll(dest); err != nil {
		return nil, fmt.Errorf("registry: add git: clear dest: %w", err)
	}

	// Shallow clone with no terminal prompts. Preserve the parent
	// environment and add GIT_TERMINAL_PROMPT=0 so a missing/expired
	// credential never blocks the scaffolder with a password prompt.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", spec, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone %s: %s: %w", spec, strings.TrimSpace(string(out)), err)
	}

	// Best-effort: capture the resolved HEAD sha so a refresh can
	// see if upstream has moved. Not fatal if git is missing or
	// the clone has no commits.
	version := "git"
	if sha, _ := gitHeadSHA(dest); sha != "" {
		version = sha
	}

	return &Pinned{
		Name:      filepath.Base(dest),
		Source:    spec,
		Version:   version,
		LocalPath: dest,
	}, nil
}

// expandHome returns path with a leading "~" or "~/" expanded to
// the user's home directory. Pure-Go; no shell.
func expandHome(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(h, path[2:]), nil
	}
	return path, nil
}

// copyDir recursively copies src to dst. Both must be directories.
// Used as a fallback when os.Symlink fails.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		// Regular file: copy bytes + mode.
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Re-create symlinks as symlinks.
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// gitHeadSHA returns the resolved HEAD sha for the repo at dir, or
// "" if the lookup fails (no git on PATH, empty repo, etc).
func gitHeadSHA(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	return s, nil
}

func isLocalPath(s string) bool {
	return len(s) > 0 && (s[0] == '/' || s[0] == '.' || s[0] == '~')
}

func isGitURL(s string) bool {
	for _, prefix := range []string{"http://", "https://", "git@", "git://", "ssh://"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// sanitiseRepoName extracts the repo basename from a git URL. E.g.
//   "https://github.com/foo/bar.git"  -> "bar"
//   "git@github.com:foo/bar.git"      -> "bar"
func sanitiseRepoName(rawURL string) string {
	base := rawURL
	// Drop the scheme / protocol prefix so we can find the last "/"
	// or ":" separator.
	for _, prefix := range []string{"https://", "http://", "git://", "ssh://"} {
		if strings.HasPrefix(base, prefix) {
			base = strings.TrimPrefix(base, prefix)
			break
		}
	}
	base = strings.TrimPrefix(base, "git@")
	// For scp-style URLs ("git@host:owner/repo.git") the colon
	// separates host from path.
	if i := strings.LastIndexAny(base, "/:"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".git")
	return base
}

// ─── pinned templates (local state) ───────────────────────────────

// PinnedPath returns the absolute path to the pinned.json file.
// Atomic writes (writePinned) protect against partial updates.
func (c *Client) PinnedPath() string {
	return filepath.Join(c.CacheDir, "pinned.json")
}

// ListPinned returns the persisted Pinned entries. A missing file
// is not an error — it returns (nil, nil), which the CLI formats as
// "No pinned templates".
func (c *Client) ListPinned() ([]Pinned, error) {
	b, err := os.ReadFile(c.PinnedPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var out []Pinned
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("registry: pinned.json: %w", err)
	}
	return out, nil
}

// Pin persists a Pinned record. If a record with the same Name
// already exists it is replaced; otherwise the new record is
// appended. Writes are atomic (write to temp file, then rename).
func (c *Client) Pin(p Pinned) error {
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return err
	}
	// Default LocalPath for older callers that pre-date the field.
	if p.LocalPath == "" {
		p.LocalPath = filepath.Join(c.CacheDir, "templates", p.Name)
	}
	all, _ := c.ListPinned()
	for i, x := range all {
		if x.Name == p.Name {
			all[i] = p
			return c.writePinned(all)
		}
	}
	all = append(all, p)
	return c.writePinned(all)
}

// Unpin removes the Pinned record with the given name (if any). It
// does NOT remove the on-disk clone/copy — the user can do that
// manually if they want.
func (c *Client) Unpin(name string) error {
	all, _ := c.ListPinned()
	out := all[:0]
	for _, x := range all {
		if x.Name != name {
			out = append(out, x)
		}
	}
	return c.writePinned(out)
}

// writePinned writes the pinned list atomically: marshal to JSON,
// write to a sibling temp file, fsync, then rename over the real
// file. This prevents a partial write (e.g. process killed) from
// leaving pinned.json in a corrupt state.
func (c *Client) writePinned(all []Pinned) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	finalPath := c.PinnedPath()
	tmp, err := os.CreateTemp(filepath.Dir(finalPath), ".pinned-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything below fails.
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}
