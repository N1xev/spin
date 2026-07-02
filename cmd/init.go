package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Scaffold a new external template directory",
	Long:  "Scaffold a new external template directory named <name> in the current directory. Creates a spin.toml manifest and a _base/ tree with one example file you can edit.",
	Example: `  # Scaffold a new template called "my-cli-template"
  spin init my-cli-template

  # Scaffold into a different directory
  spin init my-template --dir ./templates`,
	Args:          cobra.ExactArgs(1),
	RunE:          runInit,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var initDirFlag string

func init() {
	initCmd.Flags().StringVar(&initDirFlag, "dir", "", "parent directory to create the template in (default: current dir)")
	rootCmd.AddCommand(initCmd)
}

// initFileTemplate is the body of the placeholder _base/file.txt
// the user can edit. Kept short and obvious so it doesn't look
// like a real example of the templating language.
const initFileTemplate = `# {{.name}}

This file is rendered by Go text/template against the resolved
param values. {{.name}} is the project name passed to
` + "`spin new`" + `; you can add params to spin.toml and they
become available here.

Edit this file and spin.toml to taste. The full template docs
live at <https://github.com/N1xev/spin>.
`

// initReadme is the README body for the new template. It links
// to the spin docs and lists the editable parts.
const initReadme = `# <name> template

A [spin](https://github.com/N1xev/spin) template. Scaffold a
project from it with:

` + "```" + `
spin new my-project --template .
` + "```" + `

## Files

- ` + "`spin.toml`" + `: template manifest. Edit ` + "`name`" + `,
  ` + "`description`" + `, ` + "`params`" + `, and ` + "`[[post]]`" + `
  to taste.
- ` + "`_base/`" + `: the file tree rendered into the user's
  project. Files ending in ` + "`.tmpl`" + ` are processed by
  Go text/template; everything else is copied verbatim.

## Tips

- ` + "`spin new my-app --template . --print-params`" + `
  previews the resolved params as JSON without writing files.
- ` + "`spin new my-app --template . --dry-run`" + ` lists
  the files that WOULD be written.
- Add ` + "`[[post]]`" + ` steps in spin.toml to run shell
  commands after the files are written (e.g. ` + "`go mod init`" + `).
`

// runInit is the RunE for `spin init`. Creates <dir>/<name>/
// with spin.toml, _base/file.txt, and a README.md. Errors out
// if the destination already exists (the user can pick another
// name or remove the dir first).
func runInit(cmd *cobra.Command, args []string) error {
	name := args[0]
	if name == "" {
		return errors.New("spin init: name is required")
	}
	if err := validateTemplateName(name); err != nil {
		return fmt.Errorf("spin init: %w", err)
	}

	parent := initDirFlag
	if parent == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("spin init: getwd: %w", err)
		}
		parent = wd
	}
	dest := filepath.Join(parent, name)

	// Refuse to overwrite an existing directory. Users with intent
	// to overwrite can `rm -rf` and re-run; we don't want a typo
	// to clobber a real template.
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("spin init: %s already exists; pick a different name or remove it first", dest)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("spin init: stat %s: %w", dest, err)
	}

	if err := os.MkdirAll(filepath.Join(dest, "_base"), 0o755); err != nil {
		return fmt.Errorf("spin init: mkdir: %w", err)
	}

	files := map[string]string{
		"spin.toml":    initSpinToml(name),
		"_base/file.txt.tmpl": initFileTemplate,
		"README.md":    initReadme,
	}
	for rel, body := range files {
		full := filepath.Join(dest, rel)
		// Ensure parent dir exists for nested entries (we only
		// have _base/ today, but this keeps the loop safe if
		// the manifest is extended).
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("spin init: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			return fmt.Errorf("spin init: write %s: %w", rel, err)
		}
	}

	// And `spin add <dest>` so the user can use --template <name>
	// immediately. We do this LAST so a partial init doesn't
	// leave a broken pin.
	_ = tryAutoPin(name, dest, cmd.OutOrStdout())

	printSuccess("created template %q at %s", name, dest)
	printHint("edit spin.toml and _base/, then `spin new <project> --template %s`", name)
	return nil
}

// initSpinToml renders the starting manifest for a new template.
// It includes a couple of example params (project_name, license)
// and a no-op post step the user can replace, so the file is
// immediately renderable end-to-end.
func initSpinToml(name string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("name = %q\n", name))
	b.WriteString("description = \"A new spin template -- edit me\"\n")
	b.WriteString("version = \"0.1.0\"\n")
	b.WriteString("type = \"cli\"\n")
	b.WriteString("language = \"go\"\n")
	b.WriteString("min_spin_version = \"0.1.0\"\n\n")
	b.WriteString("[params]\n\n")
	b.WriteString("[params.license]\n")
	b.WriteString("type = \"select\"\n")
	b.WriteString("prompt = \"License\"\n")
	b.WriteString("options = [\"MIT\", \"Apache-2.0\", \"BSD-3-Clause\", \"Proprietary\"]\n")
	b.WriteString("default = \"MIT\"\n\n")
	b.WriteString("[[post]]\n")
	b.WriteString("run = \"echo 'post hook ran for {{.name}}'\"\n")
	return b.String()
}

// tryAutoPin runs `spin add <dest>` (silently) so the freshly
// created template is immediately usable as `--template <name>`.
// Errors are best-effort: the user can run `spin add` by hand.
//
// Currently a no-op placeholder; we deliberately do not auto-pin
// because templates in development are usually just a directory,
// and re-pinning on every `init` is annoying. The hint points
// the user at `spin add` for the offline case.
func tryAutoPin(_, dest string, out io.Writer) error {
	fmt.Fprintf(out, "  (run `spin add %s` to pin for offline use later)\n", dest)
	return nil
}

// validateTemplateName rejects names with characters that would
// be awkward in a directory name, in a pinned-name lookup, or on
// the wire (e.g. a path separator would let the user create
// templates outside the intended parent). Empty and dot-only
// names are also rejected.
func validateTemplateName(name string) error {
	if name == "" || name == "." || name == ".." {
		return errors.New("name must be non-empty and not \".\" or \"..\"")
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("name %q must not contain path separators or NUL", name)
	}
	return nil
}
