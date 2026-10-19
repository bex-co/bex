# CLI sweep 535 — restart against a hibernated (auto-slept) free service

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI against production, 2026-10-19T10:22–10:25Z. Tree note: another worker's w8/129 rebase is in progress in this checkout; this sweep touched none of it (sync was fetch-only, code reads via `origin/main`).

One free web fixture: create → `live` → seed one `200` request → set `idleTTLSeconds: 60` → `Hibernated` at +93s → `bex restart` (URL deliberately **not** curled first, so the restart — not traffic — is the wake) → deleted, former-URL `404`.

| step | observed result |
| --- | --- |
| PATCH `{"idleTTLSeconds":60}` (top level) | `400 unknown field "idleTTLSeconds"` — strict-body decoding names it; the field lives under `serviceDetails` on the write side (top level on reads, as sweep 514's payload showed). Recorded as a shape fact, not a bug: the Render PATCH dialect nests service settings there. |
| PATCH `{"serviceDetails":{"idleTTLSeconds":60}}` | `200`; reads back `idleTTLSeconds: 60`. |
| Idle wait | `Running` at +31/+62s, `Hibernated` at +93s; `services instances` → `[]`. |
| `bex restart <id> --confirm` while Hibernated | Exit `0` ("restarted successfully"); a new deploy goes `created → update_in_progress → live`, phase returns `Running`, URL serves `200`. Restart is an explicit operator action, so wake-by-restart (distinct from the suspended 409 of sweeps 503/533 and from traffic wake) is the truthful behavior. |

Cleanup: exact-owned `srv-dbauumnmnaes738pfog0` deleted by ID; former-URL 404 verified.

- Unrun: `deploys create` while hibernated (same explicit-action class), idle-TTL edge values (negative/>7d refuse by the named `0-%d` message — read in code, not re-probed live).
- Deploy lag means deployed revisions can precede local `main`.
