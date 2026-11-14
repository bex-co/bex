# ADR108 — Muse Code as a Cloud Agent profile (`muse`)

**Status:** accepted (2026-11-13, w7/m390). The model route and billing are w7/m391; admin enablement is w7/m392.

## Context

Meta's Muse Code is a coding agent CLI (`muse`, [docs](https://dev.meta.ai/docs/muse-code/)). The bex-co organization maintains an unofficial Agent Client Protocol adapter for it, [`@bex-co/muse-code-acp`](https://github.com/bex-co/muse-code-acp) (Apache-2.0, derived from claude-agent-acp). [ADR047](ADR047-cloud-coding-agent-sessions.md) D7 makes an agent a reviewed profile in `lego/backend/internal/agentsession/agent-profiles.json`: an executable, its arguments and bootstrap environment, and how the driver points it at the gateway model proxy ([ADR062](ADR062-sandbox-credential-vault.md)).

User decisions, 2026-11-13: Muse sessions use **Meta's official API** (`api.meta.ai`), not OpenRouter, and **only Muse Spark 1.3 on the contributor tier** (`muse-spark-1.3-contributor`). Meta's [pricing page](https://dev.meta.ai/docs/pricing-rate-limits) lists that model at $0.10 input, $0.002 cached input and $0.20 output per million tokens, 150 requests and 3,000,000 tokens per minute per key, and states that on this tier Meta may use "the prompts, code, files, and context you send, plus the model responses" to train future models.

### Probe (2026-11-13)

Muse 1.4.14-R7309.1 (`muse-aarch64-linux`, SHA-256 checked against Meta's release manifest) and adapter 0.11.0 ran in a `node:22.23.4-bookworm-slim` container as uid 10001, the agent image's base and user, driven over ACP by a probe client. A stub Meta Responses server stood in for the model.

| Question | Result |
| --- | --- |
| Model traffic | With `MUSE_CODE_ACP_GATEWAY_URL=<proxy>/v1` and `MUSE_CODE_ACP_GATEWAY_KEY=<placeholder>`, every model request was `POST /v1/responses` (`application/json`, `stream: true`, `store: false`) with `Authorization: Bearer <placeholder>`. No other path was requested. |
| Model id | `model` in `~/.config/muse/settings.json` is honored: all requests asked for `muse-spark-1.3-contributor`. Without it Muse asks for `muse-spark-1.2`. |
| Requests per turn | A one-line reply took 5–7 requests: the answer plus Muse's built-in "goal" and "verify" reminder checks. |
| Startup | `initialize` 0.4–1.6 s, `session/new` < 0.1 s, a stub turn 1.7–3.5 s. |
| Muse's own sandbox | With the default (`sandbox: enabled`) every shell call failed: `bwrap: Can't mount proc on /newroot/proc: Operation not permitted`. With `sandbox: disabled` (needs `MUSE_CODE_ACP_ALLOW_YOLO=1`), `nativeApprovalPolicy: allowAll` and mode `bypassApprovals`, shell calls ran and no permission request reached the client. |
| Other hosts | Muse also opens a connection to `api.meta.ai` directly, outside the configured endpoint. With no network, or with `api.meta.ai` resolving to a black-holed address, the turn completed with no added latency. |
| Session state | `~/.local/share/muse/` (sessions, `session-index.db`, local tracing) and `~/.local/share/muse-code-acp/sessions`; settings in `~/.config/muse/`. |
| ACP surface | `loadSession`, `session/list`/`close`/`resume`/`fork`, stdio and HTTP MCP, images. No `sessionCapabilities.systemPrompt`. One auth method, `meta-api-key`; the client never has to call `authenticate` when the gateway pair is set. |

The real contributor model was not called: no Meta key was available. Requests and tokens per real turn are measured in w7/m391 t005.

## Decision

### D1 — Profile

`muse` runs `/usr/local/bin/muse-code-acp` from `@bex-co/muse-code-acp@0.11.0`. Its environment sets `MUSE_CODE_EXECUTABLE` to the pinned binary and `MUSE_CODE_ACP_ALLOW_YOLO=1`. The model proxy route is `MUSE_CODE_ACP_GATEWAY_URL` (suffix `/v1`) and `MUSE_CODE_ACP_GATEWAY_KEY`; the registered endpoint is `https://api.meta.ai/v1`. The label is "Muse Code".

### D2 — Binary supply

Meta ships `muse` as a static native binary, not an npm package, so the manifest gains an optional `nativeBinary {version, path, artifacts: {amd64|arm64: {url, sha256}}}`. The image's `native-binaries` stage downloads the artifact for the target architecture (production builds `linux/amd64`; local dev builds are `arm64`) and refuses it unless the SHA-256 matches; `install-profiles` checks the installed file again, that it is root-owned, and that `<path> --version` reports `version`. It lives read-only at `/opt/muse/muse`: the sandbox runs as uid 10001 and cannot replace it. The pin is 1.4.14-R7309.1, the newest host the adapter's CI verifies, with checksums from Meta's release manifest. It adds about 415 MB to the image. Trivy finds no package inventory in the binary, so the image CVE gate neither covers nor blocks it; the pin and checksum are the control, and a version bump is a reviewed manifest change.

### D3 — Muse's sandbox is off; bex's is the boundary

The agent already runs inside bex's gVisor sandbox with session egress policy, and every profile runs under `sandbox-auto-approve` (DO_NOT_DO `#APPROVAL-MODE`: no approval prompts reach people). Muse's bwrap sandbox cannot start in a container anyway. After `session/new` and `session/load` the driver sets the ACP session config options `sandbox=disabled`, `nativeApprovalPolicy=allowAll` and `mode=bypassApprovals`, declared in the profile as `sessionConfig`. `allowAll` also skips Muse's approval-review model calls.

### D4 — Model

The profile declares `model: muse-spark-1.3-contributor`, the only model Muse sessions use. The driver writes it to `~/.config/muse/settings.json` before the adapter starts. w7/m391 pins the same id at the gateway.

### D5 — Platform route only

The profile is `platformOnly`: it never runs on a workspace's own model key. Until a bex-owned route serves it in a workspace (w7/m391), `muse` is absent from that workspace's capabilities and a create is refused with `AGENT_SESSION_AGENT_UNAVAILABLE`, and the model credential mint refuses it rather than reading the workspace key.

### D6 — Egress

Only the gateway model proxy carries Muse's model traffic. `api.meta.ai` stays out of the tenant pod's egress: the direct connection Muse makes on its own is not needed for a turn. The proxy admits only `POST /v1/responses` for `api.meta.ai`.

### D7 — Instructions

The adapter has no `systemPrompt` capability, so the bex overlay goes in the first-prompt preamble, as for Gemini. If live turns show Muse ignoring the delivery contract that way, the fix is an adapter release that advertises `systemPrompt`.

## Consequences

- Meta trains on everything a Muse session sends. w7/m391 amends [ADR091](ADR091-agent-model-pricing.md) D8 and the terms page; w7/m392 keeps Muse off until a workspace admin acknowledges it.
- One bex key carries every workspace's Muse traffic within 150 requests per minute; w7/m391 caps concurrent Muse sessions.
- **Open:** Meta's terms for running Muse Code on behalf of other people's code in a hosted service were not found. This needs a check before Muse is enabled outside first-party workspaces.
