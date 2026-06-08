package rust

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/spin/internal/ecosystem"
)

// Render produces the file map (path → file contents) for the rust
// project. It is self-contained: no calls into internal/scaffold.
func (e *Ecosystem) Render(ctx ecosystem.Context) (map[string][]byte, error) {
	files := map[string][]byte{}

	name := ctx.Name
	if name == "" {
		return nil, fmt.Errorf("rust: project name is required")
	}

	edition := ctx.GetString("edition")
	rustVer := ctx.GetString("rust-version")
	author := ctx.GetString("author")
	description := ctx.GetString("description")
	projectType := ctx.GetString("type")
	if projectType == "" {
		projectType = "bin"
	}

	// Source file determines the Cargo.toml [[bin]]/[lib]/[[example]] block.
	switch projectType {
	case "bin":
		files["src/main.rs"] = []byte(renderMainRS(name))
		files["Cargo.toml"] = []byte(renderCargoToml(cargoTomlParams{
			Name: name, Edition: edition, RustVersion: rustVer,
			Author: author, Description: description, Type: "bin",
		}))
	case "lib":
		files["src/lib.rs"] = []byte(renderLibRS(name))
		files["Cargo.toml"] = []byte(renderCargoToml(cargoTomlParams{
			Name: name, Edition: edition, RustVersion: rustVer,
			Author: author, Description: description, Type: "lib",
		}))
	case "example":
		files[filepath.Join("examples", name+".rs")] = []byte(renderExampleRS(name))
		files["Cargo.toml"] = []byte(renderCargoToml(cargoTomlParams{
			Name: name, Edition: edition, RustVersion: rustVer,
			Author: author, Description: description, Type: "example",
		}))
	}

	// .gitignore (unless disabled).
	if ctx.GetBool("gitignore") {
		files[".gitignore"] = []byte(renderGitignore(projectType))
	}

	// spin.config.toml with the [tasks] block.
	files["spin.config.toml"] = []byte(renderSpinConfig())

	return files, nil
}

type cargoTomlParams struct {
	Name        string
	LibName     string // only set for type=lib
	Edition     string
	RustVersion string
	Author      string
	Description string
	Type        string
}

func renderCargoToml(p cargoTomlParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[package]\n")
	fmt.Fprintf(&b, "name = %q\n", p.Name)
	fmt.Fprintf(&b, "version = \"0.1.0\"\n")
	fmt.Fprintf(&b, "edition = %q\n", p.Edition)
	if p.RustVersion != "" {
		fmt.Fprintf(&b, "rust-version = %q\n", p.RustVersion)
	}
	if p.Author != "" {
		fmt.Fprintf(&b, "authors = [%q]\n", p.Author)
	}
	if p.Description != "" {
		fmt.Fprintf(&b, "description = %q\n", p.Description)
	}
	fmt.Fprintf(&b, "\n[dependencies]\n")

	switch p.Type {
	case "bin":
		fmt.Fprintf(&b, "\n[[bin]]\nname = %q\npath = \"src/main.rs\"\n", p.Name)
	case "lib":
		libName := strings.ReplaceAll(p.Name, "-", "_")
		fmt.Fprintf(&b, "\n[lib]\nname = %q\npath = \"src/lib.rs\"\n", libName)
	case "example":
		fmt.Fprintf(&b, "\n[[example]]\nname = %q\npath = \"examples/%s.rs\"\n", p.Name, p.Name)
	}
	return b.String()
}

func renderMainRS(name string) string {
	return fmt.Sprintf(`fn main() {
    println!("Hello, %s!");
}
`, name)
}

func renderLibRS(name string) string {
	return fmt.Sprintf(`pub fn hello() -> &'static str {
    "Hello, %s!"
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn test_hello() {
        assert_eq!(hello(), "Hello, %s!");
    }
}
`, name, name)
}

func renderExampleRS(name string) string {
	return fmt.Sprintf(`fn main() {
    println!("Hello, %s!");
}
`, name)
}

func renderGitignore(projectType string) string {
	// Per cargo convention: libraries ignore Cargo.lock (it should match
	// the published version); binaries do not.
	if projectType == "lib" {
		return "/target\nCargo.lock\n"
	}
	return "/target\n"
}

func renderSpinConfig() string {
	return `[tasks]
build = "cargo build"
test = "cargo test"
run = "cargo run"
clippy = "cargo clippy"
fmt = "cargo fmt"
`
}

// writeFiles writes the rendered file map to disk with a path-traversal
// guard. Mirrors the safety pattern used in template.writeFiles: any
// file whose cleaned path escapes `dest` is rejected. Exposed (within
// the package) for unit testing.
func writeFiles(dest string, files map[string][]byte) error {
	cleanDest := filepath.Clean(dest) + string(filepath.Separator)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dest, err)
	}
	for rel, content := range files {
		full := filepath.Join(dest, rel)
		cleanFull := filepath.Clean(full)
		if !strings.HasPrefix(cleanFull+string(filepath.Separator), cleanDest) {
			return fmt.Errorf(
				"path traversal: rendered %q resolves to %q which is outside project root %q",
				rel, cleanFull, cleanDest,
			)
		}
		dir := filepath.Dir(full)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", dir, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", full, err)
		}
	}
	return nil
}
