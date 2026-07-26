# bex worker node image (baked Hetzner snapshot)

`bex-worker.pkr.hcl` bakes a Hetzner Cloud snapshot that carries **runc, containerd, and kubelet/kubeadm/kubectl** preinstalled at the pinned versions, so an autoscaled worker joins the cluster **without** downloading ~350 MB from `github.com` + `pkgs.k8s.io` on the scale-up hot path (w1/m36; FUTURE-MAYBE "Node bring-up efficiency").

- **~150 MB** of runc + containerd + kubelet downloads → baked in.
- **~200 MB** of `kubeadm config images pull` (control-plane images a worker never runs) → dropped entirely (w1/m36 t002).
- apt list trimmed to the packages provisioning actually uses (w1/m36 t003).

What stays in cloud-init (the `KubeadmConfigTemplate` `files:` + `preKubeadmCommands` in [`../clusterapi/overlays/hetzner-caph/cluster.yaml`](../clusterapi/overlays/hetzner-caph/cluster.yaml)): containerd's service unit + `config.toml`, the tenant-only zot `certs.d`, the sysctls/modules, and `systemctl enable kubelet`. The image is **just binaries + packages**, so one snapshot serves both worker pools (tenant + platform) and the pool-specific runtime config lives where it belongs. The **control-plane is out of scope** — it still boots `ubuntu-24.04` and provisions the old way.

## How CAPH picks the image

CAPH resolves `HCloudMachineTemplate.spec.template.spec.imageName` against a snapshot **label** `caph-image-name`, newest match wins — not the description. The bake stamps `caph-image-name = bex-worker`, and `cluster.yaml` sets `imageName: bex-worker`. Snapshot descriptions include the Kubernetes/containerd/runc version tuple so a runtime-only upgrade does not collide with an older bake; every version still reuses the stable label, and new machines pick the newest `bex-worker` snapshot automatically. The version-specific labels (`bex.co/k8s`, `bex.co/containerd`, and `bex.co/runc`) make the selected artifact auditable.

## Baking (CI, not a laptop)

Run the **`snapshot (bake worker image)`** workflow ([`.github/workflows/snapshot.yml`](../../.github/workflows/snapshot.yml)) via _Run workflow_ (workflow_dispatch). It installs Packer and runs `packer build` with the `HCLOUD_TOKEN` secret. Baking spins up a throwaway `cpx22` (the older Intel `cx` line is create-blocked in fsn1; the x86 snapshot still runs on prod's `cx33`) and creates a billed snapshot, so it is manual-only.

Local dry check (no build, no token needed for `validate` beyond the plugin):

```sh
cd infra/packer
packer init .
packer validate .
```

## ⚠️ Ordering — bake before you roll

The overlay's worker `HCloudMachineTemplate`s already point at `imageName: bex-worker` and their `preKubeadmCommands` **assume** the baked binaries. Applying that overlay **before** the snapshot exists leaves CAPH unable to resolve the image and new machines never provision. Sequence every roll (initial and each version bump):

1. **Bake** — run `snapshot.yml`. Confirm the snapshot exists: `hcloud image list --type snapshot --selector caph-image-name=bex-worker`.
2. **Verify one node** — scale a worker pool by one and watch a single machine provision from the snapshot, then join `Ready`:
   ```sh
   kubectl scale machinedeployment bex-tenant-0 --replicas=2   # or bump the autoscaler min briefly
   kubectl get machines -w
   ```
   On the new node's provision log (`hcloud server ... ` cloud-init output, or `journalctl -u cloud-final`) confirm **no** `wget github.com/...runc|containerd`, **no** `pkgs.k8s.io` kubelet install, and **no** `kubeadm config images pull`. Then confirm a tenant App schedules and pulls from Zot first-try (the w1/m26 property this roll depends on).
3. **Roll both pools** — merge/apply the overlay. The MachineDeployments repoint to `bex-tenant-0-baked` / `bex-platform-baked`, which triggers a rolling update (CAPI creates a new machine, waits for `Ready`, then removes an old one — a broken image stalls the roll with the old nodes intact rather than causing an outage). Watch: `kubectl get machines,nodes -w`.
4. **Measure** — record machine-created → node-`Ready` before/after in [`.pm/w1/m36/README.md`](../../.pm/w1/m36/README.md) (Measurements).
5. **Clean up the rotated-away templates** once the roll has settled and the old machines are gone:
   ```sh
   kubectl delete hcloudmachinetemplate bex-tenant-0 bex-platform -n default
   ```

## Why the template names are rotated

`HCloudMachineTemplate.Spec` is **immutable** (CAPH v1.1.7's webhook rejects any `Spec` update). Changing `imageName` therefore needs a **new** template name that the `MachineDeployment` `infrastructureRef` repoints to — and that repoint is exactly what rolls the pool. Hence `bex-tenant-0` → `bex-tenant-0-baked` and `bex-platform` → `bex-platform-baked`. The next image bump rotates again (`-baked` → `-baked2`, …).

## Bumping the Kubernetes / containerd / runc version

1. Bump `kubernetes_version` / `containerd_version` / `runc_version` defaults in `bex-worker.pkr.hcl` (and pass `kubernetes_version` to the workflow).
2. Bump the matching pins in `cluster.yaml`: `version:` on the `KubeadmControlPlane` + `MachineDeployment`s, the `CONTAINERD=` / `RUNC=` exports in the **control-plane** `preKubeadmCommands` (still download-at-boot — out of m36 scope), and the kubeadm-matched etcd image (`cluster.yaml` already flags this).
3. Bake → verify → rotate the worker `HCloudMachineTemplate` names → roll (above).

`scripts/clusterapi-validate.sh` guards the shape **and the pins**: it fails CI if a worker template drifts back to `ubuntu-24.04`, a worker `preKubeadmCommands` block reintroduces the runc/containerd/kubelet download / `kubeadm config images pull` / a trimmed apt package, or the packer pins (k8s/containerd/runc) diverge from `cluster.yaml` — so step 1 without step 2 (or vice-versa) is caught before it merges.
