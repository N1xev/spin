package template

import (
	"fmt"
	"os"

	"github.com/N1xev/spin/internal/params"
)

// SpinToml is the parsed manifest at the root of an external template.
//
// Example:
//
//	name            = "rust-cli"
//	version         = "0.1.0"
//	description     = "Minimal Rust CLI"
//	type            = "cli"
//	language        = "rust"
//	license         = "MIT"
//	repository      = "https://github.com/me/rust-cli-template"
//	min_spin_version = "0.2.0"
//
//	[author]
//	name  = "Sam"
//	email = "sam@example.com"
//	url   = "https://sam.example.com"
//
//	[params]
//	project_name = { type = "text", prompt = "Project name" }
//	edition      = { type = "select", options = ["2021", "2024"], default = "2021" }
//
//	[[post]]
//	run = "cargo init --name {{.project_name}}"
//
//	[[post]]
//	run = "git init && git add -A"
type SpinToml struct {
	Name           string                 `toml:"name"`
	Version        string                 `toml:"version"`
	Description    string                 `toml:"description"`
	Type           string                 `toml:"type"`     // "tui" | "cli" | "lib" | ...
	Language       string                 `toml:"language"` // "go" | "rust" | "ts" | ...
	Author         Author                 `toml:"author"`
	License        string                 `toml:"license"`
	Repository     string                 `toml:"repository"`
	MinSpinVersion string                 `toml:"min_spin_version"`
	Exclude        []string               `toml:"exclude"`
	Params         map[string]params.Spec `toml:"params"`
	Post           []PostStep             `toml:"post"`
	Tags           []string               `toml:"tags"`
}

// Author identifies the template creator. All fields are optional;
// templates only need to fill what they want to publish.
type Author struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
	URL   string `toml:"url"`
}

// PostStep is one command in the post-scaffold hook. The shell command
// is templated against the resolved param + flag values, then run via
// `sh -c` in the project root. Steps execute in order; the hook
// stops on the first failure.
//
// This is intentionally a list, not a single string -- it matches the
// shape npm scripts, Taskfile.yml, and Just converged on, and gives
// a clean path to per-step metadata (env, cwd, on_error) without a
// breaking schema change.
type PostStep struct {
	Run string `toml:"run"`
}

// ParseSpinToml reads and parses a spin.toml file from disk.
func ParseSpinToml(path string) (*SpinToml, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSpinTomlBytes(b)
}

// ParseSpinTomlBytes parses a spin.toml from raw bytes. Uses the
// BurntSushi/toml-style lib via the standard library's encoding/toml?
// No -- encoding/toml/v2 was promoted to stdlib in Go 1.23. We use it
// here.
//
//	[Note: if encoding/toml is not available in the build's Go version,
//	 fall back to pelletier/go-toml. The scaffolded go.mod pins Go 1.25.]
func ParseSpinTomlBytes(b []byte) (*SpinToml, error) {
	// encoding/toml/v2 was promoted to stdlib; using it would require
	// an import. To keep the v2 skeleton self-contained without an
	// extra dep, we use a minimal hand-rolled parser sufficient for
	// the schema. Full TOML features (multiline, inline tables beyond
	// `{ type = "..." }`, datetime) are NOT supported in v2.0; the
	// registry will validate manifests before publishing.
	st := &SpinToml{Params: map[string]params.Spec{}}
	if err := parseTOML(b, st); err != nil {
		return nil, err
	}
	if st.Name == "" {
		return nil, fmt.Errorf("spin.toml: name is required")
	}
	return st, nil
}
