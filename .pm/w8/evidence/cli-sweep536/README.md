# CLI sweep 536 — `deploys create` against a hibernated free service

**Verdict: no bug filed.** Companion to sweep 535 (restart while hibernated). Same isolated session, `bex-canary`, 2026-10-19T10:27–10:30Z.

Fresh owned free web fixture → `live` → one seed request → `idleTTLSeconds: 60` → `Hibernated` at +130s → `bex deploys create --confirm --wait` (URL not curled first) → deleted, former-URL `404`.

| step | observed result |
| --- | --- |
| Deploy create while `Hibernated` | Exit `0` under `--wait`; the deploy completes, phase returns `Running`, URL serves `200`. Like restart (535), an explicit operator deploy wakes the service — distinct from suspended (409, sweeps 503/533) and from traffic-wake. |
| w8/129 live spot-check (en route) | Baseline Postgres payload still has no `suspenders`/`role` keys — expected: the fix (`df689525f`) is at local HEAD but the deployed image pin is still `8e57d4eaa121`. Deploy lag, not a regression; recheck queued for the next pin. |

Cleanup: exact-owned `srv-dbav0te05jqs73cjk350` deleted by ID; former-URL 404 verified.

- Unrun: deploy create with `--image` override while hibernated, cancel of the waking deploy mid-rollout.
- Deploy lag means deployed revisions can precede local `main`.
