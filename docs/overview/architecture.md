# Architecture

`spin` is built around one first-class concept (a Template) and two pipelines (a template pipeline and a registry pipeline). This page describes how the four internal packages fit together and where the boundary between them lives.

## The two pipelines

### Template pipeline (`spin new`)

```
cmd/new.go
  -> internal/template/Loader.Load(spec)
       -> [local path] Detect(dir)
       -> [git URL]    cloneGit + Detect
       -> [pin name]   loadPinned
  -> internal/template.Template.ResolveForm(values, interactive)
       -> internal/params.SetDefaults (non-interactive)
       -> OR:           internal/params.Form.Run (huh form)
  -> internal/template.Template.RenderToWithPost(dest, resolved)
       -> Render(values)
            -> walk _base/, run text/template on .tmpl files
       -> writeFiles(dest, files)  // path-traversal guarded
       -> RunPostHook(t, values, dest)
            -> for each [[post]] step: render + sh -c
       -> deleteSpinToml(dest)  // TPL-16
```

### Registry pipeline (`spin add` / `list` / `update` / `remove` / `search`)

```
cmd/add.go    -> internal/registry/Client.Add(spec)     -> Client.Pin
cmd/list.go   -> internal/registry/Client.ListPinned
cmd/update.go -> internal/registry/Client.Refresh      -> Client.Pin
cmd/remove.go -> internal/registry/Client.Unpin        -> optional os.RemoveAll
cmd/search.go -> internal/registry/Client.Search       -> HTTP GET <IndexURL>/search?q=<q>
```

The two pipelines share the `Template` type (registry's `addGit` returns it for `add` callers) and the same `~/.config/spin/` filesystem layout, but they don't share any active state at runtime.

## The four internal packages

| Package | Role | Key types |
| --- | --- | --- |
| `internal/params` | Param spec + huh form | `Spec`, `Param`, `Form` |
| `internal/template` | Template load + render + post-hook | `Template`, `SpinToml`, `Loader` |
| `internal/registry` | Pin store + public registry HTTP | `Client`, `Pinned`, `SearchResult` |
| `internal/version` | Build-time version var | `Version` |

`internal/params` is the leaf - it doesn't import anything else in the repo. `internal/template` depends on `internal/params` (param specs) and `internal/registry` (pin lookups in the loader). `internal/registry` depends on `internal/version` only for the user-facing fallback messages.

## The Template type

The single type that ties the pipelines together. Defined in `internal/template/template.go:15-22`:

```go
type Template struct {
    Name        string
    Source      string  // local path on disk (post-clone)
    Repo        string  // git URL, if any (empty for local paths)
    SpinToml    *SpinToml
    BaseDir     string  // _base/ inside Source
    PostHookDir string  // _post/ inside Source (currently unused)
}
```

`Source` is always set; `Repo` is set only for git-sourced templates. The `cmd/` code uses `tpl.Repo != ""` as the gate for the post-success pin prompt.

## The Loader's three-mode dispatch

`internal/template/loader.go:67-84` `Load(spec)` tries, in order:

1. **Local path** (`isLocalPath`: starts with `/`, `.`, or `~`). Calls `Detect(dir)` directly. No network.
2. **Git URL** (`isGitURL`: starts with `http://`, `https://`, `git@`, `git://`, `ssh://`). Shallow-clones into the cache, then `Detect`s.
3. **Pinned name** (anything else). Looks up `spec` in `pinned.json`. Errors with "re-run `spin add`" if the on-disk cache is missing.

There is no implicit `user/repo` shorthand expansion in the loader. That's `Client.Add`'s job, only called by `spin add`. So `spin new --template foo/bar` (without a prior `spin add foo/bar`) errors out.

## Why the split

The `internal/template` package is the **rendering** concern: how to take a Template and a values map and produce a tree on disk. The `internal/registry` package is the **pinning + discovery** concern: how to remember a template, refresh it, and find new ones. They're separated so a future alternate loader (e.g. an OCI registry or a private HTTP server) can be added without touching the renderer.

## Related

- [What is spin?](what-is-spin.md) - the pitch and core value.
- [`internal/template` package](packages/template.md) - the renderer in detail.
- [`internal/registry` package](packages/registry.md) - the pin store in detail.
- [Pinning model](concepts/pinning.md) - the `pinned.json` schema and the shorthand expansion.
- [Template engine](concepts/template-engine.md) - the funcs, the path-traversal guard, TPL-16.
