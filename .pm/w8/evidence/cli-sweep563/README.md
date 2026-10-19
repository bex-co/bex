# CLI sweep 563 — notification settings: two enums, one policy (owned fixture, REST boundary)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T17:0x–17:36Z. Owned fixture `qa563-notify` (`srv-dbb594dt4o5c73dqgbr0`, free hello-app, reached `live`) exercised via same-credential REST (`/tmp/qa501/notify563.py`) because the pinned CLI exposes no notify flags — CLI reads corroborate (`notifyOnFail` on the create response). Fixture deleted via CLI and proven absent (REST 404; workspace 8). These are boundary reads/writes on an owned resource, in the sweep-544 sense.

Contract (pinned OpenAPI + `lego/types/v1alpha1/notification_policy.go`): legacy per-service `notifyOnFail` ∈ `default|notify|ignore` (service PATCH) and authoritative `notificationsToSend` ∈ `default|none|failure|all` (override routes). Writing either re-derives the other; a legacy write clears the stored override.

- **Round-trips exact in all six transitions:** initial `default/default` → legacy `ignore` ⇒ `ignore/none` → override `all` ⇒ `notify/all` → legacy `notify` ⇒ `notify/failure` (the documented lossy step: legacy has no `all`, and the write clears the override) → override `none` ⇒ `ignore/none` → override `default` ⇒ `default/default`.
- **Refusals named, state unmoved:** bogus values both routes (service lists its enum: "notifyOnFail must be one of default|notify|ignore"; override returns the boundary's `invalid request body at /notificationsToSend`), cross-wired enums refused both ways (`notify` into notificationsToSend, `failure` into notifyOnFail), wrong JSON type refused at the field. Empty-string `notifyOnFail` acts as an explicit reset to `default` (verified from a non-default state — it moved the pair); empty-object override PATCH is a 200 no-op (verified from `failure`).
- **Overrides list:** `GET /notification-settings/overrides?ownerId=<canary>` returns one row per service (8, fixture included, `{override:{type,serviceId,previewNotificationsEnabled,notificationsToSend}, cursor}`); the repeated `serviceId` filter narrows to the fixture. Override GET/PATCH on synthetic and Postgres ids → uniform 404.
- **Owner-level route is a documented omission, not a bug:** `GET /notification-settings/owners/{workspace}` 404s for the real workspace too; `retrieve/patch-owner-notification-settings` are allowlisted omissions in `render_operation_coverage_test.go` pointing at ADR018 § Render operation omissions.
- **Harness note:** the raw-REST helper's `X-Bex-Workspace` header does not scope list reads (the unscoped list answered with a different member workspace's 6 services; by-id reads were unaffected). First read as a missing-row anomaly, resolved by `?ownerId=…` — probe artifact, read-only throughout, nothing outside `bex-canary` was mutated.

## Unrun

- Actual notification **delivery** on a failed deploy under each policy (no observable mailbox in this harness; settings state is the verifiable surface).
- `previewNotificationsEnabled` transitions (read as `default` throughout; not written).
- Owner-default PATCH (documented omission above; also a shared-workspace mutation).
