# w6 · m624 — A service that probes by exec mounts its health probe from bex's image, so its app starts without waiting for the kubelet's relist

**Worker:** worker6 **Goal:** a pod that carries bex's health-probe helper gets the binary from an `image` volume of bex's image instead of the `bex-health-probe-install` init container. With no init container, the kubelet starts the app in the same pass that starts the pod, and the 0–1 s wait for its next relist is gone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator mounts the health probe from an image volume when the cluster serves them](t001.md) | 1h30m | — |
| t002 | [Prove the image volume on the local CAPD cluster](t002.md) | 1h | t001 |
| t003 | [Turn on `ImageVolume` on production's API servers and tenant kubelets](t003.md) | 1h30m | — |
| t004 | [Turn the image volume on in production's operator](t004.md) | 20m | t001, t002, t003 |
| t005 | [Measure on production and record the window](t005.md) | 1h30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- On production, as the QA user in `bex-canary`, code deploys and restarts of a disk-bearing Starter web image service (disposable) with a Health Check Path, and with no `PORT`, each have a window within 0.5 s of a named-port twin's on the same node, measured as in [w6/170](../done/170.md). Their pods carry no init container and mount `/bex-health/health-probe` from bex's image through an `image` volume.
- kube-apiserver and the kubelet of every node in `bex-tenant-0`, `bex-tenant-xl` and `bex-tenant-xxl` report `kubernetes_feature_enabled{name="ImageVolume"} 1`.
- An operator with the setting off keeps the installer. Turning the setting on rolls no running service: each template adopts the image volume on its next deploy, restart or settings change.
- ADR082 § D3, ADR004 and the ADR018 disk row record the mechanism and the measured windows.
- If t002 or t005 shows that an image volume cannot carry the probe on production, ADR082 records the relist wait (0–1 s) as the bound for a service that probes by exec, with the evidence, and the milestone closes with that record. This was the note's own fallback.

## Source + Goal linkage

- **Source:** inbox note [w6/176](../done/176.md), filed by w6/170 on 2026-11-24 from its production measure and promoted by `/loopx w6` the same day.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)). Render says a disk service is "unavailable for a few seconds" on a deploy. [ADR082 § D3](../../../docs/ADR082-persistent-disks.md#d3--mechanism-deployment--operator-managed-pvc-recreate-strategy) bounds bex's window at a named-port service's 3.99–4.26 s, and a service that probes by exec is the last same-node case above it: 4.99–5.50 s whenever the relist waits the full second.
- **Expected outcome:** a disk service with a Health Check Path, or a web image that names no port, is down no longer on a deploy or restart than one that names its port. Every other service that carries the helper rolls up to 1 s faster.
- **Why now:** w6/170 removed the installer's CPU throttle, which leaves the relist as the only term between such a service and its twin. `ImageVolume` is beta on production's v1.34.9 and becomes on by default in v1.35, so the gate this milestone sets is a no-op at the next minor upgrade.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. This is an operator mechanism and a cluster setting, and t005 checks the customer-observable window.

## Triage findings (2026-11-24)

The note's directions, weighed against the code at `3dd1fccbae` and upstream sources:

- **Turning on the kubelet's `EventedPLEG` is rejected.** Kubernetes 1.34 lists it as Alpha, default off (`pkg/features/kube_features.go` on release-1.34). [KEP-3386](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/3386-kubelet-evented-pleg) is back at alpha, with beta targeted for v1.38. It would change how every pod on the node reports its status.
- **Chosen: an `image` volume ([KEP-4639](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4639-oci-volume-source)).**
  - **Gate.** `ImageVolume` is beta and off by default in 1.33–1.34, beta and on by default in 1.35, and GA in 1.36 (kubernetes/website `feature-gates/ImageVolume.md`). It must be on in both kube-apiserver and the kubelet. Production runs v1.34.9, where `kubernetes_feature_enabled{name="ImageVolume",stage="BETA"}` reads 0 ([w6/m595](../done/m595/README.md)). The local CAPD cluster runs v1.36.1, where it is GA.
  - **The binary can run from it.** The KEP dropped its `noexec` requirement on 2025-06-17 (KEP history). containerd's image mount requires `readonly` and adds no `noexec` (`internal/cri/server/container_image_mount.go`). containerd supports it from v2.0, and production pins 2.3.3. `subPath` works from Kubernetes 1.33.
  - **Admission allows it.** Pod Security's restricted level lists `image` among its allowed volume types (`pod-security-admission/policy/check_restrictedVolumes.go`, release-1.34). `operator-workload-admission.yaml` refuses only host paths and crown Secrets.
  - **The binary is already in bex's image.** bex's image carries it at `/health-probe` (`lego/Dockerfile:84`), and the operator already pins that image for the installer (`release_config_snapshot.go:783`).
- **Other ways to get the binary into the app container are rejected.**
  - **A regular sidecar that copies it in** would start in the same pass as the app. But its requests count toward the pod's and the plan's quota, and a small CPU limit makes runc start it slowly, which is w6/170's mechanism. It would also be a second container in the instance.
  - **A restartable init container** (a native sidecar) would not help. The kubelet starts the app only on a sync after it sees the sidecar running, so that wait is a relist too.
  - **A `hostPath` volume** is refused by Pod Security baseline and by `operator-workload-admission.yaml`.
  - **A ConfigMap** holds 1 MiB, and the binary is about 5 MiB.
- **Waiting for the v1.35 upgrade is not the plan.** No upgrade is scheduled. It would need a re-baked patched kubelet (`bex-kubelet/v1.34.9-cri-records.1`, `infra/packer/bex-worker.pkr.hcl`) and the staged one-minor path in [infra/clusterapi/README.md](../../../infra/clusterapi/README.md).
- **Where App pods run.** Only on `bex-tenant-0`, `bex-tenant-xl` and `bex-tenant-xxl` (KubeadmConfigTemplates of the same names). `bex-tenant-burst` and `bex-tenant-lg` use `bex-tenant-build`, which runs builds only, and `bex-platform` runs no App.
- **The roll's cost.** The API server's gate rolls the control plane (KubeadmControlPlane). Each tenant pool needs its own `rolloutAfter`, because a MachineDeployment does not re-roll on an in-place template edit (`82d55f40dd`). Each roll is surge-first, and every disk service on a rolled pool moves once: about 33 s ([w6/173](../done/173.md)). w1/144 rolled every pool and the control plane, and w6/m547 t005 rolled `bex-tenant-0` for a kubelet setting.
