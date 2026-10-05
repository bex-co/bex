# w5 · m115 — By-id verbs: one resolver, one 403/404 rule, and a guard that checks types instead of source text

**Worker:** worker5 **Goal:** Every by-id verb on REST, GraphQL and MCP resolves its workspace through `core.ScopeByID`, answers non-members per one documented ADR072 rule and audits the refusal; a type-level guard makes a new id kind impossible to wire otherwise. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Write the by-id policy matrix into ADR072 and the backend guide | 30m | — |
| t002 | Move Blueprints and API keys onto ScopeByID | 45m | t001 |
| t003 | Make ScopeByID route only, with one core policy for choosing the workspace | 1h | t002 |
| t004 | Fix environment create and device unregister outside the default workspace | 45m | t003 |
| t005 | Make routing hooks required interface methods | 30m | t003 |
| t006 | Replace the source-text guard with an id-kind registry | 1h | t003 |
| t007 | Render parity | 20m | t004, t005, t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

- Every workspace-owned by-id family on REST, GraphQL and MCP, including Blueprints and API keys, answers per one documented ADR072 matrix (named exemptions only).
- A non-member refusal writes one audit row, and an ownerless API key revoke writes exactly one.
- Environment create in another workspace's project, and device unregister without ownerId, act in the right workspace.
- The guard fails when a workspace-owned id kind has no owner lookup or exemption, and a comment can't satisfy it.
- Backend tests (real DB) and lint pass, and w4/m172's DoD probes still pass.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05. The same fix class landed four times in one day: w4/m169 (`9a95c445c`), w4/194 (`0401d251e`), w4/m172 (`d1e4c9315`), and w4/199 (`f837cec46`), which restored ADR072 #8's typed-id 403 after m172 collapsed it and turned deploys red. Code-verified: Blueprints and API keys still answer non-members 404 while `ScopeByID` families answer 403, and the guard matches source text.
- **Goal linkage:** ADR006/ADR018 Render-compatible by-id API; ADR072 #8 typed-id semantics; ADR012 authz and audit.
- **Expected outcome:** By-id verbs behave the same everywhere, every refusal is audited, and a new id kind can't skip routing.
- **Why now:** Four same-class fixes in 24 hours, one of which broke deploys; the remaining stragglers are known.
- **Render parity included:** status codes of by-id verbs change on REST/GraphQL/MCP.
- **Gap vs w4/m172 (blocked on its live replay, t007):** m172 deliberately kept Blueprints (m169) and API keys (194) unchanged ("Controls, unchanged" in its README). This milestone is that deferred consolidation. Re-run m172's DoD probes after it ships.
