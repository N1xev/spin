# spin documentation

Reference docs for `spin`, the language-agnostic scaffolder for external templates. Templates can target any language or framework: Go, Rust, TypeScript, Python, anything with a `spin.toml` and a `_base/` tree. The tree is organised by topic, not by source file. Start with one of the overviews below, or jump straight to a command, a package, or a concept.

## Overviews

- [What is spin?](overview/what-is-spin.md) - the one-paragraph pitch and the core value.
- [Architecture](overview/architecture.md) - the two-pipeline model: a template pipeline and a registry pipeline.
- [Quickstart](overview/quickstart.md) - the 5 commands you actually need to know on day one.

## Commands

One file per user-facing command. Each covers all flags, exit codes, and edge cases.

- [`spin new <name>`](commands/new.md) - scaffold a project from a template.
- [`spin add <spec>`](commands/add.md) - pin a template for offline use.
- [`spin list`](commands/list.md) - show pinned templates.
- [`spin update [name]`](commands/update.md) - refresh a pinned template's cache (with rollback).
- [`spin remove <name>`](commands/remove.md) - drop a pin (and optionally purge the cache).
- [`spin search <query>`](commands/search.md) - query the public registry.
- [`spin init <name>`](commands/init.md) - scaffold a new template directory.
- [`spin version`](commands/version.md) - print the spin version.

## Internal packages

The four `internal/` packages, by public API surface and pipeline role.

- [`internal/params`](packages/params.md) - the 8 param types, the huh form, the default-application path.
- [`internal/template`](packages/template.md) - the Template struct, the loader, the renderer, the post-hook.
- [`internal/registry`](packages/registry.md) - the registry HTTP client, the pin store, atomic writes.
- [`internal/version`](packages/version.md) - the build-time `Version` var.

## Concepts

The deeper reference material: schemas, semantics, edge cases.

- [Template schema (`spin.toml`)](concepts/template-schema.md) - full grammar, the 8 param types, `[[post]]`, `exclude`, `min_spin_version`.
- [Template engine](concepts/template-engine.md) - the funcs, the `.tmpl` extension stripping, the path-traversal guard, TPL-16.
- [Pinning model](concepts/pinning.md) - `pinned.json` schema, cache layout, `user/repo` shorthand, the rollback contract.
- [XDG layout and env vars](concepts/xdg-layout.md) - where things go on disk, the registry URL fallback chain.
- [Non-interactive use](concepts/non-interactive.md) - `--param`, `--print-params`, `--dry-run`, the unknown-key error.
- [Registry protocol](concepts/registry-protocol.md) - the search HTTP API, `ErrNotDeployed`, the `.invalid` default.
- [Registry design](concepts/registry-design.md) - the server side: hosting (Fly.io + Postgres), the index repo, the PR-based submission workflow, search backend.

### Companion repos

The registry is split across three repos. The CLI (this one) is the read-side client. The other two live separately and have their own READMEs.

- [`github.com/N1xev/spin-registry`](https://github.com/N1xev/spin-registry) - the Go server. Endpoints, local dev with `docker run postgres:16`, Fly.io deploy.
- [`github.com/N1xev/spin-index`](https://github.com/N1xev/spin-index) - the human-facing catalog (`templates/index.toml`). Author guide, PR workflow, validation CI.

## Development

For people hacking on spin itself.

- [Building](development/building.md) - Taskfile targets, `go build`, the `-ldflags` version-injection trick.
- [Testing](development/testing.md) - `go test ./... -count=1`, the XDG isolation pattern in tests.
- [Scripts](development/scripts.md) - `check-v1-leaks.sh`, `dogfood.sh`.
- [CI](development/ci.md) - the `ci` and `dogfood` GitHub Actions workflows.

## Reference

- [Exit codes](reference/exit-codes.md) - 0 / 1 / 130, and the "friendly not deployed" exception.
- [Environment variables](reference/env-vars.md) - `SPIN_REGISTRY_URL`, `SPIN_REGISTRY`, `XDG_CONFIG_HOME`, `GIT_TERMINAL_PROMPT`.
- [Constraints](reference/constraints.md) - `CGO_ENABLED=0`, Go 1.23+ for spin, Go 1.25+ for bubbles-v2-using projects, no v1 charm paths.

## Other documentation in the repo

These predate `docs/` and cover different ground. Read them for context, not for the reference shape.

- [`README.md`](../README.md) - the user-facing quick-start (4 commands, the install line, a non-interactive example).
- [`PRD.md`](../PRD.md) - the full product requirements doc: rationale, evolution, file-by-file inventory, v2.x roadmap.
