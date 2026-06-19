# Exit codes

What each spin subcommand returns to the shell, and when. Most users only ever see `0` and `1`; `130` is the SIGINT escape hatch.

Source: `main.go:1-28`, `cmd/*.go`, `internal/registry/types.go:22-23`, `cmd/search.go:40-46`.

## The three codes

| Code | Meaning |
| --- | --- |
| `0` | Success. Includes "not yet deployed" cases for `spin search` and the `--print-params` / `--dry-run` short-circuits for `spin new`. |
| `1` | A loader, parse, render, post-hook, or write error. Any unexpected failure. |
| `130` | The user hit `Ctrl-C` during an interactive form. The form is killed before any output is written. |

`main.go` uses `fang.Execute(ctx, rootCmd)` to translate cobra's per-command errors into exit codes. The translation is "command returned an error -> `os.Exit(1)`", which is why a typo in a flag, a missing template, a failed `git clone`, and a post-hook that exited non-zero all collapse into a single `1`.

## Per-command summary

| Command | 0 | 1 | 130 |
| --- | --- | --- | --- |
| `spin new` | scaffolded, or `--print-params` / `--dry-run` short-circuited | loader, parse, render, post-hook, write | form interrupted by `Ctrl-C` |
| `spin add` | pinned to `pinned.json` and the cache populated | git clone failed, source missing, write to `pinned.json` failed | not applicable (no form) |
| `spin list` | table or JSON printed, even when the pin store is empty | read error on `pinned.json` (corrupt JSON, permission) | not applicable |
| `spin update` | refreshed in place; old cache removed | git fetch failed, source missing, rollback failed (warned) | not applicable (no form) |
| `spin remove` | unpinned (and optionally purged) | pin name not found, `os.RemoveAll` failed | not applicable |
| `spin search` | results printed, **or** "not yet deployed" friendly path | registry returned a non-404 error, JSON decode failed | not applicable (no form) |
| `spin init` | template files written to `./<name>/` | output dir not writable, internal write error | not applicable (no form) |
| `spin version` | version string printed | not applicable | not applicable |

The `spin search` row is the unusual one: a `1` for "not deployed" would be hostile. See [Registry protocol](../concepts/registry-protocol.md) for the `ErrNotDeployed` mapping that makes the friendly path exit `0`.

## Why 130 and not 1 for SIGINT

When a user hits `Ctrl-C` in the middle of the huh form, the process should die **immediately** and the shell prompt should not see a stack trace. The cleanest way is to handle the abort in the form code and call `os.Exit(130)` directly, bypassing cobra's error path entirely. This is why `spin new` exits `130` on form cancel and `1` on a real error - the user explicitly asked to quit.

`130` is `128 + SIGINT(2)`, the standard shell convention for "killed by signal 2". Scripts that test for "user cancelled" can do so portably.

## `spin new` and the short-circuit flags

`--print-params` and `--dry-run` both exit `0` without writing anything to the destination. This is intentional: piping the JSON output of `--print-params` into `jq` should never fail; running `--dry-run` to preview a scaffold should never leave artifacts.

`--print-params` is checked first in `runNew` (cmd/new.go:141-146). If both flags are set, `--print-params` wins and `--dry-run` is ignored. Exit code is `0` in either case.

## CI / script conventions

```sh
# Strict: any non-zero is a failure
if ! spin new myapp --template go-cli --param port=9090; then
  echo "scaffold failed" >&2
  exit 1
fi

# Tolerant: accept "no registry yet" as success
if ! spin search go; then
  case $? in
    0) ;;                          # no error
    1) echo "registry error"; exit 1 ;;
    *) echo "unexpected exit"; exit 1 ;;
  esac
fi
```

The "treat 0/1 only" convention is safe because spin does not introduce new exit codes beyond the three above. A new subcommand that needs a distinct code should be loud about it in its command doc page.

## Edge cases

- **`spin new` with a Huh form and SIGINT during a non-form step** (e.g., the render phase): the form is already closed; SIGINT is delivered to the running process. Default Go behaviour: the process dies with the signal. The exit code is `130` from the shell's perspective (the signal convention), but the process itself may not have called `os.Exit(130)` explicitly.
- **`spin update` rollback failure**: the cache is renamed back. If the rename fails (e.g., dest dir permissions changed mid-update), the user gets a warning printed to stderr but the command still exits `1`. The cache may be in an inconsistent state; the next `spin update` is the user's recovery path.
- **`spin remove` on a nonexistent pin name**: returns `1` with a clear message. The pin store is not touched.
- **`spin search` with `SPIN_REGISTRY_URL` set to a real server returning 5xx**: exits `1` (registry error path), not `0` (friendly path). Only `404` and the network-error set map to `0`.
- **Process killed by SIGTERM (signal 15)**: shell sees exit `143` (`128 + 15`). spin does not install a SIGTERM handler; the default Go runtime behaviour applies. This is the same code any Go binary would return.

## Related

- [Non-interactive use](../concepts/non-interactive.md) - the `--print-params` and `--dry-run` short-circuits.
- [Registry protocol](../concepts/registry-protocol.md) - the `ErrNotDeployed` mapping that keeps `spin search` exit-0.
- [Pinning model](../concepts/pinning.md) - the rollback contract in `spin update`.
- [`spin new`](../commands/new.md), [`spin search`](../commands/search.md), [`spin update`](../commands/update.md) - the per-command docs.
