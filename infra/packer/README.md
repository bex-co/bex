# bex node image (baked Hetzner snapshot)

`bex-worker.pkr.hcl` bakes a Hetzner Cloud snapshot that carries **runc, containerd, and kubelet/kubeadm/kubectl** preinstalled at pinned versions. Autoscaled workers join without downloading privileged runtimes on the scale-up hot path, and replacement control-plane nodes consume the same reviewed bytes instead of executing artifacts selected from live aliases or co-fetched checksums.

- **~150 MB** of runc + containerd + kubelet downloads → baked in.
- **~200 MB** of `kubeadm config images pull` (control-plane images a worker never runs) → dropped entirely (w1/m36 t002).
- apt list trimmed to the packages provisioning actually uses (w1/m36 t003).

What stays in cloud-init (the `KubeadmConfigTemplate`/`KubeadmControlPlane` `files:` + `preKubeadmCommands` in [`../clusterapi/overlays/hetzner-caph/cluster.yaml`](../clusterapi/overlays/hetzner-caph/cluster.yaml)): containerd's service unit + `config.toml`, the tenant-only zot `certs.d`, sysctls/modules, and `systemctl enable kubelet`. The image is **just binaries + packages**, so one versioned snapshot serves control-plane, tenant, and platform nodes while pool-specific runtime configuration remains in CAPI.

## How CAPH picks the image

CAPH resolves `HCloudMachineTemplate.spec.template.spec.imageName` against a snapshot **label** `caph-image-name`, not the description, and requires that selector to match exactly one image. The reviewed `infra/packer/release.conf` selects a versioned label such as `bex-worker-k8s-1-34`; `snapshot.yml` immediately retires that label from every older match. `cluster.yaml` uses that exact selector for control-plane and worker templates. Snapshot descriptions include the Kubernetes/containerd/runc tuple, and corresponding labels make the artifact auditable while retired snapshots remain available for an explicit rollback.

## Baking (CI, not a laptop)

Run the **`snapshot (bake worker image)`** workflow ([`.github/workflows/snapshot.yml`](../../.github/workflows/snapshot.yml)) via _Run workflow_ (workflow_dispatch). It installs Packer and runs `packer build` with the `HCLOUD_TOKEN` secret. Baking spins up a throwaway `cpx22` (the older Intel `cx` line is create-blocked in fsn1; the x86 snapshot still runs on prod's `cx33`) and creates a billed snapshot, so it is manual-only.

Local dry check (no build, no token needed for `validate` beyond the plugin):

```sh
cd infra/packer
packer init .
packer validate .
```

## ⚠️ Ordering — bake before you roll

The overlay's `HCloudMachineTemplate`s point at the versioned `IMAGE_NAME` in `release.conf`, and their bootstrap commands **assume** those baked binaries. Applying the overlay before that snapshot exists leaves CAPH unable to resolve the image and new machines never provision. Sequence every roll (initial and each version bump):

1. **Bake** — run `snapshot.yml`. Confirm exactly one snapshot exists for the reviewed selector, for example: `hcloud image list --type snapshot --selector caph-image-name=bex-worker-k8s-1-34`.
2. **Verify one node** — scale a worker pool by one and watch a single machine provision from the snapshot, then join `Ready`:
   ```sh
   kubectl scale machinedeployment bex-tenant-0 --replicas=2   # or bump the autoscaler min briefly
   kubectl get machines -w
   ```
   On the new node's provision log (`hcloud server ... ` cloud-init output, or `journalctl -u cloud-final`) confirm **no** `wget github.com/...runc|containerd`, **no** `pkgs.k8s.io` kubelet install, and **no** `kubeadm config images pull`. Then confirm a tenant App schedules and pulls from Zot first-try (the w1/m26 property this roll depends on).
3. **Roll workers, then control plane** — merge/apply the overlay. MachineDeployments roll workers using their versioned templates. After those prove the snapshot, KCP rolls one stacked-etcd member at a time onto its new immutable template; watch quorum and `kubectl get machines,nodes -w` throughout.
4. **Measure** — record machine-created → node-`Ready` before/after in [`.pm/w1/m36/README.md`](../../.pm/w1/m36/README.md) (Measurements).
5. **Clean up the rotated-away templates** once the roll has settled and the old machines are gone:
   ```sh
   kubectl delete hcloudmachinetemplate bex-tenant-0 bex-platform -n default
   ```

## Why the template names are rotated

`HCloudMachineTemplate.Spec` is **immutable** (CAPH v1.1.7's webhook rejects any `Spec` update). Changing `imageName` therefore needs a **new** template name that the `MachineDeployment` `infrastructureRef` repoints to — and that repoint is exactly what rolls the pool. Hence `bex-tenant-0` → `bex-tenant-0-baked` and `bex-platform` → `bex-platform-baked`. The next image bump rotates again (`-baked` → `-baked2`, …).

## Bumping the Kubernetes / containerd / runc version

1. Bump `kubernetes_version` / `containerd_version` / `runc_version` defaults in `bex-worker.pkr.hcl` (and pass `kubernetes_version` to the workflow).
2. Bump the matching pins in `cluster.yaml`: `version:` on `KubeadmControlPlane` + `MachineDeployment`s, the local baked-version assertions in control-plane bootstrap, its immutable template name, and the kubeadm-matched etcd image (`cluster.yaml` flags this).
3. Bake → verify → rotate the worker and control-plane `HCloudMachineTemplate` names → roll (above).

`scripts/clusterapi-validate.sh` guards the shape **and the pins**: it fails CI if an active template leaves the reviewed versioned snapshot, bootstrap reintroduces privileged runtime downloads, or the Packer pins diverge from `cluster.yaml`.
