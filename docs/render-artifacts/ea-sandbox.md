# Render CLI `ea sandbox` wire shape (w3/m32 t008)

Captured from the official `render-oss/cli` source and verified with v2.21.0, so bex-api's `/v1/sandboxes*` adapter (m32/m33, ADR042 D2) matches the real wire behavior and `render ea sandbox create/exec/list/stop` works unmodified. Rows 238–241 of [cli-compatibility-checklist.md](../cli-compatibility-checklist.md) are green.

All requests carry the workspace as an `ownerId` query parameter (Render's team id, `tea-…`); the server base path is `/v1/`.

## Endpoints

| CLI command | Method + path | Request | Response |
| --- | --- | --- | --- |
| `ea sandbox create` | `POST /v1/sandboxes?ownerId=` | `{ "plan": <plan>, "image"?: <ref>, "networkPolicy"?: {…} }` | `Sandbox` |
| `ea sandbox list` | `GET /v1/sandboxes?ownerId=&cursor=&status=` | — (repeat `status=` per filter; `--all` sends every status) | `[{ sandbox, cursor }]` (cursor pagination) |
| (get by id) | `GET /v1/sandboxes/{sandboxId}?ownerId=` | — | `Sandbox` |
| `ea sandbox exec` (v2.24.0+, step 1) | `POST /v1/sandboxes/{sandboxId}/runs/{operation}/token?ownerId=` (`operation` = `stream`) | `{ "command": "…" }` | `201` `SandboxConnectResponse` `{ executionId: "exe-…", expiresAt, method: "POST", token, uri }` |
| `ea sandbox exec` (v2.24.0+, step 2) | `POST <uri>` = `/v1/sandboxes/{sandboxId}/runs/{executionId}/stream`, `Authorization: Bearer <token>`, `Accept: text/event-stream` | `{ "command": "…" }` (must equal the minted command) | `200` SSE `output` events followed by `exit` with `exit_code`, or `error` with `{status, message}` |
| `ea sandbox exec` (≤ v2.21.0, legacy) | `POST /v1/sandboxes/{sandboxId}/exec?ownerId=` | `{ "command": "…" }` | same SSE stream, single step |
| `ea sandbox stop` | `POST /v1/sandboxes/{sandboxId}/terminate?ownerId=` | — | 200/204 |
| (logs) | `GET /v1/sandboxes/{sandboxId}/logs?ownerId=` | — | log events |
| (files, `ea sandboxes copy`) | `POST /v1/sandboxes/{sandboxId}/files/{operation}/token` (`upload`/`download`) then the returned `uri` | **none** — `ownerId` and `path` are query parameters (corrected w7/m150 t001; the mint sends no body) | connect token / raw body for a single file, tar+gzip for a directory — **not served by bex** (`404`; ledger gap) |

Since v2.24.0 the CLI flow is two requests (`pkg/sandbox/repo.go` `connect` + `ExecSandboxStream`): the mint returns a connect token and the `uri` to redeem it at, and the redeem's response body is the SSE stream. The connect token is not an OAuth credential — bex signs it with a key derived from the gateway exec secret, binds it to the workspace, sandbox, `executionId`, operation, and the exact command bytes, expires it within 60 s, and consumes it once across replicas — so the redeem route is served outside the OAuth gate (classified in the always-public inventory) and re-applies the exec gate under the token's minter at redeem time. Refusals there are JSON `{"message"}` bodies with a precise status: 401 not a connect token, 403 wrong sandbox/execution/command, 409 already used, 410 expired, 404 `SANDBOX_NOT_FOUND` if the sandbox vanished, 429 from the IP limiter. Each `event: output` carries `{"stream":"stdout|stderr","data":"…"}`; the terminal `event: exit` carries `{"exit_code":N}` (the key `pkg/sandbox/sse.go` has decoded at every pin — bex emitted `exitCode` before w7/m147, which the CLI read as 0), and `event: error` carries `{"status":N,"message":"…"}` (404 terminated target, 403 access revoked mid-stream, 503 exec failed to start), which the CLI prints as `sandbox exec stream error status N: message`. bex adds the internal `exitCode`/`error`/`code` keys beside them for one release so a bex-api reader that rolls out apart from the gateway decodes either. A missing exit event is a CLI error. bex-api performs workspace plus durable owner/admin authorization, signs a short-lived single-use ticket binding the exact command and sandbox namespace, and reverse-proxies the isolated ssh-gateway's stream. Only that gateway holds namespace-scoped `pods/exec`. MCP `sandbox_exec` consumes the same SSE path and returns buffered stdout/stderr/exit code.

## Models

**Sandbox** — `{ id: <SandboxId>, plan: <SandboxPlan>, networkPolicy: <SandboxNetworkPolicy>, status: <SandboxStatus> }` (plus timestamps).

**SandboxPlan** (compute size; matches Workflow plans of the same name): `starter` · `standard` · `pro`.

**SandboxStatus**: `creating` · `running` · `suspended` · `resuming` · `errored` · `terminated`.

Note: Render's sandbox lifecycle has a `suspended`/`resuming` pair (pause/resume) distinct from `terminated`. bex maps `suspended` ⇄ OpenSandbox pause (rootfs-only on the k8s substrate, ADR042 D5) and `terminated` ⇄ delete.

## Deliberate bex divergences

- **`plan` → tier**: Render's `starter/standard/pro` must map onto bex's compute tiers (or a sandbox-specific sizing); the value is echoed back on `Sandbox`.
- **`networkPolicy`**: the field is no longer accepted-and-ignored. Omitted policy normalizes to `{default:"deny-all"}`; `deny-all` is the only accepted Render enum and round-trips from durable OpenSandbox metadata. `allow-all` and unknown values fail before any runtime call with `SANDBOX_NETWORK_POLICY_UNSUPPORTED` across REST, GraphQL, MCP, and the CLI. In bex, `deny-all` means default-deny plus a platform-managed HTTPS FQDN allowlist required for agent package/source/model access; arbitrary tenant-authored rules are unsupported. Cilium requires an approved exact SNI and explicitly denies private/rebound HTTPS targets. This narrower-than-Render posture is deliberate and enforced outside gVisor.
- **Visibility/ownership**: Render's `ownerId` is a team boundary. bex adds a per-caller boundary inside that workspace: ordinary members list/get/exec/stop only sandboxes whose reserved owner/workspace metadata matches their resolved identity and whose sandbox-regime plus enforced-policy stamps are intact; an explicit workspace admin may operate all fully hardened owned sandboxes in the workspace. Foreign, ownerless, and incompletely hardened objects are indistinguishable from missing ones (`SANDBOX_NOT_FOUND`).
- **`image`**: bex's `Create` is template-based (ADR014 D2 / ADR042 D2) — the image is fixed at template registration, not taken from an arbitrary request ref.
