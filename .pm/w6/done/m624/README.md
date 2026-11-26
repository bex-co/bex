# w6 · m624 — A service that probes by exec mounts its health probe from bex's image, so its app starts without waiting for the kubelet's relist

**Worker:** worker6 **Goal:** a pod that carries bex's health-probe helper gets the binary from an `image` volume of bex's image instead of the `bex-health-probe-install` init container. With no init container, the kubelet starts the app in the same pass that starts the pod, and the 0–1 s wait for its next relist is gone. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator mounts the health probe from an image volume when the cluster serves them](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [Prove the image volume on the local CAPD cluster](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [Turn on `ImageVolume` on production's API servers and tenant kubelets](done/t003.md) — **DONE** | 1h30m | — |
| t004 | [Turn the image volume on in production's operator](done/t004.md) — **DONE** | 20m | t001, t002, t003 |
| t005 | [Measure on production and record the window](done/t005.md) — **DONE** | 1h30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- On production, as the QA user in `bex-canary`, code deploys and restarts of a disk-bearing Starter web image service (disposable) with a Health Check Path, and with no `PORT`, each have a window within 0.5 s of a named-port twin's on the same node, measured as in [w6/170](../../done/170.md). Their pods carry no init container and mount `/bex-health/health-probe` from bex's image through an `image` volume.
- kube-apiserver and the kubelet of every node in `bex-tenant-0`, `bex-tenant-xl` and `bex-tenant-xxl` report `kubernetes_feature_enabled{name="ImageVolume"} 1`.
- An operator with the setting off keeps the installer. Turning the setting on rolls no running service: each template adopts the image volume on its next deploy, restart or settings change.
- ADR082 § D3, ADR004 and the ADR018 disk row record the mechanism and the measured windows.
- If t002 or t005 shows that an image volume cannot carry the probe on production, ADR082 records the relist wait (0–1 s) as the bound for a service that probes by exec, with the evidence, and the milestone closes with that record. This was the note's own fallback.

## Source + Goal linkage

- **Source:** inbox note [w6/176](../../done/176.md), filed by w6/170 on 2026-11-24 from its production measure and promoted by `/loopx w6` the same day.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)). Render says a disk service is "unavailable for a few seconds" on a deploy. [ADR082 § D3](../../../../docs/ADR082-persistent-disks.md#d3--mechanism-deployment--operator-managed-pvc-recreate-strategy) bounds bex's window at a named-port service's 3.99–4.26 s, and a service that probes by exec is the last same-node case above it: 4.99–5.50 s whenever the relist waits the full second.
- **Expected outcome:** a disk service with a Health Check Path, or a web image that names no port, is down no longer on a deploy or restart than one that names its port. Every other service that carries the helper rolls up to 1 s faster.
- **Why now:** w6/170 removed the installer's CPU throttle, which leaves the relist as the only term between such a service and its twin. `ImageVolume` is beta on production's v1.34.9 and becomes on by default in v1.35, so the gate this milestone sets is a no-op at the next minor upgrade.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. This is an operator mechanism and a cluster setting, and t005 checks the customer-observable window.

## Triage findings (2026-11-24)

The note's directions, weighed against the code at `3dd1fccbae` and upstream sources:

- **Turning on the kubelet's `EventedPLEG` is rejected.** Kubernetes 1.34 lists it as Alpha, default off (`pkg/features/kube_features.go` on release-1.34). [KEP-3386](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/3386-kubelet-evented-pleg) is back at alpha, with beta targeted for v1.38. It would change how every pod on the node reports its status.
- **Chosen: an `image` volume ([KEP-4639](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4639-oci-volume-source)).**
  - **Gate.** `ImageVolume` is beta and off by default in 1.33–1.34, beta and on by default in 1.35, and GA in 1.36 (kubernetes/website `feature-gates/ImageVolume.md`). It must be on in both kube-apiserver and the kubelet. Production runs v1.34.9, where `kubernetes_feature_enabled{name="ImageVolume",stage="BETA"}` reads 0 ([w6/m595](../../done/m595/README.md)). The local CAPD cluster runs v1.36.1, where it is GA.
  - **The binary can run from it.** The KEP dropped its `noexec` requirement on 2025-06-17 (KEP history). containerd's image mount requires `readonly` and adds no `noexec` (`internal/cri/server/container_image_mount.go`). containerd supports it from v2.0, and production pins 2.3.3. `subPath` works from Kubernetes 1.33.
  - **Admission allows it.** Pod Security's restricted level lists `image` among its allowed volume types (`pod-security-admission/policy/check_restrictedVolumes.go`, release-1.34). `operator-workload-admission.yaml` refuses only host paths and crown Secrets.
  - **The binary is already in bex's image.** bex's image carries it at `/health-probe` (`lego/Dockerfile:84`), and the operator already pins that image for the installer (`release_config_snapshot.go:783`).
- **Other ways to get the binary into the app container are rejected.**
  - **A regular sidecar that copies it in** would start in the same pass as the app. But its requests count toward the pod's and the plan's quota, and a small CPU limit makes runc start it slowly, which is w6/170's mechanism. It would also be a second container in the instance.
  - **A restartable init container** (a native sidecar) would not help. The kubelet starts the app only on a sync after it sees the sidecar running, so that wait is a relist too.
  - **A `hostPath` volume** is refused by Pod Security baseline and by `operator-workload-admission.yaml`.
  - **A ConfigMap** holds 1 MiB, and the binary is about 5 MiB.
- **Waiting for the v1.35 upgrade is not the plan.** No upgrade is scheduled. It would need a re-baked patched kubelet (`bex-kubelet/v1.34.9-cri-records.1`, `infra/packer/bex-worker.pkr.hcl`) and the staged one-minor path in [infra/clusterapi/README.md](../../../../infra/clusterapi/README.md).
- **Where App pods run.** Only on `bex-tenant-0`, `bex-tenant-xl` and `bex-tenant-xxl` (KubeadmConfigTemplates of the same names). `bex-tenant-burst` and `bex-tenant-lg` use `bex-tenant-build`, which runs builds only, and `bex-platform` runs no App.
- **The roll's cost.** The API server's gate rolls the control plane (KubeadmControlPlane). Each tenant pool needs its own `rolloutAfter`, because a MachineDeployment does not re-roll on an in-place template edit (`82d55f40dd`). Each roll is surge-first, and every disk service on a rolled pool moves once: about 33 s ([w6/173](../../done/173.md)). w1/144 rolled every pool and the control plane, and w6/m547 t005 rolled `bex-tenant-0` for a kubelet setting.

## Parked (2026-11-24, `/loopx w6`)

- **Shipped.** The operator mounts the helper from an `image` volume of bex's image when `BEX_HEALTH_PROBE_IMAGE_VOLUME=true`, with no init container, and adopts the change lazily in both directions (t001). The helper sits in a directory of its own in bex's image, `/bex-health`, mounted with `subPath: bex-health`, because containerd mounts only a directory `subPath` from an image volume (t002). On the local CAPD cluster, disk services with a Health Check Path and with no port started in 0.61–0.81 s against their twin's 0.64–0.84 s, where the installer had taken 2.62–4.05 s (t002). The App pools' kubelet templates carry `ImageVolume`, and `scripts/clusterapi-validate.sh` guards them (t003).
- **The gate.** Turning `ImageVolume` on in kube-apiserver rolls the control plane, and its first surge found no cx33 in fsn1 (`resource_unavailable`; Hetzner reads cx33 unavailable there and cpx32 available). The change was withdrawn as w1/144 did, and the control plane is back on its three members (t003). `bex-tenant-0`'s roll needs cx33 surges too. Production keeps the installer until both have rolled: t004 turns the setting on only after t003.
- **Next.** Once fsn1 can create a cx33, or once the user approves the cpx32 fallback for the control plane (ADR053, about €33 more a month per node), finish t003 as its "Left" says, then t004, t005 and t008.

## Gate check (2026-11-25, `/loopx w6`)

- **Source:** verified evidence; the `/loopx w6` board listed m624 under `blocked.ready`.
- **Pin.** Production's newest pin, `chore(deploy): pin platform images to 7b046987bcd8` (`4b533337ab`), carries `10549cd8b2` (t001's operator change). The date gate 2026-11-25T10:00Z has passed (checked 10:42Z).
- **fsn1.** `GET /v1/server_types/115` reads cx33 `available: false` in fsn1, nbg1 and hel1 again. w1/144 saw the same flag while Hetzner created a cx33 (2026-11-16), so the flag does not decide. The roll itself is the probe, as the gate says: re-apply the control-plane hunk of `3ac4bb829d` and watch its first surge. If Hetzner refuses it, revert first and park again with the date a day later.
- **Not approved:** the cpx32 fallback for the control plane (ADR053) is still the user's to take through `/unblock w6`.
- **Result: parked again.** `b93fb1ccce` re-applied the hunk, and Hetzner refused the first surge, `bex-control-plane-nwbmc`, at 10:56:55Z (`resource_unavailable` for cx33 in fsn1). `a184c14412` reverted it, and KCP was back on its three members at 10:58:26Z with no server left at Hetzner. cpx32 also read unavailable in fsn1 at 10:59Z. Next probe from 2026-11-26T11:00Z, the same way ([t003](done/t003.md)).

## Gate check (2026-11-26, `/loopx w6`)

- **Source:** verified evidence; the `/loopx w6` board listed m624 under `blocked.ready`.
- **Pin.** Production's newest pin, `chore(deploy): pin platform images to 11695fb886cb` (`ec6211f2ca`), carries `10549cd8b2` (t001's operator change). The date gate 2026-11-26T11:00Z has passed (checked 13:53Z).
- **fsn1.** `GET /v1/server_types/115` reads cx33 `available: false` in fsn1, nbg1 and hel1 at 13:54Z; cpx32 (`server_types/110`) reads `available: true` in fsn1. The flag does not decide (w1/144), so the roll itself is the probe: re-apply the control-plane hunk of `3ac4bb829d` and watch its first surge. If Hetzner refuses it, wait for that push's app-cluster run to finish, then revert, and park again with the date a day later.
- **Not approved:** the cpx32 fallback for the control plane (ADR053) is still the user's to take through `/unblock w6`.
- KCP `bex-control-plane` stood on its three members (`ff5b6`, `mmgdq`, `zvxjm`, all up to date) before the roll, and no app-cluster run was in progress.

## Closeout (2026-11-26, `/loopx w6`)

- **t003.** `30588c6353` rolled the control plane onto `ImageVolume` (fsn1 placed a cx33 for the first surge; one refused surge was deleted by hand, [w6/191](../../191.md)), and `c98690e829` rolled `bex-tenant-0`. Every API server and the kubelet of every control-plane and `bex-tenant-0` node read the gate 1; `bex-tenant-xl` and `-xxl` have no nodes and their templates carry it. `scripts/verify-substrate.sh` passes all ten checks.
- **t004.** `28b4f18bbe` turned `BEX_HEALTH_PROBE_IMAGE_VOLUME` on in `config/prod`; Argo synced it at 15:26:40Z and no service rolled.
- **t005.** On `w6-m624-t005` (disposable), pods with a Health Check Path or no port had no init container and mounted the helper from the image volume. They reached a running app in 1.88–2.01 s from their cgroup, against the named-port twin's 1.78–2.00 s, and were down 5.25–5.51 s (Health Check Path) and 5.50 s (no-port code deploys), against the twin's 4.75–5.00 s: at most two 0.25 s probe steps over, for the exec probe and the no-port release's port promotion. ADR082 § D3, ADR004 and the ADR018 disk row record it.
- **Definition of done.** Every line holds: the windows and pod shape (t005), the gate on the API servers and the three App pools (t003), an operator with the setting off keeps the installer and turning it on rolled no service (t001, t007, t004), and the docs.
