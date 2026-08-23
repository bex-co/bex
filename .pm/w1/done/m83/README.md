# w1 · m83 — Persistent disks 1/4: CRD contract + operator mechanism

**Worker:** worker1 **Goal:** an App with `spec.disk` runs as a single-instance `Recreate` Deployment with a LUKS-encrypted Hetzner-volume PVC mounted at `mountPath`, grow-only resizable, surviving suspend/resume — the ADR082 D2–D4 mechanism, end to end in types + operator. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on             |
| ---- | --------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | `DiskSpec` on the App CRD: fields, defaults, CEL, codegen — **DONE**         | 45m | —                      |
| t002 | Operator: per-disk PVC + `Recreate` + mount + safe-to-evict — **DONE**       | 1h  | t001                   |
| t003 | RBAC markers + admission-policy PVC CREATE/DELETE rules — **DONE**           | 30m | t002                   |
| t004 | LUKS StorageClass + per-disk passphrase Secret — **DONE**                    | 45m | t002                   |
| t005 | Grow-only resize path + disk status conditions — **DONE**                    | 45m | t002                   |
| t006 | Suspend/resume + deletion lifecycle + stale-disk cleanup — **DONE**          | 30m | t002                   |
| t007 | Simplify pass over the changed operator/types code — **DONE**                | 30m | t003, t004, t005, t006 |
| t008 | Test coverage: envtest for CEL + reconcile + resize + delete — **DONE**      | 1h  | t007                   |
| t009 | Closeout — **DONE**                                                         | 15m | t008                   |

## Outcome notes

- **Two bugs the tests caught, both in cleanup.** The first cut skipped disk cleanup when `status.disk` was nil — but an `Update()` round-trip discards in-memory status, so a detach could leave a PVC (and its Hetzner bill) orphaned forever. The second cut cleaned up unconditionally, which made every stateless App's reconcile do a cached PVC read and so started a cluster-wide PVC informer — that broke `secret_cache_test.go`'s restricted-permission manager. The shipped design is a durable `app.bex.co/disk-provisioned` annotation written *before* the first child exists, plus blind deletes: no informer, no orphan, and idempotent if a delete fails halfway.
- **Deploy semantics are field-partial.** The `TestAppSpecIdentityClassificationIsExhaustive` guard forced the decision: `Disk` is a release input (attach/detach/remount rewrites the pod template, as on Render), but its **size is deliberately excluded** from the release fingerprint, so a grow is applied online instead of redeploying the service. Pinned by `TestDiskGrowIsNotARelease`.
- **Simplify pass produced a real consolidation:** the PVC-expansion preconditions (bound → class present → class found → expandable → patch, with quota-Forbidden backoff) existed once in the KeyValue expander and were about to exist twice. They are now one `expandPVCTo` in `storage.go` returning an abstract outcome each caller maps to its own vocabulary; KeyValue's messages and requeues are unchanged and its tests pass untouched.
- **LUKS is shipped as opt-in, not on.** `deploy/gitops/base/disk-storageclass.yaml` defines `hcloud-volumes-luks` with per-disk passphrase templating, and the operator mints the passphrase Secret for every disk, but `BEX_DISK_STORAGE_CLASS` ships **unset** (cluster default class). **Not verified, and not verifiable from here:** that `cryptsetup` is present on the CAPH node image, and that the hcloud-csi node ServiceAccount can read Secrets in tenant namespaces. Both are recorded in the class manifest; pointing the operator at that class before checking them would fail every disk mount, not just its encryption. Encryption-at-rest parity is therefore **not yet claimed** — carry this into the ADR082 record when m86 closes it out.

## Definition of done

On a cluster with an expandable StorageClass: applying an App CR with `spec.disk {name, mountPath, sizeGB}` produces a `Recreate` Deployment (replicas ≤ 1) mounting PVC `disk-<app>` at `mountPath` with the `safe-to-evict: "false"` annotation; CEL rejects disks on cron/static/free-tier/multi-replica/autoscaled Apps and any `sizeGB` shrink; growing `sizeGB` expands the PVC (condition `DiskResizePending` until observed); `suspended: true` scales to 0 with the PVC intact; removing `spec.disk` or deleting the App removes the PVC. All asserted by envtest; `make test` + `make lint` green. Disk-less Apps produce byte-identical objects to before this milestone. — **Met**, with these specifics:

- Deployment shape **met** and asserted twice: on the pure projection and through `applyDeploymentSpec` itself, so the wiring (and the secret-files mount surviving beside the disk) is pinned, not assumed.
- CEL refusals **met** — one `DescribeTable` per refusal against the real API server (cron, static, free tier, 2 instances, autoscaling, relative path, root, two reserved paths, over-max size), plus a subdirectory of a reserved path accepted and a live grow-then-shrink round trip.
- Grow **met**, including the two blocked states (`StorageBlockedByQuota`, `StorageClassNotExpandable`) staying green reconciles rather than errors, and `DiskResizePending` while the filesystem trails.
- Suspend/detach **met**. Detach asserts the delete was *issued* rather than the object being gone: envtest runs the API server's `StorageObjectInUseProtection` admission but not the controller-manager that clears the finalizer, so a deleted PVC lingers Terminating there.
- App-deletion cleanup is by owner reference (asserted present on both children); the GC itself is Kubernetes' guarantee and does not run in envtest.
- `make test` (124 specs + unit) and `make lint` (all four modules) green; `lego/backend` still builds against the changed CRD types.
- **Not met, and deliberately not claimed: encryption at rest.** See the LUKS note above — the class exists and the passphrases are minted, but the operator does not point at it until the node prerequisites are verified on a live cluster.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D2–D4, D11 stage 1), evidence in [docs/render-artifacts/disks.md](../../../docs/render-artifacts/disks.md); the DO_NOT_DO persistent-disk anti-goal was re-opened by explicit user decision 2026-08-22 (re-open record in [.pm/DO_NOT_DO.md](../../DO_NOT_DO.md)).
- **Goal linkage:** Render parity (ADR018) — converts the parity ledger's largest deliberate gap into a shipping feature; unlocks the stateful `render.yaml` workloads (n8n, WordPress, self-managed DBs) the Blueprint compiler hard-refuses today.
- **Expected outcome:** the platform mechanism for service disks exists and is envtest-proven, so the API/billing (m84), snapshots (m85), and Blueprint/dashboard (m86) stages have a substrate to build on.
- **Why now:** first stage of ADR082's D11 rollout order; every later disk milestone depends on the CRD contract and reconcile behavior landing first.
- **Render parity closing task omitted:** this milestone is operator-internal mechanism — no REST/GraphQL/MCP/dashboard surface changes; the wire surface lands in m84, which carries the parity task.
