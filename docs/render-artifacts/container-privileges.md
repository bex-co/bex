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

   This is a live parity bug, owned by w6/m149/t006.

3. **Seccomp is the remaining runtime divergence.** Render applies no filter. bex RuntimeDefault blocks `io_uring` and unprivileged user namespaces (the Chromium sandbox, bubblewrap). Relaxing it needs an explicit user decision (w6/m149/t003).
4. **These already match:** `NoNewPrivs`, the AppArmor profile, no pod user namespace, `ping_group_range` and the `protected_*` sysctls.
5. **Separate parity finding: Docker Command launch.**
   - bex runs `dockerCommand` as `sh -c <command>` (`shellCommand`, `lego/operator/internal/controller/deployment_projection.go:217`). Render does not use a shell. The quote-free `python3 -c exec(...)` command ran on Render but crash-looped on bex with `/bin/sh: 1: Syntax error: "(" unexpected`.
   - Render keeps quote characters literally: `sh -c "…"` failed there with `File name too long`.
   - Commands containing shell syntax therefore behave differently on the two platforms.
6. **Not measured:** Render private services (no free plan) and paid instance types; bex with pod user namespaces (w6/m149/t002).
