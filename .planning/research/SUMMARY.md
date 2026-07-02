# Project Research Summary

**Project:** spin (v2.x local-registry milestone)
**Domain:** CLI scaffolder, zero-backend template registry
**Researched:** 2026-07-03
**Confidence:** HIGH

## Executive Summary

The v2.x local-registry milestone replaces `internal/registry`'s HTTP client stub (the `.invalid` URL placeholder) with a git/local filesystem registry model. Registries are first-class entities on disk: `~/.config/spin/registries.json` tracks registered registries, and `~/.config/spin/registries/<alias>/` holds cloned or symlinked registry directories containing `registry.toml` and `templates/*.toml` files. Users get four new `spin registry` subcommands (add/list/update/remove), a local `spin search` that reads TOML directly, and a `<alias>/<id>` shorthand resolved through the index.

The implementation is a pure plumbing exercise: no new dependencies are required. The existing dependency footprint (BurntSushi/toml for TOML, stdlib `os/exec` for git, stdlib `os` for filesystem) covers every new capability. The work is about wiring existing patterns correctly rather than inventing new ones: mirror `writePinned` for atomic `registries.json` writes, mirror `addGit`/`addLocal` for registry clone/symlink, and keep the resolver as a thin adapter that returns a source string consumable by the existing `template.Loader.Load`. The three-phase structure (manager+CLI, index+resolver+rewire, HTTP deletion) is the right split because each phase produces a working artifact and the final phase is pure cleanup.

The most damaging risks are not technical complexity but namespace collisions: alias format validation must be airtight before any filesystem write, the resolution precedence between `<alias>/<id>` and legacy `Pinned.Name` must be explicit and tested, and the transient clone lifecycle (transient temp -> pin or garbage) must be defined before the resolver touches disk.

## Key Findings

### Recommended Stack

**No new dependencies.** The existing Go module graph covers every requirement. BurntSushi/toml v1.6.0 handles registry.toml and all `templates/*.toml` reads. Stdlib `os/exec` with `git clone --depth=1` and `GIT_TERMINAL_PROMPT=0` covers all git operations. Stdlib `os.Symlink` + `copyDir` fallback covers local registry linking. Stdlib `json.MarshalIndent` + temp-file + rename covers atomic `registries.json` writes.

**Do not add go-git.** It would introduce a ~30 MB dep tree for an operation `os/exec` already handles correctly across all existing spin code. Any proposal to add it is a Phase 1 audit blocker.

**Do not pursue `encoding/toml/v2`.** A stale comment in `spin_toml.go` suggests it was promoted to stdlib. It was not. BurntSushi/toml stays.

**Core technologies:**
- `github.com/BurntSushi/toml` v1.6.0 — TOML parsing for registry.toml and templates/*.toml (already direct dep)
- `charm.land/lipgloss/v2` v2.0.3 — styled registry list table output (already direct dep)
- `charm.land/huh/v2` v2.0.3 — interactive confirm on `registry remove` if TTY (already direct dep)
- `os/exec` git — clone/fetch for registry refresh (stdlib, existing pattern)
- `os.Symlink` + `copyDir` — local registry linking (stdlib, existing pattern)
- `json.MarshalIndent` + temp-file + rename — atomic `registries.json` writes (stdlib, mirrors writePinned)

### Expected Features

**Must have (table stakes) — missing any breaks the rewire:**
- `spin registry add <alias> <source>` — git URL or local path; validates alias format before any filesystem write; atomically updates registries.json
- `spin registry list` — shows alias, source, kind, cache path, template count; `--json` for scripting
- `spin registry update [alias]` — git fetch + reset for git registries; no-op with notice for local registries; reports per-registry outcome
- `spin registry remove <alias>` — removes entry from registries.json and deletes cache dir; refuses if pinned templates depend on the registry (unless `--purge-pinned`)
- `spin search <query>` rewired to local TOML — reads `registries/*/templates/*.toml` via Index.Build; replaces HTTP call entirely (HTTP deleted in Phase 8)
- `<alias>/<id>` shorthand in `spin add` and `spin new` — fourth spec kind inserted before `loadPinned` in Loader.Load cascade
- Registry metadata validation — registry-level (registry.toml parses, required fields present, templates/ dir exists) and template-level (TOML parses, required id/name/source present, source is resolvable)
- Invalid-metadata reporting during `spin registry update` — skip-and-warn per file; aggregate summary at end; never abort run
- Pin prompt on registry-resolved `spin new` — existing `promptPinAfterSuccess` fires automatically if the registry-resolved loader sets `tpl.Repo` from the metadata `source` field (not the cache path)
- Drop HTTP client + `SPIN_REGISTRY_URL`/`SPIN_REGISTRY` env vars + `ErrNotDeployed`/`DefaultIndexURL` — Phase 8 cleanup

**Should have (ship-worthy polish):**
- Alias collision check + alias format validation — reject `/`, `\`, `:`, whitespace, `..`, leading `-`, NUL bytes; error before any mkdir/git clone
- `spin registry update --quiet` — suppress per-registry output; print only summary line (CI use case)
- `last_updated` timestamp in registries.json — set by successful `spin registry update`; shown in `spin registry list`
- `spin registry add` source validation — after clone/symlink, stat for `registry.toml`; missing -> rollback and error

**Defer to v2.x+ or indefinitely:**
- Registry health check / `spin registry doctor` — deferred
- `--registry <alias>` filter on search — deferred
- `spin registry info <alias>` — deferred
- Centralized registry server / HTTP API — anti-feature, never
- Cross-device sync — anti-feature, out of scope
- Authenticated registry login — covered by git credentials today
- Built-in default registry — anti-feature (couples spin to an org)
- Web UI for browsing — anti-feature per PROJECT.md
- Auto-update on `spin search` — anti-feature (hostile to offline users)
- Search results cache — anti-feature (reads are already sub-millisecond)
- Registry schema versioning — premature
- `spin.lock` pinning registry commit SHAs — premature

### Architecture Approach

The new architecture splits `internal/registry/client.go` (which currently glues HTTP client + pin state together) into three clean layers: `manager.go` (CRUD over registries.json + clone/symlink/disown), `index.go` (walk `templates/*.toml`, validate, filter), and `resolve.go` (`<alias>/<id>` -> Resolved{Source, Kind}). The existing client.go is renamed `pin.go` after the HTTP pieces are deleted. The critical design property is that `resolve.go` returns a `Source` string (git URL or local path) compatible with the existing `template.Loader.Load` API, so the Loader needs no signature change.

Storage layout:
- `~/.config/spin/registries.json` — array of registry records (alias, source, kind, path, added_at, last_updated)
- `~/.config/spin/registries/<alias>/` — cloned git repo or symlinked local path
- `~/.config/spin/registries/<alias>/registry.toml` — registry-level metadata
- `~/.config/spin/registries/<alias>/templates/*.toml` — per-template metadata

pinned.json format is unchanged. Existing pins keep working.

**Major components:**
1. `internal/registry/manager.go` — new. CRUD over registries.json; clone (git) or link (local) registries; refresh and remove. Exposes `Add`, `List`, `Refresh`, `Remove`, `Get`.
2. `internal/registry/index.go` — new. Builds in-memory index from all `templates/*.toml` across registered registries. Validates per-file. `Search(query, limit)` returns filtered slice.
3. `internal/registry/resolve.go` — new. Parses `<alias>/<id>`. Looks up alias in registries.json, reads `templates/<id>.toml`, returns `Resolved{Source, Kind}`.
4. `internal/registry/pin.go` — renamed from client.go (HTTP bits deleted). Pin state + clone/copy helpers only.
5. `cmd/registry.go` — new. Cobra subcommands: `registry add/list/update/remove`.

### Critical Pitfalls

1. **Alias path traversal** — `spin registry add foo/../bar <url>` can escape the cache root. Prevention: centralized `ValidateAlias` called in cobra Args validator before any filesystem write. Reject `/`, `\`, `:`, `..`, leading `-`, NUL. Assert `filepath.Rel(cacheDir, dest)` produces no leading `..`.

2. **`<alias>/<id>` vs `Pinned.Name` namespace collision** — A string like `example/go-api` can be a legacy pinned name or a registry shorthand. Precedence must be explicit: local path > git URL > `<alias>/<id>` registry template > legacy `Pinned.Name` > `user/repo` shorthand. Document at the resolution site. Test all combinations.

3. **Alias collision across two `spin registry add` calls** — `add A URL1` then `add A URL2` silently destroys the first if the pattern reuses `addGit`'s "clear dest first" dance. Prevention: detect existing alias before any filesystem write; refuse with a clear error unless `--force`. Never mutate the filesystem before updating registries.json.

4. **`git clone` half-state on failure** — A failed clone leaves `registries/<alias>/` with `.git/` but no `registry.toml`. Prevention: clone to a sibling temp dir (`registries/<alias>.new-<ts>/`), validate `registry.toml` exists, then atomic-rename to `registries/<alias>/`. Mirror the `refreshOne` backup-and-rename pattern from `cmd/update.go:106-175`.

5. **Transient clone lifecycle ambiguity** — `spin new <alias>/<id>` fetches to a transient dir; the user declines pin; the clone leaks. Prevention: clone to `os.MkdirTemp(transient/)`; on Pin? yes rename to pinned; on no remove the transient; on crash a startup sweep cleans transient dirs older than 1 hour.

## Implications for Roadmap

### Phase 6A: Manager + `spin registry` CLI + `registries.json`

**Rationale:** The manager is the foundation everything else depends on. It must exist and be correct before the index reader can read registries, before the resolver can look up aliases, and before any CLI command can wire to the new layer. The CLI subcommands are the user-visible proof the manager works.

**Delivers:**
- `internal/registry/manager.go` — Add, List, Refresh, Remove, Get; atomic writeRegistries (mirrors writePinned)
- `internal/registry/types.go` — add Registry, RegistriesConfig, TemplateMetadata; drop Entry, SearchResult, DefaultIndexURL, ErrNotDeployed, ErrNotImplemented
- `cmd/registry.go` — `spin registry add/list/update/remove` wired to Manager
- Alias validation (`ValidateAlias`) in manager + cobra Args validators
- Atomic `registries.json` writes
- Git clone to temp-then-rename (rollback on missing registry.toml)
- Symlink + copy-fallback for local registries with Windows warning
- `spin registry remove` refuses if pinned templates depend on registry (unless `--purge-pinned`)
- `withEmptyConfig(t)` test helper enforcing XDG_CONFIG_HOME isolation

**Avoids:** Pitfalls 1, 3, 4, 7 (alias validation, alias collision, clone half-state, symlink Windows), 8 (remove with pinned templates), 10 (update partial state), 12 (atomic writes), 15 (test isolation)

---

### Phase 7B: Index reader + resolver + CLI rewire

**Rationale:** The manager exists; now wire the index and resolver. This is where `spin search` becomes local-only, where `<alias>/<id>` works in `spin add` and `spin new`, and where the transient clone lifecycle is defined.

**Delivers:**
- `internal/registry/index.go` — Index.Build walks all registries' templates/*.toml; validates; Index.Search filters by substring match on id/name/tags/description
- `internal/registry/resolve.go` — ResolveShorthand parses `<alias>/<id>`, looks up alias, reads `templates/<id>.toml`, returns Resolved{Source, Kind}
- `cmd/search.go` rewired — replaces `Client.SearchWithLimit` HTTP call with `Index.Build().Search()`; drop ErrNotDeployed branch; `spin search --json` now returns real entries
- `cmd/add.go` rewired — detect `<alias>/<id>` (contains `/`, not path, not git URL); route through ResolveShorthand; call pin.go Add with resolved Source
- `cmd/new.go` rewired — same `<alias>/<id>` detection; ResolveShorthand; pin-hit short-circuit (if Source already in pinned.json, use pinned name instead of re-cloning); verify `promptPinAfterSuccess` fires from resolved Source
- Transient clone lifecycle: clone-to-temp-dir-then-rename; startup sweep cleans stale transients
- Resolution precedence documented and tested (local > git > `<alias>/<id>` > Pinned.Name > shorthand)

**Avoids:** Pitfalls 2 (namespace collision), 5 (metadata validation — index reader is where validation runs), 6 (resolution race — snapshot at start of resolution), 13 (transient clone lifecycle), 14 (concurrent update + new — snapshot discipline)

---

### Phase 8: Delete HTTP client + docs

**Rationale:** Cleanup is only safe after Phase 7B lands and zero remaining code references `Client.Search`, `ErrNotDeployed`, `DefaultIndexURL`, or `SPIN_REGISTRY_URL`.

**Delivers:**
- Delete `internal/registry/search.go`
- Rename `internal/registry/client.go` -> `internal/registry/pin.go`; delete HTTP-only code (IndexURL, HTTP, Search, SearchWithLimit, isNetworkError, New() env-var reads)
- Delete `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`, `Entry`, `SearchResult` from types.go
- Update `internal/registry/doc.go`
- Tests updated: `client_test.go` -> `pin_test.go` + new `manager_test.go`/`index_test.go`/`resolve_test.go`; delete tests that set `SPIN_REGISTRY_URL`
- `go build` and `go test ./...` pass with zero references to deleted symbols
- Docs pass: README, PROJECT.md "Validated" section, spin-registry.md revision note

**Avoids:** Anti-pattern 3 from ARCHITECTURE.md (HTTP fallback for search)

---

### Phase Ordering Rationale

- Phase 6A first: manager must own registries.json before any other layer can read or write it. CLI subcommands prove the manager end-to-end.
- Phase 7B second: depends on Phase 6A's registries.json existing. The resolver and index need a manager to talk to. CLI rewires depend on the resolver.
- Phase 8 last: cleanup is only safe when nothing references the old HTTP symbols. Doing it earlier blocks on a half-finished search rewire.

### Research Flags

**Needs research during planning:**
- **Phase 7B (resolver + transient lifecycle):** The transient clone lifecycle decision (clone-to-temp-then-rename vs clone-to-real-then-rm-on-no) has UX and disk usage implications. Recommend settling in a design doc before coding starts.
- **Phase 7B (search scoring):** Sort order in `spin search` — alphabetical by id or relevance scoring? Spec is silent. Recommend relevance scoring but needs a concrete algorithm decision.

**Standard patterns (skip research-phase):**
- **Phase 6A (atomic writes):** writePinned pattern is directly copyable from client.go:557-595.
- **Phase 6A (git clone/fetch):** addGit pattern directly copyable from client.go:222-258.
- **Phase 6A (symlink/copy fallback):** addLocal pattern directly copyable from client.go:181-220.
- **Phase 6A (refresh with rollback):** refreshOne from cmd/update.go:106-175 is directly reusable.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Spec is explicit about no new deps; all capabilities map to existing libs; BurntSushi/toml verified current |
| Features | HIGH | spin-registry.md is concrete and complete; every feature traced to a spec line or a UX gap that is obvious |
| Architecture | HIGH | Integration points are all from existing code; data flows are traced to actual file:line references |
| Pitfalls | HIGH | All pitfalls derived from existing code analysis; prevention strategies tied to specific code patterns |

**Overall confidence:** HIGH

**Gaps to Address:**
- **Resolution precedence** — The spec does not state the order in which local path, git URL, `<alias>/<id>`, pinned name, and `user/repo` shorthand are evaluated. Recommend locking this in a design doc before Phase 7B coding.
- **Sort order in `spin search`** — Alphabetical vs relevance scoring is not specified. Recommend relevance (exact id > name contains > tag contains > description contains) with id tie-break, documented in manpage.
- **Exit code for `spin registry update` on partial failure** — 0 if any registry succeeded, 1 if all failed, or always 0 (warnings only)? Recommend: 0 if at least one registry had valid templates; 1 only if every registry is invalid or missing.
- **Transient clone lifecycle** — Not yet defined in a single place. Recommend design doc before Phase 7B.

## Sources

### Primary (HIGH confidence)
- `spin-registry.md` — the feature contract; defines layout, commands, validation rules
- `.planning/PROJECT.md` — v2.x milestone description; phases 6/7/8; constraint to drop HTTP stub
- `internal/registry/client.go` — existing writePinned, addGit, addLocal, SanitiseRepoName patterns to mirror
- `internal/template/loader.go` — existing Load cascade; site of 4th spec kind insertion
- `cmd/new.go:251-287` — promptPinAfterSuccess hook to preserve
- `cmd/update.go:106-175` — refreshOne rollback pattern to mirror

### Secondary (MEDIUM confidence)
- BurntSushi/toml v1.6.0 via Context7 — confirmed current; Unmarshal API; Encode API
- Integration gotchas around `isLocalPath`/`isGitURL`/`isShorthand` reuse — inferred from shared domain

### Tertiary (LOW confidence)
- New failure modes not covered in existing code (path traversal via alias, soft-delete + shorthand interaction) — inferred, needs validation in tests

---
*Research completed: 2026-07-03*
*Ready for roadmap: yes*
