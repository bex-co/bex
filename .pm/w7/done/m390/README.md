# w7 · m390 — Muse Code runs as a fourth Cloud Agent profile (`muse`) in the agent sandbox

**Worker:** worker7 **Goal:** Meta's Muse Code, driven by the bex-maintained ACP adapter `@bex-co/muse-code-acp`, runs inside the production gVisor agent sandbox as the `muse` profile, with its model traffic going only to the gateway model proxy and no real key in the pod. It is not offered to any workspace until m391 gives it a model route and m392 lets an admin enable it. **Status:** done (2026-11-14) (t001–t006, t008–t010 done; t007 waits for the production deploy, then t011)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Probe Muse Code in the production agent sandbox and write the decision record (new ADR) — **DONE** | 75m | — |
| t002 | Agent image: pinned native `muse` binary and a `nativeBinary` profile-manifest field — **DONE** | 60m | t001 |
| t003 | `muse` profile and backend contract, hidden until a route serves it — **DONE** | 45m | t002 |
| t004 | Gateway model proxy admits only `POST /v1/responses` on `api.meta.ai` — **DONE** | 40m | t003 |
| t005 | Driver bootstrap for `muse`: proxy routing, Muse sandbox off, overlay, `bex` MCP, failure mapping — **DONE** | 75m | t003 |
| t006 | Composer: a Muse Code agent option (en/zh), hidden where Muse is not offered — **DONE** | 30m | t003 |
| t007 | Production proof in the gVisor sandbox — **DONE** | 60m | t004, t005, t006, w7/m393/t002 |
| t008 | Render parity (Devin reference) — **DONE** | 30m | t007 |
| t009 | Simplify — **DONE** | 30m | t008 |
| t010 | Test coverage — **DONE** | 45m | t008 |
| t011 | Closeout — **DONE** | 10m | t009, t010 |

## Definition of done

- The deployed agent image carries the pinned `muse` binary (SHA-256 verified at build, no self-update) and `@bex-co/muse-code-acp@0.11.0` at the `muse` profile's executable; the image build fails if either is missing or at another version.
- In a production gVisor agent sandbox, the `muse` profile's adapter answers ACP `initialize` and `session/new` with Muse's own sandbox off, and its model request reaches the session's gateway model-proxy URL (never `api.meta.ai` directly). The pod spec and process environment hold only the session placeholder key.
- The gateway model proxy forwards only `POST /v1/responses` (JSON) for host `api.meta.ai` and refuses every other path, method and foreign Pod (tests).
- Driver tests pin the `muse` bootstrap environment, the overlay channel and the failure-to-failed-turn mapping.
- `muse` is in the profile manifest but appears in no workspace's capabilities projection or composer, and `createAgentSession` with `agent: muse` is refused with a typed error, until a route serves it (m391) and an admin enables it (m392).
- Backend real-DB suite, driver tests, `make lint`, dashboard `yarn lint` + `yarn test` pass, and the change is live on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm add meta muse coding agent with muse-code-acp to our cloud agents`, 2026-11-13, item 1, filed by `/pmx them all to w7`. User decisions the same day: use Meta's official API (`api.meta.ai`), not OpenRouter, and only Muse Spark 1.3 on the contributor tier (`muse-spark-1.3-contributor`). Evidence: [bex-co/muse-code-acp](https://github.com/bex-co/muse-code-acp) README and source (`src/client-provider.ts` `MUSE_CODE_ACP_GATEWAY_URL`/`_KEY`, `src/mcp-overlay.ts` `endpoint_transport {base_url, auth: "bearer"}`, `src/agent-capabilities.ts` — no `systemPrompt`, `loadSession` yes, `src/safety-settings.ts` sandbox-off needs `MUSE_CODE_ACP_ALLOW_YOLO=1`); its CI pins Meta's public `muse-x86-linux` artifacts by SHA-256 (`.github/workflows/ci.yml`); the adapter's loopback tests show Muse speaks `POST {base}/responses`. [Muse Code docs](https://dev.meta.ai/docs/muse-code/).
- **Goal linkage:** ADR008 pillar 5; [ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md) D7 — agent pluggability is a reviewed command/args/env profile in `lego/backend/internal/agentsession/agent-profiles.json`; ADR062's model proxy keeps the real key out of the sandbox.
- **Expected outcome:** a fourth, independently maintained coding agent can run in bex sandboxes under the same credential, egress and delivery contracts as Claude Code, Codex and Gemini.
- **Why now:** m391 (the Meta route) and m392 (admin enablement) build on this profile; the adapter is maintained in-house, so gaps can be fixed upstream quickly.
- **Render parity included** (Devin reference): Devin's web app has an agent selector next to the message box ([docs](https://docs.devin.ai/get-started/first-run.md)); the profile list is exposed through REST, GraphQL, MCP and the dashboard.
- **Recommendations recorded, not user sign-offs:** (1) use the bex-co adapter rather than another community adapter (e.g. `BrokkAi/muse-acp`); (2) turn Muse's own bwrap sandbox off inside bex's gVisor sandbox and auto-approve its permission requests, as the `sandbox-auto-approve` policy already does for every profile (DO_NOT_DO `#APPROVAL-MODE`); (3) the published Meta terms for running Muse Code on tenants' behalf in a hosted service were not found — the new ADR records this as open.
- **Interpretation recorded:** "contributor only" is read as: Muse never runs on a workspace's own model key, so no BYO path is added for `muse`.

## Progress (2026-11-14)

- **t001:** probe recorded in [ADR108](../../../../docs/ADR108-muse-code-agent.md): Muse 1.4.14 + adapter 0.11.0 in the agent image's base as uid 10001; only `POST /v1/responses` with the placeholder Bearer reaches the configured proxy URL; the `muse-spark-1.3-contributor` pin is honored from Muse's settings; Muse's bwrap sandbox fails in a container and runs with `sandbox=disabled`; the direct `api.meta.ai` connection Muse makes on its own is not needed for a turn. No Meta key was available, so requests and tokens per real turn move to w7/m391 t005.
- **t002–t006:** `nativeBinary` (per-architecture URL + SHA-256), `model`, `platformOnly` and `sessionConfig` in the profile manifest (Go + TS validators share `profiles-invalid-cases.json`); `native-pins` → `native-binaries` image stages and build-time checks in `install-profiles`; `muse` profile; capabilities hide and create refuses a platform-only profile with `AGENT_SESSION_AGENT_UNAVAILABLE` until a route serves it, and the mint never reads the workspace key for it; proxy admits only `POST /v1/responses` on `api.meta.ai`; driver writes the model to Muse's settings and sets the session config after `session/new`/`session/load`; composer and automation form list exactly the capabilities' agents. A local arm64 image build passed the fetch and build-time checks, and a headless muse turn through the real `bex-agent-driver` in that image succeeded against a stub proxy (`scripts/lib/verify-muse-profile.mjs`).
- **t008 (Render parity, Devin reference):** REST `GET /v1/agent-sessions/capabilities`, GraphQL `agentSessionCapabilities` and MCP `get_agent_session_capabilities` all project `Service.Capabilities`, and create on every surface runs `Service.Create`, so the muse filter and the typed refusal are identical across surfaces; no surface carries a fixed agent enum. Devin offers only the agents an admin made available ([docs](https://docs.devin.ai/get-started/first-run.md)); bex offers only the agents a workspace can run, and the admin control itself is w7/m392. No ADR018 row changes (agent sessions are a bex extension).
- **t009:** `/simplify` (reuse, quality, efficiency reviews) applied: streamed hashing of the binary, a `native-pins` stage so unrelated profile edits keep the download cached, retry only the transfer, `writeJsonAtomic` and `sha256Hex` reused, the dashboard trusts the capabilities list instead of a client-side platform-only set, a `model` on a profile the driver cannot pin it for is refused, and the live verifier drives the real driver instead of a copy.
- **t010:** Go (manifest contract, invalid cases, Meta scheme, mint refusal with a mutation check, capabilities/create refusal, proxy boundary), driver (model settings merge, session-config order and refusal, release profile load, model scope) and dashboard (agent list from capabilities, Muse create payload) tests.
- **t007 (2026-11-14):** m390 is live (deploy run 43566568833, 706a389d5). The dispatched isolation matrix (`verify_agent_driver=true`) first failed on a stale `oregon` assertion left by w32/016 (fixed in 430ba206f), then at "agent sandbox never reached Ready": the public `template: agent` sandbox's driver exits 1 because it has required session env (`BEX_AGENT_MODEL_PROXY_URL`) since 7a2303ffc, and the public create refuses `BEX_*` names (diagnostics from 30563ff17, run 43570243129). Real sessions are unaffected. The leg is rebuilt around a chat-only session in w7/m393; t007 records its production run. Run 43579027377 (2026-11-14, production, image `bex-agent-sandbox@sha256:100cf65d…` from deploy run 43566568833): the muse leg (`scripts/lib/verify-muse-profile.mjs`) ran the real `bex-agent-driver` headless with the `muse` profile inside a gVisor agent-session sandbox and passed ("muse profile runs in the agent sandbox with model traffic on the proxy only"), with every other agent-leg check and the full matrix green.
- **t011:** closed 2026-11-14.
