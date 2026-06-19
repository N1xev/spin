# Architecture Research

**Domain:** Go project scaffold CLI (charmbracelet v2 flavor)
**Researched:** 2026-06-02
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                       CLI Layer (cobra + fang)                       │
│   rootCmd  ─┬─ new    (scaffold pipeline)                            │
│             ├─ run    (wraps air / go run)                           │
│             ├─ build  (wraps go build)                               │
│             ├─ test   (wraps prism / go test)                        │
│             ├─ vet    (wraps go vet)                                 │
│             └─ fmt    (wraps gofumpt + goimports)                    │
├─────────────────────────────────────────────────────────────────────┤
│                       Flag & State Layer                             │
│   Project struct  ←── flags ──→ interactive layer (gum)              │
│   (validated, resolved: libs, type, template, ai)                    │
├─────────────────────────────────────────────────────────────────────┤
│   Interactive Layer        │   Template Engine                       │
│   gum subprocess wrapper   │   go:embed.FS  +  text/template         │
│   (auto-detected; falls    │   (base + variant + library overlays)   │
│    back to flags or stdin) │                                          │
├─────────────────────────────────────────────────────────────────────┤
│   Wrapper Layer              │   AI / AGENTS Layer                   │
│   run/build/test/vet/fmt     │   internal/agents  →  AGENTS.md       │
│   (thin shims; detect tool   │   (template + project metadata)       │
│    on PATH, fail soft)       │                                          │
├─────────────────────────────────────────────────────────────────────┤
│                       Filesystem Sink                                │
│   ./<name>/  ── go.mod, main.go, .air.toml, Taskfile, AGENTS.md     │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| `cmd/root.go` | Cobra root, version, fang wiring | `fang.Execute(ctx, rootCmd)` |
| `cmd/new.go` | `spin new` subcommand -- calls scaffold pipeline | cobra `RunE` → `scaffold.New(opts)` |
| `cmd/run.go` | `spin run` -- detects `.air.toml`, falls back to `go run` | exec.CommandContext |
| `cmd/build.go` | `spin build` -- `go build -o bin/<name>` | exec.CommandContext |
| `cmd/test.go` | `spin test` -- `prism` if on PATH, else `go test` | exec.LookPath + exec.Command |
| `cmd/vet.go` | `spin vet` -- `go vet ./...` | exec.CommandContext |
| `cmd/fmt.go` | `spin fmt` -- `gofumpt` → `goimports` → `go fmt` fallback chain | exec.LookPath chain |
| `internal/scaffold` | Template loading, rendering, file emission | `embed.FS` + `text/template` |
| `internal/scaffold.Project` | Strongly-typed resolved scaffold config (libs, type, template name, ai on/off) | struct passed to `template.Execute` |
| `internal/interactive` | gum subprocess wrapper; collects missing flags | `os/exec` against `gum` binary |
| `internal/agents` | Render `AGENTS.md` from project metadata | template, gated by `--ai` |
| `templates/` | Embedded template tree (base + variants + per-library files) | `//go:embed all:templates` |
| `internal/version` | Version metadata (fang's `--version` requires it) | `var` populated via `-ldflags` |

## Recommended Project Structure

```
spin/
├── main.go                       # entrypoint: fang.Execute(ctx, rootCmd)
├── go.mod                        # module charm.land/spin/v2
├── go.sum
├── README.md
├── LICENSE
├── Taskfile.yml                  # self-dogfooding -- uses spin-like tasks
├── Makefile                      # alt entry, mirrors Taskfile
├── .air.toml                     # for `spin run` in spin's own dev
├── .gitignore
│
├── cmd/
│   ├── root.go                   # cobra root + fang + version
│   ├── new.go                    # spin new (scaffold entrypoint)
│   ├── run.go                    # spin run
│   ├── build.go                  # spin build
│   ├── test.go                   # spin test
│   ├── vet.go                    # spin vet
│   └── fmt.go                    # spin fmt
│
├── internal/
│   ├── scaffold/
│   │   ├── scaffold.go           # pipeline: New(opts) → Project.Render() → emit
│   │   ├── project.go            # Project struct (resolved config)
│   │   ├── template.go           # embed.FS walk, template parsing
│   │   ├── renderer.go           # text/template execution + funcMap
│   │   ├── emit.go               # write files, set perms, mkdir -p
│   │   ├── hooks.go              # post-scaffold hooks (go mod tidy, etc.)
│   │   └── git.go                # `git init` the new project
│   ├── interactive/
│   │   ├── gum.go                # gum binary detection + exec helpers
│   │   ├── prompts.go            # askProjectType, askLibs, askTemplate, askAI
│   │   └── fallback.go           # stdin/flag fallback if gum absent
│   ├── wrappers/
│   │   ├── run.go                # air? → go run
│   │   ├── build.go              # go build -o bin/<name>
│   │   ├── test.go               # prism? → go test
│   │   ├── vet.go                # go vet ./...
│   │   ├── fmt.go                # gofumpt → goimports → go fmt chain
│   │   └── tool.go               # exec.LookPath helper
│   ├── agents/
│   │   ├── agents.go             # render AGENTS.md from Project
│   │   └── detect.go             # detect libs → populate context for AI
│   └── version/
│       └── version.go            # Version, Commit, Date (ldflags-injected)
│
├── templates/                    # go:embed source
│   ├── _base/                    # always rendered
│   │   ├── go.mod.tmpl
│   │   ├── README.md.tmpl
│   │   ├── .air.toml.tmpl
│   │   ├── .gitignore.tmpl
│   │   ├── Taskfile.yml.tmpl
│   │   ├── Makefile.tmpl
│   │   ├── AGENTS.md.tmpl        # consumed by internal/agents
│   │   └── main.go.tmpl          # bare-bones entry; replaced by variant
│   ├── variant_tui/              # --tui
│   │   └── main.go.tmpl
│   ├── variant_cli/              # --cli (cobra + fang)
│   │   ├── main.go.tmpl
│   │   └── cmd/root.go.tmpl
│   ├── variant_all/              # --all (TUI + CLI combo)
│   │   └── main.go.tmpl
│   └── lib/                      # per-charm-library overlays
│       ├── bubbletea.go.tmpl     # wired into main.go via #include-style
│       ├── lipgloss.go.tmpl
│       ├── huh.go.tmpl
│       ├── glamour.go.tmpl
│       ├── glow.go.tmpl
│       ├── wish.go.tmpl
│       ├── log.go.tmpl
│       ├── crush.go.tmpl
│       ├── modifiers.go.tmpl
│       ├── ansi.go.tmpl
│       ├── runewidth.go.tmpl
│       ├── cobra.go.tmpl
│       ├── fang.go.tmpl
│       └── viper.go.tmpl
│
└── .planning/                    # gsd metadata, not shipped
```

### Structure Rationale

- **`cmd/`** -- thin cobra subcommand files. Each subcommand is one file. Flag declarations live with the subcommand; the resolver/validator is in `internal/scaffold`. Keeps cobra idiomatic and matches `cobra-cli init` convention.
- **`internal/scaffold/`** -- owns the entire pipeline: parse → render → emit → init git. Single importable surface. `Project` struct is the contract between flag parsing and template rendering -- avoids passing 30 args through.
- **`internal/interactive/`** -- gum lives behind an interface. The scaffold pipeline never knows whether flags came from CLI or gum. If gum is missing, fallback to `os.Stdin` reads or fail with a clear message. Easy to swap to `huh` later (see Patterns).
- **`internal/wrappers/`** -- one file per wrapped command. Each is a pure function `(projectRoot, args) error`. The `cmd/<x>.go` file just calls into the wrapper. Trivial to test in isolation.
- **`internal/agents/`** -- separated so AGENTS.md rendering can evolve independently (e.g., one day support Cursor's `.cursorrules` or Copilot's `.github/copilot-instructions.md`) without touching the scaffold pipeline.
- **`templates/_base/`** -- files rendered for every project. `_base/main.go.tmpl` is the placeholder overwritten by the selected variant.
- **`templates/variant_*/`** -- one per top-level project type. Only `main.go.tmpl` differs; everything else is inherited from `_base`.
- **`templates/lib/`** -- per-library overlays. Each lib template is a snippet (struct/import/wiring) merged into `main.go` via a template `{{define}}` block, not a standalone file. This avoids 200-line `if/else` trees inside `main.go.tmpl`.

## Architectural Patterns

### Pattern 1: Embed-First Template Engine with Variant + Overlay Composition

**What:** A `go:embed` rooted at `templates/` exposes a virtual filesystem. At scaffold time, walk the tree: copy `_base/*` first, then overwrite with `variant_<chosen>/*`, then `lib/<each>/*` (last-write-wins). Text rendering happens via `text/template` against the `Project` struct.

**When to use:** Always for spin -- this is the core mechanism. The variant+overlay split keeps templates small and combinable; adding a new library is one file in `templates/lib/`.

**Trade-offs:**
- Pro: zero template-engine dependency, composable, testable in isolation.
- Pro: variant + overlay matches how charm libs combine (any subset, any combo).
- Con: file name collisions in overlays need explicit precedence rules. Mitigation: keep overlay files in `lib/` named after the Go file they wire into (`bubbletea.go.tmpl` → renders to `bubbletea.go` and is wired via blank import or init in `main.go`).
- Con: complex `{{if}}` trees in `main.go.tmpl` for many libs. Mitigation: split per-lib wiring into a separate `_lib_<name>.go` file emitted from `lib/<name>.go.tmpl` and called from `main.go.tmpl` via `lib_<name>.Init(model)`.

**Example (rendering skeleton):**
```go
// internal/scaffold/scaffold.go
type Project struct {
    Name      string
    Module    string
    Type      string // "tui" | "cli" | "all"
    Libs      []string
    Template  string
    AI        bool
    Viper     bool
    CharmMajor int
}

func (p *Project) Render(fsys embed.FS) error {
    parts := []string{"_base", "variant_" + p.Type}
    for _, lib := range p.Libs {
        parts = append(parts, "lib/"+lib)
    }
    return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
        // overlay: later parts win
        // run text/template with p as data
    })
}
```

### Pattern 2: gum Subprocess Wrapper with Interface Boundary

**What:** A `prompter` interface in `internal/interactive` with one method per prompt (`Choose`, `Input`, `Confirm`). The default implementation shells out to `gum`; an alternative implementation reads from `os.Stdin` or returns flag-supplied values. The scaffold pipeline depends only on the interface.

**When to use:** Always, because the requirement is gum-driven interactive *and* `--no-interactive` flag-only mode. The interface keeps both paths identical from the caller's perspective.

**Trade-offs:**
- Pro: gum subprocess is dead-simple, no in-process TUI state to manage.
- Pro: swapping to `huh` later (in-process, charm-native) is a one-file change.
- Con: gum must be on PATH for interactive mode. Mitigation: `exec.LookPath("gum")` up front; if absent, print a clear message + offer to install via `go install`.
- Con: subprocess startup latency per prompt (~50–100 ms). Acceptable for a scaffolder (cold path, user is waiting for prompts anyway).

**Example:**
```go
// internal/interactive/gum.go
type Prompter interface {
    Choose(title string, options []string, def string) (string, error)
    Input(title, placeholder, def string) (string, error)
    Confirm(title string, def bool) (bool, error)
}

type Gum struct{ Bin string }

func (g *Gum) Choose(title string, opts []string, def string) (string, error) {
    args := []string{"choose", "--header", title}
    if def != "" { args = append(args, "--selected", def) }
    args = append(args, opts...)
    cmd := exec.Command(g.Bin, args...)
    out, err := cmd.Output()
    return strings.TrimSpace(string(out)), err
}
```

### Pattern 3: Tool-Wrapper Layer with LookPath Fallback Chain

**What:** Each wrapper (`run`, `build`, `test`, `vet`, `fmt`) is a pure function that takes `(projectRoot string, extraArgs []string) error`. The wrapper does `exec.LookPath` for the preferred tool (e.g., `air`, `prism`, `gofumpt`) and falls back to the standard tool (`go run`, `go test`, `gofmt`) with a one-time warning.

**When to use:** Always -- every wrapper command in PROJECT.md. Keeps `cmd/<x>.go` files minimal (cobra flag binding + one call).

**Trade-offs:**
- Pro: works on a fresh machine with only Go installed.
- Pro: degrades gracefully -- the user gets a useful scaffold even if they haven't installed `air`/`prism`/`gofumpt` yet.
- Con: silent fallback could confuse power users. Mitigation: print a one-line "using `go test` (install prism for parallel workers: `go install ...`)" message.
- Con: tool detection at every invocation is slightly wasteful. Mitigation: detect once per process, cache in a `var`.

**Example:**
```go
// internal/wrappers/test.go
func Test(projectRoot string, args []string) error {
    bin, err := exec.LookPath("prism")
    if err != nil {
        fmt.Fprintln(os.Stderr, "note: prism not found, falling back to go test (install: go install github.com/DaltonSW/prism@latest)")
        bin, _ = exec.LookPath("go")
        return runBin(bin, projectRoot, append([]string{"test"}, args...))
    }
    return runBin(bin, projectRoot, append([]string{"go", "test", "./..."}, args...))
}
```

### Pattern 4: Single-Entry Subcommand File (cobra convention)

**What:** Each `cmd/<subcommand>.go` defines exactly one cobra `*cobra.Command` value and one `init()` that attaches it to the root. No business logic -- only flag binding and a single call into `internal/`.

**When to use:** Always. Matches the `cobra-cli add` convention and keeps each subcommand a 30-line file that's easy to scan.

**Trade-offs:**
- Pro: conventional, every Go developer recognizes it.
- Pro: trivial to remove a subcommand -- delete one file.
- Con: flag definitions are scattered. Mitigation: keep flag struct + binding in the subcommand file, but the *resolution/validation* in `internal/scaffold`.

**Example:**
```go
// cmd/new.go
func init() {
    rootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
    Use:   "new <name>",
    Short: "Scaffold a new charmbracelet project",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        opts, err := scaffold.ResolveFlags(cmd, args)
        if err != nil { return err }
        if !opts.NoInteractive {
            opts, err = interactive.PromptMissing(opts)
            if err != nil { return err }
        }
        return scaffold.New(opts)
    },
}
```

## Data Flow

### Scaffold Flow (`spin new <name>`)

```
User: spin new myapp --tui --bubbletea --lipgloss --ai
              │
              ▼
┌──────────────────────────────────────────────────────────┐
│ cobra + fang                                             │
│   parse positional: name="myapp"                        │
│   parse flags:    tui=true, libs=[bubbletea,lipgloss],  │
│                   ai=true                               │
└──────────────────────────────────────────────────────────┘
              │
              ▼
┌──────────────────────────────────────────────────────────┐
│ scaffold.ResolveFlags(cmd, args)                         │
│   validate: name is valid Go package, no path conflicts  │
│   build Project{ Name:"myapp", Type:"tui", Libs:[...],  │
│                   AI:true, Module:"myapp" }              │
│   if any required field missing:                        │
│       → call interactive.PromptMissing(opts)            │
│       → gum subprocess (or stdin fallback)              │
└──────────────────────────────────────────────────────────┘
              │
              ▼
┌──────────────────────────────────────────────────────────┐
│ scaffold.New(project)                                    │
│   1. mkdir ./<name>/                                     │
│   2. for path in fs.WalkDir(embedFS, "templates"):      │
│        resolve overlay (base → variant → libs)          │
│        parse text/template with project as data         │
│        emit to ./<name>/<path>                          │
│   3. if --ai: emit AGENTS.md from agents template       │
│   4. hooks.PostScaffold(project):                        │
│        - git init                                        │
│        - go mod tidy  (best-effort, may need network)   │
│   5. print success banner (lipgloss) + next-steps       │
└──────────────────────────────────────────────────────────┘
              │
              ▼
./myapp/    (ready to `go run`)
```

### Wrapper Flow (`spin test`)

```
User: spin test ./internal/...
              │
              ▼
cobra: parse args = ["./internal/..."]
              │
              ▼
wrappers.Test(projectRoot, args)
              │
              ├─ exec.LookPath("prism")
              │     ├─ found  → exec.Command("prism", "go", "test", "./...", args...)
              │     └─ not    → warn + exec.Command("go", "test", args...)
              │
              ▼
stream stdout/stderr to user's terminal; propagate exit code
```

### Key Data Flows

1. **Flag → Project struct → Template data:** The `Project` struct is the single source of truth after `ResolveFlags`. Templates reference fields directly: `{{ .Name }}`, `{{ if .AI }}...{{ end }}`, `{{ range .Libs }}...{{ end }}`. No magic globals.
2. **Overlay resolution:** Deterministic -- base files load first, variant files overwrite on filename match, lib files overwrite on filename match. Last write wins, by design.
3. **Interactive fallback chain:** If `gum` is missing AND `--no-interactive` is set AND flags are incomplete → fail with actionable error. If `gum` is missing AND interactive is on → fall back to `os.Stdin` reads (using `bufio.Scanner` + a small set of `fmt.Println` prompts).
4. **Tool detection (wrappers):** `exec.LookPath` at command-invocation time, not at startup. Lets a user `go install prism` mid-session and have the next `spin test` use it.

## Build Order

The dependency graph between components drives the suggested phase structure:

```
[1] Minimal scaffold (one flag)              ← proves pipeline end-to-end
      │
      ▼
[2] All flags + Project struct validation     ← completes CLI surface
      │
      ▼
[3] Interactive layer (gum)                   ← needs Project to know what's missing
      │
      ▼
[4] Template variants (--template)            ← needs scaffold engine + Project fields
      │
      ▼
[5] Wrappers (run/build/test/vet/fmt)         ← independent of scaffold; build in parallel
      │
      ▼
[6] AI / AGENTS.md (--ai)                     ← needs Project metadata populated
```

**Why this order:**
- **Phase 1 must produce a runnable project** -- even if it's just `spin new foo --tui` and writes a hardcoded `main.go`. Validates the embed + render + emit pipeline.
- **Phase 2 can be done without interactive** -- all flags are just CLI args. Project struct validation lives here. Tests can be written against flag combinations.
- **Phase 3 layers on top** -- the `Prompter` interface (Pattern 2) means `ResolveFlags` can stay the same; only the missing-field branch changes.
- **Phase 4 is template authoring** -- by this point the engine is stable, so templates are pure content work.
- **Phase 5 is fully independent** of the scaffold pipeline; can ship in any phase after the root cmd exists. Best deferred so scaffold isn't blocked.
- **Phase 6 is a leaf** -- depends only on the `Project` struct, which is finalized by Phase 2. Cheap to add later.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|---------------------------|
| 1 template (now) | Single `templates/` tree, embedded as one `embed.FS` |
| 5–10 templates | Add `templates/<name>/` directories; `--template` already routes to them. Engine unchanged. |
| 10+ templates | Consider splitting embed.FS per template for binary-size savings; ship a `spin-template` subcommand to add new template directories. |
| Plugin/template-repo support | `--template-repo <url>` (already in PROJECT.md) → `git clone` to a temp dir, point `embed.FS` (now `os.DirFS`) at the cloned path. Engine unchanged if templates are loaded from an `fs.FS` interface. |

### Scaling Priorities

1. **First bottleneck:** `go:embed` makes the binary size = size of all embedded templates. If templates grow large (images, big assets), allow opt-in `--no-embed` builds.
2. **Second bottleneck:** Walking + parsing templates on every `spin new` is O(files). Fine for <100 templates; cache the parsed template tree in a `sync.Once` if it ever matters.

## Anti-Patterns

### Anti-Pattern 1: God-Object `*Scaffolder` with all flags as methods

**What people do:** `scaffolder.SetName().SetLibs().SetType().SetAI().Execute()` -- fluent builder sprawl.
**Why it's wrong:** Hard to validate, hard to test, hides which fields are required. The cobra flag bindings become stringly-typed.
**Do this instead:** Single `Project` struct populated by `ResolveFlags`, then passed to `scaffold.New(project)`. All required-field validation is in one place.

### Anti-Pattern 2: Embedding templates as Go strings (`const tmpl = "..."`)

**What people do:** Put template content inside Go source for "type safety".
**Why it's wrong:** Defeats `go:embed` (no syntax highlighting in editor, no diff-friendly format, mixes content with code).
**Do this instead:** `//go:embed all:templates` in `internal/scaffold/embed.go`, walk the resulting `embed.FS`. Keep `.tmpl` files in their own directory tree.

### Anti-Pattern 3: One Mega-`main.go.tmpl` with 500 lines of `{{if}}` blocks

**What people do:** Every library's wiring is a giant conditional in one file.
**Why it's wrong:** Unmaintainable; adding a new lib means editing one massive file; template errors are hard to localize.
**Do this instead:** Per-library snippet templates in `templates/lib/<name>.go.tmpl`, each rendering a separate Go file in the output project. `main.go.tmpl` calls into them via standard Go imports (e.g., `import _ "myapp/libbubbletea"`).

### Anti-Pattern 4: Hardcoding charm v2 import paths in templates

**What people do:** Write `import tea "github.com/charmbracelet/bubbletea"` in a `main.go.tmpl` somewhere, then ship a v2 binary that breaks.
**Why it's wrong:** Charm v2 uses vanity imports (`charm.land/bubbletea/v2`). v1 paths are deprecated. Templates must use the v2 paths exclusively.
**Do this instead:** Centralize all charm imports in `templates/lib/*.go.tmpl` and have a single test that greps generated projects for `github.com/charmbracelet/` to catch regressions.

### Anti-Pattern 5: Wrapping `go test` with shell scripts

**What people do:** `exec.Command("bash", "-c", "go test ./...")` -- easy one-liner, but breaks on Windows and won't propagate signals cleanly.
**Why it's wrong:** Spin is `CGO_ENABLED=0` portable; bash assumptions violate that.
**Do this instead:** `exec.CommandContext(ctx, "go", "test", args...)` with explicit arg slices. Always `cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr` for streaming.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| `gum` (subprocess) | `exec.LookPath` + `exec.Command`, fall back to stdin if missing | If absent, print install hint; never silently degrade |
| `air` (subprocess, for `spin run`) | `exec.LookPath`; fall back to `go run` | Detect `.air.toml` to decide which to use |
| `prism` (subprocess, for `spin test`) | `exec.LookPath`; fall back to `go test` | Same as above |
| `gofumpt` / `goimports` (subprocess, for `spin fmt`) | Lookup chain: `gofumpt` → `goimports` → `gofmt` | Apply in order; only run the next if previous is missing |
| `go` toolchain | `exec.Command` for all build/test/vet | Detect via `runtime.GOROOT()` or just trust `$PATH` |
| `git` (for `git init` in new project) | `exec.Command`; non-fatal if missing | User can `git init` later |
| External template repo (`--template-repo`) | `git clone --depth 1` to temp dir, then `os.DirFS` | Clean up temp dir in `defer` |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `cmd/*` ↔ `internal/*` | Direct function calls; no globals | Each `cmd/<sub>.go` calls exactly one `internal/<pkg>.X(...)` function |
| `internal/scaffold` ↔ `internal/interactive` | `Prompter` interface | `scaffold` never imports `interactive`; `interactive` returns values that fill `scaffold.Project` |
| `internal/scaffold` ↔ `internal/agents` | `Project` struct (read-only) | `agents` takes a `*Project` and returns rendered `[]byte` |
| `internal/wrappers` ↔ `cmd/wrappers` | Function calls | `cmd/test.go` calls `wrappers.Test(root, args)`; nothing more |
| `templates/` ↔ `internal/scaffold` | `embed.FS` injected at scaffold time | Scaffold package is the only one that knows the embed path |
| Root cmd ↔ subcommands | `rootCmd.AddCommand(...)` in `init()` files | Standard cobra; no custom registry |

## Dogfooding Notes

Spin uses the charm stack to build itself. This is both a constraint and a showcase.

- **Help output:** `fang.Execute(ctx, rootCmd)` wraps the root cmd. `spin --help` looks like a charm product from day one.
- **Version flag:** fang requires a `version` package (`internal/version`) populated via `-ldflags="-X .../version.Version=..."` in the Taskfile.
- **Banner / success messages:** `internal/scaffold` uses `charm.land/lipgloss/v2` to style the "Created ./myapp" output. Matches the fang aesthetic.
- **Self-template:** `templates/_base/` includes a `Taskfile.yml.tmpl` and `.air.toml.tmpl` -- the same files spin itself ships. Developers who scaffold with spin can run `spin run` / `spin test` in their new project immediately.
- **Readme preview (future):** Could pipe the generated `README.md` through `charm.land/glamour/v2` for a one-shot preview at scaffold time. Out of scope for v1; noted as a future enhancement.
- **AGENTS.md content:** When `--ai` is set, the generated `AGENTS.md` describes the *new* project's stack -- including the fact that `spin` is a charm-flavored scaffolder. Self-referential and useful for AI assistants picking up the new project.

## Sources

- [Fang docs (Context7)](https://context7.com/charmbracelet/fang/llms.txt) -- `fang.Execute(ctx, rootCmd)` pattern, version flag requirements
- [Fang README](https://github.com/charmbracelet/fang/blob/main/README.md) -- styled help, manpages, completions
- [Bubble Tea v2 Upgrade Guide](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md) -- `charm.land/bubbletea/v2` import path, model pattern
- [Lipgloss v2 Upgrade Guide](https://github.com/charmbracelet/lipgloss/blob/main/UPGRADE_GUIDE_V2.md) -- `lipgloss.NewStyle()` is pure value; no Renderer
- [Huh v2 docs (Context7)](https://context7.com/charmbracelet/huh/llms.txt) -- `huh.NewForm`, `huh.NewInput().Run()` for standalone fields
- [Gum README](https://github.com/charmbracelet/gum/blob/main/README.md) -- `gum choose` / `gum input` / `gum confirm` / `gum write` invocation
- [Cobra Generator docs (Context7)](https://context7.com/spf13/cobra-cli/llms.txt) -- `cmd/root.go` + `main.go` layout, `cobra-cli init` convention
- [.planning/PROJECT.md](file:///home/samouly/Projects/Golang/loom/.planning/PROJECT.md) -- requirements, constraints, key decisions

---
*Architecture research for: spin (Go scaffold CLI, charmbracelet v2 flavor)*
*Researched: 2026-06-02*
