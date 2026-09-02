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
