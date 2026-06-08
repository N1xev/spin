---
phase: 05-v2-0-universal-scaffolder-task-runner
plan: 02
subsystem: ecosystem-dispatch + template-loader
tags: [ecosystem, dispatch, template-loader, v2, backward-compat, deprecation]

# Dependency graph
requires:
  - phase: 05-v2-0-skeleton
    provides: "internal/ecosystem/ interfaces + Registry, internal/template/ skeleton (loader, template, spin_toml, form, engine), cmd/new.go legacy shim, cmd/new_charm.go + cmd/new_extras.go v2 dispatch, defaultRegistry() seeded with charm"
  - phase: 05-01
    provides: "internal/ecosystems/rust/ (proof of universality); defaultRegistry() now seeds both charm and rust; cmd/new_rust.go"
provides:
  - "`spin new <name>` (no ecosystem) prints a one-time deprecation notice and routes to charm with output identical to the v1 path (BC-02 + ECO-08)"
  - "`spin new <eco> <name>` dispatches to the right ecosystem; unknown ecosystem returns a clear error listing known ones (ECO-07 + ECO-09)"
  - "`spin new <name> --template <user/repo>` (v1 form) bridges onto the v2 charm flow with the external template loader"
  - "`spin new charm <name> --template <user/repo>` clones a git repo, parses spin.toml, runs the huh form (or applies defaults in non-TTY), renders, runs post-hooks, deletes spin.toml (TPL-12, TPL-15, TPL-16, TPL-17)"
  - "Template loader uses ~/.config/spin/templates/ as cache dir and sets GIT_TERMINAL_PROMPT=0 to avoid blocking on credentials"
affects: [phase-05-plan-03, phase-05-plan-04, phase-05-plan-05, future ecosystem packages that need template overlays]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "runNew dispatch: 0 args -> legacy; 1 unknown + 1 arg -> legacy + deprecation notice; 1 known -> v2 dispatch; 1 unknown + >=2 args -> error listing ecosystems"
    - "Per-process one-time deprecation notice (deprecationPrinted bool guard) — not per-invocation spam"
    - "Template loader's NewCacheDir resolves to XDG config dir via os.UserConfigDir() (was ~/.cache/spin/templates)"
    - "ResolveForm applies defaults FIRST then user-supplied values, so explicit CLI flags win over the template's own defaults"
    - "params.Value is unwrapped to raw Go primitives (string/int/bool/[]string) before text/template rendering, so `{{.project_name}}` interpolates as the name, not the Value struct dump"
    - "Test files in package ecosystem use inline stubEco (not imported from internal/ecosystems/*) to avoid the import cycle that those concrete ecosystems would create"
    - "Spin.toml is removed from the rendered output via a defensive filepath.Walk (catches both top-level and nested spin.toml)"

key-files:
  created:
    - internal/template/post_hook.go
    - internal/template/template_test.go
    - internal/ecosystem/registry_test.go
    - cmd/new_test.go
  modified:
    - cmd/new.go (deprecation notice + dispatchV2 + looksLikeV2Template)
    - cmd/new_charm.go (--template v2 flow with loader, merge, post-hook)
    - cmd/new_extras.go (legacy --template bridge onto runNewCharm)
    - internal/template/loader.go (XDG cache, GIT_TERMINAL_PROMPT=0, Lister, Clear)
    - internal/template/template.go (RenderToWithPost + deleteSpinToml)
    - internal/template/form.go (apply defaults before user values, UnwrapValue exported)
    - internal/template/engine.go (WriteFiles exported)

key-decisions:
  - "Deprecation notice is per-process (not per-invocation): deprecationPrinted bool guard, printed once per process via os.Stderr"
  - "First positional arg is interpreted as an ecosystem name when it matches defaultRegistry().Names() case-insensitively; otherwise it is treated as the v1.0 project name"
  - "When the first positional is UNKNOWN AND len(args) >= 2, return a clear error listing available ecosystems; do not silently fall back to legacy"
  - "cobra.MaximumNArgs relaxed from 1 to 2 to allow `spin new <eco> <name>` past the args check; runNew does the real validation"
  - "Template loader cache moved from ~/.cache/spin/templates to ~/.config/spin/templates (XDG_CONFIG_HOME; matches XDG Base Directory spec; keeps pinned templates near other config)"
  - "cloneGit uses os.Environ() + append(GIT_TERMINAL_PROMPT=0) so missing creds never block the scaffolder with a password prompt"
  - "ResolveForm reorders: defaults applied FIRST, then user-supplied values. This fixes a bug where the default `\"<nil>\"` (asString(nil) -> \"<nil>\") was overwriting user values"
  - "params.Value wrappers are unwrapped to raw primitives before text/template rendering, otherwise `{{.project_name}}` renders the {String: ... Int: 0 ...} struct dump"
  - "TPL-16 (delete spin.toml) implemented as a defensive filepath.Walk in deleteSpinToml, so it catches nested spin.toml in subdirs (e.g. a template that accidentally includes a spin.toml in _base/)"
  - "runNewCharm's --template branch uses the charm ecosystem's own `template` flag (no duplicate cobra flag binding), distinguishing v2 git specs from v1 bundled variant names via looksLikeV2Template"
  - "Legacy `spin new <name> --template <ref>` bridges to runNewCharm via a fresh cobra.Command (charm flags bound + user's Changed flag values copied over), avoiding a fresh parse round-trip"

requirements-completed: [BC-01, BC-02, BC-03, TPL-12, TPL-13, TPL-15, TPL-16, TPL-17, TPL-18, ECO-07, ECO-08, ECO-09, ECO-10]

# Metrics
duration: 25min
completed: 2026-06-08
---

# Phase 5 Plan 2: Ecosystem Dispatch + Template Loader Summary

**Wires the v2.0 ecosystem dispatch and external template loader end-to-end: `spin new <name>` prints a one-time deprecation notice and routes to charm; `spin new <ecosystem> <name>` dispatches to the right ecosystem; `spin new <name> --template <user/repo>` clones a git repo, parses spin.toml, runs the huh form, renders, runs post-hooks, and deletes spin.toml from the output.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-06-08T21:01:07Z
- **Completed:** 2026-06-08T21:26:12Z
- **Tasks:** 3
- **Files modified:** 11 (3 created, 4 modified new files, 4 modified existing files)

## Accomplishments

### Task 1 — One-time deprecation notice + ecosystem dispatch (cmd/new.go)

- `cmd/new.go` refactored: `runNew` now dispatches based on the first positional:
  - 0 args → legacy path with deprecation notice
  - 1 known ecosystem → v2 dispatch (charm/rust)
  - 1 unknown + 1 arg → legacy path with deprecation notice
  - 1 unknown + ≥2 args → clear error listing available ecosystems
- `dispatchV2(args, cmd)` helper: looks up ecosystem, collects cmd flags, builds `ecosystem.Context`, runs `Validate → Render → PostScaffold`
- `printDeprecationNotice()` is rate-limited per process via `deprecationPrinted` bool
- `cobra.MaximumNArgs` relaxed from 1 to 2 so the v2 form passes the args check
- `cmd/new_test.go` (new): TestPrintDeprecationNotice_OncePerProcess + TestIsKnownEcosystem

### Task 2 — Template loader upgrade + v2 dispatch end-to-end

- `internal/template/loader.go`:
  - `defaultCacheDir()` → `~/.config/spin/templates/` via `os.UserConfigDir()`
  - `cloneGit()` → `cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")` (preserves parent env)
  - New `Lister()` and `Clear(ref)` helpers for tests
- `internal/template/post_hook.go` (new):
  - `RunPostHook(t, values, dir)` renders `[post].run` as a text/template, runs via `sh -c` in dir
  - Unwraps `params.Value` wrappers so the template sees raw Go primitives
- `internal/template/template.go`:
  - `RenderToWithPost(dest, values)`: writes file map → runs post-hook → deletes spin.toml (defensive walk catches nested copies)
- `internal/template/form.go`:
  - `ResolveForm` reorders: defaults first, then user-supplied values (fixes the `<nil>` interpolation bug)
  - `UnwrapValue` exported for `post_hook.go` reuse
- `internal/template/engine.go`:
  - `WriteFiles` exported so `cmd/new_charm.go` can write the merged ecosystem+template file map
- `cmd/new_charm.go`:
  - When `--template` value looks like a v2 git spec (URL or user/repo), the loader takes over: clone, parse spin.toml, huh form, merge with ecosystem files via `mergeMaps`, write via `template.WriteFiles`, run post-hook
  - `isTerminalCmd()` helper (mattn/go-isatty) for huh form gating
- `cmd/new_extras.go`:
  - `PreRunE` bridges the v1 form `spin new <name> --template <ref>` onto `runNewCharm`
  - `dispatchNewCharmWithTemplate` builds a fresh cobra command with charm flags bound + user's `Changed` flag values copied over
  - `hasNewSubcommand` distinguishes `spin new charm ...` (which already handles --template) from the bare form (which needs the bridge)
- `internal/template/template_test.go` (new): 4 tests covering spin.toml deletion, nested spin.toml, UnwrapValue (5 sub-cases), XDG cache dir

### Task 3 — Registry unit tests + single-source-of-truth verification

- `internal/ecosystem/registry_test.go` (new): 6 tests using an inline `stubEco` (NO import from concrete ecosystem packages to avoid the cycle)
  - TestRegistry_Get_UnknownEcosystem
  - TestRegistry_Get_KnownEcosystem
  - TestRegistry_Names_StableOrder
  - TestRegistry_Detect_MarkerBased (uses two stubs with file markers)
  - TestRegistry_Detect_NoMatch
  - TestRegistry_All
- `grep -rn "ecosystem.NewRegistry" --include="*.go" .` shows exactly one call site (`cmd/ecosystem.go`'s `defaultRegistry`)

## Task Commits

Each task was committed atomically:

1. **Task 1: One-time deprecation notice + ecosystem dispatch** — `1d1fe82` (feat)
2. **Task 2: Template loader upgrade + v2 dispatch end-to-end** — `549ec71` (feat)
3. **Task 3: Registry unit tests + defaultRegistry single source of truth** — `64d0e8e` (test)

## Files Created/Modified

### Created

- `cmd/new_test.go` — deprecation helper + ecosystem lookup tests
- `internal/template/post_hook.go` — `RunPostHook` + value unwrapping
- `internal/template/template_test.go` — RenderToWithPost + UnwrapValue + cache dir tests
- `internal/ecosystem/registry_test.go` — 6 registry tests with inline `stubEco`

### Modified

- `cmd/new.go` — added `deprecationPrinted`, `printDeprecationNotice()`, `isKnownEcosystem()`, `looksLikeV2Template()`, `dispatchV2()`; refactored `runNew`; relaxed cobra MaxNArgs to 2
- `cmd/new_charm.go` — added --template v2 flow (loader, merge, post-hook); `mergeMaps()` and `isTerminalCmd()` helpers
- `cmd/new_extras.go` — PreRunE bridges v1 --template form onto v2 dispatch via fresh cobra command; `hasNewSubcommand`, `dispatchNewCharmWithTemplate`, `boolToString`, `charmFlagsForDispatch`
- `internal/template/loader.go` — XDG cache dir; `GIT_TERMINAL_PROMPT=0` in env; `Lister()`, `Clear(ref)`
- `internal/template/template.go` — `RenderToWithPost` + `deleteSpinToml` (defensive walk)
- `internal/template/form.go` — reordered default/apply, exported `UnwrapValue`
- `internal/template/engine.go` — exported `WriteFiles` (delegates to existing `writeFiles`)

## Decisions Made

### Ecosystem dispatch routing

The first positional arg drives the dispatch:
- Known ecosystem name (case-insensitive match against `defaultRegistry().Names()`) → v2 dispatch
- Unknown name with ≥2 args → clear error listing available ecosystems
- Otherwise → legacy path with one-time deprecation notice

This shape keeps the v1.0 single-arg form working AND adds the v2.0 two-arg form. Both `spin new charm demo` and `spin new demo --tui --bubbletea` produce the same output (per BC-01 / BC-02).

### Template loader cache location

Moved from `~/.cache/spin/templates/` to `~/.config/spin/templates/` per XDG Base Directory spec. `os.UserConfigDir()` resolves correctly on Linux/macOS/Windows; falls back to `$HOME/.config/spin/templates` on error. Templates are config-ish (the user pinned them), not cache-ish (they shouldn't be wiped by `tmpreaper`).

### Template form value resolution order

`ResolveForm` now applies defaults FIRST, then user-supplied values. The previous order (user values, then defaults) had a subtle bug: `asString(nil)` returned the literal string `"<nil>"` which `SetDefaults` then treated as a real default and overwrote the user value. The new order ensures explicit CLI flags always win, regardless of whether the template declared a default.

### params.Value unwrapping for text/template

`params.Value` is a multi-field struct `{String, Int, Bool, List, Path}` (one populated at a time). Passing it directly to `text/template.Execute` makes `{{.project_name}}` render as `{<nil> 0 false [] }`. `UnwrapValue` extracts the populated field; the post-hook and form both call it before rendering.

### TPL-16: defensive walk for spin.toml deletion

`deleteSpinToml(dest)` is a `filepath.Walk` that removes every `spin.toml` in the output tree, not just the top-level one. This catches the edge case where a template accidentally includes a `spin.toml` in `_base/` (e.g. via copy-raw-files) — the spec is "spin.toml is deleted from the output", and the walk enforces that.

### Test-time stub approach (Task 3)

`internal/ecosystem/registry_test.go` defines a `stubEco` type INLINE rather than importing the concrete `internal/ecosystems/{charm,rust}` packages. Reason: those packages import `internal/ecosystem` (the package under test), so importing them back would create a compile-time import cycle. The stub is small (10 methods) and a `var _ Ecosystem = (*stubEco)(nil)` compile-time assertion guards against future interface changes.

### --template flag binding on charm subcommand

The plan's step 4 asked for a NEW cobra `--template` flag on `newCharmCmd`. This conflicted with the charm ecosystem's own `template` flag (bundled variant name). The fix: the charm ecosystem's existing `template` flag is reused, and the value is interpreted as a v2 git spec when it looks like one (`looksLikeV2Template` in `cmd/new.go`). The v1 default `"tui-bubbletea"` doesn't contain `/`, so the heuristic is unambiguous.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Blocking] ResolveForm ordering overwrote user values with literal "<nil>"**
- **Found during:** Task 2 (verification)
- **Issue:** `spin new charm tproj --template /tmp/test-template ...` rendered `{{.project_name}}` as `<nil>` and the post-hook failed. The root cause: `asString(nil)` in `internal/params/parse.go` returns the literal string `"<nil>"` (via `fmt.Sprint(nil)`). `SetDefaults` then treated `"<nil>"` as a real default and overwrote the user-supplied "tproj".
- **Fix:** Reordered `ResolveForm` to apply defaults FIRST, then user-supplied values. Explicit CLI flags now win over the template's own defaults.
- **Files modified:** `internal/template/form.go`
- **Verification:** `{{.project_name}}` now interpolates to "tproj"; the post-hook runs successfully.
- **Committed in:** `549ec71` (part of Task 2 commit)

**2. [Rule 2 - Missing critical] params.Value struct dump in template output**
- **Found during:** Task 2 (verification)
- **Issue:** After fixing the `<nil>` issue, `{{.project_name}}` rendered as `{<nil> 0 false [] }` — the literal Go struct dump of `params.Value`. text/template formats struct values via their String() method, which is the zero-value representation.
- **Fix:** Added `UnwrapValue(v params.Value) any` in `internal/template/form.go` that extracts the populated field. Called from `ResolveForm` and `RunPostHook` before passing values to text/template.
- **Files modified:** `internal/template/form.go`, `internal/template/post_hook.go`
- **Verification:** `{{.project_name}}` interpolates to the actual string; post-hook renders correctly.
- **Committed in:** `549ec71` (part of Task 2 commit)

**3. [Rule 2 - Missing critical] Pre-existing template/cmd files uncommitted from skeleton**
- **Found during:** Task 2 (pre-commit check)
- **Issue:** `cmd/new_charm.go`, `cmd/new_extras.go`, and the entire `internal/template/` package were untracked (committed as part of the v2.0 skeleton on 2026-06-08, but not in any later commit because no task touched them). The plan's task 2 needed to modify them.
- **Fix:** The Task 2 commit `549ec71` includes the new file contents for these skeleton files alongside the modifications. This is correct per the plan (`git add` lists them as part of the task).
- **Files added (untracked → tracked):** `cmd/new_charm.go`, `cmd/new_extras.go`, `internal/template/{loader,template,spin_toml,form,engine,parse}.go`
- **Committed in:** `549ec71` (part of Task 2 commit)

**4. [Rule 2 - Missing critical] cmd/new_charm.go added duplicate --template flag**
- **Found during:** Task 2 (build)
- **Issue:** Following the plan's step 4 ("Add a new flag binding: --template on newCharmCmd") verbatim caused a duplicate-flag panic: the charm ecosystem already declares its own `template` flag (bundled variant name). Cobra refused to register a second one.
- **Fix:** Removed the redundant binding. The charm ecosystem's existing `template` flag is reused; the value is interpreted as a v2 git spec via `looksLikeV2Template`. v1 default `"tui-bubbletea"` has no `/`, so the heuristic is unambiguous.
- **Files modified:** `cmd/new_charm.go`
- **Verification:** `spin new charm demo --tui --bubbletea ...` works; `spin new charm demo --template /tmp/test-template ...` invokes the v2 loader; v1 default of `tui-bubbletea` still flows to the legacy template engine.
- **Committed in:** `549ec71` (part of Task 2 commit)

**5. [Rule 1 - Blocking] cmd/new_extras.go dispatch prepended "charm" to args**
- **Found during:** Task 2 (verification)
- **Issue:** The PreRunE bridge called `dispatchNewCharmWithTemplate(newArgs, ...)` where `newArgs = ["charm", "spin-v1-tpl"]`. Inside, `runNewCharm(cmd, newArgs)` then read `args[0]` as `ctx.Name` — but the v2 form has `args[0]="charm"` (the ecosystem) and `args[1]="spin-v1-tpl"` (the name). The fix was to pass the original args (project name only) to `runNewCharm`, since `runNewCharm` is a subcommand with `ExactArgs(1)` and expects args[0] to be the name.
- **Fix:** Pass `args` (not `newArgs`) into `dispatchNewCharmWithTemplate`. The "charm" prefix is only needed for `dispatchV2` (the v2 ecosystem dispatcher), not for `runNewCharm`.
- **Files modified:** `cmd/new_extras.go`
- **Verification:** `spin new spin-v1-tpl --template /tmp/test-template --tui --bubbletea ...` now produces `./spin-v1-tpl/` with the template rendered into it.
- **Committed in:** `549ec71` (part of Task 2 commit)

**6. [Rule 1 - Blocking] Flag copy skipped flags already bound on cmd**
- **Found during:** Task 2 (verification)
- **Issue:** `dispatchNewCharmWithTemplate` bound the charm ecosystem's flags (including `tui`, `bubbletea`) and then iterated the parent's flags, skipping any already on cmd. This meant the user's `--tui --bubbletea` were never copied — the charm's defaults (false) won, and the charm ecosystem's Validate returned "type=tui requires --bubbletea".
- **Fix:** For each parent flag that the user actually changed (`f.Changed`), `Set()` the value on cmd. This OVERRIDES the default with the user's value, even for flags that exist on both.
- **Files modified:** `cmd/new_extras.go`
- **Verification:** `spin new spin-v1-tpl --template /tmp/test-template --tui --bubbletea ...` now reaches the post-hook and renders correctly.
- **Committed in:** `549ec71` (part of Task 2 commit)

---

**Total deviations:** 6 auto-fixed (2 bug, 4 missing-critical; all from Task 2)
**Impact on plan:** All auto-fixes essential for the v2 template-on-charm flow to work end-to-end. The plan's step 4 (add --template flag to newCharmCmd) was a misreading of the existing charm ecosystem's flag surface; the fix reuses the existing flag with a heuristic. No scope creep.

## Issues Encountered

### Pre-existing test flake (unrelated)

`go test ./internal/wrap/...` `TestRun_WithAirToml` times out (660s). This is documented in STATE.md (line 178-181) as a pre-existing environment issue with the `air` binary in the test runner, NOT introduced by this plan. All other tests in the repository pass cleanly.

### Cwd-reset between Bash tool calls

The shell `cd` does not persist between Bash tool calls. Verification commands needed to be self-contained: each `rm -rf /tmp/<name>` was followed by a fresh `cd /tmp && /tmp/spin new <name> ...` in the same command.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The v2 ecosystem dispatch is end-to-end: `spin new charm <name>`, `spin new rust <name>`, and the legacy `spin new <name>` (with deprecation notice) all work.
- The external template loader is wired into `spin new charm <name> --template <ref>` and the v1 form `spin new <name> --template <ref>`.
- `spin.toml` is removed from the output directory (TPL-16 satisfied).
- The loader cache is at `~/.config/spin/templates/` per XDG spec.
- 6 registry tests + 1 deprecation test + 1 isKnownEcosystem test + 4 template tests all pass.
- The runner source-precedence chain (Plan 04's job) can now consume the cargo + go fallback tasks returned by `Ecosystem.Tasks()`.
- The post-hook pipeline is in place; Plan 05 (registry) can layer on top.

---

*Phase: 05-v2-0-universal-scaffolder-task-runner*
*Completed: 2026-06-08*

## Self-Check: PASSED

All 4 verification points:
- SUMMARY.md exists at the expected path
- Task 1 commit `1d1fe82` exists in git log
- Task 2 commit `549ec71` exists in git log
- Task 3 commit `64d0e8e` exists in git log
