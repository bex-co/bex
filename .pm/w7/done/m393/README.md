# w7 · m393 — The isolation matrix's agent leg starts its sandbox through a real chat-only agent session

**Worker:** worker7 **Goal:** `isolation-matrix.yml` with `verify_agent_driver=true` runs every agent-image check on production again, against a sandbox started the way production starts one: a chat-only agent session in the disposable workspace. **Status:** done (2026-11-14)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verifier: the agent leg's sandbox comes from a chat-only agent session with a placeholder workspace model key — **DONE** | 90m | — |
| t002 | Production run: every agent-leg check passes, the muse profile included — **DONE** | 45m | t001 |
| t003 | Simplify — **DONE** | 30m | t002 |
| t004 | Test coverage — **DONE** | 30m | t002 |
| t005 | Closeout — **DONE** | 10m | t003, t004 |

## Definition of done

- `scripts/verify-sandbox-isolation.sh` with `BEX_VERIFY_AGENT_DRIVER=1` no longer creates a public `template: agent` sandbox. It sets a placeholder model key on disposable workspace A, starts a chat-only session there, waits for that session's BatchSandbox pod to be Ready, runs the agent leg against it (driver healthy on loopback, peer sandbox denied on 8787, gateway-only identity, standalone real-key refusal, the muse profile) and stops the session in cleanup.
- A dispatched `isolation-matrix.yml -f verify_agent_driver=true` run on production passes, and its agent-leg PASS lines are recorded here.

## Source + Goal linkage

- **Source:** found by w7/m390 t007, 2026-11-14. Run 43570243129 (with the readiness diagnostics added in 30563ff17) showed the agent sandbox's `sandbox` container exiting with code 1 right after start; the same image run locally exits with `BEX_AGENT_MODEL_PROXY_URL is required: the driver only runs behind the bex model proxy`. The driver has required that variable, `BEX_AGENT_SESSION_ID` and the session's placeholder key since 7a2303ffc (2026-10-22), and the public sandbox create refuses every `BEX_*` env name (`reservedSandboxEnvPrefixes`, `lego/backend/internal/sandbox/service.go`). Only `AgentSessionLifecycle.CreateAgentSessionSandbox`, reached by an agent session create, gives the pod that env. The agent leg last passed on 2026-08-03 (w3/m40) and could not have passed since 2026-10-22; before 370a8638b no CI run enabled it.
- **Goal linkage:** ADR042 / ADR047 (agent sandboxes keep the sandbox isolation posture), ADR108 (the muse profile's production proof, w7/m390 t007, depends on this leg).
- **Expected outcome:** the agent leg verifies a sandbox exactly as a user's session gets it, and m390 t007 can record its production proof.
- **Why now:** m390 cannot close without it, and every agent-image change since 2026-10-22 shipped without this leg.
- **Render parity omitted:** an internal CI verifier; no REST, GraphQL, MCP or UI change.
- **Design notes:** a chat-only session needs no repo, branch or GitHub connection (`validateCreate`); the driver stays up after a turn (`BEX_AGENT_EXIT_AFTER_TURN=0`). The placeholder key makes the first turn fail upstream, which the leg does not need. The public API accepting `template: agent` at all is filed separately as w7/063.

## Progress (2026-11-14)

- **t001:** `scripts/verify-sandbox-isolation.sh` with `BEX_VERIFY_AGENT_DRIVER=1` sets a per-run placeholder model key on workspace A, starts a chat-only `claude` session there, and finds its BatchSandbox by the `bex.co/agent-session` label (or the session's `sandboxId`) (b2bf0a46f). Two production findings shaped the rest. A failed session skips the idle grace and its sandbox is reclaimed at once (w5/m80), so the placeholder turn lost the pod mid-leg (run 43571918197). The verifier now applies a CiliumNetworkPolicy `egressDeny` from the session's pods to `bex-system` before the pod starts, so the model request hangs and no placeholder-key request leaves the cluster (088114a70). Left running, that held turn's dropped retries pushed sandbox A's drops out of the node's Hubble buffer (run 43576889956 failed only the Hubble check; run 43577969524 without the agent leg passed it), so the leg cancels the session and lifts the hold after its last check (1a6220ce1). Readiness timeouts print the BatchSandbox status, pod conditions, container states and events (30563ff17).
- **t002:** run 43579027377 (`verify_agent_driver=true`, 1a6220ce1) passed end to end on production. Agent leg: peer sandbox to agent driver denied; gateway label spoof denied; gateway workload identity to the health and UI-message SSE endpoints allowed, to a raw ACP launch route refused; the driver refuses a standalone real-key launch; "muse profile runs in the agent sandbox with model traffic on the proxy only"; then "Hubble attributes policy drops to the attacker sandbox".
- **t003 (Simplify):** `/simplify` (reuse, quality, efficiency reviews) over the verifier changes and `Template.SessionOnly` (7af7ed382). The session lookup now uses the session label alone; the dropped `sandboxId` fallback looked up a `bex.co/sandbox-id` label that session sandboxes never carry. `wait_ready_pod` returns and its callers fail, instead of calling `fail` inside a command substitution. One `owned_pods` filter and one hold selector. Cleanup clears the placeholder model key, and the session is cancelled once. Run 43580573152 confirmed the refactor on production (green, agent leg included).
- **t004 (Test coverage):** the verifier is itself the test, and its paths ran on production: the agent leg and Hubble check in runs 43579027377 and 43580573152, and the readiness diagnostics on a real failure in run 43570243129. The lookup and diagnostic helpers were also exercised against a stub `kubectl` locally. No committed unit test: they are thin kubectl/jq glue whose meaningful check is the live cluster. The backend half (w7/063) carries its own tests.
- **t005:** closed 2026-11-14.
