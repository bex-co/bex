# w10 · m6 — Cross-stream note burn-down round 2: Traefik LB label lifecycle + orphaned DatabaseLogs prune

**Worker:** worker10 **Goal:** the two remaining actionable cross-stream sub-hour items land as one burn-down round (the m4 pattern, per w10's spare-capacity charter): the production Traefik LB's Terraform block stops threatening to strip the hcloud CCM's `service-uid` label on the next apply, and the dashboard sheds the orphaned `DatabaseLogs` GraphQL document the Postgres-logs re-point left behind. **Status:** todo

## Tasks (in order)

| id   | title                                                                                      | est | depends_on   |
| ---- | -------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Terraform: `ignore_changes = [labels]` on `hcloud_load_balancer.traefik` + plan-diff proof    | 30m | —            |
| t002 | Prune the orphaned `DatabaseLogs` query document + generated types from the dashboard         | 15m | —            |
| t003 | Simplify — `/simplify` over the milestone's diff                                              | 15m | t001, t002   |
| t004 | Test coverage — guards appropriate to the shipped changes                                     | 15m | t001, t002   |
| t005 | Closeout — move to `done/` when the DoD holds                                                 | 15m | t004         |

## Definition of done

A `terraform plan` against production shows no attempt to modify the LB's labels (the CCM's `hcloud-ccm/service-uid` label survives an apply), with the plan output recorded; `grep -r DatabaseLogs dashboard/src` returns no orphaned document/types, and dashboard typecheck + tests stay green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 21, 2026-07-15, materialized under w10 per the user's standing "for w9 and w10" capacity directive (the round-19 m4 precedent). Absorbs `w1/025` (filed round 19, re-verified at HEAD: `infra/terraform/main.tf`'s traefik LB `lifecycle` block has `prevent_destroy` only, while its own comment says the CCM owns the `service-uid` label on the same object — the next apply fights the CCM exactly as the block was written to avoid) + dashboard-gap mine G2 (`databases.graphql:382` `query DatabaseLogs` + `operations.ts:608-645` types are referenced by no component/hook since `6f1bbaa7` re-pointed the viewer at the generic `LogsDocument`; the backend `databaseLogs` field STAYS — MCP uses it and `w3/m30/t006` extends it).
- **Goal linkage:** production-edge stability (`docs/ADR002-architecture.md` §Stable production edge) + dashboard code health; both are shipped-feature residue, squarely the "dig/polish existing" mandate.
- **Expected outcome:** no Terraform↔CCM label fight on the next apply; no dead GraphQL documents inviting future confusion.
- **Why now:** each sub-hour alone (grouped per the sizing rule); the terraform item is a live footgun armed on every `terraform apply`; w10's queue hit zero mid-round.
- **Render parity:** omitted — pure infra + dead-code removal, no REST/GraphQL/MCP/UI behavior change.
