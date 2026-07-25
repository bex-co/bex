# w1 · m23 — Misc: small parity + hardening/dev-infra chores

**Worker:** worker1 **Goal:** Clear the sub-hour w1 inbox in one shippable chunk — make health-check gating real (`healthCheckPath`), keep dependency hygiene current (Dependabot), let the GitOps stack schedule on the local mock, and sweep stale "single-node / data-loss" comments the m19 rebuild inverted. **Status:** done (2026-07-12)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                                                       |
| ---- | ----------------------------------------------------------------------------------------- | --- | --------------------------------------------------------------- |
| t001 | Wire `spec.healthCheckPath` into a ReadinessProbe — from `005` — **DONE**                 | 45m | —                                                               |
| t002 | Triage the 33 Dependabot findings; batch safe upgrades — from `006` — **DONE**            | 45m | —                                                               |
| t003 | Mock cluster: label workers `bex.co/pool=platform` so GitOps schedules locally — from `015` — **DONE** | 30m | —                                                               |
| t004 | Sweep stale "single-node / data-loss" comments inverted by m19 — from `016` — **DONE**    | 30m | —                                                               |
| t005 | Simplify — `/simplify` over the code changed (t001, t003) — **DONE**                      | 15m | w1/m23/t001,w1/m23/t003                                          |
| t006 | Test coverage — envtest for the ReadinessProbe from t001 — **DONE**                       | 20m | w1/m23/t001                                                     |
| t007 | Closeout — verify each chore's end state, then move the milestone to `done/` — **DONE**   | 10m | w1/m23/t002,w1/m23/t004,w1/m23/t005,w1/m23/t006                 |

## Definition of done

- `healthCheckPath`: either the operator sets an HTTP `ReadinessProbe` from `spec.healthCheckPath` (so replica-readiness gating becomes real health gating) with an envtest asserting it, or the field is removed from the contract — decided and reflected in `docs/ADR004-app-deployment.md`.
- Dependabot: the findings triaged (ecosystem + reachability of the 2 criticals documented); safe upgrades batched and merged; any needing breaking bumps filed as a follow-up note.
- Mock cluster: `scripts/mock-cluster.sh` labels its worker nodes `bex.co/pool=platform` at create, so the platform GitOps stack (kratos/hydra/openfga/openbao, CNPG, prometheus/loki, operator/api/dashboard) schedules on the mock instead of going Pending — unblocking w4/m7, w4/m11, parts of w3/m5–m6 locally.
- Comment sweep: the stale local-path/single-node/data-loss claims in `deploy/gitops/base/{loki,prometheus,log-shipper}.yaml`, `docs/ADR010-observability.md:42`, the dead `10.0.0.0/16` in `traefik.values.yaml`, and the `docs/ADR002-architecture.md` §Data-layering line are corrected to the post-m19 reality (hcloud-csi network volumes survive node loss; network is `10.10.0.0/16`). Zero behavior change.

## Outcome (2026-07-12)

- **t001 + t006 (health gating):** `reconcileKubernetes` now sets an HTTP `ReadinessProbe` (`GET spec.healthCheckPath` on the app port; empty → `/`) on every non-worker container — workers/cron/static_site expose no port and carry no probe. The `HealthCheckPath` type comment + the generated CRD description were rewritten from "unused" to what it does; `docs/ADR004-app-deployment.md` states the readiness story truthfully; the `ADR018` parity row flipped ◐→✅. Three envtest cases assert it (explicit path, default `/`, worker-omitted) — mutation-tested to confirm they fail if the probe desyncs (not a tautology). `make test` green.
- **t002 (Dependabot):** all 33 alerts are npm (dashboard) — zero Go-module findings; both criticals (`vitest`, `shell-quote`) are dev-only/unreachable but fixed anyway. Safe batch: `vitest` 4.0.16→4.1.10, `vite` 7.3.0→7.3.6, `js-cookie` 3.0.5→3.0.8 (+ `@vitest/coverage-v8` peer), plus `resolutions` for `shell-quote`/`undici`(^7.28.0 — the prior ^7.24.0 floor was stale)/`ws`/`flatted`/`immutable`. `typecheck`/`test` (694)/`build` all green. Residuals (lodash no-fix; minimatch/picomatch multi-major; srvx; @tanstack/start-server-core; vite 8.x breaking) captured in `w1/018.md`.
- **t003 (mock pool label):** rather than a create-time `kubectl label` (which `scale N` workers would miss), the `bex.co/pool=platform` label is baked into the CAPD worker's `kubelet --node-labels` in `infra/clusterapi/overlays/local-capd/cluster.yaml` — mirroring the prod hetzner-caph template minus the taint (tolerations are no-ops on untainted nodes), so local/prod chart values stay identical and scaled-in workers carry it too. `scripts/mock-cluster.sh` carries a pointer comment.
- **t004 (comment sweep):** corrected the single-node/data-loss claims in `loki.yaml`, `prometheus.yaml`, `log-shipper.yaml`, `docs/ADR010-observability.md`, the dead `10.0.0.0/16` (now `10.10.0.0/16`) in the prod `traefik.values.yaml`, the base `traefik.values.yaml` single-node framing, and `docs/ADR002-architecture.md` §Data-layering (single etcd _member_ ≠ single-node cluster). Also aligned the same etcd claim in `docs/ADR003-control-plane.md` for consistency. Comments/docs only, prettier-clean.
- **t005 (simplify):** aligned the new probe with the codebase's compact idiom (`&corev1.Probe{ProbeHandler: …}`, matching `keyvalue_controller.go`); consciously declined the rest (comment density matches the file; the two `!worker` guards separate concerns). gofmt/vet/test green.

## Source + Goal linkage

- **Source:** inbox notes `w1/005` (2026-07-08 docs-vs-code audit), `w1/006` (Dependabot, 2026-07-08 push), `w1/015` + `w1/016` (2026-07-11 board review themes A & F); all moved to `w1/done/` on promotion. Follow-up residuals from `006` → `w1/018.md`.
- **Goal linkage:** pillar 1 (Render `healthCheckPath` parity, t001) + platform hygiene / dev-loop unblocking (w1/m7 hardening theme, t002–t004).
- **Expected outcome:** the health-gating story in `docs/ADR004-app-deployment.md` becomes true; the security backlog is current; the mock cluster can run the full platform stack; the tree stops asserting pre-m19 falsehoods.
- **Why now:** each item is sub-hour and homeless — the sizing rule keeps them out of their own milestones, so they're grouped here (your "misc milestone for the chores"). t003 in particular is a one-line blocker under several w4 milestones' local verification.
- **Render parity closing task: omitted.** The only user-facing item (t001) wires an existing `App` contract field (`healthCheckPath`, already in the CRD) into operator mechanism — it adds no new REST/GraphQL/MCP/UI surface; the other three are security hygiene, local-dev infra, and docs. Simplify + Test coverage are scoped to the one code-bearing item (t001, plus t003's template change).
