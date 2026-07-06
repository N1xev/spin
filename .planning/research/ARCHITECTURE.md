# Architecture Research: spin v2.x local-registry

**Domain:** CLI scaffolder, zero-backend template registry
**Researched:** 2026-07-03
**Confidence:** HIGH (architecture is already constrained by `spin-registry.md`; integration points are derived from existing code paths)

## Scope

This file answers how the new registry **Manager**, **Index**, and **Resolve** layers integrate with the existing `internal/template/loader.go` and `internal/registry/client.go`, per the spec at `spin-registry.md`. The HTTP-based registry stub is replaced by a local index built from cloned/symlinked registry directories.

The existing v2.0 split is:
- `internal/registry/client.go` -- **two responsibilities glued together**: HTTP client (`Search`, `IndexURL`, `HTTP`, `ErrNotDeployed`) and local pin state (`Add`/`Pin`/`Unpin`/`Purge`/`Refresh`/`ListPinned`/`PinnedPath`).
- `internal/template/loader.go` -- resolves a spec (local path / git URL / pinned name) into a `*Template`.

The v2.x split is:
- `internal/registry/manager.go` -- **new**. CRUD over `registries.json` + the on-disk registries tree (`~/.config/spin/registries/<alias>/`).
- `internal/registry/index.go` -- **new**. Reads `templates/*.toml` from each registered registry, validates, builds an in-memory `Index`.
- `internal/registry/resolve.go` -- **new**. Parses `<alias>/<id>` and produces a resolved source spec (URL or local path) ready for `template.Loader.Load`.
- `internal/registry/pin.go` -- **renamed from `client.go`** (HTTP pieces deleted). Holds `Pin`, `Unpin`, `Purge`, `Refresh`, `ListPinned`, `ListAllPinned`, `PinnedPath`, `writePinned`, `addLocal`, `addGit`, `SanitiseRepoName`, `isLocalPath`, `isGitURL`, `expandHome`, `copyDir`, `gitHeadSHA`. No HTTP, no `IndexURL`, no `Search`, no `ErrNotDeployed`.
- `internal/registry/search.go` -- **deleted** (replaced by index-driven local search).
- `internal/registry/types.go` -- **modified**: drop `Entry`, `SearchResult`, `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`. Add `Registry`, `RegistriesConfig`, `TemplateMetadata`, `Resolved`.

## Storage Layout

```text
~/.config/spin/                     (XDG_CONFIG_HOME-aware; os.UserConfigDir)
+-- pinned.json                     # unchanged: pin records consumed by Pin/Unpin/Refresh
+-- templates/                      # unchanged: pinned template clones (LocalPath root)
|   +-- go-cli/                     # pinned template (was a git URL)
|   +-- foo/                        # pinned template (was a local path symlink)
+-- registries/                     # NEW: per-registry local clones / symlinks
    +-- official/                   # git clone of the registry repo
    |   +-- registry.toml
    |   +-- templates/
    |       +-- go-api.toml         # id = "spin/go-api"
    |       +-- rust-cli.toml
    +-- company/                    # symlink to a local path the user passed to `spin registry add`
        +-- registry.toml
        +-- templates/
            +-- billing-api.toml

~/.config/spin/registries.json      # NEW: config + bookkeeping
# {
#   "registries": [
#     {"alias": "official", "source": "https://github.com/spin-org/registry", "kind": "git",  "path": "~/.config/spin/registries/official", "added_at": "..."},
#     {"alias": "company",  "source": "/opt/company-registry",            "kind": "local","path": "~/.config/spin/registries/company",  "added_at": "..."}
#   ]
# }
```

Key constraint: `pinned.json` format is unchanged so every existing pin keeps working (`PROJECT.md` validated requirement).

## Component Responsibilities

| Component | Responsibility | Communicates With |
|-----------|----------------|-------------------|
| `cmd/registry.go` (new) | Cobra subcommands: `registry add/list/update/remove` | `registry.Manager` |
| `cmd/search.go` (modified) | Iterate every registry's `templates/*.toml` via `registry.Index`, filter, format | `registry.Index`, `registry.Manager` |
| `cmd/add.go` (modified) | Accept `<alias>/<id>` shorthand, resolve via `registry.Resolve`, then delegate to existing `Client.Pin`/`Add` flow | `registry.Resolve`, `registry.Manager` (for index lookup), `pin.go` |
| `cmd/new.go` (modified) | Accept `<alias>/<id>` shorthand in `--template` / positional, resolve to a local path or git URL, hand off to `template.Loader.Load` | `registry.Resolve`, `template.Loader` |
| `cmd/list.go` (modified) | List pinned (existing) + optionally list registered registries via `registry.Manager.List` | `pin.go`, `registry.Manager` |
| `cmd/update.go` (modified) | Re-clone/symlink registries (new) in addition to refreshing pinned templates (existing) | `registry.Manager.Refresh`, `pin.go` |
| `cmd/remove.go` (unchanged) | Remove pinned templates (existing behaviour) | `pin.go` |
| `registry.Manager` (new) | CRUD over `registries.json`. Clone (git) or symlink (local) into `registries/<alias>/`. Refresh / remove a registry. Resolve an alias to its on-disk path. | filesystem |
| `registry.Index` (new) | Build an in-memory snapshot of every `templates/*.toml` under every registry dir. Validate; drop invalid. Filter by query string. | `registry.Manager` (to enumerate registries) |
| `registry.Resolve` (new) | Parse `<alias>/<id>`. Return the source spec (URL or path) + `LocalPath` (registry dir, not the template itself). | `registry.Manager`, `registry.Index` |
| `pin.go` (renamed `client.go`) | Pin state + clone/copy logic only. Exposes `Add`, `Pin`, `Unpin`, `Purge`, `Refresh`, `ListPinned`, `ListAllPinned`, `PinnedPath`. | filesystem, git |
| `template.Loader` (modified) | `Load(spec)` now accepts an additional source kind: a resolved `SourceSpec` from `registry.Resolve`. Keep `isLocalPath` / `isGitURL` / `loadPinned` / `cloneGit`. | `pin.go` (read pinned), git, filesystem |

## Integration Points (per CLI command)

### (a) `spin registry add <alias> <source>`

```
cmd/registry.go -> registry.Manager.Add(alias, source)
   +-- detect source kind (isLocalPath | isGitURL | file://)
   +-- resolve CacheDir/registries.json path
   +-- loadRegistries() -> append Registry{Alias, Source, Kind, Path, AddedAt}
   +-- writeRegistries() (atomic, like writePinned)
   +-- mkdir CacheDir/registries/<alias>/
   +-- kind=git  : git clone --depth=1 <source> <dest>
   +-- kind=local: symlink <source> <dest>  (fallback: copy)
```

- **New file:** `cmd/registry.go`.
- **New API:** `Manager.Add(alias, source string) error`.
- **No HTTP code involved.** Drop-in for what `Client.Search` would have done; this command path was never wired in v2.0.

### (b) `spin registry update [alias]`

```
cmd/registry.go -> registry.Manager.Refresh(alias? string)
   +-- loadRegistries() -> list
   +-- for each target Registry:
   |     +-- rm -rf <dest>
   |     +-- git clone --depth=1 <source> <dest>     (kind=git)
   |     +-- re-symlink <source> <dest>               (kind=local)
   +-- report per-registry outcome (ok / failed)
```

- **New API:** `Manager.Refresh(alias string) error` and `Manager.RefreshAll() error`.
- **Independent of `Client.Refresh` (pin refresh).** The existing `cmd/update.go` keeps refreshing pinned templates; `cmd/registry.go` is a sibling, not a replacement. (See "Two pipelines share one Template type and one filesystem layout" constraint.)

### (c) `spin search <query>`

```
cmd/search.go -> registry.Index.Build() -> registry.Index.Search(query, limit)
   Index.Build():
     +-- Manager.List() -> []Registry
     +-- for each registry, walk <Path>/templates/*.toml:
           +-- decode TemplateMetadata (BurntSushi/toml)
           +-- validate required fields (id, source)
           +-- drop invalid + collect warning
           +-- append to in-memory slice
   Index.Search(query, limit):
     +-- filter by case-insensitive substring on id/name/tags/description
     +-- return []Entry (local; new struct, replaces registry.Entry)
     +-- honour --limit, --json
```

- **Modified file:** `cmd/search.go`.
- **Replaces:** `Client.SearchWithLimit` (HTTP).
- **ErrNotDeployed is gone** -- the local index either has results or it doesn't. Empty result still prints "No templates matched X."

### (d) `spin add <alias>/<id>`

```
cmd/add.go -> registry.Resolve.Parse("<alias>/<id>")
   Resolve.Parse:
     +-- Manager.List() -> registries
     +-- find alias -> Registry
     +-- read <Registry.Path>/templates/<id>.toml
     +-- return Resolved{ Source, Kind, TemplateName }
     +-- (or NotFound error naming the alias)

cmd/add.go -> pin.go (renamed Client) Add(Resolved.Source)
   +-- addLocal / addGit (unchanged)
   +-- Pin(*pinned) (unchanged; pinned.json format preserved)
```

- **Modified file:** `cmd/add.go` -- first branch: if arg contains `/`, route through `Resolve`. Otherwise existing behaviour (local path / git URL / user/repo shorthand).
- **Unchanged:** `pinned.json` write path.

### (e) `spin new <alias>/<id>` (no prior pin)

```
cmd/new.go -> resolveNameAndTemplate (existing) -> tplSpec = "<alias>/<id>"

cmd/new.go -> registry.Resolve.Parse(tplSpec) -> Resolved{ Source: "https://github.com/spin-org/go-api", Kind: "git" }

cmd/new.go -> template.Loader.Load(Resolved.Source)
   +-- isGitURL("https://...") -> cloneGit
   +-- git clone --depth=1 <Source> <CacheDir/templates/<SanitiseRepoName(Source)>>
   +-- Detect(dest) -> *Template
```

- **Modified file:** `cmd/new.go` -- one new branch in `resolveNameAndTemplate` / before `loader.Load`: if spec contains a `/` AND does NOT look like a git URL or local path, treat as `<alias>/<id>`.
- **The existing `Loader.Load` is unchanged.** Resolve returns a git URL; Loader handles git URLs as it does today. **This is the key design property** that keeps the Loader's diff small.

### (f) `spin new <name> <alias>/<id>` (cached pin hit)

```
cmd/new.go -> registry.Resolve.Parse("<alias>/<id>")
   Resolve returns Resolved{ Source, Kind, TemplateName, LocalPath }

cmd/new.go -> check pin cache:
   +-- pin.go.ListPinned() -> find entry where Source == Resolved.Source
   +-- if hit: use pinned.LocalPath (skip re-clone)

cmd/new.go -> template.Loader.Load(<pinned name>)
   +-- loadPinned(name) -> Detect(pinned.LocalPath) -> *Template
   +-- (zero network if cache is warm)
```

- **Optimization layer added to `cmd/new.go`**, not the Loader. The Loader's `loadPinned` already does the right thing when handed a pinned name; the new code in `cmd/new.go` is "if we resolved `<alias>/<id>`, see if Source is already pinned and use that name."
- **Alternative design considered:** add a 4th source kind to `Loader.Load` (a `Resolved` struct). Rejected for v2.x because it changes the public `Loader.Load(spec string)` signature. The pin-hit optimisation belongs in `cmd/new.go` where the Resolved already exists.

## Data Flow Diagrams

### `spin registry add official https://github.com/spin-org/registry`

```
caller
  |  registry add <alias> <source>
  v
cmd/registry.go
  |  Manager.Add("official", "https://github.com/spin-org/registry")
  v
registry.Manager
  +-- detectKind() -> "git"
  +-- append Registry{Alias,Source,Kind,Path,AddedAt} -> registries.json
  |     +-- writePinned-style atomic write
  +-- git clone --depth=1 <source> CacheDir/registries/official/
```

### `spin registry update [alias]`

```
cmd/registry.go
  |  Manager.Refresh(alias) or Manager.RefreshAll()
  v
registry.Manager
  +-- loadRegistries() -> filter
  +-- rm -rf <dest>
  +-- re-acquire:
        git  -> git clone --depth=1
        local -> symlink (or copy on Windows)
```

### `spin search go`

```
cmd/search.go
  |  Index.Build(); Index.Search("go", limit)
  v
registry.Index
  +-- Manager.List() -> []Registry
  +-- for each r: walk r.Path/templates/*.toml
  |     +-- Parse -> TemplateMetadata
  |     +-- validate required fields
  +-- filter results where id/name/tags/description contains "go"
  |
  v
cmd/search.go  -> print table / JSON
```

### `spin add example/go-api`

```
cmd/add.go
  |  spec = "example/go-api"
  +-- detectKind(spec): not local path, not git URL, contains "/"
  +-- Resolve.Parse(spec)
  |     +-- Manager.List() -> find alias "example"
  |     +-- read Registry.Path/templates/go-api.toml
  |     +-- return Resolved{ Source = "https://...", Kind = "git" }
  |
  |  Resolved.Source -> pin.go.Add
  |     +-- addGit (clone, hash, write LocalPath)
  |     +-- Pin(*pinned) -> atomic pinned.json write
```

### `spin new example/go-api` (no prior pin)

```
cmd/new.go
  |  tplSpec = "example/go-api"
  +-- resolveNameAndTemplate (existing path)
  |
  |  spec contains "/" and isn't a path or git URL
  +-- Resolve.Parse(tplSpec)
  |     +-- returns Source = "https://github.com/example/go-api"
  |
  |  pin-hit check: ListPinned().find by Source  -> none
  |
  |  Loader.Load(Resolved.Source)
  |     +-- isGitURL -> true
  |     +-- cloneGit(Source) -> Detect(dest) -> *Template
  |
  |  Render + post-hook + delete spin.toml
  |
  |  promptPinAfterSuccess (existing) -- pin Source for offline use
```

### `spin new myapp example/go-api` (cached pin hit)

```
cmd/new.go
  |  spec = "example/go-api"
  +-- Resolve.Parse -> Resolved{ Source = "https://..." }
  |
  |  pin-hit check: ListPinned().find by Source  -> HIT
  |     +-- pinned.LocalPath is the warm clone
  |     +-- treat as if user passed pinned name
  |
  |  Loader.Load(<pinned.Name>)
  |     +-- loadPinned(name) -> Detect(LocalPath)
  |     +-- *Template (no network)
  |
  v  render + post-hook + delete spin.toml
```

## Recommended Project Structure (after v2.x)

```
internal/registry/
+-- doc.go              # package doc; updated to reflect local-only model
+-- types.go            # Registry, RegistriesConfig, TemplateMetadata, Resolved, Pinned
+-- manager.go          # NEW: CRUD over registries.json + on-disk registries tree
+-- index.go            # NEW: read templates/*.toml across registries, validate, filter
+-- resolve.go          # NEW: parse <alias>/<id>, look up metadata, return Resolved
+-- pin.go              # RENAMED from client.go (HTTP bits deleted)
+-- pin_test.go         # unchanged; tests stay valid
+-- manager_test.go     # NEW
+-- index_test.go       # NEW
+-- resolve_test.go     # NEW

cmd/
+-- registry.go         # NEW: registry add|list|update|remove
+-- registry_test.go    # NEW
+-- search.go           # MODIFIED: drop HTTP, drive registry.Index
+-- add.go              # MODIFIED: <alias>/<id> branch via registry.Resolve
+-- new.go              # MODIFIED: <alias>/<id> branch via registry.Resolve + pin-hit short-circuit
+-- update.go           # UNCHANGED in shape (still refreshes pinned templates)
+-- list.go             # UNCHANGED
+-- remove.go           # UNCHANGED
+-- help_test.go, init_test.go, etc.  # UNCHANGED
```

## Patterns

### Pattern 1: Registry as a cloned metadata source

**What:** Each registry is a directory containing `registry.toml` + `templates/*.toml`. Spin treats it as opaque metadata; the registry dir is not a template itself.

**When:** Always -- every registry follows this layout.

**Trade-offs:** Pros: zero backend, idempotent (re-clone = re-index), offline-friendly. Cons: requires the user to register a registry before searching (`spin search` over zero registries = empty result).

### Pattern 2: Two-pipeline state model (pin vs registry)

**What:** Pin state and registry state live in different files (`pinned.json` vs `registries.json`) and different directories (`templates/` vs `registries/`). The Manager and the (renamed) pin code never write to each other's files.

**When:** Always.

**Trade-offs:** Pros: clean separation, pin records survive a registry remove, backward compat preserved. Cons: two config files for the user to be aware of (rare; never an issue in practice).

### Pattern 3: Resolver returns a SourceSpec, not a Template

**What:** `registry.Resolve.Parse(<alias>/<id>)` returns `Resolved{ Source, Kind, LocalPath, ... }` -- a spec compatible with the existing `template.Loader.Load(spec)` API. The Loader never knows it came from a registry.

**When:** Every `<alias>/<id>` shorthand in `add` and `new`.

**Trade-offs:** Pros: zero changes to the Loader's signature; the resolve is a thin adapter. Cons: pin-hit optimisation must be re-implemented in `cmd/new.go` (the loader doesn't know to check the pin cache for a Source it was handed).

### Pattern 4: Atomic writes everywhere

**What:** `writePinned` already exists in `client.go`. `writeRegistries` follows the same pattern: marshal JSON, write to sibling `.tmp`, fsync, rename.

**When:** Every write to `registries.json` and `pinned.json`.

**Why:** Partial writes corrupt state. The existing `writePinned` is the template; reuse the pattern, don't invent a new one.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Adding a 4th source kind to `Loader.Load`

**What people do:** Change `Loader.Load(spec string)` to `Loader.Load(spec any)` and add a `Resolved` arm.

**Why it's wrong:** Changes the public API; every test that calls `Loader.Load(string)` needs updating; breaks the "templates are the only extension surface" invariant.

**Do this instead:** Resolve returns a `Source` string (git URL or path) that flows through the existing `isGitURL`/`isLocalPath` branches. Pin-hit optimisation is a thin pre-check in `cmd/new.go`.

### Anti-Pattern 2: Storing the registry source URL in `pinned.json`

**What people do:** When pinning from `<alias>/<id>`, add a `RegistryAlias` field to `Pinned` so the user can later `spin remove --from-registry`.

**Why it's wrong:** Pin records are independent of registry metadata. A pin survives a registry being removed; the reverse should also be true.

**Do this instead:** Pin records keep their existing schema. Registry metadata is its own concern in `registries.json`.

### Anti-Pattern 3: HTTP fallback for `spin search`

**What people do:** Keep `DefaultIndexURL` / `ErrNotDeployed` as a fallback in case the local index is empty.

**Why it's wrong:** The spec explicitly says "Drop `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` env vars; drop `ErrNotDeployed` / `DefaultIndexURL`" (PROJECT.md, v2.x Active). Also: the local index is THE registry in v2.x -- there's no separate HTTP registry.

**Do this instead:** Empty result -> "no templates matched" message. User adds a registry first if they have none.

### Anti-Pattern 4: Globbing `<alias>/<id>` from `pinned.json`

**What people do:** When the user types `<alias>/<id>`, fall back to looking up `<alias>/<id>` as a pinned name in case the registry resolution fails.

**Why it's wrong:** The two namespaces are deliberately distinct. `<alias>/<id>` is a registry coordinate; a pinned name is a bare string. Conflating them means a stale pin could mask a missing registry entry, or vice versa.

**Do this instead:** Resolve returns `ErrUnknownAlias` or `ErrUnknownID` (sentinel errors). The CLI surfaces the precise failure ("registry alias 'example' not found -- run `spin registry add`").

## New vs Modified Components

### New files

| Path | Purpose |
|------|---------|
| `internal/registry/manager.go` | CRUD over `registries.json` + clone/symlink per alias |
| `internal/registry/index.go` | Walk + parse `templates/*.toml`, validate, filter |
| `internal/registry/resolve.go` | `<alias>/<id>` -> Resolved{source, kind, ...} |
| `internal/registry/manager_test.go` | CRUD + clone/symlink tests |
| `internal/registry/index_test.go` | Walk + validate + filter tests |
| `internal/registry/resolve_test.go` | Parse + lookup tests |
| `cmd/registry.go` | Cobra: `registry add/list/update/remove` |
| `cmd/registry_test.go` | CLI-level tests for the above |

### Modified files

| Path | Change |
|------|--------|
| `internal/registry/client.go` | **Renamed to `pin.go`** + delete HTTP-only code |
| `internal/registry/types.go` | Drop `Entry`, `SearchResult`, `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`; add `Registry`, `RegistriesConfig`, `TemplateMetadata`, `Resolved` |
| `internal/registry/doc.go` | Update package doc to reflect local-only model |
| `internal/registry/search.go` | **Deleted** |
| `internal/template/loader.go` | **Likely no change** -- Resolved hands back a Source string that flows through existing `isGitURL`/`isLocalPath` branches. If pin-hit opt needs to live in the Loader, add a `Loader.LoadResolved(Resolved)` helper. Otherwise `Loader` stays as-is. |
| `cmd/search.go` | Replace `Client.SearchWithLimit` with `Index.Build()` + `Index.Search()`; drop HTTP-error handling, `ErrNotDeployed`, `--json` payload struct |
| `cmd/add.go` | One branch up front: if arg contains `/` and is not a local path / git URL, route through `registry.Resolve`. Otherwise unchanged. |
| `cmd/new.go` | Same branch in `resolveNameAndTemplate`; pin-hit short-circuit before `Loader.Load` |
| `cmd/update.go` | **Unchanged in shape.** `spin update` still refreshes pinned templates. `spin registry update` (different command) refreshes registries. They share `Refresh`-style rollback semantics but live in different files. |
| `cmd/list.go` | Unchanged for `spin list` (pinned). Add `--registries` mode? Optional; defer. |
| `cmd/remove.go` | Unchanged. |
| `cmd/init.go` | Unchanged. |

### Unchanged files

| Path | Why |
|------|-----|
| `internal/template/template.go` | `Template.Render` / `Detect` don't know about registries |
| `internal/params/`, `internal/version/` | No registry involvement |
| `cmd/root.go`, `cmd/version.go`, `cmd/doc.go`, `cmd/print.go` | No changes |
| All `*_test.go` files for unchanged commands | Tests stay valid |

### Deleted code

| Symbol | File | Why |
|--------|------|-----|
| `Client.IndexURL`, `Client.HTTP` | `client.go` | No HTTP client anymore |
| `Client.Search`, `Client.SearchWithLimit` | `client.go` | Replaced by `Index.Search` |
| `isNetworkError` | `client.go` | Only the HTTP client cared |
| `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented` | `types.go` | v2.x spec drops these |
| `Entry`, `SearchResult` | `types.go` | Replaced by `TemplateMetadata` + the table row type used by `Index.Search` |
| `internal/registry/search.go` | entire file | FormatSearch moves into `cmd/search.go`; SortByPopularity is dropped (no downloads field in local metadata) |
| `SPIN_REGISTRY_URL`, `SPIN_REGISTRY` env reads | `client.go` (`New`) | No HTTP to point at |

## Storage migration / backward compatibility

**No migration step is required for `pinned.json`.** Pin records keep the same fields (`Name`, `Source`, `PinnedAt`, `Version`, `LocalPath`, `Removed`) and the same file path. The Loader's `loadPinned` reads them unchanged.

`registries.json` is a new file; first run with no registries means `spin search` returns "no templates matched." This is the documented v2.x behaviour (PROJECT.md: "Replace the HTTP-based registry stub").

`templates/` directory is unchanged. Existing clones continue to be served by `Loader.loadPinned`.

## Suggested Build Order (3 phases)

The user pre-specified phases A/B/C in `PROJECT.md`. This file recommends the implementation order within those phases to keep the diff reviewable.

### Phase A (PROJECT.md: "manager + `spin registry` CLI + `registries.json`")

Self-contained -- touches registry internals only. No calls into Loader. No calls from cmd except the new `cmd/registry.go`.

1. Add `Registry`, `RegistriesConfig`, `TemplateMetadata` to `internal/registry/types.go`. Drop `Entry`, `SearchResult`, `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`. **Don't delete yet** -- call sites still reference them.
2. Create `internal/registry/manager.go`:
   - `Manager.Add(alias, source string) error`
   - `Manager.List() ([]Registry, error)`
   - `Manager.Refresh(alias string) error` / `Manager.RefreshAll() error`
   - `Manager.Remove(alias string) error`
   - `Manager.Get(alias string) (*Registry, error)`
   - `writeRegistries([]Registry)` -- clones `writePinned` pattern.
3. Create `internal/registry/manager_test.go` (CRUD on a `t.TempDir()` rooted manager).
4. Create `cmd/registry.go` with `registry add/list/update/remove` calling the Manager. Test with `cmd/registry_test.go`.
5. Keep `Client.Search` etc. compiling -- `cmd/search.go` still uses them; do not touch `cmd/search.go` until Phase B.

**Phase A exit criterion:** `spin registry add official <url>` creates `registries.json` + clones the dir; `spin registry list` shows it; `spin registry remove official` cleans both up. No regressions in `spin search` / `spin add` / `spin new` / `spin update` / `spin list`.

### Phase B (PROJECT.md: "index reader + `<alias>/<id>` resolver + rewire `search`/`add`/`new`/`loader`")

Depends on Phase A. This is where the resolver + index land and the CLI gets rewired.

1. Create `internal/registry/index.go`:
   - `Index.Build(manager *Manager) (*Index, error)`
   - `(*Index).Search(query string, limit int) []TemplateMetadata`
   - `(*Index).Get(alias, id string) (*TemplateMetadata, error)`
2. Create `internal/registry/resolve.go`:
   - `Resolved{ Source, Kind, LocalPath, ... }`
   - `Resolve.Parse(spec string, manager *Manager, index *Index) (Resolved, error)` -- sentinel errors: `ErrUnknownAlias`, `ErrUnknownID`.
3. Modify `cmd/search.go`:
   - Replace `Client.SearchWithLimit` with `Index.Build(manager).Search(...)`.
   - Drop `ErrNotDeployed` branch + friendly-message block.
   - `--json` payload shape changes from `SearchResult` to a local struct (`{query, total, entries: [{id, name, tags, source, ...}]}`).
   - Move `FormatSearch` body into this file (delete `internal/registry/search.go`).
4. Modify `cmd/add.go`:
   - Detect `<alias>/<id>` shape (contains `/`, not a path, not a git URL, not a `user/repo` shorthand that already worked).
   - Resolve -> `Resolved.Source` -> existing `pin.go.Add` flow. Pin record written unchanged.
5. Modify `cmd/new.go`:
   - Same `<alias>/<id>` detection in `resolveNameAndTemplate`.
   - Resolve -> `Resolved.Source` -> `Loader.Load(Source)`.
   - Pin-hit short-circuit: if `pin.go.ListPinned()` already has a row with this Source, use the pinned name instead.
6. Decide on the Loader diff:
   - **Default:** no change to `internal/template/loader.go`. Resolved hands a string; existing branches handle it.
   - **Optional:** add `Loader.LoadResolved(Resolved)` if the pin-hit short-circuit is awkward to express at the call site. Keep the original `Load(string)` for backward compat with v2.0 tests.

**Phase B exit criterion:** `spin add <alias>/<id>` and `spin new <alias>/<id>` both work end-to-end; `spin search <query>` reads from local index; nothing references `Client.Search` or `ErrNotDeployed` anymore. `pinned.json` is unchanged in shape.

### Phase C (PROJECT.md: "delete HTTP client code + docs pass")

Cleanup -- safe only after Phase B has zero remaining references.

1. Delete `internal/registry/search.go`.
2. Rename `internal/registry/client.go` -> `internal/registry/pin.go` (or `pin_state.go` to avoid the verb confusion with `Manager.Pin`-if-it-ever-existed). Update package-internal references (`registry.New()` call in `loader.go`, in `cmd/*.go`).
3. Delete from `pin.go` (the renamed file):
   - `IndexURL`, `HTTP` fields and constructor reads
   - `Search`, `SearchWithLimit`, `isNetworkError`
   - `New()` env-var logic (`SPIN_REGISTRY_URL`, `SPIN_REGISTRY`)
4. Delete from `types.go`: `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`, `Entry`, `SearchResult`.
5. Update `internal/registry/doc.go` to reflect the local-only model.
6. Update tests:
   - `client_test.go` -> split into `pin_test.go` (HTTP bits removed) and the new `manager_test.go` / `index_test.go` / `resolve_test.go`.
   - Any test that sets `SPIN_REGISTRY_URL` becomes a no-op or is deleted.
7. Docs pass: README, PROJECT.md "Validated" section, `spin-registry.md` revision note.

**Phase C exit criterion:** `go build` and `go test ./...` pass with zero references to `Client.Search`, `ErrNotDeployed`, `DefaultIndexURL`, `SPIN_REGISTRY_URL`, or the `Entry` / `SearchResult` types.

## Risks Specific to This Architecture

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `<alias>/<id>` collides with existing `user/repo` shorthand in `cmd/add.go` (existing code accepts `me/repo` and treats it as a git URL shorthand) | Medium | The existing shorthand only accepts exactly one slash, no scheme, not a path/git URL -- same predicate. Reuse `isShorthand(spec)` to route: if the alias resolves in the local index, prefer that; else fall back to the existing shorthand. |
| `registries.json` is corrupted mid-write | Low | Atomic write pattern (write `.tmp`, fsync, rename) -- same as `writePinned`. |
| User has an old `pinned.json` that includes `Source` URLs the new resolver can't reach (registry removed, repo deleted) | Low | Existing `Loader.cloneGit` already surfaces "git clone failed" cleanly. Pin-hit short-circuit in `cmd/new.go` falls through to the loader on failure. |
| Multiple registries define the same `<id>` | Low | Resolver errors with "ambiguous id 'go-api' in registries: official, community" -- first match wins if user passes just `/go-api` (no alias). |
| User runs `spin search` before `spin registry add` | Medium | Print "no registries registered -- run `spin registry add <alias> <url-or-path>`" instead of empty result. |
| Loader.Load is called with a Source that is a registry dir, not a template dir | Low | Loader's `Detect` requires `spin.toml` + `_base/`; registry dirs have `registry.toml` + `templates/` and will fail Detect cleanly. |

## Sources

- `spin-registry.md` (repo root) -- the v2.x spec; defines registry layout, commands, validation rules
- `.planning/PROJECT.md` -- v2.x Active section: phases 6/7/8, the constraint to drop `SPIN_REGISTRY_URL` / `ErrNotDeployed` / `DefaultIndexURL`, the constraint to keep `pinned.json` unchanged
- `internal/template/loader.go` -- `Load` / `loadPinned` / `cloneGit` / `isLocalPath` / `isGitURL`; the existing three-source spec
- `internal/registry/client.go` -- the two-responsibility file being split; HTTP pieces identified for deletion, pin-state pieces for retention under new name
- `internal/registry/types.go` -- current shape; types being deleted vs added
- `internal/template/template.go` -- `Detect` / `Render` / `RenderToWithPost`; unchanged
- `cmd/add.go`, `cmd/new.go`, `cmd/search.go`, `cmd/list.go`, `cmd/update.go`, `cmd/remove.go` -- the CLI surfaces being modified
- `cmd/update_test.go`, `cmd/new_test.go` -- existing test shape; ensure phase rewrites don't break these

---

*Architecture research for: spin v2.x local-registry milestone (phases 6/7/8)*
*Researched: 2026-07-03*