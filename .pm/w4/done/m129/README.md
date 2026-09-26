# w4 · m129 — A re-created service inherits the deleted service's Activity feed

**Worker:** worker4 **Goal:** a service's activity feed and its outbound webhook deliveries describe operations performed on **that** service — identified by the row that cannot be reused — so deleting a service ends its history and creating one with a name someone used before starts an empty one. **Status:** done (production replay passed 2026-09-26, `/qa-find-bugs` passes 190–191)

## Tasks (in order)

| id   | title                                                                                      | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Decide the key and the migration shape: what identifies a service to an audit-derived event — **DONE** | 40m | —            |
| t002 | Re-key the audit arm of `serviceEventsQuery` and of its webhook twin — **DONE** | 50m | w4/m129/t001 |
| t003 | Place the rows already written name-only, and the three name spellings the twin recognizes — **DONE** | 35m | w4/m129/t001 |
| t004 | Gate `apps.SetIdleTTL`, which `w4/m122`'s own audit misclassified as unable to fail — **DONE** | 35m | —            |
| t005 | `serviceEvents` returns an event `serviceEvent(id:)` denies, and its default window hides it — **DONE** | 30m | w4/m129/t002 |
| t006 | Render parity — the feed and the webhook payloads across REST/GraphQL/MCP/UI — **DONE** | 30m | w4/m129/t003, w4/m129/t004, w4/m129/t005 |
| t007 | Simplify — `/simplify` over the code this milestone changed — **DONE** | 25m | w4/m129/t006 |
| t008 | Test coverage — inherited history cannot come back, and a refused verb writes no event — **DONE** | 45m | w4/m129/t006 |
| t009 | Closeout — close the milestone once the definition of done actually holds — **DONE** | 15m | w4/m129/t008 |

## Definition of done

Each bullet is a probe run live, in the shape pass 156 ran them.

- **A re-created service starts with an empty feed.** Create a free web service, change its idle timeout three times, delete it, then create a new service **with the same name**. `serviceEvents(serviceId: <new id>, startTime:"2026-09-01T00:00:00Z", endTime:<now>)` returns **only** the new service's own rows — today it returns the deleted service's three `idle_timeout_changed` rows as well, plus one from an even earlier service of that name. Verified live 2026-09-21: the re-created service reported **seven** `idle_timeout_changed` events older than its own `createdAt`, the oldest 4h45m older.
- **A never-used name is unaffected, and stays unaffected.** The control from the same pass must keep passing: a service created with a name no service in the workspace has held returns **zero** events older than its `createdAt`.
- **The deploy-derived arm is not regressed.** `deploy_started`, `deploy_ended`, `service_hibernated` and `service_woken` are keyed by `app_id` today and correctly did **not** carry over — they must still appear, in the same order, for the service that produced them. This is the regression risk of t002: the fix must not re-key the arm that is already right.
- **Outbound webhooks carry the same guarantee.** A webhook endpoint subscribed to the service-event types receives no delivery describing an operation on a deleted service, for a service that merely reuses its name. `webhookEventsQuery`'s audit arm joins on the target name today (`store/webhooks.go:708-720`), so it inherits the same defect and must be fixed with it — this is stated as a DoD bullet because it is a customer-visible delivery, not an internal read.
- **A refused `setIdleTimeout` writes no event.** `setIdleTimeout(id:…, idleTTLSeconds: 999999999)` answers `bad request: idleTTLSeconds must be 0-604800` and adds **zero** rows to the feed. Today it adds one `idle_timeout_changed`, verified live in isolation: one refused call took the feed from 6 events to 7.
- **A no-op `setIdleTimeout` is decided explicitly, not by accident.** Setting the value it already has (60 → 60) mints an event today. t004 must state whether that stays (an accepted write is a write) or is suppressed, and `w4/m122`'s doctrine comment must say which — the doctrine already names "an idempotent no-op" as routine, so silence here is what produced the defect.
- **`w4/m122`'s classification is corrected where it is wrong, not just where it bit.** The comment at `events/service.go:320-329` asserts `apps.SetIdleTTL` has "no validation, type gate or no-op path at all". It has both a post-authorization bounds check (`apps/service.go:3450-3452`) and a no-op path. The two verbs named alongside it (`apps.SetDisplayName`, `deploys.RegenerateDeployHook`) must each be re-checked and the comment corrected to match what the code does. The three verbs in the neighbouring "validates BEFORE it authorizes" bucket were re-read this pass and **are** correctly placed — `SetIPAllowList`, `SetNotifyOnFail` and `SetNotificationsToSend` all validate ahead of `s.patch` — so do not churn them.
- **The by-id read and the list agree.** Every id `serviceEvents` returns resolves through `serviceEvent(id:)`. Today the inherited row is the only one of eleven that answers `EVENT_NOT_FOUND`, which is how the defect was first noticed.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 156 (w4-targeted, `muse.env` credentials), journey 15 (free-tier sleep → wake), which this loop had never swept. Fixtures `qa-20260921-sleep` (twice, `srv-daovjqbbdpcs73f5ec60` then `srv-daovpa3bdpcs73f5ecdg`) and the control `qa-20260921-zzq7neverused` (`srv-daovov3bdpcs73f5ecbg`), all deleted; workspace returned to baseline.
- **Goal linkage:** ADR038 (events) and the Activity feed's contract as a truthful record of what happened to **a** service. Direct continuation of **`w4/m122`** (2026-09-21), which established "an attempt is not an accomplishment" and whose t004 was required to classify all 48 `eventTypes` verbs with "no entry left unclassified" — t004 of this milestone is that audit being wrong about one verb, in the direction its own doctrine warns about. Adjacent to **`w4/120`** (a revived blueprint inheriting its predecessor's sync history) and **`w4/129`** (usage rows naming resources that no longer exist): three instances of the same underlying shape, a record keyed by a reusable name outliving the thing it described.
- **Expected outcome:** a user reading a service's Activity feed sees only that service's history, and a webhook consumer building on the feed can trust that an event about `srv-X` describes `srv-X`. Deleting a service becomes a real end to its record, which is also what makes "delete and recreate" a usable recovery step rather than one that silently imports a stranger's history.
- **Why now:** the mechanism is confirmed with a clean control rather than inferred, so the expensive part of the work is already done. It is also cheap to hit by accident and impossible to notice: names are recycled constantly (every `qa-` fixture, every `web`/`api`/`worker` a team recreates), the inherited rows land **below** the fold in reverse-chronological order, and the API's default time window hides them — so the feed looks right until someone widens the range. And because `webhookEventsQuery` shares the join, the wrong attribution is already being **delivered** to subscribers, not just displayed.
- **Render parity task included** — the event vocabulary and the feed are exposed on REST, GraphQL and MCP (`GET /services/{id}/events`), rendered by the dashboard Activity tab, and delivered by outbound webhooks, so the change touches all four surfaces.

## Evidence (live, 2026-09-21 pass 156)

**1. The mechanism, with its control.** Both services are new rows — the re-created one got a **different** id (`srv-daovpa3bdpcs73f5ecdg` ≠ `srv-daovjqbbdpcs73f5ec60`), so this is name reuse, not id reuse:

```
create "qa-20260921-sleep"            → srv-daovjqbbdpcs73f5ec60, createdAt 03:45:45Z
  setIdleTimeout ×6 (3 accepted, 3 refused), hibernate, wake
delete it
create "qa-20260921-sleep" again      → srv-daovpa3bdpcs73f5ecdg, createdAt 03:57:29Z
  serviceEvents(wide range) → 8 events, 7 of them OLDER than createdAt:
      idle_timeout_changed 2026-09-22T03:52:31Z   ← the deleted service's
      idle_timeout_changed 2026-09-22T03:52:21Z
      idle_timeout_changed 2026-09-22T03:52:12Z
      idle_timeout_changed 2026-09-22T03:48:46Z
      idle_timeout_changed 2026-09-22T03:48:45Z
      idle_timeout_changed 2026-09-22T03:48:45Z
      idle_timeout_changed 2026-09-21T23:09:32Z   ← an EARLIER service of the same name

control: create "qa-20260921-zzq7neverused" → srv-daovov3bdpcs73f5ecbg, createdAt 03:56:44Z
  serviceEvents(same wide range) → 1 event, ZERO older than createdAt
```

Note **which** events crossed over: only the audit-derived `idle_timeout_changed`. The deleted service's `deploy_started`, `deploy_ended`, `service_hibernated` and `service_woken` did **not** — they are keyed by `app_id`. That asymmetry is the finding, and it is also the fix's specification.

**2. The root cause, stated by the code itself.** `store/events.go:464-467`, the doc comment on `ListServiceEvents`:

```go
// appID is the app's control-plane row id (deploys are keyed by it); target is
// core.ServiceTarget(appName) (audit rows are keyed by it) — the two sources key
// on different identifiers for the same service, which is why both are passed
// rather than derived here.
```

and the audit arm of the CTE, `WHERE ((a.target = $2 AND a.workspace_id = ANY($3)) …`, with `$2` = that name-derived target. The deploy arms use `WHERE d.app_id = $1`.

**3. The webhook twin shares the join.** `store/webhooks.go:708-720` — _"The audit arm joins on the target name scoped to the app's own tenant"_ — and it recognizes **three** name spellings (`<tenantID>-<appName>`, the legacy `<tenantName>-<appName>`, and a bare app-name fallback), all name-based, so t003 must place all three.

**4. The by-id divergence, and the control run against all eleven rows.** Ten of eleven resolve and report the queried service; the inherited one is the only failure:

```
service_woken          2026-09-22T03:54:12Z  byId=OK/sameSvc
idle_timeout_changed   2026-09-22T03:52:31Z  byId=OK/sameSvc
   … eight more, all OK/sameSvc …
idle_timeout_changed   2026-09-21T23:09:32Z  byId=NOT_FOUND   ← EVENT_NOT_FOUND
```

**5. A refused write mints an event.** Isolated, one call at a time, counting the feed between each:

```
baseline                                    → 6 events (3 idle_timeout_changed)
setIdleTimeout 999999999 → REFUSED
   "bad request: idleTTLSeconds must be 0-604800"
                                            → 7 events (4)   ← +1 for a refusal
setIdleTimeout 60 (already 60) → ACCEPTED   → 8 events (5)   ← +1 for a no-op
setIdleTimeout 90 → ACCEPTED                → 9 events (6)   ← +1, correct
```

The refused call changed nothing: `idleTTLSeconds` read back unchanged after each rejection.

**6. The default window hides the inherited row.** `serviceEvents(serviceId:…, limit:50)` with no range returned **10** events; the same query with an explicit `startTime`/`endTime` returned **11**; the dashboard Activity tab header read **11** and rendered the extra row as "Idle timeout updated · 4h". So the row is user-visible while the API's own default answer omits it.

## Not part of this milestone

- **The audit row's own `targetName` is null and its `resource` is the workspace** for `apps.SetIdleTTL` (`auditLogs` at `2026-09-21T23:09:32Z` reads `res=workspace:tea-…`, `target=null`, `metadata=null`). That is the audit **read surface** not projecting what the store's `target` column holds; `w4/done/111.md` already carries the related observation that a forensic row with a null `targetName` states a weaker claim than its wording implies. Re-file separately if it matters; the feed fix does not need it.
- **Cross-tenant attribution by shared name** is a *documented* caveat of the same join (`webhooks.go:713-719`: a default-workspace caller's write on a name two tenants share "attributes to both"). It is a deliberate, recorded trade-off, not this defect. Do not silently change it while re-keying — t001 must say whether the new key ends it as a side effect, and if so, that is a separate decision to surface.

## Decision (2026-09-21)

Use the existing `service_event_index.app_id` as the audit event's immutable owner in both service and webhook feeds. Migration 0083 already captures that association at insertion and the by-id read uses it; deleting an app cascades its index entries while retaining the raw audit evidence. `core.AuthorizeApp` has already fetched the App, including `LabelAppID`, before its authorize-and-audit call, so managed writes can record `service:<srv-id>` without another lookup. Name-only targets remain a compatibility path for unmanaged CRs and older replicas.

Reject a new generation column (duplicates the immutable app id), name-only lifetime joins (re-resolve ownership on every read and can misattribute a delayed write), and revival (creating a new service is not reviving its predecessor). A forward migration accepts typed targets, limits legacy name resolution to rows at or after the live app's creation, and removes impossible pre-creation index associations without deleting audit rows or guessing a backfill. Unindexed old audit evidence stays available in the audit log, but is not presented as a current service event.

Five read-time name predicates disappear: the primary target and `LegacyTarget` in the service list, and tenant-id-prefixed, tenant-name-prefixed, and bare names in the webhook query. All three historical spellings remain supported by the insert trigger under the lifetime guard. Datastore events retain their existing immutable dpg-/red- targets.

New managed writes using typed ids cease the documented default-workspace shared-name ambiguity as an explicit consequence; historical name-only/default-workspace associations retain the existing policy. The API keeps its Render-compatible one-hour default window; the dashboard deliberately requests a wider explicit window. Add the default to GraphQL field documentation rather than narrowing the UI's history.

## Verification and remaining gate (2026-09-21)

Full backend tests pass with real Postgres and OpenFGA, backend lint reports zero issues, and both list/webhook mutation checks reject inherited rows in the replacement generation. Simplify's three reviews were applied. Raw historical audits remain intact; only invalid service ownership associations are removed. Three pre-existing timing-dependent test fixtures were made deterministic while validating the full suite.

The remaining gate is t009's production replay after the release pipeline deploys this change. Local database and transport tests establish the implementation contract, not production rollout or delivery. Keep this milestone blocked until that replay is recorded.
