# w7 · m393 — The isolation matrix's agent leg starts its sandbox through a real chat-only agent session

**Worker:** worker7 **Goal:** `isolation-matrix.yml` with `verify_agent_driver=true` runs every agent-image check on production again, against a sandbox started the way production starts one: a chat-only agent session in the disposable workspace. **Status:** pending

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verifier: the agent leg's sandbox comes from a chat-only agent session with a placeholder workspace model key | 90m | — |
| t002 | Production run: every agent-leg check passes, the muse profile included | 45m | t001 |
| t003 | Simplify | 30m | t002 |
| t004 | Test coverage | 30m | t002 |
| t005 | Closeout | 10m | t003, t004 |

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
