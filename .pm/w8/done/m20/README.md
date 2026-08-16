# w8 · m20 — Blueprint grouping hardening: transactional writes, quota, audit, disconnect reclaim

**Worker:** worker8 **Goal:** the blueprint `projects`/`environments` grouping path stops being the board's known soft spot — grouping writes become transactional (no partial rows on mid-loop failure), per-workspace grouping creation is quota-capped with a coded refusal identical across REST/GraphQL/MCP, grouping writes emit audit events like every other mutating verb, and `DisconnectBlueprint` reclaims the empty grouping rows + ACL tuples it created (never deployed resources — Render disconnect semantics preserved). **Status:** done

## Tasks (in order)

| id   | title                                                                        | est | depends_on       |
| ---- | ---------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Transactional grouping writes (no partial project/environment rows) — **DONE**          | 45m | —                |
| t002 | Per-workspace grouping quota + coded refusal across surfaces — **DONE**                 | 45m | t001             |
| t003 | Audit events for grouping writes + conditional per-grouping ACL writes — **DONE**       | 30m | t001             |
| t004 | `DisconnectBlueprint` reclaims orphaned grouping rows + ACL tuples — **DONE**           | 30m | t001             |
| t005 | Render parity check (refusal/error shapes consistent REST/GraphQL/MCP/UI) — **DONE**    | 30m | t002, t003, t004 |
| t006 | Simplify (`/simplify` over the changed code) — **DONE**                                 | 30m | t005             |
| t007 | Test coverage (tx rollback, quota refusal, audit emission, disconnect sweep) — **DONE** | 45m | t005             |
| t008 | Closeout — **DONE**                                                                     | 15m | t007             |

## Definition of done

A blueprint sync whose grouping loop fails midway leaves zero project/environment rows or ACL tuples from that sync (verified by an injected-failure test). A sync or create whose groupings would exceed the per-workspace quota is refused before any write with one coded error, byte-identical across REST, GraphQL, and MCP. Every grouping create/update emits an audit event carrying actor + workspace + blueprint id. Disconnecting a blueprint deletes the grouping rows and ACL tuples it minted that no surviving resource references, while deployed services/datastores remain untouched (Render disconnect semantics). All covered by integration tests against the real ephemeral Postgres + OpenFGA CI harness.

## Source + Goal linkage

- **Source:** promotes the deferred residuals of `w1/049` #5 (security-scan round 7, 2026-08-14: "no per-tenant grouping quota, no transaction — mid-loop failure persists partial rows, an unconditional per-grouping ACL write, grouping writes bypass the audit events, and `DisconnectBlueprint` never reclaims orphans"), resurfaced by the blueprint Render-parity review 2026-08-15. Fix-shape precedent: `w7/done/m72/t004` (exports maxItems + active cap); same per-workspace-quota bucket `w1/048` deferred for webhooks.
- **Goal linkage:** tenancy + enforced authz (the Render-alternative core) and the ADR028→ADR060 security-review lineage — this closes a triaged register item rather than leaving it to be re-reported by scan round 8.
- **Expected outcome:** the grouping write path has the same durability/audit/quota properties as every other mutating blueprint verb; the `w1/049` #5 register entry can be marked materialized.
- **Why now:** round-7 triage is fresh and the m19 spec-parity work touches the same blueprint service files — doing both in sequence in one workstream avoids merge friction; every prior scan round shows deferred register items get re-reported until closed. Render parity task included: the quota refusal is a tenant-facing coded error that must be identical across REST/GraphQL/MCP (a bex extension — Render has no grouping quota — so it is documented as an extension, not silent drift).
- **DO_NOT_DO constraints honored:** no change to disconnect-leaves-resources semantics; no new user-facing product surface — hardening of an existing one.
