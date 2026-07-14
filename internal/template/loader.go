package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/N1xev/spin/internal/log"
	"github.com/N1xev/spin/internal/registry"
	srcspec "github.com/N1xev/spin/internal/spec"
	"github.com/N1xev/spin/internal/version"
)

// Loader fetches a template from a local path, a git URL, or a name
// in ~/.config/spin/pinned.json, and returns it ready to render.
type Loader struct {
	CacheDir string // where to store cloned templates; defaults to ~/.config/spin/templates
	// PromptInvalidPinned is called when a template exists on disk but
	// fails validation. It returns true to keep the clone, false to
	// remove it, or a non-nil error to surface directly. A nil hook
	// keeps the clone (used by non-interactive runs and tests).
	PromptInvalidPinned func(name, localPath string, detectErr error) (bool, error)
	// PromptExistingDest is called when cloneGit finds dest already
	// exists. A nil hook wipes and re-clones, which suits scripts/CI.
	PromptExistingDest func(name, localPath string) (DestAction, error)
}

// defaultInvalidPinnedPrompt keeps the clone; non-interactive runs
// don't delete user data.
func defaultInvalidPinnedPrompt(_, _ string, _ error) (bool, error) {
	return true, nil
}

// defaultExistingDestPrompt wipes and re-clones.
func defaultExistingDestPrompt(_, _ string) (DestAction, error) {
	return DestWipe, nil
}

func NewLoader(cacheDir string) *Loader {
	if cacheDir == "" {
		cacheDir = defaultCacheDir()
	}
	return &Loader{CacheDir: cacheDir}
}

// Load fetches a template by source spec using a background context.
// Prefer LoadContext when a cancellable context is available.
func (l *Loader) Load(spec string) (*Template, error) {
	return l.LoadContext(context.Background(), spec)
}

// LoadContext fetches a template by source spec. The spec can be a
// local path, a git URL, a `<alias>/<id>` registry shorthand, or a
// pinned name from ~/.config/spin/pinned.json. ctx bounds any git
// clone the loader performs.
func (l *Loader) LoadContext(ctx context.Context, spec string) (*Template, error) {
	var t *Template
	var err error

	if srcspec.IsLocalPath(spec) {
		t, err = Detect(spec)
	} else if srcspec.IsGitURL(spec) {
		t, err = l.cloneGit(ctx, spec)
	} else if registry.IsShorthand(spec) {
		t, err = l.loadShorthand(ctx, spec)
	} else {
		t, err = l.loadPinned(ctx, spec)
		if t == nil && err == nil {
			return nil, fmt.Errorf("%q is not a local path, git URL, or pinned name (run `spin add %s` first to pin a git URL or registry shorthand)", spec, spec)
		}
	}
	if err != nil {
		return nil, err
	}
	l.warnMinSpinVersion(t)
	return t, nil
}

// loadShorthand resolves `<alias>/<id>` against the registry manager,
// then routes the resolved source through the git-URL or local-path
// path. The original spec is preserved on the returned Template so
// the post-scaffold pin prompt offers to re-pin the shorthand.
//
// If the resolved source is already pinned locally, the cached
// template is used instead of re-cloning.
func (l *Loader) loadShorthand(ctx context.Context, spec string) (*Template, error) {
	mgr := registry.NewManager()
	resolved, err := mgr.ResolveShorthand(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Check if the resolved source is already pinned.
	if srcspec.IsGitURL(resolved.Source) {
		client := registry.New()
		if pinned, err := client.ListPinned(ctx); err == nil {
			for _, p := range pinned {
				if p.Source == resolved.Source && p.LocalPath != "" {
					if t, err := Detect(p.LocalPath); err == nil {
						t.Spec = spec
						t.Repo = resolved.Source
						return t, nil
					}
				}
			}
		}
	}

	var tpl *Template
	switch {
	case srcspec.IsLocalPath(resolved.Source):
		tpl, err = Detect(resolved.Source)
	case srcspec.IsGitURL(resolved.Source):
		tpl, err = l.cloneGit(ctx, resolved.Source)
	default:
		return nil, fmt.Errorf("registry: shorthand %q resolved to %q which is neither a local path nor a git URL", spec, resolved.Source)
	}
	if err != nil {
		return nil, err
	}
	if tpl != nil {
		tpl.Spec = spec
		if srcspec.IsGitURL(resolved.Source) {
			tpl.Repo = resolved.Source
		}
	}
	return tpl, nil
}

// loadPinned looks up spec in the registry's pinned.json. Returns
// (nil, nil) when the spec is not pinned -- that's not an error, it
// just means the loader should fall through to the next source kind.
func (l *Loader) loadPinned(ctx context.Context, spec string) (*Template, error) {
	client := registry.New()
	pinned, err := client.ListPinned(ctx)
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
					if rerr := client.Unpin(ctx, p.Name); rerr == nil {
						if rmErr := os.RemoveAll(p.LocalPath); rmErr != nil {
							log.Debug("failed to remove invalid pinned template", "path", p.LocalPath, "err", rmErr)
						}
					}
					return nil, fmt.Errorf("pinned %q removed (was: %w)", p.Name, err)
				}
				return nil, fmt.Errorf("pinned %q: %w", p.Name, err)
			}
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
		log.Warn("template requires a newer spin version; some features may not work",
			"template", t.Name,
			"required", t.SpinToml.MinSpinVersion,
			"running", version.Version)
	}
}

func (l *Loader) cloneGit(ctx context.Context, url string) (*Template, error) {
	name := registry.SanitiseRepoName(url)
	dest := filepath.Join(l.CacheDir, name)

	// Something already at dest (a prior clone or stale files): ask
	// before touching it instead of always wiping.
	if l.destExists(dest) {
		prompt := l.PromptExistingDest
		if prompt == nil {
			prompt = defaultExistingDestPrompt
		}
		action, aerr := prompt(name, dest)
		if aerr != nil {
			return nil, aerr
		}
		switch action {
		case DestReuse, DestPin:
			tpl, err := l.detectOrPromptInvalid(name, dest)
			if err != nil {
				return nil, err
			}
			tpl.Repo = url
			return tpl, nil
		case DestWipe:
			if err := os.RemoveAll(dest); err != nil {
				return nil, fmt.Errorf("clear cache for %s: %w", dest, err)
			}
		case DestCancel:
			return nil, fmt.Errorf("%q exists at %s; cancelled", name, dest)
		}
	}

	// Shallow clone, no terminal prompts. GIT_TERMINAL_PROMPT=0 keeps
	// a missing credential from blocking on a password prompt; the
	// context bounds a slow or hanging remote.
	if err := registry.GitClone(ctx, url, dest); err != nil {
		return nil, err
	}
	tpl, err := l.detectOrPromptInvalid(name, dest)
	if err != nil {
		return nil, err
	}
	tpl.Repo = url
	return tpl, nil
}

func (l *Loader) destExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DestAction is the user's choice when a pre-existing clone is found
// at the destination.
type DestAction int

const (
	DestReuse  DestAction = iota // use the existing clone as-is
	DestPin                      // reuse and persist the source for offline use
	DestWipe                     // remove the clone and re-clone
	DestCancel                   // abort without changes
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
		if rmErr := os.RemoveAll(dest); rmErr != nil {
			log.Debug("failed to remove invalid template clone", "path", dest, "err", rmErr)
		}
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
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return filepath.Join(h, ".config", "spin", "templates")
	}
	return "/tmp/spin-templates"
}
