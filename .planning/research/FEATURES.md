# Feature Research: spin v2.x Local-Registry Milestone

**Domain:** CLI tooling -- local-registry layer for a language-agnostic scaffolder
**Researched:** 2026-07-03
**Confidence:** HIGH (spec in spin-registry.md is concrete; current internal/registry and internal/template packages read directly; only Context7 unverified, but no new library decisions are needed -- the spec reuses github.com/BurntSushi/toml already in tree)

---

## Executive Summary

The v2.x local-registry milestone replaces `internal/registry`'s HTTP `Client` (the `.invalid` stub) with a **git/local-index layer**. Registries are first-class entities on disk (`~/.config/spin/registries.json` + `~/.config/spin/registries/<alias>/`), `spin search` reads them locally, and `<alias>/<id>` becomes a fourth template spec kind alongside local path / git URL / pinned name. The scope is bounded: four new `spin registry` subcommands, two new resolver paths (`<alias>/<id>` in `add` and `new`), one rewire of `search`, and one validation layer that runs during `registry update`.

The spec (`spin-registry.md`) is **opinionated and complete** -- it dictates the layout, the validation rules, the storage locations, and the source-resolver behaviour. The features below are what the spec calls for, **plus** a small set of differentiators the spec leaves implicit but are required to ship a coherent user experience (e.g. alias collision check on `registry add`, JSON output for `spin registry list`, `--quiet` for `registry update`, stable alphabetical sort in `search`, error reporting UX during `registry update`).

Two categories are explicitly NOT features: server-side anything (the whole pitch is "zero backend"), and cross-device sync (registries are local clones; if a user wants sync they put the registry in a git repo they already share). Both are documented as anti-features to lock the scope.

No new dependencies are required. `github.com/BurntSushi/toml` (already in `go.mod` for spin.toml parsing) handles registry.toml and templates/*.toml. The git clone / pull paths already exist in `internal/registry/client.go` and are reused for registries.

---

## Feature Landscape

### Table Stakes (Per spin-registry.md -- Users Expect These)

These are the must-haves called out in the spec. Missing any one of them means the rewire is incomplete and existing v2.0 workflows break or feel broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `spin registry add <alias> <source>` | The single command that makes the whole layer exist. Spec example: `spin registry add official https://github.com/spin-org/registry`. Without it, no discovery. | LOW-MEDIUM | Source resolution: local path (`/`, `.`, `~` prefix) -> symlink under `~/.config/spin/registries/<alias>/`; git URL -> shallow clone same path. Reject `<alias>` that collides with an existing alias unless `--force`. Validate that the source actually looks like a registry (has `registry.toml`) before persisting. |
| `spin registry list` | Discovery mirror of `spin list`. Spec: `spin registry list`. Users must be able to see what registries are registered, where they live on disk, and which are git vs local. | LOW | Show alias, source, kind (git/local), cache path, last-updated timestamp if available. `--json` flag for scripting (matches `spin list --json` UX). Reuse `pinnedRow` table pattern. |
| `spin registry update [alias]` | Git registries need to be pullable; otherwise they go stale. Spec: `spin registry update` (all) and `spin registry update official` (one). | MEDIUM | For each git registry: `git pull --ff-only` in the cache dir (timeout, no prompts). For local: no-op (print a notice). Report per-registry status. Collect warnings into a single end-of-run summary (see "Validation reporting" below). |
| `spin registry remove <alias>` | Symmetric counterpart of `add`. Spec example: `spin registry remove official`. Without it, registries.json grows unbounded. | LOW | Drop the row from `registries.json`; delete the cache dir under `~/.config/spin/registries/<alias>/`. Refuse to remove a registry that currently has a pinned template sourced from it (matches `spin remove --purge` behaviour). |
| `spin search <query>` reads local TOML | The rewire's centrepiece. Spec flowchart: read `~/.config/spin/registries/*/templates/*.toml`, validate, build index, filter, display. Replace `Client.SearchWithLimit` HTTP call. | MEDIUM-HIGH | Each entry has `alias/id`, `name`, `description`, `source`, `tags`, `language`, `type`, `version`, `updated_at`, `downloads` (downloads always 0 in v2.x; placeholder for parity). Sort by `alias/id` asc (no popularity metric yet) or by relevance to query (spec is silent -- see Differentiation note below). Format: lipgloss table, default + `--json` (now meaningful, not a skeleton). |
| `<alias>/<id>` shorthand accepted by `spin add` and `spin new` | Spec's resolution contract: "Find Template Metadata -> Resolve Source". A user typing `spin add official/go-api` or `spin new myapp official/go-api` must work. | MEDIUM | Add to `internal/template/Loader.Load` spec detection (after `isLocalPath`, `isGitURL`, `loadPinned`). Pattern: exactly one `/`, both sides non-empty, neither side contains `/`. On match: scan registered registries for one containing `templates/<id>.toml`; read the `source` field; pass that source to the existing `addGit` / `addLocal` paths. If `source` is itself an alias/id, recurse once (max depth 2; reject cycles). |
| Registry metadata validation (`registry.toml` + `templates/*.toml`) | Spec section "Registry Validation": required fields present, ids unique within a registry, sources resolvable. "Invalid metadata files are ignored and reported during `spin registry update`." | MEDIUM | Three validation buckets: (a) registry-level: `registry.toml` parses, required fields (`id`, `name`) present, `templates/` dir exists; (b) template-level: TOML parses, required fields (`id`, `name`, `source`) present, `source` is one of local/git/alias-id; (c) cross-cutting: no two templates in same registry share the same `id`. Invalid files are skipped from the search index but counted and surfaced. |
| Invalid-metadata reporting during `spin registry update` | Spec: "Invalid template metadata files are ignored and reported during `spin registry update`." | MEDIUM | Aggregate count: "official: 12 templates indexed, 1 skipped (templates/foo.toml: missing 'source' field)." Format: registry-by-registry summary at end of `update` run; exit code 0 if at least one template was indexed, 1 if a registry had zero valid templates (debatable -- flag for phase decision). Never abort the whole update on one bad file. |
| Pin-prompt on `spin new` (registry-resolved templates) | Spec scaffold flowchart K->M: "Pin Template? -> Save to Local Template Cache". The existing `promptPinAfterSuccess` covers this for git-URL and local-path specs. Extend it to fire when the template was loaded via the registry path. | LOW | The `promptPinAfterSuccess` hook in `cmd/new.go:251` already checks `tpl.Repo != ""`. After registry-resolution, `tpl.Repo` is the registry template's `source` field, so the existing hook fires automatically. Verification: ensure the registry-resolved path sets `tpl.Repo` from the metadata's `source` (not the cache path) -- otherwise the pin would pin the local registry-cache copy, not the upstream template. |
| Drop HTTP client + `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` env vars + `ErrNotDeployed` | Project.md constraint: "Drop `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` env vars; drop `ErrNotDeployed` / `DefaultIndexURL`". | LOW | Phase 8 cleanup: delete `internal/registry/client.go` HTTP path (or keep client.go just for Add/Refresh/Pin/Purge), delete `internal/registry/search.go` (HTTP formatter), delete `DefaultIndexURL` const. `spin search` becomes a pure local read. |

### Differentiators (Per spin-registry.md -- Ship-Worthy Polish)

These are not in the spec but are required to ship a coherent UX. Each has a one-line rationale; none bloat scope.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Alias collision check + alias format validation | A typo or duplicate alias corrupts the index silently. | LOW | Reject `<alias>` containing `/`, whitespace, or path separators at `registry add`. Reject `--force`-less collision. Required for `Load(spec)`'s `<alias>/<id>` pattern to work -- if `alias` is ambiguous, the resolver can't pick one. |
| `spin registry list --json` | Same DX story as `spin list --json`. Users expect a machine-readable path; the table is for humans. | LOW | JSON object per registry: `alias`, `source`, `kind` (git/local), `cache_path`, `templates_count`, `last_updated`. |
| `spin registry update --quiet` | Long-running refresh across many registries shouldn't print per-registry output in CI logs. | LOW | Suppress per-registry success lines; print only the summary line. Mirror the pattern from `update_test.go` and the existing `spin update` summary. |
| `spin search --json` with full entries populated | v2.0's `--json` is a skeleton (search.md line 21). v2.x should make it real so CI pipelines can grep `entries` directly. | LOW | Same shape as the existing `SearchResult` (`query`, `total`, `entries[]`), with each entry carrying the resolved `source` and the registry `alias` it came from. |
| `spin search` relevance scoring (vs alphabetical) | Spec is silent on sort order. Naive alphabetical is fine but `--query "go"` matching `go-api`, `gin-rest`, `go-tui`, `cli-go-starter` should rank by token overlap, not alphabetically. | LOW-MEDIUM | Simple substring + token-match scoring: exact id match > name match > description match > tag match; tie-break by id. No external dep, no Lucene-style index. Document the ranking rule in the manpage so it's not magical. |
| `spin registry remove` refuses if pinned-templates depend on the registry | A user who `spin add official/go-api` and then `spin registry remove official` would leave a dangling pin. | LOW | Walk pinned.json: any pin whose `Source` is inside the registry's `source` URL? -- error with the names of the dependent pins; suggest `spin remove <pin> --purge` first. Same UX as `git branch -d` refusing to delete the checked-out branch. |
| Atomic `registries.json` writes | Mirror the existing `writePinned` (atomic temp + rename) for the registry config file. A crash mid-write should not leave the file unparseable. | LOW | Same pattern as `writePinned` in `internal/registry/client.go:561`. Reuse the helper or copy. |
| Last-updated timestamp in `registries.json` | Lets `spin registry list` show "updated 2h ago" without re-stat'ing every cache dir every call. | LOW | Per-registry: `last_updated` set by `spin registry update` (successful pull -> now RFC3339; local registry -> set on `add`, never updated). Cheap, additive. |
| `spin registry add` validates the source is a registry before persisting | A user typing `spin registry add foo https://github.com/me/my-random-repo` would silently get a useless entry. | LOW | After clone/symlink, stat for `registry.toml`. Missing -> error, roll back the clone. Spec says "Required registry metadata fields are present" -- this is the runtime enforcement. |

### Anti-Features (Commonly Requested, Often Problematic)

These are deliberately NOT shipped. Documented to lock scope.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Centralised registry server / HTTP API** | "Single source of truth!" | Defeats the whole pitch. PROJECT.md and spin-registry.md both mandate zero-backend. Server means hosting, moderation, downtime. The whole ecosystem is git repos. | Registries ARE git repos. If you want a single canonical index, publish a git repo and `spin registry add official https://github.com/you/registry`. |
| **Auto-publish on `spin init` / first run** | "Make the discovery frictionless!" | Users who never asked for discovery would get auto-registered registries. Silent network on startup. Breaks the "no surprises" thesis. | Document `spin registry add official ...` as a one-liner onboarding step. (Optionally, an `init` subcommand that prints the recommended `add` -- but does NOT execute it.) |
| **Cross-device sync of registries.json** | "I want my registries on every machine!" | Out of scope for the tool; dotfiles managers (chezmoi, stow, git bare repos) already solve this. | User can symlink `~/.config/spin/registries.json` into their dotfiles repo. |
| **Registry health checks / registry ratings / stars** | "Let me know if a registry is unreliable!" | Requires a server (central metrics) or N registry pings (privacy problem). Adds noise to the local-first UX. | Trust the URL you added. Remove + re-add if a registry goes bad. |
| **`<alias>/<sub-alias>/<id>` deeply nested ids** | "Namespacing!" | The spec is one-slash deep. Adding more levels means every resolver and the template metadata schema change. | Flat model. If users want namespacing, they name their templates accordingly: `backend-go-api`, `frontend-react-app`. |
| **Auto-update registries on every `spin search` call** | "Always fresh!" | Network on every search is hostile to offline users and slow. Surprise. | `spin registry update` is explicit. Document it as the refresh verb. |
| **Caching search results across runs** | "Performance!" | Adds an invalidation problem (when does the cache re-read?). The local TOML read is already fast. | No cache. Reading 100 toml files is sub-millisecond on any modern disk. |
| **Built-in default registry** | "Just include one so `spin search` works out of the box!" | Couples spin to a specific org's registry. Governance / branding risk. | Empty by default. Onboarding docs list popular registries the user can `add`. |
| **`spin registry login` / authenticated registries** | "Support private registries!" | The git URL is the auth surface; private repos work today via `GIT_TERMINAL_PROMPT=0` + SSH key, no extra protocol needed. Adding login duplicates git auth. | Document SSH key setup. If a user adds `https://github.com/org/private-registry`, git's existing credential helper handles auth. |
| **Schema-versioning / migration system for registry.toml** | "Future-proof the format!" | Premature. Add when a breaking change is actually proposed. | Keep the v1 format minimal. Document field semantics so additions don't break parsers (TOML ignores unknown fields). |
| **Lockfiles (`spin.lock`) pinning the resolved registry commit SHAs** | "Reproducible installs!" | Adds significant complexity. The pin record already stores a `version` SHA from `gitHeadSHA`; the registry layer's commit SHA is recoverable from `git -C ~/.config/spin/registries/<alias> rev-parse HEAD` at need. | Defer until reproducible-builds becomes a real complaint. |
| **Web UI / TUI for browsing registries** | "Visual discovery!" | Anti-feature per PROJECT.md ("GUI/TUI mode for the scaffolder itself -- out of scope"). | `spin search` with filtering is the UX. |
| **Registry -> auto-add templates on `registry add`** | "One-step onboarding!" | Couples `add` to a network fetch + write to pinned.json. Surprising side effect. | User runs `spin add <alias>/<id>` explicitly after `spin registry add`. |

---

## Feature Dependencies

```
Phase 6 (A): manager + `spin registry` CLI + `registries.json`
    └──requires──> `internal/registry/manager.go` (new) -- CRUD on registries.json
                       └──requires──> atomic write helper (reuse writePinned)
    └──requires──> alias validation (no `/`, no whitespace)
    └──requires──> `cmd/registry.go` -- new cobra subcommand tree
                       └──requires──> `spin registry add|list|update|remove`
    └──requires──> `internal/registry/source.go` (new) -- detect local-vs-git
                       └──requires──> reuse isLocalPath / isGitURL from client.go
    └──requires──> home-dir resolution (reuses expandHome)
    └──independent──> search rewire (Phase 7)
    └──independent──> <alias>/<id> resolver (Phase 7)

Phase 7 (B): index reader + resolver + rewire
    └──requires──> Phase 6 manager (can read registries.json)
    └──requires──> `internal/registry/index.go` (new) -- scan, validate, build
                       └──requires──> github.com/BurntSushi/toml (already in go.mod)
                       └──requires──> registry metadata struct (registry-level)
                       └──requires──> template metadata struct (per-templates/*.toml)
                       └──requires──> validation rules (required fields, unique ids)
    └──requires──> `internal/template/loader.go` -- add 4th spec kind
                       └──requires──> spec detection (exactly one `/`)
                       └──requires──> index lookup by alias/id
                       └──requires──> source extraction from metadata
    └──requires──> rewire `cmd/search.go` to call index.Search (was HTTP)
    └──requires──> rewire `cmd/add.go` to accept <alias>/<id>
    └──requires──> rewire `cmd/new.go` to accept <alias>/<id>
                       └──requires──> registry-resolved path sets tpl.Repo correctly
                       └──requires──> promptPinAfterSuccess already covers this
    └──enhances──> invalid-metadata reporting (uses index.Validate)

Phase 8 (C): delete HTTP code + docs
    └──requires──> Phase 7 fully landed (search no longer needs HTTP)
    └──requires──> remove `DefaultIndexURL`, `ErrNotDeployed`, `ErrNotImplemented`
    └──requires──> remove `Client.Search`, `Client.SearchWithLimit`, HTTP timeout, isNetworkError
    └──requires──> remove `internal/registry/search.go` (HTTP formatter)
    └──enhances──> `spin search --json` becomes real (entries populated)
    └──independent──> test fixtures for valid + invalid registry TOML
```

### Dependency Notes

- **Phase 6 first:** The manager must exist before anything else can read registries.json. The CLI subcommand tree is the user-visible artifact that proves the manager works end-to-end. <alias>/<id> resolution is impossible without a manager.
- **Phase 7 second:** The index reader needs the manager. The loader's fourth spec kind needs the index reader. `search` and `add` rewires are trivial once both exist.
- **Phase 8 last:** Removing the HTTP path is a "clean up what we replaced" task. Doing it earlier would block on a half-finished search rewire.
- **Pin-prompt inheritance:** The existing `promptPinAfterSuccess` (cmd/new.go:251) is wired off `tpl.Repo`. As long as the registry-resolved loader sets `tpl.Repo` from the metadata `source` field (NOT the local registry cache path), the prompt fires automatically. This means the prompt is NOT a new feature -- it's a verification that the existing hook continues to work for the new spec kind.
- **Reuse of existing helpers:** `expandHome`, `copyDir`, `os.Symlink`, `gitHeadSHA`, `SanitiseRepoName`, `writePinned` -- all reusable. The git clone path for registries is identical to the existing `addGit` minus the `templates/` subdirectory destination. Strong DRY case for a small helper in `internal/registry/source.go`.
- **No new deps:** TOML via `github.com/BurntSushi/toml` (already in go.mod for `internal/template/spin_toml.go`). Git operations via `os/exec`. No external registry clients (the whole pitch).

---

## MVP Definition

### Launch With (v2.x)

These are non-negotiable. The milestone is incomplete without every one.

- [ ] `spin registry add <alias> <source>` -- works for local path AND git URL
- [ ] `spin registry list` -- shows alias, source, kind, cache path, template count
- [ ] `spin registry update [alias]` -- git pull per registry, no-op for local
- [ ] `spin registry remove <alias>` -- deletes entry + cache
- [ ] `spin search <query>` reads local TOML only (HTTP path deleted in Phase 8)
- [ ] `<alias>/<id>` accepted by `spin add` and `spin new` (4th spec kind)
- [ ] Registry metadata validation (registry.toml + templates/*.toml)
- [ ] Invalid-metadata reporting during `spin registry update`
- [ ] `pin.json` format unchanged (backward compat with existing v2.0 pins)
- [ ] Pin-prompt fires on registry-resolved `spin new` (regression test for promptPinAfterSuccess)
- [ ] Alias collision + format check on `registry add`
- [ ] `registries.json` atomic writes
- [ ] `spin registry remove` refuses if pinned templates depend on the registry
- [ ] Drop HTTP client + env vars + `ErrNotDeployed` + `DefaultIndexURL`
- [ ] `spin search --json` populates `entries` (no longer a skeleton)
- [ ] `spin registry list --json` for scripting

### Add After Validation (v2.x+)

- [ ] `spin registry update --quiet` for CI friendliness
- [ ] `last_updated` timestamp per registry (cheap; nice UX)
- [ ] `--registry <alias>` flag on `spin search` to scope results
- [ ] `spin registry info <alias>` (debug aid: show resolved paths, last error, etc.)
- [ ] `spin registry doctor` (walk every registry, report health)

### Future Consideration (v3+)

- [ ] Cross-device sync of registries (out of scope; user's dotfiles)
- [ ] Registry server / HTTP API (anti-feature; defer indefinitely)
- [ ] Authenticated registry login (covered by git credentials today)
- [ ] Registry schema versioning + migration tooling (premature)
- [ ] `spin.lock` pinning registry commit SHAs (premature)
- [ ] Web UI for browsing registries (anti-feature per PROJECT.md)
- [ ] Built-in default registry (couples spin to specific org)
- [ ] Auto-update on `spin search` (hostile to offline users)
- [ ] Cache layer for search results (unnecessary; reads are fast)

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| `spin registry add` (local + git) | HIGH | LOW | P1 |
| `spin registry list` | HIGH | LOW | P1 |
| `spin registry update` | HIGH | MEDIUM | P1 |
| `spin registry remove` | HIGH | LOW | P1 |
| `spin search` rewire to local TOML | HIGH | MEDIUM | P1 |
| `<alias>/<id>` in `add` and `new` | HIGH | MEDIUM | P1 |
| Registry metadata validation | HIGH | MEDIUM | P1 |
| Invalid-metadata reporting | HIGH | LOW | P1 |
| Pin-prompt on registry-resolved `new` | HIGH | LOW | P1 (regression test) |
| Drop HTTP code (Phase 8) | HIGH | LOW | P1 |
| Alias collision + format check | MEDIUM | LOW | P1 |
| `registries.json` atomic writes | MEDIUM | LOW | P1 |
| `registry remove` refuses dependent pins | MEDIUM | LOW | P1 |
| `spin search --json` populated entries | MEDIUM | LOW | P1 |
| `spin registry list --json` | MEDIUM | LOW | P1 |
| `spin registry update --quiet` | MEDIUM | LOW | P2 |
| `last_updated` per registry | LOW | LOW | P2 |
| `--registry <alias>` filter on search | MEDIUM | LOW | P2 |
| `spin registry info <alias>` | LOW | LOW | P2 |
| `spin registry doctor` | LOW | MEDIUM | P3 |
| Centralised registry server | LOW (until needed) | HIGH | Anti (defer) |
| Cross-device sync | LOW | HIGH | Anti (out of scope) |
| Registry login / auth | LOW | MEDIUM | Anti (git covers it) |
| Built-in default registry | LOW | LOW | Anti (couples spin to org) |
| Web UI for registries | LOW | HIGH | Anti (out of scope per PROJECT.md) |
| Auto-update on search | LOW | MEDIUM | Anti (hostile to offline) |
| Search results cache | LOW | MEDIUM | Anti (premature) |
| Registry schema versioning | LOW | MEDIUM | Anti (premature) |
| `spin.lock` registry SHAs | LOW | MEDIUM | Anti (premature) |

**Priority key:**
- P1: Must have for v2.x milestone launch
- P2: Should have, add when budget allows
- P3: Nice to have, future consideration (or never)

---

## Validation Rules Summary

Centralised here so phase planning can split these between index reader and update reporter.

### Registry-level (`registry.toml`)

- File exists at `<cache>/registry.toml`. Missing -> registry is invalid; refuse to register at `add` time, report at `update` time.
- TOML parses. Parse error -> invalid; same reporting.
- Required fields present: `id`, `name`. Optional: `description`, `homepage`, `maintainer`, `license`.
- `templates/` directory exists. Missing -> invalid.

### Template-level (`<cache>/templates/*.toml`)

- File exists and TOML parses. Parse error -> skip file, increment counter, collect error.
- Required fields present: `id`, `name`, `source`. Optional: `description`, `tags`, `authors`, `license`, `homepage`, `language`, `type`, `version`, `updated_at`, `downloads`.
- `source` is non-empty and matches one of: local path (`/`, `.`, `~` prefix), git URL (scheme prefix), or `<alias>/<id>` shorthand. Anything else -> skip with "unresolvable source" error.
- `id` is unique within the registry. Duplicate -> skip the SECOND occurrence, report both.

### Cross-cutting

- Alias uniqueness across `registries.json`. Enforced at `registry add` (refuse collision without `--force`).
- Alias format: no `/`, no whitespace, no `..`, no path separator. Enforced at `registry add`.
- Alias length: 1-64 chars. Reasonable upper bound.

### Error Reporting UX (during `spin registry update`)

For each registry, after update completes, print ONE line:

```
<alias>: updated <n> templates, skipped <m> (registry)
<alias>: updated <n> templates, skipped <m> (local)         # local is always "updated 0, skipped 0"
<alias>: failed to update: <error>                         # git pull failed
<alias>: invalid registry: <reason>                        # registry.toml missing/malformed
```

Followed by per-file errors (capped at 5 per registry to avoid log spam):

```
  templates/foo.toml: missing required field 'source'
  templates/bar.toml: duplicate id 'go-api' (also in templates/baz.toml)
  templates/baz.toml: TOML parse error: ...
```

Exit code: 0 if at least one registry updated successfully and no fatal errors; 1 if ALL registries failed (debatable -- phase decision). Per-registry warnings never abort the run.

---

## Migration & Compatibility Notes

These are not features per se but are constraints the implementation MUST honour.

- **`pinned.json` format unchanged.** Existing v2.0 pins (Name, Source, PinnedAt, Version, LocalPath, Removed) keep working. Phase 7's <alias>/<id> resolver returns the same `Pinned` shape that `addGit` / `addLocal` return today.
- **No `SPIN_REGISTRY_URL` or `SPIN_REGISTRY` env var.** Drop them in Phase 8. If a user has either set, ignore it silently (don't break their shell env). Document in changelog.
- **First-run UX.** A user with no registries registered who runs `spin search foo` should get a helpful hint, not just "0 results". Print: "no registries registered; run `spin registry add <alias> <git-or-path>` to add one" then exit 0.
- **Tests must not regress.** `internal/registry/client_test.go` and `internal/template/loader_test.go` cover the v2.0 surface. Phase 8 removes HTTP tests; all other tests must keep passing unchanged. The new <alias>/<id> path needs fresh fixtures under `internal/registry/testdata/registry-{valid,invalid,duplicate-id}/`.

---

## Open Questions for Phase Decisions

These are spec-ambiguous. Flag for the requirements doc or first-phase discussion.

1. **Sort order in `spin search`:** alphabetical by id, or relevance scoring? Spec silent. Recommend relevance scoring with id tie-break; document the rule.
2. **`spin registry update` exit code on partial failure:** 0 if any succeeded, 1 if all failed, or always 0 (warning-only)? Recommend: 0 if any registry updated successfully OR any registry had at least one valid template; 1 only if EVERY registry is invalid/missing.
3. **Duplicate-id behaviour:** Skip second occurrence (recommended for safety), or refuse to register the registry at all? Recommend: skip + report, never block.
4. **Private git registry URLs:** Just works via existing git credential helpers? Or warn on `https://` without a credential helper? Recommend: works as-is; spec is silent on auth, and the existing `GIT_TERMINAL_PROMPT=0` is preserved.
5. **`spin registry add` of a registry that is the parent dir of an existing template:** Allowed? The user's mental model might say "my repo has a subdir that is also a registry". Recommend: allow; the resolver sees `registry.toml` at root and registers as registry.
6. **`downloads` field on indexed entries:** Always 0 in v2.x, or omit entirely? Spec lists it in `Entry` but no metric to populate it. Recommend: omit from display; include field in JSON as 0 for forward compatibility.

---

## Sources

- `spin-registry.md` (project root, 2026-07-03) -- the spec under research; HIGH confidence, single source of truth
- `.planning/PROJECT.md` (Current Milestone: v2.x local-registry, Constraints) -- scope and constraints; HIGH confidence
- `.planning/PROJECT.md` (v2.x Pivot 2026-06-10) -- rationale for templates-as-only-extension-surface; HIGH confidence
- `internal/registry/client.go` -- existing pin/unpin/list/refresh paths to reuse; HIGH confidence (direct read)
- `internal/registry/types.go` -- existing Pinned, Entry, SearchResult shapes; HIGH confidence (direct read)
- `internal/registry/search.go` -- existing HTTP-only formatter to delete in Phase 8; HIGH confidence (direct read)
- `internal/template/loader.go` -- existing spec detection (local / git / pinned); site of the 4th spec kind; HIGH confidence (direct read)
- `internal/template/template.go` -- Detect + Render + post-hook; what registry-resolved templates must satisfy; HIGH confidence (direct read)
- `cmd/search.go` -- current HTTP-using search, to be rewired; HIGH confidence (direct read)
- `cmd/add.go` -- current Add path, to accept `<alias>/<id>`; HIGH confidence (direct read)
- `cmd/new.go` (runNew, promptPinAfterSuccess at line 251) -- pin-prompt logic the registry path must preserve; HIGH confidence (direct read)
- `cmd/list.go` -- table + `--json` UX pattern for `spin registry list`; HIGH confidence (direct read)
- `cmd/update.go` -- rollback + summary + warn-on-fail pattern for `spin registry update`; HIGH confidence (direct read)
- `cmd/remove.go` -- soft-delete + `--purge` pattern; matches the "remove refuses if dependent pins" anti-feature resolution; HIGH confidence (direct read)
- `docs/commands/search.md` -- current search semantics being replaced (v2.0 HTTP stub); HIGH confidence (direct read)
- `.planning/research/FEATURES.md` (2026-06-02) -- prior research, including the anti-feature precedents ("online template marketplace / registry" listed as anti-feature in v1); HIGH confidence (direct read)

---

*Feature research for: spin v2.x local-registry milestone*
*Researched: 2026-07-03*