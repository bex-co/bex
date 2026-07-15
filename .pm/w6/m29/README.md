# w6 · m29 — Tenant image pulls by design: imagePullSecret on generated workloads

**Worker:** worker6 **Goal:** Fresh tenant nodes pull authed Zot images because the operator attaches `BEX_REGISTRY_PULL_SECRET` to the workloads it generates — the behavior docs/ADR022 already documents — retiring the hand-patched `default`-SA interim fix (undeclared prod state). **Status:** todo

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Operator: attach `BEX_REGISTRY_PULL_SECRET` as `imagePullSecrets` on generated Deployments/CronJobs (+ static/predeploy Jobs where they pull tenant images); wire the env var into the prod operator deployment | 60m | —          |
| t002 | Undo the interim drift: remove the hand-patched `default`-SA `imagePullSecrets` once t001 is live; confirm no other tenant namespace needs the same | 20m | t001       |
| t003 | Live verify: git-built image pulls first-try on a fresh tenant node with no SA patch present               | 30m | t002       |
| t004 | Simplify — `/simplify` over the code this milestone changed                                                | 20m | t003       |
| t005 | Test coverage — envtest: generated workloads carry the secret when set, omit it when unset (byte-identical dev default) | 30m | t003       |
| t006 | Closeout — DoD met → move milestone to `done/`                                                             | 10m | t005       |

## Definition of done

A fresh tenant node (autoscale-up or roll) pulls an authed Zot image via the workload-attached `imagePullSecrets` alone: `kubectl get sa default -o yaml` in the apps namespace shows no hand patch, every operator-generated pod spec that pulls a tenant image carries the secret when `BEX_REGISTRY_PULL_SECRET` is set, and behavior with it unset is byte-identical to today (dev default). ADR022's described behavior is the implemented behavior.

## Source + Goal linkage

- **Source:** promotes `w7/001` (filed 2026-07-15 by the `w1/m36` tenant-pool roll incident: every git-built App went `ImagePullBackOff` on the fresh node — the images had only ever pulled because they were node-cached from before `w7/m8` enabled Zot auth; interim SA patch applied live = GitOps drift). Materialized under **w6** for capacity (topical owner w7 takes m33 this round; the w6 m19–m23 cross-placement precedent).
- **Goal linkage:** docs/ADR022-tenant-isolation.md §Registry access control — `PULL_SECRET` is specified as "attached to tenant Deployments/CronJobs as an `imagePullSecret`"; the operator just never did it.
- **Expected outcome:** the outage class is gone — autoscaler scale-ups, node failures/remediations, and future rolls pull images first-try; prod state is fully declared again.
- **Why now:** every fresh tenant node hits this today; only the undeclared hand patch is holding prod up, and the next cluster rebuild would silently lose it.
- **Render parity closing task: omitted** — operator-internal mechanism; no REST/GraphQL/MCP/UI surface change.
