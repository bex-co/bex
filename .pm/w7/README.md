# w7 — Tenant isolation & security hardening (worker7)

**Worker:** worker7 Created 2026-07-09 from `/pm-brainstorm for w7` (take 2). Executes GOAL.md V0 #7 ("Security review") as work, and is the re-scope of tenant isolation the w1/m6 removal anticipated (`DO_NOT_DO.md` ladder: namespace tier → microVM, never vcluster). w1/m9 closes the API-layer front door (OpenFGA); w7 closes the runtime side doors verified open on 2026-07-09: a flat pod network (all tenant Apps in one namespace, zero tenant NetworkPolicies), no Pod Security/quota enforcement (tenant pods can run privileged and carry SA tokens), and a public API with no rate limiting. Ordered by hole size: network isolation first, then workload hardening, then API abuse limits; sequence alongside/after w1/m9, before real tenants exist to migrate.

## Local dev environment

Develop against `.pm/w7/dev-7/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-7` gets its own Kratos + Hydra + Mailpit (namespace `dev-7-auth`) and app namespace (`dev-7`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=7 (`dev-7/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash .pm/w7/dev-7/up.sh` — bring it up (idempotent — safe to re-run)
- `bash .pm/w7/dev-7/status.sh` — health check (processes, pods, HTTP)
- `bash .pm/w7/dev-7/down.sh` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)

`up.sh` prints the dashboard command to point at it once bex-api is running.

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
- [x] **m12** — Delete really deletes: purge orphaned tenant artifacts on service/Postgres/Key Value deletion (12 tasks) ← from `/pm service deletion across all service, db, key value types` 2026-07-14 (verbs shipped in w2/m4 + w5/m14; this closes the delete-time teardown gaps)
- [x] **m28** — Build logs: ship `type=build` into the log store (9 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (ADR018 §Logs "`type=build` stays empty by design"; unblocks `w5/m29`); placed here for capacity — topical owner w3 has 4 open milestones (the m27 precedent)
- [x] **m29** — Execute and record the ADR031 restore drills (7 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (`w2/done/m27`'s "operational drills require live cluster execution" residual — etcd, OpenBao, and bex-db restores have never actually been run)
- [x] **m30** — Render OpenAPI contract-conformance suite in CI (8 tasks) ← from `/pm-brainstorm more milestones for each worker` round 4, 2026-07-14 (the parity ledger's manual "verified vs Render's OpenAPI" method has no automated guard; the gap-well is dry so the risk is now drift, not absence). Mechanizes ADR018's central claim; w7's CI-guard charter (gitleaks/trivy/structural guards). Render-parity closing task omitted — the milestone IS the parity check
- [x] **m31** — `renderSubdomainPolicy`: disable the platform subdomain (9 tasks) ← from `/pm-brainstorm` round 6, 2026-07-14 (field-level spec-grep: enum enabled|disabled on webServiceDetails + POST/PATCH, zero hits in `lego/`; bex mirror = drop `<slug>.onbex.co` from `effectiveHosts` while custom hosts serve); host policy is the m6 domain-guard territory
- [x] **m32** — Service inbound `ipAllowList`: web services + static sites (10 tasks) ← from `/pm-brainstorm` round 7, 2026-07-14 (systematic field-diff: `[{cidrBlock, description}]` on webServiceDetails + staticSiteDetails POST/PATCH, zero hits in apps; the m5 Traefik-middleware mechanism, HTTP flavor)
- [x] **m33** — Fix CNPG bootstrap vs. the tenant egress deny (6 tasks) ← promotes `004` 2026-07-15 (production defect from the `w9/m3` rollout verification: new managed Postgres on tenant nodes stalls in CNPG init — the m4 egress-deny policy selects the workspace label the operator propagates onto CNPG pods, blocking their k8s-API traffic; deny-overrides-allow ⇒ the selectors must be split)
- [ ] **m34** — Rate-limit response headers (verify-first) (7 tasks) ← from `/pm-brainstorm` round 10, 2026-07-15 (polishes `m3`: bex sends only `Retry-After` on the 429 itself — verified in `internal/api/ratelimit.go`; t001 captures Render's actual header contract from the live API + pinned spec, and a Render-ships-nothing finding closes the milestone as parity-by-absence)

## Inbox

- `001.md` — Fresh tenant nodes cannot pull authenticated Zot images because App workloads carry no `imagePullSecret` — **promoted to `w6/m29`** 2026-07-15 (materialized under w6 for capacity); the note stays here until w6/m29 closes out, which retires it to `done/` (the `w6/015` cross-workstream precedent)

> `004.md` promoted to **m33** 2026-07-15; note moved to `done/`.

## Not in w7 (deliberate)

- **microVM runtime tier** (Kata/gVisor `RuntimeClass`) — the isolation ladder's next rung; parked in [`.pm/FUTURE-MAYBE.md`](../FUTURE-MAYBE.md) with a public-GA trigger.
- **Dependabot triage** — stays `w1/006` per that note's own instruction (sub-hour triage; promote only if fixes need breaking upgrades); w7 cross-references it as adjacent hygiene.
- **Static-CIDR firewalls / vcluster / sandboxes** — DO_NOT_DO items. m1's pod-level east-west policies are a different layer from the banned `:22`/`:6443` source-IP allowlist (no source-IP allowlists anywhere in w7).
