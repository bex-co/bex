# Render CLI `ea sandbox` wire shape (w3/m32 t008)

Captured from the official `render-oss/cli` source (the generated OpenAPI client `pkg/client/client_gen.go` + `pkg/client/sandboxes` + `pkg/sandbox/repo.go`), so bex-api's `/v1/sandboxes*` adapter (m32 t009, ADR042 D2) matches it and `render ea sandbox create/exec/list/stop` works unmodified. Rows 238–241 of [cli-compatibility-checklist.md](../cli-compatibility-checklist.md) (today 404).

All requests carry the workspace as an `ownerId` query parameter (Render's team id, `tea-…`); the server base path is `/v1/`.

## Endpoints

| CLI command | Method + path | Request | Response |
| --- | --- | --- | --- |
| `ea sandbox create` | `POST /v1/sandboxes?ownerId=` | `{ "plan": <plan>, "image"?: <ref>, "networkPolicy"?: {…} }` | `Sandbox` |
| `ea sandbox list` | `GET /v1/sandboxes?ownerId=&cursor=&status=` | — (repeat `status=` per filter; `--all` sends every status) | `[{ sandbox, cursor }]` (cursor pagination) |
| (get by id) | `GET /v1/sandboxes/{sandboxId}?ownerId=` | — | `Sandbox` |
| `ea sandbox exec` | `POST /v1/sandboxes/{sandboxId}/runs/{operation}/token?ownerId=` | `operation` = `stream`; body carries the command | `{ method, uri, token }` → open `uri` with `token` for the SSE exec stream |
| `ea sandbox stop` | `POST /v1/sandboxes/{sandboxId}/terminate?ownerId=` | — | 200/204 |
| (logs) | `GET /v1/sandboxes/{sandboxId}/logs?ownerId=` | — | log events |
| (files) | `GET /v1/sandboxes/{sandboxId}/files/list`, `.../files/{path}/token` | — | file entries / token |

The exec flow is two-step: `exec` first POSTs to `…/runs/stream/token` to obtain `{ method, uri, token }` (a `SandboxConnectResponse`), then issues that method+uri with the token to stream stdout/stderr as SSE (`ExecSandboxStream` in `pkg/sandbox/repo.go`). bex-api's MCP `sandbox_exec` can surface the same run over OpenSandbox `execd` without the two-step token dance (agents drive exec via MCP, ADR014 D3); the REST two-step is what the unmodified CLI needs.

## Models

**Sandbox** — `{ id: <SandboxId>, plan: <SandboxPlan>, networkPolicy: <SandboxNetworkPolicy>, status: <SandboxStatus> }` (plus timestamps).

**SandboxPlan** (compute size; matches Workflow plans of the same name): `starter` · `standard` · `pro`.

**SandboxStatus**: `creating` · `running` · `suspended` · `resuming` · `errored` · `terminated`.

Note: Render's sandbox lifecycle has a `suspended`/`resuming` pair (pause/resume) distinct from `terminated`. bex maps `suspended` ⇄ OpenSandbox pause (rootfs-only on the k8s substrate, ADR042 D5) and `terminated` ⇄ delete.

## Divergences bex must decide (for t009)

- **`plan` → tier**: Render's `starter/standard/pro` must map onto bex's compute tiers (or a sandbox-specific sizing); the value is echoed back on `Sandbox`.
- **`networkPolicy`**: Render exposes a per-sandbox default network posture. bex's sandbox is sealed by the `<ws>-sandbox` boundary (ADR042 D3 / ADR043 D2), so the field is accepted and reflected but the effective policy is the namespace's.
- **`image`**: bex's `Create` is template-based (ADR014 D2 / ADR042 D2) — the image is fixed at template registration, not taken from an arbitrary request ref.
