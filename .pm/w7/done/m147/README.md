# w7 · m147 — Restore `bex ea sandboxes exec` under the pinned Render CLI: run connect-token handshake + SSE exit/error shapes

**Worker:** worker7 **Goal:** the shipped `bex` launcher (Render CLI v2.27.0 pin) runs a command in a sandbox end to end and returns that command's real exit status and error messages, as the compatibility ledger already claims. **Status:** done — 2026-09-15 (code shipped `00e52317a`; live DoD verified against production after deploy run 34950840699).

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Reproduce both failures live with the pinned CLI and capture the redacted wire — **DONE** | 30m | —                          |
| t002 | Emit the pinned client's SSE `exit`/`error` payload shapes without breaking internal readers — **DONE** | 40m | —                          |
| t003 | Mint run connect tokens: `POST /v1/sandboxes/{sandboxId}/runs/{operation}/token` — **DONE** | 60m | —                          |
| t004 | Redeem the connect token at the returned `uri` and stream the exec — **DONE** | 75m | w7/m147/t003               |
| t005 | Blast-radius verification across exec callers, legacy route, and route guards — **DONE** | 40m | w7/m147/t002, w7/m147/t004 |
| t006 | Re-grade the sandbox rows in the CLI ledger, including pinned-but-ungraded commands — **DONE** | 25m | w7/m147/t005               |
| t007 | Render parity — **DONE** | 20m | w7/m147/t006               |
| t008 | Simplify — **DONE** | 20m | w7/m147/t007               |
| t009 | Test coverage — **DONE** | 45m | w7/m147/t007               |
| t010 | Closeout — **DONE** | 10m | w7/m147/t008, w7/m147/t009 |

## Definition of done

With the pinned launcher (`lego/cli`, Render v2.27.0 / `a764810a`) against production, human device login, a free disposable sandbox owned by the run:

- `bex ea sandboxes exec <sbx-id> -- sh -c 'echo out; echo err >&2; exit 7'` prints `out` on stdout and `err` on stderr and **exits 7**; `-- true` exits 0. Today the first request is `POST /v1/sandboxes/<id>/runs/stream/token`, a route bex does not register, so the command fails before any output. Even on the legacy route, the exit code is lost (see below).
- An exec against a terminated or unknown sandbox exits non-zero with a **non-empty** message from bex, not `sandbox exec stream error status 0: `.
- The connect token only authorizes this one operation on this sandbox for this owner. It expires (`expiresAt`) and cannot be replayed, used on another sandbox, or used as an API bearer token. A foreign or ownerless sandbox id gets the same `SANDBOX_NOT_FOUND` 404 as a missing one, and a caller without `can_create` is refused before any token is minted.
- MCP `sandbox_exec`, agent-session status and hibernate reads, and the scrub-before-suspend exec still see correct exit codes and errors. The legacy `POST /v1/sandboxes/{id}/exec` keeps working for older clients.
- `docs/cli-compatibility-checklist.md` and `docs/render-artifacts/ea-sandbox.md` describe the handshake the pinned client actually uses, and `ea sandboxes copy`, `ea sandbox-groups list`, and `ea sandboxes snapshots *` each carry an explicit grade.
- Delete the disposable sandbox, confirm it is absent from `ea sandboxes list`, and leave no pre-existing resource changed.

## Evidence — live reproduction 2026-09-15 (t001)

Pinned launcher built from `lego/cli` at this checkout (`bex vdev`, compatible with Render CLI v2.27.0), isolated `BEX_CLI_CONFIG_PATH` (0600), `BEX_HOST=https://api.bex.co/v1/`, human device login approved in the QA browser session, workspace `bex` (`tea-d98210cbbpdc73dcrkvg`). Production at that moment still ran the pre-m147 image (deploy run 34946928270 for `00e52317a` was in progress). Disposable sandbox created by this run: `e19a0152-8155-47a1-8ee7-f268e5965045` (starter, 3600 s timeout, 08:40:55 UTC); the workspace's one pre-existing sandbox was not touched.

**Defect A (live):** `bex ea sandboxes exec e19a0152-… -- true` → exit 1 in 0.71 s, empty stdout, stderr `Error: received response code 404: 404 page not found`. Raw replay of the client's first request with the same bearer (`User-Agent: render-cli/2.27.0`): `POST /v1/sandboxes/e19a0152-…/runs/stream/token?ownerId=tea-d98210cbbpdc73dcrkvg` body `{"command":"true"}` → `HTTP/2 404`, `content-type: text/plain`, body `404 page not found` — Go's mux default, i.e. the route did not exist.

**Defect B (live):** legacy `POST /v1/sandboxes/e19a0152-…/exec?ownerId=…` with `{"command":"sh -c 'echo out; echo err >&2; exit 7'"}` → `HTTP/2 200 text/event-stream`, body exactly `event: output` `{"stream":"stderr","data":"err\n"}`, `event: output` `{"stream":"stdout","data":"out\n"}`, `event: exit` `{"exitCode":7}`. Feeding that exit shape to the pinned `pkg/sandbox.Repo.ExecSandboxStream` (ad-hoc harness over the real module, not committed) returns `exit=0 err=<nil>`; feeding the gateway's pre-m147 error shape `{"error":"sandbox is no longer running","code":"sandbox_terminated"}` returns `sandbox exec stream error status 0: ` — the failing command reports success and the error message is empty, as filed.

## Evidence — post-fix, live 2026-09-15 (t010)

Deploy: run 34946928270 (the m147 commit) was superseded and did not roll bex; run 34950840699 (`1cb1f2d27dda`, contains `00e52317a`) built green, failed once at the GitOps write-back push (`remote: fatal error in commit_refs`), and succeeded on `gh run rerun --failed`: `deployment "bex-api" successfully rolled out` 10:01:42 UTC; `deploy/gitops/base/bex.yaml` pinned by `1df779f8a`. Same launcher, config, identity, and workspace as the reproduction. Disposable sandbox `0538f60e-fe13-47f3-b342-1563a0686497` (starter, created 10:02:57 UTC).

| Check | Result |
| --- | --- |
| `bex ea sandboxes exec 0538f60e-… -- sh -c 'echo out; echo err >&2; exit 7'` | stdout `out`, stderr `err`, **exit 7**, 1.14 s |
| `… exec 0538f60e-… -- true` | exit 0, no output |
| `… exec 00000000-0000-4000-8000-000000000000 -- true` | exit 1, `Error: received response code 404 (SANDBOX_NOT_FOUND): sandbox not found` |
| `… exec <terminated id> -- true` (after stop) | exit 1, same non-empty `SANDBOX_NOT_FOUND` message |
| Raw `POST /v1/sandboxes/0538f60e-…/runs/stream/token?ownerId=tea-d982…` `{"command":"sh -c 'exit 3'"}` | `201 application/json` `{"executionId":"exe-dakhfsfqniac73emh0v0","expiresAt":"2026-09-15T10:04:29Z","method":"POST","uri":"https://api.bex.co/v1/sandboxes/0538f60e-…/runs/exe-dakhfsfqniac73emh0v0/stream","token":<391 chars, redacted>}` — 60 s lifetime |
| Redeem at that `uri` with the token as Bearer | `200 text/event-stream`, body `event: exit` / `data: {"exit_code":3,"exitCode":3}` |
| Same token again | `409` `sandbox run connect token already used; mint a new one` |
| Fresh token, body `{"command":"id"}` (minted for `true`) | `403` `command does not match the one this connect token was minted for` |
| Fresh token on the pre-existing sandbox's path | `403` `connect token was not minted for this sandbox run` |
| OAuth access token at the redeem `uri` | `401` `invalid sandbox run connect token` |
| No bearer at the redeem `uri` | `401` `a sandbox run connect token is required as the Bearer credential` |
| Connect token at gated `POST /exec` and `GET /v1/sandboxes` | `401` `unauthorized` (both) |
| The misused fresh token, redeemed legitimately afterwards | `200`, `{"exit_code":0,"exitCode":0}` — misuse did not consume it |
| Mint for an unknown id | `404` `SANDBOX_NOT_FOUND` `sandbox not found` (the same body the exec verb returns) |
| Mint with `{operation}=attach` | `400` `unsupported run operation "attach" (only "stream" is supported)` |

Cleanup: `ea sandboxes stop 0538f60e-…` at 10:03:51 UTC; `ea sandboxes list --all` afterwards shows only the pre-existing `271ec9ce-…` (`running`, untouched throughout). The isolated device session was logged out (`bex logout`), the 0600 config and header files removed, and the QA browser cookies cleared. MCP `sandbox_exec` and the agent-session readers were not exercised live; they share `bufferExecWithLimit`, covered by the transitional-shape tests.

## Evidence (static; filed 2026-09-14)

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

## Shipped 2026-09-15 (t002–t009)

- **Routes:** gated `POST /v1/sandboxes/{id}/runs/{operation}/token` mints Render's `SandboxConnectResponse`; outside-gate `POST /v1/sandboxes/{id}/runs/{executionId}/stream` redeems it (single-use, ≤60 s, bound to workspace + sandbox + execution + command; IP limiter shared with deploy hooks; exec gate re-applied under the minter). Legacy `POST /exec` unchanged.
- **SSE:** gateway emits `exit_code` and `{status,message}` (404/403/503) beside the internal keys for one release; bex-api readers decode both.
- **Contract:** `lego/cli/testdata/sandbox-exec-contract.json` is driven from the pinned client (`lego/cli/sandbox_contract_test.go`) and proven from the real handlers (`lego/backend/internal/sandbox/connect_test.go`); three mutation spot-checks red.
- **Caller census:** 2 streaming callers (legacy REST, redeem), 1 `ExecBuffered` (MCP), 1 `systemBufferedExec` (scrub), 2 direct `bufferExec` (session status, hibernate) — all on the shared decoder.
- **Docs:** ledger rows (exec re-graded; `copy` and `sandbox-groups list` graded `[ ]`, snapshots `[-]`), `ea-sandbox.md`, `UPSTREAM_RENDER_CLI.md` step 5, `internal/api/CLAUDE.md` inventory. Gap candidate `w7/046`.
- **Live DoD (t001/t010):** verified 2026-09-15, see § Evidence — post-fix.

## Source + Goal linkage

- **Source:** continuous `/qa-find-bugs-cli` for w7, 2026-09-14 (sweep 2, static research while live auth was blocked). Checkout `0442e9063`; launcher build `bex vdev-34d0fa153` (Render v2.27.0). Deduped against open and done `.pm/` across all workstreams, w4 included: no open item covers the `runs/{operation}/token` handshake, `exit_code`, or the error-event shape. `w3/m33` (done) is the original exec work whose deferred t002 this revives.
- **Goal linkage:** Render-CLI compatibility of the imported launcher (ADR006, ADR018, `docs/bex-cli.md`) and pillar-5 sandboxes, reopened 2026-07-27 ([ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md), [ADR014](../../../docs/ADR014-sandboxes.md)). The ledger grades `ea sandbox exec` as `[x]`.
- **Expected outcome:** the only CLI path to run a command in a bex sandbox works with the shipped launcher, and scripts and agents can trust its exit status.
- **Why now:** the launcher is distributed (install script, Homebrew tap) with this pin, so every current user hits Defect A. Defect B silently turns failures into successes for anything that gates on exit status. Both are cheap to prove once auth is available, and the ledger currently certifies the broken behavior.
- **Render parity included:** the change adds tenant-facing REST routes and changes the SSE payload that REST and MCP consumers read. Shared-exec blast-radius verification is its own task (t005).
- **Out of scope:** implementing `ea sandboxes copy` (`/files/{operation}/token`), `sandbox-groups`, or snapshots. t006 grades them and files any follow-up. Ephemeral SSH and hosted-shell non-goals in `.pm/DO_NOT_DO.md` are untouched.
