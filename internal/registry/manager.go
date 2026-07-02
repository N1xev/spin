package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ErrAliasInvalid is returned when an alias fails ValidateAlias. The
// reason is included in the wrapped error so the CLI can surface it
// verbatim.
var ErrAliasInvalid = errors.New("registry: invalid alias")

// ErrAliasExists is returned by Manager.Add when the alias is already
// registered and the caller did not pass force.
var ErrAliasExists = errors.New("registry: alias already registered")

// ErrRegistryMissing is returned by Manager.Refresh/Remove/Get when
// the named alias is not in registries.json.
var ErrRegistryMissing = errors.New("registry: alias not registered")

// Manager owns the local registry catalogue (registries.json) and
// the on-disk caches under CacheDir/registries/<alias>/. It does not
// parse template metadata; that lives in index.go.
type Manager struct {
	CacheDir string // ~/.config/spin by default
}

// NewManager returns a Manager rooted at the default config dir. Use
// SetCacheDir for tests to redirect to a temp dir.
func NewManager() *Manager {
	cache, _ := os.UserConfigDir()
	if cache == "" {
		cache = "."
	}
	return &Manager{CacheDir: filepath.Join(cache, "spin")}
}

// SetCacheDir returns a copy of m with a different CacheDir. Keeps
// the Manager value cheap to copy.
func (m Manager) SetCacheDir(dir string) Manager {
	m.CacheDir = dir
	return m
}

// RegistriesPath returns the absolute path to registries.json.
func (m Manager) RegistriesPath() string {
	return filepath.Join(m.CacheDir, "registries.json")
}

// RegistriesDir returns the absolute path to the per-registry cache
// root (the parent of <alias>/).
func (m Manager) RegistriesDir() string {
	return filepath.Join(m.CacheDir, "registries")
}

// ValidateAlias checks that alias is safe to use as both a directory
// name under the cache root and a CLI argument. Reject path
// traversal, control chars, and shell-hostile characters before any
// filesystem write. Empty alias is rejected.
func ValidateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("%w: empty", ErrAliasInvalid)
	}
	if len(alias) > 128 {
		return fmt.Errorf("%w: longer than 128 chars", ErrAliasInvalid)
	}
	if alias[0] == '-' {
		return fmt.Errorf("%w: cannot start with '-'", ErrAliasInvalid)
	}
	if strings.Contains(alias, "/") || strings.Contains(alias, "\\") {
		return fmt.Errorf("%w: contains '/ or '\\'", ErrAliasInvalid)
	}
	if strings.Contains(alias, "..") {
		return fmt.Errorf("%w: contains '..'", ErrAliasInvalid)
	}
	if strings.ContainsAny(alias, ":\x00 \t\n\r") {
		return fmt.Errorf("%w: contains whitespace, ':', or NUL", ErrAliasInvalid)
	}
	return nil
}

// Load reads registries.json. A missing file is not an error: returns
// (empty, nil). A corrupt file surfaces a wrapped error so the CLI
// can suggest removing it.
func (m Manager) Load() (RegistriesConfig, error) {
	var out RegistriesConfig
	b, err := os.ReadFile(m.RegistriesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if len(b) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("registry: registries.json: %w", err)
	}
	return out, nil
}

// Get returns the registry registered as alias, or (zero, false) if
// not present.
func (m Manager) Get(alias string) (Registry, bool) {
	cfg, err := m.Load()
	if err != nil {
		return Registry{}, false
	}
	for _, r := range cfg.Registries {
		if r.Alias == alias {
			return r, true
		}
	}
	return Registry{}, false
}

// List returns every registered registry in declaration order (the
// order they were `add`ed).
func (m Manager) List() ([]Registry, error) {
	cfg, err := m.Load()
	if err != nil {
		return nil, err
	}
	return cfg.Registries, nil
}

// Add registers alias pointing at source. Source may be a local path
// or a git URL. force=false refuses to overwrite an existing alias;
// force=true wipes the existing cache before re-cloning/re-linking.
//
// Returns ErrAliasInvalid / ErrAliasExists on validation failure. On
// success returns the freshly-added Registry record (with AddedAt and
// Path set).
func (m Manager) Add(alias, source string, force bool) (Registry, error) {
	if err := ValidateAlias(alias); err != nil {
		return Registry{}, err
	}
	if source == "" {
		return Registry{}, fmt.Errorf("registry: add: empty source")
	}

	cfg, err := m.Load()
	if err != nil {
		return Registry{}, err
	}
	for _, r := range cfg.Registries {
		if r.Alias == alias && !force {
			return Registry{}, fmt.Errorf("%w: %q (use --force to replace)", ErrAliasExists, alias)
		}
	}

	// Resolve the source kind. Symlink for local paths, shallow
	// clone for git URLs. The clone goes to a sibling temp dir first
	// so a failed clone (no registry.toml, network error) leaves no
	// half-state under registries/<alias>/.
	dest := filepath.Join(m.RegistriesDir(), alias)
	if err := os.MkdirAll(m.RegistriesDir(), 0o755); err != nil {
		return Registry{}, fmt.Errorf("registry: add: mkdir cache: %w", err)
	}

	// Drop any existing entry first (--force). We do this AFTER
	// validation passes but BEFORE the new clone so the cache slot
	// is clean.
	if force {
		_ = os.RemoveAll(dest)
	}

	kind, err := m.cloneOrLink(alias, source, dest)
	if err != nil {
		return Registry{}, err
	}

	// Final sanity check: the destination must contain registry.toml.
	// If not, the source wasn't a registry -- roll back and error.
	if _, err := os.Stat(filepath.Join(dest, "registry.toml")); err != nil {
		_ = os.RemoveAll(dest)
		return Registry{}, fmt.Errorf("registry: add: %s is not a registry (missing registry.toml): %w", source, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "templates")); err != nil {
		_ = os.RemoveAll(dest)
		return Registry{}, fmt.Errorf("registry: add: %s is not a registry (missing templates/ directory): %w", source, err)
	}

	reg := Registry{
		Alias:   alias,
		Source:  source,
		Kind:    kind,
		Path:    dest,
		AddedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.upsert(reg); err != nil {
		_ = os.RemoveAll(dest)
		return Registry{}, err
	}
	return reg, nil
}

// cloneOrLink branches on whether source is local or git. Local
// sources symlink (copy-fallback on Windows); git sources shallow-
// clone to a sibling temp dir then rename into dest so a failed
// clone leaves no garbage under registries/<alias>/.
func (m Manager) cloneOrLink(alias, source, dest string) (RegistryKind, error) {
	if isLocalPath(source) {
		src, err := expandHome(source)
		if err != nil {
			return "", fmt.Errorf("registry: add: resolve source: %w", err)
		}
		info, err := os.Stat(src)
		if err != nil {
			return "", fmt.Errorf("registry: add: source: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("registry: add: source %s is not a directory", src)
		}
		if err := os.Symlink(src, dest); err != nil {
			if copyErr := copyDir(src, dest); copyErr != nil {
				return "", fmt.Errorf("registry: add: symlink (%v) and copy (%w) both failed", err, copyErr)
			}
		}
		return KindLocal, nil
	}

	// Git source: clone to a sibling temp dir, validate registry.toml
	// is present, then atomic-rename into place.
	parent := filepath.Dir(dest)
	tmp, err := os.MkdirTemp(parent, alias+".clone-")
	if err != nil {
		return "", fmt.Errorf("registry: add: tmpdir: %w", err)
	}
	defer func() {
		// If we never renamed tmp into dest, clean it up. The
		// rename below clears the tmp path so this is a no-op.
		if _, err := os.Stat(tmp); err == nil {
			_ = os.RemoveAll(tmp)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", source, tmp)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone %s: %s: %w", source, strings.TrimSpace(string(out)), err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("registry: add: rename %s -> %s: %w", tmp, dest, err)
	}
	return KindGit, nil
}

// upsert inserts reg into registries.json, replacing any existing
// record with the same Alias.
func (m Manager) upsert(reg Registry) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	found := false
	for i, r := range cfg.Registries {
		if r.Alias == reg.Alias {
			cfg.Registries[i] = reg
			found = true
			break
		}
	}
	if !found {
		cfg.Registries = append(cfg.Registries, reg)
	}
	return m.writeRegistries(cfg)
}

// Refresh pulls the latest commits for a git registry (no-op for
// local) and stamps LastUpdated. Returns ErrRegistryMissing when
// alias is not registered.
func (m Manager) Refresh(alias string) (Registry, error) {
	cfg, err := m.Load()
	if err != nil {
		return Registry{}, err
	}
	for i, r := range cfg.Registries {
		if r.Alias != alias {
			continue
		}
		if r.Kind == KindLocal {
			// No-op; don't stamp LastUpdated because nothing changed.
			return r, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "fetch", "--depth=1", "origin")
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			return r, fmt.Errorf("git fetch %s: %s: %w", r.Path, strings.TrimSpace(string(out)), err)
		}
		reset := exec.CommandContext(ctx, "git", "-C", r.Path, "reset", "--hard", "origin/HEAD")
		reset.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := reset.CombinedOutput(); err != nil {
			// Fall back to FETCH_HEAD if origin/HEAD didn't resolve.
			reset2 := exec.CommandContext(ctx, "git", "-C", r.Path, "reset", "--hard", "FETCH_HEAD")
			reset2.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
			if out2, err2 := reset2.CombinedOutput(); err2 != nil {
				return r, fmt.Errorf("git reset %s: %s: %w (also tried FETCH_HEAD: %s)", r.Path, strings.TrimSpace(string(out)), err, strings.TrimSpace(string(out2)))
			}
		}
		r.LastUpdated = time.Now().UTC().Format(time.RFC3339)
		cfg.Registries[i] = r
		if err := m.writeRegistries(cfg); err != nil {
			return r, err
		}
		return r, nil
	}
	return Registry{}, fmt.Errorf("%w: %q", ErrRegistryMissing, alias)
}

// RefreshAll refreshes every git registry in declaration order.
// Returns one error per failure so the CLI can print a per-registry
// summary; the loop never aborts on the first failure.
func (m Manager) RefreshAll() ([]Registry, []error) {
	cfg, err := m.Load()
	if err != nil {
		return nil, []error{err}
	}
	var updated []Registry
	var errs []error
	for _, r := range cfg.Registries {
		if r.Kind == KindLocal {
			continue
		}
		if _, err := m.Refresh(r.Alias); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Alias, err))
			continue
		}
		fresh, _ := m.Get(r.Alias)
		updated = append(updated, fresh)
	}
	return updated, errs
}

// Remove drops alias from registries.json and deletes the cache
// directory under registries/<alias>/. pinnedTemplates is the
// current Pinned list; if any pin's Source points inside the
// registry's path or matches its source URL, Remove refuses unless
// purgePinned is also true (in which case the offending pins are
// soft-deleted via Unpin before the cache is removed).
func (m Manager) Remove(alias string, pinnedTemplates []Pinned, purgePinned bool) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	var match *Registry
	out := cfg.Registries[:0]
	for i, r := range cfg.Registries {
		if r.Alias == alias {
			match = &cfg.Registries[i]
			continue
		}
		out = append(out, r)
	}
	if match == nil {
		return fmt.Errorf("%w: %q", ErrRegistryMissing, alias)
	}

	dependents := m.findDependentPins(*match, pinnedTemplates)
	if len(dependents) > 0 && !purgePinned {
		names := make([]string, len(dependents))
		for i, p := range dependents {
			names[i] = p.Name
		}
		return fmt.Errorf("registry: remove: %q is used by pinned templates: %s (run with --purge-pinned to drop them first)",
			alias, strings.Join(names, ", "))
	}

	if err := m.writeRegistries(RegistriesConfig{Registries: out}); err != nil {
		return err
	}

	if match.Path != "" {
		// Use os.RemoveAll on the actual filesystem path. For local
		// registries the path is a symlink -- RemoveAll removes the
		// link itself, not the target.
		if err := os.RemoveAll(match.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("registry: remove: delete cache %s: %w", match.Path, err)
		}
	}

	// After the registry is gone, mark dependent pins removed (the
	// caller's Purge is responsible for actually deleting their
	// caches). Done outside the unlink so a failure here doesn't
	// leave a registry entry pointing at a deleted cache.
	for _, p := range dependents {
		_ = (&Client{CacheDir: m.CacheDir}).Unpin(p.Name)
	}
	return nil
}

// findDependentPins returns the subset of pinned whose Source
// matches a template's source URL inside the registry (the pin was
// cloned FROM the registry) or whose LocalPath lives under the
// registry's directory (local registry case).
func (m Manager) findDependentPins(reg Registry, pinned []Pinned) []Pinned {
	if reg.Path == "" {
		return nil
	}
	// Collect the set of template `source` fields under this registry.
	templateSources := make(map[string]bool)
	tplDir := filepath.Join(reg.Path, "templates")
	if entries, err := os.ReadDir(tplDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
				continue
			}
			var tpl TemplateMetadata
			if _, err := toml.DecodeFile(filepath.Join(tplDir, e.Name()), &tpl); err != nil {
				continue
			}
			if tpl.Source != "" {
				templateSources[tpl.Source] = true
			}
		}
	}

	var out []Pinned
	for _, p := range pinned {
		if p.Source != "" && templateSources[p.Source] {
			out = append(out, p)
			continue
		}
		if reg.Kind == KindLocal && p.LocalPath != "" {
			if rel, err := filepath.Rel(reg.Path, p.LocalPath); err == nil && !strings.HasPrefix(rel, "..") {
				out = append(out, p)
			}
		}
	}
	return out
}

// writeRegistries writes the registries config atomically: marshal
// to JSON, write to a sibling temp file, fsync, then rename. Mirrors
// writePinned in client.go.
func (m Manager) writeRegistries(cfg RegistriesConfig) error {
	if err := os.MkdirAll(m.CacheDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	finalPath := m.RegistriesPath()
	tmp, err := os.CreateTemp(m.CacheDir, ".registries-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
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