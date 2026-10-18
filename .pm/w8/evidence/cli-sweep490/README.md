# CLI sweep 490 — `services get` is an upstream cobra fall-through, not a bex bug

**Verdict: no bug filed; one candidate researched to source and closed.** Zero fixtures were created; the sweep was read-only.

## Candidate and disposition

`bex services get <id> -o json` exits `0` for any argument — malformed (`srv-!!!`), well-formed-but-unknown, or real — and prints the whole workspace resource list (8 rows at probe time). The direct API read `GET /v1/services/{unknown}` correctly returns HTTP `404`, so the server side is right.

Root cause in the pinned upstream CLI (`github.com/render-oss/cli` at the version in [`lego/cli/go.mod`](../../../../lego/cli/go.mod)): there is no `get` subcommand under `services` at all — `bex services --help` lists only `create`, `delete`, `instances`, `update`. Cobra does not error on the unknown first argument; it runs the parent list command and ignores both `get` and the ID. The list path loads via `ListResourceInput` (`pkg/tui/views/resourceloader.go:8` in the pinned module), which has no ID field to filter by, so the full list is the only possible output. Both the HEAD build and the installed v0.2.1 binary behave identically, and the same fall-through would occur against Render's own API.

This is the same closed class as the unauthenticated-`logs` panic (w8/015): a pinned-upstream client wart that bex inherits by design of the launcher pin. Not filed. It also retro-corrected two earlier evidence notes (sweeps 488/489), which had credited "`bex services get`" for rows actually read from the head of the list output.

## Other probes (all expected)

| probe | observed result |
| --- | --- |
| `bex notifications`, `bex users`, `bex ssh-keys`, `bex envgroups` | `unknown command` at exit `1` — these are API-only or absent upstream, consistent everywhere. |

## Safety and unrun checks

- Read-only sweep; nothing created, nothing to clean up; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
