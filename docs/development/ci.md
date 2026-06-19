# CI

Two GitHub Actions workflows: `ci.yml` (build + test + vet + leak grep) and `dogfood.yml` (end-to-end smoke). Together they cover what `task test` + `task dogfood` cover on a developer's machine.

Source: `.github/workflows/ci.yml`, `.github/workflows/dogfood.yml`.

## `ci.yml` - the fast path

Runs on every push to `main` and every PR. Mirrors `task test`.

### Trigger

```yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
```

No path filter - any change to `main` or a PR re-runs the full suite. Cheap (single job, Go cache enabled) so this is fine.

### Permissions

```yaml
permissions:
  contents: read
```

Minimal: read the repo. No write, no PR comments, no deployments.

### Steps

1. `actions/checkout@v4` - clone the repo.
2. `actions/setup-go@v5` with `go-version: '1.25'` and `cache: true`. The cache uses the `go.sum` and Go version to key, so it's effective across runs.
3. **`go build ./...`** - compile every package. Fails on any compile error.
4. **`go test ./... -count=1`** - run the test suite, no cache.
5. **`go vet ./...`** - static analysis. Catches subtle bugs (printf format strings, unreachable code, lock copies).
6. **`bash scripts/check-v1-leaks.sh ./internal`** - the v1 deny-list grep against spin's own source.

The v1-leak step runs against `./internal` only (not `./cmd`) because the patterns are about library usage, and `cmd/` doesn't import the charm stack.

## `dogfood.yml` - the slow path

End-to-end smoke test. Runs on a narrower path filter so doc/comment-only changes don't trigger the 5-minute pipeline.

### Trigger

```yaml
on:
  push:
    branches: [main]
    paths:
      - 'cmd/**'
      - 'internal/**'
      - 'scripts/dogfood.sh'
      - '.github/workflows/dogfood.yml'
  pull_request:
    paths: [same as push]
```

`paths:` filters out changes to `docs/`, `README.md`, and other metadata paths. The intent: spin's docs and metadata can change without proving the scaffolder still works end-to-end. Scaffolder source or pipeline script changes **must** re-run dogfood.

### Steps

1. `actions/checkout@v4` + `actions/setup-go@v5` (`go-version: '1.25'`, `cache: true`).
2. **`bash scripts/dogfood.sh`** - the full pipeline (build, init, render, list, test).
3. **Failure artifact upload**: on a non-zero exit, upload the binary + any captured logs to the `dogfood-failure` artifact so the maintainer can repro without re-running.

The artifact's `if-no-files-found: ignore` means a passing job does not upload an empty archive.

### Why a separate workflow

`ci.yml` is ~30s. `dogfood.yml` is ~5min (it builds spin, renders, then runs the test suite in the rendered project). Splitting them lets fast PRs iterate without paying the dogfood cost on every push; the `paths:` filter on dogfood keeps the cost down to "you changed the scaffolder" pushes only.

A previous version of the pipeline scaffolded a real Go project and ran `go mod tidy` + `go build` + `go test` inside it. That was removed when the v2.0-template pass deleted the embedded scaffold tree. The init + render step in dogfood exercises the same internal packages end-to-end.

## Local parity

| CI step | Local command |
| --- | --- |
| `go build ./...` | `go build ./...` (no binary; or `go build -o ./bin/spin .`) |
| `go test ./... -count=1` | `task test` |
| `go vet ./...` | `go vet ./...` |
| `bash scripts/check-v1-leaks.sh ./internal` | `task grep-v1-leaks` (also runs against `./cmd`) |
| `bash scripts/dogfood.sh` | `task dogfood` |

If CI is red and your local tree is green, the most common cause is a cache key change. Re-running with `actions/setup-go@v5`'s `cache: true` should pick up the right Go module cache.

## Go version pinning

Both workflows pin `go-version: '1.25'`. The choice: spin itself doesn't import `charm.land/bubbles/v2` (which requires 1.25.0+), but the scaffolder generates projects that may, and pinning 1.25 here matches the floor for the generated ecosystem.

If the project's go.mod floor ever drops to 1.23 (per the open question in `CLAUDE.md`'s Go version tension section), update both workflow files. There are exactly two places to change.

## Edge cases

- **`go-version: '1.25'` as a string**: the setup-go action accepts `1.25`, `1.25.0`, or `1.25.x`. String form is the simplest and most common.
- **`cache: true` with a fresh dependency**: the first run after adding a new module downloads it; subsequent runs hit the cache. No manual cache-bust needed.
- **Failure artifact upload on Linux only**: the `dogfood-failure` artifact path includes `/tmp/dogfood-*.log`, which is the path the old pipeline used. The current `dogfood.sh` writes to `$REPO_ROOT/.tmp/dogfood-$$/`, so on failure the binary is uploaded but the log files are not. This is a known gap; fix the path glob if you need the logs in the artifact.
- **PR from a fork**: the `permissions: contents: read` is enough to run. Forked PRs do not get write tokens, which means secret-based integrations (none, currently) would not work; not an issue today.
- **`push: branches: [main]` without `tags`**: tag pushes do not trigger CI. Tag-driven release flows are a future-work item; right now releases are cut by hand from `main`.

## Related

- [Building](building.md) - the `go build` and ldflags.
- [Testing](testing.md) - the in-tree test suite.
- [Scripts](scripts.md) - what `dogfood.sh` and `check-v1-leaks.sh` actually do.
