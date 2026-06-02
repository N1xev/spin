# Pitfalls Research: spin (Go scaffolder for charmbracelet v2)

**Domain:** Go project scaffold CLIs targeting the charmbracelet v2 ecosystem
**Researched:** 2026-06-02
**Confidence:** MEDIUM-HIGH for v1→v2 specifics (verified via Context7 upgrade guides), MEDIUM for ecosystem patterns (verified via docs), LOW for some non-TTY/CI behavior (README gaps require source-level confirmation)

## Executive Summary

`spin` sits at a uniquely failure-prone intersection: it must (a) be a usable Go scaffolder (template execution, file generation, embed pitfalls), (b) ship the v2 charm stack which is mid-major-version churn (paths, signatures, type system), and (c) dogfood the same stack it generates. The most damaging mistakes are v1 API leakage into generated projects — they compile, look right, then break the first time the user runs the program. The second cluster is scaffolder mechanics: `go:embed` glob semantics, template FuncMap ordering, and the absence of any built-in non-TTY handling in `bubbletea`/`gum`. The third cluster is generated-project drift: once a project is on disk, no mechanism exists to update it when `spin` itself changes.

## Critical Pitfalls

### Pitfall 1: Generated project uses v1 charmbracelet import paths

**What goes wrong:**
Scaffolded code imports `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/huh`, etc. The v1 modules still exist on the proxy and compile. The user gets a "works" experience on first run, then hits the v1-to-v2 migration cliff when they try to follow any current tutorial. The fundamental architectural shift (vanity import paths, new module majors) is the v2 story; leaking v1 paths erases the whole value proposition.

**Why it happens:**
LLM training data is dense with v1 examples. Old blog posts and StackOverflow answers copy/paste v1 paths. A scaffolder that copies templates from a v1-era generator inherits the v1 imports verbatim.

**How to avoid:**
- Pin and test all import paths in the template repo: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/huh/v2`, `charm.land/bubbles/v2`, `charm.land/log/v2`, `charm.land/fang/v2`, `charm.land/glamour/v2`.
- Have a CI job that runs `go build` against the generated output and greps for forbidden prefixes (`github.com/charmbracelet/`).
- Bake import verification into `spin verify` (or whatever the post-scaffold sanity check is called).

**Warning signs:**
- `go.mod` of generated project contains `github.com/charmbracelet/*` lines.
- Any `tea.KeyCtrlC`, `tea.WithAltScreen()`, or `lipgloss.DefaultRenderer()` reference.
- `View() string` signature on a Bubble Tea model (v2 returns `tea.View`).

**Phase to address:** Phase 1 (template authoring) and Phase 2 (verification harness) — both must be in place before any user-facing release.

---

### Pitfall 2: Generated model uses removed v1 key/mouse message APIs

**What goes wrong:**
Template contains v1 message handling that no longer compiles or runs in v2:
- `case tea.KeyMsg:` then `msg.Type`, `msg.Runes`, `msg.Alt`
- `case tea.KeyCtrlC:`
- `case " ":` (space)
- `case tea.MouseMsg:` with `msg.X`, `msg.Y`, `msg.Action`
- `tea.MouseButtonLeft/Right/Middle`

These will either fail to compile, silently no-op, or panic at runtime when `MouseMsg` is the new interface type.

**Why it happens:**
- `tea.KeyMsg` is now an interface in v2 (was a struct). Direct field access fails.
- `tea.MouseMsg` is now an interface; `msg.Mouse()` is required to get the `tea.Mouse` struct.
- `KeyCtrlC` constant removed; replaced with `msg.String() == "ctrl+c"` or field-level checks.
- `msg.String()` for space now returns `"space"`, not `" "`.
- Mouse events are split by type (`MouseClickMsg`, `MouseReleaseMsg`, `MouseWheelMsg`, `MouseMotionMsg`).
- Button constants renamed: `MouseButtonLeft` → `MouseLeft`, etc.

**How to avoid:**
- Ship a v2-correct "hello TUI" example as the default template (verified by hand against the upgrade guide).
- Document each v1 pattern in a comment in the template file showing the v2 equivalent.
- Have a unit test that compiles the example and instantiates its program to ensure it boots.

**Warning signs:**
- `tea.KeyMsg` type switch (should be `KeyPressMsg` in v2).
- `msg.Type` / `msg.Runes` / `msg.Alt` field reads.
- `case " ":` branch.
- `tea.MouseButtonLeft` (or any `MouseButton*`).

**Phase to address:** Phase 1 (templates) — this is a per-template hygiene issue, not a runtime check.

---

### Pitfall 3: `View() string` instead of `View() tea.View`

**What goes wrong:**
The default "hello" template implements `View() string`. In v2 the required signature is `View() tea.View`. The scaffolder is a Bubble Tea program itself; if its own `View()` is wrong, the scaffolder TUI itself breaks (or compiles against v1 stub types if some accidental v1 import slipped in).

**Why it happens:**
Most existing Bubble Tea tutorials (pre-2025) show `View() string`. The change to `tea.View` is one of the most pervasive in v2 and is easy to miss.

**How to avoid:**
- Use `tea.NewView("...")` or `var v tea.View; v.SetContent("..."); return v` in every generated model.
- For fullscreen/mouse apps, set `v.AltScreen = true`, `v.MouseMode = tea.MouseModeCellMotion` inside `View()` (no more `tea.WithAltScreen()` program options).
- Lint the scaffolder's own templates with a `gopls`-style check or a simple AST walk before each release.

**Warning signs:**
- `func (m *model) View() string` anywhere in generated or scaffolder code.
- `tea.WithAltScreen(`, `tea.WithMouseCellMotion(`, `tea.WithReportFocus(` in code.
- `tea.EnterAltScreen` / `tea.HideCursor` command returns (all removed in v2).

**Phase to address:** Phase 1 (templates) and a permanent regression test in the scaffolder's own CI.

---

### Pitfall 4: Lipgloss v1 color/background API leakage

**What goes wrong:**
Generated code uses:
- `var c lipgloss.Color = "#ff00ff"` (v1: `Color` was a `string` type; v2: it's a function returning `image/color.Color`)
- `lipgloss.HasDarkBackground()` with no args (v2 requires `HasDarkBackground(in, out)`)
- `lipgloss.DefaultRenderer()`, `lipgloss.NewRenderer(w)`, `lipgloss.SetDefaultRenderer(r)` (all removed)
- `lipgloss.ColorProfile()`, `lipgloss.SetColorProfile(p)` (replaced by `colorprofile.Detect` and `lipgloss.Writer.Profile`)
- `WithWhitespaceForeground(c)`, `WithWhitespaceBackground(c)` (replaced by `WithWhitespaceStyle(s)`)
- `AdaptiveColor`, `CompleteColor`, `CompleteAdaptiveColor` from root package (moved to `compat`)

**Why it happens:**
The color-system redesign is large and documentation migrated gradually. v1-style code is still common in older charm tutorials.

**How to avoid:**
- Default template uses only `lipgloss.NewStyle()` plus a single `lipgloss.HasDarkBackground(os.Stdin, os.Stdout)` call wrapped in a `lipgloss.LightDark` helper if adaptive colors are needed.
- Or, inside a `tea` program, request `tea.RequestBackgroundColor` in `Init` and handle `tea.BackgroundColorMsg` in `Update` to get background info declaratively.
- Add a `// nolint:v1lipgloss` exemption if absolutely needed and document the migration in a sibling file.

**Warning signs:**
- `var _ lipgloss.Color = ""` or any `lipgloss.Color` declared as a `string` field on a struct.
- `lipgloss.NewRenderer(`, `lipgloss.DefaultRenderer(`, `lipgloss.SetDefaultRenderer(`.
- `lipgloss.AdaptiveColor{` (use `compat.AdaptiveColor` or `LightDark`).
- `style.GetForeground()` then comparing to a `string` (the getter returns `color.Color` now).

**Phase to address:** Phase 1 (templates). Critical: this is the easiest place to leak v1 because `lipgloss.NewStyle()` still works in both versions and the breakage is subtle.

---

### Pitfall 5: `go:embed` + `text/template` glob / naming collisions

**What goes wrong:**
Scaffolder embeds `templates/*.tmpl` via `go:embed` and parses with `template.ParseFS(embedFS, "templates/*.tmpl")`. Several silent failures follow:
- `ParseFS` uses `path.Match` (forward-slash globs) — Windows backslashes in patterns match nothing.
- Template names come from `filepath.Base` of filenames. `templates/admin/page.tmpl` and `templates/public/page.tmpl` collide and one clobbers the other; the loser is silently unreachable.
- `t.Execute(w, data)` fails if `t`'s name doesn't match a base filename in the parsed set. Must use `ExecuteTemplate(w, "name", data)`.
- `Funcs(funcMap)` must be called *before* `Parse`/`ParseFS`. Calling it after only overwrites existing entries; the new functions are not available to templates that haven't been parsed yet.
- Default `Option("missingkey=invalid")` silently prints `<no value>` for a typo in a template key — a frequent source of "why is my generated file showing `<no value>`?".

**Why it happens:**
The Go template package's design is well-documented but its failure modes are silent. Embed paths look filesystem-y but actually use a different rule set.

**How to avoid:**
- Adopt a naming convention that guarantees unique base names per logical template (e.g. `templates/cli_main.go.tmpl`, `templates/tui_model.go.tmpl`, not `templates/main.go.tmpl` everywhere).
- Always call `ExecuteTemplate` with an explicit name, never bare `Execute`.
- Register the `FuncMap` once on a fresh `template.New("")` *before* any parse call.
- During dev, set `Option("missingkey=error")` so a typo in a key fails loudly.
- Write a unit test that parses the embedded FS, asserts each expected template name is present, and renders each with sample data to a `bytes.Buffer`.

**Warning signs:**
- `filepath.Join("templates", "*.tmpl")` (or any backslash) in a `ParseFS` pattern.
- Multiple `*_main.go.tmpl` files in the embed tree.
- `t.Execute(w, data)` (use `ExecuteTemplate`).
- A generated file containing the literal string `<no value>`.
- Custom template functions that always return empty strings in dev builds.

**Phase to address:** Phase 1 (template engine) and Phase 2 (test coverage) — both required before any user sees a generated project.

---

### Pitfall 6: `gum` interactive prompts fail or hang in non-TTY environments (CI, piped stdin, subshells)

**What goes wrong:**
`gum input`, `gum choose`, `gum confirm`, `gum filter` require a TTY. In GitHub Actions, GitLab CI, plain cron jobs, or any context where stdin/stdout isn't a terminal (including `spin new foo | tee log.txt` or `spin new foo < /dev/null`), `gum` either errors out, hangs forever, or — depending on the command — silently produces empty output that the scaffolder then writes verbatim into a Go file (project compiles to an empty `name`).

**Why it happens:**
Gum is built on Bubble Tea, which "assumes control of stdin and stdout" (per the Bubble Tea README). There is no documented `--no-interactive` flag. The README has no CI guidance. The way gum is typically used in automation is by piping a finite input to a `filter` / `choose` — but the *user* still has to press a key to confirm.

**How to avoid:**
- Detect non-TTY at the top of `spin new` with `isatty.IsTerminal(os.Stdin)` (or `golang.org/x/term`). If not a TTY, refuse to enter prompt mode and emit a clear error: "interactive prompts require a TTY; pass --tui / --cli / --all and --cobra / --fang etc. explicitly, or run from a real terminal."
- Provide a `--no-interactive` flag (alias: `--yes` / `--batch`) that skips all gum calls and uses cobra-flag values, defaulting unspecified flags to "all-on" for the chosen project type.
- If `gum` is not on `PATH`, fall back to huh (a Go-native TUI form library) when running inside `spin` itself — but better: detect with `exec.LookPath("gum")` and either fail clearly or fall back to huh.
- Document this in `spin new --help` and in the README so CI users don't trip on it.

**Warning signs:**
- `exec.Command("gum", "input", ...).Run()` without a `isatty` guard.
- A CI workflow step that runs `spin new foo` with no stdin attached.
- Generated `main.go` containing the literal word `<no value>` or a `name` constant set to the empty string.
- Spin itself hanging in a piped shell.

**Phase to address:** Phase 1 (scaffolder CLI) — must be the first interactive feature, with the non-TTY guard in place from day one.

---

### Pitfall 7: `bubbletea` program in the scaffolder itself has no non-TTY fallback

**What goes wrong:**
If the scaffolder's own interactive flow (for example, a charm TUI mode) is implemented with `tea.NewProgram(...)`, the scaffolder crashes or hangs when launched from a pipe, a CI log, or any other non-TTY context. The README acknowledges this: "Bubble Tea apps assume control of stdin and stdout" and the documented debugging workaround is to "run delve in headless mode."

**Why it happens:**
There's no clean way for a Bubble Tea program to detect non-TTY and degrade gracefully. Programs that want to also work in pipelines (logging to a file, etc.) must detect TTY themselves and branch.

**How to avoid:**
- Even though the PROJECT spec says "the scaffolder is a CLI" (not a TUI), the scaffolder does prompt via gum. The gum prompts wrap Bubble Tea, so the same non-TTY hazard exists.
- The scaffolder's TUI components (if any) must guard with `isatty.IsTerminal(os.Stdin) && isatty.IsTerminal(os.Stdout)` and fall back to a text/cobra-driven prompt flow in non-TTY mode.
- If `--no-interactive` is set, never instantiate any bubbletea program.
- For the `spin` TUI mode (if added later), treat the `--no-interactive` flag as the canonical "headless" mode and document it.

**Warning signs:**
- A test or CI step that runs the scaffolder under `script -qfc "..."` or with `</dev/null` and expects a TUI.
- `tea.NewProgram(...)` with no surrounding TTY check.
- Logs from the scaffolder being interleaved with the TUI's redraws.

**Phase to address:** Phase 1 (scaffolder CLI) and Phase 3 (interactive UX hardening).

---

### Pitfall 8: Generated project has un-pinned or wrong-version dependencies

**What goes wrong:**
Generated `go.mod` references `charm.land/bubbletea/v2` but without an explicit version, or with a `latest` pseudo-version that resolves differently tomorrow, or with mixed v1 and v2 lines (because the template engine pulled in a transitive v1 dep). When the user runs `go mod tidy` they may pull in a newer-than-tested version, or worse, the v2 `bubbletea` requires a Go toolchain version newer than the user has installed and the build fails with a confusing `go.mod` N+1 error.

**Why it happens:**
Templates either omit versions, use pseudo-versions, or copy pinned versions from the scaffolder's own `go.mod` (which may be a development version with -rc tags). The v2 stack is still rapidly moving (v2.0.0 of bubbletea, v2.0.0-beta.2 of lipgloss as of the most recent docs).

**How to avoid:**
- Pin exact versions known to compile together in the template. Document the test matrix in the template repo.
- Include a generated `go.sum` (or instruct the user to run `go mod download`); don't rely on `go mod tidy` to "find" the right versions.
- Set `go 1.25.0` (the minimum required by fang v2 / gofumpt / air) in the generated `go.mod`'s `go` directive and document this in the README. fang v2 specifically declares `go 1.25.0` in its own go.mod.
- Add a post-generation step in `spin new` that runs `go build ./...` against the generated project before declaring success — fail the scaffolder loudly if the build is broken.

**Warning signs:**
- A generated `go.mod` line without a `/vX` suffix for a charm v2 module.
- `// indirect` v1 charmbracelet lines appearing in the generated `go.sum`.
- "go.mod requires go 1.25 (running go 1.22.x)" build errors.
- Pinning to `v0.0.0-20250101-abcdef...` pseudo-versions.

**Phase to address:** Phase 1 (templates) and Phase 2 (build verification). A "perfect first-run" promise is impossible without a post-scaffold `go build` smoke test.

---

### Pitfall 9: `cobra` + `fang` version mismatch breaks the scaffolder

**What goes wrong:**
Fang v2 (`charm.land/fang/v2`) requires `cobra v1.9.1` and `Go 1.25.0`. If the scaffolder pins fang v2 but its own go.mod allows `cobra v1.8.x` (which has subtle differences in flag handling and hidden-command behavior), users get API drift warnings or runtime panics. Conversely, pinning `cobra` to a version older than v1.8 makes the auto-generated `cobra-cli` scaffolds not work.

**Why it happens:**
The fang v2 go.mod has tight constraints; mixing older cobra releases is the default in many examples.

**How to avoid:**
- The scaffolder's own `go.mod` must pin `github.com/spf13/cobra v1.9.1` and `charm.land/fang/v2` to whatever its current latest stable version is.
- Run `go mod tidy` in CI; refuse to merge if `go.mod` would resolve cobra to anything other than the pinned line.
- The generated project gets the *same* pinned cobra (unless the user opts out with `--no-cobra` for a non-CLI project).
- For non-CLI projects, don't import cobra or fang at all.

**Warning signs:**
- `cobra.Command` examples in templates that use v1.7-style flag parsing.
- `go.sum` containing two different `cobra` versions (transitive + direct).
- "could not determine kind of name for C.cmd" build errors after a cobra upgrade.

**Phase to address:** Phase 1 (scaffolder CLI) — the scaffolder itself uses these, so it must be pinned correctly before templates can copy from it.

---

### Pitfall 10: `air` config drift (deprecated `bin` field, missing `entrypoint`)

**What goes wrong:**
Generated `.air.toml` uses `build.bin = "tmp/main"` which is deprecated in favor of `build.entrypoint = ["./tmp/main"]`. The deprecation emits a warning; a future air version will remove the field and the project will silently stop hot-reloading. Other drift:
- `include_ext = ["go"]` missing `tmpl` / `tpl` / `html` extensions, so template edits don't trigger reloads.
- `exclude_dir` missing `tmp` or `vendor` causing infinite rebuild loops.
- WSL: single quotes in the bin path require escaping (issue #305).
- Default platform-specific overrides are limited to a small field set; copying `[build.linux]` settings into a non-Linux machine will fail to load.

**Why it happens:**
Air's `.air.toml` schema is a moving target. The default scaffolded config is often a year out of date by the time users discover it.

**How to avoid:**
- Always use `build.entrypoint`, never `build.bin`.
- Include `include_ext = ["go", "tpl", "tmpl", "html", "css", "js", "json", "yaml", "toml"]` so template/static edits also trigger rebuilds.
- Set `exclude_dir = ["assets", "tmp", "vendor", "node_modules", ".git"]` and `exclude_unchanged = true` to avoid noise.
- Set `exclude_regex = ["_test\\.go$", ".*_mock\\.go$", ".*\\.generated\\.go$"]`.
- Document the `air -- -h` (separator) and the WSL escape rules in a comment block at the top of the generated `.air.toml`.

**Warning signs:**
- `[build]\nbin = "tmp/main"` in the generated config.
- A `spin run` that re-runs on every save even when the file is unchanged.
- "file watcher: too many open files" errors (missing excludes).

**Phase to address:** Phase 1 (template authoring). Phase 2 should add a smoke test: run `air -d` for 3 seconds against the generated config, save a file, confirm rebuild.

---

### Pitfall 11: `prism` requires Go 1.24+ and has no CI/non-TTY story

**What goes wrong:**
`spin test` shells out to `prism`. Prism requires Go 1.24 or later (it uses `go test -json` introduced in that version). On a user machine with Go 1.22, `prism` fails immediately with a confusing "feature requires Go 1.24+" error — the scaffolder doesn't pre-check, so the user gets a stack trace. Additionally, prism has no documented CI output mode; in GitHub Actions the colored bar output can pollute log capture.

**Why it happens:**
Prism is a thin wrapper around `go test -json` and assumes the user has a recent Go. The scaffolder is responsible for detecting a compatible Go version.

**How to avoid:**
- `spin test` should run `go version` first, fail with a clear message if < 1.24, and offer `go test ./...` as a fallback.
- The generated project's `Taskfile.yml`/`Makefile` should expose both `test: prism` and `test-fallback: go test ./...`.
- Document `--no-color` for prism in the generated README so CI users can opt out.
- Don't make prism mandatory: if `prism` isn't on PATH, fall back to `go test` (with a warning, not a failure). Same fallback logic as the formatter.

**Warning signs:**
- A user with Go 1.22 trying to run `spin test` and getting a prism traceback.
- CI logs filled with escape codes from prism's output.
- Generated `Taskfile.yml` calling `prism` with no fallback target.

**Phase to address:** Phase 1 (scaffolder CLI) — the version check belongs in the wrapper. Phase 2 (docs) for the CI guidance.

---

### Pitfall 12: `gofumpt` not installed → silent fallback to `gofmt` produces wrong formatting

**What goes wrong:**
`spin fmt` is supposed to run `gofumpt -l -w .` and then `goimports -w .`. If `gofumpt` isn't on PATH, a naive wrapper runs `gofmt` instead. The user's editor then keeps reformatting the file because gofumpt's stricter rules (no naked returns, grouped adjacent params, etc.) are not applied. The scaffolder says "format applied" but the file isn't actually formatted to the project's standards. The same hazard exists for `goimports`.

**Why it happens:**
Gofumpt requires `go install mvdan.cc/gofumpt@latest`, which itself requires Go 1.25+. Many users don't have gofumpt installed, and many `Taskfile.yml`s don't declare it as a dependency.

**How to avoid:**
- `spin fmt` should `exec.LookPath("gofumpt")` first. If missing, *fail with an install instruction* (`go install mvdan.cc/gofumpt@latest`) and an opt-out flag (`--no-strict`) that falls back to gofmt, but warn loudly that the generated code is not in the project's preferred style.
- Same for `goimports`.
- The generated `Taskfile.yml`/`Makefile` should have a `setup` target that installs both via `go install`.
- Document the `//gofumpt:diagnose` comment trick for bug reports in the generated README.

**Warning signs:**
- `gofumpt -l .` returning files in CI that `spin fmt` "succeeded" on.
- A user who never installed gofumpt but is using `spin fmt` daily and getting drift.
- Generated `Makefile` referencing `gofumpt` without a target to install it.

**Phase to address:** Phase 1 (scaffolder CLI) and Phase 2 (verification of `spin fmt` output against `gofumpt` directly).

---

### Pitfall 13: Generated `AGENTS.md` goes stale as the project grows

**What goes wrong:**
`spin new --ai` writes an `AGENTS.md` with project type, key commands, and a "what this is" blurb. Two weeks later, the user has added a `migrations/` directory, a Dockerfile, a CI workflow, and a different test runner — and `AGENTS.md` is still describing the freshly-generated state. AI assistants then confidently do the wrong thing because the file is a fossil.

**Why it happens:**
AGENTS.md has no formal spec, no version, no schema. There is no standard machine-readable way to say "this section is auto-generated by spin, regenerate via `spin update-agents`". Competing formats (`CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md`) may appear alongside.

**How to avoid:**
- The generated `AGENTS.md` includes a clear `<!-- AUTOGENERATED by spin X.Y.Z; safe to edit, but commands may drift -->` marker at the top.
- Provide a `spin agents` subcommand that re-emits the file (project authors can run it after they update dependencies or change structure).
- Make the AI-mode opt-in (the PROJECT spec already does this via `--ai`); the file is never the default.
- Keep the file's content focused on: how to build, how to test, what framework versions are pinned, and the file layout. Avoid auto-generating "architecture" descriptions that the user will want to keep current.
- Don't ship an `AGENTS.md` claim that the standard is versioned or that spin "supports" it as a spec — it is a convention.

**Warning signs:**
- Generated AGENTS.md that mentions specific source files by name (those will move).
- No "regenerate" or "edit" guidance in the file.
- Stale dependency version references inside the AI file.

**Phase to address:** Phase 1 (templates) and Phase 3 (post-generation commands). Out of scope for v1 per PROJECT.md, but the file's existence is in scope, so the marker comment and regeneration hook must ship with v1.

---

### Pitfall 14: Breaking generated projects when `spin` itself updates

**What goes wrong:**
A user runs `spin new myapp --tui --bubbletea` on day 1. Three weeks later, they upgrade `spin` (via `go install ...@latest`). They then re-run `spin new myappv2 --tui --bubbletea` and notice the new template has improved examples, but their `myapp` is on the old template. Worse, a template refactor in spin changes a *file path* or *import path* inside the template — anyone who patched their generated project to use the new pattern now has a confusing diff if they try to manually backport.

**Why it happens:**
The PROJECT spec explicitly puts "auto-updating generated projects" out of scope: "regenerate instead." That's the right call for v1, but it creates an upgrade cliff: there is no safe way to bring an old project up to the new template.

**How to avoid:**
- Make the limitation explicit in the README and in the `spin new` post-output message.
- Never silently change a template's behavior in a way that breaks the *guaranteed-to-compile* contract: the file may look different, but `go build ./...` and `go test ./...` must still pass.
- For each template, include a `// generated by spin X.Y.Z` header so users (and tools) can identify the version that produced the file.
- Avoid generating any file the user would normally edit (e.g., main.go is fine; a complex custom config might not be).
- Document a manual "merge the new template into your project" recipe (the safe `diff -ruN spin-template-v1 spin-template-v2 > patch` flow) for users who want to upgrade.
- Track the scaffolder version in the generated `go.mod` via a build-time-injected `//go:build spinv1.2.3` tag, so tools can tell at a glance which spin produced the project.

**Warning signs:**
- A generated file that no longer has a "generated by spin X.Y.Z" comment.
- A template change in spin's git history that touches a user-edited file (e.g., main.go beyond the hello example).
- A user reporting that "spin new" produces different output today than last week without a major version bump in spin.

**Phase to address:** Phase 1 (template versioning discipline) and Phase 2 (CI test that pins templates to compile). The upgrade story itself is explicitly v2+ scope; v1 just needs to not paint itself into a corner.

---

### Pitfall 15: Generated project uses CGO accidentally → cross-compile breaks

**What goes wrong:**
PROJECT.md says scaffolded projects "should build with CGO_ENABLED=0". A generated TUI accidentally pulls in a CGO-requiring dep (e.g., a SQLite driver, a TTS lib, or — more insidiously — a charmbracelet v1 lipgloss that had CGO helpers in some transitive path). The user runs `go build` on macOS and it works, then `docker build` (Linux) fails because CGO is disabled in the build environment, and they don't know why.

**Why it happens:**
CGO is a transitive concern. Pinning deps doesn't make them CGO-free; the dep tree does.

**How to avoid:**
- Run a CI smoke test: `CGO_ENABLED=0 go build ./...` on the generated project for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`. Fail the scaffolder CI if any combination fails.
- Document the CGO=0 contract in the generated README: "this project is CGO-free; if you add a CGO dep, the cross-compile guarantees are void."
- For charm v2: the v2 stack is CGO-free (the lipgloss Renderer removal in v2 also removed the CGO termios shim), so this is mostly a forward concern about user-added deps. But verify at scaffold time.
- The GoReleaser config (when shipped) should set `env: [CGO_ENABLED=0]` as the default with `overrides` only where truly required (e.g., macOS signing).

**Warning signs:**
- A go.sum containing a dep with `cgo` in its tag path.
- A build matrix CI step failing on `linux/arm64` only.
- Users reporting "works on my Mac, fails in Docker".

**Phase to address:** Phase 2 (CI matrix for the generated project) and the GoReleaser config in Phase 1 or Phase 3.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip `go mod tidy` after generation | Faster scaffold | Un-pinned transitive deps; "works for me" syndrome | Never — always run tidy |
| Hardcode charm v2 versions in `go.mod` (no `replace` directives) | Simple | Cannot test against local charm source | MVP-OK; revisit for v2 |
| Embed templates as `//go:embed templates/*.tmpl` with no per-template go file generation | Less code | Filename collisions silently clobber templates | Never — use unique base names |
| Use `os.Exit(1)` directly in cobra `RunE` errors | Trivial | Tests can't intercept; no deferred cleanup | Acceptable for `main` only; wrap in `os.Exit(fang.Execute(...))` |
| Hard-pin cobra to v1.9.1 in scaffolder | Avoids fang v2 conflict | Slower to adopt cobra improvements | Acceptable while fang v2 is the consumer |
| Single `.air.toml` shared across project types | Less work | TUI vs CLI needs differ; some include_ext drift | Only if both project types use the same hot-reload pattern |
| Auto-add `AGENTS.md` to every project | Matches "AI friendly" trend | Drifts; becomes noise; some users hate AI files | Default OFF (per PROJECT spec) |
| Skip post-scaffold `go build` smoke test | Faster UX | Ship templates that don't compile; brand damage | Never — this is the core value prop |
| Bundle a single "default" template and skip the `--template` flag | Less to maintain | Power users can't customize; PROJECT requires the flag | Default template is required, others can come later |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `gum` | Treating it as a library; calling it via `os/exec` without TTY detection | Check `isatty.IsTerminal(os.Stdin)` first; provide `--no-interactive` escape hatch |
| `air` | Using deprecated `build.bin` | Use `build.entrypoint = ["./tmp/main"]`; include `include_ext` for templates |
| `prism` | Assuming Go ≥ 1.24 universally | Check `go version`; offer `go test` fallback; document `--no-color` for CI |
| `gofumpt` | Silently falling back to `gofmt` | Fail with install instructions; allow `--no-strict` opt-out |
| `cobra` (in scaffolder) | Mixing v1.7 and v1.9 APIs | Pin v1.9.1 to match fang v2; document the choice in a code comment |
| `fang` (v1) | Using v1 import path with v2 templates | Use `charm.land/fang/v2`; rewrite themes via `WithColorSchemeFunc` |
| `bubbletea` (in scaffolder) | Wrapping in a TUI without TTY check | Default to cobra; gate any TUI behind `--tui-scaffold` flag with TTY check |
| `huh` (in scaffolder) | Using v1 import path | `charm.land/huh/v2`; accessible mode set on form, not field |
| `text/template` (in scaffolder) | Using `t.Execute` instead of `ExecuteTemplate` | Always `ExecuteTemplate(w, "name", data)` |
| `embed.FS` (in scaffolder) | Using OS path separators in `ParseFS` patterns | Always forward slashes; embed directive uses forward slashes too |
| GoReleaser | `CGO_ENABLED=1` by default | Default to `CGO_ENABLED=0`; use `overrides` only where needed |
| `lipgloss` (v1) | Using v1 color types | v2 `Color` is a function; use `LightDark` for adaptive colors |
| `lipgloss.HasDarkBackground` (v1) | Calling with no args | v2 requires `HasDarkBackground(in, out)` |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| `ParseFS` on every scaffold call | Scaffold takes 200ms+ | Parse once at `init()` time; cache `*template.Template` | At ~10 templates or more; not breaking in practice |
| `air` watching entire `node_modules` | High CPU on save, OS file-descriptor exhaustion | `exclude_dir = ["node_modules", "vendor", "tmp", "assets"]` | When the generated project grows subdirs |
| Spawning `gum` as a subprocess per prompt | Slow UX, fragile TTY | Use huh (Go-native) for the scaffolder's own prompts; reserve gum for the generated CLI's UX | If a future "spin init" does multi-step flow |
| `go mod download` on every test run | Slow CI | Commit `go.sum`; rely on Go's module cache | Annoyance, not breakage |
| `lipgloss.NewRenderer` per call (v1) | Memory churn (v2: removed, so this is moot) | Migrate to v2 (Renderer is gone) | N/A post-v2 |
| `prism` forking `go test -json` for every test run | Test overhead, no parallelism improvement | It's what prism does; embrace it; don't wrap in another shell | The wrapper is fine; do not double-wrap |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Generated `go.mod` resolves a non-canonical module path (typosquat) | User gets malicious code | Pin to exact versions in template; CI checks `go.sum` |
| `--template-repo <url>` accepts any URL without verification | User pulls arbitrary code | Document the risk; consider a `GIT_CONFIG_GLOBAL` and `GIT_TERMINAL_PROMPT=0` for non-interactive clone; add `--allow-insecure` opt-out |
| Generated project runs `curl | sh` in setup scripts | RCE if compromised | Never emit install scripts that pipe curl; use `go install` (signed) |
| `air` exec'd on user-supplied config | Bad configs could `rm -rf` if pre_cmd is crafted | When loading external `.air.toml`, sanitize pre_cmd/post_cmd; or document that the file is trusted |
| `go:embed` of a directory that contains `.env` or secrets | Leaks secrets into the binary | Use `.air.toml` `exclude_dir` for `.env`; never embed a top-level `secrets/` |
| Generated README includes a token placeholder that the user pastes | Leak via search engines or git push | Use generic placeholders like `<YOUR_API_KEY>`; warn in setup |
| `gum` forking in CI with unsanitized input | Shell injection if input is concatenated | Pass values as separate args, never via shell interpolation |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| `spin new` without flags tries to ask 6 questions via gum | "Just give me a hello world" takes 30s of clicking | Provide a `--default` flag (or default if exactly one type is set via flag) |
| Generated project doesn't compile on first run | Brand-destroying | Post-scaffold `go build ./...` smoke test; fail fast |
| `spin run` shells into `air` without checking `.air.toml` exists | Confusing "no config" error | Detect and fall back to `go run`; print a one-liner explaining the fallback |
| `spin fmt` silently falls back to gofmt | User's editor reformats; mild chaos | Fail loudly with install instructions; `--no-strict` opt-out |
| `spin build` outputs to an unconfigurable path | Fights the user's existing CI | Default to `bin/`, allow `--output` flag |
| Generated README has no "what now" section | User is dropped into a working project with no next step | Always emit a "Next steps" block with the exact commands to run |
| TUI's mouse mode set in `View()` (v2 way) but the user's terminal doesn't support it | Confusing behavior | Document the v2 declarative model; use `MouseMode = tea.MouseModeNone` for `View()`s that don't need it |
| `gum` prompts use `lipgloss` styles that don't respect NO_COLOR | Accessibility fail | Honor `NO_COLOR` env var (charm does this by default; verify in tests) |
| Interactive prompts and CI invocation look the same | CI users hit hangs | Distinct UX: `spin new --ci foo` or auto-detected non-TTY fallback |
| `AGENTS.md` shipped by default | Some users don't want AI-assistant context files | Default OFF; require explicit `--ai` / `--agents` flag |

## "Looks Done But Isn't" Checklist

- [ ] **Templates compile:** Every `--template` variant passes `go build ./...` from a clean clone. Verify by spinning up a CI matrix that runs `spin new ...` for every flag combination.
- [ ] **Templates test:** Every variant has at least one trivial test file (e.g., `package main_test.go` with a smoke test) so `go test` is non-empty.
- [ ] **No v1 imports:** CI greps generated `*.go` for `github.com/charmbracelet/` and fails on any match.
- [ ] **No v1 key/mouse API:** CI greps for `tea.KeyMsg`, `tea.MouseButtonLeft`, `tea.WithAltScreen`, `tea.KeyCtrlC`, `case " ":` and fails.
- [ ] **No v1 lipgloss:** CI greps for `lipgloss.NewRenderer`, `lipgloss.DefaultRenderer`, `var _ lipgloss.Color = ""`, `lipgloss.AdaptiveColor{`, `lipgloss.HasDarkBackground()` (no args) and fails.
- [ ] **`View() tea.View`, not string:** CI greps for `func .* View() string` in template files.
- [ ] **Fang v2 imports only:** Scaffolder's own go.mod uses `charm.land/fang/v2`; CI greps for the v1 path.
- [ ] **gofumpt available check:** `spin fmt` exits non-zero (or with a clear install hint) if gofumpt is missing, rather than silently running gofmt.
- [ ] **prism fallback exists:** Generated Taskfile has `test: prism` *and* `test-fallback: go test ./...`; `spin test` itself has a Go version check.
- [ ] **air config uses entrypoint, not bin:** CI greps generated `.air.toml` for `bin = "tmp/main"` and fails.
- [ ] **CGO=0 build matrix:** CI builds the generated project for 5 OS/arch combos with `CGO_ENABLED=0` and passes.
- [ ] **Non-TTY behavior:** `echo "" | spin new foo` exits with a clear error (not a hang, not a silent empty-name project).
- [ ] **`AGENTS.md` opt-in:** Default scaffold does not produce `AGENTS.md`; requires `--ai`.
- [ ] **Generated README has Next Steps section:** CI checks for a "## Next Steps" or "## Quickstart" header.
- [ ] **`go:embed` patterns forward-slashed:** Template `ParseFS` calls use `/` separators; CI greps for `\\` in embed patterns.
- [ ] **Unique template basenames:** A test asserts that `template.ParseFS(...).Templates()` returns N distinct names for N embed files.
- [ ] **FuncMap registered before parse:** A test asserts that custom funcs work in every template; if a func returns `<no value>`, the test fails.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Generated project uses v1 imports | MEDIUM | `gofmt -w -r 'github.com/charmbracelet/bubbletea -> charm.land/bubbletea/v2' .` per file; manually fix key/mouse/view API; rerun upgrade guide; `go mod tidy` |
| `View() string` in v2 project | LOW | Change signature, wrap return in `tea.NewView(...)`; set `v.AltScreen` etc. on the View struct |
| `lipgloss.NewRenderer` in v2 | MEDIUM | Remove Renderer entirely; switch to `lipgloss.NewStyle()`; rewrite adaptive color logic with `LightDark` |
| `go:embed` pattern with backslashes | LOW | Replace with forward slashes; rebuild; verify ParseFS still finds all files |
| `t.Execute` instead of `ExecuteTemplate` | LOW | Switch to `ExecuteTemplate(w, "name", data)`; the name is the template's base filename |
| `gum` hang in CI | LOW | Re-run with `</dev/null` and detect non-TTY; use `--no-interactive` flag |
| `air` deprecated `bin` field | LOW | Rename to `build.entrypoint = ["./tmp/main"]`; remove the deprecation warning |
| `prism` requires Go 1.24 but user has 1.22 | LOW | Either upgrade Go, or use the `test-fallback` target in the generated Taskfile |
| `gofumpt` not installed and `spin fmt` silently ran `gofmt` | LOW | `go install mvdan.cc/gofumpt@latest`; rerun `spin fmt`; commit the diff |
| Stale `AGENTS.md` | LOW | Delete the file; re-run `spin new --ai`; manually merge any user-added notes |
| Generated project pulls in CGO dep | MEDIUM-HIGH | Find the dep, replace with a CGO-free alternative, or add explicit CGO support and update README |
| Template filename collision clobbered a template | MEDIUM | Rename the clobbered template's base filename; add a uniqueness test; rebuild scaffolder |
| Schema-versioned AGENTS.md assumption broken | HIGH | Edit the file; document the limitation; consider a v2 spin feature for live-update |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| v1 charmbracelet import paths in generated code | Phase 1 (templates) | CI grep step on every generated project; `go mod tidy` passes |
| v1 key/mouse message API in generated code | Phase 1 (templates) | CI grep for `tea.KeyMsg`, `tea.MouseButtonLeft`, `tea.KeyCtrlC`, `case " ":` |
| `View() string` instead of `View() tea.View` | Phase 1 (templates) | CI grep for `View() string`; compile-test the example |
| Lipgloss v1 color/Renderer API | Phase 1 (templates) | CI grep for `lipgloss.NewRenderer`, `lipgloss.DefaultRenderer`, `lipgloss.AdaptiveColor{` |
| `go:embed` + `text/template` glob/naming collision | Phase 1 (template engine) | Unit test that parses all templates, asserts unique names, renders each with sample data |
| `gum` non-TTY hang | Phase 1 (scaffolder CLI) | `echo "" | spin new foo` exits cleanly; `--no-interactive` flag exists and works |
| `bubbletea` non-TTY hang in scaffolder itself | Phase 1 (scaffolder CLI) | TTY guard around any TUI; tests run with `</dev/null` |
| Un-pinned or wrong-version charm v2 deps | Phase 1 + Phase 2 | `go.mod` has explicit versions; `CGO_ENABLED=0 go build` for 5 OS/arch combos |
| `cobra` + `fang` version mismatch | Phase 1 (scaffolder CLI) | `go.mod` pins cobra v1.9.1, fang v2; CI `go mod tidy` is a no-op |
| `air` deprecated `bin` field | Phase 1 (templates) | CI grep for `bin = "tmp/main"`; smoke test `air -d` for 3s |
| `prism` Go 1.24+ requirement | Phase 1 (scaffolder CLI) | `spin test` checks `go version`; Taskfile has `test-fallback` target |
| `gofumpt` silent fallback to `gofmt` | Phase 1 (scaffolder CLI) | `spin fmt` exits non-zero if gofumpt missing; install hint printed |
| AGENTS.md staleness / standards drift | Phase 1 + Phase 3 (post-gen) | Generated file has version marker; `spin agents` regen command exists |
| Breaking generated projects on spin upgrade | Phase 1 (template discipline) | Generated file has `// generated by spin X.Y.Z`; documented "no auto-upgrade" policy |
| CGO accidental in generated project | Phase 2 (CI matrix) | `CGO_ENABLED=0 go build` for `linux/{amd64,arm64}`, `darwin/{amd64,arm64}`, `windows/amd64` |
| Interactive prompts hang in piped/CI context | Phase 1 (scaffolder CLI) | isatty check; `--no-interactive`; integration test with `</dev/null` |
| Generated project doesn't compile | Phase 2 (verification) | `spin new` runs `go build ./...` post-scaffold; fails loudly |
| Schema ambiguity in AGENTS.md | Phase 1 (templates) | File includes `<!-- AUTOGENERATED by spin X.Y.Z -->` marker; no versioned-spec claim |
| Unpinned gofumpt / goimports in generated project | Phase 1 (templates) | Generated `Taskfile.yml` has a `setup` target running `go install ...@latest` |
| Bubbletea v2 program option removal (`WithAltScreen` etc.) | Phase 1 (templates) | CI grep for `tea.WithAltScreen(`, `tea.WithMouseCellMotion(`, `tea.EnterAltScreen` |

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Template authoring (Phase 1) | v1 API drift in LLM-suggested snippets | Hand-verify every snippet against the v2 upgrade guide; run a "compile test" step on the example before commit |
| Scaffolder CLI (Phase 1) | TTY assumptions from cobra/fang examples | Default to cobra (no TUI); guard any bubbletea with isatty |
| Verification harness (Phase 2) | Tests pass locally but fail in CI due to non-TTY | Use `script` or `unbuffer` in CI; or set up a matrix that explicitly tests with `</dev/null` |
| Interactive UX (Phase 3) | Gum-based prompts look great locally, hang in CI | Two-mode design: interactive (TTY) and batch (`--no-interactive`); test both |
| GoReleaser config (Phase 3) | CGO leaks into the released binary via a user's transitive dep | Default `CGO_ENABLED=0`; document override path; smoke-test the released tarball with `CGO_ENABLED=0 go build` of an example downstream user |
| Generated project post-mortem (Phase 3) | User reports a generated project that doesn't compile; hard to reproduce | Include `// generated by spin X.Y.Z` markers; `spin doctor` subcommand dumps the env + flag set used to generate the project |
| Upgrade story (deferred to v2 per PROJECT.md) | User runs `spin new foo` weekly; templates drift; no safe way to upgrade old projects | Document the limitation loudly; provide a `spin diff <project>` (v2 feature) that shows what would change if re-scaffolded |
| External template repo (Phase 2) | A user points `--template-repo` at a URL that has v1 examples | Document the responsibility; add `--trust-external` flag; v2 could add a `spin template verify <repo>` that lints the templates |

## Sources

### Verified (HIGH confidence)
- [Bubble Tea v1→v2 Upgrade Guide (Context7)](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md) — `View() string` → `View() tea.View`, key message interface, mouse interface, removed program options
- [Lip Gloss v1→v2 Upgrade Guide (Context7)](https://github.com/charmbracelet/lipgloss/blob/main/UPGRADE_GUIDE_V2.md) — `Color` function, `Renderer` removal, `HasDarkBackground` signature, `WithWhitespaceStyle`
- [Huh v1→v2 Upgrade Guide (Context7)](https://github.com/charmbracelet/huh/blob/main/UPGRADE_GUIDE_V2.md) — v2 import paths, accessible mode at form level
- [Fang v1→v2 Upgrade Guide (Context7)](https://github.com/charmbracelet/fang/blob/main/UPGRADE_GUIDE_V2.md) — `WithColorSchemeFunc`, lipgloss v2 dependency
- [Fang go.mod (raw)](https://raw.githubusercontent.com/charmbracelet/fang/main/go.mod) — requires `cobra v1.9.1`, `go 1.25.0`
- [Air documentation (Context7)](https://context7.com/air-verse/air/llms.txt) — `build.bin` deprecated → `build.entrypoint`; Go 1.25+ required; WSL escaping
- [Prism README (Context7)](https://github.com/daltonsw/prism/blob/main/README.md) — Go 1.24+ required; drop-in `go test` replacement
- [Gofumpt README](https://github.com/mvdan/gofumpt) — Go 1.25+ to build; skips `vendor` and `testdata` by default; `-extra` flag
- [Go `text/template` package docs (Context7)](https://pkg.go.dev/text/template) — `ParseFS` uses `path.Match`, Funcs-before-Parse, template name collisions, missingkey options
- [GoReleaser Go builder docs](https://goreleaser.com/customization/builds/builders/go/) — `targets: go_first_class`, `CGO_ENABLED=0`, `mod_timestamp`, `ldflags` version stamping

### Verified (MEDIUM confidence)
- [Gum README (Context7)](https://github.com/charmbracelet/gum/blob/main/README.md) — interactive TUI; no documented `--no-interactive`; assumes TTY
- [Cobra GitHub page](https://github.com/spf13/cobra) — pin to v1.9.1 to match fang v2
- [Charm.land homepage](https://charm.land/) — current library list; no v2 stack version statement on marketing page

### Inferred (LOW confidence — needs source-level confirmation)
- Bubble Tea program's non-TTY behavior: README explicitly says "assumes control of stdin and stdout" but does not document the non-TTY failure mode. The behavior described here is inferred from the "use delve in headless mode" debugging guidance and from the gum README's lack of CI guidance.
- AGENTS.md as a stable standard: GitHub repo has 21.9k stars but no versioned spec, no formal governance. The "drift" warning is based on the absence of these signals, not on a documented instability.
- Prism CI behavior: no CI-specific documentation in the README; behavior inferred from `--no-color` flag existence and the use of lipgloss (which honors `NO_COLOR`).

### Unverified gaps to address in phase-specific research
- Exact `tea.NewProgram` failure mode in non-TTY (need to read `tea.go` source)
- `gum` exit code when stdin isn't a TTY (need to read per-command help or source)
- Fang v2's exact error when paired with cobra < v1.9 (likely a compile error in fang's own source, but not confirmed)
- Whether `prism` exits non-zero on Go < 1.24 (likely yes, but not documented)
- The current stable v2 version of each charm library at scaffold-generation time (changing fast; pin via CI at release)

---

*Pitfalls research for: spin (Go scaffolder for charmbracelet v2)*
*Researched: 2026-06-02*
