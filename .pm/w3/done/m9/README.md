# w3 · m9 — Deploy notifications

**Worker:** worker3 **Goal:** email workspace members on deploy success/failure, matching Render's notification-settings surface. **Status:** done — `backend/internal/notifications` (store + service + REST/GraphQL/MCP), the reconciler's `DeployNotifier` hook, and the dashboard Settings → Notifications panel all ship; `/simplify` found and fixed a real correctness-adjacent efficiency issue (notifications were blocking the reconciler's hot path) before this closed out.

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on | status |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | `notification_settings` store: per-workspace/per-member preferences (deploy started/succeeded/failed)   | 45m | —          | — **DONE** (migration `0013_notification_settings`; scoped to `deploySucceeded`/`deployFailed` only — Render's "deploy started" has no request-cycle trigger point in this pass, so it isn't modeled inert; `internal/store/notifications.go`: `GetNotificationSettings`/`UpsertNotificationSettings`/`ListNotifyRecipients`) |
| t002 | Hook into the deploy-status transition path (`w2/m5`) to fire notifications on success/failure           | 45m | t001       | — **DONE** (`internal/store.Reconciler.DeployNotifier`, called from `recordDeploy` only on the pass that actually closes the deploy row — gated on `CloseDeploy`'s own idempotency bool. Backgrounded via a goroutine + `context.WithoutCancel` after `/simplify` found the synchronous call blocking `ReconcileOnce` for every other App in the pass) |
| t003 | Email delivery via the existing `mailer.SMTP` (reuse the `w4/m12` invite-email pattern)                  | 30m | t002       | — **DONE** (`notifications.Service.NotifyDeploy`; best-effort, logged not returned, same shape as `members.sendInvite`; per-recipient identity-lookup + send run concurrently, capped at 8, after `/simplify`'s efficiency finding) |
| t004 | REST/GraphQL/MCP: get/update notification settings, Render-shaped                                        | 45m | t001       | — **DONE** (`GET`/`PATCH /v1/notification-settings`; GraphQL `notificationSettings`/`updateNotificationSettings`; MCP `get_notification_settings`/`update_notification_settings` — self-service, no `workspaceId` arg, same shape as `usage`) |
| t005 | Dashboard: notification preferences in Settings                                                          | 45m | t004       | — **DONE** (`dashboard/src/features/notifications`: Settings → Notifications panel, two switches; GraphQL types hand-spliced into `definitions.ts` from an offline `SCHEMA_JSON` codegen run — same precedent as `ScaleServiceDocument`, since a full regen against this repo's current `.graphql` sources drifts on unrelated pre-existing operations without a live bex-api) |
| t006 | Render parity — verify REST/GraphQL/MCP/UI field/shape consistency vs render.com                         | 30m | t003,t005  | — **DONE** (`docs/ADR018-render-parity.md` Notifications row: ✖✖✖✖ → ✅✅✅✅, evidence + the "deploy started" scope note) |
| t007 | Simplify — `/simplify` over the code this milestone changed                                              | 15m | t006       | — **DONE** (4 agents; reuse/altitude clean; 1 significant efficiency fix applied — see below — plus a dashboard redundant-refetch fix and a toggle-row dedup; 2 findings skipped with reasons: a `caller()` helper extraction that risked a behavior change, and a GraphQL non-null change that would've broken this codebase's consistent nullable-output-field convention) |
| t008 | Test coverage — meaningful tests for the notification-firing + settings behavior                         | 30m | t006       | — **DONE** (Go: `store_pg_test.go` `TestNotificationSettings` against a real Postgres; `notifications/service_test.go` — defaults, update, recipient filtering, non-terminal-status no-op; `reconciler_test.go` `TestRecordDeployNotifiesExactlyOnceOnClose` — race-clean with the backgrounded notify. Dashboard: 3 test files, 11 cases, hooks + panel) |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                                            | 10m | t007,t008  | — **DONE** |

## Definition of done

A deploy failure/success on a service with notifications enabled sends an email to the workspace's members within the same request cycle the deploy-status write happens; preferences are readable/writable on REST, GraphQL, MCP, and the dashboard.

**Met.** Verified hermetically: `go build`/`go vet`/`go test ./...` (backend, including a real-Postgres integration test and a race-detector run of the reconciler hook) and `yarn typecheck`/`yarn lint`/`yarn test`/`yarn build` (dashboard) all pass. No live-cluster acceptance run was needed for this milestone (unlike m5's Loki durability, nothing here requires proving survival across a pod restart) — the reconciler hook, store queries, and every adapter are exercised by the test suite above.

## What shipped

- **Scope trim, stated not hidden**: only `deploySucceeded`/`deployFailed` preferences exist. t001's title mentions "started" (matching Render's real settings surface), but the milestone's own DoD only requires succeeded/failed, and there is no clean request-cycle hook for "started" yet (it would need wiring at `CreateDeploy`/`Trigger` call sites, a separate change) — shipping the field without a trigger would be a silently-dead setting, so it's left out and noted here + in the parity ledger instead.
- **The reconciler notify hook doesn't block reconciliation.** `/simplify`'s efficiency pass caught that a synchronous `NotifyDeploy` call inside `recordDeploy` would serialize 2×N blocking network round-trips (Kratos identity lookups + SMTP sends) into the reconciler's per-pass loop over every App — a slow relay or a large workspace would stall reconciliation cluster-wide, not just for the notifying tenant. Fixed before this shipped: the call is backgrounded (`go ... context.WithoutCancel(ctx)`) and the per-recipient work inside `NotifyDeploy` itself runs concurrently, capped at 8.
- **Dashboard mutation writes straight into the query cache** (`useUpdateNotificationSettings`'s `update` callback) instead of a redundant `refetch()` — one network round-trip per toggle, not two.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12, `docs/ADR018-render-parity.md` "Notifications (deploy/failure alerts)" row.
- **Goal linkage:** pillar 1, completes the deploy-lifecycle surface chain (`w2/m5` → `w2/m10` → `w3/m7` events → this).
- **Expected outcome:** workspace members get emailed on deploy success/failure, matching Render's notification-settings surface.
- **Why now:** both blocking prerequisites (deploy events `w2/m5`, SMTP courier `w4/m7`) are done and otherwise unused by this feature.
- **Render parity closing task:** included — new REST/GraphQL/MCP/UI surface.
