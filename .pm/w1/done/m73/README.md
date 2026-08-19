# w1 · m73 — Close the digest-pinning inventory for the images bex chooses

**Worker:** worker1 **Goal:** every container image the platform selects on a tenant's behalf resolves to a digest, so a retagged upstream image cannot become code that holds a tenant's password, credentials, or backup data. **Status:** done (2026-08-18)

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Pin the two remaining runtime images (tenant-version Valkey, redis_exporter) | 45m | — — **DONE** |
| t002 | Make the guard cover the whole class, not the sampled members | 45m | t001 — **DONE** |
| t003 | Re-scope what genuinely remains, in the register and the ADR  | 30m | t002 — **DONE** |
| t004 | Closeout                                                      | 15m | t002, t003 — **DONE** |

## Why now

`.pm/w1/046.md`'s supply-chain chunk (R5-F6) has been re-reported in **seven consecutive security rounds** (ADR055 F7 → round-13 #9 → ADR069's deferral). Most of it has since been closed one image at a time — buildkit, alpine/git, busybox, cosign, the AWS CLI uploader, and the default Valkey are all digest-pinned today. What is left is small enough to finish rather than re-report an eighth time.

## What is actually open (measured at `701f2a87`)

| reference | where | status |
| --- | --- | --- |
| `valkey/valkey:<version>-alpine` | `keyvalue_controller.go:valkeyImage` | **unpinned** when a tenant sets `spec.version` |
| `oliver006/redis_exporter:alpine` | `keyvalue_controller.go:kvExporterImage` | **unpinned**, a mutable tag |
| `apk add age` at container start | `keyvalue_backup.go`, `etcd-backup/cronjob.yaml`, `openbao-backup/cronjob.yaml` | not a pinning problem — see t003 |
| clusterctl / Helm / Argo / k3s / runc downloads | CI + provisioning (F13/F14/F15) | a different class — see t003 |

**The Valkey deferral rests on a claim that is not true.** `valkeyImage`'s comment says bex "cannot carry a pre-resolved digest for every major it has not seen" — but `KeyValueSpec.Version` carries `+kubebuilder:validation:Enum="7";"8"`, so the set is closed, finite, and known at compile time. Two digests close it.

## Definition of done

- Every image bex selects for a tenant workload resolves by digest, including the explicit-version Valkey path and the metrics sidecar.
- The guard asserts the **class**, not a sample: adding a version to the CRD enum without adding its digest fails a test.
- An unpinnable reference (a version with no recorded digest) falls back to a pinned image rather than a mutable tag — it never silently runs unpinned.
- What remains open is recorded honestly, with the reason it is not a pinning change.

## Source + Goal linkage

- **Source:** `.pm/w1/046.md` R5-F6, promoted 2026-08-18 by user direction during the w1 triage.
- **Goal linkage:** ADR022/ADR050's tenant-isolation posture — the sidecar shares the tenant's pod and its password; the snapshot stage mounts plaintext backup data.
- **Expected outcome:** the seven-round re-report ends, and what is left is a packaging decision rather than an unpinned image.

## What shipped

**Two images pinned.** `valkey/valkey:7-alpine` → `sha256:211d9cb0…` and `oliver006/redis_exporter:alpine` → `sha256:d4e0a0ad…`, both resolved from the registry rather than copied from a scan report. `valkey:8-alpine` resolved to exactly the digest already pinned as the default, which is a useful cross-check that the method is sound.

**The deferral's premise was wrong, and that is the finding.** `valkeyImage`'s comment justified leaving the explicit-version path unpinned because bex "cannot carry a pre-resolved digest for every major it has not seen". `KeyValueSpec.Version` has carried `+kubebuilder:validation:Enum="7";"8"` the whole time, so bex can only ever see two. Seven rounds re-reported this class; the part that survived was protected by a rationale nobody re-checked against the CRD.

**The guard now covers the class, not the members.** `TestValkeyImagesArePinnedForEveryPermittedVersion` parses the kubebuilder enum marker off `KeyValueSpec.Version` — the same line the generated schema is built from — and requires a pinned digest for each value, with the fix command in the failure message. Adding a major to the enum without its digest fails CI. `TestValkeyImageNeverComposesAMutableTag` pins the fallback behaviour: an unrecognized version (unreachable through the CRD) resolves to the pinned default rather than a composed tag.

**One test's expectation was inverted, deliberately.** `TestKeyValueBackupFixedImagesAreDigestPinned` encoded the exemption — `explicit-version` was asserted **un**pinned. It now asserts both enum majors pinned, and its comment records why the exemption is gone rather than leaving a reader to wonder.

**What remains, and why it is not this milestone.** `apk add age` at container start (KeyValue backup, etcd-backup, openbao-backup) resolves code from a mutable package index — the fix is a published image carrying `age` (ADR060 §D7), a packaging change, not a pin. The CI/provisioning download class (clusterctl, remote Helm charts, the Argo install manifest, `get.k3s.io`, runc/containerd/runsc checked against same-origin checksums) is a different mechanism again. Both are recorded in `046.md` and in ADR069's deferral, which now says which half closed.
