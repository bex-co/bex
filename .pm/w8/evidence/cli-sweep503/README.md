# CLI sweep 503 — deploy/restart against a suspended service

**Verdict: one minor bug filed** as [w8/128](../../128.md) (restart-path refusals name the internal App name). The suspend invariant itself holds. Same isolated session as sweeps 501–502, `bex-canary`, HEAD build `97a170c1b`, 2026-10-19T08:58–09:02Z.

One free image web service (`ealen/echo-server:0.9.2`) created → `live` → HTTPS 200 → suspended over REST (`202`) → public host answers `503 This service is suspended.` → CLI deploy and restart probed → resumed (`202`) → deleted by ID (404 + former-URL 404 verified).

| step | observed result |
| --- | --- |
| Suspend (REST) | `202`; host serves the content-negotiated suspended `503`, matching the parity ledger (never wakes on traffic). |
| `bex deploys create <id>` (no `--confirm`) | Interactive prompt on a non-TTY ends `Error: EOF`, exit 1 — upstream non-TTY guard, not a server defect. |
| `bex deploys create <id> --confirm` | Exit 1, `409 … service "srv-…" is suspended`. Correct refusal, names the caller's ID. |
| `bex restart <id> --confirm` | Exit 1, `409 … service "tea-<workspace>-<slug>" is suspended` — internal App CR name. Filed as w8/128: `rollback_config.go:152` passes `a.Name` to the shared `validateTrigger`; raw REST reproduces it, so it is server-side on HEAD, not deploy lag or CLI rendering. |
| Resume (REST) then delete | `202`; fixture deleted by exact ID; absence proven. The post-resume URL still read `503` at +10 s (rollout/wake lag) and was deleted before a `200` recheck — timing unverified, not filed (sweep 500 observed resume returning a running instance). |
| Raw `POST /deploys` with empty body | `400 request body does not match the Render API schema` (probe artifact: the CLI sends a schema-valid body and reaches the 409 above). |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbatn7vbcrps73dmoa80` deleted; `deploys list` 404 and former-URL 404 verified. Suspend/resume ran over REST with the same QA credential (the pinned CLI exposes no suspend verb) — diagnosis only, not counted as CLI coverage. The filing in w8/128 is uncommitted (no `/ship` authorization in this loop request).
- Unrun: Rollback on a suspended service (same shared path as the filed bug; its acceptance covers it), resume-to-200 timing, deploy on a billing-suspended workspace (no safe fixture).
- Deploy lag means deployed revisions can precede local `main`; this finding was traced in HEAD code, so lag does not explain it.
