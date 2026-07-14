# w7 — Tenant isolation & security hardening (worker7)

**Worker:** worker7 Created 2026-07-09 from `/pm-brainstorm for w7` (take 2). Executes GOAL.md V0 #7 ("Security review") as work, and is the re-scope of tenant isolation the w1/m6 removal anticipated (`DO_NOT_DO.md` ladder: namespace tier → microVM, never vcluster). w1/m9 closes the API-layer front door (OpenFGA); w7 closes the runtime side doors verified open on 2026-07-09: a flat pod network (all tenant Apps in one namespace, zero tenant NetworkPolicies), no Pod Security/quota enforcement (tenant pods can run privileged and carry SA tokens), and a public API with no rate limiting. Ordered by hole size: network isolation first, then workload hardening, then API abuse limits; sequence alongside/after w1/m9, before real tenants exist to migrate.

## Milestones

- [x] **m1** — East-west tenant isolation: default-deny network for tenant workloads (8 tasks) ← from `/pm-brainstorm for w7` 2026-07-09
- [x] **m2** — Tenant workload hardening: Pod Security baseline + quotas + token hygiene (7 tasks) ← from `/pm-brainstorm for w7` 2026-07-09
- [x] **m3** — bex-api abuse hardening: Render-shaped rate limits + request caps (8 tasks) ← from `/pm-brainstorm for w7` 2026-07-09
- [x] **m4** — Tenant egress hardening: block cloud metadata + node-local endpoints (6 tasks) ← from `/pm-brainstorm for w7` 2026-07-11
- [x] **m5** — Managed Key Value network access controls (ipAllowList parity) (8 tasks) ← from `/pm-brainstorm for w7` 2026-07-11
- [x] **m6** — Custom domain collision + reserved-host guard (Render "already in use" parity) (7 tasks) ← from `/pm-brainstorm for w7` 2026-07-12
- [x] **m7** — Least-privilege platform RBAC (operator + bex-api secret scoping) (6 tasks) ← from `/pm-brainstorm for w7` 2026-07-12
- [x] **m8** — Tenant registry authn/z (close the unauthenticated Zot hole) (8 tasks) ← from `/pm-brainstorm more for w7` 2026-07-12
- [x] **m9** — Per-workspace abuse limits (creation caps + build concurrency) (7 tasks) ← from `/pm-brainstorm more for w7` round 2, 2026-07-12
- [x] **m10** — Security hygiene: image CVE scanning in CI + HTTP hardening headers (7 tasks) ← from `/pm-brainstorm more milestones to work on` 2026-07-13, groups `001`, `002` (each sub-hour)
- [x] **m11** — Admission-time tenant-image signature verification (8 tasks) ← from `/pm-brainstorm more milestones to work on` 2026-07-13 (`w6/006` shipped signing, verification was deferred and never picked back up — flagged three times across two workstreams)
- [x] **m27** — Blueprints dashboard surface (list · manifest · validate · sync) (10 tasks) ← from `/pm-brainstorm more tasks for w5` 2026-07-13; closes last ✖ UI cell in parity ledger
- [ ] **m12** — Delete really deletes: purge orphaned tenant artifacts on service/Postgres/Key Value deletion (12 tasks) ← from `/pm service deletion across all service, db, key value types` 2026-07-14 (verbs shipped in w2/m4 + w5/m14; this closes the delete-time teardown gaps)
- [x] **m28** — Build logs: ship `type=build` into the log store (9 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (ADR018 §Logs "`type=build` stays empty by design"; unblocks `w5/m29`); placed here for capacity — topical owner w3 has 4 open milestones (the m27 precedent)
- [x] **m29** — Execute and record the ADR031 restore drills (7 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (`w2/done/m27`'s "operational drills require live cluster execution" residual — etcd, OpenBao, and bex-db restores have never actually been run)

## Inbox

- `003.md` — Secret scanning in CI (gitleaks over pushed diffs/history) — renumbered from `002.md` 2026-07-14: it had collided with the done HTTP-headers note (`done/002.md`, grouped into m10); this one was never done and stays open

## Not in w7 (deliberate)

- **microVM runtime tier** (Kata/gVisor `RuntimeClass`) — the isolation ladder's next rung; parked in [`.pm/FUTURE-MAYBE.md`](../FUTURE-MAYBE.md) with a public-GA trigger.
- **Dependabot triage** — stays `w1/006` per that note's own instruction (sub-hour triage; promote only if fixes need breaking upgrades); w7 cross-references it as adjacent hygiene.
- **Static-CIDR firewalls / vcluster / sandboxes** — DO_NOT_DO items. m1's pod-level east-west policies are a different layer from the banned `:22`/`:6443` source-IP allowlist (no source-IP allowlists anywhere in w7).
