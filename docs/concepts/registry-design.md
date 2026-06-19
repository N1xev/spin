# Registry design

The server side of `spin search`. Where it runs, what's in the database, how authors submit templates, how the catalog is built and deployed, and how the `.invalid` default flips over to a real URL when the server goes live.

Source: `internal/registry/client.go`, `internal/registry/types.go`, `docs/concepts/registry-protocol.md` (the CLI-side companion).

## The split

There are two artifacts, kept in two repos:

| Repo | What it is | What it does |
| --- | --- | --- |
| `github.com/N1xev/spin` | The CLI (this repo) | Reads from the registry. `spin search`, `spin add`, `spin update`. |
| `github.com/N1xev/spin-index` | The template catalog | A `templates/index.toml` file. PR-based. The source of truth. |
| `github.com/N1xev/spin-registry` | The server | A Go service. Reads the catalog, serves `/v1/search` + `/v1/templates/...`. Deployed to Fly.io. |

The CLI **never** writes to the registry. It only reads. Authors never talk to the server directly; they talk to the index repo. This keeps the server's surface area read-only and the auth model simple.

## Why three repos

- **CLI and server evolve at different cadences.** The CLI ships whenever a flag changes; the server ships when the schema or the API does. Sharing a repo would tie them together.
- **The index repo is small enough to read in a single PR review.** It's a TOML file. The reviewer can see "this adds one template" without paging through Go code.
- **The server has its own CI** (build + deploy to Fly) that does not run on CLI PRs.

## Architecture

```
+--------+        PR + spin.toml          +-----------------+
| Author | -----------------------------> | spin-index repo |
+--------+                                +-----------------+
                                                  |
                                          CI: validate every entry
                                          by running `spin new` against
                                          the candidate template repo
                                                  |
                                                  v
                                       +-------------------+
                                       | index.toml (merged)|
                                       +-------------------+
                                                  |
                                                  v
                                       +-------------------+
                                       | spin-registry     |
                                       |  (Go server,      |
                                       |   Fly.io)         |
                                       |                   |
                                       |  /v1/search       |
                                       |  /v1/templates/x  |
                                       +-------------------+
                                                  ^
                                                  |  HTTP GET
                                       +-------------------+
                                       |  spin CLI         |
                                       |  (user machine)   |
                                       +-------------------+
```

Three repos, one direction of data flow. The index repo is the only thing authors touch; the CLI is the only thing users touch; the server is the only thing the CLI talks to.

## The data model

The server stores the catalog in Postgres (Fly's free tier includes a small Postgres). One schema, three tables.

```sql
CREATE TABLE authors (
    id          BIGSERIAL PRIMARY KEY,
    github_login TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE templates (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,        -- "go-cli-bubbletea"
    description  TEXT NOT NULL,                -- one-liner
    language     TEXT NOT NULL,                -- "go"
    type         TEXT NOT NULL,                -- "tui" | "cli" | "lib" | "service"
    tags         TEXT[] NOT NULL DEFAULT '{}', -- ["bubbletea", "lipgloss"]
    source_url   TEXT NOT NULL,                -- https://github.com/foo/bar
    source_ref   TEXT NOT NULL DEFAULT 'main', -- branch or tag
    author_id    BIGINT NOT NULL REFERENCES authors(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    downloads    BIGINT NOT NULL DEFAULT 0     -- incremented on each /v1/templates/:name hit
);

CREATE INDEX templates_name_trgm ON templates USING gin (name gin_trgm_ops);
CREATE INDEX templates_tags_idx ON templates USING gin (tags);
CREATE INDEX templates_description_trgm ON templates USING gin (description gin_trm_ops);

CREATE TABLE template_versions (
    id          BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    version     TEXT NOT NULL,                  -- semver
    commit_sha  TEXT NOT NULL,                  -- the git SHA at publish time
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (template_id, version)
);
```

Why `source_url` lives in the table and not just in the index repo: it's the pointer the CLI uses to `git clone`. Putting it next to the rest of the metadata means a single SQL query serves `/v1/search` and a single SQL query serves `/v1/templates/:name`. The CLI never has to know that an index repo exists.

The `gin_trgm_ops` index is a Postgres trigram extension. It makes `WHERE name ILIKE '%q%'` fast (sublinear in the table size). Activate it once:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

## The HTTP API

The full surface, in five endpoints. The CLI uses one of them today (`GET /v1/search`); the rest exist for browsers / future clients / the maintainer's `spin-registry` admin tool.

### `GET /v1/search?q=<query>&limit=<n>&offset=<n>`

The endpoint `spin search` hits. Same wire format as the CLI expects today (`docs/concepts/registry-protocol.md`).

| Param | Default | Meaning |
| --- | --- | --- |
| `q` | `""` | Free-text query. Matches against `name`, `description`, `tags`. |
| `limit` | `20` | Max rows returned. Hard cap at 100. |
| `offset` | `0` | Pagination. |
| `type` | unset | Optional filter (`tui`, `cli`, `lib`, `service`). |
| `language` | unset | Optional filter (`go`, `rust`, `typescript`, ...). |
| `sort` | `relevance` | `relevance` (trigram similarity), `downloads`, `updated_at`, `name`. |

The response:

```json
{
  "query": "go",
  "total": 42,
  "entries": [
    {
      "name": "go-cli-bubbletea",
      "description": "Cobra + bubbletea v2 TUI scaffold",
      "tags": ["go", "bubbletea", "tui"],
      "language": "go",
      "type": "tui",
      "version": "0.3.1",
      "downloads": 1234,
      "source": "https://github.com/me/go-cli-bubbletea.git",
      "updated_at": "2026-06-14T12:34:56Z"
    }
  ]
}
```

Server-side ranking for `sort=relevance`: a weighted sum of trigram similarity on `name` (highest weight), `description` (medium), and tag overlap (lowest). Tunable; the v1 weights are `name=1.0, description=0.4, tag_overlap=0.7`. Documented in the `Search()` query function in the server source.

### `GET /v1/templates/:name`

Single template lookup. Used by `spin add` in a future variant that resolves a name to a URL without a search round-trip. Returns one `Entry` shape, plus the version history.

```json
{
  "entry": { "name": "go-cli-bubbletea", "...": "..." },
  "versions": [
    { "version": "0.3.1", "commit_sha": "abc123...", "published_at": "2026-06-14T12:34:56Z" },
    { "version": "0.3.0", "commit_sha": "def456...", "published_at": "2026-05-30T08:11:22Z" }
  ]
}
```

### `GET /v1/templates/:name/versions`

Just the version list. Lighter than the full `/templates/:name`; intended for `spin update` once the CLI grows a "what's the latest version of this template?" call.

### `GET /v1/healthz`

Liveness probe. Returns `{"ok": true, "version": "0.1.0", "git_sha": "..."}`. Fly's health check hits this every 10 seconds.

### `GET /v1/metrics` (optional, v1.1)

Prometheus-format counters: `spin_registry_search_total`, `spin_registry_template_lookup_total{name="..."}`. Not in v1; add when there's a Grafana board to put them on.

## The index repo

`github.com/N1xev/spin-index`. A single TOML file is the catalog.

```toml
# templates/index.toml

[[template]]
name        = "go-cli-bubbletea"
description = "Cobra + bubbletea v2 TUI scaffold"
language    = "go"
type        = "tui"
tags        = ["go", "bubbletea", "tui", "cobra"]
source      = "https://github.com/N1xev/go-cli-bubbletea.git"
ref         = "main"
author      = "N1xev"

[[template]]
name        = "rust-cli"
description = "Minimal Rust CLI with clap"
language    = "rust"
type        = "cli"
tags        = ["rust", "clap"]
source      = "https://github.com/N1xev/rust-cli.git"
ref         = "main"
author      = "N1xev"
```

### The PR workflow

1. Author forks `spin-index`.
2. Author adds a `[[template]]` block to `templates/index.toml`.
3. Author opens a PR. CI runs `.github/workflows/validate.yml` (see below).
4. Maintainer reviews the PR: does the template exist? does the description match? is the license OK?
5. Maintainer merges.
6. A second CI job (`.github/workflows/publish.yml`) builds and deploys the new server image to Fly.io with the updated catalog baked in.
7. Within 60 seconds (Fly's edge proxy cache TTL), `spin search` returns the new template.

### The validation CI

`.github/workflows/validate.yml` runs on every PR. For each `[[template]]` block in the diff:

1. `git clone <source_url> --depth=1 --branch <ref>` into a temp dir.
2. Assert the dir has `spin.toml`.
3. Parse `spin.toml` with `internal/template/parse.go` (export it as a library).
4. Run `bin/spin new validate-test --template <tempdir> --param name=validate-test --dest <out>` non-interactively.
5. Assert the render succeeded, the dest has files, no `spin.toml` leaked (TPL-16).
6. Run `cd <out> && go mod tidy && CGO_ENABLED=0 go build ./...` (only for `language = "go"` templates).
7. Post a comment on the PR with the result.

A failure in any step blocks the PR. The author fixes and pushes; CI re-runs.

### The publish CI

`.github/workflows/publish.yml` runs on merge to `main`:

1. Check out the repo.
2. Check out `github.com/N1xev/spin-registry` at the pinned SHA (via the `REGISTRY_REPO_SHA` secret in this repo's settings).
3. Copy `templates/index.toml` into the server's working tree.
4. Build the server (`go build -o out/registry .`).
5. Run `flyctl deploy --image <built image>`.

The server is rebuilt and redeployed on every index merge. No migration step is needed (the catalog is a TOML file, not rows in a DB that need updating) - the new image has the new catalog baked in.

The first deploy writes the catalog into the Postgres `templates` table via a one-shot migration:

```go
// In the server's main():
if err := loadCatalogFromTOML("index.toml", db); err != nil { ... }
```

`loadCatalogFromTOML` does an upsert per entry (matched on `name`). The DB is a cache of the TOML; the TOML is the source of truth. The `downloads` counter and the `template_versions` table are the only things that exist outside the TOML.

## The server itself

A Go service. The minimum viable shape:

```
spin-registry/
  main.go                # cobra root, subcommand: serve
  cmd/
    serve.go             # http.Server + graceful shutdown
  internal/
    api/
      search.go          # GET /v1/search
      templates.go       # GET /v1/templates/:name, /versions
      health.go          # GET /v1/healthz
    catalog/
      load.go            # TOML -> []Template
      toml.go            # struct tags
    db/
      db.go              # *sql.DB, migrations
      templates.go       # CRUD
      search.go          # the search query
    version/
      version.go         # build-time vars (mirrors the CLI)
  index.toml             # symlink or copy of the upstream index repo's file
  Dockerfile             # multi-stage: golang:1.25 -> distroless
  fly.toml               # Fly config (see below)
```

The whole thing is ~600 lines of Go. No framework - `net/http` is enough for five endpoints. The Postgres driver is `github.com/jackc/pgx/v5` with the stdlib `database/sql` adapter (so we can use `?` placeholders in a portable way if we ever move off Postgres).

## The Dockerfile

Multi-stage to keep the deployable image small.

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/registry .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/registry /registry
COPY index.toml /index.toml
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/registry", "serve"]
```

`distroless/static` is ~2MB and has no shell, no package manager, no busybox. A `nonroot` user is the default. Attack surface is essentially "the Go binary, listening on 8080".

## The fly.toml

```toml
app = "spin-registry"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[env]
  DATABASE_URL = "<set via fly secrets>"
  PORT = "8080"

[[services]]
  internal_port = 8080
  protocol = "tcp"
  auto_stop_machines = true   # free-tier friendly
  auto_start_machines = true
  min_machines_running = 0

  [[services.ports]]
    handlers = ["http"]
    port = 80
    force_https = true

  [[services.ports]]
    handlers = ["tls", "http"]
    port = 443

  [[services.http_checks]]
    interval = "10s"
    timeout = "2s"
    grace_period = "5s"
    method = "get"
    path = "/v1/healthz"

[[vm]]
  size = "shared-cpu-1x"
  memory = "256mb"             # free tier cap
  cpu_kind = "shared"
  cpus = 1
```

`min_machines_running = 0` + `auto_stop_machines = true` means the app sleeps on idle and wakes on the next request. Cold start is ~3s for a Go binary off disk; acceptable for a registry that's not on the hot path. The first user to search after idle pays the cold start; everyone else hits a warm instance.

`auto_start_machines = true` with `min_machines_running = 0` is the free-tier pattern: scale to zero when idle, scale up on demand. The trade-off is the cold start, but the registry is not user-perceived latency-critical (the user is typing in a TUI).

The `DATABASE_URL` is set via `fly secrets set DATABASE_URL=postgres://...`. The free Postgres on Fly is a separate app attached via `fly postgres attach`.

## The CLI's role

The CLI side is **unchanged** when the registry comes online. The wire format in `docs/concepts/registry-protocol.md` is exactly what the server returns. The only change is the URL:

- **Before deploy**: `https://registry.spin.invalid/v1` (DNS fails fast, friendly "not yet deployed" message).
- **After deploy**: the same URL resolves to a real Fly app. The CLI's `Client.Search` does not change.

The `spin list --json` output and the `pinned.json` format are also unchanged. The CLI downloads the template via `git clone` of `entry.source` after `spin add` - it never streams bytes from the registry. This means the registry is a metadata service, not a content service, and the storage bill is bounded by the size of `index.toml` (a few KB), not by the size of every template.

## Search backend

Postgres, not Meilisearch or Typesense. The dataset is small (hundreds of templates, not millions) and the query surface is simple (text + tag + language + type filters). `pg_trgm` + `tsvector` together cover:

- **Fuzzy text**: `name % q` uses trigram similarity. Catches typos and partial matches.
- **Tags**: `tags @> ARRAY[<tag>]` uses the GIN index on `tags`. Fast exact-match.
- **Type / language filters**: simple `WHERE` clauses, indexed trivially.
- **Ranking**: weighted sum of the above, computed in SQL.

If the dataset grows past ~10k templates or the ranking quality gets bad, the upgrade path is:

1. Add a `tsvector` column generated from `name || ' ' || description || ' ' || array_to_string(tags, ' ')`.
2. Create a GIN index on that column.
3. Switch the `sort=relevance` path to use `ts_rank` instead of trigram similarity.

If ranking still isn't good enough, swap the in-process search for Meilisearch or Typesense. The API shape (`/v1/search?q=...`) stays the same; only the internal query function changes.

## Auth model

**None for read.** `/v1/search`, `/v1/templates/:name`, `/v1/templates/:name/versions` are public. The CLI does not send any auth header. The 15-second HTTP timeout bounds the worst case.

**None for write either, because writes don't happen.** The only way a new template enters the registry is the PR-to-`spin-index` workflow. The server has no `POST` / `PUT` / `DELETE` endpoints. The maintainer's CI deploys a new image with the updated catalog. There is no per-author account, no token, no rate limit per identity.

This is a deliberate constraint. Adding write auth means choosing an identity provider, dealing with token rotation, and operating an abuse pipeline. The PR-based workflow is "auth" via GitHub's existing PR review: the maintainer is the only entity that can land a change to the catalog, and they do it through a process that already has an audit log (the PR history of `spin-index`).

If a future use case requires direct submission (e.g., a CI pipeline publishing version bumps), the auth path is:

1. Add a `POST /v1/templates/:name/versions` endpoint.
2. Auth via a deploy key (a per-CI-runner shared secret).
3. The CLI never calls this endpoint; only the `spin-registry` admin tool does.

The 80/20: read-only public API, PR-gated writes. Don't ship write auth until there's a concrete use case for it.

## Cost and operations

| Resource | Free tier? | Where the limit bites |
| --- | --- | --- |
| Fly.io app (256MB shared-cpu) | yes | 3 apps per org. We're using 1. |
| Fly.io Postgres | yes (1 shared, 1GB) | Storage. The catalog is tiny; 1GB is more than enough. |
| Bandwidth | free up to 160GB/mo | One search call is ~1KB. We can serve 100M searches/mo for free. |
| Custom domain | yes (`registry.spin.invalid` -> Fly IP) | DNS provider. Use Fly's `flyctl certs` to provision a Let's Encrypt cert. |
| The 15s HTTP timeout | N/A | Server side; the CLI has it baked in. |

The expected monthly cost of a registry serving <1M searches/mo is **$0**. The first bill shows up only if we exceed the free tier on a single dimension (very unlikely at this scale) or attach a paid Postgres (not needed).

### What the maintainer runs

- `fly apps list` - confirm the app is up.
- `fly logs` - tail the access log.
- `fly postgres connect -a spin-registry-pg` - debug DB state.
- `fly secrets list` - confirm `DATABASE_URL` is set.

That's it. No k8s, no Ansible, no Terraform. The CI rebuilds and deploys on every index merge; the maintainer just reviews PRs.

## The `.invalid` -> real URL migration

The CLI default is `https://registry.spin.invalid/v1`. The migration when the server goes live:

1. Deploy the Fly app.
2. Add a DNS A record for `registry.spin.invalid` pointing to the Fly app's IP. Wait for propagation.
3. Ship a new CLI release that bumps `DefaultIndexURL` to the now-real URL. (Optional: the env var override `SPIN_REGISTRY_URL` lets users opt in earlier.)
4. Old CLI builds still hit `.invalid`; the DNS now resolves to the real server.

The two-line change to `internal/registry/types.go`:

```go
const DefaultIndexURL = "https://registry.spin.invalid/v1"
// becomes
const DefaultIndexURL = "https://registry.spin.invalid/v1"  // unchanged
```

Plus a DNS record. The server side has no special "first deploy" code path; the first deploy is just a normal deploy with an empty `templates` table that the bootstrap migration seeds from `index.toml`.

## Why the index repo, not the registry server, is the source of truth

| Concern | Index repo | Registry server |
| --- | --- | --- |
| Audit log | PR history (free) | DB rows (no per-row history) |
| Review | GitHub PR UI (free) | Admin tool you'd have to build |
| Rollback | `git revert` (free) | DB migration backwards (build it) |
| Local dev | Edit a TOML, run `fly deploy` (or `go run .`) | Edit rows in a hosted DB (build it) |
| Discoverability | Anyone can read the repo | Same, but the URL is API-shaped |

The pattern: **the source of truth is a file in a repo**. The server is a cache + query layer. The DB is a cache of the file. If the DB dies, the server can re-bootstrap from the file in 30 seconds.

## Edge cases

- **The first template in the registry**: PR to `spin-index` adding one `[[template]]` block. CI validates it. CI deploys the server. The server bootstraps the `templates` table from the TOML. `spin search <name>` returns the entry.
- **Author deletes their template repo**: the next `spin update` of a user who pinned it errors with "source %s is gone" (`internal/registry/client.go:472-474`). The registry entry stays; the user sees the template is unavailable. The maintainer can mark it `deprecated` in the index repo (an extra `deprecated = true` field the server respects by hiding it from search and showing a banner on direct lookups).
- **Two templates with the same `name`**: rejected at the validation step (the TOML parser errors on duplicate keys). The author must rename one.
- **Author changes `source` between versions**: allowed; the `template_versions` table records the SHA at publish time. A user who pinned the old source can `spin update` and the new source is fetched.
- **Search returns 0 results**: the CLI's `Search` returns `(result, nil)` with `Entries: nil`. `cmd/search.go:55-60` handles `len(out.Entries) == 0` with a "no templates matched" message. The friendly "not yet deployed" path is **not** triggered by an empty result - it's only triggered by a network failure or HTTP 404. So an empty search is "the registry is up, nothing matched", not "the registry is down".
- **Fly Postgres sleeps**: it doesn't, by default. Postgres on Fly is a separate, always-on app. Only the compute app sleeps. The DB is free but persistent.
- **Cold start on first search after idle**: 3-5s. The user sees the spinner for a beat longer than usual. Acceptable for a CLI command they ran by hand; not acceptable for a hot path. If this becomes a complaint, the fix is `min_machines_running = 1` (still free if traffic is low enough to fit in 256MB RAM).
- **DNS propagation delay when `.invalid` flips**: up to 48h for the worst-case resolver. The CLI's `SPIN_REGISTRY_URL` override lets users opt in early. The `.invalid` -> real transition should be coordinated with a CLI release that ships a fallback env var default.

## What is NOT in v1

| Feature | Why deferred |
| --- | --- |
| Author accounts / profile pages | PR-based submission doesn't need them. |
| Web UI for browsing | The CLI is the UI. A future web UI can read the same `/v1/search` API. |
| Template version pinning at search time | The CLI already pins by `git clone`, not by registry version. |
| Download counts per version | The counter is per-template, not per-version. Add a `template_version_downloads` table if/when this matters. |
| Write API for `spin publish` | The PR-based workflow covers the v1 use case. Add when the maintainer is the bottleneck. |
| Auto-suspension of broken templates | Out of scope. A maintainer can mark a template `deprecated` in the index. |
| Multi-region deploy | One region (Fly's `iad`) is fine for a small registry. Add a second region only if p95 latency from another continent gets bad. |

## Related

- [Registry protocol](registry-protocol.md) - the CLI-side wire format. Read this first; this doc is the server-side counterpart.
- [XDG layout and env vars](xdg-layout.md) - the `SPIN_REGISTRY_URL` / `SPIN_REGISTRY` env vars that point the CLI at this server.
- [Pinning model](pinning.md) - what happens after `spin add` resolves a name to a source URL via this server.
- [Template schema](template-schema.md) - the `spin.toml` grammar the validation CI parses in every PR.
- [`internal/registry` package](../packages/registry.md) - the client that talks to this server.
- [`spin search`](../commands/search.md) - the only CLI command that hits the registry directly.
- [spin-registry README](https://github.com/N1xev/spin-registry/blob/main/README.md) - the server's own docs: endpoints, local dev, deploy.
- [spin-index README](https://github.com/N1xev/spin-index/blob/main/README.md) - the catalog's own docs: author guide, PR workflow.
