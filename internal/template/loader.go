package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/N1xev/spin/internal/registry"
	"github.com/N1xev/spin/internal/version"
)

// Loader fetches a template from a remote source and returns it ready
// to render. v2.0 supports three sources:
//   - local path on disk
//   - git URL (shallow-cloned)
//   - a name from ~/.config/spin/pinned.json
type Loader struct {
	CacheDir string // where to store cloned templates; defaults to ~/.config/spin/templates
	// PromptInvalidPinned is called when a template exists on disk
	// (a pinned template OR a freshly-cloned git URL) but fails
	// validation (missing spin.toml, no _base/). The hook receives
	// the template's Name, LocalPath, and the underlying Detect
	// error. It returns (true, nil) to KEEP the clone, (false, nil)
	// to REMOVE it, or (_, err) to skip the prompt and surface the
	// validation error directly. Nil means "no prompt" (use this in
	// tests or non-interactive runs; the loader will fall back to
	// defaultInvalidPinnedPrompt which always keeps).
	PromptInvalidPinned func(name, localPath string, detectErr error) (bool, error)
	// PromptExistingDest is called when cloneGit finds that dest
	// already exists. It receives the proposed name + localPath
	// and returns one of: destReuse, destPin, destWipe, destCancel.
	// nil falls back to destWipe (the previous behaviour). Wiring
	// this from cmd/new.go lets the user choose to pin the existing
	// clone or use it as-is instead of always re-cloning.
	PromptExistingDest func(name, localPath string) (destAction, error)
}

// defaultInvalidPinnedPrompt is the no-TTY fallback for
// PromptInvalidPinned. Always returns (true, nil) -- non-interactive
// runs don't delete user data.
func defaultInvalidPinnedPrompt(_, _ string, _ error) (bool, error) {
	return true, nil
}

// defaultExistingDestPrompt is the no-TTY fallback for
// PromptExistingDest. Wipes -- same behaviour as before this hook
// existed, so scripts that piped to spin or run in CI get the
// fast path.
func defaultExistingDestPrompt(_, _ string) (destAction, error) {
	return destWipe, nil
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
//   - a `<alias>/<id>` shorthand resolved via a registered registry
//   - a pinned name:       "my-template" (resolved from ~/.config/spin/pinned.json)
func (l *Loader) Load(spec string) (*Template, error) {
	// Local path
	if isLocalPath(spec) {
		return Detect(spec)
	}
	// Git URL
	if isGitURL(spec) {
		return l.cloneGit(spec)
	}
	// Registry shorthand `<alias>/<id>`: resolve to the template's
	// upstream source, then route to the appropriate existing path
	// (cloneGit or Detect). The original spec is preserved on the
	// returned Template so `promptPinAfterSuccess` knows where the
	// user started.
	if registry.IsShorthand(spec) {
		return l.loadShorthand(spec)
	}
	// Pinned name: look it up in pinned.json. The user's
	// `spin new --template <name>` shorthand relies on this.
	if t, err := l.loadPinned(spec); err != nil {
		return nil, err
	} else if t != nil {
		return t, nil
	}
	return nil, fmt.Errorf("%q is not a local path, git URL, or pinned name (run `spin add %s` first to pin a git URL or user/repo shorthand)", spec, spec)
}

// loadShorthand resolves `<alias>/<id>` against the registry
// manager, then routes the resolved Source through the existing
// git-URL or local-path path. tpl.Repo is set to the resolved
// source so promptPinAfterSuccess fires correctly.
func (l *Loader) loadShorthand(spec string) (*Template, error) {
	mgr := registry.NewManager()
	resolved, err := mgr.ResolveShorthand(spec)
	if err != nil {
		return nil, err
	}
	var tpl *Template
	switch {
	case isLocalPath(resolved.Source):
		tpl, err = Detect(resolved.Source)
	case isGitURL(resolved.Source):
		tpl, err = l.cloneGit(resolved.Source)
	default:
		return nil, fmt.Errorf("registry: shorthand %q resolved to %q which is neither a local path nor a git URL", spec, resolved.Source)
	}
	if err != nil {
		return nil, err
	}
	// Preserve the user's original spec so the post-scaffold pin
	// prompt offers to re-pin the registry shorthand (not the
	// upstream source URL).
	if tpl != nil {
		tpl.Spec = spec
	}
	return tpl, nil
}

// loadPinned looks up spec in the registry's pinned.json. Returns
// (nil, nil) when the spec is not pinned -- that's not an error, it
// just means the loader should fall through to the next source kind.
func (l *Loader) loadPinned(spec string) (*Template, error) {
	client := registry.New()
	pinned, err := client.ListPinned()
	if err != nil {
		return nil, fmt.Errorf("read pinned: %w", err)
	}
	for _, p := range pinned {
		if p.Name == spec {
			if _, err := os.Stat(p.LocalPath); err != nil {
				return nil, fmt.Errorf("pinned %q missing on disk at %s -- re-run `spin add %s`", p.Name, p.LocalPath, p.Source)
			}
			t, err := Detect(p.LocalPath)
			if err != nil {
				// The pinned clone exists but is malformed (no
				// spin.toml, no _base/, etc). Ask the user whether
				// to keep it (in case they want to fix it manually)
				// or remove it (clean up the bad clone).
				prompt := l.PromptInvalidPinned
				if prompt == nil {
					prompt = defaultInvalidPinnedPrompt
				}
				if keep, perr := prompt(p.Name, p.LocalPath, err); perr == nil && !keep {
					if rerr := client.Unpin(p.Name); rerr == nil {
						_ = os.RemoveAll(p.LocalPath)
					}
					return nil, fmt.Errorf("pinned %q removed (was: %w)", p.Name, err)
				}
				return nil, fmt.Errorf("pinned %q: %w", p.Name, err)
			}
			l.warnMinSpinVersion(t)
			return t, nil
		}
	}
	return nil, nil
}

// warnMinSpinVersion emits a single non-fatal line on stderr when
// the template's min_spin_version is newer than the running spin.
// Not an error: many templates will leave this unset, and even when
// set the user may knowingly use a newer-template-with-older-spin.
func (l *Loader) warnMinSpinVersion(t *Template) {
	if t.SpinToml == nil || t.SpinToml.MinSpinVersion == "" {
		return
	}
	if compareSemver(t.SpinToml.MinSpinVersion, version.Version) > 0 {
		fmt.Fprintf(os.Stderr, "warning: template %q requires spin >= %s (running %s) -- some features may not work\n",
			t.Name, t.SpinToml.MinSpinVersion, version.Version)
	}
}

func (l *Loader) cloneGit(url string) (*Template, error) {
	dest := filepath.Join(l.CacheDir, registry.SanitiseRepoName(url))

	// If we already have something at dest, ask before we touch it.
	// "Something" can mean: a previous successful clone, a previous
	// broken clone, or just stale files. The user gets four choices:
	//   - Reuse   : treat dest as-is and Detect it (no network)
	//   - Pin     : same as Reuse, but also persist the source so
	//               future `spin new --template <name>` works offline
	//   - Wipe    : rm -rf dest and fall through to the fresh clone
	//   - Cancel  : bail out without changes
	// This is what the user is asking for: don't just nuking their
	// existing clone, give them a say in what happens next.
	if l.destExists(dest) {
		prompt := l.PromptExistingDest
		if prompt == nil {
			// No prompt wired: behave like the old code -- wipe and
			// re-clone. This is the right default for scripts/CI.
			prompt = defaultExistingDestPrompt
		}
		action, aerr := prompt(registry.SanitiseRepoName(url), dest)
		if aerr != nil {
			return nil, aerr
		}
		switch action {
		case destReuse, destPin:
			return l.detectOrPromptInvalid(registry.SanitiseRepoName(url), dest)
		case destWipe:
			if err := os.RemoveAll(dest); err != nil {
				return nil, fmt.Errorf("clear cache for %s: %w", dest, err)
			}
		case destCancel:
			return nil, fmt.Errorf("%q exists at %s; cancelled", registry.SanitiseRepoName(url), dest)
		}
	}

	// Shallow clone with no terminal prompts. Preserve the parent
	// environment and add GIT_TERMINAL_PROMPT=0 so a missing/expired
	// credential never blocks the scaffolder with a password prompt.
	cmd := exec.Command("git", "clone", "--depth=1", url, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone %s: %s: %w", url, string(out), err)
	}
	// The clone succeeded; now check it's actually a template.
	return l.detectOrPromptInvalid(registry.SanitiseRepoName(url), dest)
}

// destExists reports whether path exists (file or dir). Used by
// cloneGit to detect a pre-existing clone and offer the user a
// choice other than the previous "wipe and re-clone" behaviour.
func (l *Loader) destExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// destAction is the user's choice when a pre-existing clone is
// found at the destination. See cloneGit for the full list.
type destAction int

// DestAction is the exported alias so cmd/ can refer to the
// constants without exposing the unexported int type.
type DestAction = destAction

const (
	destReuse destAction = iota
	destPin
	destWipe
	destCancel
)

// Exported names so cmd/ can use them in prompts without poking
// at the unexported ints.
const (
	DestReuse  = destReuse
	DestPin    = destPin
	DestWipe   = destWipe
	DestCancel = destCancel
)

// detectOrPromptInvalid runs Detect(dest). On failure it asks the
// user via PromptInvalidPinned whether to keep the (now-malformed)
// clone or remove it. Centralises the malformed-clone handling so
// the fresh-clone path and the reuse-existing path share it.
func (l *Loader) detectOrPromptInvalid(name, dest string) (*Template, error) {
	t, err := Detect(dest)
	if err == nil {
		return t, nil
	}
	prompt := l.PromptInvalidPinned
	if prompt == nil {
		prompt = defaultInvalidPinnedPrompt
	}
	keep, perr := prompt(name, dest, err)
	if perr != nil {
		return nil, perr
	}
	if !keep {
		_ = os.RemoveAll(dest)
		return nil, fmt.Errorf("%q at %s removed (was: %w)", name, dest, err)
	}
	return nil, fmt.Errorf("%q at %s: %w", name, dest, err)
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
// directory name produced by SanitiseRepoName). Used by tests to
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

// compareSemver returns -1, 0, 1 like strings.Compare but on
// semver components. "0.2.0" > "0.1.0"; "1.0" == "1.0.0" (missing
// components are treated as 0). Non-numeric segments are treated as
// 0 so the comparison degrades gracefully on malformed input.
func compareSemver(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := max(len(as), len(bs))
	for i := range n {
		var ai, bi int
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
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
	return os.UserHomeDir()
}
