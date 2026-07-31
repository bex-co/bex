# w7 · m59 — Per-workspace storage & object quota hardening (ADR045 Finding 4)

**Worker:** worker7 **Goal:** every tenant namespace's ResourceQuota bounds storage bytes, PVC count, and LoadBalancer/NodePort Services per plan — closing the unbounded-disk cost-abuse vector the round-3 audit filed as Medium **Status:** done — **DONE 2026-07-31**

## Tasks (in order)

| id   | title                                                                                             | est | depends_on | status |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Add per-plan `requests.storage` + `persistentvolumeclaims` + LB/NodePort zero-caps to `quotaForPlan` | 45m | —          | — **DONE** |
| t002 | Disk-autoscaler quota collision: surface a Database status condition, don't hot-loop               | 60m | t001       | — **DONE** |
| t003 | Rollout to existing `tea-*` namespaces + document per-plan values + ADR045 disposition             | 30m | t002       | — **DONE** |
| t004 | Simplify pass over the changed code                                                                 | 20m | t003       | — **DONE** |
| t005 | Test coverage: quota shape per plan, quota-rejected grow surfacing, rollout convergence             | 45m | t003       | — **DONE** |
| t006 | Closeout                                                                                            | 10m | t005       | — **DONE** |

## Definition of done

- Every provisioned tenant namespace's ResourceQuota carries `requests.storage`, `persistentvolumeclaims`, `services.loadbalancers: 0`, and `services.nodeports: 0` matching its plan tier, with the per-plan values documented.
- A PVC request or CNPG disk-autoscale grow that would exceed the namespace quota is rejected by the API server and observably surfaced on the owning Database/KeyValue status (condition or status message), with the operator backing off instead of hot-loop erroring; growth resumes once quota allows.
- Existing `tea-*` namespaces converge to the new quota shape on reconcile without disrupting running workloads.
- `docs/ADR045-security-review-round3.md` Finding 4's disposition row records the fix.
- Backend + operator suites and lint green.

## Source + Goal linkage

- **Source:** `docs/ADR045-security-review-round3.md` Finding 4 [Medium] via `/pm-brainstorm more for w7` round 2, 2026-07-30 — the ownership check found the ADR's "cross-referenced to w3/m34" filed no actual m34 task, and m34 is decision-gate-blocked, so the finding was unowned. Verified live: `quotaForPlan` (`lego/backend/internal/store/namespaces.go:363`) has no storage/PVC/LB dimensions while the CNPG disk-autoscaling loop grows tenant PVCs automatically.
- **Goal linkage:** tenant isolation / per-workspace abuse limits — the w7/m9 creation-caps lineage pushed to the storage axis; also protects billing integrity (Postgres storage is billed per GB-second from `lego/types/tiers/tiers.yaml` sizes, so an uncapped disk is an unbounded-bill vector).
- **Expected outcome:** a tenant can no longer inflate storage without bound (by count-capped-but-size-unbounded PVCs or autoscale growth); quota-blocked growth is visible on the resource's status instead of failing silently.
- **Why now:** the finding is Medium and live today (not only when w3/m34 lands); the tier catalog already carries the per-plan storage numbers to anchor the caps; and w3/m34 will later make this quota the **sole** enforcement path — the caps must exist before that. Coordination: this touches the same `quotaForPlan` seam m34 will edit; m34 is blocked, so this lands first.
- **Render parity:** omitted — operator/quota mechanism with no REST/GraphQL/MCP/UI surface change; failures surface via existing status fields.
