# w3 · m42 — Sandbox hibernate/resume under PSS (ADR047 gap 2, closes the m32 deferral)

**Worker:** worker3 **Goal:** rootfs pause/resume works on the production substrate again — the OpenSandbox snapshot-commit Job no longer needs `hostPath` inside the tenant namespace — closing w3/m32's deferred pause/resume items and unblocking session hibernation (ADR047 D1) and its billing economics (D6: hibernated sessions do not accrue compute). **Status:** t001+t002 DONE, live-verified on prod 2026-08-02; t003 code done. `scripts/verify-sandbox-pause-resume-live.sh` PASSED against production: rootfs marker survived create → pause (pod deleted; commit Job confined to `opensandbox-snapshot`; snapshot Succeed; pushed to the per-tenant Zot repo) → resume (pod recreated from the snapshot image via the per-workspace pull credential) → readback. Full chain shipped this session: carried controller patch (job namespace + per-tenant repo nesting) active at a CI-maintained digest; `SandboxNamespaceRegistryReconciler` mints/revokes per-ws `snap-<ns>` read-only Zot users + `bex-snapshot-pull` Secrets (proven: 15/15 fleet + fresh-workspace mint + delete-time revoke); prod transport values ON; push secret provisioned out-of-band. Live bring-up findings fixed en route: bex-api bind RBAC + admission allowlists for the new RoleBinding, Zot ingress allowlist for the commit Jobs, the actual containerd socket path, a latent lost-update race on the shared zot Secrets (MergeFromWithOptimisticLock), and — decisive — runsc's default memory overlay defeating commit snapshots (`--overlay2=none` at node bootstrap in `infra/.../sandbox-pool.yaml`, applied in place on the live sandbox node) plus gVisor's `/tmp` tmpfs (markers must live on rootfs; documented tenant semantic). m32's rootfs pause/resume deferral is closed. Remaining: t003's live agent-`session/load` continuity demo folds into the m41 agent-session E2E (driver code + tests landed); t004/t005 substance done in-session (formal task closeout pending); upstream PR staged on the fork (<https://github.com/puncsky/OpenSandbox/pull/1>).

## Tasks (in order)

| id   | title                                                                        | est  | depends_on |
| ---- | ------------------------------------------------------------------------------ | ---- | ---------- |
| t001 | Upstream OpenSandbox fix: snapshot-commit Job out of the tenant namespace       | 120m | —          |
| t002 | Re-enable rootfs pause/resume; verify on the prod gVisor substrate              | 60m  | t001       |
| t003 | Session resume path: driver restart + `session/load` when the agent advertises it | 45m | t002       |
| t004 | Simplify pass over the changed lifecycle code                                  | 20m  | t003       |
| t005 | Test coverage: pause/resume lifecycle + resume-path behavior                   | 45m  | t003       |
| t006 | Closeout                                                                       | 10m  | t005       |

## Definition of done

- The snapshot-commit path runs outside the tenant `<ws>-sandbox` namespace (or otherwise without `hostPath` under the tenant PSS baseline) — the ADR042 D5 blocker is gone; the fix is upstreamed to OpenSandbox (PR merged or a carried patch documented in `deploy/gitops/`).
- Sandbox pause (rootfs snapshot) and resume work on the production gVisor substrate: a paused sandbox's pod is gone, resume restores the rootfs and restarts the workload — verified live with evidence.
- For agent sessions: resume restarts the driver, which uses `existingSessionId` → ACP `session/load` **only when** the agent advertises `loadSession` (m37 t005 contract); otherwise the honest fallback (fresh turn on restored rootfs) runs — mode recorded on the session.
- Credential scrub/re-fetch on the snapshot boundary (w3/m38 t004) holds through a real pause/resume cycle.
- w3/m32's two deferred items (rootfs pause/resume, warm Pool applicability) are re-dispositioned: pause/resume closed here; warm Pool re-evaluated and either noted as follow-up or dropped with rationale.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) gap 2 + ADR042 D5 notes; w3/m32's amended DoD deferral ("rootfs pause/resume + warm Pool deferred, blocked by m31's baseline-PSS isolation floor"). `/pm-brainstorm` decomposition 2026-08-01.
- **Goal linkage:** pillar 5 — hibernation is load-bearing for both the session product (multi-hour idle sessions) and its economics (w7/m76 excludes hibernated time from `sandbox_compute_seconds`).
- **Expected outcome:** idle sessions stop costing compute; suspend/resume verbs are truthful again on the prod substrate.
- **Why now:** ADR047 wave 1's only upstream-scope-uncertain item — starting it early absorbs the risk; w3/m41's steering can ship on re-dispatch, but resume is the intended mechanism. Render parity omitted: substrate mechanism — no REST/GraphQL/MCP/UI semantics change (the existing suspend/resume verbs regain function; their surface shape is unchanged).
