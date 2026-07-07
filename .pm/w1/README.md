# w1 — bex platform roadmap (worker1)

**Worker:** worker1 Converted from the `.tmp/` backlog (items 001–010) on 2026-07-02. Ordered roughly by priority/dependency: de-risk the live system, build the source-of-truth control plane, then the elastic/cost machinery, then pipeline, isolation, and hardening.

## Milestones

- [x] **m1** — Reliability: fix config drift + back up etcd (4 tasks) ← from `009`, `007` — done 2026-07-05, moved to `done/m1/`
- [ ] **m2** — Control plane: Postgres source of truth in `lego/backend` (7 tasks) ← from `005` (t001 done)
- [ ] **m2.5** — Refactor bex-api into feature packages (one package per feature) (9 tasks) ← from `/pm` architecture review 2026-07-06
- [ ] **m3** — Elastic substrate: bin-pack + autoscale (5 tasks) ← from `002`, `004` (001 done)
- [ ] **m4** — Free tier = sleep: scale-to-zero + wake activator (5 tasks) ← from `003`
- [ ] **m5** — Build & deploy from git, in-cluster (3 tasks) ← from `008`
- [ ] **m6** — Multi-tenant isolation (4 tasks) ← from `006`
- [ ] **m7** — Prod hardening: network · secrets · images (5 tasks) ← from `010`
