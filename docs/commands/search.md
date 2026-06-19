# `spin search <query>`

Query the public template registry. Returns a sorted list of matches, or a friendly "not yet deployed" message until the registry server ships.

Source: `cmd/search.go:12-58`.

## Synopsis

```sh
spin search <query>           # human-readable table
spin search <query> --json   # JSON wire format
spin search <query> --limit 5
```

## Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--limit` | int | `20` | Cap the number of returned entries. `<= 0` means no cap. |
| `--json` | bool | `false` | Emit a JSON object on stdout. Currently a skeleton (see [Edge cases](#edge-cases)). |

`<query>` is a positional argument. `cobra.MinimumNArgs(1)`. Empty queries error out at cobra's arg-validation step.

## What it does

1. Calls `registry.Client.SearchWithLimit(args[0], searchLimit)`.
2. Maps the response:
   - `ErrNotDeployed` → friendly message: "the public spin registry isn't deployed yet. Until it is, you can scaffold from any git URL: `spin new <name> --template <git-url>`" (cmd/search.go:40-46). Exits 0.
   - Other errors → returned as-is, fang renders them. Exits 1.
3. For a successful response, formats the entries via `registry.FormatSearch` (registry/search.go:10) - a human-readable table sorted by download count (descending). The `--json` path is currently a skeleton that emits `{"query": ..., "total": ..., "entries": []}` without populating `entries` (cmd/search.go:51-55).

## Examples

```sh
spin search go
# NAME                  DESCRIPTION                       LANGUAGE  TAGS
#   go-cli              Minimal Go CLI with cobra         go        cli,starter
#   go-tui              TUI starter with bubbletea        go        tui
#   gin-rest            REST API with gin                 go        web,api

spin search rust --limit 5
spin search cli --json
```

## Exit codes

- `0` - success, **and** also for the "not yet deployed" friendly case. The "the registry isn't deployed" path is a normal state, not a failure.
- `1` - any other error (HTTP 5xx, malformed response, etc.)

## Edge cases

- **Registry not deployed (default)**: `SPIN_REGISTRY_URL` is unset, so the client falls back to `https://registry.spin.invalid/v1` (registry/types.go:14). The `.invalid` TLD is RFC 2606 reserved and never resolves. DNS lookup fails, `isNetworkError` returns true, `Search` returns `ErrNotDeployed`, the user sees the friendly message. No 15-second DNS wait.
- **Registry deployed but 404s**: same `ErrNotDeployed` path. Server operators who 404 `/search` instead of returning a real response get the same friendly UX.
- **Registry deployed and returns real results**: the table is rendered. Sorted by `Downloads` desc via `SortByPopularity` (registry/search.go:28-32).
- **`--json` is a skeleton**: as of v2.0-template, the JSON output emits the query, the total, and an empty `entries` array. The table is the source of truth. The wire format is "what the registry server will speak" - the client side hasn't been updated to match yet.
- **Network timeouts (15s)**: `Client.HTTP` has a `15 * time.Second` timeout (client.go:45). Slow / hanging servers produce `ErrNotDeployed` via the `context deadline exceeded` string-inspection branch (client.go:99-130).
- **`XDG_CONFIG_HOME` override**: does not affect `Search` (the cache dir is irrelevant to the search call). The override is for `pinned.json` and `templates/`.

## Internal calls

- `registry.New()` (registry/client.go:31) - reads `SPIN_REGISTRY_URL` (preferred) / `SPIN_REGISTRY` (fallback) / `DefaultIndexURL`.
- `client.SearchWithLimit(query, limit)` (client.go:60) - the HTTP GET + limit clamp.
- `registry.FormatSearch(result, plain)` (registry/search.go:10) - the table formatter.
- `SortByPopularity(entries)` (registry/search.go:28) - by `Downloads` desc.

## Related

- [Registry protocol](../concepts/registry-protocol.md) - the HTTP API in detail, `ErrNotDeployed`, the `.invalid` TLD trick.
- [XDG layout and env vars](../concepts/xdg-layout.md) - the `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` fallback chain.
- [`internal/registry` package](../packages/registry.md) - the `Client` and `SearchResult` types.
