# w4 · m28 — Environment inbound-IP rules: real enforcement semantics

**Worker:** worker4 **Goal:** Environment-level IP rules mean what Render's docs say: they apply to all eligible public services (web/static), compose with workspace/service-level rules (a source must pass every layer), and an empty list means deny-all — with existing bex environments migrated safely (seeded open, no lockout). **Status:** todo

## Tasks (in order)

| id   | title                                                                                                | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Design decision (verify-first): eligible-target set, layer composition, empty-list migration          | 60m | —                |
| t002 | Core + store: composed rule evaluation across workspace/environment/service layers                    | 60m | t001             |
| t003 | Enforcement fan-out to eligible public web services/static sites (App CR projection → operator/Traefik) | 90m | t002             |
| t004 | Surfaces: REST/GraphQL/MCP semantics + dashboard copy for seeded default and deny-all                 | 45m | t002             |
| t005 | Render parity                                                                                          | 30m | t003, t004       |
| t006 | Simplify                                                                                               | 20m | t005             |
| t007 | Test coverage                                                                                          | 45m | t005             |
| t008 | Closeout                                                                                               | 15m | t007             |

## Definition of done

A CIDR outside an environment's rules is blocked at a member web service's public URL, not just its datastores (live test on a dev-N stack); empty-list deny-all matches Render, with existing environments migrated seeded-open and a regression test proving no lockout at migration; layered workspace+environment+service composition has a table-driven test; `docs/render-artifacts/protected-environments.md`'s "bex implementation decisions and drift" paragraph is rewritten as shipped behavior.

## Source + Goal linkage

- **Source:** promotes `.pm/w4/018.md` (filed by `w5/m31`'s Render-parity closeout, 2026-07-15). Evidence: `docs/render-artifacts/protected-environments.md:38,48` + Render's Inbound IP Rules docs — Render applies environment rules to eligible public web services/static sites plus datastores, composes them with workspace/service rules, treats zero rules as deny-all (dashboard seeds `0.0.0.0/0`); bex fans `Environment.ipAllowList` only to member Postgres/Key Value and treats empty as open.
- **Goal linkage:** tenant isolation (docs/ADR022-tenant-isolation.md) + Render parity on protected environments (docs/ADR032-environments.md).
- **Expected outcome:** no silent open-by-default divergence — the dashboard editor w5/m31 shipped now controls what it appears to control.
- **Why now:** the divergence was filed the same day the dashboard editor shipped; the UI currently implies enforcement breadth the backend doesn't deliver — a security-semantics gap, not UI copy.
- **Render parity:** included — REST/GraphQL/MCP semantics + dashboard copy change.
- **Coordinate with:** `w4/m24` (ipAllowList `{cidrBlock, description}` persistence touches the same lists — sequence or rebase, don't collide) and `w4/m23`'s shipped wire-shape work.
