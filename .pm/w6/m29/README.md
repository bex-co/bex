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

## Implementation evidence (2026-07-15)

- The original `w7/m8` implementation was already present and running in prod when this milestone started: `AppReconciler.imagePullSecrets` feeds Deployment, CronJob/manual-run, pre-deploy, and static-publish pod specs; `cmd/manager` reads `BEX_REGISTRY_PULL_SECRET`; the prod manager manifest sets `bex-registry-pull`.
- The live sweep found the residual that made the ServiceAccount fallback necessary: `default/eden-cms-v2` still had no workload-level secret because generation 79's build was failing while its generation-23 Deployment stayed live. The operator only reached the normal injection seam after a successful build.
- The pending implementation adds an early, update-only Deployment/CronJob backfill before source builds can halt reconciliation, removes the default-ServiceAccount fallback from `deploy/gitops/base/tenant-namespace.yaml`, and adds envtests for configured/unconfigured Deployment, CronJob, manual-run Job, pre-deploy Job, stale-workload backfill, and idempotence. Static-publish set/unset coverage already exists in `internal/publish`.
- Verification is green: focused controller/predeploy/publish tests, `make test`, `scripts/gitops-validate.sh`, and change-scoped golangci-lint (`--new-from-rev=HEAD`, zero issues). The repository-wide lint command still reports 31 unrelated pre-existing findings outside this diff.
- The fresh-node proof passed live at 2026-07-15T21:21Z. The autoscaler had scaled `bex-tenant-0` from 1→2 and created node `bex-tenant-0-8fkmg-dq6cr` at 21:12:53Z. With `bex-platform-prod` auto-sync temporarily paused, the old tenant node cordoned, and `default/default` ServiceAccount `imagePullSecrets` empty, the App-owned `eden-cms-v2` Deployment was patched with the workload secret. Pod `eden-cms-v2-574b6645ff-8x5g8` scheduled on the fresh node and pulled `zot.bex-registry.svc:5000/eden-cms-v2:gen-23` first-try in 11.566s (`sha256:09c45ee5503c6f65e909baa5a92948f26940c2765148faae6da607ba463e779e`), became Ready, and emitted no auth/`ImagePullBackOff` event. Argo auto-sync, old-node scheduling, and the interim SA fallback were restored after evidence capture; the workload-level secret remains on `eden-cms-v2`.
- Prod has one tenant namespace (`default`), four Zot-backed Deployments, no Zot-backed CronJobs/Jobs, and all four Deployments now carry `bex-registry-pull`. The default ServiceAccount still carries the interim fallback because the pending GitOps deletion cannot become durable until the change is shipped. After `$ship`: wait for the operator/GitOps rollout, confirm the backfill remains converged, confirm the ServiceAccount entry is absent, and repeat the namespace/workload sweep before closing tasks.

## Definition of done

A fresh tenant node (autoscale-up or roll) pulls an authed Zot image via the workload-attached `imagePullSecrets` alone: `kubectl get sa default -o yaml` in the apps namespace shows no hand patch, every operator-generated pod spec that pulls a tenant image carries the secret when `BEX_REGISTRY_PULL_SECRET` is set, and behavior with it unset is byte-identical to today (dev default). ADR022's described behavior is the implemented behavior.

## Source + Goal linkage

- **Source:** promotes `w7/001` (filed 2026-07-15 by the `w1/m36` tenant-pool roll incident: every git-built App went `ImagePullBackOff` on the fresh node — the images had only ever pulled because they were node-cached from before `w7/m8` enabled Zot auth; interim SA patch applied live = GitOps drift). Materialized under **w6** for capacity (topical owner w7 takes m33 this round; the w6 m19–m23 cross-placement precedent).
- **Goal linkage:** docs/ADR022-tenant-isolation.md §Registry access control — `PULL_SECRET` is specified as "attached to tenant Deployments/CronJobs as an `imagePullSecret`"; the operator just never did it.
- **Expected outcome:** the outage class is gone — autoscaler scale-ups, node failures/remediations, and future rolls pull images first-try; prod state is fully declared again.
- **Why now:** every fresh tenant node hits this today; only the undeclared hand patch is holding prod up, and the next cluster rebuild would silently lose it.
- **Render parity closing task: omitted** — operator-internal mechanism; no REST/GraphQL/MCP/UI surface change.
