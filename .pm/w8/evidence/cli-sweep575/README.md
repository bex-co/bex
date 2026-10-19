# CLI sweep 575 — service instances across restart and suspend/resume

**Verdict: no bug filed** (one mid-sweep alarm traced to this harness, below). Same isolated QA session, `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T19:58–20:07Z. Owned fixture `qa575-instances` (`srv-dbb7co7o17is73furt70`, free hello-app), deleted with REST + URL + instances 404 proof.

- **Restart rollout is observable and orderly:** one instance before; at +1s after `bex restart` both the old and the fresh instance ids list together; by +20s only the new id remains; deploy `live` at +30s. `logs --instance <real-id>` returns that pod's lines; a bogus instance id returns zero rows (exit 0).
- **Suspend/resume (REST verbs — the pinned CLI exposes them only for Postgres/Key Value):** suspend → 202, instances drain to `[]` immediately, platform URL `503`; resume → 202, a **new** instance id on the same (still `live`) deploy, URL back to `200`; the superseded deploy reads `deactivated`.
- **Selectors:** baseline cron → `[]` (its runs are not service instances); Postgres id, synthetic id, and the deleted fixture → client-wrapped `404 Not Found`, exit 1.
- **Harness artifact, recorded so it is not re-litigated:** mid-suspend, `suspended` appeared stuck at `not_suspended` for two minutes. Cause was this probe, not the product: `bex services <id>` prints the **whole workspace list** (the pinned client has no get-one service verb), and the parser read row `[0]` — a baseline cron. Side-by-side reads then agreed everywhere: REST get-by-id, REST list, and CLI list all show `suspended: "suspended"`, `suspenders: ["user"]` while suspended and `not_suspended`/`[]` after resume.

## Unrun

- Instances during a *failing* rollout (old instance retention while the new one never readies) — the port-mismatch stall of sweep 562 touched the deploy side; instance listing during the stall was not captured.
- Multi-instance services (paid instance counts / autoscaler-chosen counts).
