# Registry protocol

The HTTP API `spin search` talks to. The wire format, the `ErrNotDeployed` mapping, and the `.invalid` TLD trick.

Server source: [`github.com/N1xev/spin-registry`](https://github.com/N1xev/spin-registry) - this is the implementation that serves `/v1/search`, `/v1/templates/:name`, `/v1/templates/:name/versions`, and `/v1/healthz`. The CLI is the read-side client; the catalog lives in [`github.com/N1xev/spin-index`](https://github.com/N1xev/spin-index). See [Registry design](registry-design.md) for the three-repo split.

Source: `internal/registry/client.go:54-130`, `types.go:14-59`.

## The endpoint

`GET <IndexURL>/search?q=<query>`. `<IndexURL>` is one of:

- `SPIN_REGISTRY_URL` if set (preferred).
- `SPIN_REGISTRY` if set (fallback for v2.0-skeleton callers).
- `https://registry.spin.invalid/v1` (the default).

The default uses the RFC 2606 reserved `.invalid` TLD, which never resolves. DNS lookup fails fast - no 15-second wait - and the client maps the failure to `ErrNotDeployed` (see below).

## The request

```http
GET /search?q=go HTTP/1.1
Host: <IndexURL host>
User-Agent: spin/<version>
```

The query string is URL-encoded via `url.Values.Encode()`. Only `q` is set. No auth, no cookies, no special headers.

## The response

```json
{
  "query": "go",
  "total": 42,
  "entries": [
    {
      "name": "go-cli",
      "description": "Minimal Go CLI with cobra",
      "tags": ["go", "cli", "starter"],
      "language": "go",
      "type": "cli",
      "version": "0.1.0",
      "downloads": 1234,
      "source": "https://github.com/me/go-cli-template.git",
      "updated_at": "2026-06-14T12:34:56Z"
    }
  ]
}
```

The shape is `SearchResult` (types.go:43-47) with `Entry` records (types.go:30-40). The server returns the full result; the client may clamp to the `--limit` (client.go:88-90).

## Error mapping

`isNetworkError` (client.go:99-130) maps low-level network failures to `ErrNotDeployed` (types.go:22-23):

| Failure | `errors.As` match | String-inspection match |
| --- | --- | --- |
| DNS lookup failure | `*net.DNSError` | `"no such host"` |
| Connection refused | `*net.OpError` | `"connection refused"` |
| I/O timeout | `*net.OpError` | `"i/o timeout"` |
| Network unreachable | `*net.OpError` | `"network is unreachable"` |
| No route to host | `*net.OpError` | `"no route to host"` |
| HTTP client timeout (15s) | (no `errors.As` match) | `"context deadline exceeded"` |

The string-inspection fallback exists because the stdlib wraps `*net.OpError` in `*url.Error` in some paths, and "context deadline exceeded" from the HTTP client's `Timeout` doesn't satisfy `errors.As` against either type.

`HTTP 404` from a real server is also `ErrNotDeployed` (client.go:77-79). Server operators who 404 `/search` instead of returning real data get the same friendly UX as the "not yet deployed" case.

Other HTTP failures return the status code and body verbatim:

```go
fmt.Errorf("registry: %s: %s", resp.Status, string(body))
```

Malformed JSON returns:

```go
fmt.Errorf("registry: decode: %w", err)
```

## The friendly path

`cmd/search.go:40-46` catches `ErrNotDeployed` and prints:

```
the public spin registry isn't deployed yet. Until it is, you can scaffold
from any git URL:
  spin new <name> --template <git-url>
  or pin one for offline use:
  spin add <git-url>
```

Exit code is 0 - "not yet deployed" is a normal state, not a failure. The user gets the hint, the script exits cleanly.

## The HTTP client

`Client.HTTP` (client.go:24) is a `*http.Client` with a 15-second `Timeout` (client.go:45). This bounds the total time a search call can take, including DNS, connect, TLS, and response read.

The 15s is not configurable. If the registry server is genuinely slow, the user sees a `context deadline exceeded` error, which `isNetworkError` maps to `ErrNotDeployed` (above) - so a slow server looks the same as a missing one.

## What the wire format is NOT

- Not a streaming API. The full result is one HTTP response.
- Not auth'd. No API keys, no bearer tokens. If a private registry is needed in the future, `SPIN_REGISTRY_URL` would be the place to put the auth (the client would need to learn to send it).
- Not paginated. `total` is a hint, `entries` is the full page. The `--limit` flag is a client-side clamp, not a server-side filter.

## Edge cases

- **The `.invalid` TLD is RFC 2606 reserved**: deliberately chosen so the default URL always fails fast. The test suite relies on this: it doesn't mock the network, it just expects `ErrNotDeployed`.
- **`SPIN_REGISTRY_URL` with a trailing slash**: `Client.New` doesn't trim it, but `Search` constructs the URL as `c.IndexURL + "/search"`, so a trailing slash is harmless.
- **Server returns `entries: null`**: `json.Decode` will fail with "json: cannot unmarshal null into Go struct field SearchResult.entries of type []registry.Entry" (Go's default for a missing-but-nullable field). The client wraps this as `registry: decode: ...`. A real server should return `[]` for empty results.
- **Server returns `total: 0` with non-empty `entries`**: the limit clamp (client.go:88-90) doesn't fire because `len(out.Entries) > limit` is false. The user sees all entries with `total: 0`. The fields are independent.

## Related

- [XDG layout and env vars](xdg-layout.md) - the env var precedence.
- [`spin search`](../commands/search.md) - the user-facing command.
- [`internal/registry` package](../packages/registry.md) - the `Client` and `SearchResult` types.
- [Registry design](registry-design.md) - the server side: hosting, the data model, the PR-based submission workflow, the deploy pipeline.
