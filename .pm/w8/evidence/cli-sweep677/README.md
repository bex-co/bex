# CLI sweep 677 — notification settings dialects (live, 2026-10-19)

Caller's own member settings only; the one mutation was restored and read-back verified.

- Render's owner-level `GET|PATCH /v1/notification-settings/owners/{ownerId}` → 404: deliberately not implemented (ADR018 records the divergence; Slack owner-policy is a non-goal). Bex's member surface is `GET|PATCH /v1/notification-settings` (optional `?ownerId=`), returning `{"deployStarted":false,"deploySucceeded":false,"deployFailed":true}` for the QA user, identical with the canary `ownerId` named.
- `PATCH {"deployFailed":"yes"}` → 400 naming the Go bool type mismatch. `GET /notification-settings/overrides` → 200 with the Render `[{override:{…},cursor}]` envelope.
- Wart recorded, not filed: `PATCH {"bogusField":true}` answered 200 and zero-filled the view (deployFailed flipped true→false; restored immediately and verified). The cross-surface contract is full-replace — GraphQL `updateNotificationSettings` requires all three booleans (`ReqArg`, `lego/backend/internal/notifications/graphql.go:311`) and the dashboard sends the whole view — but REST silently treats absent as `false` and ignores unknown fields on this bex-native route, unlike the strict unknown-field refusals elsewhere (sweeps 639/667). Off-contract partial bodies can silently disable failure mail; on-contract callers are unaffected.

No bug filed.
