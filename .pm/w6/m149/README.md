# w6 · m149 — Tenant containers run what Render runs: a Render-compatible privilege policy in every workspace (privilege slice of deploying `render-examples/clickhouse`)

**Worker:** worker6 **Goal:** in any workspace, a new Docker/image service whose unchanged image initializes as root (it changes file ownership, then switches to an unprivileged user, as ClickHouse, Postgres, MySQL and nginx images do) starts and serves. The policy is persisted per service, applied to every path that runs the service image, and confined by a pod user namespace where t002 proves that works. ADR089, ADR022 and ADR018 record the user decision that Render parity outranks the earlier drop-ALL hardening. **Status:** todo (t001, t006 done 2026-10-19; t002 done 2026-10-20; the `render-v1` policy value shipped across types/store/operator/admission — t004/t005/t007 remain open for the creation default and `hostUsers`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture Render's container privilege profile and bex's measured one (evidence artifact) — **DONE** | 40m | — |
| t002 | Spike: runtime pods in a pod user namespace with Render's capability set and a persistent disk — **DONE** | 1h | — |
| t003 | Decide and amend the ADRs: ADR089 D2 accepted as amended, ADR022 hardening section, ADR018, ADR004, ADR070 | 1h | t001, t002 |
| t004 | Types + store + backend: the new persisted policy and its creation default | 1h | t003, w6/m148/t002 |
| t005 | Operator: project the policy (capabilities + `hostUsers`) onto every path that runs the service image | 1h | t003 |
| t006 | Low-port contract: align the `< 1024` refusals and the diagnosis with the measured node behavior — **DONE** | 45m | t001, t003 |
| t007 | Workload admission: admit exactly the new policy's shape | 45m | t005 |
| t008 | Explicit migration operation for existing services (never on upgrade) | 45m | t004, t005 |
| t009 | Canary in the `bex` workspace, then make the policy the default for every workspace | 45m | t004, t005, t006, t007, t008 |
| t010 | Live acceptance in a customer workspace: unchanged ClickHouse, unchanged nginx, isolation, legacy | 1h15m | t009, w6/m148/t007 |
| t011 | Render parity | 20m | t010 |
| t012 | Simplify | 15m | t011 |
| t013 | Test coverage | 40m | t011, t012 |
| t014 | Closeout | 10m | t013 |

## Definition of done

1. **The decision is recorded.**
   - ADR089 is **Accepted**, with D2 amended to the t003 policy and the user decision of 2026-10-17 cited.
   - The ADR022 "Tenant container hardening" section no longer says "Deliberate posture (kept over Render parity)" and describes both the legacy and the new contract.
   - ADR018 (Runtime row `:79`) names the policy and every remaining divergence.
   - `docs/render-artifacts/container-privileges.md` holds the live Render probe (done 2026-10-17) and the bex values measured from a running tenant pod.
2. **Unchanged ClickHouse in a customer workspace.** The test uses a workspace that was **not** in the old `BEX_IMAGE_COMPATIBILITY_WORKSPACES` list. The byte-for-byte `render-examples/clickhouse@355817b1` Blueprint gets these results:
   - its first deploy reaches Live;
   - a peer gets `Ok.` from `http://<slug>:8123/ping` and a `SELECT 1` result over native `<slug>:9000` (ports come from w6/m148);
   - a sentinel row written through 9000 is still readable after a Restart and after a manual redeploy.
3. **The running pod matches the policy.** These values are read inside the pod (`/proc/1/status`, `/proc/1/uid_map`), not from the projected object:
   - `CapBnd` equals exactly the t003 set;
   - `NoNewPrivs` and `Seccomp` equal the t003 decision;
   - if t003 adopted user namespaces, `uid_map` is not the host identity map `0 0 4294967295`.
4. **A second unchanged root-init image.** The official `nginx` image, whose root master runs `setuid` workers, is created as a web service:
   - it uses the port t006 decided (its default port 80, if t006 removed the floor);
   - `curl -sSI https://<slug>.onbex.co/` returns 200.
5. **Legacy is unchanged.**
   - A `strict-v1` service and an `image-v1` service created before the upgrade keep their persisted policy and a byte-identical pod template: same ReplicaSet hash, no rollout.
   - The t008 migration of one `strict-v1` service passes in two steps: its dry run writes nothing, and the real run rolls it exactly once onto the new policy and back to Live.
6. **Isolation holds, and admission rejects anything outside the policy.**
   - From a new-policy pod, these targets stay denied: another workspace's pods, node/kubelet ports, `169.254.169.254`, and platform services.
   - The pod uses no privileged mode, host namespaces or extra devices.
   - A server-side dry-run of an operator-identity workload that adds a capability outside the set is rejected. So is a new-policy workload missing `hostUsers: false`, if t003 adopted user namespaces.
   - Native-runtime services, managed datastores, backups and platform workloads still use `tenantSecCtx()` unchanged.

## Render evidence (measured 2026-10-17)

A live probe ran as root on a Render free web service (`python:3.12-slim`, Oregon) and was deleted afterwards. The full table, method and raw JSON are in [`docs/render-artifacts/container-privileges.md`](../../../docs/render-artifacts/container-privileges.md).

- **Capabilities.** Render's bounding and effective set is exactly `CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETUID, SYS_CHROOT`.
  - It is **not** Docker's default: no `NET_BIND_SERVICE`, `KILL`, `SETPCAP`, `SETFCAP`, `FSETID`, `MKNOD`, `AUDIT_WRITE` or `NET_RAW`.
  - bex `image-v1` is this set minus `FOWNER` and `SYS_CHROOT`.
  - Render community reports agree: setcap binaries fail to exec on Render (`caddy`, `coturn`) because `NET_BIND_SERVICE` is missing from the bounding set.
- **`NoNewPrivs` is `1`**, the same as bex's `allowPrivilegeEscalation: false`.
- **Seccomp is `0`: Render applies no filter.** `io_uring_setup` and `unshare(CLONE_NEWUSER)` succeed. bex's RuntimeDefault blocks both. This is the one runtime divergence a matching capability set leaves.
- **Low ports.** `ip_unprivileged_port_start` is `0`, so both root and uid 101 bind ports 80/81 without `NET_BIND_SERVICE`. `ping_group_range` covers all gids.
- **No pod user namespace** (`uid_map` is the identity map). AppArmor is `cri-containerd.apparmor.d (enforce)`. The host is GKE with kernel 6.8, containerd and real runc (not gVisor). `fs.protected_regular` is `2`.
- **Not measured:** private services (no free plan; assumed to share the runtime) and paid instance types.
- **Parity criterion** (t003 records it): every image that starts on Render starts on bex unchanged. The working capability set is therefore **Render's measured six**, not Docker's default or PSS baseline. That is a much smaller widening than first assumed: `image-v1` plus `FOWNER` and `SYS_CHROOT`.

## Mechanism (current `main`, 2026-10-17)

- **Capability projection.**
  - `tenantSecCtx()` (`lego/operator/internal/controller/app_controller.go:5093`) sets drop `ALL`, `allowPrivilegeEscalation: false` and RuntimeDefault seccomp.
  - `appSecCtx()` (`container_policy.go:46`) adds only `CHOWN, DAC_OVERRIDE, SETUID, SETGID` for `image-v1`. It is the single policy point for every path that runs the service image:
    - the Deployment (`deployment_projection.go:264`);
    - the CronJob and one-off cron run (`cronPodSpec`, `app_controller.go:4738`);
    - the pre-deploy Job (`reconcilePreDeploy`, `:5816`).
  - Datastores, backups and platform containers call `tenantSecCtx()` directly (`database_controller.go:1811`, `keyvalue_*.go`, `disk_backup.go:267`) and are out of scope.
- **User namespaces exist only for execution pods.**
  - `execution.HardenPod()` sets `hostUsers: false` (`lego/operator/internal/execution/security.go:96-98`). It is used by build (`build.go:1184`), publish (`publish.go:460,764`) and pre-deploy (`predeploy.go:183`).
  - The runtime Deployment and CronJob pod templates set no `hostUsers` (no `HostUsers` anywhere in `internal/controller`), so container root is host UID 0 today.
  - Tenant-pool kubelets already run `UserNamespacesSupport=true` (`infra/clusterapi/overlays/hetzner-caph/cluster.yaml:105`). Bootstrap fails closed without containerd v2.3.3 (`:135`).
- **Low ports are already bindable on production, which contradicts ADR022** (measured 2026-10-19, t001).
  - Nodes write `containerd config default` with no override (`cluster.yaml:140`).
  - containerd v2.3.3's CRI default is `EnableUnprivilegedPorts: true` (`internal/cri/config/config_unix.go:110` at tag v2.3.3). That sets `net.ipv4.ip_unprivileged_port_start=0` in every non-host-network pod.
  - A live `strict-v1` pod reads `0`, and its root binds `:80`. These all rest on a stale premise:
    - ADR022's "cannot bind ports below 1024 even as root";
    - the backend refusal `minTenantPort = 1024` (`lego/backend/internal/apps/service.go:4569,4585`);
    - the crash diagnosis (`app_controller.go:5134`);
    - w6/m148's `≥ 1024` primary floor (`image_network.go:78`, m148 t001).
- **Workload admission** is in `deploy/gitops/base/operator-workload-admission.yaml`:
  - `imageCompatibilityWorkload`/`imageCompatibilityCaps` (`:96-110`) key on the `app.bex.co/container-policy` label plus a matching App owner UID;
  - `hostUsers: false` is required only in isolated execution namespaces (`:155-161`);
  - the restricted container grammar applies outside them (`:163-193`).
- **Namespace Pod Security** is `baseline` (`lego/backend/internal/store/namespaces.go:398`). It admits the PSS baseline set and forbids `NET_RAW`, privileged mode and host namespaces.
- **Persisted policy.** `0135_app_image_compatibility.up.sql` CHECKs `container_policy IN ('', 'strict-v1', 'image-v1')`. The latest migration is `0148`; use the next free number when implementing. Creation chooses the policy in `configureNewImageCompatibility` (`service.go:2434`) behind `imageCompatibilityAdmits` (`:2451`). w6/m148/t002 splits ports out of that gate, leaving it to gate capabilities only.
- **Disks** are LUKS-encrypted `hcloud-volumes` PVCs in a `Recreate` Deployment (ADR082 `:38`, `:81-89`). A pod user namespace needs idmapped mounts on that filesystem. That is unverified (t002 locally, t009 on Hetzner).

## Contract changes (every artifact moves with the ADR)

| Contract | Artifact | Owner |
| --- | --- | --- |
| ADR text | ADR089 (status, D2, Alternatives "restore every Docker capability" entry, checkpoint), ADR022 hardening section, ADR018 Runtime row `:79`, ADR004 low-port statements, ADR070 `:159` hardening claim, `docs/AGENTS.md` ADR089 one-liner ("proposed") | t003, t011 |
| Evidence | `docs/render-artifacts/container-privileges.md` (new) | t001, t010 |
| CRD contract (`lego/types`) | `v1alpha1/container_policy.go` new constant and docs; `app_types.go` enum; `operator/config/crd/bases/app.bex.co_apps.yaml` + deepcopy via `make` in `lego/operator` | t004 |
| Store | new migration widening the `container_policy` CHECK (down refuses while rows use it, as `0135` does); `store.go` round trip; real-Postgres test | t004 |
| Admission | `operator-workload-admission.yaml` new policy variable/caps (+ `hostUsers` rule if adopted); the existing API-server regression from w4/m142 (`d95d017ba`) extended | t007 |
| Runtime config | `lego/operator/config/prod/kustomization.yaml:107-113` and its comment; `lego/backend/AGENTS.md` env table row for `BEX_IMAGE_COMPATIBILITY_WORKSPACES` (as left by m148 t002) | t009 |
| Low-port contract | `minTenantPort` + its doc comment, the error text clients see, `app_controller.go:5134` diagnosis, m148's primary floor if still present | t006 |
| Guards that pin today's behavior | operator `TestApplicationContainerPolicies` (`container_policy_test.go:29`), `hardening_test.go`; backend `TestImageCompatibilityCreationPolicy` / `TestImageCompatibilityIsScopedToListedWorkspaces` (`apps/container_policy_test.go:36,182`); port-floor tests for `minTenantPort`. Each changes deliberately with a comment pointing here | t004–t007, t013 |

## Source + Goal linkage

- **Source:** the user's 2026-10-17 "Deploy render-examples/clickhouse to bex" thread, in three steps:
  1. A gap analysis found two blockers, the ADR089 capability policy and image ports. The ports became w6/m148.
  2. Asked whether bex can match Render on privileges, the answer was: yes, but only by amending ADR022 and ADR089 first.
  3. User decision, same day: "render parity 比之前的 security hardening 重要，所以修改 adr，同时对应的实际改动，一并都 /pm for w6" ("Render parity matters more than the earlier security hardening, so amend the ADRs and file the matching changes together on w6").

  Research: ADR089 D2 and its checkpoint, ADR022 "Tenant container hardening", w4/m142 (image-v1 admission), and the evidence above. w6/m148 t011's closeout step 4 asks for this milestone.
- **Goal linkage:** ADR008's core hosting pillar ("the open-source Render alternative"). ADR089's customer outcome is that unchanged Render Docker examples deploy. w1/m175 recorded "Running the MySQL/MinIO images live depends on ADR089".
- **Expected outcome:** official root-init images (ClickHouse, Postgres, MySQL, Redis, MongoDB, nginx) deploy unchanged in every workspace, as they do on Render. Existing services keep their current privileges until deliberately migrated.
- **Why now:**
  - w6/m148 removes the port blocker, which leaves the capability policy as the only reason unchanged ClickHouse fails for customers.
  - The capability code (`image-v1`, persisted policy, admission) already shipped and runs in the `bex` workspace. What is missing is the decision, the wider set, user-namespace confinement, and the rollout.
  - The low-port refusal may already contradict node behavior. That is a live parity bug: a write is refused that would have worked.
- **Render parity included:** creation and deploy outcomes change on REST, GraphQL, MCP and the dashboard, and so do `PORT`/port validation and the failure diagnosis.
- **Related, not taken over:**
  - w6/m148: ports, `openPorts` and references. It is a hard dependency through t004 and t010.
  - w4/m142: image-v1 admission (done; extended here).
  - `.pm/w10/002.md`: the `RENDER_API_KEY` blocker, shared with t001.
  - `w4/blocked/m181`/`m182`: unrelated pre-deploy and port-edit routing.

## Out of scope

- Privileged containers, host network/PID/IPC, devices, `SYS_ADMIN`/`NET_ADMIN`/`NET_RAW`, or any capability beyond Render's measured set.
- Unconfined seccomp. Render runs without a filter, but bex keeps RuntimeDefault unless t003 records an explicit user decision. Unconfined, or a Localhost profile that allows `io_uring` and user namespaces, is a separate, security-reviewed change.
- Tenant-selectable capability lists or a per-service privilege setting. Render exposes none.
- gVisor/Kata for runtime services; native-runtime services (bex-built images do not need root initialization); managed datastores, backups, builds and platform containers.
- Port discovery, `openPorts` and Blueprint port references (w6/m148); the CMS browser checker.
