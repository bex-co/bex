# Render CLI `ea sandbox` wire shape (w3/m32 t008)

Captured from the official `render-oss/cli` source and verified with v2.21.0, so bex-api's `/v1/sandboxes*` adapter (m32/m33, ADR042 D2) matches the real wire behavior and `render ea sandbox create/exec/list/stop` works unmodified. Rows 238–241 of [cli-compatibility-checklist.md](../cli-compatibility-checklist.md) are green.

All requests carry the workspace as an `ownerId` query parameter (Render's team id, `tea-…`); the server base path is `/v1/`.

## Endpoints

| CLI command | Method + path | Request | Response |
| --- | --- | --- | --- |
| `ea sandbox create` | `POST /v1/sandboxes?ownerId=` | `{ "plan": <plan>, "image"?: <ref>, "networkPolicy"?: {…} }` | `Sandbox` |
| `ea sandbox list` | `GET /v1/sandboxes?ownerId=&cursor=&status=` | — (repeat `status=` per filter; `--all` sends every status) | `[{ sandbox, cursor }]` (cursor pagination) |
| (get by id) | `GET /v1/sandboxes/{sandboxId}?ownerId=` | — | `Sandbox` |
| `ea sandbox exec` | `POST /v1/sandboxes/{sandboxId}/exec?ownerId=` | `{ "command": "…" }` | SSE `output` events followed by `exit` with `exitCode` |
| `ea sandbox stop` | `POST /v1/sandboxes/{sandboxId}/terminate?ownerId=` | — | 200/204 |
| (logs) | `GET /v1/sandboxes/{sandboxId}/logs?ownerId=` | — | log events |
| (files) | `GET /v1/sandboxes/{sandboxId}/files/list`, `.../files/{path}/token` | — | file entries / token |

The real CLI flow is one POST whose response body is the SSE stream. Each `event: output` carries `{"stream":"stdout|stderr","data":"…"}` and the terminal `event: exit` carries `{"exitCode":N}`; a missing exit event is a CLI error. bex-api performs workspace plus durable owner/admin authorization, signs a short-lived single-use ticket binding the exact command and sandbox namespace, and reverse-proxies the isolated ssh-gateway's stream. Only that gateway holds namespace-scoped `pods/exec`. MCP `sandbox_exec` consumes the same SSE path and returns buffered stdout/stderr/exit code.

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
