# spin -- Product Requirements Document

**Version:** 2.0 (External-Template Scaffolder)
**Status:** v2.0-template complete -- validated 2026-06-14. README + PRD in sync with the v2 model.
**Repository root:** `/home/samouly/Projects/Golang/spin`
**Binary name:** `spin`
**Language:** Go 1.23+
**Distribution:** single static binary (`CGO_ENABLED=0`), installed via `go install`

---

## 1. One-Line Pitch

> One CLI to scaffold projects from external templates and run their tasks. `spin new <name> --template <spec>` for greenfield, `spin add <spec>` to pin a template for offline use, and `spin update [name]` to refresh a pinned template's cache.

---

## 2. What This Is

`spin` is a **language-agnostic scaffolder for external templates**. A template is a git repository or local directory that contains a `spin.toml` manifest and a `_base/` tree of overlay files. `spin new` resolves the template, prompts (or accepts) the param values, renders `_base/` into the user's project, and runs the template's `[[post]]` steps.

`spin` is built around two concepts:

1. **Template** -- the first-class citizen. A directory with `spin.toml` and a `_base/` tree. `spin` knows nothing about a template's language or framework; the template's `_base/` and `[[post]]` steps are entirely the author's responsibility.
2. **Pin** -- a name + on-disk cache entry in `~/.config/spin/pinned.json`. Lets `spin new --template <name>` work offline against a previously-cloned git repo. Cached under `~/.config/spin/templates/<name>/`.

Earlier v2.x concepts (the `Ecosystem` interface, compiled-in `charm` and `rust` ecosystems, the `spin new <ecosystem> <name>` form, the universal `spin run <task>` task runner, the `Builder` stub) were removed in the v2.0-template pass. Templates are the only extension surface now.

`spin` itself is built with **cobra + charm.land/fang/v2** so the tool dogfoods the charmbracelet v2 stack.

### Core Value

> Generate a runnable project from any external template with one command.
> `spin new myapp --template go-cli && cd myapp && go run .` produces a
> project that builds, tests, and runs without extra setup -- regardless
> of language, framework, or build tool. The template author owns the
> details; `spin` owns the load / prompt / render / post-hook pipeline.

---

## 3. Tech Stack

### Runtime

| Layer | Choice | Why |
|-------|--------|-----|
| Go version | **1.23** (spin itself) | Sufficient -- spin doesn't import bubbles v2 |
| Project Go version (charm projects with bubbles) | **1.25.0+** | Required by `charm.land/bubbles/v2` |
| CLI framework | **cobra** v1.9.1 | De facto Go CLI standard |
| CLI styling | **charm.land/fang/v2** | Drop-in for cobra; charm-style help, errors, completions |
| Config (opt-in) | **spf13/viper** v1.20.x | Only wired when user passes `--viper` |
| Output styling | **charm.land/lipgloss/v2** | Success messages, color |
| Logging | **charm.land/log/v2** | In-project + spin-itself |
| Forms (fallback) | **charm.land/huh/v2** | In-process, used when `gum` is absent |
| Prompts (primary) | **gum** (binary) | Shells out; best-in-class CLI prompts |
| No CGO | yes | `CGO_ENABLED=0` for static binaries |

### Toolchain `spin` wraps

| Tool | Install | Purpose |
|------|---------|---------|
| `air` | `go install github.com/air-verse/air@latest` | Hot reload (`.air.toml` present → `air`; absent → `go run .`) |
| `prism` | `go install go.dalton.dog/prism@latest` | Test runner (parallel + colored; falls back to `go test`) |
| `gofumpt` | `go install mvdan.cc/gofumpt@latest` | Stricter formatter (falls back to `gofmt`) |
| `goimports` | `go install golang.org/x/tools/cmd/goimports@latest` | Import fixer (runs after gofumpt) |
| `golangci-lint` | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` | Lint (invoked directly by users / CI; not wrapped by a `spin` subcommand) |
| `goreleaser` | `go install github.com/goreleaser/goreleaser/v2@latest` | Cross-platform build (for end-user projects) |

### Charm v2 module paths (v2.0)

```
charm.land/bubbletea/v2
charm.land/lipgloss/v2 (+ subpackages: table, tree, list)
charm.land/bubbles/v2 (+ subpackages: spinner, textinput, ...)
charm.land/huh/v2
charm.land/glamour/v2
charm.land/wish/v2 (+ subpackages: bubbletea, logging, activeterm)
charm.land/log/v2
charm.land/fang/v2
github.com/charmbracelet/glow/v2        (binary)
github.com/charmbracelet/crush          (binary)
github.com/charmbracelet/x              (subpackages: ansi, pony, vt)
```

v1 paths are **forbidden** -- `scripts/check-v1-leaks.sh` enforces 22 v1→v2 patterns and 1 deprecated air config.

---

## 4. Command Surface (10 files in `cmd/`)

Every command is wired into the cobra root and dispatched by `main.go` through `fang.Execute(ctx, rootCmd, fang.WithVersion(version.Version))`.

### 4.1 Root -- `cmd/root.go`

`spin` with no subcommand prints help. `SilenceUsage` / `SilenceErrors` are set per-command to keep error output clean.

### 4.2 `spin new` -- `cmd/new.go`

The flagship command. v2 contract: `spin new <name> --template <spec>`. Three `<spec>` kinds:

- **local path**: `/abs/path` or `./relative` or `~/...` -- no network
- **git URL**: `https://...`, `git@...`, `git://...`, `ssh://...` -- shallow-cloned into the cache
- **pinned name**: `my-template` -- looked up in `~/.config/spin/pinned.json` and rendered from the on-disk cache

`--template` also accepts the `user/repo` shorthand, which the loader transparently expands to `https://github.com/user/repo.git`.

Flags:
- `-t, --template <spec>` (required) -- the template source
- `-d, --dest <path>` -- destination directory (default: `./<name>`; `~` expanded)
- `--param key=value` (repeatable) -- non-interactive param values; coercion per the param's type
- `--print-params` -- print the resolved params as JSON and exit (no files written)
- `--dry-run` -- render to a temp dir, print the file list, and clean up

When the destination already has a previous clone, the loader prompts with four choices (Reuse / Pin / Wipe / Cancel). On invalid-pinned templates, it prompts Keep / Remove. Both prompts are no-op in non-TTY mode.

### 4.3 `spin add` -- `cmd/add.go`

`spin add <spec>` -- pin a template locally. The spec can be a local path, a git URL, or a `user/repo` shorthand. The result is a `Pinned` record in `~/.config/spin/pinned.json` plus an on-disk cache under `~/.config/spin/templates/<name>/`. The flag `--list` is an alias for `spin list`.

### 4.4 `spin list` -- `cmd/list.go`

Print every pinned template. Default output is a styled table; `--json` emits a JSON array suitable for `jq` and other tooling.

### 4.5 `spin update` -- `cmd/update.go`

`spin update [name]` -- re-clone or re-copy a pinned template's on-disk cache. With a name, refreshes just that pin; without, refreshes all. The old cache is moved to a `.bak-<unix-ts>` sibling before the refresh; on failure the .bak is moved back into place. On success the .bak is removed. Refresh supports both local-path sources (recopy) and git URLs (re-clone).

### 4.6 `spin remove` -- `cmd/remove.go`

`spin remove <name>` -- drop a pin. The on-disk cache is left alone by default (a future `spin add` reuses it); `--purge` deletes the cache too. Alias: `rm`.

### 4.7 `spin search` -- `cmd/search.go`

`spin search <query>` against the hosted registry. Graceful "not yet deployed" message on 404 / network error.

### 4.8 `spin init` -- `cmd/init.go`

`spin init <name>` -- scaffold a new template directory in the current working directory (or `--dir <parent>`). Produces a starter `spin.toml` (with example params + a no-op `[[post]]` step), a `_base/file.txt.tmpl` placeholder, and a `README.md`. The result is immediately renderable: `spin new my-app --template <dir>` works without any further edits.

### 4.9 `spin version` -- `cmd/version.go`

Prints `version.Version` (overridable via `-ldflags`).

### 4.10 `main.go`

```
fang.Execute(ctx, rootCmd, fang.WithVersion(version.Version))
```

`os.Exit(1)` on error; huh's `ErrUserAborted` is handled in `cmd/new.go` with a direct `os.Exit(130)` to avoid fang's "Aborted." print.

---

## 5. Internal Packages (4 packages, file by file)

`spin` is a thin CLI over four internal packages. The previous v2.x packages (`internal/ecosystem/`, `internal/ecosystems/`, `internal/prompt/`, `internal/scaffold/`, `internal/update/`, `internal/runner/`) were removed in the v2.0-template pass; what remains is the load / prompt / render / pin pipeline plus a build-time version constant.

### 5.1 `internal/version/` -- 1 file

**`version.go`** -- `var Version = "0.1.0"`. Overridable via `-ldflags "-X github.com/N1xev/spin/internal/version.Version=v2.0.0"`. Used by `cmd/version.go` and by `fang.WithVersion(...)` in `main.go`.

### 5.2 `internal/params/` -- 13 files (10 source + 3 test)

The typed parameter system used by templates (`spin.toml` `[params]` section) and any caller that wants a huh-backed form.

**`param.go`** -- Core abstractions:
```go
type Type string                                  // one of 8 types
type Value struct { String, Path string; Int int; Bool bool; List []string }

type Param interface {
    Name() string
    Type() Type
    Prompt() string
    Default() any
    Hmm() string                                  // hint shown under the field
    Apply(Value)
    Value() Value
    HuhField() huh.Field                          // build a huh field for the form
    String() string                               // serialised form (for --param coercion)
}

type Spec struct {                                // raw spec from spin.toml
    Type     Type
    Default  any
    Prompt   string
    Help     string
    Min, Max *int
    Options  []string
}

var ErrUnknownType = ...                          // carries the offending name + type
```

**`form.go`** -- `Form() *huh.Form` (groups 4 per row), `Run() error` (returns `huh.ErrUserAborted` on Ctrl-C), `SetDefaults([]Param)` -- applies `Default()` to each.

**`parse.go`** -- `Parse(SpecMap) ([]Param, error)`, `ParseOne(name, spec)`, helpers `asString` / `asInt` / `asBool` / `asStringSlice`.

**`text.go`** -- `TextParam` with `huh.NewInput`.

**`textarea.go`** -- `TextareaParam` with `huh.NewText` + `CharLimit(0)` (no limit).

**`number.go`** -- `NumberParam` with `huh.NewInput` + `Validate(func(string) error { ... })` enforcing min/max.

**`select.go`** -- `SelectParam` with `huh.NewSelect[string]`.

**`multiselect.go`** -- `MultiSelectParam` with `huh.NewMultiSelect[string]`.

**`bool.go`** -- `BoolParam` with `huh.NewConfirm` (Yes/No).

**`path.go`** -- `PathParam` with `huh.NewFilePicker` (file or dir mode based on suffix).

**`secret.go`** -- `SecretParam` with `huh.NewInput` + `EchoMode(huh.EchoModePassword)`. **No default** (security: never leak a placeholder).

**`parse_test.go`** + **`param_test.go`** -- 14 tests covering round-trip, defaults, shorthand, unknown-type error, SetDefaults order preservation, etc.

### 5.3 `internal/template/` -- 10 files (8 source + 2 test)

The external template system. A template is a directory with a `spin.toml` manifest (metadata + params + post-hooks) and a `_base/` tree of overlay files. This package is the load / parse / render / post-hook pipeline; the registry cache is owned by `internal/registry/`.

**`engine.go`** -- `renderFile`, `funcMap` (`upper`, `lower`, `title`, `trim`, `join`, `default`, `snakeCase`, `shellQuote`), `WriteFiles` / `writeFiles` with a path-traversal guard (rejects any key containing `..`, starting with `/`, or with NUL).

**`template.go`** -- Core types + flow:
```go
type Template struct {
    BaseDir string                                  // absolute path to the loaded template
    Meta    SpinToml
}

func Detect(dir string) (*Template, error)          // requires spin.toml + _base/
func (t *Template) Render(values map[string]any) (map[string][]byte, error)
func (t *Template) RenderTo(dest string, values map[string]any) error
func (t *Template) RenderToWithPost(dest string, values map[string]any) error
// RenderToWithPost: Render -> WriteFiles -> RunPostHook -> deleteSpinToml (defensive)
```

`deleteSpinToml` walks `dest` and removes any `spin.toml` (TPL-16: covers the case where the template's `_base/` accidentally included one).

`isExcluded(path, patterns)` and `stripTmplExt(p)` support `.spinignore`-style filtering and `.tmpl` suffix stripping.

**`loader.go`** -- The single entry point for `Load(spec string)`:
```go
type Loader struct { CacheDir string; LookPath func(string) (string, error); ... }
func NewLoader(cacheDir string) *Loader
func (l *Loader) Load(spec string) (*Template, error)
func (l *Loader) Lister() ([]string, error)         // enumerate the on-disk cache
func (l *Loader) Clear(ref string) error
```

Dispatch chain inside `Load`:
1. `isLocalPath(spec)` (`/...`, `./...`, `~/...`) -> `loadLocal`
2. `isGitURL(spec)` (`https://...`, `git@...`, `git://...`, `ssh://...`) -> `cloneGit` (`--depth=1`, `GIT_TERMINAL_PROMPT=0`) -> return template
3. `loadPinned(spec)` -> look up `pinned.json` -> `loadPinnedFromPath` (use the `LocalPath` recorded at pin time)
4. `isShorthand(spec)` (`user/repo`, no slash-prefix) -> `expandShorthand` -> `https://github.com/user/repo.git` -> `cloneGit`

Heuristics:
- `isLocalPath` -> starts with `/`, `./`, `~`
- `isGitURL` -> starts with `https://`, `http://`, `git://`, `ssh://`, or `git@`
- Ambiguous `foo/bar` -> treated as a **registry shorthand**, not a local path

Default cache lives under XDG (`os.UserConfigDir()` -> `~/.config/spin/templates/`). When the destination already has a previous clone, the loader prompts with four choices (Reuse / Pin / Wipe / Cancel) via `destAction`; for invalid pinned records it prompts Keep / Remove. Both prompts are no-ops in non-TTY mode. `sanitiseRepoName`, `compareSemver`, and `warnMinSpinVersion` support safe naming, version-aware refresh, and `min_spin_version` warnings.

**`spin_toml.go`** -- Schema:
```go
type SpinToml struct {
    Name        string
    Description string
    Type        string                                // "service" | "library" | "binary" | ...
    Language    string
    MinSpinVersion string
    Author      Author
    Params      map[string]Spec
    Post        []PostStep
    Tags        []string
    Includes    []string
    Excludes    []string
}

type Author struct { Name, Email string }
type PostStep struct { Run, Description string }

func ParseSpinToml(path string) (*SpinToml, error)
func ParseSpinTomlBytes(b []byte) (*SpinToml, error)   // hand-rolled mini-parser
```

`ParseSpinTomlBytes` handles `[params]` / `[post]` sections, inline tables, the shorthand `name = "default"` form (legacy v1), and the `[params.<name>]` per-param block.

**`parse.go`** -- `parseTOML` -- handles inline tables `{ type, prompt, default, min, max, options }`, the shorthand, and the typed coercion helpers (`coerceParamValue`, `specFromMap`, `asInt64`).

**`post_hook.go`** -- `RunPostHook(tpl, values, dir)` -- `sh -c` in the dir, with `text/template` rendering against unwrapped values (no funcMap). `unwrapValues` / `unwrapAny` flatten `params.Value` to primitives so the template author can use `{{.name}}` / `{{.features}}` directly.

**`form.go`** -- The huh form built from a `Template`'s params:
```go
func (t *Template) BuildForm(values map[string]any) (*huh.Form, error)
func (t *Template) ResolveForm(values map[string]any, interactive bool) (map[string]any, error)
func (t *Template) Hints() []string
```
`ResolveForm` applies defaults first, then user values, unwraps `params.Value` to primitives, and returns the `map[string]any` the renderer consumes. `UnwrapValue(v params.Value) any` is the public primitive for callers that want to reuse the same flattening.

**`loader_test.go`** + **`template_test.go`** -- cover LocalPath / GitURL / Pinned / Shorthand dispatch, missing `spin.toml`, missing `_base/`, path-traversal guard, defensive `spin.toml` deletion, post-hook shell execution, huh abort, and `min_spin_version` warning.

### 5.4 `internal/registry/` -- 4 files (3 source + 1 test)

The local pin store + hosted-registry client. The registry server is a separate project; the client degrades gracefully when it is unreachable.

**`client.go`** -- `Client` with `IndexURL`, `HTTP` (default `*http.Client` with 10s timeout), `CacheDir`:
```go
type Client struct { IndexURL string; HTTP *http.Client; CacheDir string }

func New() *Client
// priority: SPIN_REGISTRY_URL > SPIN_REGISTRY > DefaultIndexURL
// cache: ~/.config/spin/templates/

func (c *Client) Search(query string) (*SearchResult, error)
func (c *Client) SearchWithLimit(query string, limit int) (*SearchResult, error)
func (c *Client) Add(spec string) (*Pinned, error)            // local -> copyDir; git -> shallow clone
func (c *Client) Refresh(pin Pinned) (Pinned, error)         // re-copy or re-clone a pin's cache
func (c *Client) PinnedPath() string
func (c *Client) ListPinned() ([]Pinned, error)
func (c *Client) Pin(p Pinned) error                          // atomic: temp file + rename
func (c *Client) Unpin(name string) error

// helpers:
func expandHome(path string) (string, error)
func copyDir(src, dst string) error
func gitHeadSHA(dir string) (string, error)
func CopyTreeForTest(src, dst string) error                  // exported for tests
```

`Add` dispatches on `isLocalPath` / `isGitURL` / `isShorthand`. Local pins are recorded with `Source = <abs path>` and `Version = "local"`; git pins store the resolved URL plus the post-clone HEAD SHA. `Refresh` is intentionally non-destructive at the public level: callers (i.e. `cmd/update.go`) are expected to snapshot the existing `LocalPath` first via `os.Rename` to a `.bak-<unix-ts>` sibling, then call `Refresh`, then either delete the backup on success or `os.Rename` it back on failure.

`isNetworkError(err)` covers DNS failures, `*net.OpError`, and string needles for "connection refused" / "timeout" so the CLI can present a clean "registry not reachable" message instead of a raw stack trace.

**`types.go`** -- Constants + types:
```go
const DefaultIndexURL = "https://registry.spin.invalid/v1"   // RFC 2606 .invalid TLD

var ErrNotDeployed = fmt.Errorf("registry: public index not yet deployed; ...")
var ErrNotImplemented = ErrNotDeployed                       // backward-compat alias

type Entry struct {
    Name, Description, Language, Type, Version, Source, UpdatedAt string
    Tags                                                        []string
    Downloads                                                   int
}

type SearchResult struct {
    Query   string
    Total   int
    Entries []Entry
}

type Pinned struct {
    Name      string `json:"name"`        // template name
    Source    string `json:"source"`      // git URL or local path
    PinnedAt  string `json:"pinned_at"`   // ISO 8601
    Version   string `json:"version"`     // last-seen registry version, or "local", or SHA
    LocalPath string `json:"local_path"`  // absolute path on disk (v2.0+)
}
```

**`search.go`** -- `FormatSearch(*SearchResult, plain bool) string` (text + JSON), `SortByPopularity([]Entry) []Entry` (descending score), `truncate` helper.

## 6. Templates

Templates are **external** to spin. The repo ships no embedded scaffolds. A template is a directory with a `spin.toml` manifest and a `_base/` tree of overlay files. The tree is walked; files ending in `.tmpl` are rendered with `text/template` against the resolved param values; everything else is copied verbatim.

`spin init <name>` scaffolds a starter template directory (manifest + `_base/file.txt.tmpl` + README) in the CWD. The starter is renderable end-to-end: `spin new myapp --template <starter>` produces a working project without any edits to the manifest.

The `spin.toml` schema, the rendering pipeline, and the post-hook runner live in `internal/template/`. The pin cache (`~/.config/spin/templates/<name>/`) and the `~/.config/spin/pinned.json` registry live in `internal/registry/`. The pipeline is:

```
spec ──┬── isLocalPath  ──▶ loadLocal          (./abs/~/...)
       ├── isGitURL     ──▶ cloneGit           (--depth=1, GIT_TERMINAL_PROMPT=0)
       ├── loadPinned   ──▶ pinned.json lookup ─▶ loadPinnedFromPath
       └── isShorthand  ──▶ expand to https://github.com/user/repo.git ──▶ cloneGit
```

The loader prompts on stale cache (Reuse / Pin / Wipe / Cancel) and on invalid pinned records (Keep / Remove); both prompts are no-ops in non-TTY mode. A `.spinignore`-style exclude list and `min_spin_version` warnings are supported. The renderer deletes any `spin.toml` it finds in the destination (TPL-16) so the rendered project never carries the template's manifest.

---

## 7. Scripts (4 files in `scripts/`)

**`dogfood.sh`** -- CI dogfooding. Builds spin → scaffolds a fixture in a tempdir → runs `go mod tidy` → `CGO=0 go build` → `go test` → `check-v1-leaks`. Catches regressions in the scaffolder using the scaffolder.

**`check-v1-leaks.sh`** -- Cross-template v1-leak guard. 22 patterns:
- 16 charmbracelet v1 import paths
- 6 deprecated API usages (Bubble Tea v1 `View() string`, `WithAltScreen`, etc.)
- 1 deprecated air config (`bin = "tmp/main"` -- should be `cmd = "go build -o ./tmp/main ."`)

Splits out into **`check-air-bin.sh`** for focused failure messages.

**`check-taskfile-setup.sh`** -- Requires the `setup:` target with 4 installs: gofumpt, goimports, air, prism. Runs in CI before the build.

---

## 8. CI Workflows (2 files in `.github/workflows/`)

**`dogfood.yml`** -- Rebuilds spin in CI on every push. Triggers `scripts/dogfood.sh`.

**`ci.yml`** (or equivalent) -- Runs `go test ./...`, `go vet ./...`, `golangci-lint run`, and the check-v1-leaks guard on every PR.

---

## 9. Architecture -- One Concept, Two Pipelines

```
            ┌──────────────────────────────────────────────────────────┐
            │                       spin CLI                          │
            │                  (cobra + fang v2)                       │
            └──────────────────────────────────────────────────────────┘
                                  │
                                  ▼
            ┌──────────────────────────────────────────────────────────┐
            │                   internal/template/                    │
            │                                                           │
            │   loader  ──▶  parse spin.toml  ──▶  build huh form     │
            │       │              │                       │           │
            │       │              │     (or --param flag) │           │
            │       ▼              ▼                       ▼           │
            │   clone/copy  ResolveForm (defaults + user values)       │
            │       │              │                       │           │
            │       └──────────────┴───────────────────────┘           │
            │                              │                           │
            │                              ▼                           │
            │       Render(_base/)  ──▶  WriteFiles (path-safe)        │
            │                              │                           │
            │                              ▼                           │
            │                      RunPostHook (sh -c)                  │
            │                              │                           │
            │                              ▼                           │
            │                  deleteSpinToml (defensive)               │
            └──────────────────────────────────────────────────────────┘
                                  │
                                  ▼
            ┌──────────────────────────────────────────────────────────┐
            │                  internal/registry/                      │
            │                                                           │
            │   pinned.json  ◀──▶  ~/.config/spin/templates/<name>/     │
            │     (atomic)              (the on-disk cache)             │
            │                                                           │
            │   spin add     ──▶  copy or shallow-clone into cache      │
            │   spin list    ──▶  read pinned.json (table or --json)    │
            │   spin update  ──▶  snapshot → Refresh → restore on fail  │
            │   spin remove  ──▶  drop pin (--purge also drops cache)   │
            │   spin search  ──▶  public index (graceful "not deployed")│
            └──────────────────────────────────────────────────────────┘
```

### Why this shape

- **Template is the only concept.** The previous v2.x split into `Ecosystem` (compiled-in language) + `Template` (external git) + a `Builder` stub is gone. A template is self-describing: `spin.toml` declares its params, `[[post]]` steps, and metadata; the `_base/` tree is the artifact. `spin` is the load / prompt / render / post-hook pipeline. The template author owns the language, the framework, and the build commands.
- **Two pipelines, one CLI surface.** `internal/template/` does the rendering; `internal/registry/` does the pin store and public-index client. The CLI is a thin cobra layer over both. The form, the post-hook runner, the file writer, and the path-traversal guard are all in `internal/template/`. The pin list, refresh, and removal are all in `internal/registry/`.

Earlier `Ecosystem`, `Builder`, and universal `spin run <task>` runner concepts were removed during the v2.0-template pass. The runner in particular is no longer part of `spin` -- the rendered project owns its own task runner (Make, Task, npm, etc.) and `spin` exits after the post-hooks run.

---

## 10. Requirements Coverage

### v1.0 milestone (charmbracelet scaffolder, superseded)

The v1 scaffolder targeted the charmbracelet v1 stack with `--tui` / `--cli` / `--all` and per-library subflags, `spin build / test / vet / fmt / lint` as first-class commands, `spin run` for hot reload, and bundled `.air.toml` / `Taskfile.yml` / `Makefile` / pinned `go.mod` generation. v1.0 was a closed milestone; everything below ships in v2.0 instead. The v1 forms are removed in the post-milestone pass (§17.2 / §17.3).

### v2.0 milestone (41 requirements, validated 2026-06-09)

Template model (the only model):
- `spin new <name> --template <spec>` is the only scaffold entry point
- `spec` is a local path, git URL, pinned name, or `user/repo` shorthand (expanded to `https://github.com/user/repo.git`)
- Templates are shallow-cloned with `GIT_TERMINAL_PROMPT=0` and cached under `~/.config/spin/templates/<name>/`
- `spin.toml` declares metadata + params (8 types) + post-hooks + `min_spin_version`
- huh v2 form from params when TTY; defaults in non-TTY
- `--param key=value` (repeatable) overrides defaults for non-interactive CI use
- `spin.toml` is deleted after render; `[[post]]` runs on success
- Path-traversal guard in the file writer; defensive `spin.toml` removal (TPL-16)
- `spin init <name>` scaffolds a starter template directory (manifest + `_base/` + README) that is immediately renderable end-to-end

Pin + registry model:
- `spin add <spec>` pins a template to `~/.config/spin/pinned.json` + the on-disk cache
- `spin list` shows pinned templates (table by default; `--json` for `jq`/scripts)
- `spin update [name]` refreshes a pin's cache (or all pins). Old cache is moved to `.bak-<unix-ts>`; on success the backup is removed; on failure it is moved back into place
- `spin remove <name>` drops a pin (typo protection: unknown name → error). `--purge` also deletes the on-disk cache. Alias: `rm`
- `spin search <query>` against the hosted registry; friendly "not yet deployed" message on 404 / network error
- `SPIN_REGISTRY_URL` env override; `SPIN_REGISTRY` honored as a backward-compat fallback

Production-readiness:
- Rollback-aware `spin update` (snapshot → refresh → restore on failure)
- `spin list --json` stable wire format for scripts
- `--param key=value` non-interactive flow with per-type coercion
- README + PRD in sync with the v2 model

---

## 11. Constraints

1. **Tech stack**: Go 1.23+; cobra + charm.land/fang/v2 + charm.land/lipgloss/v2; charm v2 only for spin itself.
2. **Distribution**: single static binary; `go install github.com/N1xev/spin@latest`.
3. **No CGO**: `CGO_ENABLED=0` for spin.
4. **External templates only**: no compiled-in scaffolds, no embedded `_base/`. `spin init` produces a starter template that the user customises.
5. **External templates via git**: shallow clone, `GIT_TERMINAL_PROMPT=0`.
6. **Pin store**: `~/.config/spin/pinned.json` (atomic temp-file + rename writes); `~/.config/spin/templates/<name>/` for the on-disk cache. `XDG_CONFIG_HOME` overrides the config dir for tests / CI.
7. **Graceful degradation**: registry server not deployed → friendly message, never a stack trace. Non-TTY loader prompts (Reuse / Pin / Wipe / Cancel; Keep / Remove) become no-ops.
8. **Path-traversal guard**: any rendered file whose key contains `..`, starts with `/`, or contains a NUL byte is rejected.
9. **No v1 leftovers**: no `internal/scaffold/`, no `internal/ecosystems/`, no `internal/prompt/`, no `internal/runner/`, no `internal/update/`, no `cmd/new_charm.go`, no `cmd/new_rust.go`, no `cmd/ecosystem.go`, no `cmd/run.go`. The v1 dispatch surface (`spin new charm <name>`, `spin new rust <name>`, `spin run <task>`, `spin build / test / vet / fmt / lint`, `spin doctor`) is gone.

---

## 12. Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Binary name = `spin` | Short verb; evokes "spinning up" a project | Validated v1 |
| Template = the only concept | `spin.toml` + `_base/` cover the parameterized-scaffold use case without a separate ecosystem / builder contract. The template author owns the language, framework, and post-hooks; `spin` owns load / prompt / render / post | Locked v2 (post-milestone) |
| External templates only (no embedded scaffolds) | The previous embedded charm/rust overlay systems were Go-specific and duplicated what templates express more cleanly. Removing them keeps the binary small and lets any community template use any language | Locked v2 (post-milestone) |
| `spin.toml` for template params; deleted after use | Templates self-describe; the rendered project never carries the template's manifest | Locked v2 |
| 8 param types: text, textarea, number, select, multiselect, bool, path, secret | Covers 80% of template questions; huh v2 supports all of them | Locked v2 |
| `~/.config/spin/pinned.json` as the pin store (atomic temp-file + rename) | Survives crashes mid-write; `XDG_CONFIG_HOME` is honoured for tests | Locked v2 |
| `spin update` rolls back via `.bak-<unix-ts>` | `os.Rename` is atomic on the same filesystem; snapshot-then-refresh-then-restore is the standard pattern (git, apt, brew) | Locked v2 |
| `spin list --json` uses an explicit `pinnedRow` struct (not the on-disk `Pinned`) | Wire format and on-disk format are allowed to diverge; the JSON view can add/remove/rename fields without breaking pinned.json compatibility | Locked v2 |
| `--param key=value` is a `StringArray` (repeatable) with per-type coercion | CI / scripts need a way to bypass the huh form; one flag per value is the unix way; coercion per the param's declared type gives a clear error on bad input | Locked v2 |
| `spin init` writes a starter that is immediately renderable end-to-end | The value of `spin init` is "give me a working skeleton in 1 second" -- the starter has example params and a no-op `[[post]]` so `spin new myapp --template <starter>` succeeds without edits | Locked v2 |
| Huh v2 abort handling: direct `os.Exit(130)` to bypass fang's "Aborted." print | The fang default prints "Aborted." after the error returns, which looks like a crash; the 130 exit code is the standard SIGINT convention | Locked v2 |
| Charm v2 only in spin itself | v1 paths are deprecated and get no fixes; the README/PRD pin `charm.land/<lib>/v2` for every charm lib | Locked v2 |

---

## 13. Out of Scope (v2.x deferred)

- Compiled-in ecosystems for specific languages (the previous charm + rust ecosystems are gone; templates are the only extension surface)
- Non-Go TUI frameworks (tview, ratatui) -- out of scope; charm is not a `spin` concept any more, so this only matters to template authors
- External template loading via Go plugins -- v2.x; v2.0 is filesystem / git only
- `spin workspace` / `go.work` management -- v2.x
- GUI/TUI mode for the scaffolder itself (TUI is for the rendered project, not for `spin`)
- `spin release` wrapping goreleaser -- deferred
- Dockerfile / compose generation -- out of scope
- Universal `spin update` (the current update is gone with the runner; pinned-template cache refresh is the only "update" surface)
- Universal `spin run <task>` runner -- out of scope; the rendered project owns its own task runner
- `spin doctor` -- out of scope; the v1 doctor was a Go-only health check with no clean template-level analog

---

## 14. Backward Compatibility Strategy

v2.0 is the only shipped model. There is no v1 form kept:

- `spin new <name> --template <spec>` is the only scaffold entry point. There is no `spin new charm` / `spin new rust` / `spin new <ecosystem> <name>` form.
- The v1 task wrappers (`spin build / test / vet / fmt / lint`), the v1 `spin run` universal runner, the v1 `spin doctor`, and the v1 `spin new <name>` (no-ecosystem) form are all gone. The rendered project owns its own task runner.
- The previous `internal/scaffold/`, `internal/ecosystems/`, `internal/prompt/`, `internal/runner/`, and `internal/update/` packages are removed. The current internal surface is `internal/version/`, `internal/params/`, `internal/template/`, `internal/registry/`.
- Users upgrading from a v1 scaffolder should pull a v2 template (`spin add <user/repo>`) and re-scaffold; the template is the source of truth.

---

## 15. File-by-File Inventory

### Root

| File | Purpose |
|------|---------|
| `go.mod` | Module `github.com/N1xev/spin`, Go 1.23, charm v2 deps |
| `go.sum` | Dependency checksums |
| `Taskfile.yml` | Self-hosted `task` commands (test, build, vet, lint, dogfood) |
| `.golangci.yml` | Linter config (v2) |
| `.gitignore` | bin/, tmp/, dist/, fixtures, coverage |
| `CLAUDE.md` | Project briefing for AI agents (GSD workflow, stack, conventions) |
| `README.md` | User-facing intro + install + quickstart |
| `PRD.md` | **This document** |

### `cmd/` (14 files)

| File | Purpose |
|------|---------|
| `main.go` | `fang.Execute(ctx, rootCmd, fang.WithVersion(...))` + error→exit mapping |
| `root.go` | rootCmd, FlagErrorFunc with Levenshtein suggestions |
| `new.go` | `spin new <name> --template <spec>` -- local / git / pinned / shorthand dispatch |
| `add.go` | `spin add <spec>` -- pin a template locally |
| `list.go` | `spin list` -- show pinned templates (table or `--json`) |
| `update.go` | `spin update [name]` -- refresh a pin's cache, with rollback |
| `remove.go` | `spin remove <name>` -- drop a pin (`--purge` to also delete the cache) |
| `search.go` | `spin search <query>` -- query the public registry |
| `init.go` | `spin init <name>` -- scaffold a new template directory |
| `version.go` | `spin version` -- print the build version |
| `print.go` | Shared success/hint/error printers (lipgloss-styled) |
| `help_test.go` | `spin --help` content checks |
| `param_test.go` | `--param` flag + coercion unit tests |
| `init_test.go`, `update_test.go`, `remove_list_test.go` | CLI tests for the new commands |

### `internal/` (4 packages)

| Package | Files | Purpose |
|---------|-------|---------|
| `version/` | version.go | Build version (ldflags-overridable) |
| `params/` | param.go, form.go, parse.go, text.go, textarea.go, number.go, select.go, multiselect.go, bool.go, path.go, secret.go, parse_test.go, param_test.go | Typed form params + huh v2 adapters |
| `template/` | engine.go, template.go, loader.go, spin_toml.go, parse.go, post_hook.go, form.go, loader_test.go, template_test.go | External template system: load / parse / render / post-hook pipeline |
| `registry/` | client.go, types.go, search.go, client_test.go | Pin store + public registry client (graceful on network error) |

### `scripts/` (2 files)

| File | Purpose |
|------|---------|
| `dogfood.sh` | End-to-end smoke test: `spin init` -> render non-interactively -> assert pin store works. Matches `.github/workflows/dogfood.yml` so the same pipeline runs locally and in CI |
| `check-v1-leaks.sh` | v1 charmbracelet path / API pattern guard. Runs against `./internal` and `./cmd` in CI; against `internal/scaffold/templates` in the previous embedded-template model |

### `.github/workflows/` (2 files)

| File | Purpose |
|------|---------|
| `dogfood.yml` | Rebuilds spin on push |
| `ci.yml` | Test + vet + lint + v1-leak guard |

### `.github/workflows/` (2 files)

| File | Purpose |
|------|---------|
| `dogfood.yml` | Rebuilds spin on push |
| `ci.yml` | Test + vet + lint + v1-leak guard |

### `.planning/` (5 documents)

| File | Purpose |
|------|---------|
| `PROJECT.md` | Top-level project brief, requirements, decisions, evolution |
| `ROADMAP.md` | 5-phase roadmap (all complete as of 2026-06-09) |
| `STATE.md` | Live state: status=milestone_complete, 23/23 plans, 100% |
| `REQUIREMENTS.md` | Detailed v1 + v2 requirement list |
| `RESEARCH.md` | Initial domain research (charm stack) |

---

## 16. Quickstart Examples

```bash
# Scaffold from a local template directory
spin new myapp --template ~/code/templates/go-cli

# Scaffold from a git URL
spin new myapp --template https://github.com/me/go-cli-template.git

# Pin a template for offline use
spin add https://github.com/me/go-cli-template.git
spin new myapp --template go-cli-template

# Non-interactive: supply params as flags (great for CI)
spin new myapp --template go-cli \
  --param port=8080 \
  --param verbose=true \
  --param features=ci,release \
  --param name=myapp

# Preview the resolved params without writing files
spin new myapp --template go-cli --print-params

# Preview the file list without writing
spin new myapp --template go-cli --dry-run

# List, refresh, remove
spin list
spin list --json
spin update go-cli
spin update          # refresh every pin
spin remove go-cli
spin remove go-cli --purge

# Search the public registry (graceful "not yet deployed" until shipped)
spin search cobra

# Scaffold a new template directory
spin init my-template
spin new my-app --template ./my-template   # render it end-to-end
```

---

## 17. Recent Fixes (post-template pass, 2026-06-14)

The v2.0-template pass shrank `spin` to the load / prompt / render / post-hook pipeline plus a pin store, then closed the production-readiness gaps in the surface that remained. Sections 17.1-17.3 below are kept as a historical record of the older v2.0 milestone (compiled-in ecosystems, universal runner, charm+rust dispatch); the current model is described in §1-§16 above.

### 17.4 Production-readiness pass (2026-06-14)

| # | Change | Why |
|---|--------|-----|
| P1 | Added `--param key=value` (repeatable `StringArray`) to `cmd/new.go`; parses each entry as `key=value`, validates the key against `Template.Meta.Params`, and coerces the value to the param's declared type (number / bool / select / multiselect / text / textarea / path / secret) | CI / scripts need a way to bypass the huh form. The previous behaviour (interactive prompt only) made `spin new` unusable in pipelines |
| P2 | Added `internal/registry/Client.Refresh(pin Pinned) (Pinned, error)`; re-copies a local pin's source over the existing cache, or shallow-clones a git pin's source. Caller (`cmd/update.go`) is responsible for snapshotting first via `os.Rename` to `.bak-<unix-ts>` | Rollback-aware `spin update`: `Refresh` itself is destructive; the snapshot/restore pattern is owned by the caller so the same primitive can be reused for non-destructive flows later |
| P3 | Added `cmd/update.go` (was a stub); iterates `Client.ListPinned()` with an optional name filter, snapshots the existing cache, calls `Refresh`, and either removes the backup (success) or moves it back into place (failure) | `spin update [name]` was advertised but not implemented. The rollback primitive guarantees a failed `git clone` cannot lose the user's working cache |
| P4 | Added `cmd/remove.go` (`spin remove <name>` with `rm` alias and `--purge` flag). Unknown name → error with "run `spin list` to see pinned" hint; `--purge` deletes the on-disk cache after the pin is dropped | Users had `spin add` but no way to undo a typo or a stale pin. The cache was orphaned on every `spin add` and never reclaimed |
| P5 | Added `--json` to `cmd/list.go`. The wire format is a separate `pinnedRow` struct (Name, Version, PinnedAt, Description, Source, LocalPath) so the on-disk `Pinned` and the JSON view can evolve independently | `jq`-style scripting was the obvious follow-up to `spin add` / `spin list` but the table output was unparseable |
| P6 | Added `cmd/init.go` (`spin init <name>` with `--dir` flag). Writes `spin.toml` (with example params + a no-op `[[post]]`), `_base/file.txt.tmpl`, and `README.md`. Refuses to overwrite an existing dir. The starter is renderable end-to-end: `spin new myapp --template <starter>` succeeds without edits | The previous flow required users to hand-write a `spin.toml` (manifest schema, param block, post-hook) before they could render a project. The starter is "give me a working skeleton in 1 second" |
| P7 | Rewrote `cmd/root.go` Short/Long, `cmd/new.go` Short/Long, and the README quickstart to reflect the v2-template model. README is the user-facing intro; PRD is the technical deep-dive. Both pin the same set of commands and the same non-interactive `--param` flow | The previous README/PRD still described the v2.0-charm+rust model and the universal runner, neither of which exists in the repo any more. Users following the docs would hit dead ends |
| P8 | Added unit tests for every new command: `cmd/init_test.go` (init creates base tree, rejects collisions, honours `--dir`, rejects bad names, starter is renderable), `cmd/update_test.go` (refresh + rollback, success clears backup, no-name refreshes all), `cmd/remove_list_test.go` (remove drops the pin, --purge drops the cache, --json emits a valid array, empty list is `[]`), `cmd/param_test.go` (splitParamEntry, coerceParamValue, applyParamFlags, help mentions --param) | The new commands shipped in the same pass as the production-readiness refactor; tests are how we know `--param` coercion matches the param's type and that `spin update` actually rolls back on failure |

`go build ./...` and `go test ./...` pass across the remaining 4 internal packages (version, params, template, registry) and the 14-file `cmd/` tree.

---

## 18. Phase History

| Phase | Title | Status |
|-------|-------|--------|
| 1 | v1.0 Go+charm scaffolder | Validated 2025 (superseded) |
| 2 | Charm v2 library variants | Validated 2025 (superseded) |
| 3 | Build/test/lint wrappers | Validated 2025 (superseded) |
| 4 | Doctor + Update | Validated 2025 (superseded) |
| 5 | v2.0 compiled-in ecosystems + universal runner | Validated 2026-06-09 (replaced by v2.0-template) |
| 6 | **v2.0-template** -- external templates only, pin store, `--param` non-interactive, rollback-aware `update`, `spin init` | **Validated 2026-06-14** |

**v2.0-template success criteria (all met):**

1. `spin new <name> --template <spec>` works for local paths, git URLs, pinned names, and `user/repo` shorthand
2. `spin init <name>` produces a starter that is renderable end-to-end without edits
3. `spin update [name]` rolls back the cache on failure (`.bak-<unix-ts>` snapshot)
4. `spin list --json` is a stable wire format for `jq` / scripts
5. `spin remove <name>` drops the pin; `--purge` also drops the cache
6. `--param key=value` is repeatable and coerces per the param's declared type
7. Registry client degrades gracefully when the server is absent
8. `go build ./...` and `go test ./...` pass across `cmd/` + `internal/{version,params,template,registry}/`

---

## 19. v2.x Roadmap (Deferred Items)

1. **Hosted registry server** -- `spin-registry` (separate project; ships the index/search API; current `DefaultIndexURL` is a `.invalid` placeholder)
2. **`spin.lock`** -- pin the resolved template SHA per project, so `spin new` from a local path is reproducible across machines
3. **Deeper huh-driven scaffolder UI** -- optional TUI mode for `spin new` itself (currently CLI-only)
4. **`spin release`** -- goreleaser wrapper for the end user's project (out of scope for the scaffolder; lives in the rendered project)
5. **Wider template library** -- example templates for Python, Rust, Node, Deno, Zig, etc. (community-owned, registered via the hosted registry)

---

## 20. Non-Goals (Permanent)

- **Compiled-in ecosystems** for specific languages. Templates are the only extension surface; if you want a charm CLI starter, ship it as a template, not a built-in.
- **TUI mode for the scaffolder**. The scaffolder is a CLI; the TUI is for *rendered* projects.
- **Dockerfile / compose generation**. Out of scope; templates can include a `Dockerfile` if they want.
- **Universal `spin update` / `spin run`**. The v1 universal runner and the v2.0 universal update are both removed. The rendered project owns its own build/test/lint pipeline.

---

## 21. Command Reference

Every command, subcommand, and flag in the `cmd/` directory.

### `spin` (root) -- `cmd/root.go`

Project scaffolder for external templates. `spin` with no subcommand prints help. Unknown flags get a Levenshtein-based "Did you mean --X?" hint. `--version` prints `version.Version` (overridable via `-ldflags`).

### `spin new` -- `cmd/new.go`

Scaffold a new project from a template. The only scaffold entry point.

- `<name>` (positional) -- the destination project name. The output is written to `./<name>` (or `--dest`).
- `-t, --template <spec>` (required) -- the template source: local path (`/path`, `./path`, `~/path`), git URL (`https://...`, `git@...`, `git://...`, `ssh://...`), pinned name, or `user/repo` shorthand (expanded to `https://github.com/user/repo.git`).
- `-d, --dest <path>` -- destination directory (default: `./<name>`; `~` expanded).
- `--param key=value` -- repeatable. Non-interactive param value; coerced per the param's declared type. Validated against the template's `[params]` block.
- `--print-params` -- print the resolved params as JSON and exit (no files written).
- `--dry-run` -- render to a temp dir, print the file list, and clean up.

### `spin add` -- `cmd/add.go`

Pin a template locally so `spin new --template <name>` works offline.

- `<spec>` (positional) -- template source: local path, git URL, or `user/repo` shorthand. Stored in `~/.config/spin/pinned.json` and shallow-cloned into `~/.config/spin/templates/<name>/`.
- `--list` -- alias for `spin list` (show pinned and exit).

### `spin list` -- `cmd/list.go`

Show every pinned template.

- `--json` -- emit a stable JSON array of `pinnedRow` (Name, Version, PinnedAt, Description, Source, LocalPath) for `jq` / scripts. Empty list is `[]`.

### `spin update` -- `cmd/update.go`

Refresh a pinned template's on-disk cache (or every pin if no name given).

- `<name>` (optional positional) -- refresh a single pin; omit to refresh all.
- No flags. The refresh is rollback-aware: the old cache is moved to `.bak-<unix-ts>` first; on failure it is moved back; on success the backup is removed.

### `spin remove` -- `cmd/remove.go`

Drop a pin. Unknown name → error with "run `spin list` to see pinned" hint.

- `<name>` (positional) -- pin name to drop.
- `--purge` -- also delete the on-disk cache (default: leave the cache so a future `spin add` reuses it).
- Alias: `rm`.

### `spin search` -- `cmd/search.go`

Query the public template registry. Graceful "not yet deployed" message on 404 / network error.

- `<query>` (positional) -- search string.
- `--limit <n>` -- max results to show (default 20).
- `--json` -- machine-readable output.

### `spin init` -- `cmd/init.go`

Scaffold a new template directory. The result is immediately renderable: `spin new my-app --template <dir>` succeeds without edits.

- `<name>` (positional) -- template name. Refuses to overwrite an existing dir. Validates the name (rejects path separators, dots, NUL).
- `--dir <parent>` -- parent directory for the new template (default: current working directory).

### `spin version` -- `cmd/version.go`

Print `version.Version`. No flags.

---

## 22. Source of Truth

- **Project brief**: `.planning/PROJECT.md`
- **Live state**: `.planning/STATE.md`
- **Roadmap**: `.planning/ROADMAP.md`
- **Requirements**: `.planning/REQUIREMENTS.md`
- **This document**: `/PRD.md` (mirror of PROJECT.md + technical deep-dive)

---

*PRD generated 2026-06-14 from a complete file-by-file scan of the spin repository. v2.0-template milestone complete; production-readiness pass (`--param` non-interactive, `spin update` rollback, `spin list --json`, `spin init`, README + PRD sync) complete 2026-06-14.*
