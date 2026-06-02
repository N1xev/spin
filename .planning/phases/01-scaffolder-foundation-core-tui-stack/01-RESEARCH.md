# Phase 1: Scaffolder Foundation + Core TUI Stack — Research

**Researched:** 2026-06-02
**Domain:** Go scaffolder CLI + charmbracelet v2 TUI template emission
**Confidence:** HIGH (verified via Context7; supplemented by local go module registry)

<user_constraints>
## User Constraints (from PROJECT.md / CLAUDE.md / STATE.md)

### Locked Decisions (from PROJECT.md constraints, restated in CLAUDE.md)
- **Tech stack:** Go 1.22+ (use 1.23 if available); built with cobra + fang + gum; consumes charmbracelet v2 libs only.
- **Distribution:** single static binary; install via `go install github.com/<org>/spin@latest`.
- **Templates:** embedded via `go:embed` (default); `--template-repo` override (Phase 2 wiring, Phase 1 file structure only).
- **Test runner:** `prism` (not `go test` directly).
- **Formatter:** `gofumpt` primary with `goimports`; fall back to `gofmt` if gofumpt not installed.
- **Hot reload:** `air` with a sensible `.air.toml`.
- **No CGO:** scaffolded projects should build with `CGO_ENABLED=0`.
- **Charm v2 only:** no v1 import paths or APIs; v2 is current; researched via context7.

### Locked Decisions (from STATE.md accumulated context)
- [Phase 1] Charm v2 only — generated projects use `charm.land/<lib>/v2` import paths; v1 paths forbidden (enforced by post-scaffold `go build` smoke test).
- [Phase 1] `go 1.25.0` floor when `--bubbles` is used, `go 1.23` otherwise; `spin` itself pins `go 1.23`.
- [Phase 1] Templates embedded via `go:embed` for offline default; `--template-repo` override available (deferred to Phase 2 wiring).
- [Phase 1] Single static binary distribution via `go install` — no runtime deps, no embedded `gum` (cross-compile complications).

### Phase 1 Scope Fences (do NOT research / build)
- `--cli` variant, `--cobra`/`--fang` for CLI projects → Phase 2
- Interactive `gum` prompts, `--no-interactive` → Phase 3
- `AGENTS.md` / `--ai` → Phase 3
- `--template-repo` external override → Phase 2
- `spin run`/`build`/`test`/`vet`/`fmt` wrappers → Phase 2
- `spin doctor`/`spin add`/`spin update` → Phase 4
- `--huh`/`--glamour`/`--glow`/`--wish`/`--log`/`--harmonica` flags → Phase 2 (only `--bubbletea`, `--bubbles`, `--lipgloss` are Phase 1)
- `--cobra`/`--fang`/`--viper`/`--module`/`--license` flags technically listed in FLAG-13..17 — wired at the flag-binding level so Phase 2 only has to fill template content (defer the actual template work; keep the flag registration skeleton).

### Project Constraints (from CLAUDE.md — verbatim directives)
- Do not import v1 charmbracelet paths (`github.com/charmbracelet/...`); use `charm.land/<lib>/v2` only.
- Pin `go 1.23` in `spin`'s own `go.mod`; `go 1.25.0` in generated `go.mod` when bubbles.
- Build with `CGO_ENABLED=0`.
- Single static binary, `go install` distribution.
- Embedded templates via `go:embed`; no remote template fetch in Phase 1.
- `prism` is the test runner (Phase 2 wires it; Phase 1 only needs `go test ./...` to pass for TOOL-05).
- `gofumpt` + `goimports` (Phase 2; Phase 1 only needs files to be gofmt-clean).
- fang v2 (`charm.land/fang/v2`) drop-in for cobra root.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SCAF-01 | `spin new <name>` produces `./<name>/` | Sections 2, 5, 6, 7, 13 |
| SCAF-02 | Rejects invalid Go module path segments | Section 7 |
| SCAF-03 | Generated project contains `go.mod`, `main.go`, `.gitignore`, `README.md`, `LICENSE` | Sections 2, 5 |
| SCAF-04 | Generated project has `git init` + initial commit | Section 13 |
| SCAF-05 | Generated project builds with `CGO_ENABLED=0 go build ./...` | Sections 4, 8, 12 |
| SCAF-06 | Runs `go build ./...` post-scaffold and reports failures clearly | Section 8 |
| SCAF-07 | `spin --help` and all subcommand help render with fang styling | Section 11 |
| SCAF-08 | Refuses to overwrite existing directory without `--force` | Section 6 |
| FLAG-01 | `--tui` selects TUI variant | Section 6 |
| FLAG-02 | `--cli` (Phase 2) — flag binding only in Phase 1 | Section 6 |
| FLAG-03 | `--all` (Phase 2) — flag binding only in Phase 1 | Section 6 |
| FLAG-04 | `--bubbletea` adds `charm.land/bubbletea/v2` | Sections 4, 5 |
| FLAG-05 | `--bubbles` adds `charm.land/bubbles/v2` | Sections 4, 5 (and bumps Go to 1.25.0 per TOOL-01) |
| FLAG-06 | `--lipgloss` adds `charm.land/lipgloss/v2` | Sections 4, 5 |
| FLAG-13..15 | `--cobra`/`--fang`/`--viper` (Phase 2) — flag binding only in Phase 1 | Section 6 |
| FLAG-16 | `--module <path>` override | Section 14 |
| FLAG-17 | `--license <type>` (MIT/Apache-2.0/none) | Section 5 (template var) |
| FLAG-18 | Unknown flags → clear error with suggestion | Section 11 (fang provides; verify) |
| TMPL-01 | `--template tui-bubbletea` | Section 5 |
| TMPL-04 | Templates embedded via `go:embed` (offline) | Section 5 |
| TMPL-05 | `_base/` + `variant_<type>/` + `lib/<name>/` overlay (last-write-wins) | Section 5 |
| TMPL-06 | Generated `main.go` is a working hello-world | Sections 2, 4 |
| TMPL-07 | Template vars: `Name`, `Module`, `Year`, `License`, `Libs []string`, `Type` | Sections 5, 6 |
| TOOL-01 | `go.mod` `go 1.25.0` when `--bubbles` used | Section 4 |
| TOOL-02 | `go.mod` `go 1.23` when no bubbles | Section 4 |
| TOOL-03 | All generated imports use `charm.land/<lib>/v2` | Section 12 (grep gate) |
| TOOL-04 | Generated project ships `.air.toml` | Section 9 |
| TOOL-05 | `spin test` integration: `spin new foo --tui --bubbletea --bubbles --lipgloss && cd foo && go build ./... && go test ./...` passes | Sections 8, 15 |
</phase_requirements>

---

## 1. Phase Boundary

### 1.1 What Phase 1 MUST deliver

| Deliverable | REQ-IDs | Verification |
|------------|---------|--------------|
| Compilable `spin` binary with `cmd/root.go` (fang-wrapped cobra) | SCAF-07, FLAG-18 | `spin --help` styled; unknown flag errors with suggestion |
| `spin new <name> --tui --bubbletea --bubbles --lipgloss` end-to-end working | SCAF-01, FLAG-01/04/05/06, TOOL-01..05 | E2E smoke: scaffold → `go build` → `go test` → `go run` |
| Embedded template tree (`go:embed` rooted at `templates/`) with overlay composition | TMPL-01, TMPL-04, TMPL-05 | `go build` includes embed; walking the FS returns expected files |
| Single `Project` struct as scaffold contract | TMPL-07, INT-05 (forward-compat) | Tests can construct a `Project` and render without flags |
| Name validation (Go module path segment) | SCAF-02 | Unit test for whitelist regex |
| Existing-directory detection + `--force` | SCAF-08 | Unit test for both branches |
| Post-scaffold `go build ./...` + `go test ./...` smoke test | SCAF-05, SCAF-06, TOOL-05 | Smoke test failure surfaces with charm/log v2 structured output |
| `.air.toml` (with `build.entrypoint`, not deprecated `build.bin`) | TOOL-04, WRAP-07 (preview) | Grep test confirms `entrypoint` present, `bin = "tmp/main"` absent |
| `Taskfile.yml` with `setup` target | TOOL-05 (setup wiring) — **defer the actual `go install` commands to Phase 2, but ship the target stub** | File exists; `task setup` is invokable (no-op or with one real install) |
| `git init` + initial commit (env-guarded) | SCAF-04 | `git log --oneline` shows 1 commit after scaffold |
| CI grep suite: `github.com/charmbracelet/`, `View() string`, `tea.KeyMsg`, `tea.WithAltScreen`, `lipgloss.NewRenderer`, `lipgloss.DefaultRenderer`, `lipgloss.AdaptiveColor{` | TOOL-03 (PITFALLS #1, #2, #3, #4) | `make grep-v1-leaks` exits 0 on generated project |
| `go.mod` with correct `go` directive (1.25.0 if bubbles, 1.23 if not) | TOOL-01, TOOL-02 | Generated project's `go.mod` parsed; line matches conditional |
| README with "Next steps" section | UX table-stakes | Generated README has `## Next steps` header |

### 1.2 What Phase 1 MUST NOT deliver (out-of-scope fences)

| Excluded | Phase | Phase-1 boundary statement |
|----------|-------|-----------------------------|
| `spin run`/`build`/`test`/`vet`/`fmt` wrappers | Phase 2 | Root cmd has only `new` subcommand; no `run`/`build`/`test`/`vet`/`fmt` files in `cmd/` |
| `--cli`/`--cobra`/`--fang` template content | Phase 2 | Flag binding exists in `Project` struct; template file `variant_cli/` is a stub TODO |
| Interactive `gum` prompts | Phase 3 | `internal/interactive/` directory does not exist; `Prompter` interface is NOT defined yet |
| `AGENTS.md` / `--ai` | Phase 3 | `templates/lib/crush.go.tmpl` and `internal/agents/` NOT created |
| `--template-repo` clone | Phase 2 | `--template-repo` flag NOT registered; default always reads from embed |
| `spin doctor`/`add`/`update` | Phase 4 | No `cmd/doctor.go`/`cmd/add.go`/`cmd/update.go` |
| `--huh`/`--glamour`/`--glow`/`--wish`/`--log`/`--harmonica` content | Phase 2 | Flags MAY be registered (forwards-compat) but their `templates/lib/<name>.go.tmpl` files are empty/stub |

### 1.3 Walking Skeleton — the thinnest valid run

The minimum set of files `spin` must emit so that `go run` produces a working bubbletea hello-world:

```
./myapp/
├── go.mod                  # module <name>; go 1.25.0; require charm.land/bubbletea/v2, charm.land/lipgloss/v2, charm.land/bubbles/v2
├── main.go                 # package main; tea.NewProgram(model).Run(); quits on ctrl+c
├── .gitignore              # tmp/, bin/, *.exe, .DS_Store
├── .air.toml               # build.entrypoint = ["./tmp/main"]; include_ext=["go"]; exclude_dir=["tmp","bin"]
├── Taskfile.yml            # setup/test/build/run/fmt tasks (one stub task each; setup runs gofumpt+goimports+air+prism installs)
├── README.md               # title, Next steps section
├── LICENSE                 # MIT or Apache-2.0 text
├── internal/
│   └── ui/
│       └── styles.go       # lipgloss v2 styles (NewStyle only — no Renderer)
└── .git/                   # init + 1 commit
```

That's 7 source files + 1 hidden + git history. Everything else (`LICENSE` types beyond default, `Makefile` alt, `.golangci.yml`, `.goreleaser.yaml`) is Phase 2 or optional.

---

## 2. Stack & Versions (scaffolder itself)

Pin these in `spin`'s own `go.mod`. All versions verified via `ctx7` Context7 docs on 2026-06-02 and `go list -m -versions`.

| Library | Module path | Pin | Why | Verification |
|---------|-------------|-----|-----|--------------|
| Cobra | `github.com/spf13/cobra` | `v1.9.1` | fang v2 requires cobra ≥ v1.9; latest stable pre-Phase-1 | `go list -m` shows v1.9.1 published; v1.10.x exists but v1.9.1 is the fang-tested floor |
| Fang | `charm.land/fang/v2` | latest stable (`v0.x.y` — verify at scaffold time) | Drop-in `fang.Execute(ctx, rootCmd)` for styled help; needs cobra v1.9+ | Context7 doc shows `fang.Execute(context.Background(), cmd)` pattern |
| Lip Gloss | `charm.land/lipgloss/v2` | latest stable (v2.0.0-beta.2 line per STACK.md) | Style scaffolder output ("Created at ./foo") — dogfooding | Context7 confirms `charm.land/lipgloss/v2` import |
| Huh | `charm.land/huh/v2` | NOT imported in Phase 1 (no Prompter interface yet) | Will land in Phase 3 | Context7 confirms v2 import path |
| Log | `charm.land/log/v2` | latest stable (v2.0.0) | Scaffolder logging: `log.Info("created", "path", ...)` | Context7: `log.SetDefault`, `log.NewWithOptions` API |
| isatty | `github.com/mattn/go-isatty` | latest | Guard TTY assumptions in scaffolds even if `--no-interactive` isn't wired yet | Pre-emptive for Phase 3 prep; needed for SCAF-08 verbose check |
| charmbracelet/x ansi | `github.com/charmbracelet/x/ansi` | latest | Low-level ANSI escape if lipgloss v2 doesn't cover a case (unlikely in Phase 1) | Optional; defer unless used |

**`go` directive in `spin`'s own `go.mod`:** `go 1.23` (per CLAUDE.md / STATE.md). `spin` does not import bubbles, so it does not need 1.25.

**Distribution pin (for GoReleaser / `go install`):** `github.com/<org>/spin/v2` is the suggested vanity module path. `<org>` is TBD by user (e.g., `github.com/charmbracelet/spin` or `github.com/<user>/spin`). For Phase 1, document the install command but don't ship to a registry.

**`gofumpt` / `goimports` / `air` / `prism` / `goreleaser`:** these are dev tools for **end users**, not for `spin` itself. `spin` only needs the templates that *reference* them.

---

## 3. Stack & Versions (scaffolded TUI project)

The exact pins that go into the generated `go.mod` for the `tui-bubbletea` template. All v2 vanity paths verified via Context7.

| Library | Module path | Pin | Go floor (per lib) | Effect on generated `go.mod` |
|---------|-------------|-----|--------------------|------------------------------|
| Bubble Tea | `charm.land/bubbletea/v2` | `v2.0.0` | (transitive) | Required when `--bubbletea` is set |
| Lip Gloss | `charm.land/lipgloss/v2` | `v2.0.0-beta.2` line | (transitive) | Required when `--lipgloss` is set |
| Bubbles | `charm.land/bubbles/v2` | `v2.0.0` | **1.25.0** (per bubbles README) | Bumps generated `go.mod` `go` directive to `1.25.0` |
| go-runewidth | `github.com/mattn/go-runewidth` | latest | none | Transitive of lipgloss; rarely direct dep |
| charmbracelet/x | `github.com/charmbracelet/x` | (not pinned in Phase 1) | — | Not used in hello-world template |

**`go` directive decision matrix** (for the generated `go.mod`, evaluated in order):

| Flag combination | `go` directive | Source |
|------------------|----------------|--------|
| `--bubbles` OR (`--huh` Phase 2) | `1.25.0` | bubbles v2 docs require 1.25.0 |
| `--bubbletea` only OR `--lipgloss` only OR no TUI libs | `1.23` | STATE.md / CLAUDE.md |
| `--cobra`/`--fang` (Phase 2) | `1.23` | fang v2 declares `go 1.25.0` in its own go.mod — **OPEN DECISION** (see §15) |

**Pinning policy:** use exact `vX.Y.Z` versions, NOT `latest` (PITFALL #8). A version matrix struct in `internal/scaffold/versions.go` keeps the pins in one place. Example:

```go
// internal/scaffold/versions.go
type CharmPins struct {
    Bubbletea string // "v2.0.0"
    Lipgloss  string // "v2.0.0-beta.2"
    Bubbles   string // "v2.0.0"
    Log       string // "v2.0.0"
}

var DefaultTUI = CharmPins{
    Bubbletea: "v2.0.0",
    Lipgloss:  "v2.0.0-beta.2",
    Bubbles:   "v2.0.0",
    Log:       "v2.0.0",
}
```

**CGO contract:** every generated `go.mod` has `// +build` comment absent; charm v2 stack is pure Go. CI smoke test: `CGO_ENABLED=0 go build ./...` from the generated project root. The test is part of the post-scaffold hook in §8.

---

## 4. Template Engine Design

### 4.1 Embed FS layout

Root the `go:embed` at `templates/`. Use `//go:embed all:templates` (not `templates/*`) so hidden files (e.g., `.air.toml`, `.gitignore`) are included.

```
templates/
├── _base/                       # always rendered; last-write-wins = lowest precedence
│   ├── go.mod.tmpl
│   ├── README.md.tmpl
│   ├── .air.toml.tmpl
│   ├── .gitignore.tmpl
│   ├── Taskfile.yml.tmpl
│   ├── LICENSE-MIT.tmpl         # (gated on License=="mit")
│   ├── LICENSE-Apache-2.0.tmpl  # (gated on License=="apache-2.0")
│   └── internal/
│       └── ui/
│           └── styles.go.tmpl   # placeholder, replaced by lib/lipgloss overlay
├── variant_tui/                 # --tui
│   └── main.go.tmpl
├── variant_cli/                 # --cli (Phase 2 — Phase 1 has stub)
│   └── main.go.tmpl             # TODO marker; will be filled in Phase 2
├── variant_all/                 # --all (Phase 2)
│   └── main.go.tmpl             # TODO
├── lib/
│   ├── bubbletea.go.tmpl        # main wiring in main.go (Update/Init/View)
│   ├── bubbles.go.tmpl          # imports a spinner, list, or viewport
│   ├── lipgloss.go.tmpl         # defines the styles in internal/ui/styles.go
│   ├── huh.go.tmpl              # Phase 2: stub
│   ├── glamour.go.tmpl          # Phase 2: stub
│   ├── wish.go.tmpl             # Phase 2: stub
│   ├── log.go.tmpl              # Phase 2: stub
│   └── cobra.go.tmpl            # Phase 2: stub
```

**Note on `_base/internal/ui/styles.go.tmpl`:** This is intentional. The base file is a no-op (`package ui` + `// ...` comment). The `lib/lipgloss.go.tmpl` overwrites it with real styles. If `--lipgloss` is omitted, the no-op file ships, and the import statement in `main.go` is conditional via template `{{if}}` — but the simpler design is: the `lib/lipgloss.go.tmpl` decides whether to emit an import. If `--lipgloss` is false, that overlay does not contribute a file, and `main.go` references styles only via `{{if .Libs includes "lipgloss"}}` guards.

### 4.2 Overlay merge order (last-write-wins)

```go
// internal/scaffold/template.go
func (p *Project) overlayOrder() []string {
    layers := []string{"_base"}
    if p.Type != "" {
        layers = append(layers, "variant_"+p.Type)
    }
    for _, lib := range p.Libs {
        layers = append(layers, "lib/"+lib)
    }
    return layers
}
```

Walk the embed FS once, collect all relative paths, then for each path iterate layers in order and pick the last existing file. Render that file through `text/template` with `p` as data.

**Anti-patterns to avoid** (PITFALLS #5):
- **Never** use `t.Execute` — always `t.ExecuteTemplate(w, name, data)` with an explicit name.
- **Never** name two templates with the same basename (e.g., `main.go.tmpl` everywhere). The `lib/<name>.go.tmpl` naming scheme keeps base filenames unique.
- **Always** register `FuncMap` *before* `Parse`/`ParseFS`.
- Set `t = t.Option("missingkey=error")` in dev builds; switch to `"zero"` in production.

### 4.3 FuncMap (template helpers)

Registered in this order; `Funcs()` before `Parse`:

| Function | Signature | Used for |
|----------|-----------|----------|
| `title` | `func(string) string` | `"myapp"` → `"Myapp"` (display) |
| `upper` | `func(string) string` | binary name uppercase |
| `join` | `func([]string, string) string` | `range .Libs` joined for `go.mod` |
| `quote` | `func(string) string` | Go-quoted strings |
| `currentYear` | `func() int` | License header year (TODL: pass `{{.Year}}` instead — see below) |
| `licenseHeader` | `func(license string) string` | emits correct boilerplate per license type |
| `modulePath` | `func(p *Project) string` | returns the resolved module path (handles `--module` override) |
| `imports` | `func(p *Project) string` | emits the `import (...)` block for the active libs (skips empty) |
| `goVersion` | `func(p *Project) string` | returns `"1.25.0"` or `"1.23"` based on `.Libs` |
| `gofumptPath` | `func() string` | returns absolute path of `gofumpt` binary if on `$PATH`, else `""` (for `Taskfile.yml` ifs) |

**Defer to Phase 2:** any helper that requires resolving a tool on `$PATH` for the scaffolder (e.g., `airPath` for the generated `.air.toml`). In Phase 1, the generated `.air.toml` always uses the literal `air` binary name; the user must have it on `$PATH`.

**Year source:** prefer `{{.Year}}` over a func. The `Project` struct fills `Year: time.Now().Year()` at scaffold time. More deterministic in tests.

### 4.4 Template vars per `Project` (TMPL-07)

```go
// internal/scaffold/project.go
type Project struct {
    Name      string   // e.g. "myapp"
    Module    string   // e.g. "github.com/<user>/myapp" or override from --module
    Type      string   // "tui" | "cli" | "all"  (Phase 2 adds "cli" and "all")
    Libs      []string // ["bubbletea", "bubbles", "lipgloss"]; deterministic order
    Template  string   // e.g. "tui-bubbletea"
    License   string   // "mit" | "apache-2.0" | "none"
    Year      int      // e.g. 2026
    Force     bool     // --force: overwrite existing dir
    NoGit     bool     // --no-git: skip git init (for tests)
    Quiet     bool     // --quiet: minimal output
    SpinVer   string   // "0.1.0" — emitted in `// generated by spin X.Y.Z` markers
    Viper     bool     // Phase 2 (flag binding only in Phase 1)
    Cobra     bool     // Phase 2
    Fang      bool     // Phase 2
    AI        bool     // Phase 3
    Huh       bool     // Phase 2
    Glamour   bool     // Phase 2
    Glow      bool     // Phase 2
    Wish      bool     // Phase 2
    Log       bool     // Phase 2
    Harmonica bool     // Phase 2
    Modifiers bool     // Phase 2
    Ansi      bool     // Phase 2
    Runewidth bool     // Phase 2
}
```

All Phase-2/3/4 boolean flags are present on the struct (zero-value `false`) so the template engine and flag binding don't change when later phases add content. Only their `.tmpl` overlays are empty in Phase 1.

### 4.5 File emission: chmod + perms

Generated shell scripts (none in Phase 1, but Taskfile's `setup` target may run `go install` which doesn't need +x) — keep `0644` for everything in Phase 1. Phase 2 may emit `bin/<name>` hooks.

---

## 5. Flag & Project Struct

### 5.1 Single `Project` struct (single source of truth)

`internal/scaffold/project.go` defines `Project` (above). The `cmd/new.go` `RunE` does:
1. `p, err := scaffold.ResolveFlags(cmd, args)` — populate from cobra flag values.
2. `if err := p.Validate(); err != nil { return err }` — name regex, dir conflict, unknown-flag suggestion (the latter is automatic via fang).
3. `if !p.NoInteractive { /* Phase 3: call prompter */ }` — Phase 1 leaves this branch as a no-op (no `--no-interactive` flag yet).
4. `return scaffold.New(p)` — the main entrypoint.

### 5.2 Cobra flag binding (`cmd/new.go`)

```go
// cmd/new.go
var newCmd = &cobra.Command{
    Use:   "new <name>",
    Short: "Scaffold a new charmbracelet project",
    Args:  cobra.ExactArgs(1),
    RunE:  runNew,
}

func init() {
    rootCmd.AddCommand(newCmd)

    // Phase 1 active
    pf := newCmd.PersistentFlags()
    pf.String("module", "", "override default module path")
    pf.String("license", "mit", "license type: mit, apache-2.0, none")
    pf.String("template", "tui-bubbletea", "template name (default: tui-bubbletea)")
    pf.Bool("force", false, "overwrite existing directory")
    pf.Bool("tui", false, "TUI project variant (default if no --cli)")
    pf.Bool("bubbletea", false, "add bubbletea v2")
    pf.Bool("bubbles", false, "add bubbles v2 (implies --bubbletea; bumps go.mod to 1.25.0)")
    pf.Bool("lipgloss", false, "add lipgloss v2")

    // Phase 2+ forward-compat flag registration (no behavior)
    pf.Bool("cli", false, "CLI project variant (Phase 2)")
    pf.Bool("all", false, "TUI + CLI combo (Phase 2)")
    pf.Bool("cobra", false, "add cobra (Phase 2)")
    pf.Bool("fang", false, "add fang (Phase 2)")
    pf.Bool("viper", false, "add viper (Phase 2)")
    pf.Bool("huh", false, "add huh v2 (Phase 2)")
    pf.Bool("glamour", false, "add glamour v2 (Phase 2)")
    pf.Bool("glow", false, "add glow binary (Phase 2)")
    pf.Bool("wish", false, "add wish v2 (Phase 2)")
    pf.Bool("log", false, "add charm log v2 (Phase 2)")
    pf.Bool("harmonica", false, "add harmonica v2 (Phase 2)")
    pf.Bool("modifiers", false, "add x/modifiers (Phase 2)")
    pf.Bool("ansi", false, "add x/ansi (Phase 2)")
    pf.Bool("runewidth", false, "add go-runewidth (Phase 2)")
}
```

**Cobra validation rule:** `--bubbles` implies `--bubbletea` (bubbles is a layer on bubbletea). Implement in `ResolveFlags` (not cobra's `MarkFlagsRequiredTogether` — bubbles implies a stronger constraint than mutual requirement).

### 5.3 `ResolveFlags` signature

```go
// internal/scaffold/resolve.go
func ResolveFlags(cmd *cobra.Command, args []string) (*Project, error) {
    name := args[0]
    p := &Project{Name: name}
    p.Module = mustString(cmd, "module")  // helper
    p.License = mustString(cmd, "license")
    p.Template = mustString(cmd, "template")
    p.Force = mustBool(cmd, "force")
    // ... bind every flag to the struct ...
    if p.Module == "" { p.Module = p.Name }
    // --bubbles implies --bubbletea
    if contains(p.Libs, "bubbles") && !contains(p.Libs, "bubbletea") {
        p.Libs = append(p.Libs, "bubbletea")
    }
    p.Year = time.Now().Year()
    p.SpinVer = version.Version  // from internal/version via ldflags
    return p, nil
}
```

### 5.4 fang wiring (SCAF-07, §11)

`main.go` is a 5-line file:

```go
package main

import (
    "context"
    "os"
    "charm.land/fang/v2"
    "github.com/<org>/spin/cmd"
)

func main() {
    if err := fang.Execute(context.Background(), cmd.RootCmd()); err != nil {
        os.Exit(1)
    }
}
```

`cmd.RootCmd()` returns the singleton `*cobra.Command` constructed once (with all subcommands attached via `init()` in each subcommand file).

### 5.5 `cmd/root.go`

Provides a `RootCmd()` constructor (not a package-level var, to keep testability):

```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/<org>/spin/internal/version"
)

func RootCmd() *cobra.Command {
    root := &cobra.Command{
        Use:     "spin",
        Short:   "Scaffold a charmbracelet v2 Go project",
        Long:    "...",
        Version: version.Version,  // fang picks this up; also `--version` flag
    }
    // cobra.OnInitialize, persistent flags, etc.
    return root
}
```

### 5.6 `--force` / existing-dir check (SCAF-08)

`internal/scaffold/validate.go`:

```go
func (p *Project) Validate() error {
    if !IsValidGoModuleSegment(p.Name) {
        return fmt.Errorf("invalid project name %q: must match %s", p.Name, ModuleSegmentRegex)
    }
    target := filepath.Join(".", p.Name)
    if _, err := os.Stat(target); err == nil {
        if !p.Force {
            return fmt.Errorf("directory %q already exists; pass --force to overwrite", target)
        }
        // with --force: refuse to overwrite a non-empty dir unless it's a git repo we just init'd
        // (CWD is the user's responsibility; if they're forcing, we proceed)
    }
    return nil
}
```

---

## 6. Name Validation (SCAF-02)

Go module path segments: lowercase letters, digits, hyphens, underscores, dots. But for the project *name* (the directory and the binary name), the practical rules are stricter:

**Whitelist regex (Go RE2 syntax):**

```go
// internal/scaffold/validate.go
var ModuleSegmentRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,61}[a-z0-9]$`)

func IsValidGoModuleSegment(s string) bool {
    if len(s) < 2 || len(s) > 62 { return false }
    if !ModuleSegmentRegex.MatchString(s) { return false }
    if strings.Contains(s, "..") { return false }  // prevent path traversal
    if strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") { return false }
    if strings.HasPrefix(s, "-") { return false }    // flag-like names are confusing
    // Reserved words (Go keywords + common tooling)
    reserved := map[string]bool{
        "test": true, "tests": true, "_test": true,
        "vendor": true, "internal": true, "cmd": true,
        "go": true, "golang": true,
    }
    if reserved[s] { return false }
    return true
}
```

**Test cases (must pass):**

| Input | Verdict | Reason |
|-------|---------|--------|
| `myapp` | valid | — |
| `my-app` | valid | — |
| `my_app` | valid | — |
| `my.app` | valid | — |
| `MyApp` | invalid | has uppercase |
| `-myapp` | invalid | starts with `-` |
| `myapp-` | invalid | ends with `-` |
| `test` | invalid | reserved |
| `internal` | invalid | reserved |
| `..` | invalid | path traversal |
| `.hidden` | invalid | starts with `.` |
| `a` | invalid | too short |
| `` (empty) | invalid | empty |
| `myapp/../etc` | invalid | contains `/` |
| 64-char string | invalid | too long |

**Error message format:**

```
error: invalid project name "MyApp"
  names must be lowercase, 2–62 characters, start/end with letter or digit,
  contain only [a-z0-9._-], and not be a Go-reserved word
  example: spin new myapp --tui --bubbletea
```

---

## 7. Post-Scaffold Smoke Test (SCAF-05, SCAF-06, TOOL-05)

The `go build ./...` + `go test ./...` run is the most important guarantee in Phase 1 — the entire "perfect first run" value prop depends on it.

### 7.1 Hook location

`internal/scaffold/hooks.go`:

```go
// internal/scaffold/hooks.go
func (p *Project) VerifyBuild(log *log.Logger) error {
    log.Info("verifying build", "path", p.Name)

    // 1. cd into the project
    // 2. run `go build ./...` with CGO_ENABLED=0
    // 3. if exit != 0: print stderr verbatim, return wrapped error
    // 4. run `go test ./...`
    // 5. if exit != 0: same treatment
    // 6. return nil

    return runSmoke(p, log)
}

func runSmoke(p *Project, log *log.Logger) error {
    root := filepath.Join(".", p.Name)

    for _, step := range []struct{ name string; env []string; args []string }{
        {"build", []string{"CGO_ENABLED=0"}, []string{"go", "build", "./..."}},
        {"test", nil, []string{"go", "test", "./..."}},
    } {
        cmd := exec.Command(step.args[0], step.args[1:]...)
        cmd.Dir = root
        cmd.Env = append(os.Environ(), step.env...)
        out, err := cmd.CombinedOutput()
        if err != nil {
            log.Error("smoke test failed", "step", step.name, "err", err, "output", string(out))
            return fmt.Errorf("`%s` failed in %s:\n%s", strings.Join(step.args, " "), root, out)
        }
    }
    log.Info("smoke test passed", "path", p.Name)
    return nil
}
```

### 7.2 Failure surfacing

Use `charm.land/log/v2`'s structured logging:

```go
log.Error("post-scaffold build failed",
    "project", p.Name,
    "module", p.Module,
    "libs", p.Libs,
    "stderr", stderr,
    "exit_code", exitCode,
)
```

The error message printed to the user's terminal must include the actual `go build` output (so they can see the v1 import leak, missing module, etc.). Format:

```
error: `go build ./...` failed in ./myapp:

github.com/charmbracelet/bubbletea: module not found
hint: did you accidentally use a v1 import path? spin requires `charm.land/bubbletea/v2`

the project was scaffolded at ./myapp; inspect and fix, or remove and re-run.
```

### 7.3 What "smoke test" catches

| Failure mode | Caught by |
|--------------|-----------|
| v1 import path in template | `go build` fails: module not found |
| Wrong v2 version pin | `go build` fails: version not available |
| Missing dep in go.mod | `go build` fails: unresolved import |
| `go.mod` `go` directive too low (e.g., 1.23 when bubbles requires 1.25) | `go build` fails: toolchain directive |
| `View() string` in v2 template | `go build` fails: cannot use m.View() (string) as tea.View |
| `lipgloss.NewRenderer` in v2 | `go build` fails: undefined |
| `tea.WithAltScreen` | `go build` fails: undefined |
| Missing `tea.Quit` return in `Update` | `go build` fails or warns (compiler-enforced) |

**NOT caught by smoke test** (caught by CI grep suite, §12):
- Hard-coded v1-looking code that happens to compile (e.g., `lipgloss.Color("...")` works in v1 and v2, but the semantic is different)
- Deprecated `bin` field in `.air.toml` (no compile-time check)
- Bubble Tea program running in non-TTY (no compile-time check)

### 7.4 Skip flag

A `--no-verify` (or env var `SPIN_NO_VERIFY=1`) escape hatch for power users who want to skip the smoke test. Not required by REQ-IDs but a 1-line addition. Recommended to add for testability.

---

## 8. `.air.toml` Schema (TOOL-04, PITFALL #10)

**Critical:** `build.bin` is deprecated. Use `build.entrypoint = ["./tmp/main"]`. Include a comment block at the top documenting this.

```toml
# .air.toml — generated by spin
# Docs: https://github.com/air-verse/air
# Note: `build.entrypoint` is the modern field; `build.bin` is deprecated.
# Run with: air

root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/main ."
entrypoint = ["./tmp/main"]           # preferred over deprecated `bin`
args_bin = []

include_ext = ["go", "tpl", "tmpl", "html", "css", "js", "json", "yaml", "toml"]
exclude_dir = ["assets", "tmp", "vendor", "node_modules", ".git", "bin"]
exclude_unchanged = true
exclude_regex = ["_test\\.go$", ".*_mock\\.go$", ".*\\.generated\\.go$"]

[log]
level = "info"

[misc]
clean_on_exit = true
```

**Verified via Context7** (`/air-verse/air`):

```toml
# Source: https://github.com/air-verse/air/blob/master/README.md
[build]
entrypoint = ["./tmp/main"]
args_bin = ["server", ":8080"]
```

The schema fields verified:
- `build.cmd` — build command
- `build.entrypoint` — binary path (preferred)
- `build.args_bin` — args to binary
- `build.pre_cmd` / `post_cmd` — pre/post build hooks
- `build.include_ext` / `exclude_dir` / `exclude_regex` / `exclude_unchanged` — watcher filters
- `misc.clean_on_exit` — remove `tmp_dir` on exit

**No `build.bin`** in the generated file. This is a hard requirement and a CI grep gate (§12).

---

## 9. `Taskfile.yml` setup target (TOOL-05 + success criterion 3)

**Phase 1 stance:** ship a `Taskfile.yml` with a `setup` target that **exists and is invokable** (per success criterion 3 of the roadmap). Wire the actual `go install` commands for gofumpt + goimports + air + prism **in Phase 1** so the `setup` target is *useful* (not just a stub). The `WRAP-08` requirement is in Phase 2, but the file itself + the install chain are de-facto Phase 1 deliverables because TOOL-05 success criterion 3 requires a `Taskfile.yml` (or `Makefile`) with a `setup` target.

**The decision is:** does Phase 1 ship the `setup` target with installs, or only the file structure?

**Recommendation (preferred):** ship the installs. Reason: success criterion 3 says "contains a working `Taskfile.yml` ... with a `setup` target." A `setup` target that is empty doesn't satisfy "working." The `WRAP-08` text in REQUIREMENTS.md is just the formal ID; the spirit is captured by the success criterion. The actual tool-detection and fallback chain is Phase 2's job — but the `go install` calls themselves are static text in the template and easy to emit now.

```yaml
# Taskfile.yml — generated by spin
# https://taskfile.dev

version: '3'

vars:
  NAME: '{{.NAME | default "myapp"}}'
  BIN:  ./bin/{{.NAME}}

env:
  CGO_ENABLED: '0'

tasks:
  default:
    cmds:
      - task: build

  setup:
    desc: Install gofumpt, goimports, air, and prism
    cmds:
      - go install mvdan.cc/gofumpt@latest
      - go install golang.org/x/tools/cmd/goimports@latest
      - go install github.com/air-verse/air@latest
      - go install go.dalton.dog/prism@latest

  build:
    desc: Build the binary
    cmds:
      - go build -o {{.BIN}} .

  run:
    desc: Run with hot reload (requires `task setup`)
    cmds:
      - air

  test:
    desc: Run tests (uses prism if installed, else go test)
    cmds:
      - |
        if command -v prism >/dev/null 2>&1; then
          prism go test ./...
        else
          go test ./...
        fi

  fmt:
    desc: Format with gofumpt + goimports
    cmds:
      - test -x "$(go env GOPATH)/bin/gofumpt" && gofumpt -l -w . || gofmt -l -w .
      - test -x "$(go env GOPATH)/bin/goimports" && goimports -w . || true

  vet:
    cmds:
      - go vet ./...

  clean:
    cmds:
      - rm -rf ./bin ./tmp
```

**Alternative: ship Makefile alongside Taskfile** — Phase 1 may ship only `Taskfile.yml`. Add `Makefile` as a `--makefile` opt-in in Phase 2.

**`gofumpt` Go 1.25+ requirement** (PITFALL #11): the `go install mvdan.cc/gofumpt@latest` call requires Go 1.25+ on the user's machine. Document this in the README's "Prerequisites" section. Alternative: pin to a version of gofumpt that works with older Go (e.g., `mvdan.cc/gofumpt@v0.6.0`). **Open decision** (see §15).

---

## 10. fang Wiring (SCAF-07, FLAG-18)

Verified via Context7 (`/charmbracelet/fang`):

```go
// Source: https://context7.com/charmbracelet/fang/llms.txt
// Package: charm.land/fang/v2

package main

import (
    "context"
    "os"
    "charm.land/fang/v2"
    "github.com/spf13/cobra"
)

func main() {
    cmd := &cobra.Command{ /* ... */ }
    if err := fang.Execute(context.Background(), cmd); err != nil {
        os.Exit(1)
    }
}
```

**What fang v2 provides automatically:**
- Styled `--help` output (charm-themed; lipgloss v2 under the hood)
- Styled error output for unknown flags
- `--version` flag with custom theming
- Shell completion generation (`spin completion bash|zsh|fish|powershell`)
- Manpage generation (`--generate-manpage`)

**What it does NOT provide:**
- Unknown-flag *suggestion* (closest-match typo) — this is cobra's `SuggestFloats` or `FParseErrWhitelist.UnknownFlags` behavior. With fang enabled, cobra's defaults still apply. To enable typo suggestions explicitly, set `rootCmd.SuggestionsMinimumDistance = 2` (cobra built-in).

**Interaction with `--force` and `cobra.ExactArgs`:**
- `newCmd.Args = cobra.ExactArgs(1)` — cobra enforces exactly 1 positional arg. fang inherits this and styles the error.
- `cobra.Command.SilenceUsage = true` on `newCmd` so the error is clean and doesn't dump usage on a validation failure.

**RootCmd `Use` / `Short` / `Long` for fang:**
```go
root := &cobra.Command{
    Use:     "spin",
    Short:   "Scaffold a charmbracelet v2 Go project",
    Long:    "spin is a Go project scaffolder for the charmbracelet v2 ecosystem.\n\n" +
             "It generates ready-to-run Go projects — TUI apps, CLI tools, or both — " +
             "pre-wired with the right charmbracelet libraries, modern Go tooling " +
             "(cobra, fang, gum), hot reload (air), and the prism test runner.",
    Version: version.Version,
}
```

---

## 11. CI Grep Test for v1 Leaks (PITFALLS #1, #2, #3, #4)

A standalone script (in `scripts/check-v1-leaks.sh` or a Go test) that greps the generated project for forbidden v1 patterns. Must be a `make grep-v1-leaks` target in spin's own `Taskfile.yml` and run as part of `verify-work` and CI.

### 11.1 Patterns to grep

| Pattern | Source pitfall | What it catches |
|---------|----------------|-----------------|
| `github.com/charmbracelet/` | PITFALL #1 | Any v1 import path leaked into generated code |
| `View() string` | PITFALL #3 | v1 `View` signature (should be `View() tea.View`) |
| `tea.KeyMsg` (used as a type, not interface) | PITFALL #2 | v1 struct KeyMsg (in v2 it's an interface) — note: in v2, `tea.KeyMsg` IS the interface; this grep is conservative |
| `tea.WithAltScreen` | PITFALL #2 | v1 program option |
| `tea.WithMouseCellMotion` | PITFALL #2 | v1 program option |
| `tea.EnterAltScreen` / `tea.HideCursor` / `tea.ExitAltScreen` | PITFALL #2 | v1 commands removed in v2 |
| `lipgloss.NewRenderer` | PITFALL #4 | v1 Renderer (removed in v2) |
| `lipgloss.DefaultRenderer` | PITFALL #4 | v1 (removed) |
| `lipgloss.SetDefaultRenderer` | PITFALL #4 | v1 (removed) |
| `lipgloss.AdaptiveColor{` | PITFALL #4 | v1 type (moved to compat) |
| `lipgloss.ColorProfile()` | PITFALL #4 | v1 |
| `lipgloss.HasDarkBackground()` (no args) | PITFALL #4 | v1 signature |
| `tea.KeyCtrlC` | PITFALL #2 | v1 constant |
| `tea.MouseButtonLeft` / `MouseButtonRight` / `MouseButtonMiddle` | PITFALL #2 | v1 constants (v2 = `MouseLeft`, `MouseRight`, `MouseMiddle`) |
| `case " ":` | PITFALL #2 | v1 space-bar handling (in v2, `msg.String() == "space"`) |
| `bin = "tmp/main"` in `.air.toml` | PITFALL #10 | air `build.bin` deprecated |

**Refinement for `tea.KeyMsg`:** in v2, `tea.KeyMsg` is the **interface** (returned by `msg.(type)` to discriminate `KeyPressMsg` vs `KeyReleaseMsg`). The forbidden pattern is `case tea.KeyMsg:` when `msg` is then used as a struct (e.g., `msg.Type`, `msg.Runes`). A practical check: forbid `case tea.KeyMsg:` if the same file contains `msg.Type` or `msg.Runes` or `msg.Alt`. Simpler: forbid `case tea.KeyMsg:` *combined with* `msg.Type` or `msg.Runes` in the same file. The cleanest version is to forbid `msg.Type`, `msg.Runes`, `msg.Alt` outright — those fields don't exist on any v2 type.

### 11.2 Implementation

A `bash` script is fine for Phase 1 (avoid Go-ception):

```bash
#!/usr/bin/env bash
# scripts/check-v1-leaks.sh
# Greps a scaffolded project for v1 charmbracelet API leaks.
# Usage: check-v1-leaks.sh <project-dir>

set -e
ROOT="${1:-./myapp}"
if [[ ! -d "$ROOT" ]]; then
  echo "error: directory '$ROOT' does not exist" >&2
  exit 1
fi

PATTERNS=(
  'github\.com/charmbracelet/'
  'View\(\) string'
  'tea\.WithAltScreen'
  'tea\.WithMouseCellMotion'
  'tea\.EnterAltScreen'
  'tea\.HideCursor'
  'tea\.ExitAltScreen'
  'lipgloss\.NewRenderer'
  'lipgloss\.DefaultRenderer'
  'lipgloss\.SetDefaultRenderer'
  'lipgloss\.AdaptiveColor\{'
  'lipgloss\.ColorProfile\('
  'lipgloss\.HasDarkBackground\(\)'
  'tea\.KeyCtrlC'
  'tea\.MouseButtonLeft'
  'tea\.MouseButtonRight'
  'tea\.MouseButtonMiddle'
  'msg\.Type'
  'msg\.Runes'
  'msg\.Alt'
  'msg\.X'
  'msg\.Y'
)

# .air.toml: forbid `bin = "tmp/main"`
AIR_PATTERNS=(
  'bin\s*=\s*"tmp/main"'
)

FAIL=0
for pat in "${PATTERNS[@]}"; do
  if grep -rE --include='*.go' --include='*.tmpl' "$pat" "$ROOT" 2>/dev/null; then
    echo "FAIL: v1 pattern matched: $pat" >&2
    FAIL=1
  fi
done

for pat in "${AIR_PATTERNS[@]}"; do
  if grep -E "$pat" "$ROOT/.air.toml" 2>/dev/null; then
    echo "FAIL: deprecated air pattern: $pat" >&2
    FAIL=1
  fi
done

if [[ $FAIL -ne 0 ]]; then
  exit 1
fi
echo "OK: no v1 leaks detected in $ROOT"
```

Wire it as `task grep-v1-leaks` in spin's own `Taskfile.yml` and call it from the integration test that scaffolds a project and verifies it's clean.

**Refinement:** the `msg.Type` / `msg.Runes` / `msg.Alt` patterns are noisy because they could match unrelated structs named `msg` in tests. For Phase 1, accept the noise and refine in Phase 2 by scoping to Bubble Tea files only. Or just forbid them — if someone has a struct field named `Type` in a `msg` variable, they can rename it.

---

## 12. Git Init + Initial Commit (SCAF-04)

### 12.1 The dance

```go
// internal/scaffold/git.go
func (p *Project) GitInit(log *log.Logger) error {
    if p.NoGit {
        return nil
    }
    root := filepath.Join(".", p.Name)

    // 1. git init
    if err := runGit(root, "init", "-b", "main"); err != nil {
        return fmt.Errorf("git init: %w", err)
    }
    // 2. git add .
    if err := runGit(root, "add", "."); err != nil {
        return fmt.Errorf("git add: %w", err)
    }
    // 3. git commit -m "..."
    msg := fmt.Sprintf("scaffold %s with spin %s", p.Name, p.SpinVer)
    if err := runGit(root, "commit", "-m", msg); err != nil {
        return fmt.Errorf("git commit: %w", err)
    }
    return nil
}

func runGit(dir string, args ...string) error {
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    // env-guard: never prompt for credentials
    cmd.Env = append(os.Environ(),
        "GIT_TERMINAL_PROMPT=0",
        "GIT_AUTHOR_NAME=spin",
        "GIT_AUTHOR_EMAIL=spin@localhost",
        "GIT_COMMITTER_NAME=spin",
        "GIT_COMMITTER_EMAIL=spin@localhost",
    )
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("%s: %s", strings.Join(args, " "), out)
    }
    return nil
}
```

### 12.2 Edge cases

| Case | Handling |
|------|----------|
| `git` not on `$PATH` | `runGit` fails; log a warning and continue (don't fail the scaffold) — user can `git init` themselves |
| User has git `user.name`/`user.email` set | env-guard overrides with `spin@localhost` for determinism |
| User is on a branch-protected repo | `git init` is local-only; no remote; no protection issue |
| `--no-git` flag | skip the whole dance; the directory is left without `.git/` |
| Pre-commit hook failure | unlikely for an empty repo, but if it happens, the error is surfaced (not silent) |

### 12.3 Why env-guard

`GIT_TERMINAL_PROMPT=0` prevents git from blocking on a credential prompt in CI or scripted environments. Setting `GIT_AUTHOR_*` overrides prevents the scaffolder from failing when the user has no global git identity configured (common in fresh CI containers).

### 12.4 Default branch name

`-b main` is the modern default (was `master` historically). Some git versions may not support `-b`; fall back to `git init` then `git symbolic-ref HEAD refs/heads/main` for git < 2.28.

---

## 13. Module Path Resolution (FLAG-16)

### 13.1 Default behavior

If `--module` is not provided, the default module path is the **project name** (no GitHub prefix). Reason: `spin` doesn't know the user's GitHub username; guessing is worse than letting `go mod` resolve it.

```go
// internal/scaffold/resolve.go
if p.Module == "" {
    p.Module = p.Name  // e.g., "myapp" — user can run `go mod edit -module=...` later
}
```

**Trade-off:** the user has to manually `go mod edit -module github.com/<user>/<name>` before publishing. Alternative: prompt via gum in Phase 3 for "GitHub username?" and default to `<username>/<name>`. **Open decision** (see §15).

### 13.2 `--module <path>` override

Accepts any valid Go module path (not just segment). The regex is more permissive:

```go
var ModulePathRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*[a-z0-9]$`)
// allow / for sub-paths, e.g. github.com/foo/bar/baz
```

Validation:
- Must not start with `..` (no path traversal)
- Must contain at least one valid segment
- Each segment must match `ModuleSegmentRegex` (above)

### 13.3 Sub-path emission

If `--module github.com/foo/bar/baz` is passed:
- `go.mod` `module github.com/foo/foo/baz` line uses the full path verbatim
- The directory emitted is `./baz/` (last segment of the path)
- The `Project.Name` is `baz`; `Project.Module` is the full path

**Why:** the directory is what the user works in; the module path is what the rest of the Go toolchain sees. They're decoupled.

```go
func deriveName(module string) string {
    // "github.com/foo/bar/baz" -> "baz"
    // "myapp" -> "myapp"
    parts := strings.Split(module, "/")
    return parts[len(parts)-1]
}
```

---

## 14. Environment Availability (Phase 1)

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.23+ | Compiling `spin` | ✓ (verified `go version` = go1.26.2) | 1.26.2 | — |
| `git` | `git init` in scaffold | assumed | — | Skip with warning; user can `git init` manually |
| `air` | Generated `.air.toml`; user's `spin run` (Phase 2) | user-installed; not `spin`'s concern | — | Taskfile `setup` target installs via `go install` |
| `prism` | Generated `Taskfile.yml` test target; `spin test` (Phase 2) | user-installed | — | Taskfile falls back to `go test` |
| `gofumpt` | Generated `Taskfile.yml` `fmt` target; `spin fmt` (Phase 2) | user-installed | — | Taskfile falls back to `gofmt` |
| `goimports` | Generated `Taskfile.yml` `fmt` target | user-installed | — | Taskfile silently no-ops |
| `gum` | `spin new` interactive prompts (Phase 3) | user-installed; **not Phase 1** | — | Phase 1 doesn't prompt; not needed |
| `huh` | Scaffold-side prompts (Phase 3 fallback) | — | — | Not used in Phase 1 |
| `slopcheck` | Package legitimacy gate | NOT installable on this Nix read-only FS | — | All packages tagged `[ASSUMED]`; planner must add `checkpoint:human-verify` for any `go get` |

**Missing with fallback:** none blocking. The CI grep suite (§11), go build smoke test (§7), and git init (§12) all run with stock Go.

**Missing blocking:** none.

**Go version check:** the scaffolder itself needs Go 1.23+. The generated project needs Go 1.25.0+ if `--bubbles` is used. Phase 1 should detect and warn: `runtime.Version()` < `1.23` → print "spin requires Go 1.23+".

---

## 15. Open Decisions for Planner

The planner should make or confirm these — they are not lockable from research alone.

### 15.1 Module-path default — bare name vs `github.com/<user>/<name>`

- **Bare name (recommended for Phase 1):** simplest; no GitHub detection. User has to run `go mod edit -module github.com/<user>/<name>` before publishing.
- **GitHub prefix:** requires a `git config --get user.name` lookup (not always set in CI) or a prompt (which is Phase 3).
- **Recommendation:** ship bare-name in Phase 1. The Phase 3 prompter can ask for the GitHub org/username and rewrite `go.mod` post-scaffold.

### 15.2 `Taskfile.yml` `setup` target — install commands in Phase 1 or Phase 2?

- **Phase 1 (recommended):** ship the installs. Cost is one `Taskfile.yml`; benefit is a "working" `setup` target. The `WRAP-08` text in REQUIREMENTS.md is the formal ID; the spirit is captured by the success criterion ("contains a `Taskfile.yml` ... with a `setup` target").
- **Phase 2:** ships only the file structure; `setup` is a `TODO:`. Less useful, but cleaner separation of phases.
- **Recommendation:** Phase 1 ships the installs. The installs are static text in the template; no scaffolder logic is needed.

### 15.3 `gofumpt` install — `latest` vs pinned version

- **`@latest` (recommended):** matches the rest of the v2 stack; user gets the newest. Requires Go 1.25+ on the user's machine.
- **Pinned `@v0.6.0`:** works with Go 1.22+. But pinned versions go stale.
- **Recommendation:** `@latest` + README note "go install requires Go 1.25+". Most users running `spin` already have a recent Go.

### 15.4 `.air.toml` `cmd` output path — `./tmp/main` vs `bin/main`

- **`./tmp/main` (matches air docs):** the air reference uses this; `clean_on_exit = true` removes it on quit.
- **`bin/main`:** matches `go build -o` convention; survives across runs.
- **Recommendation:** `./tmp/main` for development; `bin/<name>` for the `task build` target. The `clean_on_exit = true` flag keeps `tmp/` tidy.

### 15.5 Generated `main.go` location — `./main.go` vs `./cmd/<name>/main.go`

- **Flat `./main.go`:** simpler, matches `cobra-cli init` default.
- **Nested `./cmd/<name>/main.go`:** matches idiomatic Go project layout for libraries + binaries; future-proofs for adding `cmd/other-tool/main.go`.
- **Recommendation for Phase 1:** flat `./main.go`. The user can move it later. The Phase 2 CLI variant template is a natural place to introduce `./cmd/<name>/` if there's demand.

### 15.6 `--all` flag — wired but no behavior in Phase 1?

- The roadmap's success criterion 1 only tests `--tui --bubbletea --bubbles --lipgloss`. `--all` is in FLAG-03 (Phase 1) but has no template (`variant_all/main.go.tmpl` is a stub).
- **Recommendation:** register `--all` as a flag (it sets `p.Type = "all"`) but emit a clear error in Phase 1: `--all is a Phase 2 feature; use --tui or --cli instead.` The flag binding is forward-compatible; only the template content is missing.

### 15.7 Generated README "Next steps" content — minimal vs verbose

- **Minimal:** "Run `go run .`" — three lines.
- **Verbose:** explain every file, every `task` target, the `prism`/`air`/`gofumpt` install story, the CGO contract, the "perfect first run" promise.
- **Recommendation:** medium — 15-20 lines. Title, 1-line description, "Next steps" with 3-4 commands, "Project layout" with bullet list, "Prerequisites" with Go version, and "Generated by spin" footer.

### 15.8 `bin/` and `tmp/` in `.gitignore` — needed?

- `bin/` — yes, common convention; ignore.
- `tmp/` — yes, created by air; ignore.
- `dist/` — yes, created by GoReleaser (Phase 3+); ignore.
- **Recommendation:** all three ignored from day one. Phase 2/3 features won't need to add them.

### 15.9 `charm.land/log/v2` in the *generated* project — yes or no?

- The scaffolder uses `charm.land/log/v2` internally (for its own output). Should the generated `main.go` import it too?
- **Pros:** consistent stack; user can drop in logging immediately.
- **Cons:** adds a dep for a hello-world; user has to learn one more lib.
- **Recommendation for Phase 1:** **not** in the generated `main.go`. The hello-world is intentionally minimal. Add `--log` flag wiring in Phase 2 to enable it.

### 15.10 Generated `AGENTS.md` — write a stub now or wait for Phase 3?

- AI-01..04 are all Phase 3. Phase 1 doesn't have a `--ai` flag.
- **Recommendation:** **do not** write `AGENTS.md` in Phase 1. The template file `templates/_base/AGENTS.md.tmpl` doesn't exist yet. Phase 3 will add the template + flag.

### 15.11 `--module` with a sub-path — emit example subdirs?

- If user passes `--module github.com/foo/bar/baz`, do we create `./myapp/internal/...`? Or just `./myapp/` flat?
- **Recommendation:** flat. `Module` is metadata in `go.mod`; doesn't change directory structure. The user's `main.go` import paths reflect the module.

---

## 16. Sources

### Primary (HIGH confidence — verified via Context7 MCP and `go list`)
- `/charmbracelet/bubbletea` (Context7) — `View() tea.View`, `tea.NewView`, `tea.NewProgram` simplified, `KeyPressMsg` typed message, removed `WithAltScreen`. Confirmed v2 import `charm.land/bubbletea/v2`.
- `/charmbracelet/lipgloss` (Context7) — `HasDarkBackground(in, out)`, `NewStyle`, `Color` function, `LightDark`, `compat.AdaptiveColor`. Confirmed v2 import `charm.land/lipgloss/v2`.
- `/charmbracelet/bubbles` (Context7) — v2 vanity import `charm.land/bubbles/v2/<sub>`; `runeutil`/`memoization` removed.
- `/charmbracelet/fang` (Context7) — `fang.Execute(context.Background(), cmd)` pattern; v2 import `charm.land/fang/v2`; requires cobra v1.9+.
- `/charmbracelet/log` (Context7) — `log.Default()` / `log.SetDefault()` / `log.NewWithOptions(buf, opts)`. Confirmed v2 import `charm.land/log/v2`.
- `/air-verse/air` (Context7) — `.air.toml` schema; `build.entrypoint = ["./tmp/main"]`; `include_ext` / `exclude_dir` / `exclude_regex` fields. Confirmed `build.bin` deprecated.
- `/spf13/cobra` (Context7) — `cobra.Command` definition; `Args: cobra.ExactArgs(1)` pattern; `cmd.Flags().String/Bool(...)` for flag binding.
- `go list -m -versions github.com/spf13/cobra` — confirmed `v1.9.1` is a published release; `v1.10.x` exists.
- `go list -m -versions github.com/spf13/viper` — confirmed `v1.20.0` and `v1.21.0` published.

### Secondary (MEDIUM confidence — verified against Go module proxy but not opened in Context7)
- `charmbracelet/bubbles/_autodocs/README.md` — Go 1.25.0 floor (cited in STACK.md, PITFALLS.md).
- `charmbracelet/fang/go.mod` — fang v2 declares `go 1.25.0` (cited in PITFALLS.md).
- `air-verse/air` install — `go install github.com/air-verse/air@latest` (cited in STACK.md).
- `daltonsw/prism` README — Go 1.24+ floor (cited in STACK.md, PITFALLS.md).
- `mvdan/gofumpt` install — `go install mvdan.cc/gofumpt@latest` (cited in STACK.md).
- `goreleaser/goreleaser` v2 install — `go install github.com/goreleaser/goreleaser/v2@latest` (Go 1.26+ for install).

### Local project files (HIGH confidence — current state of the project)
- `/home/samouly/Projects/Golang/loom/.planning/STATE.md` — Phase 1 of 4, MVP mode, scaffolding focus; decisions log.
- `/home/samouly/Projects/Golang/loom/.planning/ROADMAP.md` — Phase 1 success criteria verbatim, 5 criteria.
- `/home/samouly/Projects/Golang/loom/.planning/REQUIREMENTS.md` — 59 v1 requirements, all mapped; 30 in Phase 1.
- `/home/samouly/Projects/Golang/loom/.planning/research/STACK.md` — Verified library versions, install commands, version compatibility matrix.
- `/home/samouly/Projects/Golang/loom/.planning/research/ARCHITECTURE.md` — 4-layer architecture; `Project` struct pattern; embed+overlay template engine; gum subprocess wrapper.
- `/home/samouly/Projects/Golang/loom/.planning/research/FEATURES.md` — Table stakes vs differentiators; flag inventory; anti-features.
- `/home/samouly/Projects/Golang/loom/.planning/research/PITFALLS.md` — 15 critical pitfalls, v1→v2 API leaks, `go:embed` glob issues, `gum` non-TTY, `air` config drift, CGO leakage, gofumpt fallback.
- `/home/samouly/Projects/Golang/loom/.planning/research/SUMMARY.md` — Executive summary, key findings, confidence.
- `/home/samouly/Projects/Golang/loom/.planning/config.json` — `nyquist_validation: false` (skip Validation Architecture section); `mode: yolo`.
- `/home/samouly/Projects/Golang/loom/CLAUDE.md` — Project rules; charm v2 only; no CGO; prism/gofumpt/air/Taskfile.

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Charm v2 import paths | HIGH | Context7-verified for all 8 libraries in Phase 1 scope |
| Charm v2 API breaking changes | HIGH | Context7 upgrade guides confirmed `View() tea.View`, `KeyPressMsg`, `HasDarkBackground(in,out)` |
| `go:embed` + `text/template` engine design | HIGH | `text/template` is stdlib; pattern is well-documented |
| `.air.toml` schema (entrypoint vs bin) | HIGH | Context7-verified |
| fang v2 `Execute` signature | HIGH | Context7-verified with example |
| Cobra v1.9.1 pin | HIGH | `go list -m -versions` confirmed; fang v2 requires it |
| Go 1.25.0 floor for bubbles v2 | HIGH | Direct quote from bubbles v2 README via Context7 |
| Module path default behavior | MEDIUM | User preference — open decision §15.1 |
| `Taskfile.yml` setup target content | MEDIUM | User preference — open decision §15.2 |
| `gofumpt` version pin | MEDIUM | User preference — open decision §15.3 |
| `charm.land/log/v2` in generated `main.go` | MEDIUM | Design preference — open decision §15.9 |
| Phase 1/2/3/4 separation | HIGH | REQUIREMENTS.md and ROADMAP.md authoritative |

### Research date
2026-06-02 (today)

### Valid until
~2026-07-15 (30 days for stable v2 libraries; re-verify pin versions if `go list -m -versions` shows new stable releases before then)

---

## RESEARCH COMPLETE

**Phase:** 1 — Scaffolder Foundation + Core TUI Stack
**Confidence:** HIGH (with §15 open decisions for the planner)

### Key Findings
1. **Phase 1 is a Walking Skeleton** — the thinnest vertical slice is 7 emitted files + 1 git commit; prove the embed→render→emit→verify→git pipeline end-to-end.
2. **All v2 stack is verified** — `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2` paths confirmed via Context7; `View() tea.View`, `KeyPressMsg`, `HasDarkBackground(in,out)` API changes confirmed.
3. **Smoke test is the safety net** — `go build ./...` + `go test ./...` in the generated project catches v1 imports, wrong version pins, missing `go 1.25.0` directive, and `View() string` mistakes.
4. **CI grep suite is mandatory** — 17 forbidden patterns; bash script in `scripts/check-v1-leaks.sh`; covers PITFALLS #1, #2, #3, #4, #10.
5. **`.air.toml` must use `build.entrypoint`** — `build.bin = "tmp/main"` is deprecated; verified via Context7.

### File Created
`/home/samouly/Projects/Golang/loom/.planning/phases/01-scaffolder-foundation-core-tui-stack/01-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | All versions verified via Context7 + go list |
| Architecture | HIGH | ARCHITECTURE.md + PITFALLS.md cross-checked |
| Pitfalls | HIGH | PITFALLS.md comprehensive; grep patterns verified |
| Open Decisions | MEDIUM | 11 items flagged for planner judgment |

### Open Questions
See §15. The 11 open decisions are all "planner judgment" calls, not "missing information" gaps. None block the skeleton; they shape the polish.

### Ready for Planning
Research complete. Planner can now create PLAN.md files.
