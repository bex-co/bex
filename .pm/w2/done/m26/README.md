# w1 · m26 — Harden the build-image pull path (Zot node access, retention, drift guards)

**Worker:** worker1 **Goal:** Every tenant node — including ones the autoscaler mints fresh — can pull a git-built image with zero hand-applied config, the registry can't silently fill again, and the prod-DB ownership drift that caused it is guarded against recurrence. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Bake the containerd DNS+TLS pull-path fix into the CAPH machine template/cloud-init (`infra/`) so every autoscaler-minted tenant node can pull from Zot with zero hand-applied config; replace the `/etc/hosts` hack with a stable NodePort or node-local resolution | 1h  | —          | — **DONE** |
| t002 | Ship the `CiliumNetworkPolicy allow-node-image-pulls` fix through Argo CD (currently live but unmanaged, so it drifts on next sync)                                       | 20m | —          | — **DONE** |
| t003 | Zot retention/GC policy so a runaway rebuild loop can't refill the volume (cap generations kept per repo, or age-based prune)                                             | 45m | —          | — **DONE** |
| t004 | Zot volume-usage alert rule added to the `w3/m6` Alertmanager rule pack                                                                                                   | 20m | t003       | — **DONE** |
| t005 | Root-cause the App-generation churn loop that pushed 51 generations on `eden-cms-v2-git` (reconciler loop? clone-secret refresh?) and fix the underlying trigger          | 45m | —          | — **DONE** |
| t006 | Migration-ownership guard: root-cause how `tenant_invites` was created owned by `postgres` instead of `bex`, fix any other drifted tables, add a CI/migration-convention check preventing recurrence | 40m | —          | — **DONE** |
| t007 | Live verification: force the autoscaler to mint a fresh tenant node, confirm a git-built image pulls with zero hand-applied config                                       | 30m | t001, t002 | — **DONE** |
| t008 | Simplify: run `/simplify` over the code/config this milestone changed                                                                                                     | 30m | t007       | — **DONE** |
| t009 | Test coverage: meaningful tests/checks for the migration-ownership guard and (where feasible) the Zot retention policy                                                    | 30m | t007       | — **DONE** |
| t010 | Closeout: verify DoD, mark done, move to `w1/done/m26/`                                                                                                                   | 15m | t008, t009 | — **DONE** |

## Definition of done

A freshly autoscaled tenant node pulls a git-built image on first attempt with no manual intervention; the Cilium policy survives an Argo CD sync; Zot's volume usage is bounded by a retention policy and alerts before it fills; `tenant_invites` (and any sibling drift) is fixed and guarded against recurrence.

## Source + Goal linkage

- **Source:** `.pm/w1/017.md`, found live 2026-07-12/13 during a routine backfill; never previously sized into a milestone.
- **Goal linkage:** platform reliability — w1's own founding charter ("de-risk the live system... then the elastic/cost machinery"); this is the elastic/autoscale machinery (`w1/m3`, `w1/m20`) silently failing at the one moment it's exercised (a fresh node).
- **Expected outcome:** zero-touch image pulls on every current and future tenant node; a full registry volume becomes a paged alert instead of a silent outage; the invite-redemption permission-denied bug (already root-caused as a symptom) can't recur unnoticed.
- **Why now:** this is a live, standing production risk, not speculative work — the autoscaler can mint a new tenant node at any time and it would fail today; the note is 1-2 days old and still unowned.
- **Render parity omitted:** pure infra/mechanism (CAPH templates, NetworkPolicy, registry GC, migration hygiene) — no REST/GraphQL/MCP/UI surface changes, so the standing Render-parity closing task is skipped; standing closing tasks here are Simplify, Test coverage, Closeout.
