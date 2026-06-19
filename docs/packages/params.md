# `internal/params`

Typed param spec, the huh form, and the default-application path. The leaf package - imports nothing else in the repo.

Source: `internal/params/param.go` + per-type files in the same dir.

## Public types

### `Spec` - the wire shape

Defined in `param.go:55-62`. What gets parsed out of `spin.toml`:

```go
type Spec struct {
    Type    Type    // one of the 8 constants below
    Prompt  string  // shown above the huh field
    Default any     // default value; type depends on Type
    Min     *int    // for number; nil = unbounded
    Max     *int    // for number; nil = unbounded
    Options []string // for select / multiselect
}
```

`Min` and `Max` are pointers so their absence is distinguishable from zero. The `Default any` decodes from TOML - BurntSushi gives back a `string` for `name = "x"`, a `float64` for `name = 8080`, a `[]any` for `name = ["a", "b"]`. The `asXXX` helpers in `parse.go:48-91` coerce these into the right Go primitive.

### `Type` - the 8 param kinds

Defined in `param.go:16-25`. Constants:

- `text` - single-line input
- `textarea` - multi-line input, `CharLimit(0)` (unlimited)
- `number` - input with min/max validation
- `select` - single-select from `Options`
- `multiselect` - multi-select from `Options`; defaults pre-selected
- `bool` - yes/no confirm
- `path` - file picker (or directory picker, via `NewDir`)
- `secret` - input with `EchoMode(huh.EchoModePassword)`

An empty `Type` defaults to `text` in `ParseOne` (parse.go:27-28).

### `Value` - the resolved value

`param.go:29-35`. Only one field is populated depending on `Type`:

```go
type Value struct {
    String string
    Int    int
    Bool   bool
    List   []string
    Path   string
}
```

### `Param` - the interface

`param.go:38-51`. Every concrete param type implements:

```go
type Param interface {
    Name() string
    Type() Type
    Prompt() string
    Default() any
    Hmm() huh.Field          // the huh form field
    Apply(v Value)            // write a Value into the param
    Value() Value             // read the param's current Value
    String() string           // for --print-params / debugging
}
```

Per-type implementations live in `text.go`, `textarea.go`, `number.go`, `select.go`, `multiselect.go`, `bool.go`, `path.go`, `secret.go`.

## Parsing

`Parse(specs SpecMap) ([]Param, error)` and `ParseOne(name string, s Spec) (Param, error)` in `parse.go:12-46`. `SpecMap` is just `map[string]Spec` (parse.go:8). Unknown `Type` returns `ErrUnknownType` (param.go:65-72) wrapped with the offending name.

`coerceParamValue` (parse.go:76-88) handles the inline-table-vs-shorthand split:
- Shorthand: `license = "MIT"` → `{Type: "text", Default: "MIT"}`
- Inline: `port = { type = "number", default = 8080, min = 1, max = 65535 }` → `{Type: "number", Default: 8080, Min: &1, Max: &65535}`

`specFromMap` (parse.go:90-117) walks the inline-table keys for `type / prompt / default / min / max / options`. `asInt64` (parse.go:119-129) accepts int / int64 / float64 (TOML numbers come back as float64 from BurntSushi).

## The form

`Form(ps) huh.Form` in `form.go:17-31` groups params into `huh.Group`s. `PageSize = 4` (form.go:11) - 4 params per form page, scrollable for the rest.

`Run(ps)` (form.go:35-37) is `Form(ps).Run()`. Returns `huh.ErrUserAborted` when the user presses `Esc` or `Ctrl+C`; `cmd/new.go:131-137` catches that explicitly.

`SetDefaults(ps)` (form.go:41-66) is the non-interactive path. Walks each param and applies the `Default` to its `Value` based on `Type`. This is what `--param` skips, what `--dry-run` runs, and what `spin new` runs when stdin is not a TTY.

## How it fits in the pipeline

```
spin new
  -> applyParamFlags (cmd/new.go:375)   // --param key=value
  -> Template.ResolveForm
       -> params.SetDefaults(values)     // non-interactive
       -> OR: params.Run(values)         // interactive huh form
       -> layer caller-supplied values on top
       -> return unwrapped primitives
  -> Template.Render (text/template)    // values are the template vars
```

The "unwrapped primitives" detail matters: `text/template` needs `{{.name}}` to resolve to a string, not a `Value` struct. `template.UnwrapValue` (template/form.go:98-114) and `template.Resolver.unwrapValues` (template/form.go:64-76) do this conversion.

## Edge cases

- **Param name collisions** with built-in template vars (`name`, `project_name`): `cmd/new.go:110-113` pre-populates `name` and `project_name` from `args[0]`. A user-supplied `--param name=...` overrides the built-in (caller values layer on top of the default).
- **`Default` for a `multiselect`**: TOML-decoded as `[]any`. `asStringSlice` (parse.go:77-91) coerces to `[]string`. The huh field pre-selects those via `opt.Selected(true)`.
- **`Default` for a `secret`**: intentionally not pre-filled into the huh placeholder (cmd/new.go:451-452). Secrets in the placeholder are a footgun.
- **`NewDir` constructor for `path`**: the source has it but the live `ParseOne` does not call it - both are parsed as `TypePath` (file mode). Adding a `dir` sub-type is a one-line change but not currently wired.

## Tests

- `internal/params/parse_test.go` - covers all 8 types, the shorthand/inline split, the `asXXX` coercions.
- `cmd/param_test.go` - end-to-end tests of `--param` coercion at the CLI layer.

## Related

- [Non-interactive use](../concepts/non-interactive.md) - the `--param` grammar in detail.
- [Template schema](../concepts/template-schema.md) - what `[params.<name>]` blocks look like in `spin.toml`.
- [`internal/template` package](template.md) - the consumer of `Spec` and `Param`.
