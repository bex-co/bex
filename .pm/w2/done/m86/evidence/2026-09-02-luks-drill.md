# w2/m86 evidence — LUKS class verification and production mechanism drill (2026-09-02)

## Ground truth on the two recorded prerequisites (both resolved favorably)

**1. `cryptsetup` on the CAPH node image — NOT needed.** The hcloud-csi-driver
bundles it in its own image: v2.22.0's `Dockerfile` (`alpine:3.24`) runs
`apk add … cryptsetup …`, and `internal/volumes/cryptsetup.go` execs it inside
the node-plugin container. Confirmed in the running production plugin:

```text
$ kubectl exec -n kube-system hcloud-csi-node-9z6kx -c hcloud-csi-driver -- cryptsetup --version
cryptsetup 2.8.6 flags: UDEV BLKID KEYRING KERNEL_CAPI HW_OPAL
image: docker.io/hetznercloud/hcloud-csi-driver:v2.22.0
node:  bex-tenant-0 pool, Ubuntu 24.04.4 LTS, kernel 6.8.0-117-generic
```

The packer bake (`infra/packer/bex-worker.pkr.hcl`) is deliberately unchanged —
its trimmed apt set stays trimmed. The host only contributes the `dm_crypt`
kernel module, loaded by the host kernel's own usermode helper; the mount drill
below proves it live.

**2. hcloud-csi node SA reading tenant Secrets — wrong component, no grant
needed.** Per the CSI contract (kubernetes-csi secrets-and-credentials), the
external-provisioner resolves the class's `${pvc.name}-luks`/`${pvc.namespace}`
template into the PV's `nodePublishSecretRef`, and **the kubelet** fetches that
Secret (node-authorizer scoped to pods bound to it) and passes it in
`NodePublishVolumeRequest.secrets`. The CSI ServiceAccount never reads it. No
RBAC was added; the drill proves the read path.

**Resize on encrypted volumes:** the driver formats `--type luks1` and its
`resize.go` runs `cryptsetup resize` on the active mapping with **no
passphrase** (LUKS1 does not re-authenticate an open device), then grows the
filesystem. No node-expand secret is needed on the class.

## Production mechanism drill (scratch namespace `m86-luks-drill`)

Hand-minted Secret `disk-drill-luks` (key `encryption-passphrase`) + 10Gi PVC
`disk-drill` on `hcloud-volumes-luks` + busybox pod:

- Pod `Running`, PVC `Bound` in **32 s**; PV carried
  `nodePublishSecretRef: {name: disk-drill-luks, namespace: m86-luks-drill}`,
  volumeHandle `106770929`.
- Mount is the dm-crypt mapping: `/dev/mapper/scsi-0HC_Volume_106770929` (ext4),
  write/read of `/data/proof` OK.
- `cryptsetup status` in the node plugin: **LUKS1, aes-xts-plain64, 512-bit
  key, active**, device `/dev/sdb`.
- Grow 10Gi→11Gi applied **online in ~30–60 s**: PVC capacity 11Gi, LUKS
  mapping 20,967,424→23,064,576 sectors, ext4 11,244,744 KB — pod never
  restarted, data intact. (D4's finish-on-restart contract stays as the honest
  worst case; the maintainers still do not claim online expansion.)
- Cleanup: namespace deleted; PV gone (reclaimPolicy Delete removed Hetzner
  volume 106770929). Cost: ~1 h of an 11 GB volume (≈ €0.0001).

## Latent bug found and fixed before any tenant disk existed

The class template resolves the node-publish Secret as **`${pvc.name}-luks`**,
but `DiskLUKSSecretName` ran the shared truncation with the suffix inside the
length budget, shifting the cut point — for App names ≥54 chars the minted
Secret and the kubelet's lookup diverged, so every long-named App's encrypted
disk would have failed to mount (a missing-Secret mount error, invisible to the
tenant). Fixed by defining `DiskLUKSSecretName = DiskPVCName + "-luks"`
(`lego/types/v1alpha1/disknames.go`); pinned by
`TestDiskChildNamesStayWithinKubernetesLimits` and the new
`scripts/gitops-validate.sh` § "persistent-disk LUKS class contract" block
(template params, prod env ↔ class-name agreement, local-must-stay-unset).
Production had **zero** disk-bearing Apps at flip time (verified), so no legacy
names exist anywhere.

## The flip

`BEX_DISK_STORAGE_CLASS=hcloud-volumes-luks` is set in
`lego/operator/config/prod/kustomization.yaml` (the prod-only env overlay Argo
deploys; `config/default` stays unset so the CAPD mock keeps `local-path`).
Validator negative-tested: breaking the template or the prod env each fails
`gitops-validate.sh`. The env reaches the production operator on the next
`/ship`-driven deploy; the post-rollout check is: create a disk-bearing App,
confirm its PVC's class is `hcloud-volumes-luks` and its nightly snapshot runs.

---

# Part 2 — operator-path drill after rollout (same day, later)

The shipped env reached the production operator via CI + Argo (~12 h after the
push). The operator-path drill then ran in the bootstrap apps namespace
(`default` — the operator's codex-#4 guard rightly refused a scratch
namespace).

## Proven live, through the operator

- App `luksdrill` (`private_service`, tier `starter`, `spec.disk` 10 GB at
  `/var/data`) → **PVC `disk-luksdrill` Bound on `hcloud-volumes-luks`**,
  `DiskReady`, pod mounting `/dev/mapper/scsi-0HC_Volume_106777231` —
  the DoD's headline clause, via `BEX_DISK_STORAGE_CLASS`, no manual PVC.
- Deployment shape exact: `strategy=Recreate`,
  `cluster-autoscaler.kubernetes.io/safe-to-evict: "false"`.
- Backup CronJob `dskbak-luksdrill` created once the store was armed
  (schedule `0 2 * * *` UTC).
- App delete → full child cleanup: PVC, LUKS Secret, CronJob all gone.

## Four defects found (all fixed this session, none shipped-broken to tenants)

1. **`scripts/disk-snapshot-secret.sh` provision died on first-ever runs** —
   the age-keypair probe (`kubectl get secret | jq`) + `pipefail` aborted after
   minting the Wasabi IAM keys but before persisting them (secrets shown once,
   orphaned). Fixed with `|| true`; recovery = delete keys in Wasabi IAM,
   re-run.
2. **The manager's secretKeyRef contract was never implemented** — manager.yaml
   arms `BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY` from `bex-system/bex-disk-snapshot`,
   but the script installed that Secret into tenant namespaces only and without
   the age-recipient entry. On any manifest-armed cluster `DiskSnapshots` never
   reported configured ⇒ **no backup CronJob was ever created**. Script now
   installs the bex-system copy with all three entries; guard test added.
3. **Backup Job's ro claim source can't publish beside the running app on
   LUKS** — `MountVolume.SetUp failed … mount -o ro /dev/mapper/… Resource
   busy`: an ro superblock cannot coexist with the app's rw mount of the same
   mapper device. Fixed: claim source publishes rw, container bind stays ro
   (`disk_backup.go`), envtest-pinned.
4. **Snapshot/restore container ran as uid 65532 and died on the first real
   ext4 volume** — `open /disk/lost+found: permission denied` (root-owned
   0700; tenant files carry arbitrary uids). NOT LUKS-specific: latent for
   every disk on any class; masked on CAPD by 777 `local-path` hostPath dirs.
   Fixed: volume-touching Jobs run as root with exactly
   `DAC_OVERRIDE`/`CHOWN`/`FOWNER` (PSS-baseline-allowed), envtest-pinned.

Also armed and verified this session: bucket `bex-disk-snapshots` + both scoped
IAM identities (`verify`: all six separation checks PASS), Secrets in
bex-system + every `tea-*` namespace (+ a manual copy in `default`, which the
script's loop does not cover — filed as an inbox note along with the
no-projection gap for tenant namespaces created after a provision run).

## Observed while restarting bex-api (unrelated to disks, filed separately)

bex-api's new pods CrashLoop on startup validation:
`BEX_REQUIRE_PAYMENT_METHOD=all: BEX_STRIPE_PUBLISHABLE_KEY is required for
Stripe Payment Element` — prod bex-api rollouts have been wedged since the
billing-gate change landed (old pods keep serving; no outage). Needs the
publishable key added to the prod env chain — user-held value.

## Remaining for closeout (post-next-`/ship`)

The snapshot/restore fixes (3, 4) live in the operator image; after the next
deploy rolls it: create a disk-bearing App, trigger `dskbak-<app>`, confirm the
object lands, mutate, set `spec.disk.restoreSnapshot`, confirm pre-snapshot
state returns, delete, confirm purge — the runbook §Verifying end to end. Then
`/pm done w2/m86/t008`.

---

# Part 3 — final end-to-end drill on the fixed operator (2026-09-03)

Prerequisites resolved first: the bex-api rollout wedge (`w2/032`) was fixed by
patching the user-supplied `BEX_STRIPE_PUBLISHABLE_KEY` (pk_live, mode-checked)
into `bex-system/bex-stripe` — bex-api rolled 2/2 on the new digest and the
bex-operator Argo app went **Synced/Healthy** for the first time in days.

The fixed operator image (snapshot rw-publish + root/caps) was live via the
previous ship. One more platform defense fired, correctly: the
`bex-operator-workloads` ValidatingAdmissionPolicy denied the snapshot Job's
added capabilities (zero-caps rule outside the execution boundary). Resolved
with a **name-scoped carve-out** (`dskbak-`/`dskrst-` Jobs may add exactly
`DAC_OVERRIDE`/`CHOWN`/`FOWNER`; the `buildkitCaps` precedent) shipped as
`c3a0a1220` — a hand-applied preview worked, was then reverted by Argo within
minutes (GitOps custody confirmed the hard way), and the git-landed version
stuck. gitops-validate pins the carve-out's name anchor and capability list.

Full runbook §Verifying end to end, all on `hcloud-volumes-luks`, all through
the operator:

1. App `luksdrill2` → PVC Bound on the LUKS class, pod mounting
   `/dev/mapper/scsi-0HC_Volume_106781992`; `dskbak-luksdrill2` CronJob created.
2. `marker-v1-pre-snapshot` written; snapshot of the **running** app completed:
   `wrote default/luksdrill2/2026-09-03T07:50:40Z.tar.gz.age` — the exact case
   the rw-publish + root fixes exist for.
3. Mutated (`marker-v2`, added `file2`), set `spec.disk.restoreSnapshot` to the
   object key: operator scaled the service down, `dskrst-luksdrill2` ran and
   completed (operator deleted the succeeded Job), `disk-restored` annotation
   set, service returned — `/var/data/marker` = `marker-v1-pre-snapshot`,
   `file2` **gone**. Render's restore contract exactly.
4. App deleted → zero residue (purge ran; PVC, LUKS Secret, CronJob all gone).

One operational note: after the ~40 denial-loop failures, controller-runtime's
backoff delayed the post-fix retry by several minutes; an annotation nudge plus
patience covered it. The restore intent stayed durably on the CR the whole
time — fail-closed, visible, retryable, as designed.
