# What is spin?

A language-agnostic scaffolder for external templates. One CLI to turn any external template into a runnable project - Go, Rust, TypeScript, Python, anything.

## The pitch

> One CLI to scaffold projects from external templates. `spin new <name> --template <spec>` for greenfield, `spin add <spec>` to pin a template for offline use, `spin update [name]` to refresh a pinned template's cache.

## Core value

> Generate a runnable project from any external template with one command.
> `spin new myapp --template go-cli && cd myapp && go run .` produces a
> project that builds, tests, and runs without extra setup -- regardless
> of language, framework, or build tool. The template author owns the
> details; `spin` owns the load / prompt / render / post-hook pipeline.

## One concept, two pipelines

`spin` is built around one first-class concept and two orthogonal pipelines that consume it.

**The concept: a Template.** A directory with `spin.toml` (the manifest) and a `_base/` tree (the file overlays). The template's language, framework, and build tool are entirely the author's choice. `spin` doesn't know or care.

**Pipeline 1: the template pipeline.** Triggered by `spin new`. Resolves a template spec (local path, git URL, or pinned name), resolves the param values (interactive form, or `--param key=value` flags), renders the `_base/` tree through `text/template`, writes the result to the destination, and runs the template's `[[post]]` steps.

**Pipeline 2: the registry pipeline.** Triggered by `spin add` / `spin list` / `spin update` / `spin remove` / `spin search`. Manages the on-disk pin store (`~/.config/spin/pinned.json`) and the per-template cache (`~/.config/spin/templates/<name>/`), and queries the public registry for templates to discover.

The two pipelines share a single Template type and a single filesystem layout but otherwise are independent. You can use `spin new --template <git-url>` without ever running `spin add`, and you can use `spin add` to pre-warm the cache without ever running `spin new`.

## What it is not

- Not a language-specific scaffolder. Templates can target Go, Rust, TypeScript, Python, anything.
- Not a build system. The template's `[[post]]` steps can run `go mod tidy` or `cargo init`, but `spin` doesn't own the build.
- Not a package manager. The `spin` binary doesn't know what a "module" or "crate" is.
- Not a UI. `spin` is a CLI. Some prompts use `huh` (an in-process TUI form), some shell out to `gum` when available, but there's no first-class GUI.

## Related

- [Architecture](architecture.md) - the two-pipeline model in detail.
- [Quickstart](quickstart.md) - the 5 commands you need on day one.
- [Template schema](concepts/template-schema.md) - what a `spin.toml` actually looks like.
