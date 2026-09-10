# w7 · m90 — CNPG `enablePDB: false` for single-instance clusters

**Worker:** worker7 **Goal:** Single-instance managed Postgres stops installing a zero-disruption PDB that pins autoscaled serving nodes. **Status:** todo

**Estimate:** ~3h including standing closing tasks.

## Tasks (in order)

| id   | title                                                                                   | est | depends_on      |
| ---- | --------------------------------------------------------------------------------------- | --- | --------------- |
| t001 | Project `enablePDB: false` when `instances == 1`; leave HA (≥2) on CNPG default         | 45m | —               |
| t002 | Envtest: single-instance gets `false`; HA cluster unchanged                             | 45m | w7/m90/t001     |
| t003 | Existing single-instance Clusters reconcile to `false` (or documented one-shot)         | 30m | w7/m90/t001     |
| t004 | Close ADR060 D8 residual with drain/elasticity note                                     | 20m | w7/m90/t001     |
| t005 | Simplify                                                                                | 30m | w7/m90/t004     |
| t006 | Test coverage                                                                           | 45m | w7/m90/t004     |
| t007 | Closeout                                                                                | 15m | w7/m90/t005, w7/m90/t006 |

## Definition of done

- New single-instance managed Postgres Clusters are created with `spec.enablePDB: false`.
- HA clusters (≥2 instances) keep CNPG's default PDB behavior.
- Existing single-instance Clusters reach `enablePDB: false` via reconcile or a documented one-shot; envtest pins both shapes.
- ADR060 no longer lists this as an open D8 follow-up.

## Source + Goal linkage

- **Source:** user-approved `/pm-brainstorm for w7 for top 3 customer-impactfully work` 2026-09-09 #2; ADR060 §D8 companion fix (2026-08-09 PDB-pin incident class).
- **Goal linkage:** [.pm/GOAL.md](../../GOAL.md) #6 (add/remove machines / elasticity); ADR060 reliability.
- **Expected outcome:** Autoscaled serving nodes stop being pinned by PDBs that protect nothing on single-instance Postgres; drains and rollouts next to tenant Apps succeed.
- **Why now:** Build-pool taints only protect build nodes; serving-pool pin risk remains; last unowned D8 residual; complementary to serving `max=3` already landed.
- **Render parity omitted:** operator/CNPG mechanism only — no REST/GraphQL/MCP/UI surface change.
