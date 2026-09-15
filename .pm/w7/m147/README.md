# w7 · m147 — Restore `bex ea sandboxes exec` under the pinned Render CLI: run connect-token handshake + SSE exit/error shapes

**Worker:** worker7 **Goal:** the shipped `bex` launcher (Render CLI v2.27.0 pin) runs a command in a sandbox end to end and returns that command's real exit status and error messages, as the compatibility ledger already claims. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Reproduce both failures live with the pinned CLI and capture the redacted wire            | 30m | —                          |
| t002 | Emit the pinned client's SSE `exit`/`error` payload shapes without breaking internal readers | 40m | —                          |
| t003 | Mint run connect tokens: `POST /v1/sandboxes/{sandboxId}/runs/{operation}/token`          | 60m | —                          |
| t004 | Redeem the connect token at the returned `uri` and stream the exec                         | 75m | w7/m147/t003               |
| t005 | Blast-radius verification across exec callers, legacy route, and route guards              | 40m | w7/m147/t002, w7/m147/t004 |
| t006 | Re-grade the sandbox rows in the CLI ledger, including pinned-but-ungraded commands        | 25m | w7/m147/t005               |
| t007 | Render parity                                                                              | 20m | w7/m147/t006               |
| t008 | Simplify                                                                                   | 20m | w7/m147/t007               |
| t009 | Test coverage                                                                              | 45m | w7/m147/t007               |
| t010 | Closeout                                                                                   | 10m | w7/m147/t008, w7/m147/t009 |

## Definition of done

With the pinned launcher (`lego/cli`, Render v2.27.0 / `a764810a`) against production, human device login, a free disposable sandbox owned by the run:

- `bex ea sandboxes exec <sbx-id> -- sh -c 'echo out; echo err >&2; exit 7'` prints `out` on stdout and `err` on stderr and **exits 7**; `-- true` exits 0. Today the first request is `POST /v1/sandboxes/<id>/runs/stream/token`, a route bex does not register, so the command fails before any output. Even on the legacy route, the exit code is lost (see below).
- An exec against a terminated or unknown sandbox exits non-zero with a **non-empty** message from bex, not `sandbox exec stream error status 0: `.
- The connect token only authorizes this one operation on this sandbox for this owner. It expires (`expiresAt`) and cannot be replayed, used on another sandbox, or used as an API bearer token. A foreign or ownerless sandbox id gets the same `SANDBOX_NOT_FOUND` 404 as a missing one, and a caller without `can_create` is refused before any token is minted.
- MCP `sandbox_exec`, agent-session status and hibernate reads, and the scrub-before-suspend exec still see correct exit codes and errors. The legacy `POST /v1/sandboxes/{id}/exec` keeps working for older clients.
- `docs/cli-compatibility-checklist.md` and `docs/render-artifacts/ea-sandbox.md` describe the handshake the pinned client actually uses, and `ea sandboxes copy`, `ea sandbox-groups list`, and `ea sandboxes snapshots *` each carry an explicit grade.
- Delete the disposable sandbox, confirm it is absent from `ea sandboxes list`, and leave no pre-existing resource changed.

## Evidence (static; live repro is t001)

Filed from a continuous `/qa-find-bugs-cli` sweep for w7 on 2026-09-14 UTC. Live CLI testing was blocked that sweep (QA credentials absent), so both defects below are **source-proven, not yet reproduced live**. t001 converts them into a live wire capture before implementation.

**Defect A (blocker for the journey): the exec handshake route is missing.** In the pinned module `github.com/render-oss/cli@v1.1.3-0.20260909214233-a764810a7682`:

- `pkg/sandbox/repo.go:105-133`: `ExecSandboxStream` first calls `connect()`.
- `repo.go:298-324`: `connect()` sends `ConnectSandboxRunWithResponse(ctx, id, "stream", {OwnerId}, {command})`. The generated path is `pkg/client/client_gen.go:20227`, `POST /sandboxes/{id}/runs/{operation}/token`. It requires `JSON201` of type `SandboxConnectResponse` (`pkg/client/sandboxes/sandboxes_gen.go:310`): `{executionId, expiresAt, method, token, uri}`. A 201 without that body is its own error: `connect sandbox: success response missing connect token`.
- The client then sends `{"command":…}` with `conn.Method` to `conn.Uri`. Headers: `Authorization: Bearer <conn.Token>`, `Content-Type: application/json`, `Accept: text/event-stream`. It requires 200 and parses SSE. Any other status goes through `errFromStreamResponse`: 401/403 become sentinels, otherwise `received response code N: <message>`.
- The handshake is also in the v2.24.0 (`fe8a618`) and v2.26.0 (`6c0f561`) module caches. v2.21.0 (`d8fd7c2`) used single-step `ExecSandboxSync`. So `ea sandbox exec` has been broken for launcher users since the w2/m79 pin bump (2026-08-19). The v2.26/v2.27 re-baselines were help-graded only and did not catch it.
- bex registers only `POST /v1/sandboxes/{id}/exec` (`lego/backend/internal/sandbox/rest.go:97`), plus create/list/get/terminate/pause/resume. Nothing under `runs/` exists in `lego/`, `deploy/`, or `infra/`, and `internal/api/scope_matrix_ops.go:693` lists only `/exec`. `w3/m33/t002` designed exactly this endpoint and was **DEFERRED** (`.pm/w3/done/m33/README.md:10,21`) because v2.21.0 did not use it.

**Defect B (major, predates the pin bump): the exit and error payloads do not match the consumer.**

- Every cached pin (v2.21.0 through v2.27.0) decodes `pkg/sandbox/sse.go:23-31` `execExitEvent{ExitCode int "json:\"exit_code\""}` and `execErrorEvent{Status int "json:\"status\""; Message string "json:\"message\""}`.
- The bex gateway emits `exitEvent{ExitCode int "json:\"exitCode\""}` and `errorEvent{Error string "json:\"error\""; Code string "json:\"code\""}` (`lego/backend/internal/sshgateway/sandboxsse/sandboxsse.go:257-263`). Its comment at `:250-253` claims to mirror the CLI exactly, which is false.
- A non-zero remote exit decodes as `0`. `cmd/sandboxexec.go:106-114` `exitSandboxExec` then returns nil, so **a failing sandbox command reports success**.
- An error event decodes as `status 0` with an empty message, printed as `sandbox exec stream error status 0: `.
- m33's live acceptance (`.pm/w3/done/m33/done/t005.md:40`) only ran exit-0 commands (`echo`, `uname -r`), which hid this. The backend tests pin the wrong key: `sandboxsse_test.go:111-115`, and `sandbox/exec_test.go:72,89,138,209,241,306`.

## Source + Goal linkage

- **Source:** continuous `/qa-find-bugs-cli` for w7, 2026-09-14 (sweep 2, static research while live auth was blocked). Checkout `0442e9063`; launcher build `bex vdev-34d0fa153` (Render v2.27.0). Deduped against open and done `.pm/` across all workstreams, w4 included: no open item covers the `runs/{operation}/token` handshake, `exit_code`, or the error-event shape. `w3/m33` (done) is the original exec work whose deferred t002 this revives.
- **Goal linkage:** Render-CLI compatibility of the imported launcher (ADR006, ADR018, `docs/bex-cli.md`) and pillar-5 sandboxes, reopened 2026-07-27 ([ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md), [ADR014](../../../docs/ADR014-sandboxes.md)). The ledger grades `ea sandbox exec` as `[x]`.
- **Expected outcome:** the only CLI path to run a command in a bex sandbox works with the shipped launcher, and scripts and agents can trust its exit status.
- **Why now:** the launcher is distributed (install script, Homebrew tap) with this pin, so every current user hits Defect A. Defect B silently turns failures into successes for anything that gates on exit status. Both are cheap to prove once auth is available, and the ledger currently certifies the broken behavior.
- **Render parity included:** the change adds tenant-facing REST routes and changes the SSE payload that REST and MCP consumers read. Shared-exec blast-radius verification is its own task (t005).
- **Out of scope:** implementing `ea sandboxes copy` (`/files/{operation}/token`), `sandbox-groups`, or snapshots. t006 grades them and files any follow-up. Ephemeral SSH and hosted-shell non-goals in `.pm/DO_NOT_DO.md` are untouched.
