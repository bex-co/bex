# w2 · m86 — Disk encryption-at-rest: enable the LUKS storage class (ADR082 D3)

**Worker:** worker2 **Goal:** every tenant disk created after the flip provisions on the LUKS-encrypted storage class, closing the encryption-at-rest gap ADR082 explicitly left open **Status:** in progress (t001–t003, t005–t007 done; t004 open on its post-rollout operator-path leg; t008 gated on it)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Verify/bake `cryptsetup` into the CAPH node image — **DONE** (bundled in the csi-driver image; no bake needed) | 45m | —          |
| t002 | Verify/grant hcloud-csi node Secret access to per-disk LUKS passphrases — **DONE** (kubelet fetches it; no RBAC needed) | 30m | —          |
| t003 | Flip `BEX_DISK_STORAGE_CLASS=hcloud-volumes-luks` through the protected env chain — **DONE** (prod overlay `config/prod`; local stays unset) | 30m | t001, t002 |
| t004 | Live end-to-end: create, verify, resize, snapshot, and restore a LUKS-backed disk — mechanism drill PASSED on prod 2026-09-02; operator-path leg pending the next `/ship` rollout | 45m | t003       |
| t005 | Record the existing-disk stance and encryption claim in ADR082 + ADR018 — **DONE**         | 20m | t004       |
| t006 | Simplify — **DONE** (dead `suffix` param dropped, default render cached, yq reads batched, online-growth comment hedged) | 20m | t005       |
| t007 | Test coverage — **DONE** (naming-correspondence test + validator LUKS-contract block, both negative-tested) | 45m | t005       |
| t008 | Closeout                                                                                   | 15m | t006, t007 |

## Definition of done

- A newly created tenant disk provisions a PVC on `hcloud-volumes-luks`, and on-node inspection (`cryptsetup status` / `lsblk` on the backing volume) confirms an active LUKS mapping.
- Grow-only online resize works on the LUKS-backed disk, and the nightly snapshot plus the restore drill (`docs/runbooks/disk-snapshot-setup.md`) both pass against it.
- `BEX_DISK_STORAGE_CLASS` is set in the production deploy chain via the `.env.example` → `scripts/gh-secrets.sh` → workflow pattern; nothing secret is committed.
- ADR082 §D3 and the ADR018 disks row state exactly which disks are encrypted (created after the flip), that pre-flip disks remain on the default class, and the snapshot-restore migration path for them.
- If either live prereq fails (no `cryptsetup` on the node image, CSI ServiceAccount cannot read the passphrase Secret), the failure is diagnosed and fixed in this milestone — the flip does not ship with a broken mount path.

## Progress record (2026-09-02)

Both recorded prerequisites were verified live and both dissolved: `cryptsetup 2.8.6` ships **inside** the hcloud-csi-driver v2.22.0 image (packer bake deliberately untouched), and the passphrase Secret is fetched by the **kubelet** under the node authorizer, not the CSI ServiceAccount (no RBAC added). The production mechanism drill (scratch ns, PVC on `hcloud-volumes-luks`) proved format/mount (**LUKS1 aes-xts-plain64 512-bit**), per-PVC secret templating, online 10→11Gi growth through the encrypted mapping, and clean reclaim — evidence in `evidence/2026-09-02-luks-drill.md`. The drill surfaced and fixed a ship-blocking latent bug: `DiskLUKSSecretName` truncated differently than `DiskPVCName`, so the class's `${pvc.name}-luks` template would have missed the minted Secret for App names ≥54 chars — now defined as `DiskPVCName + "-luks"` (zero disk-bearing Apps existed, so no legacy names). The flip lives in the prod-only operator overlay (`lego/operator/config/prod/kustomization.yaml`) because the base manifest also serves the CAPD mock, where naming the class would strand every disk PVC. Guards: `TestDiskChildNamesStayWithinKubernetesLimits` pins the correspondence; `scripts/gitops-validate.sh` § "persistent-disk LUKS class contract" pins the class shape, prod env agreement, and local-must-stay-unset (all negative-tested). `make test`, `make lint`, backend/types builds, and `gitops-validate.sh` are green. **Remaining:** the env reaches the prod operator on the next `/ship`; then t004's operator-path leg (App → PVC on the LUKS class + one snapshot/restore) and t008 closeout.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-01 #1; ADR082 §D3 and the unchecked DoD box in `.pm/w1/done/m83/done/t004.md` — the mechanism (LUKS class + per-App passphrase minting, `deploy/gitops/base/disk-storageclass.yaml`, `diskLUKSSecretName` in `app_controller.go`) shipped OFF with two unverified live prereqs.
- **Goal linkage:** ADR082 persistent disks; the security lineage's tenant-data posture (ADR050 encrypts backups at rest — live volumes are the remaining gap).
- **Expected outcome:** the unencrypted disk estate stops growing; the encryption-at-rest parity claim ADR082 deliberately withheld becomes true for all post-flip disks.
- **Why now:** a PVC's storage class is immutable, so every disk created before the flip stays plaintext forever (or needs a snapshot-restore migration). Disks just went live for tenants (w1/m83–m88, done 2026-08-25) — flipping now keeps the legacy set near zero.
- **Render parity:** **omitted** — platform storage mechanism; no REST/GraphQL/MCP/dashboard contract changes (the docs claim update is t005).
