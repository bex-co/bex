# w5 · m284 — Restart waits for an in-flight deploy under Render's Wait policy

**Worker:** worker5 **Goal:** a restart requested while a deploy is in flight waits for it, as Render's Wait policy does, instead of cancelling the new deploy. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's restart behavior under Wait and bex's current cancel path | 30m | — |
| t002 | Queue a restart behind the in-flight deploy and pick the release when it runs | 2h | t001 |
| t003 | Use the same policy decision as ordinary deploys | 1h | t002 |
| t004 | Surface the queued restart on REST, GraphQL, MCP and the dashboard | 1h | t002 |
| t005 | Live walk on dev-5: restart during a build | 30m | t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 6h45m**.

## Definition of done

- In a Wait-policy workspace, `POST /v1/services/{id}/restart` (and GraphQL/MCP/dashboard) during an in-flight deploy queues; the in-flight deploy finishes live and the restart then runs on the newly serving release. No deploy row is closed "Canceled by restart".
- Override-policy workspaces keep today's behavior; the policy is resolved by the same rule as ordinary deploys.
- ADR018:95 reads the truth (no "Wait is not implemented"), with evidence from the dev-5 walk.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 2 (2026-11-07); ADR018:95.
- **Goal linkage:** Render parity first (ADR008); git push to deploy (V0 roadmap item 3).
- **Expected outcome:** clicking Restart while a push builds no longer silently drops the push.
- **Why now:** w8/m119/m121 built Wait for ordinary deploys but left restart cancelling, and the row is marked ✅, so parity sweeps miss it.
- **Decision (2026-11-07):** the user approved the round-7 proposal ("them all to w5"). Restart follows Render's rule — Wait for workspaces created on or after 2025-07-14 — through the w8/m121 resolver. This is the parity-sufficient choice under the AGENTS rule "never park parity work on an optional decision"; an always-Wait variant would be a divergence needing sign-off.
- **Render parity:** included.
