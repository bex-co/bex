# w7 · m33 — Fix CNPG bootstrap vs. the tenant egress deny

**Worker:** worker7 **Goal:** Managed Postgres works on tenant nodes again: CNPG init/instance pods can reach the Kubernetes API while tenant application workloads keep the full node/metadata egress deny. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce: managed Database on a tenant node + direct API-service probe from the CNPG init/instance identity | 30m | —          |
| t002 | Split/narrow the Cilium selectors: tenant apps keep the node/metadata deny; platform-managed CNPG pods get exactly the k8s-API reachability they need | 60m | t001       |
| t003 | Policy-render/unit regression + live bootstrap test: fresh-tenant-node Database reaches Ready and serves `SELECT 1` | 45m | t002       |
| t004 | Simplify — `/simplify` over the code this milestone changed                                              | 20m | t003       |
| t005 | Test coverage — meaningful tests for the policy split + failure modes                                    | 30m | t003       |
| t006 | Closeout — DoD met → move milestone to `done/`                                                           | 10m | t005       |

## Definition of done

A fresh `dpg-…` Database scheduled onto a tenant node reaches `Ready` and answers `SELECT 1` end to end, while the w7/m4 tenant node/metadata egress deny is re-verified intact for application pods (the reachability matrix run stays green). The Cilium policy change carries a render/unit regression test so the selector split can't silently regress.

## Source + Goal linkage

- **Source:** promotes `w7/004` (filed 2026-07-15 by the production `w9/m3` rollout verification: both a legacy and a fresh Database stalled in CNPG init — `dial tcp 10.96.0.1:443: i/o timeout` — because `default/deny-tenant-node-and-metadata-egress` selects every pod carrying `app.bex.co/workspace`, which the operator deliberately propagates onto CNPG pods).
- **Goal linkage:** managed Postgres (docs/ADR009-postgresql-management.md) must actually work under the w7 isolation posture (docs/ADR022-tenant-isolation.md); this reconciles the two.
- **Expected outcome:** creating a managed Postgres in production succeeds regardless of which node pool it lands on; no isolation regression.
- **Why now:** live production defect — any new Database landing on a tenant node is broken today. Deny-overrides-allow semantics mean no quick allow-rule can fix it; the selectors must be split deliberately.
- **Render parity closing task: omitted** — network-policy mechanism only; no REST/GraphQL/MCP/UI surface change.
- **Out of scope (separate user approval, per the note):** repairing or retiring the pre-existing unrecoverable `default/biliblilitest` cluster.
