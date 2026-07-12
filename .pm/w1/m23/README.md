# w1 · m23 — Misc: small parity + hardening/dev-infra chores

**Worker:** worker1 **Goal:** Clear the sub-hour w1 inbox in one shippable chunk — make health-check gating real (`healthCheckPath`), keep dependency hygiene current (Dependabot), let the GitOps stack schedule on the local mock, and sweep stale "single-node / data-loss" comments the m19 rebuild inverted. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                                                       |
| ---- | ----------------------------------------------------------------------------------------- | --- | --------------------------------------------------------------- |
| t001 | Wire `spec.healthCheckPath` into a ReadinessProbe (or drop the field) — from `005`         | 45m | —                                                               |
| t002 | Triage the 36 Dependabot findings; batch safe upgrades — from `006`                        | 45m | —                                                               |
| t003 | Mock cluster: label workers `bex.co/pool=platform` so GitOps schedules locally — from `015` | 30m | —                                                               |
| t004 | Sweep stale "single-node / data-loss" comments inverted by m19 — from `016`                | 30m | —                                                               |
| t005 | Simplify — `/simplify` over the code changed (t001, t003)                                   | 15m | w1/m23/t001,w1/m23/t003                                          |
| t006 | Test coverage — envtest for the ReadinessProbe from t001                                    | 20m | w1/m23/t001                                                     |
| t007 | Closeout — verify each chore's end state, then move the milestone to `done/`                | 10m | w1/m23/t002,w1/m23/t004,w1/m23/t005,w1/m23/t006                 |

## Definition of done

- `healthCheckPath`: either the operator sets an HTTP `ReadinessProbe` from `spec.healthCheckPath` (so replica-readiness gating becomes real health gating) with an envtest asserting it, or the field is removed from the contract — decided and reflected in `docs/ADR004-deployment.md`.
- Dependabot: the 36 findings triaged (ecosystem + reachability of the 2 criticals documented); safe upgrades batched and merged; any needing breaking bumps filed as a follow-up note.
- Mock cluster: `scripts/mock-cluster.sh` labels its worker nodes `bex.co/pool=platform` at create, so the platform GitOps stack (kratos/hydra/openfga/openbao, CNPG, prometheus/loki, operator/api/dashboard) schedules on the mock instead of going Pending — unblocking w4/m7, w4/m11, parts of w3/m5–m6 locally.
- Comment sweep: the stale local-path/single-node/data-loss claims in `deploy/gitops/base/{loki,prometheus,log-shipper}.yaml`, `docs/ADR010-observability.md:42`, the dead `10.0.0.0/16` in `traefik.values.yaml`, and the `docs/ADR002-architecture.md` §Data-layering line are corrected to the post-m19 reality (hcloud-csi network volumes survive node loss; network is `10.10.0.0/16`). Zero behavior change.

## Source + Goal linkage

- **Source:** inbox notes `w1/005` (2026-07-08 docs-vs-code audit), `w1/006` (Dependabot, 2026-07-08 push), `w1/015` + `w1/016` (2026-07-11 board review themes A & F); all moved to `w1/done/` on promotion.
- **Goal linkage:** pillar 1 (Render `healthCheckPath` parity, t001) + platform hygiene / dev-loop unblocking (w1/m7 hardening theme, t002–t004).
- **Expected outcome:** the health-gating story in `docs/ADR004-deployment.md` becomes true; the security backlog is current; the mock cluster can run the full platform stack; the tree stops asserting pre-m19 falsehoods.
- **Why now:** each item is sub-hour and homeless — the sizing rule keeps them out of their own milestones, so they're grouped here (your "misc milestone for the chores"). t003 in particular is a one-line blocker under several w4 milestones' local verification.
- **Render parity closing task: omitted.** The only user-facing item (t001) wires an existing `App` contract field (`healthCheckPath`, already in the CRD) into operator mechanism — it adds no new REST/GraphQL/MCP/UI surface; the other three are security hygiene, local-dev infra, and docs. Simplify + Test coverage are scoped to the one code-bearing item (t001, plus t003's script change).
