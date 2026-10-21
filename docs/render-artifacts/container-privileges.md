# Render vs bex container privilege profile

Every row comes from the same probe, [`fixtures/container-privilege-probe.py`](fixtures/container-privilege-probe.py), run as root in `docker.io/library/python:3.12-slim`. The probe reads `/proc/self/status`, the user-namespace maps and the network/fs sysctls, then tries each privileged operation directly. Redacted raw results are in `fixtures/container-privileges-*.json`.

- **Render, 2026-10-17:** free web service in Oregon, `srv-dba3nmjrjlhs738ponp0`, deleted afterwards.
- **bex production, 2026-10-19:** two free web services created through `api.bex.co` with the QA account, both deleted afterwards (REST GET 404) and the QA session revoked.
  - `strict-v1`: `srv-dbaqn8dvjvcc73a4gp7g` in workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`, not on the image-compatibility allowlist).
  - `image-v1`: `srv-dbaqn82d108c73fur01g` in workspace `bex` (`tea-d98210cbbpdc73dcrkvg`, allowlisted).

This is the t001 evidence for [w6/m149](../../.pm/w6/m149/README.md) (ADR089 D2).

## Profile

| Probe | Render | bex `strict-v1` | bex `image-v1` | Docker default (local reference) |
| --- | --- | --- | --- | --- |
| Capability bounding = effective set | `CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETUID, SYS_CHROOT` | none | `CHOWN, DAC_OVERRIDE, SETGID, SETUID` | 14: adds `FSETID, KILL, SETPCAP, NET_BIND_SERVICE, NET_RAW, MKNOD, AUDIT_WRITE, SETFCAP` |
| `NoNewPrivs` | `1` | `1` | `1` | `0` |
| Seccomp | `0` (**no filter**) | `2` (RuntimeDefault, 1 filter) | `2` (RuntimeDefault, 1 filter) | `2` |
| AppArmor | `cri-containerd.apparmor.d (enforce)` | same | same | `docker-default` |
| User namespace (`uid_map`) | none | none | none | none |
| `ip_unprivileged_port_start` | `0` | **`0`** | **`0`** | `0` |
| `ping_group_range` | `0 2147483647` | same | same | — |
| `fs.protected_regular` / `protected_fifos` | `2` / `1` | `2` / `1` | `2` / `1` | `0` / `0` |
| Kernel / runtime | `6.8.0-1041-gke` (GKE), containerd, runc (not gVisor) | `6.8.0-117-generic` (Ubuntu 24.04, Hetzner tenant pool), containerd | same | — |

## Operations

| Operation (as root unless stated) | Render | bex `strict-v1` | bex `image-v1` |
| --- | --- | --- | --- |
| `chown` a file to `101:101` (`CHOWN`) | ok | **EPERM** | ok |
| `chmod` that file after the `chown` (`FOWNER`) | ok | n/a (chown failed) | **EPERM** |
| read/write another user's 0600 file, non-sticky dir (`DAC_OVERRIDE`) | ok | n/a (setup chown failed) | ok |
| `setgroups`/`setgid`/`setuid` to 101 (`SETGID`/`SETUID`) | ok | **EPERM** | ok |
| `chroot` (`SYS_CHROOT`) | ok | **EPERM** | **EPERM** |
| signal a process that dropped to uid 101 (`KILL`) | EPERM | n/a (could not drop) | EPERM |
| `prctl(PR_CAPBSET_DROP)` (`SETPCAP`) | EPERM | EPERM | EPERM |
| `mknod` a char device (`MKNOD`) | EPERM | EPERM | EPERM |
| raw ICMP socket (`NET_RAW`) | EPERM | EPERM | EPERM |
| ICMP "ping" socket | ok | ok | ok |
| bind `:80` as root | ok | **ok** | ok |
| bind `:81` as uid 101 | ok | n/a (could not drop) | ok |
| `io_uring_setup` | **ok** | EPERM (seccomp) | EPERM (seccomp) |
| `unshare(CLONE_NEWUSER)` | **ok** | EPERM (seccomp) | EPERM (seccomp) |
| `unshare(CLONE_NEWNS)` (`SYS_ADMIN`) | EPERM | EPERM | EPERM |

In `/tmp`, root cannot `O_CREAT`-open another user's file on any of the three, because `fs.protected_regular=2` blocks it. That is not a capability difference.

## Findings

1. **Render's set is six capabilities, not Docker's default.** `image-v1` reaches parity once `FOWNER` and `SYS_CHROOT` are added.
   - The missing `FOWNER` matters in practice: an entrypoint that runs `chown` and then `chmod` works on Render and fails on bex `image-v1`. The first version of this probe crash-looped on bex for exactly that reason.
   - Render lacks `NET_BIND_SERVICE`, `KILL`, `SETPCAP` and the rest, so bex should not add them for parity.
2. **Low ports already work on bex production.** `ip_unprivileged_port_start=0` on the tenant nodes (the containerd v2.3.3 default), and even `strict-v1` root binds `:80`. These statements and checks are therefore **false** on current production:

   - ADR022's "a tenant container cannot bind ports below 1024 even as root";
   - the backend refusal `minTenantPort = 1024`;
   - the crash diagnosis "tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped)", which this run's deploy events carried.

   This was a live parity bug; w6/m149/t006 removed the floor, the claim and the diagnosis text.

3. **Seccomp is the remaining runtime divergence.** Render applies no filter. bex RuntimeDefault blocks `io_uring` and unprivileged user namespaces (the Chromium sandbox, bubblewrap). `render-v1` keeps RuntimeDefault (ADR089, 2026-10-20). Relaxing it is w6/085.
4. **These already match:** `NoNewPrivs`, the AppArmor profile, no pod user namespace, `ping_group_range` and the `protected_*` sysctls.
5. **Separate parity finding: Docker Command launch.**
   - bex runs `dockerCommand` as `sh -c <command>` (`shellCommand`, `lego/operator/internal/controller/deployment_projection.go:217`). Render does not use a shell. The quote-free `python3 -c exec(...)` command ran on Render but crash-looped on bex with `/bin/sh: 1: Syntax error: "(" unexpected`.
   - Render keeps quote characters literally: `sh -c "…"` failed there with `File name too long`.
   - Commands containing shell syntax therefore behave differently on the two platforms.
6. **Not measured:** Render private services (no free plan) and paid instance types. bex with pod user namespaces is measured below (w6/m149/t002).

## bex user namespaces (w6/m149 t002 spike, 2026-10-20)

The local CAPD cluster runs Kubernetes v1.36.1, containerd 2.3.1 and the 7.0.12-linuxkit arm64 kernel. Hetzner production uses the same containerd major version, with `UserNamespacesSupport=true`.

- **Pod spec:** each pod was hand-projected with `render-v1`: drop `ALL` plus `CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETUID, SYS_CHROOT`, `allowPrivilegeEscalation: false`, RuntimeDefault seccomp and `hostUsers: false`.
- **Image:** unchanged `postgres:17`, whose entrypoint runs `chown` and then `gosu`. `yandex/clickhouse-server:21` is amd64-only and cannot run on this arm64 host.
- **Scope:** both the spike namespace and the node fixtures were removed afterwards.

**Two volume types gave opposite results.**

- **No-go on the local `local-path` class.** The volume is a directory on the CAPD node's overlayfs root. runc refused the pod: `failed to set MOUNT_ATTR_IDMAP on /opt/local-path-provisioner/pvc-…: invalid argument (maybe the filesystem used doesn't support idmap mounts on this kernel?)`. The container never started.
  - **Cause:** this overlayfs does not accept an idmapped mount. Kubernetes needs that mount to place a pod user namespace on a volume. It is a filesystem limitation, not a bex one.
  - **Consequence:** on any storage class whose filesystem cannot be idmapped, `hostUsers: false` fails closed at container start.
- **Go on ext4.** Production disks are ext4 on LUKS `hcloud-volumes` (ADR082). Each volume was a local PV on a loopback ext4 mounted on the tenant node.

| Path | Result | Observation |
| --- | --- | --- |
| Fresh disk | go | First start ran `initdb`. Inside the pod: `uid_map 0 2522939392 65536`, `CapBnd 00000000000400cb` (exactly the six), `CapEff 0` after `gosu`, `NoNewPrivs 1`, `Seccomp 2`. |
| Host ownership | go | On the node, `pgdata` is `uid=999` and `PG_VERSION` `999:999`: the in-container UID, not shifted. |
| Re-owned disk (restart) | go | After delete and recreate, the pod starts on the volume it already owns and reads the sentinel row. |
| Pre-existing non-userns disk | go | A `hostUsers: true` Postgres initialized and wrote the volume. The same Deployment was then switched to `hostUsers: false`: it starts, reads the old row and writes a new one. Files on disk stay `999:999`. **A t008 migration needs no chown step.** |
| Secrets | go | Secret files under `/etc/secrets` are readable (owned by 0 inside the pod); `envFrom` secret values are present. |
| `emptyDir` | go | Writable. |
| SIGTERM | go | Pod deletion took 4s: `received fast shutdown request` → `database system is shut down`. |
| Exec (SSH gateway path) | go | `kubectl exec … psql` works. The gateway uses the same `exec` subresource. |
| Snapshot/restore | go | The snapshot Job was emulated with the same security context (root, no user namespace, `DAC_OVERRIDE, CHOWN, FOWNER`, a read-only source bind). It ran `tar` from the userns-written volume into a new one with `--numeric-owner`. A `hostUsers: false` Postgres then started on the restored volume and read the sentinel. |

**Verdict.** Pod user namespaces work for disk-backed runtime services on ext4, with every runtime path above. They must not be projected until Hetzner `hcloud-volumes` passes the same idmap check (w6/m149/t009): a filesystem that cannot be idmapped would turn every such deploy into a start failure.
