# Pitfalls Research: spin v2.x Local Registry Milestone

**Milestone:** v2.x -- replace HTTP registry stub with zero-backend git/local registries
**Researched:** 2026-07-03
**Confidence:** HIGH for integration patterns against existing code (read in full); MEDIUM for new failure modes (alias validation, transient clone lifecycle) where the spec is silent

---

## Executive Summary

The v2.x local-registry milestone introduces one new code path (the **Registry Manager** + `<alias>/<id>` resolver) and removes another (the HTTP `Search()`). The most damaging pitfalls cluster around two axes:

1. **Alias / id namespace collisions.** The registry layer introduces three separate keyspaces (`alias`, `registry id`, `template id`) that share the same physical directory layout (`~/.config/spin/registries/<alias>/templates/<file>.toml`). Each can shadow the other or overlap with the existing `Pinned.Name` keyspace in `pinned.json`. The spec defines the boundaries but leaves validation to the implementation -- which is where most rewrites happen.

2. **Concurrent file-system operations on a shared cache.** Two `spin` processes (or one scaffolder + one update) touching `~/.config/spin/registries/<alias>/` at the same time will collide: a `git fetch` overwrites an in-progress `rm -rf`; a `spin new` mid-resolve loses its pointer into `registries/<alias>/templates/`. The existing `pinned.json` atomic-write pattern (`writePinned` -- marshal, write tmp, fsync, rename) is the model to copy; the existing failure modes around `git clone` on a non-empty dest are the model to NOT copy (see loader `destExists`/`destReuse`/`destPin`/`destWipe`/`destCancel`).

The big-picture lessons:
- **Reuse the loader's "ask before touching" pattern** for `spin registry add` on an existing dest.
- **Borrow `writePinned`'s temp-file-then-rename discipline** for `registries.json`.
- **Treat the transient-clone lifecycle as three states** (transient, pinned, garbage) with explicit transition points; the spec's "Pin Template?" fork is the moment of conversion.
- **Reserve the alias keyspace early** (`validateAlias` runs in cobra `Args`); never let an invalid alias reach the filesystem.

---

## Critical Pitfalls

### Pitfall 1: Alias accepts path separators, dot-files, or NUL bytes

**What goes wrong:** `spin registry add ../etc <url>` or `spin registry add . <url>` or `spin registry add foo/bar <url>` lands on disk at an unintended location, escapes the cache root, or collides with an existing alias. Because `~/.config/spin/registries/<alias>/` is a literal path join (`filepath.Join(c.CacheDir, "registries", alias)`), a `/` in the alias lets a user write outside the cache root. A `.` or `..` alias silently resolves back to the registry parent, breaking `spin registry update .` and `spin registry remove .`.

**Why it happens:** The new "registry" layer adds a *user-supplied* directory component (the alias) that wasn't previously the user's concern. The existing `validateTemplateName` in `cmd/init.go` already rejects `/`, `\\`, `.`, `..`, NUL -- but only for `spin init`. The new commands need the same discipline (or shared helper).

**Consequences:**
- Path traversal: an alias of `../../../tmp/evil` lets a clone land outside `~/.config/spin/registries/`, then later `spin registry remove ../..` removes *spin* config rather than the registry.
- Hidden collision: `spin registry add local ~/foo` followed by `spin registry add local ~/bar` -- the second `add` silently `rm -rf`'s the first if you reuse `addGit`'s "clear dest" pattern.
- Args-validator bypass: cobra's `Args` validator runs once; if a path-separator alias slips through (e.g. via a flag), you discover it deep in the manager.

**Prevention:**
- Centralise alias validation in `internal/registry/manager.go` as a `ValidateAlias(string) error` helper. Reject empty, `.`, `..`, anything containing `/`, `\\`, `:` (Windows drive letter), `*`, `?`, `"`, `<`, `>`, `|`, leading `-`, NUL byte.
- Call it from cobra `Args` validators for every `spin registry *` subcommand (`add`, `remove`, `update <name>`). Mirror `validateTemplateName`'s style and reuse if practical.
- Test cases (every one must produce a clear error, not a partial write): `..`, `../foo`, `foo/bar`, `foo\\bar`, `C:foo`, `-foo`, `.`, ``, `.git`, `foo.lock` (Windows-reserved suffixes), `CON` (Windows device name -- maybe out of scope but worth a comment), `git@github.com` (so a pasted URL doesn't silently become an alias), ` registries` (leading whitespace), trailing whitespace.
- Invariant test: `filepath.Join(c.CacheDir, "registries", alias)` must remain *inside* `filepath.Join(c.CacheDir, "registries")` -- assert with `filepath.Rel` and check for leading `..`.

**Warning signs:**
- A `~/.../registries/<alias>/` directory whose parent is not `~/.config/spin/registries/`.
- Two registered registries with the same alias field in `registries.json`.
- `spin registry list` showing paths that escape the cache root.

**Phase to address:** Phase 6A (manager + CLI) -- must be in place before any `add` command can write to disk.

---

### Pitfall 2: `<alias>` collides with the existing `Pinned.Name` keyspace

**What goes wrong:** `pinned.json` stores templates keyed by `Name` (e.g. `vercel/nextjs-tailwind`). The new spec says `spin new example/go-api` looks up `<alias>/<id>`. If a user already pinned a template named `example/go-api` (via the old `spin add user/repo` flow) before the migration, and `spin add example/go-api` now ALSO succeeds (registry-aware), the same spec maps to both a registry template and a pinned clone. Either (a) lookup is non-deterministic, (b) one shadowing breaks the other, or (c) a registry template's pinned clone silently takes the alias as its `Name`, conflicting with an existing pin.

**Why it happens:** The short spec (`<alias>/<id>`) is a name. `pinned.json` rows are names. Until the resolver picks an order, "same string" is ambiguous. The spec actually addresses this implicitly by asking for `registries.json` + `registries/<alias>/` storage, but the *resolution order* is not specified:
1. Local path?
2. Git URL?
3. Pinned name?
4. `<alias>/<id>` registry template?
5. `<user>/<repo>` shorthand?

**Consequences:**
- `spin add example/go-api` when `example` is a registry and `go-api` is a template in it: should pin the *registry template's source*, not treat the whole string as a pinned name.
- `spin add example/go-api` when `example/go-api` is already in `pinned.json` (legacy): should be a de-duped re-pin, not create a second row with the same `Name`.
- A user's existing `spin list` view suddenly includes registry-resolved templates mixed with old-school pins, and the human can't tell which is which.

**Prevention:**
- Define and document a *single precedence order* at the resolution site (preferred: local path > git URL > `<alias>/<id>` registry template > legacy `Pinned.Name` > `user/repo` shorthand). The legacy `Pinned.Name` lookup is "anything with no slash and no scheme," and `<alias>/<id>` is "exactly one slash, both sides non-empty."
- The `<alias>` portion must resolve against `registries.json` *before* the template-id portion is checked. A failing alias lookup errors fast ("no such registry") rather than falling through to a misleading "no pinned template" message.
- The pinned record created from a registry template gets `Source = registryTemplate.Source` (the underlying git URL or local path from the registry's toml), `Name = "<alias>/<id>"`, and `LocalPath = <cache dir for that clone>`. De-dup on `Name`. See loader's `Pin` (replacing) over `append`.
- Test case (regression): install the legacy pin, then `spin registry add` a registry whose template id is the same. `spin new` should still find the legacy pin first by Name, OR the registry template -- whichever the chosen precedence says. Then add a second test that *flips* the precedence and verify both paths produce the right output.
- Document the precedence in `registries.json` documentation / inline comment in `cmd/add.go` resolution site.

**Warning signs:**
- `pinned.json` has two rows with the same `Name`.
- `spin list` shows both an old-school pin and a registry-resolved pin with the same display name.
- `spin new` succeeds but the underlying clone is one the user did not expect (see also Pitfall 8 -- transient clone lifecycle).

**Phase to address:** Phase 7B (resolver). Document the precedence in code before any resolution code lands.

---

### Pitfall 3: Alias collisions across registries (`spin registry add local A; spin registry add local B`)

**What goes wrong:** The second `add` with the same alias silently destroys the first registry's clone -- if you reuse `addGit`'s "remove existing dest" pattern. `addGit` (in `internal/registry/client.go:222-259`) does `os.RemoveAll(dest)` before re-cloning because the assumption was "this pin name uniquely identifies the cache." Aliases are *also* user-supplied, but they can legitimately clash: the user might `spin registry add official https://github.com/spin/registry-v1` then upgrade to `spin registry add official https://github.com/spin/registry-v2`. The current loader pattern silently drops v1.

**Why it happens:** The spec doesn't say what to do on a duplicate alias. Two valid readings:
- "Update in place -- replace the source/URL/path of the existing registry."
- "Error out -- refuse, ask the user to remove the existing registry first."

**Consequences:**
- Silent data loss: v1's clone directory is `rm -rf`'d before v2's clone starts. If v2's clone fails midway, the user's registry is gone.
- Index drift: `registries.json` is updated *last* in the pattern (Pin-then-write), but if the clone fails, you've already touched the filesystem. The atomic-write pattern protects `registries.json` but not the on-disk tree.
- A user with no existing `registries.json` entry, who types the wrong alias twice, gets two registry-of-the-same-name entries that point to different source URLs. The first wins on read; the second's data is stranded.

**Prevention:**
- Define the contract for `spin registry add <existing-alias> <new-source>`:
  - Either: refuse with "registry already registered as <alias>, run `spin registry remove <alias>` first" (force the user to be explicit).
  - Or: same as `spin registry update <alias>` semantics (refresh in place; warn if metadata changes).
- Recommend the refuse-stance as the default; offer `--force` or `--replace` for users who want the silent flow.
- When refusing: do the validation BEFORE `os.MkdirAll` and BEFORE `git clone`. Never create partial state.
- Atomicity for the in-place refresh path: clone to a *temp* sibling dir (`registries/<alias>.new-<ts>/`), swap on success. Mirror `refreshOne` in `cmd/update.go:106-175` (backup-and-rename pattern).
- Test: `add A URL1`, `add A URL2` -> second must error with "already registered"; `spin registry list` still shows A pointing at URL1; on-disk clone of A is URL1's content.

**Warning signs:**
- A second `registries.json` write for the same alias in a test trace.
- A clone directory that was `rm -rf`'d but never repopulated (orphan tmp dir; half-clone index files; no `registry.toml`).
- Two registries with the same alias in `registries.json`.

**Phase to address:** Phase 6A (manager + CLI).

---

### Pitfall 4: `git clone` failures leave `registries/<alias>/` in a half-cloned state

**What goes wrong:** `spin registry add official https://github.com/example/registry` calls `git clone --depth=1 <url> <dest>`. Possible failures:
- `<dest>` already exists (alias collision or user typo, see Pitfall 3): `git clone` errors "fatal: destination path '...' already exists and is not an empty directory."
- Network failure (DNS, timeout, refused): `registries/<alias>/` doesn't exist yet, so the failure is clean. But if you used `git init` first to pre-create it, the user is left with a `.git/` skeleton.
- Network is slow: 2-minute timeout from `addGit` (line 237) is reasonable but a flaky mobile network can stall past it.
- Authentication required: `git clone` prompts by default; `addGit` sets `GIT_TERMINAL_PROMPT=0` (line 240) but the error message is opaque ("could not read Username for 'https://github.com': terminal prompts disabled").
- Shallow clone of an empty repo: `gitHeadSHA` returns "", we set `Version = "git"` -- fine.

**Why it happens:** `addGit` and the loader's `cloneGit` use a common pattern (`exec.Command("git", "clone", "--depth=1", url, dest)`). They pre-`RemoveAll(dest)` for the pin path; the registry manager needs to either NOT pre-clear (in case of collision) or check first.

**Consequences:**
- A "ghost" registry: `registries/<alias>/` exists as a directory with `.git/` but no `registry.toml`. The registry is "registered" in memory but invalid; every subsequent `spin registry update` / `spin search` reads an invalid directory.
- Confusing error message in CI: "could not read Username" doesn't tell the user "your repo is private."

**Prevention:**
- Use `git clone --depth=1 <url> <tmp-dest>` then `os.Rename(tmp-dest, real-dest)` so a failed clone never leaves the alias directory polluted. Reuse the `refreshOne` backup-and-rename atomic pattern from `cmd/update.go`.
- Pre-check: `os.Stat(real-dest)` MUST succeed (i.e., the dest is missing) before starting the clone. Pair with the alias-collision check from Pitfall 3.
- Validate the cloned result immediately: after `git clone` returns, `os.Stat(filepath.Join(dest, "registry.toml"))` MUST succeed before we persist `registries.json`. If it doesn't, `os.RemoveAll(dest)` and return a `registry: clone missing registry.toml` error.
- Map common git errors to friendly messages: "could not read Username" → "registry requires authentication; use SSH or a public repo"; "Connection refused" → "could not reach registry host"; "Repository not found" → "the URL does not point to a git repository."
- Test: pre-create `registries/<alias>/foo.txt`, attempt `add` with a URL that errors midway (e.g., password-protected) -- after the error, `registries/<alias>/` should be the pre-existing state (untouched), not half-cloned.

**Warning signs:**
- An alias directory exists in `registries/` but doesn't contain `registry.toml`.
- `git status` reports the dir as a repo without remotes.
- A user reporting "`spin registry list` shows my registry but `spin registry update` says 'not a registry.'"

**Phase to address:** Phase 6A (manager).

---

### Pitfall 5: Registry metadata validation -- malformed TOML, missing fields, duplicate ids

**What goes wrong:** The registry layout (`registry.toml` + `templates/*.toml`) is user-controlled -- the registry author can put anything in it. A `templates/foo.toml` with no `id` field is unreadable (we'd have to use the filename as a fallback, but then `example/go-api` and `example/go-api2` both work with no semantic id). Two `templates/*.toml` files with the same `id` field collide. A template's `source = "evil://...` opens the door to whatever git/local source the registry author wants. A `registry.toml` with missing `name`/`id` is technically valid TOML but usefully invalid.

**Why it happens:** The spec lists "validation rules" and says "Invalid template metadata files are ignored and reported during `spin registry update`." The validation discipline is unspecified:
- Filename-derived fallback for missing `id`?
- Resolution when two templates have the same `id`?
- Whether `source` is verified at parse time or only at install time.
- What "report" means: stderr? return value? log file?

**Consequences:**
- Two template ids in one registry: search results are non-deterministic (map iteration order or filename order?). User searching `go-api` could get either.
- Empty or weird `id` (e.g., contains spaces, slashes, NUL): `<alias>/<id>` resolution breaks or accepts garbage.
- `source = "evil://"` or `source = "/etc/passwd"` passed straight to `loader.Load` could exfiltrate or read filesystem paths. The loader's `isLocalPath` doesn't reject `/etc/...` (only `~` and `.`); the manager should be paranoid.
- A noisy registry (50% bad templates) clogs `spin registry update` output.

**Prevention:**
- Define a `ValidateTemplateMeta(*TemplateMeta) error` function in `internal/registry/meta.go`. Required fields: `id`, `name`, `source`. Optional but validated if present: `tags` (each non-empty), `authors` (each non-empty), `license`, `homepage`.
- `id` rules: non-empty; no `/`, `\\`, NUL; max 64 chars; lowercase letters / digits / `-` / `_` / `.` (URL-safe, filesystem-safe).
- `source` rules: must pass `isLocalPath || isGitURL || isShorthand`. Reject anything else (a registry can't author templates that the loader can't resolve).
- For the *whole-registry* metadata (`registry.toml`): required fields `id`, `name`. Must match the alias in `registries.json` (the registry's `id` from the toml == the alias used at `add` time? Or are they independent? Decide and document -- recommend strict equality so a renamed `registry.toml` surfaces as an error rather than silently rebinding).
- Duplicate ids within one registry: warn at update time, error at `spin new <alias>/<id>` time. Return an error like "registry 'foo' contains two templates with id 'go-api'; skip duplicates by filename."
- On `spin registry update`, aggregate validation errors and print them (one per bad file) to stderr; do NOT exit non-zero -- partial registries are still useful.
- Test: registry with (a) template missing `id`, (b) two templates with same `id`, (c) template with `source = "evil"`, (d) template with `id = "go/api"`. Each must produce the right error or warning, not panic.

**Warning signs:**
- A panic from a nil-pointer on a `nil` `id`.
- A search returning duplicates.
- A `source` value that escapes the validation in `loader.Load`.
- `spin registry list` followed by `spin search` showing a different result count.

**Phase to address:** Phase 7B (index reader / resolver) -- the validation lives where the metadata is consumed, but the rules are written with Phase 6A so the storage layout is consistent.

---

### Pitfall 6: `<alias>/<id>` resolution race when the registry list changes mid-`spin new`

**What goes wrong:** Scenario:
1. User runs `spin new myapp --template official/go-api`. The loader reads `registries.json`, sees `official` pointing at `registries/official/`, parses `templates/go-api.toml`, clones the source, renders.
2. *Concurrently* (different shell, CI), the user runs `spin registry remove official`.
3. Halfway through step 1 (after the clone is done, before the render is complete), `registries/official/` is `rm -rf`'d by the `remove` command.
4. `spin new` either: (a) re-reads `registries.json` and finds `official` missing -> cryptic "registry not found" after a successful clone; (b) has already cached the metadata and proceeds; (c) re-reads while `remove` is mid-write and gets a corrupt or truncated `registries.json`.

**Why it happens:** Read/write of `registries.json` is best-effort; nothing about a `spin new` invocation takes a lock on the registry. `writePinned` solves the "partial write" problem but not the "concurrent read+remove" problem.

**Consequences:**
- Scrambled/missing `registries.json` from a torn write (mitigated by atomic write -- see Pitfall 12).
- Mid-flight clones whose `Source` no longer points anywhere; the user sees "template was scaffolded from an unknown registry" later in their project's history.
- Stale `spin list` results that reference gone registries.

**Prevention:**
- Use the same atomic-write discipline for `registries.json` as `pinned.json`: write to `<file>.tmp`, fsync, rename. Already implied; make it a test invariant.
- For `spin new`'s read path: snapshot the relevant registries at the *start* of resolution (read `registries.json`, walk every `registries/<alias>/templates/`, build an in-memory map keyed on `<alias>/<id>`). Use the snapshot for the duration of the resolution. Do NOT re-read on every phase.
- Document that `spin registry add/remove/update` are *not* expected to be safe to run while `spin new` is mid-flight on the *same machine*. Same-machine concurrency is rare in CLI tools; cross-machine is irrelevant. If it becomes a concern, add a file lock (`flock(2)` via `syscall.Flock`) -- but defer until evidence shows it's needed.
- Test (concurrent): `go test -race ./internal/registry/...` with goroutines reading and removing simultaneously; assert no panic and no inconsistent read.

**Warning signs:**
- A panic on `registries/<alias>/templates/` being missing when the index reader tries to walk it.
- `registries.json` parse error after a `remove`.
- A `spin new` succeeding with a template that has since been removed.

**Phase to address:** Phase 7B (index reader + resolver). Phase 6A establishes the atomic-write discipline.

---

### Pitfall 7: Symlink vs copy semantics for local registries on Windows

**What goes wrong:** The existing `addLocal` (lines 181-220 in client.go) tries `os.Symlink(src, dest)` first, then falls back to `os.Symlink(...)+copyDir(...)` if symlink fails. On Windows, `os.Symlink` typically fails unless `SeCreateSymbolicLinkPrivilege` is granted (per existing code comment line 207). On macOS/Linux, symlinking a directory the user owns is fine; symlinking across filesystem boundaries is sometimes rejected by the kernel. If we symlink `~/registry` to `~/.config/spin/registries/local/`, then the user edits `~/registry/registry.toml`, the change is visible immediately -- which is what we want. But on Windows where the symlink fails silently, we fall back to a copy -- which means the user's edits to `~/registry/` are NEVER reflected in `~/.config/spin/registries/local/`. They edit, save, re-run `spin search`, see stale results, file a bug.

**Why it happens:** The asymmetry between platforms is intentional in OS design (symlinks require elevated privileges on Windows) but creates a "works on macOS, broken on Windows" UX.

**Consequences:**
- User edits a template toml in their local registry repo, the scaffolder doesn't see the edit, they think the scaffolder is broken.
- `spin registry update local` on a copy-symlinked registry silently does nothing useful (the source-of-truth isn't the symlink target).
- Travis/Azure DevOps runners on Windows: tests that depend on symlink semantics fail mysteriously.

**Prevention:**
- For LOCAL registries (not local pin paths -- these are full registries), prefer the copy semantics by default and ship a `--symlink` flag for users who know their environment supports it. Document the choice in `spin registry add --help`.
- OR: detect symlink support up front (try-and-fallback as the loader does) and warn the user the first time we fall back: `copying instead; edits to your source directory will not be reflected -- run 'spin registry update' to refresh`.
- For `spin search` performance: build an in-memory index on `spin search` start (and invalidate on `spin registry update`). A re-read on every search via copy would be fine performance-wise but surprises users who expect "what you see is what you get" with symlinks.
- Reference: same pattern as `addLocal` at lines 181-220 of client.go -- keep the try-then-fallback, but add the warning.

**Warning signs:**
- A user reports "I edited the toml, ran search, no change."
- A Windows CI run fails to find a template that the local registry clearly has.

**Phase to address:** Phase 6A (manager + CLI).

---

### Pitfall 8: `spin registry remove <alias>` -- removing a registry that has pinned templates

**What goes wrong:** The user has:
- `spin registry add official https://...`
- `spin add official/go-api` (creates a pin)
- `spin add official/rust-cli` (creates another pin)
- `spin registry remove official`

What happens to the pins? Three reasonable choices:
- **Cascade-delete** the pins AND their on-disk caches (matches GitHub's "removing a source repo de-pins everything").
- **Warn-but-keep** the pins (they keep working; the user's `pinned.json` is the source of truth, the registry is just the discovery index).
- **Refuse** unless `--force` (the user's data is preserved by default).

The spec is silent here. The existing `soft-delete + --purge` pattern (cmd/remove.go:38-75) is the model that doesn't lose data.

**Why it happens:** A registry "remove" feels like an admin operation; a pin is a user-level data decision. The two should be independent.

**Consequences:**
- Cascade-delete destroys user data ("I just lost all my pinned templates because I cleaned up a registry").
- Refuse-without-flag is annoying and inconsistent with `spin remove` which soft-deletes by default.
- Warn-but-keep with stale reference: `spin new --template official/go-api` still works because it's in `pinned.json` -- but `spin search official/go-api` no longer surfaces it.

**Prevention:**
- Default: warn-and-keep. `spin registry remove official` prints `removing registry "official" will leave 3 templates in your pinned list; they will still work offline. Use --purge-pinned to also remove them.`
- Add `--purge-pinned`: walks `pinned.json`, finds rows whose `Source` is in the removed registry, marks them `Removed` (or hard-deletes with `--purge-pinned --purge`).
- NEVER cascade-delete implicitly. The user's pins are theirs.
- The same rule for partial failure: if `os.RemoveAll(registries/official/)` fails midway (permission error), the registry might be half-deleted. The atomic-write discipline for `registries.json` ensures we don't update the index until the disk op succeeds -- but for `os.RemoveAll` we have no rollback. Mitigation: try RemoveAll first; if it fails, leave `registries.json` unchanged and return the error. If it succeeds, then atomic-write `registries.json`. (Order matters.)
- Test: add registry, add two pins from it, remove registry without `--purge-pinned`. After: `registries.json` no longer has `official`; `pinned.json` still has both rows, with `Source` pointing at URLs that are no longer indexed. `spin list` still shows them.

**Warning signs:**
- A user reporting "my pinned templates disappeared."
- `pinned.json` rows referencing aliases that no longer exist in `registries.json`.

**Phase to address:** Phase 6A (manager + CLI).

---

### Pitfall 9: `<alias>/<id>` includes the alias -- but what if the alias itself is gone when `spin new` resolves it?

**What goes wrong:** `pinned.json` carries a `Pinned.Name` that was set from a `<alias>/<id>` template at install time. If the alias is later removed from `registries.json`, the `<alias>` portion of the string still exists in `pinned.json` (it's just a Name, not a live reference). The pin keeps working because the cached clone is still on disk. But the *reverse* lookup -- "I want to know which registry this pin came from" -- is no longer possible.

**Why it happens:** The spec doesn't bind `pinned.json` rows to a registry identity; it binds them to a Source URL.

**Consequences:**
- Confusing UX: `spin list --source-registry` is impossible; the data is gone.
- `spin new --template <pinned>` keeps working (good); `spin add <alias>/<id>` errors because alias is gone (correct); `spin add <pinned>` (using the legacy bare name -- wait, with `<alias>/<id>` format this looks like a registry reference) errors because the alias is gone (also correct, but the error message is the same as for an unknown bare-name pin).

**Prevention:**
- Keep `pinned.json`'s `Name` exactly as it was at install time. Don't rewrite on registry removal. Document the staleness as expected.
- Optionally, add an `OriginRegistry string` field to `Pinned` (forward-compat). Don't make existing readers require it. JSON omitempty keeps old pin files readable.
- The error message for `spin new <alias>/<id>` when alias is gone should be `unknown registry 'foo'` not `unknown template 'foo/bar'` -- alias resolution happens FIRST.

**Warning signs:**
- Spin output that conflates "no such registry" with "no such pin."
- Loss of audit trail: can't tell which pin came from where.

**Phase to address:** Phase 6A (manager validates aliases); Phase 7B (resolver emits the right error). Field addition to Pinned is a follow-up if the audit trail matters.

---

### Pitfall 10: `spin registry update <alias>` partial-state on a slow git fetch

**What goes wrong:** `spin registry update` re-clones (or fetches) each registry's on-disk tree. Reusing `client.Refresh` (line 508-555) re-clones on top of an existing path; the loader's `cloneGit` (line 139-185) also wipes-and-reclones. Half-clones leave `registries/<alias>/` partly old, partly new. If `spin search` runs mid-update, results mix old and new template ids.

**Why it happens:** `git clone` is not transactional at the on-disk-tree level. The `.git/index` and the working tree are mutated separately; an interrupted clone is recoverable in git (because the `.git/` dir keeps metadata) but confusing for a user who doesn't know git.

**Consequences:**
- A mixed-version registry: `templates/foo.toml` is from before; `templates/bar.toml` is from after; the registry author renamed `foo` to `baz` but the local checkout has both. Lookup is ambiguous.
- `registries.json` was already updated with the new metadata hash; rollback by reverting `registries.json` doesn't fix the on-disk tree.

**Prevention:**
- Mirror `refreshOne` (cmd/update.go:106-175): snapshot old to `registries/<alias>.bak-<ts>`, refresh onto the real path, on success delete the .bak. On failure, rename the .bak back. This is the same pattern used for `spin update` of pinned templates; reuse it for consistency.
- `client.Refresh` is suitable for git -- it does wipe-and-reclone. The wrapper logic (snapshot + rollback) needs to live one layer up, in the registry manager.
- For local registries: same -- snapshot old, copy new (or refresh symlink), restore on failure. The cost of an extra copy of a small `registry.toml` is negligible.
- Test: build a mock git server that hangs (or use a known-good URL and a network simulator), kick off `update`, interrupt; verify the old tree is restored.
- Pre-flight: if the registry didn't change on the server (compare HEAD SHA between before and after), skip the swap entirely. Avoids unnecessary churn.

**Warning signs:**
- `templates/` having template tomls from two different commit SHAs.
- `spin search` results changing between back-to-back calls with no intervening update.

**Phase to address:** Phase 6A (manager) -- the rollback wrapper is needed before any `update` works correctly.

---

### Pitfall 11: Network/prompt behavior on `--depth` vs full clone for `update`

**What goes wrong:** `spin registry add` (deep clone) and `spin registry update <alias>` (refresh / pull) both currently do `git clone --depth=1`. A shallow clone doesn't have history; a `git pull` later will still work but produces another shallow clone. After several updates, the on-disk `.git/` is still shallow -- which is fine for our use case (we never read history) but surprises users who `cd ~/.config/spin/registries/official/ && git log` and see only the latest commit.

If we instead switch to a non-shallow clone for `update`, the .git dir grows; we slow down refreshes; we invite users to read sensitive commit messages. Pick one and be consistent.

**Why it happens:** Shallow clones are a UX optimization (fast clone) but commit-history stripping has side effects.

**Prevention:**
- Document the choice (shallow always) in the registry manager's godoc.
- Add a `--full` flag to `spin registry update` for users who want history (uncommon).
- Don't change clone depth between `add` and `update` without a comment explaining why.

**Warning signs:**
- User confused that `git log` shows one commit.
- A consistency check that finds different .git directories across aliases.

**Phase to address:** Phase 6A; document the choice, no code changes beyond documenting.

---

### Pitfall 12: `registries.json` atomic-write discipline + forward-compat with the existing `Removed` pattern

**What goes wrong:** `writePinned` (line 561-595 of client.go) protects `pinned.json` against partial writes by writing to a sibling tmp file, fsync, rename. `registries.json` needs the same discipline. If we reuse the `writePinned` pattern wholesale (MarshalIndent a slice of `Registry` records, atomic rename), that's fine; if we instead write registries.json as a single `map[string]Registry` keyed on alias, we change the on-disk shape. The spec uses "aliases" as user-facing identifiers; the JSON could be either:
- Array of records `[ { alias, source, addedAt, lastUpdated, removed } ]` (parallel to `pinned.json`).
- Map keyed on alias `{"official": {source, addedAt, lastUpdated, removed}}` (parallel to `pinned.json` minus the de-dup keying).

**Why it happens:** Either is fine; the pitfalls come from mixing them or from the second pattern's lack of `Removed` semantics.

**Consequences:**
- If a registry is removed via `spin registry remove` but we keep the record (soft-delete, parallel to `Pinned.Removed`), then `spin registry list --all` shows removed records; the default hides them. Same semantics as `pinned.json`. Document this with `Removed bool` in the registry record.
- Forward-compat: the existing `pinned.json` legacy code (which can read old pin files without `Removed`) tolerates new fields because Go JSON unmarshal ignores unknown fields. Same should hold for `registries.json`.

**Prevention:**
- Use the array-of-records pattern, parallel to `pinned.json`. De-dupe on `Alias`. Set `Removed bool` for soft-delete. MarshalIndent + atomic rename. Tests round-trip the same way `TestClient_Pin_And_List_RoundTrip` tests pinned.json.
- Don't introduce a new field without `omitempty`. Test that an old registry record (no `Removed`) parses into the new struct with `Removed == false`.
- Mirror the structure: `internal/registry/registries.go` has `type Registry struct`, `type File struct { slice of Registry }`, methods `Add`, `Remove`, `Update`, `ListAll`, `ListActive`.
- Test: hand-craft a `registries.json` with one old record (no `Removed` field) and verify `ListActive` finds it.

**Warning signs:**
- A `registries.json` parsed by the new code loses old records.
- A test that writes a record and finds it missing after re-read.
- Concurrent reads of `registries.json` while it's being written (rare but possible).

**Phase to address:** Phase 6A. Parallel structure to pinned.json established up front.

---

### Pitfall 13: The "transient clone" lifecycle -- when does a fetch become a pin?

**What goes wrong:** The spec's "Scaffold Flow" mermaid has `K{Template was fetched from Registry?}` and `M{Pin Template?}`. The fork "yes / no" determines whether the cloned template ends up in `pinned.json` or is `rm -rf`'d. This lifecycle has multiple entry points:
- `spin add <alias>/<id>` -- always pins.
- `spin new <alias>/<id>` -- prompts `Pin?` after success (parallel to existing `promptPinAfterSuccess` in cmd/new.go:251-287).
- `spin registry update <alias>` -- never pins; refreshes the registry tree.

State machine in the cache filesystem:
- **Transient**: `~/.cache/spin/transient/<hash>/` (or `os.MkdirTemp` rooted at `os.TempDir()`); never in `pinned.json`; cleaned up on next `os.RemoveAll` or on process exit.
- **Pinned**: `~/.config/spin/templates/<name>/` (current behavior); in `pinned.json`; survives process exit.
- **Garbage**: half-cloned `transient/<hash>/` whose parent process crashed; cleaned by a sweep on next spin invocation or `os.RemoveAll` best-effort.

**Why it happens:** Without a single source of truth for the lifecycle, the manager has to decide at runtime: do I clone to a temp dir and rename later, or clone to the real dir and `rm -rf` if `M{no}`? Each has tradeoffs.

**Consequences:**
- Clone-to-temp-then-rename: requires double the disk during the clone (temp + real) and atomic rename at the end. Two rename operations to manage on failure (one to put temp into real, one to undo if the pin-write fails).
- Clone-to-real-then-rm-on-no: leaves a half-clone on disk if the user declines to pin; subsequent `spin new <alias>/<id>` would re-use that clone (which might be stale if the registry updated in the meantime).

**Prevention:**
- Define a single, explicit state machine. Recommend: **clone-to-temp-dir-then-rename**. The temp dir can be `os.MkdirTemp(filepath.Join(c.CacheDir, "transient"))`. On the `Pin?` yes: `os.Rename(transient, pinned)`. On no: `os.RemoveAll(transient)`. On process crash between clone and decision: a sweep at next spin startup removes transient dirs older than e.g. 1 hour.
- The clone-then-rm-on-no approach is simpler but risks the user re-running and getting the wrong snapshot. Use the temp-dir approach for safety.
- Document the lifecycle in the manager's godoc.
- Tests: (a) `spin new <alias>/<id>` declining pin results in `pinned.json` unchanged and no clone under `~/.config/spin/templates/`; (b) `spin new <alias>/<id>` accepting pin results in `pinned.json` updated and a clone under `~/.config/spin/templates/<name>/`; (c) process crash mid-decision (simulate by killing the child process): next spin sweep cleans the orphan.

**Warning signs:**
- A `~/.config/spin/templates/` directory that wasn't created by `spin add`.
- A `~/.cache/spin/transient/` with old timestamps.
- Pins that reference templates not in `pinned.json`'s expected schema.

**Phase to address:** Phase 7B (resolver + new flow). The state machine design should be settled in 6A so the manager can build the helpers.

---

### Pitfall 14: Concurrent `spin registry update` + `spin new` across processes

**What goes wrong:** CI runner with two parallel jobs: one runs `spin registry update`; another runs `spin new <alias>/<id>`. Both touch `~/.config/spin/registries/<alias>/` (the update to refresh, the new to read). The new process reads a partial .git checkout; the update's snapshot-rename swaps the working tree mid-read.

**Why it happens:** No file lock; no advisory lock; nothing in the design prevents it.

**Consequences:**
- `spin new` reads a mixed-version registry (one template from old, one from new). Lookup ambiguous.
- `spin new`'s resolution walk over `templates/*.toml` panics on a missing file (`os.ReadDir` returns the dir contents at one instant; subsequent `os.Stat` finds the file gone).
- Atomic-write protects `registries.json` from corruption but not from stale reads.

**Prevention:**
- Use a process-local snapshot of the registry list at the start of resolution (read once, walk once). Don't re-read during the resolution flow.
- If the manager is updated from a different process, the active `spin new` should be allowed to finish with stale data. Document this.
- For long-running CI scripts that pipeline multiple `spin` invocations, recommend `spin registry update && spin ...` rather than concurrent updates.
- An advisory file lock (`flock(2)` via `syscall.Flock`) on `registries.json` is over-engineering for v2.x. Skip until evidence shows it's needed.

**Warning signs:**
- A test that fails under `go test -race` due to concurrent map access on the registry index.
- CI runners showing a non-reproducible "registry corrupt" error.

**Phase to address:** Phase 7B. Document but don't implement a lock in v2.x.

---

### Pitfall 15: Test isolation -- `XDG_CONFIG_HOME` + `os.UserConfigDir` cross-pollution

**What goes wrong:** The existing test pattern (`cmd/remove_list_test.go:17-22`) uses `t.Setenv("XDG_CONFIG_HOME", dir)` to point the registry client at a fresh tempdir. The new `registries.json` reads use the same `c.CacheDir` (line 39 of client.go: `cache, _ := os.UserConfigDir(); cache = filepath.Join(cache, "spin")`). So setting `XDG_CONFIG_HOME` redirects `~/.config/spin/`, which now contains BOTH `pinned.json` and `registries.json` and `registries/`.

Subtle issues:
- `os.MkdirTemp("", "spin-*")` (no `t.TempDir()`) -- if a test writer forgets `t.TempDir()` and uses `os.MkdirTemp("", ...)` instead, the dir leaks across test runs on the same machine. Use `t.TempDir()`.
- Tests that don't set `XDG_CONFIG_HOME` will read the user's REAL `~/.config/spin/registries.json`. CI runs as the user; a leftover `~/.config/spin/registries.json` from a previous run can poison a test. ALWAYS set `XDG_CONFIG_HOME` at the top of every registry-touching test.
- Parallel tests: `t.Parallel()` + `t.Setenv("XDG_CONFIG_HOME", dir)` is not safe -- `Setenv` is process-wide, not goroutine-local. Two parallel tests using different paths would step on each other. Either serialize the registry tests or use a different isolation scheme (e.g., construct `registry.Client` directly with `CacheDir: testDir`, bypassing `New()`'s reliance on env).
- Mid-test `os.Setenv` (not `t.Setenv`) won't be restored. Always `t.Setenv`.

**Why it happens:** Existing code already uses `t.Setenv` for `XDG_CONFIG_HOME` in `remove_list_test.go`. The new tests should follow the same pattern but extend it to cover `registries.json`.

**Consequences:**
- A test that runs in CI with a clean user dir works; a test that runs on a developer's machine that has `~/.config/spin/registries.json` from yesterday's `spin registry add` fails mysteriously.
- Two tests passing under `go test -count=1` but flaking under `go test -race` due to parallel env mutation.

**Prevention:**
- A single test-helper function, `withEmptyConfig(t *testing.T) string`, that:
  - Calls `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`.
  - Creates `<tmpdir>/spin/` so future `os.UserConfigDir()` is well-defined.
  - Returns the resolved `CacheDir`.
- Every test in `internal/registry/manager_test.go` and any `cmd/registry_test.go` MUST call this helper first. No exceptions.
- Construct `*registry.Client` directly with `CacheDir: cache` -- don't use `registry.New()` in tests; tests should pin the cache path.
- Document the test pattern in `internal/registry/testing.go` (a doc-only file), or in `manager.go`'s package comment.
- For tests that need to exercise both `pinned.json` AND `registries.json`, the helpers (`seedPinned`, `seedRegistry`, `readPinned`, `readRegistry`) live in a single `testhelpers_test.go` file. Mirror existing `seedPinned`/`readPinned` in `remove_list_test.go`.
- An integration test that runs `spin registry add` then asserts `~/.config/spin/registries/<alias>/registry.toml` exists -- always with `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` at the top.

**Warning signs:**
- A test that fails only on a developer machine, not in CI.
- `go test -race` flakes that go away with `-count=1`.
- Tests passing in isolation but failing under `t.Parallel()`.

**Phase to address:** Phase 6A. Set the pattern up front so every subsequent phase inherits it.

---

## Integration Gotchas (specific to existing spin code)

### Integration 1: `writePinned` atomic-write discipline must be mirrored for `registries.json`

**Common mistake:** Writing `registries.json` directly with `os.WriteFile`, skipping the temp + fsync + rename pattern.

**Correct approach:** Port `writePinned` (lines 561-595 of `client.go`) to a `writeRegistries` function: MarshalIndent, write to `<path>.tmp-<rand>`, fsync, close, rename over. The temp file pattern survives SIGKILL between write and rename.

### Integration 2: Alias reuse of `<alias>/<id>` vs `Pinned.Name` collision

**Common mistake:** Treating `<alias>/<id>` as just another pinned-name string.

**Correct approach:** Validation in the resolution site (loader.go's `Load` or `cmd/add.go` before calling loader) splits on the first `/`; the left side is checked against `registries.json` before the right side is treated as a template id. A pin lookup with a slash in the name searches the registry path, not the pinned-name map.

### Integration 3: `SanitiseRepoName` is shared between `loader.go` and `client.go` -- it's not safe for alias-as-source

**Common mistake:** Using `SanitiseRepoName` to compute `registries/<alias>/` from a user URL. The function strips the scheme and the path and returns the *basename* -- which for `https://github.com/spin/official.git` is "official." If we use that as both the alias AND the clone directory name, we cannot also support `spin registry add official https://github.com/somebody-else/registry`.

**Correct approach:** Use a separate function `aliasFromURL` (or just take the user's `<alias>` argument as-is, after validation). The alias is user-supplied and distinct from the cache-dir name. The cache-dir name can be `alias` itself (validated, so safe) or a sanitised version if the alias needs to be URL-safe. Prefer alias-as-dir-name (after validation) for 1:1 audit.

### Integration 4: `destExists` / `destReuse` / `destPin` / `destWipe` / `destCancel` constants in `template/loader.go:203-217`

**Common mistake:** Ignoring the existing pattern that already lets a user interactively choose what to do when `cloneGit` finds an existing dest.

**Correct approach:** The registry manager's `add` flow reuses this pattern directly. If `registries/<alias>/` already exists:
- `destReuse`: detect it's already a registry; short-circuit.
- `destWipe`: blow away and re-clone (the current destructive default in `addGit`).
- `destCancel`: error out.
- New `destRefresh`: pull-only via `git fetch`, no checkout reset.

This means the manager and `addGit` don't duplicate the collision-detection logic. The constants stay in `loader.go`; the manager imports `template.DestReuse` etc.

### Integration 5: `client.Refresh` requires a non-empty `LocalPath`

**Common mistake:** Calling `client.Refresh` with a `pin` whose `LocalPath` is empty.

**Correct approach:** Refresh (line 508-555 of client.go) defensively errors with "re-run `spin add <src>`". The manager must always populate `LocalPath` before calling. Since the manager owns `registries/<alias>/`, it has the path; pass it in.

### Integration 6: `Removed` field in `Pinned` -- the soft-delete contract

**Common mistake:** New registry-aware code reads pinned.json and *modifies* `pinned.json` (e.g., writes the `Removed` field for some new semantics) without preserving the existing soft-delete contract.

**Correct approach:** The new code ONLY reads `pinned.json`. Any "registry owns this pin" metadata lives in `registries.json` (parallel structure), NOT in `pinned.json`. If a future field is needed in `pinned.json`, add it with `omitempty` and a JSON compatibility test that an old pin file (no new field) still parses cleanly -- mirror the existing soft-delete invariants.

### Integration 7: `cmd/remove.go` -- the existing `Removed` / `--purge` flow

**Common mistake:** Re-implementing `spin remove`'s soft-delete logic in the registry manager. They look similar (registries also can be "removed").

**Correct approach:** The two are different because they remove *different* things:
- `spin remove <name>` operates on `pinned.json` and `.cache/spin/templates/<name>/`.
- `spin registry remove <alias>` operates on `registries.json` and `.cache/spin/registries/<alias>/`.
They can share a helper if the helper takes the cache dir + list of records + the dirty bit, but for v2.x keep them separate. The shared *concept* is "soft-delete with optional hard-delete via --purge."

### Integration 8: `promptPinAfterSuccess` in `cmd/new.go:251-287` -- the "Pin?" interactive prompt

**Common mistake:** Writing a SECOND "Pin?" prompt for registry-resolved templates, ending up with two flows.

**Correct approach:** Extend `promptPinAfterSuccess` to recognise registry-sourced templates and route to the registry manager's pin path (which may differ from `client.Add` because the source resolution is different). One prompt, two underlying pin flows.

### Integration 9: `isLocalPath` / `isGitURL` / `isShorthand` -- the source-kind detection in `client.go:339-350`

**Common mistake:** Re-implementing these checks in the registry manager with subtly different rules.

**Correct approach:** Make them exported functions (`registry.IsLocalPath`, etc.) so the manager and any new code shares one definition. The current unexported names mean the loader and the client can't easily share.

### Integration 10: `client.Add(spec)` -- local path, git URL, shorthand

**Common mistake:** Calling `client.Add` from the registry manager with `spec = "<alias>/<id>"`. The current `Add` doesn't understand this format.

**Correct approach:** The manager resolves `<alias>/<id>` to a real source URL/path FIRST and then calls `client.Add`. Or, if we add `<alias>/<id>` to `client.Add`, document the precedence there.

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| `spin search` walks `~/.config/spin/registries/<alias>/templates/*.toml` on every call | search latency > 100ms with 50 templates × 5 registries | Build an in-memory index at startup or on `registry update`; mtime-checked cache for reads | At ~20 templates across multiple registries |
| Parsing every `templates/*.toml` on every `spin new` for the "Pin?" prompt | visible lag between `spin new ... --print-params` and prompt | Pre-build the snapshot once per `spin new`, reuse for pin prompt | At ~5 registries × ~20 templates |
| Re-cloning on every `spin registry update` even when HEAD didn't move | slow updates on flaky networks | Capture HEAD SHA after `add`; in `update`, fetch and diff SHA -- skip clone if unchanged | At any scale; mainly user-experience |
| Walking the entire `~/.config/spin/registries/` to list registries on every `spin registry list` | noticeable lag, but lower than search | Only read `registries.json`; it's the source of truth for *which* registries exist; don't walk the disk | Breaks for users with many registries; trivial fix |
| Atomic write of `pinned.json` AND `registries.json` without fsync | Lost changes on power loss | `writeRegistries` mirrors `writePinned` exactly (line 561-595 of client.go) | If a power-loss bug ever shows up |
| Loading the entire `BurntSushi/toml` parser per template | first-call latency | TOML parsing is fast enough; no premature optimization | N/A in practice |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `<alias>` path traversal | User escapes cache root with `../`, deletes arbitrary files | `ValidateAlias` is the gate; `filepath.Rel` asserts the resolved path stays under cache root |
| Registry-supplied `source = "evil"` passed through to `loader.Load` | Arbitrary URL access (incl. `file://`, SSH) | Manager validates `source` against `isLocalPath`/`isGitURL`/`isShorthand` whitelist before delegating; refuse otherwise |
| `source = "file:///etc/passwd"` or `/etc/shadow` | Reads sensitive files via `spin add` | Same whitelist; for local paths, add an opt-in `--allow-local-source` or scoped-to-home policy |
| Untrusted registry on a shared machine could leak via local-symlink | A local registry symlinked to `~/.config/spin/registries/` could read user files via the registry's `source` field | Warn the user the first time a local registry is added; require `--confirm-local-source` if the local source isn't under home (defer to v2.x) |
| Malformed TOML triggers a BurntSushi parser bug (rare but documented) | DoS via parser panic | Wrap parse errors with a friendly message; tested with known-bad inputs |
| Git clone of a hostile URL might attempt to consume github credentials | If `~/.gitconfig` has stored credentials, a hostile URL could leak them | `GIT_TERMINAL_PROMPT=0` (already used); document the implicit trust on `~/.gitconfig` |
| No signature verification on registries | A MITM on the git transport could substitute a malicious registry | Out of scope for v2.x; document; v2.x+ could add `git-crypt` or signed-tag verification |
| `registries.json` symlink attack | A malicious local registry could symlink to a target dir; `spin registry remove` follows the symlink | When deleting, use `os.RemoveAll` after `filepath.EvalSymlinks`; or refuse if the target is a symlink |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| `spin registry add <bad-alias>` errors with a Go panic or unhelpful message | User unsure what `bad-alias` means | `error: alias "foo/bar" must not contain "/"; use a single segment like "official"` |
| `spin registry add official https://...` succeeds but the user's network is down | Half-installed, half-confused | Pre-flight the clone target (DNS only, 2s) before asking the registry's metadata |
| `spin registry remove official` returns "removed" but `spin search official` still works because pins were kept | "I thought I removed it" | Clear message: "removed registry 'official'; 3 templates remain in pinned list, will keep working offline" |
| `spin new --template foo` with `foo` being both a pinned name and a registry alias | Non-deterministic lookup | Single precedence order, documented, tested |
| `spin registry update` with 50 registries is silent for 30s | No progress feedback | Print each registry's name as it's updated; aggregation summary at end |
| `spin search` returning zero results because the alias is wrong | "It's right there!" | Differentiate "unknown alias" from "no templates match" in the error |
| `spin registry add local ~/my-registry` symlink-fallback silently doing nothing on Windows | Edits don't propagate | Detect fallback, warn loudly the FIRST time per session: `note: editing ~/my-registry won't reflect in spin until you re-run 'spin registry update'` |
| `spin registry remove official` removing the alias but leaving stale pins means the user can still `spin new official/foo` if it's pinned | Confusing; the registry is gone but the template still works | Document this as the intended behavior; pin survives by design |
| Long aliases (`spin registry add very-long-organization-name ...`) -- typos aren't caught | User doesn't realise the typo until search returns empty | Alias completion? Fuzzy suggestion? out of scope; provide clear error listing known registries on a fail |

---

## "Looks Done But Isn't" Checklist

- [ ] **`spin registry add` validates the alias BEFORE creating any directory.** Verify by attempting each invalid alias (`..`, `foo/bar`, `.git`, `-foo`) and asserting no `~/.config/spin/registries/<alias>/` directory was created.
- [ ] **Atomic write for `registries.json`.** Test: kill the process between `os.WriteFile(tmp)` and `os.Rename`; next `spin registry list` shows the OLD content, not a corrupt or empty file.
- [ ] **Alias collision refuses (or asks before clobbering).** Test: `add A URL1; add A URL2` -- verify second errors AND `registries.json` only has A→URL1 AND the on-disk clone is URL1's content.
- [ ] **Registry metadata validation with skipped-vs-errored semantics.** Test: registry with one bad `templates/*.toml` and one good one. Verify `spin registry update` reports the bad one (warning) and `spin search` includes the good one. Separately, a *completely* invalid registry (missing `registry.toml`) should be an error.
- [ ] **`<alias>/<id>` resolution precedence is documented and tested.** Test cases for each combination: legacy pin + matching registry template; matching pin + matching registry template; pin only; registry template only; neither.
- [ ] **Remove-without-purge preserves pinned templates.** Test: add registry, pin two templates from it, remove registry without `--purge-pinned`. After: `pinned.json` still has both rows; `spin list` still shows them; `spin new <pinned>` still works.
- [ ] **`spin new` rejects `<alias>/<id>` with `<alias>` not registered.** Error message names the unknown alias, not the template id.
- [ ] **`spin new --template <pinned>` still works after registry removal.** Pins survive `spin registry remove <alias>`. Test by removing the registry and running `spin new --template <old-pinned-name>`.
- [ ] **Test isolation: every registry-touching test sets `XDG_CONFIG_HOME` via `t.Setenv`.** Grep test files for `os.Setenv` (without `t.`) -- any hits are bugs.
- [ ] **Windows symlink fallback is detected and warned.** Test or document: when `os.Symlink` fails on the target, fall back to copy and emit a one-time warning per session.
- [ ] **No new top-level deps.** Confirm by `go mod diff` -- if a new module appears in `go.mod`, it's a v2.x regression.
- [ ] **`go test -race ./...` is clean.** Run locally; specifically tests that touch the registry manager.
- [ ] **`spin new <alias>/<id>` --pin prompt appears once, correctly.** Test: `spin new myapp --template official/go-api` produces a huh form asking to pin; declining leaves `pinned.json` unchanged; accepting adds the pin.
- [ ] **`registries.json` is human-readable.** Inspect by hand: `cat ~/.config/spin/registries.json` -- indented JSON, all fields labelled.
- [ ] **Removed registries leave audit trail.** `spin registry list --all` shows soft-removed records (parallel to `spin list --all`).
- [ ] **Concurrent `spin registry update` and `spin new` does not panic.** A targeted goroutine test (under `-race`) that runs them against the same registry.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Alias path-traversal escape | MEDIUM | `rm -rf` the affected symlinks/files; add `ValidateAlias` invariant; add a test that the resolved path is under `CacheDir` |
| Atomic-write skipped -- `registries.json` corrupt | LOW | Restore from `~/.cache/spin/` if backed up; otherwise user re-adds registries; new code with `writeRegistries` prevents reoccurrence |
| `<alias>/<id>` resolution precedence ambiguous in code | LOW-MEDIUM | Identify the precedence site; document; add a single test that asserts the order; refactor if the order is wrong in current behavior |
| Registry with 100% invalid metadata | LOW | `rm -rf registries/<alias>/`; `rm` the entry from `registries.json`; add metadata validation in CI |
| Remove cascade-deleted a pin the user wanted | MEDIUM | Re-add the pin (`spin add <source>`); data on disk may still exist under `~/.config/spin/templates/<name>/` if pin was a soft-delete, lost if hard-delete. Document this; default to soft-delete-only |
| Test pollution between runs | LOW | Add `withEmptyConfig(t)` helper that ALL tests use; document; clean `~/.config/spin/` in test fixture |
| Symlink fallback not detected on Windows | MEDIUM | First-time warning; user runs `spin registry update local` to refresh; document |
| Pin metadata drift (pin Source no longer in any registry) | LOW | Pin survives; if the source URL no longer exists, the clone survives for offline use; document this as a feature |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|-------------|
| Alias path separators, dot-files, NUL | Phase 6A (manager + CLI) | `withEmptyConfig(t)` + table-driven `TestValidateAlias` covering `..`, `/`, `\\`, `.`, NUL, leading `-`; verify NO directory created on failure |
| Alias collides with `Pinned.Name` keyspace | Phase 7B (resolver) | Test fixture: pin + matching registry template; assert documented precedence |
| Alias collision between two `add`s of same alias | Phase 6A (manager + CLI) | `TestAdd_AliasCollision_Refuses` test; verify second `add` errors without touching the first |
| `git clone` half-state + missing `registry.toml` | Phase 6A (manager + CLI) | Inject failing git URL; verify `registries/<alias>/` is empty (not half-cloned) after error |
| Registry metadata validation | Phase 7B (index reader + resolver) | Fixture with 3 valid + 2 invalid templates; verify `spin registry update` reports warnings, `spin search` includes only valid |
| `<alias>/<id>` resolution race (file changes during resolve) | Phase 7B (resolver) | Snapshot-test; goroutine test (`-race`) |
| Symlink vs copy semantics | Phase 6A (manager + CLI) | On Linux: symlink path; on Windows-symlink-disabled: copy + warn test |
| Remove with pinned templates | Phase 6A (manager + CLI) | Test: pin 2 from registry; remove without `--purge-pinned`; verify pins survive |
| Pin survival across registry removal | Phase 6A + 7B | `TestSpin_NewPinnedWorksAfterRegistryRemove` |
| `spin registry update` partial state | Phase 6A (manager + CLI) | Test: snapshot-rename wrapper rejects bad clones; rollback restores old |
| Shallow vs full clone consistency | Phase 6A (manager + CLI) | Document in godoc |
| `registries.json` atomic write + forward compat | Phase 6A (manager + CLI) | `TestRegistries_RoundTrip` parallel to `TestClient_Pin_And_List_RoundTrip`; verify hand-crafted legacy JSON parses |
| Transient clone lifecycle | Phase 7B (resolver) | `TestNew_TransientCloneLifecycle`: pin=no leaves no clone, pin=yes adds one |
| Concurrent `spin registry update` + `spin new` | Phase 7B (resolver) | Goroutine test under `-race` |
| Test isolation (XDG_CONFIG_HOME) | Phase 6A | Helper `withEmptyConfig(t)` enforced; CI grep for `os.Setenv` returns no hits in registry-touching tests |

---

## Sources

### Read directly (HIGH confidence)

- `/home/samouly/Projects/Golang/spin/spin-registry.md` -- the spec being implemented
- `/home/samouly/Projects/Golang/spin/internal/template/loader.go` -- existing cloneGit, destExists, destReuse/destPin/destWipe/destCancel constants, PromptExistingDest hook contract
- `/home/samouly/Projects/Golang/spin/internal/registry/client.go` -- addGit with `os.RemoveAll(dest)` precondition; addLocal with symlink-then-copy fallback; SanitiseRepoName; Refresh with non-empty LocalPath; writePinned atomic-write pattern; isLocalPath/isGitURL/isShorthand classification; ErrNotDeployed (to be removed in v2.x)
- `/home/samouly/Projects/Golang/spin/internal/registry/types.go` -- Pinned struct + the `Removed bool` soft-delete field (omitempty preserves forward compat)
- `/home/samouly/Projects/Golang/spin/cmd/remove.go` -- the soft-delete + --purge two-step flow to model registries after
- `/home/samouly/Projects/Golang/spin/cmd/add.go` -- PinnedAt field setting; Add/Pin sequence that the manager must mirror
- `/home/samouly/Projects/Golang/spin/cmd/update.go:106-175` -- refreshOne: snapshot-old, refresh, atomic-rename, on-failure-rollback pattern to reuse for `spin registry update`
- `/home/samouly/Projects/Golang/spin/cmd/new.go:251-287` -- promptPinAfterSuccess: the existing "Pin?" prompt the registry flow should extend
- `/home/samouly/Projects/Golang/spin/cmd/init.go:185-193` -- validateTemplateName: existing alias/name validation pattern with strings.ContainsAny for separator rejection
- `/home/samouly/Projects/Golang/spin/cmd/remove_list_test.go:17-58` -- withEmptyPinned/seedPinned/readPinned helpers + XDG_CONFIG_HOME test isolation pattern
- `/home/samouly/Projects/Golang/spin/internal/registry/client_test.go` -- TestClient_Pin_DeDupeByName, TestClient_Add_LocalPath, TestErrNotImplementedAlias: pin-handling tests to mirror for registries
- `/home/samouly/Projects/Golang/spin/.planning/PROJECT.md` -- v2.x milestone description; "Phases 6/7/8 (A/B/C)" structure; "pinned.json format unchanged" constraint

### Inferred (MEDIUM confidence)

- New failure modes not covered in existing code:
  - Path traversal via alias (no existing alias/path validation for registries)
  - Soft-delete + `<alias>/<id>` shorthand interaction
  - Transient clone state machine across two turns of interactivity
- The `SanitiseRepoName` function returning the *basename* (not the alias) means it can't be reused for alias computation -- sourced from the function's docstring and behavior, but the consequence (alias-as-separate-field) is an inference.
- Pitfall 14 (cross-process concurrent registry update + spin new): the lack of a file lock is an inference from the absence of one in the existing `writePinned` design. Documented as "document but don't implement."

### Unverified gaps to address in phase-specific research

- The exact `BurntSushi/toml` error format for malformed input (no parsing-time issues observed in current code)
- Whether `os.MkdirTemp` under a custom dir on Windows returns a path with a UNC prefix or a drive-letter prefix (could affect the `filepath.Rel` invariant test)
- The exact behaviour of `os.UserConfigDir` when `XDG_CONFIG_HOME` is set on Windows (does it honour `%AppData%` or the Linux-style XDG? -- this affects the test isolation helper)
- The current `spin doctor` output (if any) for "your registry is corrupted" diagnostic -- may want to add one in v2.x but defer
- Whether the existing `tryAutoPin` (cmd/init.go:175-178) should auto-pin when a *registry*-sourced template is used in `init` -- defer

---
