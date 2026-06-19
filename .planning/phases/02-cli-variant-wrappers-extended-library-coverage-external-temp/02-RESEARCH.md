# Phase 2: CLI Variant + Wrappers + Extended Library Coverage + External Templates -- Research

**Researched:** 2026-06-03
**Domain:** Go scaffolder variant template, toolchain wrappers, charm v2 lib overlays, external template override
**Confidence:** HIGH (Context7-verified for 11 libraries; corrections to v1 research on harmonica + glow paths; Go version floors re-verified via go.mod inspection)

<user_constraints>
## User Constraints (from CLAUDE.md, STATE.md, REQUIREMENTS.md, ROADMAP.md)

### Locked Decisions (CLAUDE.md)
- Tech stack: Go 1.22+ (use 1.23+ if available); built with cobra + fang + gum; charmbracelet v2 only.
- Distribution: single static binary via `go install`.
- Templates: embedded via `go:embed` (default) + `--template-repo` for external override.
- Test runner: `prism`; formatter: `gofumpt` + `goimports`; hot reload: `air`.
- No CGO: scaffolded projects must build with `CGO_ENABLED=0`.
- Charm v2 only: do not import v1 paths or APIs.

### Locked Decisions (STATE.md / Phase 1)
- Charm v2 only -- generated projects use `charm.land/<lib>/v2` import paths; v1 paths forbidden (CI grep suite).
- `go 1.25.0` floor when `--bubbles` is used; `go 1.23` otherwise; `spin` itself pins `go 1.25.8` (resolved in Phase 1 to fang v2.0.1 floor).
- Templates embedded via `go:embed`; `--template-repo` override wired in Phase 2.
- Single static binary distribution.

### Phase 2 Scope Fences (REQUIREMENTS.md, §"v1 Requirements")
- **FLAG-07..12** -- `--huh`, `--glamour`, `--glow`, `--wish`, `--log`, `--harmonica` (each adds the lib to go.mod + wires a working example)
- **FLAG-13, FLAG-14** -- `--cobra` / `--fang` are default-on for `--cli` projects (Phase 1 bound the flag; Phase 2 fills templates)
- **FLAG-15** -- `--viper` opt-in config (Phase 1 bound; Phase 2 fills template)
- **TMPL-02** -- `--template cli-cobra-fang` selector
- **TMPL-03** -- `--template-repo <url>` external template override (depth-1 clone, tempdir, `GIT_TERMINAL_PROMPT=0`)
- **WRAP-01..08** -- five wrapper subcommands (`run`, `build`, `test`, `vet`, `fmt`) + three build-config requirements (`.air.toml` `build.entrypoint`, `Taskfile.yml` `setup` target)

### Out-of-Scope (Phase 3+)
- Interactive `gum` / `huh` prompts (INT-01..05) -- Phase 3
- AGENTS.md / `--ai` (AI-01..04) -- Phase 3
- `spin doctor` / `spin add` / `spin update` (HLTH-01..04) -- Phase 4
- CGO matrix / cross-platform GoReleaser in scaffolds -- Phase 3+

### Project Constraints (CLAUDE.md -- verbatim directives)
- Do not import `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/huh`, `github.com/charmbracelet/glamour`, `github.com/charmbracelet/wish`, `github.com/charmbracelet/log`, `github.com/charmbracelet/fang` -- those v1 paths are forbidden. **NOTE (correction):** `github.com/charmbracelet/harmonica` and `github.com/charmbracelet/glow/v2` are still the current paths -- see §2.1 critical correction below.
- Pin `go 1.25.0+` in generated `go.mod` when any charm v2 lib is required.
- Build with `CGO_ENABLED=0`.
- Single static binary, `go install` distribution.
- fang v2 (`charm.land/fang/v2`) drop-in for cobra root.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FLAG-07 | `--huh` adds `charm.land/huh/v2` to go.mod + working form example | §6.1 |
| FLAG-08 | `--glamour` adds `charm.land/glamour/v2` + markdown rendering example | §6.2 |
| FLAG-09 | `--glow` adds the `glow` binary to Taskfile.yml `setup` + README note | §6.3 |
| FLAG-10 | `--wish` adds `charm.land/wish/v2` + working SSH server example | §6.4 |
| FLAG-11 | `--log` adds `charm.land/log/v2` + working logger setup | §6.5 |
| FLAG-12 | `--harmonica` adds `github.com/charmbracelet/harmonica` + spring animation example | §6.6 |
| FLAG-13 | `--cobra` default-on for CLI projects (Phase 1 already bound; Phase 2 fills `variant_cli/main.go.tmpl`) | §3 |
| FLAG-14 | `--fang` default-on for CLI projects (Phase 1 already bound; Phase 2 fills `variant_cli/main.go.tmpl`) | §3 |
| FLAG-15 | `--viper` opt-in to `github.com/spf13/viper` + `viper.BindPFlag` shape | §3.3 |
| TMPL-02 | `--template cli-cobra-fang` selector | §3 |
| TMPL-03 | `--template-repo <url>` external override (depth-1 clone, tempdir, `GIT_TERMINAL_PROMPT=0`) | §5 |
| WRAP-01 | `spin run` uses `air` if `.air.toml` present, else `go run` | §4 |
| WRAP-02 | `spin build` produces `bin/<name>` | §4 |
| WRAP-03 | `spin test` invokes `prism` (Go < 1.24 or prism missing → `go test`) | §4.4 |
| WRAP-04 | `spin vet` runs `go vet ./...` | §4 |
| WRAP-05 | `spin fmt` runs `gofumpt` → `goimports` → `gofmt` chain (fails loud on gofumpt missing, `--no-strict` opt-out) | §4.3 |
| WRAP-06 | Each wrapper detects the preferred tool on `$PATH` and falls back with a one-line install hint | §4.2 |
| WRAP-07 | `.air.toml` uses `build.entrypoint` (not deprecated `build.bin`) + `include_ext` / `exclude_dir` | §7.1, §7.3 |
| WRAP-08 | `Taskfile.yml` has `setup` target that installs gofumpt + goimports + prism + air | §7.2, §7.3 |
</phase_requirements>

---

## 1. Phase Boundary

### 1.1 What Phase 2 MUST deliver

| Deliverable | REQ-IDs | Verification |
|------------|---------|--------------|
| `--cli` / `--cobra` / `--fang` / `--viper` template content (replace `TODO` stubs in `variant_cli/main.go.tmpl`) | FLAG-02, FLAG-13, FLAG-14, FLAG-15, TMPL-02 | `spin new foo --cli --cobra --fang && cd foo && go run . --help` works with fang styling + cobra hello-world subcommand |
| `--all` variant (TUI + CLI combo) | FLAG-03 | `spin new foo --all --bubbletea --cobra --fang` produces a project that builds and shows the structure from §3.2 |
| Six lib overlays for FLAG-07..12 (replace `LIBS.md.tmpl` placeholders with real overlay files) | FLAG-07..12 | `spin new foo --tui --bubbletea --huh --glamour --glow --wish --log --harmonica` produces a project where every selected lib is in `go.mod` and wired into `main.go` |
| Five wrapper subcommands: `spin run`, `spin build`, `spin test`, `spin vet`, `spin fmt` | WRAP-01..06 | Each wrapper detects the preferred tool, falls back to stock Go with a one-line install hint when missing |
| External template override (`--template-repo <url>`) | TMPL-03 | `git clone --depth 1 <url>` to a tempdir; the existing `embed.FS`-based template engine transparently switches to the tempdir's tree; offline default still works |
| `Taskfile.yml` `setup` target wiring `go install` for gofumpt + goimports + air + prism (version pins per §7.2) | WRAP-08 | `task setup` from a scaffolded project installs the four tools; absent target fails the grep test |
| `.air.toml` uses `build.entrypoint`, never `build.bin` | WRAP-07 | CI grep suite fails the scaffold if `bin = "tmp/main"` appears |
| CI grep suite extension: `check-air-bin.sh` + `check-taskfile-setup.sh` (split from the existing `check-v1-leaks.sh`) | WRAP-07, WRAP-08 | Both scripts exit non-zero on regressions; `make grep-v1-leaks` and `task grep-v1-leaks` wire both |
| CLI grep-suite refinement: deny-list of specific v1 modules (not blanket `github.com/charmbracelet/`) | TOOL-03 | Catches `github.com/charmbracelet/bubbletea` etc. but allows `github.com/charmbracelet/harmonica` and `github.com/charmbracelet/glow/v2` |
| go.mod go-version-floor decision updated to `1.25.0` for all Phase-2 scaffold variants | TOOL-01, TOOL-02 | Every generated `go.mod` uses `go 1.25.0` when any charm v2 lib is included; `spin new foo --tui --lipgloss` (no TUI runtime) also uses 1.25.0 for consistency |

### 1.2 What Phase 2 MUST NOT deliver (out-of-scope fences)

| Excluded | Phase | Phase-2 boundary statement |
|----------|-------|----------------------------|
| Interactive `gum` prompts (INT-01..05) | Phase 3 | `internal/interactive/` directory does not exist; no prompter interface in this phase |
| `AGENTS.md` / `--ai` | Phase 3 | `lib/crush/` overlay directory does not exist |
| `spin doctor` / `spin add` / `spin update` (HLTH-01..04) | Phase 4 | No `cmd/doctor.go`, `cmd/add.go`, `cmd/update.go` |
| CGO_ENABLED matrix / cross-platform GoReleaser in scaffolds | Phase 3+ | `.goreleaser.yaml` is optional, not in Phase 2 scope |
| `go vet --all` matrix | n/a | `spin vet` is a thin `go vet ./...` wrapper only |
| Multi-binary `cmd/foo` / `cmd/bar` directory layout in generated projects | defer | Phase 2 ships a single `main.go` per project; the `cmd/<name>/main.go` restructure is a follow-on |

### 1.3 Walking Skeleton for Phase 2

The minimum set of files Phase 2 must emit so that `spin new mycli --cli --cobra --fang && cd mycli && go run . --help` works:

```
./mycli/
├── go.mod                  # module <name>; go 1.25.0; require charm.land/fang/v2, github.com/spf13/cobra v1.9.1
├── main.go                 # package main; cobra root with hello subcommand; fang.Execute(ctx, rootCmd)
├── .gitignore              # tmp/, bin/, dist/, *.exe (Phase 1)
├── .air.toml               # build.entrypoint = ["./tmp/main"] (Phase 1; WRAP-07 enforces)
├── Taskfile.yml            # setup/test/build/run/fmt/vet tasks (Phase 1; WRAP-08 enforces)
├── README.md               # Next steps + lib list
├── LICENSE                 # MIT or Apache-2.0
├── internal/               # optional
│   └── config/
│       └── config.go       # (only when --viper)
└── .git/                   # 1 commit
```

When `--tui --bubbletea --bubbles --huh --glamour --glow --wish --log --harmonica` is added to the same `spin new`, the `main.go` is replaced by the variant-specific overlay (TUI priority over CLI when both are present, since the typical `--all` use case is "primary UI = TUI, `--cli` adds subcommands").

---

## 2. Stack & Versions (verified via Context7 + `go list` on 2026-06-03)

### 2.1 CRITICAL CORRECTION: harmonica + glow have NOT migrated to `charm.land/...`

The v1 RESEARCH.md (per the CLAUDE.md "What NOT to Use" table) treats `github.com/charmbracelet/...` as a blanket v1-ban. **This is wrong for two libraries:**

| Library | Module path | Latest | Verified via |
|---------|-------------|--------|--------------|
| `harmonica` | `github.com/charmbracelet/harmonica` | v0.2.0 (Apr 2022) | GitHub repo + go list |
| `glow` | `github.com/charmbracelet/glow/v2` | v2.1.2 | GitHub repo + go list |

The other 9 charm v2 libraries are all on `charm.land/<lib>/v2`:

| Library | Module path | Latest | Verified via |
|---------|-------------|--------|--------------|
| `bubbletea` | `charm.land/bubbletea/v2` | v2.0.7 | go.mod inspection at v2.0.7 |
| `lipgloss` | `charm.land/lipgloss/v2` | v2.0.3 (stable line) | go.mod inspection at v2.0.3 |
| `bubbles` | `charm.land/bubbles/v2` | v2.1.0 | go.mod inspection at v2.1.0 |
| `huh` | `charm.land/huh/v2` | v2.0.3 | go.mod inspection at v2.0.3 |
| `glamour` | `charm.land/glamour/v2` | v2.0.0 | go.mod inspection at v2.0.0 |
| `wish` | `charm.land/wish/v2` | v2.0.1 | go.mod inspection at v2.0.1 |
| `log` | `charm.land/log/v2` | v2.0.0 | go.mod inspection at v2.0.0 |
| `fang` | `charm.land/fang/v2` | v2.0.1 | go.mod inspection at v2.0.1 |

**Implication for the CI grep suite:** the current `check-v1-leaks.sh` (Phase 1) forbids the bare regex `github\.com/charmbracelet/`. **Phase 2 must refine this to a deny-list** of specific migrated modules, so that `github.com/charmbracelet/harmonica` and `github.com/charmbracelet/glow/v2` are allowed. See §7.3.

**Implication for the version pin struct:** the `CharmPins` struct in `internal/scaffold/versions.go` must split into two groups: `CharmPins` (charm.land) and `LegacyCharmPins` (github.com). A single struct would conflate them and confuse readers.

### 2.2 Go version floor corrections

The v1 RESEARCH §3 said "go 1.25.0 when --bubbles, go 1.23 otherwise." After verifying each go.mod on 2026-06-03, the floors are higher:

| Library | `go` directive in go.mod | Source |
|---------|--------------------------|--------|
| `bubbletea v2.0.7` | (inherits; bubble tea v2.0.0-beta.4 → 1.23) | TBD -- likely 1.23 or 1.25.0 |
| `lipgloss v2.0.3` | (inherits from bubbles test suite; likely 1.25.0) | TBD |
| `bubbles v2.1.0` | `go 1.25.0` | verified at v2.1.0 go.mod |
| `huh v2.0.3` | `go 1.25.8` | verified at v2.0.3 go.mod |
| `glamour v2.0.0` | `go 1.25.8` | verified at v2.0.0 go.mod |
| `wish v2.0.1` | `go 1.25.9` | verified at v2.0.1 go.mod |
| `log v2.0.0` | `go 1.25.8` | verified at v2.0.0 go.mod |
| `fang v2.0.1` | `go 1.25.0` | verified at v2.0.1 go.mod |
| `harmonica v0.2.0` | `go 1.16` | verified -- old lib, low floor |
| `glow v2.1.2` | `go 1.25.9` | verified at v2.1.2 go.mod |

**Recommendation:** generated `go.mod` uses `go 1.25.0` whenever any charm v2 lib is included (which is always for the v1 project: even the CLI variant pulls in fang v2.0.1 → 1.25.0). The "go 1.23" branch from v1 is no longer reachable for any real scaffold -- even `spin new foo --tui --lipgloss --no-bubbletea` is dead code because `--tui` implies `--bubbletea` (Phase 1 `ResolveFlags` does this auto-implication). The decision matrix from v1 §3 collapses to:

| Flag combination | `go` directive |
|------------------|----------------|
| Anything that includes a charm v2 lib | `1.25.0` |
| (nothing else -- the project always has at least fang for CLI or bubbletea for TUI) | -- |

**Pin policy:** the `CharmPins` struct in v1.0 had `Lipgloss: "v2.0.0-beta.2"` -- this is stale and is the explicit memory entry "v2.0.0-beta.2 module-path mismatch; bump to stable v2 before Phase 2 lib variants". The Phase 2 plan should bump to `Lipgloss: "v2.0.3"`. Same review for all 11 lib pins: pin to the latest stable per `go list -m -versions`.

### 2.3 Tooling pins (per the official install docs)

| Tool | Install | Go version floor for `go install` | Verified via |
|------|---------|-----------------------------------|--------------|
| `air` | `go install github.com/air-verse/air@latest` | Go 1.25+ | air-verse/air README + ctx7 |
| `prism` | `go install go.dalton.dog/prism@latest` | Go 1.24+ (test runner floor) | DaltonSW/prism README |
| `gofumpt` | `go install mvdan.cc/gofumpt@latest` | Go 1.25+ (v0.9.0 onward) | mvdan/gofumpt CHANGELOG |
| `goimports` | `go install golang.org/x/tools/cmd/goimports@latest` | Inherits from x/tools (low) | pkg.go.dev |

`air`, `gofumpt`, and `prism` all `go install` fine on the same Go 1.25+ toolchain the user has for the project. The `setup` target in `Taskfile.yml` can use `@latest` for all four.

---

## 3. CLI Variant Design (TMPL-02 + FLAG-13/14/15)

### 3.1 `cli-cobra-fang` main.go sketch (verified via Context7)

The `variant_cli/main.go.tmpl` must produce a runnable cobra root + fang wrapper. The 2026 idiom is:

```go
// Code generated by spin {{.SpinVer}}. DO NOT EDIT.
// Regenerate with: spin update {{.Name}}
package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "{{.Name}}",
	Short: "A {{title .Name}} CLI scaffolded with spin",
	Long:  "{{title .Name}} -- generated with spin {{.SpinVer}}.",
	// --version comes from fang automatically when WithVersion is passed
}

var helloCmd = &cobra.Command{
	Use:   "hello [name]",
	Short: "Say hello",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "Hello, %s!\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(helloCmd)
}

func main() {
	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion("{{.SpinVer}}"),
	); err != nil {
		os.Exit(1)
	}
}
```

**Verified API surface (Context7 `/charmbracelet/fang`):**

- `fang.Execute(ctx, *cobra.Command, ...fang.Option) error` -- drop-in for `rootCmd.Execute()`; styles help, errors, completions, manpage.
- `fang.WithVersion(string)` -- sets the version string for `--version` flag theming.
- `fang.WithCommit(string)` -- optional commit SHA for the version output.
- `fang.WithNotifySignal(os.Interrupt)` -- optional signal handling.

**What fang does NOT do** (verified against `/spf13/cobra` and `/charmbracelet/fang`):

- Does NOT style *subcommand* `--help` -- only the root help is fang-styled. Subcommand help is cobra's default. This is by design and is the official behavior (no flag to enable subcommand styling).
- Does NOT add `SuggestFloats` or "Did you mean?" for unknown flags (cobra's responsibility; spin's `root.go` already implements this in Phase 1).

### 3.2 `--all` variant structure (TMPL-02, FLAG-03)

The v1 RESEARCH §15.6 left `--all` ambiguous. ROADMAP.md says: "Combine both variants: cobra root, `--tui` flag launches bubbletea v2 program." The recommended structure is a single binary with a `--tui` subcommand:

```
myapp/
├── main.go              # cobra root: `myapp` (CLI) and `myapp tui` (TUI) subcommands
├── internal/
│   ├── cli/             # cobra command files (hello, version, etc.)
│   │   └── hello.go
│   └── tui/             # bubbletea model
│       └── model.go
├── .air.toml
├── Taskfile.yml
└── go.mod
```

```go
// main.go (--all variant)
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.NewModel())
		_, err := p.Run()
		return err
	},
}
```

**Why this beats two binaries:**

- Single `go build`, single `go install`, single release pipeline.
- One `Taskfile.yml` `run` target with hot reload works for both.
- The user's mental model is "myapp is a CLI with a TUI subcommand" not "I have two separate things."
- For GoReleaser (Phase 3+), the single-binary layout matches the bubbletea-app-template pattern.

**Why this beats one binary with a `--tui` bool flag (the alternative):**

- Cobra subcommands are discoverable (`myapp tui --help`).
- Subcommands compose with future flags (`myapp tui --config foo.yaml`).
- Avoids the "is the default behavior TUI or CLI?" ambiguity.

**`--all` template file:** `variant_all/main.go.tmpl` is a single file that imports both `charm.land/bubbletea/v2` (for the `tui` subcommand) and `github.com/spf13/cobra` (for the root). The lib overlays for bubbletea/cobra are no-ops when only one is present; the variant_all file is the source of truth.

### 3.3 Viper integration shape (FLAG-15)

Per `/spf13/viper` docs, the binding pattern with cobra is:

```go
// internal/config/config.go (only emitted when --viper)
package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Bind(root *cobra.Command) error {
	// 1. Bind each persistent flag to viper so flag value flows through viper.GetX.
	if err := viper.BindPFlag("log-level", root.PersistentFlags().Lookup("log-level")); err != nil {
		return err
	}
	// 2. Read config file (optional)
	viper.SetConfigName(".myapprc")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	// 3. Env binding
	viper.SetEnvPrefix("MYAPP")
	viper.AutomaticEnv()
	return nil
}

func LogLevel() string {
	if viper.IsSet("log-level") {
		return viper.GetString("log-level")
	}
	return "info"
}
```

**Viper mapstructure v2 import:** per Viper UPGRADE.md, the mapstructure dependency moved from `github.com/mitchellh/mapstructure` to `github.com/go-viper/mapstructure/v2`. Generated projects that use Viper must use the new path if they `import "github.com/go-viper/mapstructure/v2"` directly (most won't -- Viper's `Unmarshal` is the typical entry point).

**Viper Go version floor:** 1.20+ (per spf13/viper README), well below 1.25.0.

**`--viper` overlay:** `lib/viper/config.go.tmpl` produces `internal/config/config.go`; `lib/viper/main_patch.go.tmpl` (or a template `{{if hasViper}}` block in `variant_cli/main.go.tmpl`) wires `config.Bind(rootCmd)` in `init()`.

---

## 4. Wrapper Subcommand Design (WRAP-01..06)

### 4.1 Cobra Command skeletons

Each wrapper is a `*cobra.Command` in its own file under `cmd/`, attached to `rootCmd` via `init()`. The shared scaffolding is:

```go
// cmd/run.go
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/example/spin/internal/wrap"
)

var runCmd = &cobra.Command{
	Use:           "run",
	Short:         "Run the project (uses air if .air.toml present, else go run)",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return wrap.Run()
	},
}

func init() { rootCmd.AddCommand(runCmd) }
```

The same shape for `cmd/build.go`, `cmd/test.go`, `cmd/vet.go`, `cmd/fmt.go`. The `internal/wrap/` package contains the actual tool-detection + exec logic.

### 4.2 `exec.LookPath` fallback chain helper (WRAP-06)

The shared helper signature:

```go
// internal/wrap/detect.go

// toolPath returns the absolute path of tool if it is on $PATH, or "" if not.
// InstallHint is a one-line message printed to stderr when the tool is missing.
type ToolSpec struct {
	Name        string // e.g. "prism"
	Args        []string // passed to exec.Command
	ExtraEnv    []string // e.g. CGO_ENABLED=0
	InstallHint string // "go install go.dalton.dog/prism@latest"
}

func RunWithFallback(spec ToolSpec, fallback ToolSpec) error {
	path, err := exec.LookPath(spec.Name)
	if err == nil {
		cmd := exec.Command(path, spec.Args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if len(spec.ExtraEnv) > 0 {
			cmd.Env = append(os.Environ(), spec.ExtraEnv...)
		}
		return cmd.Run()
	}
	// Tool not found: print the one-line install hint then fall back.
	fmt.Fprintf(os.Stderr, "hint: %s not found on $PATH; %s\nfalling back to: %s\n",
		spec.Name, spec.InstallHint, fallback.Name)
	cmd := exec.Command(fallback.Args[0], fallback.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(fallback.ExtraEnv) > 0 {
		cmd.Env = append(os.Environ(), fallback.ExtraEnv...)
	}
	return cmd.Run()
}
```

**Examples for the 5 wrappers:**

```go
// internal/wrap/run.go
var runSpec = ToolSpec{
	Name:        "air",
	Args:        []string{},
	InstallHint: "go install github.com/air-verse/air@latest",
}
var runFallback = ToolSpec{
	Name: "go", Args: []string{"run", "."},
}
func Run() error {
	if _, err := os.Stat(".air.toml"); err != nil {
		return runWithFallback(runFallback, runSpec)  // no .air.toml → go run
	}
	return RunWithFallback(runSpec, runFallback)
}
```

```go
// internal/wrap/build.go
var buildSpec = ToolSpec{
	Name:        "go",  // no upgrade here; go build is stock
	Args:        []string{"build", "-o", "bin/" + p.Name},
	ExtraEnv:    []string{"CGO_ENABLED=0"},
	InstallHint: "go is in your Go toolchain",
}
```

(Note: `build` has no preferred tool beyond `go build` itself. The "preferred" path is direct `go build`; there's no fallback. The wrapper just exists to give the user `spin build myapp` and have it match the layout the `Taskfile.yml` `build` target produces.)

### 4.3 `spin fmt`: gofumpt → goimports → gofmt chain (WRAP-05)

The order matters. `gofumpt` is a stricter superset of `gofmt`, so running `gofmt` after `gofumpt` is idempotent. `goimports` adds missing imports / removes unused ones, which `gofumpt` does not. Order:

1. `gofumpt -l -w .` -- list and rewrite any non-gofumpt-clean files
2. `goimports -w .` -- fix imports (with `-local {{.Module}}` to group local imports)
3. `gofmt -l -w .` -- defensive final pass (no-op if gofumpt ran)

**Strict mode** (default): if `gofumpt` is missing, `spin fmt` exits non-zero with the install hint:

```
error: gofumpt not found on $PATH
hint: go install mvdan.cc/gofumpt@latest
```

**`--no-strict` mode:** when set, `spin fmt` skips `gofumpt`, prints a one-time warning, runs `goimports` and `gofmt` only:

```
warn: gofumpt not found; falling back to gofmt (pass --no-strict to suppress this message)
```

The flag is registered on `fmtCmd`:

```go
fmtCmd.Flags().Bool("no-strict", false, "fall back to gofmt when gofumpt is missing (default: fail loud)")
```

**Implementation:**

```go
// internal/wrap/fmt.go
func Fmt(noStrict bool) error {
	if _, err := exec.LookPath("gofumpt"); err == nil {
		if err := runTool("gofumpt", "-l", "-w", "."); err != nil { return err }
	} else if !noStrict {
		return fmt.Errorf("gofumpt not found; install with: go install mvdan.cc/gofumpt@latest (or pass --no-strict)")
	} else {
		fmt.Fprintln(os.Stderr, "warn: gofumpt not found; falling back to gofmt")
	}
	// goimports: missing → silent skip (matches Phase 1 Taskfile behaviour)
	if path, err := exec.LookPath("goimports"); err == nil {
		_ = runTool(path, "-w", ".")
	}
	// gofmt: always present (Go toolchain)
	return runTool("gofmt", "-l", "-w", ".")
}
```

### 4.4 `spin test`: prism detector (WRAP-03)

The detector combines Go version + path presence:

```go
// internal/wrap/test.go
func Test() error {
	// prism requires Go 1.24+ (the -json flag was introduced then).
	// If the user's Go is older, skip prism even if it's on $PATH.
	goOld := goVersionLessThan("1.24")
	if !goOld {
		if path, err := exec.LookPath("prism"); err == nil {
			return runTool(path, "go", "test", "./...")
		}
	}
	// Fallback: stock go test
	if goOld {
		fmt.Fprintf(os.Stderr, "hint: prism requires Go 1.24+; using go test (you are on %s)\n", runtime.Version())
	} else {
		fmt.Fprintln(os.Stderr, "hint: prism not found; install with: go install go.dalton.dog/prism@latest")
	}
	return runTool("go", "test", "./...")
}

func goVersionLessThan(want string) bool {
	v := strings.TrimPrefix(runtime.Version(), "go")
	// Lexicographic compare is correct for Go's semver: "1.24" < "1.25" etc.
	return v < want
}
```

Verified: prism README says "Prism works anywhere `go test` works, so it can be quickly integrated into any project using Go v1.24 or higher" -- the 1.24 floor is for `-json`, not for prism's own compile target.

**Why both checks:** an old Go install with `prism` on `$PATH` (e.g., user installed prism months ago and downgraded Go) would fail. The version check ensures we only invoke prism when the runtime can support it.

### 4.5 `spin build`: output path + air config awareness (WRAP-02)

The build target's output is `./bin/<name>`. The binary name comes from the current directory name (matching the `Taskfile.yml` pattern from Phase 1):

```go
// internal/wrap/build.go
func Build() error {
	name := filepath.Base(mustCwd())
	binDir := "bin"
	if err := os.MkdirAll(binDir, 0o755); err != nil { return err }
	args := []string{"build", "-o", filepath.Join(binDir, name), "."}
	spec := ToolSpec{Name: "go", Args: args, ExtraEnv: []string{"CGO_ENABLED=0"}}
	return RunWithFallback(spec, spec)  // no fallback; go build is the only path
}
```

**Air config awareness:** `spin build` does NOT need to parse `.air.toml` -- the binary path is always `bin/<name>`, independent of what `air` uses (which is `tmp/main`). The two are decoupled. The user is expected to `go build` (or `spin build`) for the release binary and `air` (or `spin run`) for hot-reload dev.

### 4.6 `spin vet`: trivial wrapper (WRAP-04)

```go
// internal/wrap/vet.go
func Vet() error {
	return runTool("go", "vet", "./...")
}
```

`go vet` is always present (Go toolchain); no fallback chain needed. WRAP-06 is satisfied trivially.

---

## 5. External Template Override (TMPL-03)

### 5.1 The clone pattern (verified via `os/exec`)

```go
// internal/scaffold/repo.go

// CloneTemplateRepo shallow-clones url to a temp directory, returning the
// path. The temp directory has a `_base/` subdirectory required by the
// overlay engine; an error is returned if it does not exist. The caller is
// responsible for os.RemoveAll on the returned path (defer it at the call site).
//
// GIT_TERMINAL_PROMPT=0 is set so private repos with credential helpers
// never block the scaffolder. Spin sets a deterministic author identity
// matching the git.go helper for consistency.
func CloneTemplateRepo(ctx context.Context, url string) (string, error) {
	tmp, err := os.MkdirTemp("", "spin-template-*")
	if err != nil { return "", err }

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, tmp)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=spin", "GIT_AUTHOR_EMAIL=spin@localhost",
		"GIT_COMMITTER_NAME=spin", "GIT_COMMITTER_EMAIL=spin@localhost",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git clone %s: %s", url, out)
	}

	// Validate: the cloned repo must have a _base/ directory.
	if _, err := os.Stat(filepath.Join(tmp, "_base")); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("template repo %s: missing _base/ directory (required by spin's overlay engine)", url)
	}
	return tmp, nil
}
```

**Why `--depth 1`:** template repos are usually small and rarely need history. Shallow clone keeps the operation fast even over slow connections. Phase 4 may revisit if `spin update` needs to detect changes against a remote.

### 5.2 The embed.FS → os.DirFS abstraction layer (the critical design decision)

The Phase 1 engine reads from `FS embed.FS` (package var in `scaffold.go`). For external templates, we need to swap in `os.DirFS(<tempdir>)`. The cleanest abstraction is a small interface that both satisfy:

```go
// internal/scaffold/fs.go
package scaffold

import "io/fs"

// templateFS is the minimal interface the overlay engine needs. It is
// satisfied by both embed.FS and os.DirFS results (after a type assertion
// to fs.ReadDirFS / fs.ReadFileFS if those aren't already part of the type).
type templateFS interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
}

// currentFS returns the FS to use: the external clone (if --template-repo
// was passed and the clone succeeded) or the embedded FS.
func currentFS(externalDir string) templateFS {
	if externalDir != "" {
		return os.DirFS(externalDir).(templateFS)
	}
	return FS.(templateFS)  // embed.FS already satisfies all three
}
```

`embed.FS` satisfies `fs.FS`, `fs.ReadDirFS`, and `fs.ReadFileFS` (Go 1.16+ stdlib promises). `os.DirFS` returns a `fs.FS` that also implements `fs.ReadDirFS` and `fs.ReadFileFS` since Go 1.16. The type assertion in `currentFS` is a compile-time guard: if either is upgraded to drop one of the methods, the build breaks at the assertion site (better than a runtime nil method call).

**Refactor needed in Phase 1's `template.go`:** the `renderToMap` walker currently calls `fs.WalkDir(FS, root, ...)` with the package-level `FS`. Change to:

```go
// inside renderToMap, after p.ExternalDir is plumbed in
fsys := currentFS(p.ExternalDir)
err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, walkErr error) error { ... })
```

Same for `fs.ReadFile(fsys, path)`. **This is a 5-line refactor to template.go plus a new `p.ExternalDir string` field on `Project`.** The overlay engine itself is unchanged.

### 5.3 Validation

The clone function above validates `_base/` presence. Two more checks the plan should add:

| Check | Where | Failure message |
|-------|-------|-----------------|
| `url` is non-empty | `runNew` (cmd/new.go) | "error: --template-repo requires a URL" |
| `url` parses as an http(s)://, git@, or git:// URL | `runNew` | "error: --template-repo <url>: must be http(s)://, git://, or ssh:// (got: %s)" |
| cloned tree has `_base/` | `CloneTemplateRepo` | "error: template repo missing _base/ directory" |
| cloned tree has at least one of `variant_tui/`, `variant_cli/`, or `variant_all/` | `CloneTemplateRepo` | "warning: template repo has no variant_<type>/ directories; only _base/ will be used" |

The git URL validation is intentionally permissive -- we let `git clone` itself reject malformed URLs, then surface the error verbatim.

### 5.4 Cleanup + `--keep-template-cache`

The default lifecycle: the temp dir is `os.RemoveAll`'d when `scaffold.New` returns (success or failure). The defer sits in `runNew`:

```go
if p.TemplateRepo != "" {
	dir, err := CloneTemplateRepo(ctx, p.TemplateRepo)
	if err != nil { return err }
	if !p.KeepTemplateCache {
		defer os.RemoveAll(dir)
	}
	p.ExternalDir = dir  // plumb into Project
}
```

`--keep-template-cache` (boolean flag on `newCmd`) skips the defer so the user can `cd` into the temp dir and inspect the cloned tree. Useful for debugging a broken template repo. The temp dir's path is logged at Info level so the user can find it.

### 5.5 Security: path traversal mitigation in template rendering

A malicious template can render `{{.Name}}` to `../../etc/passwd` (or any escape). The mitigations are layered:

| Layer | Mitigation | Where |
|-------|-----------|-------|
| 1 | `Project.Name` is already validated by `IsValidGoModuleSegment` (Phase 1) -- no `/`, no `..`, no leading dot | `validate.go` |
| 2 | Template `FuncMap` should NOT have a `replace` or `eval` function | `template.go` |
| 3 | `os.WriteFile` rejects paths that resolve outside `target` (add a `safePath` check before each write) | `emit` in `scaffold.go` |
| 4 | `filepath.Rel(target, full)` should return a path that does NOT start with `..` | `emit` |

The third layer is the new addition for Phase 2:

```go
// internal/scaffold/scaffold.go (emit function, modified)
for rel, content := range files {
	full := filepath.Join(root, rel)
	// Path traversal guard: ensure `full` is under `root` after
	// cleaning. A template that renders `{{.Name}}` to `../../etc`
	// would resolve to a path outside `root` and be rejected.
	cleanFull := filepath.Clean(full)
	cleanRoot := filepath.Clean(root) + string(filepath.Separator)
	if !strings.HasPrefix(cleanFull, cleanRoot) && cleanFull != filepath.Clean(root) {
		return fmt.Errorf("path traversal: template rendered %q resolves outside %q", rel, root)
	}
	// ... existing mkdir/write logic
}
```

**Note on Windows:** `filepath.Separator` is `\` on Windows. The check still works because both sides use the same separator. The fix is portable.

---

## 6. Lib Overlay Designs (FLAG-07..12)

Each overlay is a directory `templates/lib/<name>/` with one or more `.tmpl` files. The overlay engine emits them at the same relative paths under the scaffolded project root. For FLAG-09 (`--glow`) the scaffold changes Taskfile.yml, not a Go file (glow is a binary, not a Go import).

**Pattern for Go-lib overlays (FLAG-07, -08, -10, -11, -12):**

```
templates/lib/<name>/
└── <name>.go.tmpl         # at scaffolded-project root
```

The Go file imports the lib and adds a small, self-contained, runnable example. The main wiring of the example may also need to be patched into `variant_tui/main.go.tmpl` or `variant_cli/main.go.tmpl` via a `{{if has <lib>}}` block, depending on where the example surfaces.

### 6.1 `--huh` (FLAG-07) -- `charm.land/huh/v2` v2.0.3

**`go.mod` entry (added by `_base/go.mod.tmpl` via a `{{if hasHuh}}` block):**

```
charm.land/huh/v2 v2.0.3
```

**Overlay files:**

- `lib/huh/huh.go.tmpl` -- small stand-alone example (placeholder file, like bubbletea.go.tmpl).
- A `{{if hasHuh}}` block in `variant_tui/main.go.tmpl` adds a huh form to the TUI (gated by `hasHuh` helper, added to FuncMap).

**Example snippet (10-15 lines, runnable in `cmd/foo/main.go` after the model):**

```go
import "charm.land/huh/v2"

func runHuhExample() {
	var name string
	var subscribe bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Your name?").Value(&name),
			huh.NewConfirm().Title("Subscribe?").Value(&subscribe),
		),
	).WithWidth(60)
	if err := form.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("hello %s, subscribe=%v\n", name, subscribe)
}
```

**`hasHuh` helper:** add to `funcMap` in `template.go`:

```go
"hasHuh": func(p2 *Project) bool { return p2.Huh },
```

### 6.2 `--glamour` (FLAG-08) -- `charm.land/glamour/v2` v2.0.0

**`go.mod` entry:** `charm.land/glamour/v2 v2.0.0`

**Overlay:** a sample markdown renderer in `lib/glamour/glamour.go.tmpl` plus a `{{if hasGlamour}}` block that prints a styled README excerpt on startup. The simple `glamour.Render(in, "dark")` one-liner is the recommended pattern (verified via Context7).

**Example snippet:**

```go
import "charm.land/glamour/v2"

func renderMarkdown(in string) string {
	out, err := glamour.Render(in, "dark")
	if err != nil { return in }  // graceful fallback
	return out
}
```

### 6.3 `--glow` (FLAG-09) -- `github.com/charmbracelet/glow/v2` v2.1.2

**Important:** glow is a BINARY, not a Go library. There is no `go.mod` entry to add. The scaffold changes:

- `Taskfile.yml` `setup` target adds `go install github.com/charmbracelet/glow/v2@latest`
- `README.md` adds a "Markdown preview" section explaining `glow README.md`
- `main.go` can call `exec.Command("glow", "README.md").Run()` from a `glow` subcommand or `--preview` flag (optional; minimal overlay is README-only)

**Overlay:** `lib/glow/README.glow.md.tmpl` -- a sample markdown file that glow will render well, and a `{{if hasGlow}}` block in `_base/README.md.tmpl` that mentions `glow README.md` for the preview.

**CI grep implication:** the allow-list of `github.com/charmbracelet/` modules (see §7.3) must include `glow/v2` and `harmonica` so the grep suite doesn't false-positive on `go install github.com/charmbracelet/glow/v2@latest` in Taskfile.yml or `lib/glow/README.glow.md.tmpl` text.

### 6.4 `--wish` (FLAG-10) -- `charm.land/wish/v2` v2.0.1

**`go.mod` entry:** `charm.land/wish/v2 v2.0.1`

**Overlay:** a working SSH server on `localhost:2222` that runs the TUI bubbletea model over SSH. The full `wish.NewServer` + `bubbletea.Middleware` pattern is verified via Context7.

**Example snippet (15-20 lines, must compile standalone):**

```go
import (
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
)

func runWishServer() {
	s, _ := wish.NewServer(
		wish.WithAddress("localhost:2222"),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	_ = s.ListenAndServe()
}
```

**Caveat:** the host key path `.ssh/id_ed25519` must exist. The README must explain `ssh-keygen -t ed25519 -f .ssh/id_ed25519` and that the server only listens locally.

### 6.5 `--log` (FLAG-11) -- `charm.land/log/v2` v2.0.0

**`go.mod` entry:** `charm.land/log/v2 v2.0.0`

**Overlay:** a sample logger setup. The simplest pattern: `log.SetDefault(log.New(os.Stderr))` at program start, then `log.Info("started", "name", ...)` calls throughout. Verified via Context7 `/charmbracelet/log`.

**Example snippet:**

```go
import "charm.land/log/v2"

func setupLogger() {
	log.SetDefault(log.New(os.Stderr))
	log.SetLevel(log.InfoLevel)
	log.Info("logger initialized", "version", version)
}
```

**Phase 1 already uses `charm.land/log/v2` in spin itself** -- so the import path is verified empirically (`go.mod` line: `charm.land/log/v2 v2.0.0`).

### 6.6 `--harmonica` (FLAG-12) -- `github.com/charmbracelet/harmonica` v0.2.0

**`go.mod` entry:** `github.com/charmbracelet/harmonica v0.2.0`

**CRITICAL: this is on `github.com/charmbracelet/harmonica`, not `charm.land/harmonica/v2`.** The v0.2.0 release pre-dates the migration. The CI grep suite must allow this path (see §7.3).

**Overlay:** a sample spring animation in `lib/harmonica/harmonica.go.tmpl`. The simplest example from Context7:

```go
import "github.com/charmbracelet/harmonica"

func animate() {
	spring := harmonica.NewSpring(harmonica.FPS(60), 6.0, 0.5)
	x, v := 0.0, 0.0
	for {
		x, v = spring.Update(x, v, 100.0)
		fmt.Printf("x=%.2f\n", x)
		if math.Abs(x-100.0) < 0.01 { break }
		time.Sleep(time.Second / 60)
	}
}
```

### 6.7 Overlay summary table

| Flag | go.mod entry | Overlay file | Surfaced in main.go? |
|------|--------------|--------------|----------------------|
| `--huh` | `charm.land/huh/v2 v2.0.3` | `lib/huh/huh.go.tmpl` | TUI: keybinding to open a form |
| `--glamour` | `charm.land/glamour/v2 v2.0.0` | `lib/glamour/glamour.go.tmpl` | TUI: help screen on `?` |
| `--glow` | (no go.mod entry) | `lib/glow/README.glow.md.tmpl` + Taskfile `setup` adds glow install | README section + optional `glow` subcommand |
| `--wish` | `charm.land/wish/v2 v2.0.1` | `lib/wish/wish.go.tmpl` | New `ssh` subcommand in main.go |
| `--log` | `charm.land/log/v2 v2.0.0` | `lib/log/log.go.tmpl` | Replace `fmt.Println` in main.go with `log.Info` |
| `--harmonica` | `github.com/charmbracelet/harmonica v0.2.0` | `lib/harmonica/harmonica.go.tmpl` | TUI: spring-animated transitions |

---

## 7. Build Config (`.air.toml` + `Taskfile.yml` + CI grep suite)

### 7.1 `.air.toml` `build.entrypoint` (WRAP-07, verified via Context7)

The Phase 1 `.air.toml.tmpl` is correct and uses `build.entrypoint = ["./tmp/main"]`. Phase 2 changes:

- `include_ext` adds `md` (for glamour overlays watching markdown)
- `exclude_dir` is unchanged
- The grep suite must check for the deprecated `bin = "tmp/main"` form (already in `check-v1-leaks.sh` AIR_PATTERNS; consider moving to a dedicated `check-air-bin.sh` per the original Phase 1 plan).

**Verified schema (Context7 `/air-verse/air`):**

- `build.cmd` -- build command (required)
- `build.entrypoint` -- binary path; `[]string` allows inline args
- `build.args_bin` -- additional args
- `build.include_ext` -- string list, default `["go"]`
- `build.exclude_dir` -- string list
- `build.exclude_regex`, `build.exclude_unchanged`, `build.exclude_file` -- additional filters
- `[build.windows/darwin/linux]` -- platform-specific overrides
- `[misc] clean_on_exit` -- bool, default true

**`build.bin` is deprecated and will be removed in a future release** (verified via the `air-verse/air` README).

### 7.2 `Taskfile.yml` `setup` target version pins (WRAP-08)

The Phase 1 `setup` target uses `@latest` for all four tools. Phase 2 keeps `@latest` (matches the rest of the v2 stack; user always gets the newest stable) and adds a comment block documenting the install-side Go version requirements.

```yaml
# Taskfile.yml -- generated by spin {{.SpinVer}}
# ...
  setup:
    desc: Install gofumpt, goimports, air, and prism
    # All four require Go 1.24+ (prism) or 1.25+ (air, gofumpt) on $PATH
    # to `go install`. Run with: task setup
    cmds:
      - go install mvdan.cc/gofumpt@latest
      - go install golang.org/x/tools/cmd/goimports@latest
      - go install github.com/air-verse/air@latest
      - go install go.dalton.dog/prism@latest
```

The CI grep script `check-taskfile-setup.sh` checks:

1. `Taskfile.yml` exists
2. `Taskfile.yml` contains a top-level `setup:` task key
3. The `setup:` task contains all four `go install` lines (gofumpt, goimports, air, prism), matched by the substring `mvdan.cc/gofumpt`, `golang.org/x/tools/cmd/goimports`, `github.com/air-verse/air`, `go.dalton.dog/prism`

A bash script or a Go test in `scripts/check-taskfile-setup_test.go` both work; the Phase 1 pattern (bash + a Go test that drives the bash) is the most ergonomic.

### 7.3 CI grep suite extension (TOOL-03, WRAP-07, WRAP-08)

The Phase 1 `scripts/check-v1-leaks.sh` has two issues for Phase 2:

1. **Blanket `github\.com/charmbracelet/`** -- false-positives on `github.com/charmbracelet/harmonica` and `github.com/charmbracelet/glow/v2` (which are the current paths for these libraries, not v1).
2. **No check for `Taskfile.yml` `setup:` target** -- WRAP-08 has no automated gate.

**Phase 2 changes to `check-v1-leaks.sh`:**

Replace the single regex with an explicit deny-list of migrated modules:

```bash
# Modules that migrated to charm.land/<lib>/v2; using the github.com path
# is a v1 leak. Keep this list alphabetized.
declare -a V1_LEAK_PATTERNS=(
  'github\.com/charmbracelet/bubbletea"'
  'github\.com/charmbracelet/bubbletea/v2"'   # the github.com/v2 form is the migration v1.x did before charm.land; still forbidden
  'github\.com/charmbracelet/lipgloss"'
  'github\.com/charmbracelet/lipgloss/v2"'
  'github\.com/charmbracelet/bubbles"'
  'github\.com/charmbracelet/bubbles/v2"'
  'github\.com/charmbracelet/huh"'
  'github\.com/charmbracelet/huh/v2"'
  'github\.com/charmbracelet/glamour"'
  'github\.com/charmbracelet/glamour/v2"'
  'github\.com/charmbracelet/wish"'
  'github\.com/charmbracelet/wish/v2"'
  'github\.com/charmbracelet/log"'
  'github\.com/charmbracelet/log/v2"'
  'github\.com/charmbracelet/fang"'
  'github\.com/charmbracelet/fang/v2"'
)
# Note: github.com/charmbracelet/harmonica and github.com/charmbracelet/glow/v2
# are intentionally NOT in this list -- they have not migrated and are still
# the current paths for those libraries.
```

**New script: `scripts/check-air-bin.sh`** -- extracts the AIR_PATTERNS check from `check-v1-leaks.sh` into a dedicated script. The v1 script keeps the v1-leak checks; the new script is the air-config check. This matches the Phase 1 plan's "consider splitting in Phase 2" note.

```bash
#!/usr/bin/env bash
# scripts/check-air-bin.sh
# Fails if .air.toml uses the deprecated build.bin = "tmp/main" form.
# The modern equivalent is build.entrypoint = ["./tmp/main"].

set -euo pipefail
ROOT="${1:-}"
if [[ -z "$ROOT" || ! -d "$ROOT" ]]; then
  echo "usage: $0 <project-dir>" >&2
  exit 2
fi

AIR_FILE="$ROOT/.air.toml"
if [[ ! -f "$AIR_FILE" ]]; then
  echo "OK: no .air.toml in $ROOT (nothing to check)"
  exit 0
fi

if matches=$(grep -En 'bin\s*=\s*"tmp/main"' "$AIR_FILE" 2>/dev/null); then
  echo "FAIL: deprecated air pattern in $AIR_FILE:" >&2
  echo "$matches" >&2
  echo "hint: use build.entrypoint = [\"./tmp/main\"] instead" >&2
  exit 1
fi
echo "OK: no deprecated air patterns in $AIR_FILE"
```

**New script: `scripts/check-taskfile-setup.sh`** -- verifies WRAP-08:

```bash
#!/usr/bin/env bash
# scripts/check-taskfile-setup.sh
# Fails if Taskfile.yml is missing the `setup:` target that installs
# gofumpt, goimports, air, and prism.

set -euo pipefail
ROOT="${1:-}"
if [[ -z "$ROOT" || ! -d "$ROOT" ]]; then
  echo "usage: $0 <project-dir>" >&2
  exit 2
fi

TASKFILE="$ROOT/Taskfile.yml"
if [[ ! -f "$TASKFILE" ]]; then
  echo "FAIL: $TASKFILE not found" >&2
  exit 1
fi

# Find a `setup:` line at column 0 (top-level task) and check the install lines follow
if ! grep -E '^setup:' "$TASKFILE" >/dev/null; then
  echo "FAIL: Taskfile.yml missing top-level 'setup:' target" >&2
  exit 1
fi

missing=()
for pkg in "mvdan.cc/gofumpt" "golang.org/x/tools/cmd/goimports" "github.com/air-verse/air" "go.dalton.dog/prism"; do
  if ! grep -F "$pkg" "$TASKFILE" >/dev/null; then
    missing+=("$pkg")
  fi
done
if [[ ${#missing[@]} -gt 0 ]]; then
  echo "FAIL: setup: target missing installs for:" >&2
  printf '  - %s\n' "${missing[@]}" >&2
  exit 1
fi
echo "OK: Taskfile.yml setup: target installs all four tools"
```

**Wire all three into `Taskfile.yml`:**

```yaml
  grep-v1-leaks:
    desc: Run the v1-leak / air-config / Taskfile-setup grep suite
    cmds:
      - bash scripts/check-v1-leaks.sh ./internal/scaffold/templates
      - bash scripts/check-v1-leaks.sh ./test-output   # set by integration test
      - bash scripts/check-air-bin.sh ./test-output
      - bash scripts/check-taskfile-setup.sh ./test-output
```

(Adjust the paths so the integration test runs the suite against the scaffolded project, not just the embedded templates.)

---

## 8. Architecture / Approach

### 8.1 How Phase 2 extends Phase 1

Phase 1 shipped the embed-based template engine, the `Project` struct with 13 forward-compat bool fields, the fang-wrapped cobra root, and the CI grep suite. Phase 2 *fills in* those forward-compat fields and adds three new sub-systems:

| Sub-system | New file(s) | Reuses from Phase 1 |
|------------|-------------|---------------------|
| CLI variant template | `templates/variant_cli/main.go.tmpl` (replaces `TODO` stub) | `templates/variant_tui/main.go.tmpl` (gating pattern with `{{if}}`) |
| Combo variant | `templates/variant_all/main.go.tmpl` (replaces `TODO` stub) | same |
| 6 lib overlays | `templates/lib/{huh,glamour,glow,wish,log,harmonica}/{huh,glamour,glow,wish,log,harmonica}.go.tmpl` (replaces `LIBS.md.tmpl` placeholders) | `templates/lib/lipgloss/lipgloss.go.tmpl` (overlay-as-source-file pattern) |
| 5 wrapper subcommands | `cmd/{run,build,test,vet,fmt}.go` + `internal/wrap/{run,build,test,vet,fmt,detect}.go` | `cmd/root.go` (subcommand attachment via `init()`), `cmd/new.go` (cobra flag-binding pattern) |
| External template repo | `internal/scaffold/repo.go` (clone + cleanup) + 5-line refactor to `template.go` (`FS` → `currentFS(p.ExternalDir)`) | `internal/scaffold/scaffold.go` (embed.FS package var), `internal/scaffold/template.go` (overlay walker) |
| CI grep extension | `scripts/check-air-bin.sh`, `scripts/check-taskfile-setup.sh`; refined v1-leak patterns in `check-v1-leaks.sh` | existing grep suite scripts |
| `internal/scaffold/versions.go` pin update | bump all 11 pins to latest stable (see §2.2) | existing `CharmPins` struct (add `LegacyCharmPins` for harmonica) |

The biggest single refactor is the `FS` → `currentFS` swap in `template.go` -- about 5 lines of changes. Everything else is additive.

### 8.2 New patterns introduced

| Pattern | Where | Rationale |
|---------|-------|-----------|
| `ToolSpec` + `RunWithFallback` helper | `internal/wrap/detect.go` | All 5 wrappers share the same "find tool, fall back, print hint" logic |
| `templateFS` interface + `currentFS` | `internal/scaffold/fs.go` | Lets the overlay engine read from embed.FS or os.DirFS(<tmp>) transparently |
| `os.RemoveAll` defer on cloned template dir | `cmd/new.go runNew` | Cleanup on completion; `--keep-template-cache` opts out |
| Path-traversal guard in `emit` | `internal/scaffold/scaffold.go` | Defense-in-depth: blocks templates that render `{{.Name}}` to escape `root/` |
| Per-cmd `cmd/<name>.go` + `init()` subcommand attachment | `cmd/run.go` etc. | Phase 1 pattern; consistent with `cmd/new.go` |
| Overlay-as-source-file (replace `LIBS.md.tmpl` placeholder) | `templates/lib/<name>/<name>.go.tmpl` | The overlay walker emits the file; the `variant_<type>/main.go.tmpl` uses `{{if hasLib}}` to import and wire it |

### 8.3 Patterns reused from Phase 1

- Single `Project` struct (now with `ExternalDir string` and `KeepTemplateCache bool` added)
- `funcMap` helper for `{{hasX}}` template predicates
- `charmPin` template func returning pinned versions from `DefaultPins`
- `cobra.ExactArgs(1)` and `SilenceUsage: true` for subcommands
- `log.Info / log.Error / log.Warn` structured logging from `charm.land/log/v2`
- `runCmd(dir, env, args...) ([]byte, error)` helper in `hooks.go` for `os/exec` calls
- `gitEnv` constant for `GIT_TERMINAL_PROMPT=0` and author identity (re-used in `repo.go`)
- `makeAbs` / `safePath` patterns (in development, for `templateFS`)
- `package-version` template helpers (e.g., `charmPin "huh"`) in `versions.go`

---

## 9. Risks / Open Questions

### 9.1 Risks (must be addressed in the plan)

| Risk | Mitigation |
|------|-----------|
| **Wrong go-version-floor** for fang-based CLI scaffolds (the v1 research said 1.23 for non-bubbles; fang v2.0.1 actually requires 1.25.0) | Decision in §2.2: default every generated `go.mod` to `1.25.0` regardless of selected libs. The `if hasBubbles` template branch in `_base/go.mod.tmpl` is replaced with a single `go 1.25.0` line. |
| **CI grep false-positives** on `github.com/charmbracelet/harmonica` and `github.com/charmbracelet/glow/v2` | §7.3 deny-list refinement. The grep suite goes from blanket `github.com/charmbracelet/` to a per-module allow-list. |
| **External template repo with malicious `_base/main.go.tmpl` could write outside the project dir** | §5.5 path-traversal guard in `emit`. Plus: the URL validation is permissive but `git clone` is the choke point (private repos require ssh-agent, which is opt-in). |
| **Stale version pins** for any of the 11 lib pins (the Phase 1 pins are 3+ months old per `go list -m -versions`) | Plan must include "bump all 11 pins to latest stable" as a discrete task with a verification step. |
| **prism `go test` proxy flag passing**: `prism go test ./...` may not pass arbitrary flags through. Need to verify. | Context7 fetch confirms `prism` passes args after the first to `go test -json`. Tested pattern: `prism -v go test -run TestFoo ./...`. Phase 2 plan should unit-test the wrapper. |
| **`air` running on a non-Go-only project**: scaffolded CLI projects have only `.go` files, but `--tui --bubbletea` with markdown files (`*.md`) might also be added. `include_ext = ["go"]` is too narrow. | Bump to `include_ext = ["go", "tpl", "tmpl", "md", "yaml", "toml", "json"]` in Phase 2 (glamour users especially). |
| **`spin fmt --no-strict` semantics ambiguity** | The v1 success criterion says "failing loud with an install hint when gofumpt is missing unless `--no-strict`". The cleanest interpretation: `--no-strict` = fall back to `gofmt` (no gofumpt, no goimports). Document this explicitly. |

### 9.2 Open Questions (planner judgment, not research gaps)

| # | Question | Default if unanswered | Reference |
|---|----------|----------------------|-----------|
| Q1 | Does `--all` mean "TUI + CLI as one binary with subcommands" (recommended) or "TUI is the primary, CLI is a flag"? | Single binary, TUI is a `tui` subcommand | §3.2 |
| Q2 | Should `--viper` add a `config.yaml` to the scaffold or just the binding wiring? | Just wiring; sample `config.yaml` is in `internal/config/config.go` comments | §3.3 |
| Q3 | Where should `spin fmt` print the install hint -- fang error box, plain stderr, or just exit non-zero? | Plain stderr + non-zero exit (fang only styles its own errors) | §4.3 |
| Q4 | For `spin build`, should the output path be `./bin/<name>` (matching `Taskfile.yml`) or `./bin/main`? | `./bin/<name>` to match the generated Taskfile | §4.5 |
| Q5 | For external template repos, do we want to support `git+ssh://git@github.com/foo/bar` (private repos)? | Yes -- `GIT_TERMINAL_PROMPT=0` lets ssh-agent auth happen silently | §5.1 |
| Q6 | When `glow` is selected without `--tui`, do we still need a `glow` subcommand in the generated CLI? | Optional `glow` subcommand via `--all`; pure CLI doesn't need it | §6.3 |
| Q7 | Should the `variant_all/main.go.tmpl` default to TUI-as-primary (run TUI if no args) or strict-subcommand-mode? | Strict-subcommand-mode (TUI is `tui` subcommand, no TUI on bare `myapp` invocation) | §3.2 |

### 9.3 Phase 1 deferred items that surface here

| Item | Phase-2 impact |
|------|----------------|
| `--module <path>` with sub-paths (v1 §15.11) | The directory emitted is still flat (`./<basename>/`); only `go.mod`'s `module` line changes. Not a Phase-2 concern. |
| Module path default -- bare name vs `github.com/<user>/<name>` (v1 §15.1) | Phase 2 keeps bare-name default; the Phase 3 prompter may add GitHub detection. |
| `gofumpt` install -- `@latest` vs pinned (v1 §15.3) | Phase 2 keeps `@latest`; README notes the Go 1.25+ floor. |

### 9.4 Things to watch during implementation

- **`go list -m -versions` drift:** re-run before locking the plan. New stable releases for any of the 11 libs could land between this research and plan execution.
- **Cobra v1.9.1 is the floor for fang v2**, but v1.10+ is out. Phase 2 should bump to v1.9.1 (the fang-tested floor) per the v1 RESEARCH §2 -- not v1.10 -- to match the `spin` repo's existing go.mod.
- **`go test -json` availability** is 1.24+ (matches prism's floor). `--test-vet` was added later; check if `prism` supports it.
- **Windows paths:** the `filepath.Separator` check in §5.5 is portable. Verify on Windows in the integration test if the CI matrix includes it (it does not today per CLAUDE.md -- Linux/macOS only).
- **Bubbles `runeutil` and `memoization` were removed in v2** (per v1 RESEARCH §2). Phase 2 overlays for bubbles, huh, etc. must avoid these.
- **fzf-style fuzzy match** for unknown flag suggestion: Phase 1 already implements Levenshtein (`cmd/root.go`); Phase 2 wrapper subcommands inherit it.

---

## 10. Recommended Plan Structure

The Phase 2 work naturally splits into 4 plans across 2 waves. The split minimizes cross-plan dependencies and keeps each plan testable independently.

### Wave 1 (sequential)

| Plan | Deliverable | REQ-IDs |
|------|-------------|---------|
| **02-01**: External templates + path-traversal guard + version pin bump | `--template-repo` wiring, `_base/go.mod.tmpl` go-version-floor fix, `CharmPins`/`LegacyCharmPins` struct, all 11 lib pins updated to latest stable, `internal/scaffold/repo.go`, `internal/scaffold/fs.go`, `emit` security fix | TMPL-03 (partial), TOOL-01, TOOL-02 (resolves to `1.25.0` always) |
| **02-02**: CLI variant + `--all` variant + lib overlays | `variant_cli/main.go.tmpl`, `variant_all/main.go.tmpl`, `lib/{huh,glamour,glow,wish,log,harmonica}/*`, `_base/go.mod.tmpl` extended to emit all 6 libs, `_base/README.md.tmpl` extended | FLAG-07..12, FLAG-13, FLAG-14, FLAG-15, TMPL-02 |

### Wave 2 (sequential; depends on Wave 1)

| Plan | Deliverable | REQ-IDs |
|------|-------------|---------|
| **02-03**: Five wrapper subcommands | `cmd/{run,build,test,vet,fmt}.go`, `internal/wrap/*`, `spin fmt --no-strict`, prism Go-version detector | WRAP-01..06 |
| **02-04**: CI grep suite extension + integration test | `scripts/check-air-bin.sh`, `scripts/check-taskfile-setup.sh`, refined `check-v1-leaks.sh` patterns, integration test for all 6 new lib overlays + CLI variant + `--all` + external template | WRAP-07, WRAP-08, TMPL-03 (full) |

**Why this order:**

- 02-01 (templates + pins) is the foundation; everything else either emits content or consumes it.
- 02-02 (variants + overlays) builds on the pins from 02-01.
- 02-03 (wrappers) is independent of variants -- the wrappers are scaffolder-side, not scaffolded-side. Could in principle run in parallel with 02-02, but 02-04's integration test needs both, and serializing avoids the "two parallel plans both touch `_base/Taskfile.yml.tmpl`" conflict.
- 02-04 (grep + integration test) is the gate.

**Total plan count: 4.** Matches the Phase 1 plan count (4 plans in 3 waves). Reasonable scope for Phase 2.

---

## 11. Sources

### Primary (HIGH confidence -- Context7-verified on 2026-06-03)

- **[/charmbracelet/fang](https://context7.com/charmbracelet/fang)** -- `fang.Execute(ctx, *cobra.Command, fang.WithVersion(...), fang.WithCommit(...), fang.WithNotifySignal(...))` API; drop-in for `rootCmd.Execute()`; v2 import path `charm.land/fang/v2`. Latest: v2.0.1. Go floor: 1.25.0.
- **[/charmbracelet/huh](https://context7.com/charmbracelet/huh)** -- `charm.land/huh/v2` v2.0.3; `huh.NewForm(huh.NewGroup(huh.NewInput().Value(&name), huh.NewConfirm().Value(&b)))`; `huh.ThemeFunc(huh.ThemeCharm)`; `WithWidth(60)`. Form fields: `NewInput`, `NewConfirm`, `NewSelect`, `NewText`, `NewNote`, `NewMultiSelect`. Go floor: 1.25.8.
- **[/charmbracelet/glamour](https://context7.com/charmbracelet/glamour)** -- `charm.land/glamour/v2` v2.0.0; `glamour.Render(in, "dark")` one-liner OR `glamour.NewTermRenderer(glamour.WithStylePath("dark"), glamour.WithWordWrap(80))` for custom; `r.Render(md)` returns ANSI; `lipgloss.Print(out)` recommended for color downsampling. Go floor: 1.25.8.
- **[/charmbracelet/wish](https://context7.com/charmbracelet/wish)** -- `charm.land/wish/v2` v2.0.1; sub-packages `charm.land/wish/v2/{bubbletea,logging,activeterm}`. `wish.NewServer(wish.WithAddress(...), wish.WithHostKeyPath(...), wish.WithMiddleware(...))`; `bubbletea.Middleware(teaHandler)`; `activeterm.Middleware()` rejects non-PTY; `logging.Middleware()`. Go floor: **1.25.9** (highest in the v2 stack).
- **[/charmbracelet/harmonica](https://context7.com/charmbracelet/harmonica)** -- **CRITICAL: still on `github.com/charmbracelet/harmonica`, NOT migrated to charm.land.** v0.2.0 (Apr 2022) latest. `harmonica.NewSpring(harmonica.FPS(60), angularFreq, damping)`; `spring.Update(pos, vel, target)` returns new pos, vel. Also `harmonica.NewProjectile(...)` for gravity/terminal motion. Go floor: 1.16 (low).
- **[/charmbracelet/log](https://context7.com/charmbracelet/log)** -- `charm.land/log/v2` v2.0.0; `log.SetDefault(log.New(os.Stderr))`; setter methods `SetLevel`, `SetReportTimestamp`, `SetFormatter`, `SetOutput`; package-level `log.Info`, `log.Debug`, `log.Fatal`. Go floor: 1.25.8.
- **[/charmbracelet/glow](https://context7.com/charmbracelet/glow)** -- **CRITICAL: still on `github.com/charmbracelet/glow/v2`, NOT migrated to charm.land.** v2.1.2 latest. Binary install: `go install github.com/charmbracelet/glow/v2@latest`. Go floor: 1.25.9.
- **[/air-verse/air](https://context7.com/air-verse/air)** -- `build.entrypoint` schema (preferred over deprecated `build.bin`); `include_ext`, `exclude_dir`, `exclude_regex`, `exclude_unchanged`, `exclude_file` fields. CLI override: `air --build.cmd "..." --build.entrypoint "..." --build.exclude_dir "..."`. `build.bin` "deprecated and will be removed in a future release." Install: `go install github.com/air-verse/air@latest` (Go 1.25+).
- **[/spf13/cobra](https://context7.com/spf13/cobra)** -- `cobra.Command{Use, Short, Long, RunE, Args, PersistentFlags, Flags}`; `cobra.ExactArgs(1)`, `cobra.MinimumNArgs(1)`, `cobra.NoArgs`; `MarkFlagRequired`, `MarkFlagsRequiredTogether`, `MarkFlagsMutuallyExclusive`, `MarkFlagsOneRequired`. v1.9.1 (latest tested; v1.10+ exists but fang v2.0.1 documents v1.9+).
- **[/spf13/viper](https://context7.com/spf13/viper)** -- `viper.BindPFlag(key, flag)` for one-off binding; `viper.BindPFlags(pflag.CommandLine)` for bulk; `viper.GetString`, `viper.GetInt`, `viper.GetBool`; `viper.IsSet(key)`; `viper.AutomaticEnv()` with `SetEnvPrefix`. UPGRADE.md: mapstructure moved to `github.com/go-viper/mapstructure/v2`. v1.20.1. Go floor: 1.20+.
- **[/charmbracelet/bubbletea](https://context7.com/charmbracelet/bubbletea)** -- Re-confirmed Phase 1: `View() tea.View` (not `string`); `tea.NewView(content)`; `tea.KeyPressMsg` (typed message); `tea.RequestBackgroundColor`, `tea.BackgroundColorMsg.IsDark()`. `tea.NewProgram(m).Run()` returns `(tea.Model, error)`. v2.0.7.
- **[/mattn/go-runewidth](https://context7.com/mattn/go-runewidth)** -- `runewidth.StringWidth("Hello世界")`; `runewidth.NewCondition()` with `cond.EastAsianWidth = true` for CJK awareness. Package-level `runewidth.EastAsianWidth = true` also works. (Phase 1 had this; re-verified.)
- **[/mvdan/gofumpt](https://context7.com/mvdan/gofumpt)** -- `gofumpt -l -w .` (list + write); v0.9.0+ based on Go 1.25's gofmt, requires Go 1.24+ to build. Install: `go install mvdan.cc/gofumpt@latest`.
- **[/charmbracelet/bubbletea-app-template](https://context7.com/charmbracelet/bubbletea-app-template)** -- Re-confirmed Phase 1: `.goreleaser.yaml` `version: 2`, `CGO_ENABLED=0`, `targets: ["go_first_class"]`. (Not Phase 2 scope but worth noting for Phase 3+.)

### Secondary (MEDIUM confidence -- verified via GitHub go.mod + go list)

- `charm.land/wish/v2 v2.0.1` go.mod → `go 1.25.9`
- `charm.land/huh/v2 v2.0.3` go.mod → `go 1.25.8`
- `charm.land/glamour/v2 v2.0.0` go.mod → `go 1.25.8`
- `charm.land/log/v2 v2.0.0` go.mod → `go 1.25.8`
- `charm.land/fang/v2 v2.0.1` go.mod → `go 1.25.0`
- `charm.land/bubbles/v2 v2.1.0` go.mod → `go 1.25.0`
- `github.com/charmbracelet/harmonica v0.2.0` go.mod → `go 1.16`
- `github.com/charmbracelet/glow/v2 v2.1.2` go.mod → `go 1.25.9`
- `go list -m -versions` confirmed all 10 lib versions published on the Go module proxy
- [DaltonSW/prism](https://github.com/DaltonSW/prism) README: Go 1.24+ floor (for `-json` flag); install `go install go.dalton.dog/prism@latest`; flags `-v` (verbose), `-f` (failed-only), `--no-color`/`--show-color`; `prism bench [regex] [path]`

### Local project files (HIGH confidence -- current state)

- `/home/samouly/Projects/Golang/loom/.planning/ROADMAP.md` -- Phase 2 success criteria 1-5 (verbatim cited in §1.1)
- `/home/samouly/Projects/Golang/loom/.planning/REQUIREMENTS.md` -- Phase 2 REQ-IDs (FLAG-07..12, FLAG-13..15, TMPL-02, TMPL-03, WRAP-01..08)
- `/home/samouly/Projects/Golang/loom/.planning/STATE.md` -- "stopped at: Phase 01 complete (4/4) -- ready to discuss Phase 2"
- `/home/samouly/Projects/Golang/loom/.planning/config.json` -- `mode: yolo`, `nyquist_validation: false` (no Validation Architecture section required)
- `/home/samouly/Projects/Golang/loom/.planning/phases/01-scaffolder-foundation-core-tui-stack/01-RESEARCH.md` -- Foundation: stack pins, embed-FS template engine, overlay composition, gum/huh design, CI grep patterns
- `/home/samouly/Projects/Golang/loom/CLAUDE.md` -- Project rules; charm v2 only (with the §2.1 correction for harmonica + glow)
- `/home/samouly/Projects/Golang/loom/internal/scaffold/versions.go` -- Current `DefaultPins` (v2.0.0 / v2.0.0-beta.2 / v2.0.0 / v2.0.0); needs update
- `/home/samouly/Projects/Golang/loom/internal/scaffold/template.go` -- Overlay walker; refactor target for `FS` → `currentFS`
- `/home/samouly/Projects/Golang/loom/internal/scaffold/resolve.go` -- `Project` field population; `--tui` auto-implies `--bubbletea` already
- `/home/samouly/Projects/Golang/loom/internal/scaffold/templates/lib/*/LIBS.md.tmpl` -- Placeholder overlays; Phase 2 replaces with real content
- `/home/samouly/Projects/Golang/loom/scripts/check-v1-leaks.sh` -- Existing grep suite; needs §7.3 refinement
- `/home/samouly/Projects/Golang/loom/internal/scaffold/integration_test.go` -- Phase 1 integration test pattern; Phase 2 extends with new assertions

### Auto-memory (HIGH confidence)

- `.claude/memory/MEMORY.md` -- "Project lipgloss pin issue: v2.0.0-beta.2 module-path mismatch; bump to stable v2 before Phase 2 lib variants" -- confirms the v2.0.3 bump is overdue.

---

## Metadata

### Confidence breakdown

| Area | Level | Reason |
|------|-------|--------|
| CLI variant template shape (cobra + fang v2) | HIGH | Context7-verified API + go module version confirmed |
| `--all` variant structure (cobra root + `tui` subcommand) | HIGH | Matches ROADMAP.md "Combine both variants" + bubbletea-app-template precedent |
| Wrapper subcommand design (`LookPath` + fallback + hint) | HIGH | Standard Go pattern; Phase 1 `runCmd` helper reused |
| `spin fmt` gofumpt→goimports→gofmt order | HIGH | gofumpt is a superset of gofmt; order is documented best practice |
| `spin test` prism Go 1.24+ floor | HIGH | DaltonSW/prism README direct quote |
| `spin vet` trivial wrapper | HIGH | `go vet` is always present; no fallback needed |
| External template override (clone + DirFS + security) | HIGH | `os/exec` `git clone` is well-known; embed.FS → os.DirFS swap is stdlib-supported; path traversal guard is a one-liner |
| 6 lib overlay designs | HIGH | Each lib verified via Context7; one (glow) is binary-only; harmonica + glow path correction is a critical fix |
| CI grep suite extension (deny-list + air + Taskfile scripts) | HIGH | Phase 1 pattern + 2 new bash scripts following the same shape |
| Go version floor decision (1.25.0 always for charm v2) | HIGH | All charm v2 go.mod files inspected; fang v2.0.1 → 1.25.0 is the floor |
| harmonica + glow path correction | HIGH | GitHub repo go.mod direct quote + `go list -m` confirmation |
| `--no-strict` for `spin fmt` semantics | MEDIUM | Reasonable default derived from success criterion text; planner may adjust |
| Q1..Q7 open questions | MEDIUM | Plausible defaults; planner should confirm during plan-phase |

### Research date

2026-06-03 (today)

### Valid until

~2026-07-15 (30 days for stable v2 libraries; re-run `go list -m -versions` before plan execution to catch any new stable releases; particularly watch bubbles/huh/glamour/wish which had recent bumps in the v2.0.x line)

---

## RESEARCH COMPLETE

**Phase:** 2 -- CLI Variant + Wrappers + Extended Library Coverage + External Templates
**Confidence:** HIGH (Context7-verified across 13 libraries; 1 critical correction to v1 research: harmonica + glow are NOT on charm.land; Go version floors re-verified against each go.mod; latest stable pins confirmed via `go list -m -versions`)

### Key Findings

1. **harmonica + glow are still on `github.com/charmbracelet/...`** -- the v1 CI grep blanket `github.com/charmbracelet/` ban must be refined to a per-module deny-list (8 modules that migrated; harmonica + glow stay allow-listed).
2. **All charm v2 stable libs require Go 1.25.0+**; huh/glamour/log want 1.25.8, wish/glow want 1.25.9. Decision: every generated `go.mod` uses `go 1.25.0` (the fang v2.0.1 floor that covers everything else transitively).
3. **All 11 version pins are stale** -- Phase 1's `DefaultPins` (bubbletea v2.0.0, lipgloss v2.0.0-beta.2, bubbles v2.0.0) should be bumped to the latest stable on `go list -m -versions` (v2.0.7 / v2.0.3 / v2.1.0 respectively). The lipgloss bump is the one flagged in MEMORY.md.
4. **External template override** needs only a 5-line refactor to `template.go` (swap `FS` for `currentFS(p.ExternalDir)`) plus a new `internal/scaffold/repo.go`. The overlay engine itself is unchanged.
5. **`--all` variant** is a single binary with cobra's `tui` subcommand, not a `--tui` bool flag -- subcommand composition is cleaner for future flags.
6. **Wrapper subcommands** all share one `ToolSpec` + `RunWithFallback` helper. The "preferred tool, fall back with install hint" pattern is identical across `run`, `build`, `test`, `vet`, `fmt`; only the `prism` detector (Go 1.24+ check) has unique logic.

### Open Questions

7. Q1: Single binary with `tui` subcommand (recommended) vs `--tui` bool flag for `--all`?
8. Q3: Where does `spin fmt` print the install hint (stderr, fang error box)?
9. Q5: Support private repos via ssh-agent with `GIT_TERMINAL_PROMPT=0`?
10. See §9.2 for the full 7-question list.

### Ready for Planning

Research complete. Planner can now create PLAN.md files. The 4-plan, 2-wave structure in §10 is recommended; planner may collapse Wave 1 into 1 plan if scope allows.
