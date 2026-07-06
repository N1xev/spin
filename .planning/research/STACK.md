# Stack Additions: v2.x Local-Registry Milestone

**Project:** spin
**Milestone:** v2.x local-registry (Phases 6-8)
**Researched:** 2026-07-03
**Confidence:** HIGH (verified against Context7 + existing internal/registry patterns)

## TL;DR

**No new dependencies needed.** The local-registry milestone can ship entirely with
the existing dependency footprint. The new `manager` + `index` + `resolver` layers
all reuse what `spin` already pulls in:

| Concern | Reuse from | Add anything? |
|---------|-----------|---------------|
| TOML parsing (registry.toml + templates/*.toml) | `github.com/BurntSushi/toml` v1.6.0 (already a direct dep) | NO |
| Atomic write of `registries.json` | Mirror the existing `writePinned` pattern in `internal/registry/client.go` (stdlib only) | NO |
| Git clone / fetch for registry refresh | `os/exec` + `git clone --depth=1` + `GIT_TERMINAL_PROMPT=0` (existing pattern) | NO |
| Local-path registry linking | `os.Symlink` with `copyDir` fallback (existing pattern in `addLocal`) | NO |
| File walking + listing | `os.ReadDir` + `filepath.WalkDir` (stdlib) | NO |
| Path traversal safety | `filepath.Rel` + `strings.HasPrefix` (stdlib) | NO |
| Search scoring / filtering | In-process `strings.Contains` (stdlib) | NO |
| XDG config dir | `os.UserConfigDir` (stdlib) | NO |

This is the right answer because:

1. The spec (`spin-registry.md`) explicitly says "No new deps needed; reuse
   `github.com/BurntSushi/toml` for registry metadata".
2. The capability profile is dominated by **filesystem and git plumbing** --
   all covered by the Go standard library and the existing patterns.
3. Every existing `spin` test pattern (atomic write, git clone, symlink/copy
   fallback, BurntSushi/toml decode) is exactly what the new layers need.

## Existing Stack (unchanged)

`go.mod` already carries everything we need. No version bumps, no additions.

| Library | Already in go.mod | Used by registry milestone |
|---------|-------------------|----------------------------|
| `github.com/BurntSushi/toml` v1.6.0 | direct dep (since v2.0) | parse `registry.toml` + each `templates/<id>.toml` |
| `github.com/spf13/cobra` v1.10.2 | direct dep | `spin registry {add,list,update,remove}` subcommands |
| `charm.land/lipgloss/v2` v2.0.3 | direct dep | styled registry list table + success messages |
| `charm.land/huh/v2` v2.0.3 | direct dep | interactive confirm on `registry remove` if TTY |
| `charm.land/log/v2` (transitive via fang) | direct dep | scaffolder logging |
| `golang.org/x/text` v0.24.0 | direct dep | (no use in registry layer; pre-existing) |

## Recommended Patterns (mirror existing code, don't introduce new abstractions)

### 1. TOML parsing for `registry.toml` and `templates/*.toml`

**Decision:** keep `github.com/BurntSushi/toml` v1.6.0. No swap.

**Rationale:**

- BurntSushi/toml is the de-facto Go TOML library and was originally
  proposed for stdlib inclusion. It is already a direct dep of `spin`
  and used in `internal/template/parse.go` for `spin.toml`.
- The registry spec uses trivial TOML: scalar fields, `tags = [...]` array,
  no datetime, no inline tables beyond `id = "..."` strings. BurntSushi
  handles this with one `toml.Unmarshal` call against a typed struct.
- Encoding is only required for `registries.json`, which is JSON (matches
  `pinned.json`). No TOML encoding needed at runtime.

**Pattern (mirror `parse.go`):**

```go
// internal/registry/registry_meta.go (new file in Phase 6A)

type rawRegistryMeta struct {
    ID          string   `toml:"id"`
    Name        string   `toml:"name"`
    Description string   `toml:"description"`
    Homepage    string   `toml:"homepage"`
    Maintainer  string   `toml:"maintainer"`
    License     string   `toml:"license"`
}

type rawTemplateMeta struct {
    ID          string   `toml:"id"`
    Name        string   `toml:"name"`
    Description string   `toml:"description"`
    Source      string   `toml:"source"`
    Tags        []string `toml:"tags"`
    Authors     []string `toml:"authors"`
    License     string   `toml:"license"`
    Homepage    string   `toml:"homepage"`
}
```

**Why not swap to `encoding/toml/v2` (stdlib) or `pelletier/go-toml`?**

- `internal/template/spin_toml.go` line 85-90 has a stale comment saying
  "encoding/toml/v2 was promoted to stdlib; using it would require an
  import". This is wrong: `encoding/toml/v2` was proposed but not landed
  in Go 1.23/1.24/1.25. **Do not pursue this swap.** It is a phantom
  refactor that would break the v2.0 validation status.
- A swap to `pelletier/go-toml` would introduce a second TOML library
  in the module graph. The v2.x spec says no new deps; we already have
  BurntSushi.

### 2. Atomic write of `registries.json`

**Decision:** mirror `writePinned` in `internal/registry/client.go` lines
557-595. Stdlib only.

**Pattern:**

```go
// internal/registry/manager.go (Phase 6A)

func (m *Manager) writeRegistries(all []RegistryEntry) error {
    b, err := json.MarshalIndent(all, "", "  ")
    if err != nil {
        return err
    }
    finalPath := filepath.Join(m.ConfigDir, "registries.json")
    if err := os.MkdirAll(m.ConfigDir, 0o755); err != nil {
        return err
    }
    tmp, err := os.CreateTemp(m.ConfigDir, ".registries-*.json.tmp")
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
```

**Why this pattern:** `pinned.json` uses the same flow and the v2.0 spec
already validates it. Reuse the exact dance: temp in same dir (so the
rename is atomic on POSIX), `Sync` before `Close`, defer cleanup on failure.
Two files using identical write logic is fine; abstract only when a third
file needs it.

**Storage path:** `~/.config/spin/registries.json` -- same XDG resolution
that `pinned.json` uses (`os.UserConfigDir()`).

### 3. Git clone for registry refresh

**Decision:** reuse the `git clone --depth=1` + `GIT_TERMINAL_PROMPT=0`
pattern from `internal/registry/client.go` lines 222-258 (`addGit`).

**Pattern:**

```go
// internal/registry/manager.go (Phase 6A)
func (m *Manager) cloneRegistry(alias, sourceURL, dest string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", sourceURL, dest)
    cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git clone %s: %s: %w", sourceURL, strings.TrimSpace(string(out)), err)
    }
    return nil
}

func (m *Manager) fetchRegistry(alias, dest string) error {
    // For `spin registry update <alias>` on an existing clone, prefer
    // `git fetch --depth=1 --prune` + `git reset --hard origin/HEAD`
    // over a full re-clone. Less bandwidth, preserves any local
    // metadata the user might have added (e.g. ignored test files).
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    if out, err := exec.CommandContext(ctx, "git", "-C", dest,
        "fetch", "--depth=1", "--prune", "--tags").CombinedOutput(); err != nil {
        return fmt.Errorf("git fetch %s: %s: %w", dest, strings.TrimSpace(string(out)), err)
    }
    if out, err := exec.CommandContext(ctx, "git", "-C", dest,
        "reset", "--hard", "origin/HEAD").CombinedOutput(); err != nil {
        // Some registries don't have an origin/HEAD ref. Fall back to
        // the default branch the working tree is tracking.
        if out2, err2 := exec.CommandContext(ctx, "git", "-C", dest,
            "reset", "--hard", "@{u}").CombinedOutput(); err2 != nil {
            return fmt.Errorf("git reset %s: %s / %s: %w", dest,
                strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)), err)
        }
    }
    return nil
}
```

**Notes:**

- **Why `--depth=1`:** matches the existing pattern (`addGit`, `Refresh`,
  `cloneGit` in `loader.go`). Public registry repos are large; shallow
  keeps the refresh fast and the disk footprint small.
- **Why `GIT_TERMINAL_PROMPT=0`:** also matches existing pattern. A
  misconfigured credential never blocks the scaffolder.
- **Why `5*time.Minute` (vs `2*time.Minute` for templates):** registries
  can be much larger than individual templates (many `templates/*.toml`
  entries). Same shape, longer ceiling.
- **Why `git fetch` for `update`, not `git clone`:** `update` is run on a
  clone that already exists. Re-cloning is wasteful and overwrites local
  state. The fetch+reset dance is the canonical "update a shallow clone"
  recipe. Fall back to re-clone only if `fetch` errors out.

### 4. Local-path registries: symlink vs copy

**Decision:** mirror the existing `addLocal` logic at
`internal/registry/client.go` lines 181-220.

**Pattern:**

```go
// internal/registry/manager.go (Phase 6A)
func (m *Manager) linkLocalRegistry(alias, src, dest string) error {
    // Remove any previous symlink/copy so the link is fresh.
    if err := os.RemoveAll(dest); err != nil {
        return fmt.Errorf("registry: clear %s: %w", dest, err)
    }
    // Try symlink first (cheap, no copy). Fall back to recursive
    // copy if the FS doesn't support symlinks (e.g. Windows without
    // SeCreateSymbolicLinkPrivilege, some FAT/exFAT mounts).
    if err := os.Symlink(src, dest); err != nil {
        if copyErr := copyDir(src, dest); copyErr != nil {
            return fmt.Errorf("registry: symlink (%v) and copy (%w) both failed", err, copyErr)
        }
    }
    return nil
}
```

**Why symlink-first, copy-fallback:** identical to the v2.0 pattern for
`addLocal` in `client.go`. Edits to the source registry are seen
immediately (good for authoring). On filesystems that disallow symlinks,
the fallback keeps the feature working.

**Why not just copy:** copy doubles disk and means `update` requires a
full re-copy. Symlink is a single inode.

**Edge case to handle:** if the source path is relative, resolve against
the user's CWD at `add` time and store the absolute path in
`registries.json`. The relative-source case is uncommon but possible;
`spin-registry.md` line 271 shows `spin registry add local ../registry`.

### 5. Index reader: walking `templates/*.toml`

**Decision:** in-process walk with `filepath.WalkDir` (stdlib) + per-file
`toml.Unmarshal` (BurntSushi). No indexing library.

**Pattern:**

```go
// internal/registry/index.go (Phase 6B)
func (m *Manager) ReadIndex() ([]TemplateMeta, error) {
    var out []TemplateMeta
    for _, reg := range m.registries() {
        tplDir := filepath.Join(m.registriesDir(), reg.Alias, "templates")
        entries, err := os.ReadDir(tplDir)
        if err != nil {
            if os.IsNotExist(err) {
                continue // missing templates/ dir == empty registry
            }
            return nil, fmt.Errorf("read %s: %w", tplDir, err)
        }
        for _, e := range entries {
            if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
                continue
            }
            b, err := os.ReadFile(filepath.Join(tplDir, e.Name()))
            if err != nil {
                // skip-and-warn per spec ("Invalid template metadata
                // files are ignored and reported")
                m.reportSkipped(reg.Alias, e.Name(), err)
                continue
            }
            var meta rawTemplateMeta
            if err := toml.Unmarshal(b, &meta); err != nil {
                m.reportSkipped(reg.Alias, e.Name(), err)
                continue
            }
            if meta.ID == "" || meta.Source == "" {
                m.reportSkipped(reg.Alias, e.Name(), errors.New("missing required id or source"))
                continue
            }
            out = append(out, TemplateMeta{
                Registry: reg.Alias,
                ID:       meta.ID,
                Name:     meta.Name,
                Source:   meta.Source,
                Tags:     meta.Tags,
                ...
            })
        }
    }
    sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
    return out, nil
}
```

**Why no search index library (Bleve, etc):** the spec's search is
substring match across name + tags + description. The volume per registry
is `templates/*.toml` -- tens to low hundreds. In-memory slice scan is
microseconds. Adding Bleve would mean a heavy dep for a problem that
doesn't exist yet. Keep it stdlib.

**Search scoring (when we add it):** simple priority --
`id exact > name contains > tag contains > description contains`. No
library.

### 6. `<alias>/<id>` resolver

**Decision:** pure function in `internal/registry/resolver.go`. No deps.

**Pattern:**

```go
// internal/registry/resolver.go (Phase 6B)
func (m *Manager) ResolveShorthand(shorthand string) (TemplateMeta, error) {
    parts := strings.SplitN(shorthand, "/", 2)
    if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
        return TemplateMeta{}, fmt.Errorf("registry: expected <alias>/<id>, got %q", shorthand)
    }
    alias, id := parts[0], parts[1]
    idx, err := m.ReadIndex()
    if err != nil {
        return TemplateMeta{}, err
    }
    for _, t := range idx {
        if t.Registry == alias && t.ID == id {
            return t, nil
        }
    }
    return TemplateMeta{}, fmt.Errorf("registry: %s/%s not found in any registered registry", alias, id)
}
```

**Why this is enough:** `ReadIndex` returns all entries across all
registries. Linear scan over a few hundred entries is faster than a
single network round-trip. The spec's resolver doesn't need to be clever
until registries have thousands of entries (which doesn't exist today).

### 7. Rewiring `spin search` / `spin add` / `spin new` (Phase 7B)

**Decision:** keep `internal/registry` types (`Entry`, `SearchResult`) as
the shape `cmd/search.go` consumes. Populate from `Manager.ReadIndex()`
instead of HTTP.

**Pattern (in `cmd/search.go`):**

```go
func runSearch(cmd *cobra.Command, args []string) error {
    mgr := registry.NewManager()
    idx, err := mgr.SearchIndex(args[0], searchLimit) // substring match
    if err != nil {
        return err
    }
    res := &registry.SearchResult{
        Query:   args[0],
        Total:   len(idx),
        Entries: idx, // []TemplateMeta is structurally compatible with []Entry
    }
    if searchJSON {
        return json.NewEncoder(os.Stdout).Encode(res)
    }
    fmt.Print(registry.FormatSearch(res, false))
    return nil
}
```

**Why not delete `Entry`/`SearchResult` types:** they are the JSON shape
the public spec promises. `spin search --json` must remain
machine-readable, even though the source is now local files. The cleanest
move is: `TemplateMeta` becomes the on-disk shape, and `FormatSearch` /
`SearchResult` stay as the CLI output shape (with `TemplateMeta` carrying
the same JSON tags).

**Rewiring `spin add` and `spin new`:** accept `<alias>/<id>` in addition
to the existing local-path / git-URL / pinned-name shortcuts. The
existing `Loader.Load` falls through to `loadPinned` last; the new
shortcut needs to come BEFORE `loadPinned` (otherwise `example/go-api`
will look like an unknown name). Order in `Loader.Load` becomes:

1. local path (`/...`, `./...`, `~/...`)
2. git URL (`https://...`, `git@...`)
3. `<alias>/<id>` shorthand (new) -- one slash, no scheme
4. pinned name lookup (existing)

### 8. Path traversal safety for `templates/*.toml` reads

**Decision:** keep reads within the registered root. Stdlib only.

**Pattern:**

```go
// used everywhere a path derived from metadata could escape
cleanRel := filepath.Clean(rel)
if strings.HasPrefix(cleanRel, "..") || strings.Contains(cleanRel, "/../") {
    return fmt.Errorf("invalid path %q", rel)
}
full := filepath.Join(tplDir, cleanRel)
```

This is needed if/when templates store relative paths in their metadata
(for future "include" or "screenshots" features). Today's spec only uses
flat `templates/<id>.toml` paths, so the risk is small -- but the
guardrail should be there from day one.

## Alternatives Considered

| Concern | Recommended | Alternative | Why not |
|---------|------------|-------------|---------|
| TOML library | `github.com/BurntSushi/toml` v1.6.0 | `pelletier/go-toml` v2 | Second TOML lib in module graph; BurntSushi is already a direct dep; pelletier is heavier and used less in the charm ecosystem |
| TOML library | BurntSushi | `encoding/toml/v2` (stdlib) | Phantom: proposed but never landed in Go 1.23/1.24/1.25. `internal/template/spin_toml.go:85` has a stale comment suggesting otherwise -- do NOT act on it |
| Atomic JSON write | `writePinned` pattern (stdlib) | `github.com/google/renameio/v2` | Adds a dep to replicate a 25-line pattern; no benefit |
| Git operations | `os/exec` git | `github.com/go-git/go-git` v5 | Pure-Go but slow on large repos, large dep (~30 MB module), and `os/exec` is already the pattern across `internal/registry` and `internal/template`. **MUST NOT add go-git** -- would be a Phase 1 audit blocker |
| Index/search | stdlib `strings.Contains` + `sort.Slice` | `github.com/blevesearch/bleve/v2` | Overkill for tens-to-hundreds of templates; adds CGO + a 100+ MB dep tree |
| Registry file layout | `os.Symlink` + `copyDir` fallback (existing) | Hard-link (`os.Link`) | Hardlinks don't cross filesystems and break when the source moves. Symlinks are what v2.0 already uses |
| Fetch strategy | `git fetch --depth=1 --prune` + `git reset --hard origin/HEAD` | Full re-clone | Wasteful for `update`; full re-clone is the fallback when fetch fails |
| Prompt style for `registry remove` | `charm.land/huh/v2` (in-process) | `gum` shell-out | huh is already a direct dep; works in non-TTY by short-circuiting |
| `manager` location | `internal/registry/manager.go`, `internal/registry/index.go`, `internal/registry/resolver.go` | New `internal/registrymgr/` package | Same domain, same `Pinned`/`CacheDir` plumbing; a separate package would duplicate the XDG dir helper and the pinned-vs-registered paths |

## Why NO New Deps Is the Right Answer

Three independent reasons converge on the same answer:

1. **Spec directive.** `spin-registry.md` and `.planning/PROJECT.md` line
   34 both say "No new deps needed; reuse `github.com/BurntSushi/toml` for
   registry metadata".
2. **Capability profile.** The new layers are: parse TOML, walk a
   directory, resolve a slash-delimited shorthand, atomically write
   JSON, clone a git repo. Every one of these is a stdlib or
   already-imported library operation.
3. **Existing pattern parity.** `internal/registry/client.go` already
   handles TOML-ish data (JSON for `pinned.json`), atomic write
   (`writePinned`), git clone (`addGit`), local path linking (`addLocal`
   with `copyDir` fallback), and the XDG config dir (`os.UserConfigDir`).
   The new layers don't need anything the existing layers don't already
   exercise.

If a future milestone needs full-text indexing, pluggable auth, or
registry mirroring, that's the time to revisit deps -- not now.

## Integration Points with Existing Code

| Existing | Used by milestone | Change needed |
|----------|-------------------|---------------|
| `internal/registry/client.go::writePinned` (lines 557-595) | `Manager.writeRegistries` (new) | None -- copy the pattern verbatim |
| `internal/registry/client.go::addGit` (lines 222-258) | `Manager.cloneRegistry` (new) | None -- same flags, longer timeout |
| `internal/registry/client.go::addLocal` (lines 181-220) | `Manager.linkLocalRegistry` (new) | None -- same symlink-then-copy logic |
| `internal/registry/client.go::SanitiseRepoName` (lines 363-381) | `Manager.cloneRegistry` for dest path | None -- reuse |
| `internal/registry/client.go::PinnedPath` / `ListPinned` | unchanged | None |
| `internal/template/parse.go::parseTOML` (lines 44-71) | `Manager.ReadIndex` per-file decode | None -- same library, new struct tags |
| `internal/template/loader.go::Load` (lines 67-84) | `<alias>/<id>` path inserted before `loadPinned` | **Small change** to Load() ordering |
| `internal/template/loader.go::isGitURL` (lines 279-286) | unchanged | None |
| `cmd/search.go::runSearch` (lines 38-65) | reads `Manager.SearchIndex` instead of HTTP | **Replace HTTP call with local index call**; keep JSON shape via `SearchResult` |
| `cmd/add.go::runAdd` (lines 37-65) | accepts `<alias>/<id>` shorthand via `Manager.ResolveShorthand` | **Add shorthand branch before `client.Add(spec)`** |
| `cmd/list.go`, `cmd/remove.go`, `cmd/update.go` | unchanged for pinned-templates; new `cmd/registry.go` for registries | **Add `cmd/registry.go`** for `spin registry {add,list,update,remove}` |
| `internal/registry/types.go::Entry`, `SearchResult` | output shape for `spin search --json` | **Rename to `TemplateMeta` in storage; keep `Entry` as the JSON-tagged shape for CLI output** (or add JSON tags to `TemplateMeta`) |

## Sources

- `github.com/BurntSushi/toml` v1.6.0 -- verified latest stable via Context7
  (`/burntsushi/toml` library ID, HIGH confidence)
- `internal/registry/client.go` lines 178-330 -- existing git clone + symlink +
  copyDir patterns (HIGH, source of truth)
- `internal/registry/client.go` lines 557-595 -- atomic writePinned pattern
  (HIGH, source of truth)
- `internal/template/parse.go` lines 44-71 -- existing TOML decode pattern
  (HIGH, source of truth)
- `internal/template/loader.go` lines 67-84, 279-286 -- existing spec
  classification (local/git/pinned) and how to extend it (HIGH, source of truth)
- `spin-registry.md` (in-repo spec) -- the feature contract
- `.planning/PROJECT.md` lines 32-35 -- milestone constraints ("No new deps
  needed; reuse BurntSushi/toml")
- Context7: `/burntsushi/toml` -- latest version, Unmarshal API, Encode API
  (HIGH)

## Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| No new deps needed | HIGH | Spec is explicit; all required capabilities map to existing libs |
| BurntSushi/toml is current latest | HIGH | Verified via Context7; `go.mod` already on v1.6.0 |
| `encoding/toml/v2` is NOT in stdlib | MEDIUM | Comments in `spin_toml.go` suggest otherwise; verified not landed in Go 1.23-1.25 |
| Atomic write pattern reuses `writePinned` | HIGH | Pattern is in `client.go:557-595`; copying it is straightforward |
| Git clone pattern reuses `addGit` | HIGH | Same flags + env vars; same timeout shape |
| Symlink-then-copy reuses `addLocal` | HIGH | Identical decision tree |
| `Loader.Load` extension point | HIGH | Source code shows the if/else cascade; insertion point is clear |
| `TemplateMeta` vs `Entry` shape compatibility | MEDIUM | Both have Name, Description, Tags; structural fit, but field renames (Source vs Repository) need a tag audit |

## Open Questions

- Should `spin registry remove` also delete the on-disk `~/.config/spin/registries/<alias>/`
  clone? Default to YES (mirror `Purge` for pinned templates), but offer
  `--keep-files` for users who want to manually inspect before deleting.
- Should `TemplateMeta` and `Entry` collapse into one type, or stay as two
  (one for storage, one for CLI output)? Recommend collapse -- the JSON
  tags can do double duty and we avoid mapping code.
- Should `spin registry update <alias>` accept a new `<source>` to migrate
  a registry to a new URL, or refuse and require `remove` + `add`? Recommend
  refuse (simpler audit trail; matches `pinned.json` behaviour).
- Should `Manager.ReadIndex` cache the result in memory across one CLI
  invocation (one ReadIndex per command), or re-walk on every call? Recommend
  call once per command -- matches `ListPinned` behaviour, avoids stale
  state in long-running commands (we don't have any today).