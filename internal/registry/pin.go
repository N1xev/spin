package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Client is the local pin store. It owns ~/.config/spin/pinned.json
// and the per-template caches under ~/.config/spin/templates/. The
// v2.x registry layer (manager.go) owns a separate registries store
// at ~/.config/spin/registries.json plus per-registry clones under
// ~/.config/spin/registries/<alias>/.
type Client struct {
	CacheDir string // where Pinned entries are stored; defaults to ~/.config/spin/pinned.json
}

// New returns a Client rooted at the default config dir.
func New() *Client {
	cache, _ := os.UserConfigDir()
	if cache == "" {
		cache = "."
	}
	return &Client{CacheDir: filepath.Join(cache, "spin")}
}

// Add resolves a spec (local path, git URL, or GitHub user/repo
// shorthand) into a Pinned template. The clone/copy is performed
// before the Pinned record is returned, so the caller can write it
// to pinned.json only on success.
//
// "user/repo" is transparently expanded to https://github.com/
// user/repo.git -- that's the obvious thing a user will try, and
// there is no useful reason to force them to type the full URL. The
// shorthand is a thin convenience on top of addGit; it does NOT
// require the registry server to be deployed.
func (c *Client) Add(spec string) (*Pinned, error) {
	if spec == "" {
		return nil, fmt.Errorf("registry: add: empty spec")
	}
	if isShorthand(spec) {
		spec = expandShorthand(spec)
	}
	switch {
	case isLocalPath(spec):
		return c.addLocal(spec)
	case isGitURL(spec):
		return c.addGit(spec)
	default:
		return nil, fmt.Errorf("registry: cannot resolve spec %q; expected a local path, a git URL, or a user/repo shorthand", spec)
	}
}

// isShorthand reports whether spec looks like "user/repo" -- exactly
// one slash, no scheme, no leading dot/tilde, no second slash (so
// paths like "foo/bar/baz" or URLs like "git@host:foo/bar" are not
// mistaken for it). Both sides must be non-empty.
func isShorthand(s string) bool {
	if s == "" || isLocalPath(s) || isGitURL(s) {
		return false
	}
	first := strings.IndexByte(s, '/')
	if first <= 0 || first == len(s)-1 {
		return false
	}
	return strings.IndexByte(s[first+1:], '/') < 0
}

// expandShorthand turns "user/repo" into the canonical GitHub URL.
func expandShorthand(s string) string {
	return "https://github.com/" + s + ".git"
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
	dest := filepath.Join(templatesDir, SanitiseRepoName(spec))

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

// CopyTreeForTest is a test-only helper that exposes copyDir
// outside the package. The leading lowercase `c` would normally
// stay unexported; cmd/update_test.go uses this to seed the
// pinned LocalPath with a known-good initial copy. Production
// code should call (c *Client).Refresh or (c *Client).Add.
func CopyTreeForTest(src, dst string) error { return copyDir(src, dst) }

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

// SanitiseRepoName extracts the repo basename from a git URL. E.g.
//
//	"https://github.com/foo/bar.git"  -> "bar"
//	"git@github.com:foo/bar.git"      -> "bar"
//	"github.com/foo/bar"              -> "bar"
//
// The result is used as a directory name under the cache dir, so it
// must be safe across filesystems: lowercase, no scheme, no .git
// suffix. The function is the single source of truth for this
// transformation; both this package and internal/template call it
// to keep cache layout consistent.
func SanitiseRepoName(rawURL string) string {
	base := rawURL
	// Drop the scheme / protocol prefix so we can find the last "/"
	// or ":" separator.
	for _, prefix := range []string{"https://", "http://", "git://", "ssh://"} {
		if after, ok := strings.CutPrefix(base, prefix); ok {
			base = after
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

// ListAllPinned returns every persisted Pinned entry, including the
// soft-deleted ones (Removed=true). A missing file is not an error:
// it returns (nil, nil). Use this when you need to see removed pins
// (e.g. `spin list --all`) or look up a pin that may already have
// been un-pinned.
func (c *Client) ListAllPinned() ([]Pinned, error) {
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

// ListPinned returns the active Pinned entries (Removed=false).
// Soft-deleted pins are filtered out so the default views of
// `spin list`, `spin new`, and `spin update` don't surface them.
func (c *Client) ListPinned() ([]Pinned, error) {
	all, err := c.ListAllPinned()
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, x := range all {
		if !x.Removed {
			out = append(out, x)
		}
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

// Unpin soft-deletes the Pinned record with the given name: it
// marks Removed=true so the entry is hidden from ListPinned but
// still in pinned.json. The on-disk cache is left alone -- call
// Purge(name) to drop the entry AND delete its cache.
func (c *Client) Unpin(name string) error {
	all, _ := c.ListAllPinned()
	found := false
	for i, x := range all {
		if x.Name == name {
			all[i].Removed = true
			found = true
		}
	}
	if !found {
		return nil
	}
	return c.writePinned(all)
}

// Purge removes the named Pinned record entirely and deletes its
// on-disk cache (if any). Use this after Unpin to fully reclaim
// disk space; calling it on an already-removed pin is fine. Returns
// an error if no pin (active or removed) is named that.
func (c *Client) Purge(name string) error {
	all, _ := c.ListAllPinned()
	var match *Pinned
	out := all[:0]
	for i, x := range all {
		if x.Name == name {
			match = &all[i]
			continue
		}
		out = append(out, x)
	}
	if match == nil {
		return fmt.Errorf("registry: purge: no pinned template named %q", name)
	}
	if match.LocalPath != "" {
		if err := os.RemoveAll(match.LocalPath); err != nil {
			return fmt.Errorf("registry: purge: delete cache %s: %w", match.LocalPath, err)
		}
	}
	return c.writePinned(out)
}

// Refresh re-clones (or re-copies) the on-disk cache for a pinned
// template in place, then updates the pin record's Version with the
// newly resolved HEAD SHA (or "local" for local-path sources). The
// LocalPath is preserved so any code that referenced it by path
// still works. If the pin has gone missing on disk, the user is
// told to run `spin add` again rather than getting a half-built
// clone back.
//
// `pin` is passed by value so callers can decide whether to keep
// the returned record (call Pin with it) or just inspect it.
func (c *Client) Refresh(pin Pinned) (Pinned, error) {
	if pin.Name == "" {
		return Pinned{}, fmt.Errorf("registry: refresh: empty pin name")
	}
	if pin.LocalPath == "" {
		return Pinned{}, fmt.Errorf("registry: refresh: pin %q has no LocalPath; re-run `spin add`", pin.Name)
	}

	// Branch on source kind. Local paths are re-copied in place
	// (cheap, no network). Git URLs re-clone on top of the existing
	// dir -- `git fetch` would also work, but a full re-clone is
	// simpler and matches the freshness the user expects.
	//
	// Note: we do NOT require pin.LocalPath to exist. `cmd.update`
	// moves it aside to a .bak snapshot for rollback; from Refresh's
	// point of view a missing LocalPath is just a fresh clone.
	switch {
	case isLocalPath(pin.Source):
		// Best-effort: blow away the cached copy and recopy from src.
		// If the source is also missing, the user can re-pin; we
		// don't want to silently keep a stale copy.
		if _, err := os.Stat(pin.Source); err != nil {
			return Pinned{}, fmt.Errorf("registry: refresh: source %s is gone: %w", pin.Source, err)
		}
		if err := os.RemoveAll(pin.LocalPath); err != nil {
			return Pinned{}, fmt.Errorf("registry: refresh: clear %s: %w", pin.LocalPath, err)
		}
		if err := copyDir(pin.Source, pin.LocalPath); err != nil {
			return Pinned{}, fmt.Errorf("registry: refresh: copy %s: %w", pin.Source, err)
		}
		pin.Version = "local"
	case isGitURL(pin.Source):
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", pin.Source, pin.LocalPath)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			return Pinned{}, fmt.Errorf("git clone %s: %s: %w", pin.Source, strings.TrimSpace(string(out)), err)
		}
		pin.Version = "git"
		if sha, _ := gitHeadSHA(pin.LocalPath); sha != "" {
			pin.Version = sha
		}
	default:
		return Pinned{}, fmt.Errorf("registry: refresh: %q has unknown source %q", pin.Name, pin.Source)
	}
	return pin, nil
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