package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Loader fetches a template from a remote source and returns it ready
// to render. v2.0 supports two sources:
//   - local path on disk
//   - git URL (shallow-cloned)
//
// Future: the registry index, downloaded as a tarball.
type Loader struct {
	CacheDir string // where to store cloned templates; defaults to ~/.config/spin/templates
}

func NewLoader(cacheDir string) *Loader {
	if cacheDir == "" {
		cacheDir = defaultCacheDir()
	}
	return &Loader{CacheDir: cacheDir}
}

// Load fetches a template by source spec. The spec can be:
//   - a local path:        "/path/to/template"
//   - a git URL:           "https://github.com/foo/bar.git"
//   - a user/repo on the registry (future): "foo/bar"
func (l *Loader) Load(spec string) (*Template, error) {
	// Local path
	if isLocalPath(spec) {
		return Detect(spec)
	}
	// Git URL
	if isGitURL(spec) {
		return l.cloneGit(spec)
	}
	// Registry: defer to internal/registry
	return nil, fmt.Errorf("template loader: registry lookups not yet wired in v2.0 (%q)", spec)
}

func (l *Loader) cloneGit(url string) (*Template, error) {
	dest := filepath.Join(l.CacheDir, sanitiseRepoName(url))
	// Shallow clone with no terminal prompts. Preserve the parent
	// environment and add GIT_TERMINAL_PROMPT=0 so a missing/expired
	// credential never blocks the scaffolder with a password prompt.
	cmd := exec.Command("git", "clone", "--depth=1", url, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone %s: %s: %w", url, string(out), err)
	}
	return Detect(dest)
}

// Lister returns the basenames of all top-level entries in the
// loader's cache directory. Used by tests to assert cache behaviour
// without exposing the cache dir directly.
func (l *Loader) Lister() ([]string, error) {
	entries, err := os.ReadDir(l.CacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out, nil
}

// Clear removes the cached clone of the given ref (the sanitised
// directory name produced by sanitiseRepoName). Used by tests to
// keep the cache clean between runs. No-op if the ref is not cached.
func (l *Loader) Clear(ref string) error {
	dest := filepath.Join(l.CacheDir, ref)
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(dest)
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

func sanitiseRepoName(url string) string {
	base := url
	if i := lastIndexAny(url, "/:"); i >= 0 {
		base = url[i+1:]
	}
	base = trimSuffix(base, ".git")
	return base
}

func lastIndexAny(s, chars string) int {
	for i := len(s) - 1; i >= 0; i-- {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}

func trimSuffix(s, suf string) string {
	if len(s) > len(suf) && s[len(s)-len(suf):] == suf {
		return s[:len(s)-len(suf)]
	}
	return s
}

func defaultCacheDir() string {
	// XDG-style: prefer os.UserConfigDir() (respects XDG_CONFIG_HOME
	// on Linux, ~/Library/Application Support on macOS, %AppData% on
	// Windows). Fall back to $HOME/.config/spin/templates on error.
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, "spin", "templates")
	}
	if h, err := homeDir(); err == nil && h != "" {
		return filepath.Join(h, ".config", "spin", "templates")
	}
	return "/tmp/spin-templates"
}

// homeDir is os.UserHomeDir, kept here to avoid an os import collision
// with the wider package (which has its own os-using files).
func homeDir() (string, error) {
	out, err := exec.Command("sh", "-c", "echo $HOME").Output()
	if err != nil {
		return "", err
	}
	s := string(out)
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s, nil
}
