# ADR114 — DeepSeek Harness as a Cloud Agent profile (`deepseek`)

**Status:** accepted (2026-11-15). User request, 2026-11-15: add DeepSeek Harness's ACP mode as a cloud agent, like Muse ([ADR108](ADR108-muse-code-agent.md)), on DeepSeek's latest Flash model through OpenRouter.

## Context

DeepSeek Harness (`dsh`, [`@deepseek-ai/dsh`](https://github.com/deepseek-ai/deepseek-harness), MIT) is DeepSeek's open-source agent harness. It ships its own Agent Client Protocol server: `dsh --profile acp` (the `acp` app bundle, `@deepseek-ai/dsh-acp`). Its model layer is pi-ai, whose catalog has an `openrouter` route. That route lists `~deepseek/deepseek-flash-latest`, OpenRouter's alias that always points at the newest DeepSeek Flash model (V4.1 Flash `1017` on 2026-11-15). The alias speaks OpenAI Chat Completions with OpenRouter's reasoning format.

[ADR047](ADR047-cloud-coding-agent-sessions.md) D7 makes an agent a reviewed profile in `lego/backend/internal/agentsession/agent-profiles.json`. [ADR090](ADR090-agent-owned-delivery.md)'s platform OpenRouter route already serves the claude profile of every workspace without its own key (w10/m14 t007) with a bex-owned OpenRouter key (`BEX_AGENT_PLATFORM_MODEL_KEY`), pinned to `deepseek/deepseek-v4.1-flash-1017`.

### Probe (2026-11-15)

A container matching the agent image ran `@deepseek-ai/dsh@0.2.2-rc.3` as uid 10001, without network: `node:22.23.4-bookworm-slim` with `/opt/node24/bin/node` copied from `node:24.22.2-bookworm-slim`, and dsh installed with `npm install --global --ignore-scripts`. It was launched exactly as the driver launches it and driven over ACP by a probe client. A stub OpenRouter served Chat Completions: a `bash` tool call first, then a reply.

| Question | Result |
| --- | --- |
| Model traffic | Every request was `POST <baseURL>/chat/completions` with `Authorization: Bearer <placeholder>`, `model: ~deepseek/deepseek-flash-latest`, `stream: true`, `stream_options.include_usage`, `reasoning.effort: high` and the tools. The body had no `provider` object, so the gateway's model pin and data policy apply unchanged. No other request was made. |
| Startup | `initialize` 0.9 s, `session/new` < 0.1 s. A two-request stub turn with one shell call took 0.1 s. |
| ACP surface | `mcpCapabilities.http`, image prompts, `session/close`/`list`/`resume`. No `loadSession`, no `systemPrompt`. `authMethods` is empty. Config options are `model` (only the pinned one listed) and `reasoning_effort` (`high`). |
| Tools | dsh's own (`bash`, `read`, `edit`, `write`, `grep`, `glob`, subagents, todo, jobs). The shell ran in the session's `cwd` (`/workspace`) as uid 10001, and no `session/request_permission` reached the client. Reasoning arrived as `agent_thought_chunk`. |
| A repository `.env` | dsh loads its launch directory's `.env` and refuses to start if that file sets a proxy or another bootstrap variable. Launched from an empty directory, with `HTTPS_PROXY` in `/workspace/.env`, it started normally. |
| Size | About 1.2 GB installed: around 600 packages, including its Web UI, a 146 MB LibreOffice WASM build and speech models. |

The real model and gVisor were not exercised. The live check is a session in the bex workspace after deploy.

## Decision

### D1 — Profile

- **Executable.** `deepseek` runs `/usr/local/bin/dsh` from `@deepseek-ai/dsh@0.2.2-rc.3` with `--profile acp`. dsh publishes only release candidates, so the manifest's exact-pin rule now admits an exact prerelease version (`x.y.z-rc.N`). A dist-tag is still refused.
- **Environment.** `DSH_PERMISSION_MODE=danger-full-access`, `DSH_TELEMETRY_DISABLED=1` and `SHELL=/bin/bash`.
- **Model route.** `OPENROUTER_BASE_URL` (suffix `/v1`) and `OPENROUTER_API_KEY`. The registered endpoint is `https://openrouter.ai/api`.
- **Session state.** `~/.dsh`.
- **Label.** "DeepSeek Harness".

### D2 — Node 24 for dsh only

dsh requires Node ≥ 24.9; the image's Node is 22, and an agent's own shell commands (customer builds and tests) run on it.

- **Runtime.** A new optional manifest field, `nodeRuntime`, names the Node binary that runs the executable. The image copies it from a pinned `node:24.22.2-bookworm-slim` stage to `/opt/node24/bin/node`, on the same Debian release and so the same glibc and libstdc++. The driver spawns `<nodeRuntime> <executable> <args>`.
- **Build check.** `install-profiles` checks that the runtime is root-owned and meets the package's `engines.node` floor.
- **Install.** dsh installs with the other adapters under `--ignore-scripts`. On Linux its install scripts only `chmod` a macOS helper and print messages; native parts (node-pty, koffi, ripgrep) ship as N-API prebuilds or per-platform optional packages.

### D3 — Model

The profile declares `model: ~deepseek/deepseek-flash-latest`. Before each start the driver writes a launch overlay that dsh applies after its profile layer (`--patch $DSH_HOME/bex-acp.patch.yml`) and passes it on the command line. The overlay:

- points pi-ai's `openrouter` route at the gateway model proxy (`baseURL`, `apiKeyEnv: OPENROUTER_API_KEY`, `api: openai-completions`, reasoning `high`), with that model only;
- selects it for the `acp` app;
- turns off `tool-web` (its search needs DeepSeek's own API) and DeepSeek's session-log upload.

The driver starts dsh from an empty directory (`$DSH_HOME/bex-launch`, emptied on every start because `~/.dsh` outlives a turn), so neither a repository's `.env` nor one the agent wrote there changes the agent's environment or stops it. The workspace is still the session's `cwd`.

The manifest's model id rule is now bex-api's route model id rule, which admits OpenRouter's `provider/model` ids and `~` aliases.

### D4 — Platform route only, on the platform OpenRouter key

The profile is `platformOnly` ([ADR108](ADR108-muse-code-agent.md) D5), generalized by provider: a platform-only profile runs on the bex-owned key of its endpoint's provider. That is the Meta key for `api.meta.ai` and the platform OpenRouter route's key for `openrouter.ai`. Both routes cover every workspace (w10/m14 t007).

- **Mint.** It hands the gateway the platform key for `openrouter.ai` under the `/api` prefix, pinned to the profile's own model rather than the route's claude model. The workspace's own key is never read for it.
- **Billing.** Sessions are priced from OpenRouter's reported cost, spend-capped and counted toward the trust tier like the claude route ([ADR091](ADR091-agent-model-pricing.md)).
- **Availability.** Without the route the profile is offered nowhere: capabilities omit it, and a create is refused with `AGENT_SESSION_AGENT_UNAVAILABLE`.
- **No session cap.** OpenRouter's account balance is the global backstop (ADR091 D5), so unlike Muse there is no live-session cap.

### D5 — Gateway

- **Admitted operation.** On `openrouter.ai` the model proxy also admits `POST /v1/chat/completions` (no query). It is relayed to `https://openrouter.ai/api/v1/chat/completions`.
- **Metering.** That path is metered like Messages: the usage tee reads the Chat Completions usage on the stream's final chunk, or on a JSON response. `prompt_tokens` includes the cached and cache-written tokens it details, and `cost` is OpenRouter's.
- **Data policy.** The request carries the platform data policy (`provider.data_collection: deny`), so the terms' routing policy at `/legal/agent-models` covers DeepSeek sessions without amendment.
- **No egress change.** `openrouter.ai` is already a registered model provider.

### D6 — Instructions and continuity

dsh has no ACP `systemPrompt`, so the bex overlay goes in the first-prompt preamble, as for Gemini and Muse. It has no `session/load`, so a follow-up turn primes from the durable transcript ([ADR047](ADR047-cloud-coding-agent-sessions.md) D3).

### D7 — Availability in a workspace

DeepSeek sessions go through OpenRouter under the same no-training routing as the claude route, so the profile needs no data-use acknowledgement. It is on by default where the route serves it, and admins can turn it off like any agent (w7/m392).

## Consequences

- **Image size.** The agent image grows by about 1.2 GB uncompressed. Installing the ACP bundle alone (`@deepseek-ai/dsh-acp-app` and its base) instead of the whole CLI would cut most of it. That is a follow-up.
- **Preview software.** dsh is a developer preview; its SAFETY.md says it is not security-audited. bex's gVisor sandbox, session egress policy and gateway-held credentials remain the boundary, as for every profile.
- **Unpinned transitive dependencies.** They resolve at build time (pi-ai `^1.3.0` installs 1.5.x), as for the other npm adapters. A version bump is a reviewed manifest change.
- **Coarse streaming.** dsh's ACP server sends finished messages, not token deltas, so the transcript streams coarser than Claude Code's.
- **Open:** the live check of a session with the real model under gVisor.
