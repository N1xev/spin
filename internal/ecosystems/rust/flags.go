package rust

import "github.com/example/spin/internal/ecosystem"

// Flags is the full set of flags the rust ecosystem declares.
// Each flag is rendered into the cobra command tree at registration
// time and into a huh form at interactive-prompt time.
func Flags() []ecosystem.Flag {
	return []ecosystem.Flag{
		// ── project type ───────────────────────────────────────
		ecosystem.ChoiceFlag("type", "bin", []string{"bin", "lib", "example"}, "project-type").
			WithHelp("Project type: bin (binary), lib (library), or example (single-file example).").
			WithPrompt("Project type"),

		// ── metadata ───────────────────────────────────────────
		ecosystem.StringFlag("edition", "2021", "metadata").
			WithPrompt("Rust edition").
			WithHelp("Rust edition (2015, 2018, 2021, 2024)."),
		ecosystem.StringFlag("rust-version", "1.75", "metadata").
			WithPrompt("Minimum Rust version"),
		ecosystem.StringFlag("author", "", "metadata").
			WithPrompt("Author name").
			WithHelp("Defaults to git config user.name; empty if unknown."),
		ecosystem.StringFlag("description", "", "metadata").
			WithPrompt("Project description"),

		// ── metadata: AI / docs ────────────────────────────────
		ecosystem.BoolFlag("ai", false, "metadata").
			WithHelp("Also generate AGENTS.md describing the project.").
			WithPrompt("Generate AGENTS.md?"),

		// ── CLI behavior ───────────────────────────────────────
		ecosystem.BoolFlag("gitignore", true, "behavior").
			WithHelp("Include a .gitignore that excludes target/.").
			WithPrompt("Include .gitignore?"),
		ecosystem.BoolFlag("force", false, "behavior").
			WithHelp("Overwrite an existing directory.").
			WithPrompt("Force overwrite?"),
		ecosystem.BoolFlag("no-git", false, "behavior").
			WithHelp("Skip `git init` and the initial commit.").
			WithPrompt("Skip git init?"),
		ecosystem.BoolFlag("quiet", false, "behavior").
			WithHelp("Suppress non-error output.").
			WithPrompt("Quiet mode?"),
		ecosystem.BoolFlag("no-interactive", false, "behavior").
			WithHelp("Disable all prompts; flags-only mode (CI/scripted).").
			WithPrompt("Non-interactive?"),
	}
}
