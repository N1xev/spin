# Testing spin

How the suite is organized, the XDG-isolation pattern that keeps tests off the user's real `~/.config/spin/`, and the gotchas around external tool dependencies.

Source: `internal/template/*_test.go`, `internal/registry/*_test.go`, `cmd/*_test.go`, `cmd/update_test.go`.

## Running the suite

```sh
go test ./... -count=1
```

`-count=1` disables Go's test cache. Without it, `task test` will pass on a no-op edit because the results are cached. The `-count=1` is mandatory for CI correctness; `Taskfile.yml:23-26` already includes it.

To run a single package:

```sh
go test ./internal/template/ -count=1
```

To run a single test by name:

```sh
go test ./internal/template/ -count=1 -run TestStripTmplExt
```

## The XDG-isolation pattern

Tests that touch `pinned.json`, the templates cache, or the registry env vars **must not** pollute `~/.config/spin/` on a developer's machine. The pattern (used throughout `cmd/*_test.go` and `internal/registry/*_test.go`):

```go
func TestSomething(t *testing.T) {
    t.Setenv("XDG_CONFIG_HOME", t.TempDir())
    t.Setenv("HOME", t.TempDir()) // Linux: UserConfigDir uses XDG first
    // ... test code ...
}
```

`t.Setenv` restores the original env on test cleanup. `t.TempDir()` returns a fresh dir that `t.Cleanup` removes. Together: every test starts with an empty cache and a clean env, regardless of who is running it or where.

On macOS, `os.UserConfigDir` ignores `XDG_CONFIG_HOME` and returns `~/Library/Application Support/`. Tests that need full isolation on macOS also set `HOME` to redirect the Library path. The `cmd/doctor_test.go`-style pattern sets both.

On Windows, `os.UserConfigDir` returns `%AppData%`. Tests can `t.Setenv("AppData", t.TempDir())` for the same effect. (The current suite doesn't have Windows-specific tests; if you add one, follow the same `t.Setenv` recipe.)

## The `t.TempDir()` work-dir trick

Tests that need to render templates (e.g., `internal/template/engine_test.go`) call `t.TempDir()` for both the `BaseDir` and the destination. This keeps each test in its own sandbox.

Some scaffolder paths refuse to walk up from world-writable dirs (e.g., `/tmp` is mode `1777`). The test must not use the system temp dir as the BaseDir if the test ever calls `go mod tidy` or `go build` inside it - Go's tooling walks up looking for a module root and stops at the first "system" dir.

`t.TempDir()` is **not** world-writable (its mode is `0700`), so this concern doesn't apply. But if you copy a test into a CI script and the work dir leaks to `/tmp`, you may see `go: warning: "all" matched no packages` from `go mod tidy` because Go refused to walk up from `/tmp`.

## Tests that touch the network

The `internal/registry/client_test.go` suite never mocks the network. It relies on the `.invalid` TLD default URL (`internal/registry/types.go:14`) to make the DNS lookup fail fast, then asserts that `Search` returns `ErrNotDeployed`.

This is the only safe way to test "the registry is not yet deployed" - mocking the HTTP client would let the test pass with a fake response and miss the real failure mode (DNS, TCP, TLS, HTTP timeout). See [Registry protocol](../concepts/registry-protocol.md) for the `isNetworkError` table.

If you need to test a real registry response, set `SPIN_REGISTRY_URL` to a local test server (httptest.NewServer), but be aware that the `isNetworkError` mapping only fires on network failures, not on HTTP error status codes, so a 4xx/5xx from a test server will not be mapped to `ErrNotDeployed`.

## Tests that touch git

`internal/template/loader_test.go` and `internal/registry/client_test.go` may call `git clone` against fixtures. The test sets `GIT_TERMINAL_PROMPT=0` indirectly (the loader sets it before calling git) so a missing credential fails fast instead of blocking the test on a tty prompt.

Tests must not depend on a remote git server. If a test needs a "git source" fixture, it should create a local file:// URL or use `git init` to make a tiny repo in `t.TempDir()` and pass that path. Network-dependent tests are flaky and slow.

## Test patterns

| Test | File pattern |
| --- | --- |
| `internal/template/*_test.go` | Tmpl-ext stripping, render, post-hook, funcs, walk, exclude globs. |
| `internal/registry/*_test.go` | Pinned JSON round-trip, atomic write, shorthand, SHA capture. |
| `cmd/*_test.go` | CLI invocation, flag parsing, exit codes. |
| `scripts/check-v1-leaks.sh` | A bash script, not a Go test; run via `task grep-v1-leaks`. |
| `scripts/dogfood.sh` | End-to-end smoke; run via `task dogfood` or directly. |

## Edge cases

- **`go test ./...` runs every package**: this includes packages with no tests. The output may include `ok <pkg> [no test files]`. That's normal.
- **Test cache poisoning**: if a test mutates a file the next test depends on, the cached result from the *first* run may be reused. Use `t.TempDir()` for all on-disk state.
- **Parallel test safety**: tests using the same `XDG_CONFIG_HOME` value (e.g., the real `~/.config/spin/`) will race. The `t.Setenv` pattern prevents this because each test's `t.TempDir()` is unique.
- **`go test -race ./... -count=1`**: useful for catching data races. Slow but thorough. The CI doesn't run it by default; add it locally when working on the loader or the registry client.
- **The `count=1` flag**: spelled with one `1`, not `-count 1`. Both are accepted by `go test`, but the Taskfile uses the former.

## Related

- [Building](building.md) - the `task build` half of dev workflow.
- [Scripts](scripts.md) - `check-v1-leaks.sh` and `dogfood.sh` that are not Go tests.
- [CI](ci.md) - the GitHub Actions that run `go test ./... -count=1` on every push.
