# w6 · m151 — `render-v1` runtime pods run in a pod user namespace (`hostUsers: false`)

**Worker:** worker6 **Goal:** `render-v1` runtime pods (Deployment, CronJob, manual run, predeploy) run with `hostUsers: false`, so the wider capability set and the relaxed seccomp profile act inside a user namespace, never as host root. It ships behind a switch, is proven on Hetzner `hcloud-volumes` first, then turned on. **Status:** todo (t001–t004 done 2026-10-21; widened to every workspace, closeout after the widen deploys)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Operator: project `hostUsers: false` for `render-v1` behind `BEX_RENDER_V1_USER_NAMESPACES` (default off) on all four paths — **DONE** | 45m | — |
| t002 | Admission: `render-v1` workloads may set `hostUsers: false`; when the switch is on, require it — **DONE** | 30m | t001 |
| t003 | Hetzner proof: a disk-backed `render-v1` canary with the switch on in the `bex` workspace (idmapped `hcloud-volumes`, restart, snapshot/restore) — **DONE** | 45m | t002, w6/m149/t009 |
| t004 | Turn the switch on in production; record the residual risk in ADR089/ADR022 — **DONE** | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 30m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

1. **Pod confinement.** A new `render-v1` service in a customer workspace reads `uid_map` other than `0 0 4294967295` inside the pod. Unchanged ClickHouse, with its disk, still deploys, restarts and passes a snapshot/restore round trip.
2. **Legacy unchanged.** `strict-v1`/`image-v1` pod templates are byte-identical with the switch on or off.
3. **Rollback.** Turning the switch off restores host-user pods on the next rollout, with no data migration (t002 spike: on-disk UIDs are unshifted).

## Source + Goal linkage

- **Source:** w6/086, split out of w6/m149 (2026-10-20). Evidence: `docs/render-artifacts/container-privileges.md` § bex user namespaces. On ext4 every path worked; on a non-idmappable filesystem the pod fails closed.
- **Goal linkage:** defense in depth for the Render-parity privilege policy (ADR008 § Render parity first keeps parity; this bounds its blast radius). It is not a parity item: Render runs no pod user namespace.
- **Expected outcome:** a container escape from a `render-v1` pod lands as an unprivileged host UID.
- **Why now:** `render-v1` widened capabilities (m149) and seccomp (m150). Confinement should follow before many `render-v1` services exist.
- **Render parity task omitted:** runtime-internal; no REST/GraphQL/MCP/UI change.
