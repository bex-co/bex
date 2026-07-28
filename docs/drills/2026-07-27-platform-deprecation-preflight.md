# Platform deprecation preflight — 2026-07-27

**Observed at:** 2026-07-28T02:41:35Z (2026-07-27 America/Los_Angeles)

**Scope:** w1/m56 t001, production application cluster plus the maintained local CAPD cluster

**Method:** read-only Kubernetes, CAPI, Hetzner Cloud, repository, and backup metadata inspection. Kubeconfigs, Secret data, tokens, database credentials, and private keys were not recorded.

## Decision

The milestone may proceed, but t002 is **NO-GO** until the CAPI controller placement failure below is corrected and `scripts/verify-substrate.sh` passes. All other discovered non-zero legacy state has a named normalization step before its fallback is removed.

The supported Kubernetes target is **v1.34**, reached in the mandatory sequence **v1.31 → v1.32 → v1.33 → v1.34**. This is the smallest supported destination shared by the deployed platform components, limiting the rollout to three one-minor transitions.

## Version and compatibility inventory

| Component | Production | Target/decision | Compatibility evidence |
| --- | --- | --- | --- |
| Kubernetes | API server, 10 Machines, and 10 Ready Nodes at v1.31.0 | v1.34.x, one minor at a time | [Kubernetes releases](https://kubernetes.io/releases/) lists 1.34 as supported and 1.31 as end-of-life since 2025-11-11. |
| Cluster API | v1.13.3 | keep within supported v1.13 patch line during the fleet roll | [CAPI v1.13.4](https://github.com/kubernetes-sigs/cluster-api/releases/tag/v1.13.4) supports management clusters 1.32–1.36 and workload clusters 1.30–1.36. |
| CAPH | v1.1.7 | keep | [CAPH v1.1.7](https://github.com/syself/cluster-api-provider-hetzner/releases/tag/v1.1.7) supports CAPI 1.10–1.13 and Kubernetes 1.32–1.36; it does not support the current 1.31 fleet. |
| CloudNativePG | 1.30.0 | keep until plugin migration and restore drills complete | [CNPG 1.30](https://cloudnative-pg.io/docs/1.30/release_notes/v1.30/) supports Kubernetes 1.34–1.36. It is the migration window before native Barman removal in CNPG 1.31. |
| Barman Cloud plugin | absent | install v0.13.0 before switching either backup path | [Plugin installation](https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/) identifies v0.13.0 as the current release; follow the [native-to-plugin migration](https://cloudnative-pg.io/plugin-barman-cloud/docs/migration/) atomically. |
| kpack | v0.17.2 with `KUBERNETES_MIN_VERSION=1.31.0` override | v0.18.0 after the v1.34 fleet roll; remove override | Official `release-0.18.0.yaml` SHA-256: `cde8b7df8d31d6a5758ec4880eec45009f17811baf3df5a29b76a144fe200e69`; upstream no longer declares the override. |
| Cilium | v1.19.5 | keep for v1.34 | [Cilium 1.19 requirements](https://docs.cilium.io/en/stable/network/kubernetes/requirements/) guarantee Kubernetes 1.31–1.34 compatibility. |
| cert-manager | v1.20.3 | keep for v1.34 | [cert-manager supported releases](https://cert-manager.io/docs/releases/) lists Kubernetes 1.32–1.35 for 1.20. |
| Traefik | v3.7.5 | verify on every hop | Kubernetes API integration has no narrower version gate in the deployed configuration; routing smoke tests remain mandatory. |

Repository Kubernetes 1.31 pins exist in the production CAPI template, worker Packer default, snapshot workflow default, and etcd-backup documentation. t002 must update every provisioning pin, not only the live CAPI objects.

GitHub Actions inventory before t012:

- `actions/checkout@v4` (14), `actions/setup-go@v5` (4), and `actions/setup-node@v4` (2) are the known Node 20-era majors.
- Other actions: `actions/cache@v4` (2), `anchore/sbom-action/download-syft@v0` (1), `aquasecurity/trivy-action@v0.36.0` (4), `azure/setup-helm@v4` (1), `azure/setup-kubectl@v4` (1), Docker actions (4), `gitleaks/gitleaks-action@v2` (1), HashiCorp setup actions (2), and `sigstore/cosign-installer@v3` (1). t012 must inspect every action's declared runtime and update maintained majors rather than assuming only the three known entries need changes.

## Production state

### Fleet and placement

- KubeadmControlPlane `default/bex-control-plane`: 3/3 available, v1.31.0.
- 10/10 Machines are `Running`, have NodeRefs, and request v1.31.0.
- 10/10 Nodes are Ready: three control-plane, three platform, and four tenant nodes. Autoscaler activity observed during the first sample settled before the final count.
- `scripts/verify-substrate.sh` passes access, self-management, and fleet shape, then fails placement: `capi-controller-manager` has no node selector and runs on a tenant node. t002 must pin it to a control-plane node, keep its existing control-plane tolerations, wait for Ready, and obtain a completely green substrate verification before changing a Kubernetes version.

### Legacy-state counts

| State | Production | Maintained local cluster | Normalization gate |
| --- | --: | --: | --- |
| Database objects missing canonical `spec.name` | 0 | 0 | Recount immediately before t009 fallback deletion. |
| Database flat/legacy CIDR entries | 0 | 0 | Recount immediately before t009 fallback deletion. |
| KeyValue objects missing canonical `spec.name` | 0 | 0 | Recount immediately before t009 fallback deletion. |
| KeyValue flat/legacy CIDR entries | 0 | 0 | Recount immediately before t009 fallback deletion. |
| Active Apps missing release fingerprint | 0 | 0 | Recount before t010 code removal and prove new/rebuilt Apps write it at creation. |
| Active Apps missing artifact fingerprint | 0 | 0 | Same gate as release fingerprints. |
| Current `bex-build` artifacts missing App UID | 0 of 16 | 0 | Artifact-specific Jobs, kpack Images/Builds, clone Secrets, ServiceAccounts, and NetworkPolicies must remain zero. |
| Old clone Secret copies in `bex-system` missing App UID | 28 | 0 | t010: recheck for `bex-system` Pod references, delete all unreferenced copies, then require zero before removing adoption code. Same-name references found in other namespaces are namespace-local and do not reference these copies. |
| Legacy datastore `IngressRouteTCP` objects | 0 | 0 | Recount cluster-wide before t011 cleanup removal. |
| Legacy datastore `MiddlewareTCP` objects | 0 | 0 | Recount cluster-wide before t011 cleanup removal. |
| Edge LB explicit per-server targets (`bex-traefik`) | 0 | n/a | Retain the private label selector and recount before t011. |

The three/four generic workload Pods without an App UID in local/production are ordinary Deployment Pods, not cross-namespace build artifacts. The 36 generic production Secrets without an App UID consist of the 28 old clone copies above plus eight source/runtime/default Secrets outside the artifact-adoption scope. The three Kubernetes API load balancers each intentionally target their three control-plane servers; t011 must not treat those control-plane targets as legacy edge targets.

## Rollback checkpoints

| Data | Last successful evidence at inspection | Redacted location | Gate before mutation |
| --- | --- | --- | --- |
| Application-cluster etcd | CronJob success at 2026-07-27T03:15:17Z | `s3://bex-tfstate/etcd-snapshots/` | Run and verify a fresh snapshot before the first control-plane hop. Preserve the pre-hop object identity in the transition record. |
| OpenBao | CronJob success at 2026-07-27T03:45:11Z | `s3://bex-tfstate/openbao-snapshots/` | Require a fresh successful snapshot before fleet mutation because tenant backup credentials depend on OpenBao. |
| bex-db | CNPG base backup completed at 2026-07-28T02:04:05Z; continuous native WAL archive configured | `s3://bex-tfstate/bex-db/bex-db/` | Keep native configuration untouched until plugin ObjectStore is Ready, a fresh plugin backup completes, and the non-destructive plugin PITR drill passes. |
| Tenant Postgres | Existing native Barman destinations are generated per Database under the configured Postgres backup prefix | configured `BEX_DB_BACKUP_DESTINATION`, per-cluster suffix; credential-free value only | Select a live backed-up tenant, record its redacted prefix, create known data, take a fresh plugin backup, and prove non-destructive PITR before removing native fields. |

At inspection, `bex-db` was 2/2 Ready, `bex-db-nightly` retained its `0 4 * * *` schedule, and 169 of 170 retained Backup objects were completed. Hydra, Kratos, and OpenFGA CNPG clusters were each 2/2 Ready but intentionally had no backup configuration in this milestone's current scope.

## Per-task go/no-go gates

1. **t002 — fleet:** NO-GO until CAPI placement is fixed and full substrate verification passes. Take fresh etcd/OpenBao snapshots. Roll v1.32, verify control plane/workers/Cilium/storage/workloads and product smoke tests; repeat independently for v1.33 and v1.34. Never skip a minor. Record rollback evidence after each hop.
2. **t003 — kpack:** start only when every live node and CAPI template reports v1.34. Vendor the checksummed v0.18.0 release, remove the compatibility override, require controller/webhook Ready, then prove source build, publish, deploy, cancel, and newest-wins behavior.
3. **t004 — plugin/ObjectStore:** install v0.13.0 and require plugin Ready before declaring ObjectStores. Resolve the existing destinations and Secret references without reading Secret values. Missing/mismatched references must fail closed.
4. **t005 — tenant migration:** switch backup, ScheduledBackup, and recovery references atomically for newly reconciled backed-up Databases. Do not delete native recovery support yet.
5. **t006 — bex-db migration:** render plugin-only bex-db and ScheduledBackup configuration against the existing prefix/schedule/retention, but keep a rollback commit and do not advance to native-code deletion.
6. **t007 — restore proof:** take fresh plugin backups for one tenant and bex-db; restore each to throwaway clusters at recorded PITR targets; verify known rows, source health, cleanup, versions, timestamps, and object prefixes without credentials.
7. **t008 — native removal:** GO only after both t007 drills pass. Then remove all active native `barmanObjectStore` fields, code, validation, alerts, and current runbook instructions; historical evidence remains intact.
8. **t009 — datastore normalization:** a second production/local audit must still show zero legacy names and CIDR shapes. Exercise canonical lifecycle and allowlist behavior before deleting fallback branches and scripts.
9. **t010 — metadata normalization:** first delete the 28 old, unreferenced `bex-system` clone Secret copies and confirm zero artifact-specific missing-UID resources and zero missing fingerprints. Prove fresh/rebuilt creation writes canonical metadata before deleting adoption/backfill code.
10. **t011 — routes/load balancers:** a second audit must show zero legacy datastore routes and zero explicit targets on `bex-traefik`. Preserve the edge label selector and the intentional Kubernetes API control-plane targets, then remove only the recurring legacy cleanup logic.

Every mutation retains its pre-change Git commit as the manifest rollback point. Production objects are changed only by committed declarative configuration or a documented one-time normalization with before/after counts.

## Fleet transition evidence

### Kubernetes 1.31 → 1.32.13

- Rollback point: the pre-hop etcd and OpenBao Jobs `m56-preupgrade-etcd-20260728-0249` and `m56-preupgrade-openbao-20260728-0249` both completed successfully. The manifest rollback point before the version change is commit `973efa7c` after the control-plane capacity correction; each subsequently staged pool has its own preceding commit.
- Worker artifact: the manually dispatched [snapshot workflow](https://github.com/bex-co/bex/actions/runs/30324484063) produced exactly one active `bex-worker-k8s-1-32` image with Kubernetes 1.32.13, containerd 2.3.3, and runc 1.5.1. Older artifacts retained different selectors for rollback.
- Control plane: KCP rolled one member at a time onto the immutable `bex-control-plane-cpx32` template after Hetzner rejected new `cx33` capacity in `fsn1`. Two failed Machines without NodeRefs were deleted only after confirming they never represented live Nodes. The final three Machines were Running and Ready at v1.32.13 with stacked etcd `3.5.24-0` and `RollingOut=False`.
- Workers: burst canary, tenant baseline, and platform pools were rolled separately. The successful reconciliations are the [burst](https://github.com/bex-co/bex/actions/runs/30325874538), [baseline](https://github.com/bex-co/bex/actions/runs/30326054428), and [platform](https://github.com/bex-co/bex/actions/runs/30326233648) workflow runs. The final fleet was 3 control-plane, 3 platform, and 3 tenant Machines, all Running with NodeRefs and Ready Nodes at v1.32.13.
- Stateful handling: OpenBao's PodDisruptionBudget held each platform drain while the rescheduled Raft member was sealed. Each moved member was unsealed with the existing runbook before the next Machine could drain; the final cluster was 3/3 Ready, unsealed, and on distinct platform Nodes. Four platform CNPG clusters remained fully Ready.
- Verification: `scripts/verify-substrate.sh` passed all access, self-management, shape, controller placement, OpenBao/CNPG data, scheduler, CSR, network/firewall, autoscaler, and no-pet checks. The isolated `BEX_SSH_KNOWN_HOSTS_FILE` prevented immutable control-plane key rotation from modifying a user's persistent SSH trust database.
- Post-hop recovery checkpoint: `m56-k132-etcd-20260728-0351` completed at `2026-07-28T03:52:02Z`; `m56-k132-openbao-20260728-0351` completed at `2026-07-28T03:51:52Z`. Both reported one successful completion. The etcd backup Job used the restore-capable 3.6 client against the live kubeadm etcd 3.5.24 cluster and therefore exercised the actual cross-version snapshot path.

### Kubernetes 1.32.13 → 1.33.13

- Rollback point: the manifest immediately before the hop was commit `24ac73a8`; control-plane staging, compatibility repair, and each worker-pool transition remained separate commits. The 1.32 post-hop Jobs above were the data rollback point.
- Worker artifact: the manually dispatched [snapshot workflow](https://github.com/bex-co/bex/actions/runs/30327279482) produced snapshot `413474279`, the sole active `bex-worker-k8s-1-33` selector, with Kubernetes 1.33.13, containerd 2.3.3, and runc 1.5.1.
- Control plane: the first replacement exposed Kubernetes 1.33's removal of the deprecated kube-apiserver `--cloud-provider` flag. Machine `bex-control-plane-xb9g8` never acquired a NodeRef and was deleted only after its live log showed `unknown flag: --cloud-provider`; commit `ce4d9bc9` removed the API-server-only flag while retaining external CCM configuration and added a structural validation guard. The corrected [control-plane workflow](https://github.com/bex-co/bex/actions/runs/30327677109) converged three Running and Ready Machines at v1.33.13 with stacked etcd `3.5.24-0` and `RollingOut=False`.
- Workers: the [burst canary](https://github.com/bex-co/bex/actions/runs/30328343445) converged first. Hetzner then rejected both attempted tenant-baseline `cx33` replacements with `resource_unavailable`; neither failed Machine had a provider ID or NodeRef. The active baseline and platform references were moved to new immutable `cpx32` templates, preserving x86 and the 4-vCPU/8-GB capacity shape. The final [baseline](https://github.com/bex-co/bex/actions/runs/30328899880) and [platform](https://github.com/bex-co/bex/actions/runs/30329157793) workflows succeeded. The fleet ended at 3 control-plane, 3 platform, and 3 tenant Machines, all Running with NodeRefs and Ready Nodes at v1.33.13.
- Stateful handling: the OpenBao PDB again held each platform drain while the rescheduled RWO volume attached and the moved member was sealed. Each of the three members was unsealed and made Ready before CAPI drained the next Machine. The final OpenBao cluster was 3/3 Ready and unsealed on distinct platform Nodes; Hydra, Kratos, OpenFGA, and bex-db CNPG clusters remained 2/2 Ready throughout.
- Verification: `scripts/verify-substrate.sh` passed all access, self-management, shape, controller placement, OpenBao/CNPG data, scheduler, CSR, network/firewall, autoscaler, and no-pet checks. The verification observed nine Running Machines and nine Ready Nodes, all at their staged desired versions, and used an isolated known-hosts file.
- Post-hop recovery checkpoint: `m56-k133-etcd-20260728-0446` completed at `2026-07-28T04:47:12Z`; `m56-k133-openbao-20260728-0446` completed at `2026-07-28T04:47:08Z`. Both reported one successful completion and their logs confirmed object-store uploads. The restore-capable etcd 3.6.8 client successfully snapshotted the live stacked etcd 3.5.24 cluster.

### Kubernetes 1.33.13 → 1.34.9

- Rollback point: the 1.33 post-hop recovery Jobs above were the data rollback point. Commit `926e690f` staged the v1.34.9 control plane; the burst, baseline, and platform pools were then staged independently by commits `e7ba9c9f`, `8130e85b`, and `83c5d3b9`.
- Worker artifact: the manually dispatched [snapshot workflow](https://github.com/bex-co/bex/actions/runs/30329774078) produced snapshot `413475166`, the sole active `bex-worker-k8s-1-34` selector, with Kubernetes 1.34.9, containerd 2.3.3, and runc 1.5.1.
- Control plane: CAPI replaced the three members one at a time and converged v1.34.9 with stacked etcd `3.6.5-0`. The first [application-cluster workflow](https://github.com/bex-co/bex/actions/runs/30329877874) observed the healthy rollout but failed when one transient API-server timeout terminated the rollout waiter. Commit `e07a95e1` made bounded CAPI status reads retryable, and commit `093e95d1` added the waiter to that workflow's path filter. The corrected [control-plane workflow](https://github.com/bex-co/bex/actions/runs/30330543032) passed with three Running Machines, three Ready Nodes, and KCP `Available=True` and `RollingOut=False`.
- Workers: the [burst canary](https://github.com/bex-co/bex/actions/runs/30330652637), [tenant baseline](https://github.com/bex-co/bex/actions/runs/30331011016), and [platform](https://github.com/bex-co/bex/actions/runs/30331241922) workflows all succeeded in sequence. The final fleet has three control-plane, three platform, and three tenant Machines; all nine are Running with NodeRefs and all nine Nodes are Ready at v1.34.9.
- Stateful handling: the OpenBao PodDisruptionBudget serialized the three platform replacements. Each moved Raft member was unsealed and returned to Ready before the next replacement proceeded. The final OpenBao cluster is 3/3 Ready and unsealed on distinct platform Nodes; Hydra, Kratos, OpenFGA, and bex-db CNPG clusters are each 2/2 Ready.
- Verification: `scripts/verify-substrate.sh` passed all access, self-management, shape, controller placement, OpenBao/CNPG data, scheduler, CSR, network/firewall, autoscaler, and no-pet checks. A reused control-plane address produced an expected stale-key warning only inside the disposable known-hosts file; verification selected another healthy control-plane endpoint and never changed the user's persistent SSH trust database.
- Post-hop recovery checkpoint: `m56-k134-etcd-20260728-0531` completed at `2026-07-28T05:32:02Z`; `m56-k134-openbao-20260728-0531` completed at `2026-07-28T05:31:59Z`. Both reported one successful completion and their logs confirmed uploads to their redacted object-store prefixes. The restore-capable etcd `3.6.8-0` backup client successfully snapshotted the live stacked etcd `3.6.5-0` cluster.

### Final product acceptance on v1.34.9

- Browser/API build path: the first uniquely named `hello-go` lifecycle exposed a real per-App Zot authorization race. Commit `044f96f7` now requires the repository-scoped registry credential to be active before creating any source-build Job; the full operator test suite passed and deployment run [30332395484](https://github.com/bex-co/bex/actions/runs/30332395484) promoted the fix. The rerun (`srv-d9k48t6smouc73ajjpg0`) visibly waited in `RegistryCredsPending`, then built and served the unique token, suspended to three sustained non-serving probes, resumed with the same token, deleted, and left no App, Pod, pull Secret, or Zot htpasswd identity.
- Elastic build capacity: that rerun also exposed the old Cluster Autoscaler v1.31.5 treating Cilium's transient `node.cilium.io/agent-not-ready` taint as permanent in a zero-sized node template. Commit `7260c8a8` moved CA to the matching v1.34.5 and declared the taint with `--startup-taint`; CI now rejects CA/workload-minor drift or loss of the declaration. Production then scaled `bex-tenant-burst` from 0 to 2 without a manual replica change, created two v1.34.9 Ready Nodes, and scheduled both waiting builds.
- Managed Postgres: the unique database `dpg-d9k4ejmsmouc73ajjphg` exposed a namespace-wide Cilium egress-enforcement gap in the CNPG init path. Commit `741c0492` added an identity-scoped, API-only CNPG allow and an exact GitOps guard. The same live database then reached Ready, accepted a SQL write, hibernated to zero compute Pods through the official CLI, resumed with the row intact, and was deleted with no Database CR, CNPG Cluster, credential Secret, or Pod residue.
- Final state: all control-plane and worker templates remain v1.34.9; the running autoscaler is v1.34.5; `scripts/verify-substrate.sh`, App lifecycle, Postgres lifecycle/SQL, recovery Jobs, Cilium, storage, OpenBao, and platform CNPG checks are green. The only remaining `1.31.0` production value is kpack v0.17.2's temporary minimum-version compatibility override, intentionally owned and removed with t003 rather than a Kubernetes fleet/provisioning pin.

### Build and release metadata normalization

- Before code removal, production and maintained-local inventories reported zero current build artifacts missing `app.bex.co/app-uid` and zero active releases missing fingerprints. The 28 old `bex-system` clone Secret copies were rechecked against namespace-local Pod references, found unreferenced, deleted by exact name, and recounted at zero. Current source/runtime Secrets outside the cross-namespace artifact contract were not changed.
- Commit `a82f3a9e` removed timestamp/name-based artifact adoption, UID-less reclaim, and release-fingerprint backfill. Build, pre-deploy, publish, and finalizer paths now require an exact App UID; creation-shape tests cover every supported execution-artifact kind and Job pod template. Deployment run [30353112208](https://github.com/bex-co/bex/actions/runs/30353112208) passed all suites and rolled the change to production.
- The official Render CLI v2.21.0 created Dockerfile-backed service `srv-d9k955v5ic4c73frfd8g`. Its initial build and explicit rebuild completed at release generations 1 and 2 with non-empty artifact and release fingerprints. Both generation Jobs, their pod templates, build Pods, copied clone Secret, and build-registry Secret carried the exact App UID from creation; no adoption pass was observed.
- Exact-ID deletion removed that service's App CR and every scoped artifact. A preceding failed native-runtime probe also left zero residue after exact-ID deletion. The final production/local audit found zero artifact-specific missing-UID objects, zero old `bex-system` clone copies, and zero kpack artifacts missing UID; every non-failed production App retained canonical fingerprints.

### Datastore route and edge-target cleanup retirement

- Immediately before and after removal, cluster-wide production and maintained-local inventories each contained only the current `bex-system/bex-ssh-gateway` `IngressRouteTCP`; legacy datastore `IngressRouteTCP` and `MiddlewareTCP` counts were zero.
- The production `bex-traefik` Load Balancer had zero explicit server targets and exactly one private `caph-cluster-bex=owned,machine_type=worker` selector. It resolved six workers and retained listeners `22,80,443,5432,6379`, each with at least one healthy selector-resolved backend. Intentional Kubernetes API load-balancer targets were not changed.
- Commit `5c9a7d4b` removed recurring Database/Key Value Traefik deletion branches and migration-only tests. The infrastructure workflow's `remove_target` mutation loop was replaced by a bounded read-only assertion over exact LB count, target shape, listener set, and per-listener health; local GitOps validation rejects regression to the mutation loop.
- [Terraform run 30355567062](https://github.com/bex-co/bex/actions/runs/30355567062) applied with no target cleanup and passed the new live check. [Operator run 30355566342](https://github.com/bex-co/bex/actions/runs/30355566342), [GitOps run 30355566794](https://github.com/bex-co/bex/actions/runs/30355566794), and [deployment run 30355567026](https://github.com/bex-co/bex/actions/runs/30355567026) were green. After rollout, the controller and both SNI-proxy DaemonSets were Ready and the route/LB inventories remained canonical and healthy.
