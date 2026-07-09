# w7 — Tenant isolation & security hardening (worker7)

**Worker:** worker7 Created 2026-07-09 from `/pm-brainstorm for w7` (take 2). Executes GOAL.md V0 #7 ("Security review") as work, and is the re-scope of tenant isolation the w1/m6 removal anticipated (`DO_NOT_DO.md` ladder: namespace tier → microVM, never vcluster). w1/m9 closes the API-layer front door (OpenFGA); w7 closes the runtime side doors verified open on 2026-07-09: a flat pod network (all tenant Apps in one namespace, zero tenant NetworkPolicies), no Pod Security/quota enforcement (tenant pods can run privileged and carry SA tokens), and a public API with no rate limiting. Ordered by hole size: network isolation first, then workload hardening, then API abuse limits; sequence alongside/after w1/m9, before real tenants exist to migrate.

## Milestones

- [x] **m1** — East-west tenant isolation: default-deny network for tenant workloads (8 tasks) ← from `/pm-brainstorm for w7` 2026-07-09
- [ ] **m2** — Tenant workload hardening: Pod Security baseline + quotas + token hygiene (7 tasks) ← from `/pm-brainstorm for w7` 2026-07-09
- [ ] **m3** — bex-api abuse hardening: Render-shaped rate limits + request caps (8 tasks) ← from `/pm-brainstorm for w7` 2026-07-09

## Inbox

- `001.md` — Image CVE scanning in CI (trivy over the built image in `deploy.yml`)

## Not in w7 (deliberate)

- **microVM runtime tier** (Kata/gVisor `RuntimeClass`) — the isolation ladder's next rung; parked in [`.pm/FUTURE-MAYBE.md`](../FUTURE-MAYBE.md) with a public-GA trigger.
- **Dependabot triage** — stays `w1/006` per that note's own instruction (sub-hour triage; promote only if fixes need breaking upgrades); w7 cross-references it as adjacent hygiene.
- **Static-CIDR firewalls / vcluster / sandboxes** — DO_NOT_DO items. m1's pod-level east-west policies are a different layer from the banned `:22`/`:6443` source-IP allowlist (no source-IP allowlists anywhere in w7).
