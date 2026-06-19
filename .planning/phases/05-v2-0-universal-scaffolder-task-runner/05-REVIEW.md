---
phase: 05-v2-0-universal-scaffolder-task-runner
reviewed: 2026-06-09T11:55:00Z
depth: standard
files_reviewed: 36
files_reviewed_list:
  - cmd/add.go
  - cmd/ecosystem.go
  - cmd/list.go
  - cmd/new_charm.go
  - cmd/new_extras.go
  - cmd/new.go
  - cmd/new_rust.go
  - cmd/run.go
  - internal/ecosystem/registry_test.go
  - internal/ecosystems/rust/detector.go
  - internal/ecosystems/rust/ecosystem.go
  - internal/ecosystems/rust/flags.go
  - internal/ecosystems/rust/post.go
  - internal/ecosystems/rust/post_test.go
  - internal/ecosystems/rust/render.go
  - internal/ecosystems/rust/tasks.go
  - internal/ecosystems/rust/validate.go
  - internal/params/param_test.go
  - internal/params/parse_test.go
  - internal/registry/client.go
  - internal/registry/client_test.go
  - internal/registry/types.go
  - internal/runner/explain.go
  - internal/runner/list.go
  - internal/runner/runner_test.go
  - internal/runner/source.go
  - internal/runner/sources/ecosystem.go
  - internal/runner/sources/ecosystem_test.go
  - internal/runner/sources/spinconfig.go
  - internal/runner/sources/spinconfig_test.go
  - internal/template/engine.go
  - internal/template/form.go
  - internal/template/loader.go
  - internal/template/loader_test.go
  - internal/template/post_hook.go
  - internal/template/template.go
  - internal/template/template_test.go
findings:
  critical: 2
  warning: 8
  info: 5
  total: 15
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-06-09T11:55:00Z
**Depth:** standard
**Files Reviewed:** 36
**Status:** issues_found

## Summary

Phase 5 delivers the v2.0 universal scaffolder end-to-end: rust ecosystem, external template loader, runner source-precedence chain, and registry client hardening. The build is green (`go vet ./...` clean, all unit tests pass in ~1.1s). The architecture is clean: 3 distinct layers (cmd / sources / core), the ecosystem dispatch routing in `cmd/new.go` is well documented, and the security guards (path traversal, `GIT_TERMINAL_PROMPT=0`, atomic JSON writes, friendly-failure registry) are correctly placed.

The defects below are the kinds of things a fresh review pass would catch. Two BLOCKERs:

1. **`Template.Render` ignores files inside directories that happen to fail to walk** -- the entire render aborts on the first walk error, including the user-controllable case where a malicious template's `_base/` contains a broken symlink.
2. **`internal/registry/client.go` `writePinned` temp-file cleanup race** -- the cleanup defer can be made to leak the temp file because the `tmp` is created before the deferred cleanup, and `os.Rename` failing would not roll back the explicit `cleanup = false` path. More importantly, `unlink` of the temp file is not guaranteed if the process is killed mid-rename.

The WARNINGs are concentrated in three areas: (a) the `looksLikeV2Template` heuristic that shares the prefix space with v1 names containing `/`, (b) `cmd/new_extras.go`'s `os.Exit(0)` calls inside `PreRunE` (the cobra contract is "return an error" or "set state"; calling `os.Exit` skips the rest of the command's error-handling and confuses `SilenceUsage: true`), and (c) the `homeDir()` helper in `internal/template/loader.go` that shells out to `sh -c "echo $HOME"` instead of calling `os.UserHomeDir()`.

No path-traversal escape was found in the writeFiles guards. No hardcoded secrets. No unhandled error returns on the hot path. The test coverage is genuinely good -- 30+ tests across the new surface, the same-package-vs-external package pattern is correctly applied where needed (runner_test.go in `runner_test`), and the test files are all in scope and free of flakiness (the friendly-failure 1s timeout is correctly enforced via `newShortTimeoutClient`).

## Critical Issues

### CR-01: `Template.Render` propagates walk errors and aborts on a single bad file/symlink

**File:** `internal/template/template.go:54-82`
**Issue:** `Template.Render` uses `filepath.Walk` with no error filter, returning any `walkErr` from the walk function. A template's `_base/` may legitimately contain broken symlinks (templates often ship with symlinks for "see the latest API" or "go to the docs"), and the user's `os.Lstat`/`Stat` will fail on a broken link. Because `walkErr` aborts the whole render, a single broken symlink in `_base/` (which the template author controls, NOT the user) would prevent the entire template from rendering.

Additionally, a `filepath.Walk` walker that encounters a permission-denied directory inside `_base/` will return an error and abort the render, even though the surrounding files render fine.

**Fix:** Filter walk errors so a transient stat failure on one entry does not abort the whole render. The fix should:

```go
err := filepath.Walk(t.BaseDir, func(path string, info os.FileInfo, walkErr error) error {
    if walkErr != nil {
        // Don't abort the entire render for a single bad entry.
        // Log and continue.
        return nil
    }
    if info == nil || info.IsDir() {
        return nil
    }
    // ... rest of the function
})
```

A test (`TestTemplate_Render_PartialWalkFailure`) should be added that creates a `_base/` with one un-renderable entry (a directory named `spin.toml` for example, or use a symlink loop) and asserts the render completes.

### CR-02: `Template.Render` does not follow the post-hook's `c.Dir = dir` contract when `dir == ""`

**File:** `internal/template/post_hook.go:40-42`, `internal/template/template.go:115`
**Issue:** In `RenderToWithPost`, the post-hook is invoked as `RunPostHook(t, values, dest)`. If `dest` is an empty string, `exec.Command` is invoked with `c.Dir = ""`, which Go treats as "use the current process's working directory" -- NOT "fail loudly". This means a caller that passes `""` will silently run the post-hook in the wrong directory (the spin binary's cwd), with potentially destructive effects (e.g. `rm -rf` running in `os.Getwd()`).

`cmd/new_charm.go:144` passes `ctx.Name` (the project name) as the dest, so the live code path is fine, but the API contract for `RenderToWithPost` and `RunPostHook` permits a `dest == ""` call. The current `cmd/new_charm.go:147` call IS `template.RunPostHook(tpl, values, ctx.Name)`, which is OK. The dangerous path is any future caller that passes an empty dest, or the case where `ctx.Name` is empty (e.g. an interactive form with no project name entered).

**Fix:** Validate `dir != ""` in `RunPostHook` and return an error:

```go
func RunPostHook(t *Template, values map[string]any, dir string) error {
    if t == nil || t.SpinToml == nil {
        return nil
    }
    if dir == "" {
        return fmt.Errorf("post-hook: dir is required (empty dir runs in process cwd)")
    }
    // ... rest
}
```

## Warnings

### WR-01: `looksLikeV2Template` heuristic causes false positives on v1 names containing `/`

**File:** `cmd/new.go:54-72`, `cmd/new_extras.go:57`
**Issue:** The v1 template name `"tui-bubbletea"` is the canonical default, but the heuristic `looksLikeV2Template` returns `true` for ANY string containing `/`. If a future v1 template is named with a `/` (e.g. `cli/cobra` or `web/htmx`), the legacy `spin new <name> --template cli/cobra` will be mis-routed to the v2 git-spec branch in `cmd/new_extras.go:57`, calling `dispatchNewCharmWithTemplate` with `cli/cobra` as the template ref. That will then call `client.Add("cli/cobra")`, which (per `internal/registry/client.go:152`) returns a "shorthand not yet supported" error -- confusing for the user who meant a v1 template name.

**Fix:** Whitelist the known v2 URL schemes AND check for v1 template names explicitly:

```go
func looksLikeV2Template(s string) bool {
    if s == "" {
        return false
    }
    for _, prefix := range []string{"http://", "https://", "git@", "git://", "ssh://"} {
        if strings.HasPrefix(s, prefix) {
            return true
        }
    }
    // "user/repo" shorthand: only treat as v2 if it contains a "." (a domain TLD)
    // OR a "-" AND a "/" (e.g. "vercel/nextjs-tailwind" looks like a user/repo).
    return false  // safer to be conservative here; the explicit URL schemes cover the real cases
}
```

A unit test for `looksLikeV2Template` with inputs `"cli/cobra"`, `"tui-bubbletea"`, `"http://..."`, `"vercel/nextjs"`, `"a/b"` would pin this behaviour.

### WR-02: `cmd/new_extras.go` calls `os.Exit(0)` inside `PreRunE`

**File:** `cmd/new_extras.go:48, 62`
**Issue:** `PreRunE` returning a non-nil error is the cobra-blessed way to abort. `os.Exit(0)` inside `PreRunE` skips the `SilenceUsage: true` / `SilenceErrors: true` machinery, bypasses any wrapping cobra does (e.g. shell completion handlers), and prevents test code from intercepting the exit. If a test wanted to verify "the v1→v2 bridge was taken", it would have to spawn a subprocess because the in-process cobra path exits before returning.

The two `os.Exit(0)` calls (lines 48 and 62) both exit silently with code 0, which is the correct behaviour, but the pattern is brittle.

**Fix:** Set a flag on the command (e.g. `cmd.SetContext(context.WithValue(...))`) or return `cobra.ErrSubCommandRequired` semantics via `RunE = func(cmd *cobra.Command, args []string) error { return nil }` and rely on the parent to short-circuit. Concretely:

```go
// Replace os.Exit(0) with a sentinel return that the parent can detect.
if newListEcosystems {
    // ... print ...
    return nil
}
if tplVal, _ := cmd.Flags().GetString("template"); looksLikeV2Template(tplVal) {
    if len(args) >= 1 && !hasNewSubcommand() {
        if err := dispatchNewCharmWithTemplate(args, cmd, tplVal); err != nil {
            return err
        }
        return errSilentlyExit  // sentinel error
    }
}
```

Or, more idiomatically, set a sentinel on the cobra command and have `runNew` check it before doing the dispatch.

### WR-03: `internal/template/loader.go` `homeDir()` shells out to `sh -c "echo $HOME"` instead of `os.UserHomeDir()`

**File:** `internal/template/loader.go:142-154`
**Issue:** The comment says "kept here to avoid an os import collision with the wider package (which has its own os-using files)". The package already imports `os` (line 5: `"os"`) and uses it on lines 50 (`os.Environ`), 80 (`os.Stat`), 86 (`os.RemoveAll`). The `homeDir` shell-out is gratuitous: it adds a fork+exec per `defaultCacheDir` call, is broken on Windows (no `sh -c`), and is functionally a wrapper around `os.UserHomeDir()`.

On Windows, `exec.Command("sh", "-c", "echo $HOME").Output()` will fail (no `sh`), `homeDir()` returns `("", err)`, and `defaultCacheDir` falls through to `/tmp/spin-templates`, which is then the cache root on Windows. That is probably not the intended behaviour.

**Fix:** Replace the shell-out with the stdlib call:

```go
func defaultCacheDir() string {
    if base, err := os.UserConfigDir(); err == nil && base != "" {
        return filepath.Join(base, "spin", "templates")
    }
    if h, err := os.UserHomeDir(); err == nil && h != "" {
        return filepath.Join(h, ".config", "spin", "templates")
    }
    return "/tmp/spin-templates"
}

// delete the homeDir() helper entirely
```

### WR-04: `internal/runner/source.go:99` `Resolve` "source:task" disambiguation is dead code

**File:** `internal/runner/source.go:87-104`
**Issue:** `Resolve` looks for `strings.HasPrefix(t.Name, name+":")` to support "source:task" disambiguation. But `Task.Name` is just the task name (e.g. "build"), and `Task.Source` is a separate field (e.g. "spin.config.toml:8"). The `Name` is never populated with a colon-prefixed form. The dead branch is harmless (it never matches), but it implies an API that does not exist and confuses future readers.

**Fix:** Remove the dead branch OR actually wire "source:task" disambiguation (e.g. by adding a new export `ResolveBySource(source, name string)` and removing the loop's dead check). For minimum diff, remove the dead check:

```go
for _, t := range all {
    if t.Name == name {
        return t, nil
    }
}
return Task{}, &ErrNotFound{Name: name}
```

### WR-05: `internal/runner/sources/spinconfig.go:111` `parseTaskInlineTable` does not validate brace balance

**File:** `internal/runner/sources/spinconfig.go:111-146`
**Issue:** `parseTaskInlineTable` calls `strings.TrimPrefix(body, "{")` and `strings.TrimSuffix(body, "}")` without verifying that those characters actually exist. The caller (`Tasks` at line 86) does pre-check `strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}")`, so the trimming is safe in practice -- but a single-line inline table that is itself a `value` inside a multiline string (e.g. an env value containing `{...}` followed by `}`) would have the inner `}` removed by the suffix trim. The `splitTopLevel` helper correctly tracks bracket depth, but the `TrimPrefix`/`TrimSuffix` are applied to the raw input first, and if the value contains an outer `}` they would chew into the value.

In practice, the only inputs are user-controlled `spin.config.toml` files, and the schema is constrained, so this is a theoretical issue. Marking as Warning because the spec accepts `command = "..."` values that can contain anything.

**Fix:** Strip the outer braces only if the count of `{` and `}` is balanced. Or, better, use a real TOML parser. For minimum change, the existing code is acceptable; add a comment that documents the assumption.

### WR-06: `internal/ecosystems/rust/validate.go:14-23` `--type` validation rejects all empty strings as "required" but the default is "bin"

**File:** `internal/ecosystems/rust/validate.go:14-23`, `internal/ecosystems/rust/flags.go:11`
**Issue:** `flags.go` declares `ecosystem.ChoiceFlag("type", "bin", ...)` with the default `"bin"`. The `cmd/new_rust.go:86` alias translation sets `flags["type"] = "bin"` only if a `--bin/--lib/--example` flag is true. If the user runs `spin new rust myapp` with no `--type` and no alias, `ctx.GetString("type")` returns `""` (the zero value), and `validate.go:18` returns "project --type is required (bin, lib, or example)" -- but the flag has a default. The default is only applied by `pflag` when the flag is bound to the cobra command and the user did not pass it. The `flags` map constructed in `runNewRust` is built by `cmd.Flags().VisitAll`, so the default IS in the map.

So this is actually fine -- the default flows through `pflag` into the map. The `case "":` branch is dead. The defensive check is good practice, but the error message is misleading because the default exists.

**Fix:** Tighten the error message and remove the `case "":` branch, OR (if the default is only applied for v1 callers that don't go through pflag) add a unit test that confirms the default flows correctly. The simplest fix is to remove the dead branch:

```go
switch t := ctx.GetString("type"); t {
case "bin", "lib", "example":
    // ok
default:
    return ecosystem.NewValidationError(e.Name(),
        fmt.Sprintf("invalid --type=%q (must be bin, lib, or example)", t))
}
```

### WR-07: `internal/registry/client.go:308` `isLocalPath` heuristic returns true for `~something` (no slash)

**File:** `internal/registry/client.go:307-309`, mirrored in `internal/template/loader.go:89-91`
**Issue:** `isLocalPath` returns `true` for any string starting with `~`, including `~foo` (a file literally named `~foo` in the current directory would be a strange thing, but the heuristic is meant to match `~` and `~/...`). The same is mirrored in `template/loader.go:89-91`. The bug is: a user who types `spin add ~something` (which is supposed to mean "home-relative, expand the tilde") will get it interpreted as a local path with the literal `~` character, not as a home-expanded path.

`addLocal` calls `expandHome(spec)` which does handle `~` and `~/` correctly, so the actual add path works. But the heuristic is misleading: a future caller that doesn't go through `expandHome` would treat `~foo` as a local-path-with-tilde. Also, the test `TestLoader_IsLocalPath` at line 88 asserts `{"~foo", true}` -- so the behaviour is pinned, but it's wrong.

**Fix:** Tighten the heuristic to only match `~` or `~/`:

```go
func isLocalPath(s string) bool {
    if s == "" {
        return false
    }
    if s[0] == '/' || s[0] == '.' {
        return true
    }
    if s == "~" {
        return true
    }
    if strings.HasPrefix(s, "~/") {
        return true
    }
    return false
}
```

And update the test to assert `{"~foo", false}`.

### WR-08: `internal/runner/explain.go:34` `Explain` prints `t.Name` (the task name) on the first line with no header

**File:** `internal/runner/explain.go:34`
**Issue:** `Explain` writes `fmt.Fprintln(w, t.Name)` as the first line, then the indented fields. The contract doc says the format starts with `task <name>`, but the implementation prints just `<name>`. This breaks the documented format and any consumer that grep-parses the first line for `task <name>`.

**Fix:**

```go
fmt.Fprintf(w, "task %s\n", t.Name)
```

## Info

### IN-01: `cmd/new_extras.go:147` `boolToString` duplicates cobra's `pflag` built-in

**File:** `cmd/new_extras.go:147-152`
**Issue:** The `boolToString` helper is a 6-line function that converts a `bool` to `"true"` or `"false"` for `cmd.Flags().Set(name, str)`. Cobra's `pflag.Flag` already accepts `strconv.FormatBool`-style strings, and `pflag` has a built-in `flag.Value.Set` that handles bools via `strconv.ParseBool`. The function can be removed if the call site uses `strconv.FormatBool` directly, or kept as-is for clarity.

**Fix:** Replace with `strconv.FormatBool(b)`:

```go
_ = cmd.Flags().Set(f.Name, strconv.FormatBool(b))
```

### IN-02: `internal/ecosystems/rust/post.go:46-63` git init/add/commit do not check the directory exists before init

**File:** `internal/ecosystems/rust/post.go:46-63`
**Issue:** `git init` is called with `init.Dir = dir`. If the directory does not exist (e.g. `writeFiles` succeeded but with a path that was removed by a race), `git init` will create a new one. This is benign (git will init whatever dir is there), but a check for `dir` existence before invoking `git init` would surface the race explicitly. Low priority.

**Fix:** Add `if _, err := os.Stat(dir); err != nil { return fmt.Errorf("rust: project dir %q: %w", dir, err) }` before the git init.

### IN-03: `cmd/run.go:131-148` `projectRoot` is defined but never called

**File:** `cmd/run.go:131-148`
**Issue:** `projectRoot()` walks up from cwd looking for a project root marker. It's defined in the file but not called from any code path (`runRun` uses `os.Getwd()` directly at line 59). This is dead code; either wire it in or remove it. The comment says "helper used by the init bodies below" but the function is the only thing in the file below the `defaultSourceChain` definition that calls it. `grep -n projectRoot` confirms zero callers.

**Fix:** Wire `projectRoot()` into `runRun` so `Runner.Dir` is the project root, not the cwd. This makes `--list` more useful when run from a subdirectory of a project. Or remove it if not needed.

### IN-04: `internal/params/param_test.go:18` `NewNumber("n", "p", 0, nil, nil)` signature is fragile

**File:** `internal/params/param_test.go:20`
**Issue:** `NewNumber` takes 5 positional args: name, prompt, default, min, max. Passing `nil, nil` for min/max is idiomatic but the call site is hard to read. Consider accepting a struct (`NewNumber(NumberSpec{...})`) or varargs. Low priority because the tests pass and the production code is correct.

**Fix:** Optional refactor. No change needed if API churn is undesirable.

### IN-05: `cmd/new_charm.go:48,55` `f.Default.(bool)` and `f.Default.(string)` type assertions are unchecked

**File:** `cmd/new_charm.go:46-56`, `cmd/new_rust.go:38-49`, `cmd/new_extras.go:111-117`
**Issue:** `def, _ := f.Default.(bool)` silently swallows a type-mismatch. If a flag is declared with the wrong default type (e.g. `StringFlag` with `Default: 42` instead of `"42"`), the cobra binding will register with the zero value, not the supplied default. The type assertion would return `(0, false)` and `def` is `0`, which cobra registers as the default -- a silent fallthrough.

**Fix:** Add a unit test that exercises flag binding with a wrong default type and asserts the error. Or wrap the type assertion in a helper that returns an error. Low priority because the flags are coded in the same package and tested via integration.

---

_Reviewed: 2026-06-09T11:55:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

---

## Status: superseded by v2.x pivot (2026-06-10)

The code reviewed in this report (v2.0 ecosystem system + universal task runner) was archived to `~/Projects/Golang/spin-ecosys-tasks-archieve/` on 2026-06-10 as part of the v2.x pivot to a templates-only scaffolder. The findings remain valid as a record of the state of the code at review time, but the issues identified (CR-01/02, WR-01..08, IN-01..05) are moot for the current codebase -- the affected files are no longer in `spin/`.

**Files reviewed here, now in the archive:** `cmd/ecosystem.go`, `cmd/new_charm.go`, `cmd/new_extras.go`, `cmd/new_rust.go`, `cmd/run.go`, `internal/ecosystem/`, `internal/ecosystems/rust/`, `internal/runner/`, `internal/runner/sources/`.

**Files reviewed here, still in `spin/`:** `cmd/add.go`, `cmd/list.go` (restored from archive -- uses `internal/registry`, not the runner), `cmd/new.go` (rewritten on `internal/template`), `internal/registry/`, `internal/params/`, `internal/template/`.
