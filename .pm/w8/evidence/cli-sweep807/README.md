# CLI sweep 807 — notification owner-level non-goal + member defaults (live, 2026-10-20)

Read-only probes; nothing created. No new bug filed.

- `GET /notification-settings/owners/{tea}` → 404. Deliberate, per [ADR018](../../../docs/ADR018-render-parity.md): "Render's `/v1/notification-settings/owners/{ownerId}` is deliberately NOT implemented" (owner-level Slack policy is a recorded non-goal; member preferences take the `?ownerId=` parameter instead).
- `GET /notification-settings` (member defaults) → `{"deployStarted":false,"deploySucceeded":false,"deployFailed":true}` — the documented failures-only default for untouched rows.
- `GET /owners` lists the account's three workspaces with cursor envelopes; `GET /users` returns the QA identity with `name:""` — the documented pre-trait degradation (w4/m25), not an error.
