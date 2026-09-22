# w4 · m128 — Deploy-notification preferences are stored and applied per workspace but can only be read and written for one of them

**Worker:** worker4 **Goal:** one answer to "which workspace do my deploy-notification preferences belong to", implemented the same way in the store, the mail fan-out, and every surface — so turning deploy emails off actually stops them. **Status:** done 2026-09-21 (live re-probe of the deployed fix deferred to the next QA pass — no production access this session)

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Decide the scope of a member's notification preferences and write it into ADR024      | 40m | —          | — **DONE**
| t002 | Make the API reach the row the mail fan-out actually reads                            | 45m | t001       | — **DONE**
| t003 | Reconcile MCP's caller-scoped classification with the decision                        | 30m | t001       | — **DONE**
| t004 | Dashboard: make the Notifications panel agree with the decided scope                  | 30m | t002       | — **DONE**
| t005 | Render parity check (owner-scoped route + the ADR018 Notifications row)               | 30m | t002, t003, t004 | — **DONE**
| t006 | Simplify (`/simplify` over the changed code)                                          | 30m | t005       | — **DONE**
| t007 | Test coverage                                                                         | 45m | t005       | — **DONE**
| t008 | Closeout                                                                              | 15m | t007       | — **DONE**

## Definition of done

- **Reading and writing preferences reaches the row that governs mail.** For an account in more than one workspace, the settings read/written through REST, GraphQL and MCP are the same `(tenant_id, subject)` row that `ListNotifyRecipients` joins for that workspace. Today the API can only ever reach one workspace's row.
- **Turning deploy emails off works in every workspace the member belongs to.** After setting `deployFailed: false`, a failing deploy in *any* of the member's workspaces sends no failure mail — or, if t001 decides preferences stay per-workspace, the member can set each workspace's value and each takes effect independently.
- **No unreachable rows.** Every row `notification_settings` can hold for a member is reachable by that member through at least one surface. This is checkable directly: for each workspace the caller belongs to, a read returns that workspace's stored values.
- **MCP agrees with the rest.** `get_notification_settings` / `update_notification_settings` are either workspace-bound like every other resource tool, or the caller-scoped classification is correct because the store is caller-scoped too. The current split — MCP says caller-scoped, the schema and fan-out say per-workspace — cannot survive.
- **The parity ledger is accurate.** ADR018's Notifications row records what bex actually implements: whether `/v1/notification-settings/owners/{ownerId}` exists, and if not, why the unscoped route is the right shape.
- **Tests fail without the fix.** A test asserting a member's preference in a non-default workspace is readable and writable fails against the pre-fix code.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 145 (w4-targeted, `muse.env` credentials), notifications surface. Read-only; nothing was created. The account belongs to three workspaces (`bex`, `tian-personal`, `bex-canary`). Evidence in [t001](t001.md); in one line: the storage is `UNIQUE (tenant_id, subject)` and the mail fan-out joins on it, while `notificationSettings` takes **no** workspace argument on GraphQL, `GET /v1/notification-settings/owners/{tea-id}` is **404** on all three workspace ids, and `GET /v1/notification-settings` returns a single unscoped object.
- **Goal linkage:** the product's honesty about a control a user sets and then relies on — a preference that silently does not apply is worse than no preference. Touches [ADR024](../../../docs/ADR024-members.md)'s member-scoped surfaces and [ADR018](../../../docs/ADR018-render-parity.md)'s Notifications row, which is currently ✅ on all four surfaces.
- **Expected outcome:** a member who turns off deploy-failure email stops receiving it, in every workspace where they would otherwise get it; and a member who wants different preferences per workspace can express that. Which of those two is the promise is t001's decision — today neither is true.
- **Why now:** this is not a missing feature, it is two halves of the product disagreeing about the same data, and the half that wins is the one that sends email. `w3/m9` built the store deliberately per-workspace ("one member's override of their **per-workspace** deploy-email preferences", migration `0013_notification_settings`); the API it shipped alongside never grew the argument to address it. The gap has been invisible because a single-workspace account cannot observe it, and the parity audit that blessed the row (`w5/m59`, 2026-07-30) was explicitly **code-based with live Render captures infrastructure-blocked**, so nothing has checked this against Render's actual shape since.
- **Render parity task included** because the fix either adds an owner-scoped route and arguments across REST/GraphQL/MCP and the dashboard, or changes what the ledger claims.
- **DO_NOT_DO constraints honored:** no anti-goal touched. The Slack-delivery half of notifications stays a recorded non-goal (round 14) and is not in scope; this milestone is only about which workspace a preference belongs to.
