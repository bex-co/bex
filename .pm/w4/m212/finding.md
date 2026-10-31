# Disabled webhook intervals replay according to dispatcher progress

**Severity: major.** Functional QA a675, 2026-10-16 UTC, signed-in dashboard in workspace `tea-d98210cbbpdc73dcrkvg`. Disabling a webhook and later enabling it can deliver an event that happened while disabled. The same endpoint skips such an event when a second owned endpoint lets dispatch consume it first. Integrations cannot rely on one consistent disable interval.

## Reproduction and controls

Owned Free receiver `srv-db8r60i9cl7s73ae49gg` (`qa-20261016-a675-receiver`); A `whk-db8r6529cl7s73ae49i0`; B `whk-db8r9evdbg8c73dghbpg`. Both subscribe to `deploy_ended` and POST to the receiver's `/webhook`, which answers 204 and records only this pass's service name. All are deleted.

1. From A's Settings, disable it, restart the owned receiver, and wait for Live. Read its source event and A's delivery history before enabling A.
2. Enable A. Read history and the receiver again without triggering another deploy.
3. Repeat after a fresh Settings navigation with B also disabled.
4. Control: keep B enabled while A is disabled; restart the same receiver, wait for B's delivery, then enable A and compare the exact event ID.

| Case | Disabled state before restart | Source event | Re-enable and observed result |
| --- | --- | --- | --- |
| First failure | A confirmed false 05:21:21.909; B not yet created | `evt-5efb2thu6il0ilvaqvnh`, deploy `dep-db8r8li9cl7s73ae49qg`, ended 05:21:43 | No attempt at 05:21:58.337; enable acknowledged 05:21:59.983; delivered 05:22:00, HTTP 204; actual receiver recorded it |
| Fresh repeat | B false 05:24:50.246; A false 05:24:53.698 | `evt-2lefv4v4l7npuq3mqfj7`, deploy `dep-db8raai9cl7s73ae4a30`, ended 05:25:13 | Both false and neither history contains it at 05:25:30.281; enable A 05:25:32.102; A delivered 05:25:32, HTTP 204; receiver recorded it |
| Passing control | B true; A false 05:23:31.763 | `evt-e78r5s21vnvac7d1ck4h`, deploy `dep-db8r9m29cl7s73ae49v0`, ended 05:24:02 | B delivered 05:24:06; enable A 05:24:10.451; A still lacks that event at 05:24:45.126 |

These are newly created notifications after enabling, not retries of notifications accepted before disabling. A's pre-enable history proves that distinction. The control proves that unconditional replay is not the current contract either. Only owned workspace endpoints were changed; the platform-wide endpoint catalog and live worker cursor were not inspected.

An enabled control restart produced `evt-avprr9hkc6702kv1nelc` at 05:22:43 and delivered 05:22:46. Earlier, a synthetic `/fail` callback correctly produced HTTP 503 with `QA_A675_FAIL`; after changing the URL back, native Resend queued attempt 2 with the same event and byte-identical payload, delivered 204, retained the failed first attempt and marked its logical parent delivered. Preserve that working attempt behavior.

## Durable evidence

[evidence-a675.json](evidence-a675.json) contains the exact native mutations and complete selected GraphQL requests/responses, receiver receipts, source hashes, and cleanup verification. It excludes signing secrets, credentials and unrelated resource contents. The first false-state read follows a controlled-switch automation assertion that returned too early; the authoritative endpoint read confirms the accepted disable, and that action was not repeated.

Local screenshots were inspected: `.playwright-mcp/qa-a675-mobile-history.png` (fresh 390px history, document width 390) and `.playwright-mcp/qa-a675-replay-history-desktop.png`. They illustrate history rendering; event-time and enable-time proof lives in the durable JSON. Final pre-cleanup API census: no failed/pending requests; console: zero errors. These captures do not assert cryptographic signature validation.

## Root cause on refreshed main

- `lego/backend/internal/webhooks/worker.go:296–313`: RunOnce returns when no endpoints are enabled. It reads only the current enabled set.
- `:367–403`: dispatch tails one shared cursor, filters source rows to workspaces with enabled endpoints, and commits projection plus cursor together.
- `:421–435`: fanOutPage checks subscription type and the endpoint's original CreatedAt, with no eligibility state for disabled intervals. The observed old event is newer than CreatedAt, so a later pass can admit it.
- `lego/backend/internal/store/webhooks.go:49–73,267–319,334–348`: the store model and both enabled-state writers retain only current enabled state, creation and ordinary update times. The dispatch read selects id, tenant, event types and created_at. Updating unrelated Name/URL also changes updated_at, so that timestamp cannot safely stand in for activation history.
- `lego/backend/internal/store/feedtail.go:45–82,111,187–233`: each process caches its cursor; quiet windows park at one-minute intervals. Consumed rows advance it even if a particular endpoint gets no notification. Which disabled-period rows remain available when A returns therefore depends on dispatch progress. This explains the passing B control; the precise live cursor/replica schedule is unmeasured.

The schema cannot express source-time eligibility intervals today. Fix the endpoint/dispatch state, rather than changing history rendering, broadening filters, or making the shared cursor chase the clock every poll. Advancing a global cursor alone cannot enforce a per-endpoint promise under other workspaces or stale worker caches.

## Target behavior and fix boundaries

**Disabling closes that endpoint's source-event eligibility interval; enabling opens a new interval. A new notification must not be created for an event whose source time falls inside a disabled interval, independently of other endpoints, quiet parking, worker restarts or replica lag.** Events during an enabled interval retain their existing at-least-once guarantee, including enabled-period source events not yet dispatched at the toggle. Already queued automatic/manual attempts pause while disabled and retain their existing recovery and identity semantics on re-enable.

Persist enough endpoint eligibility history/generation state to express those intervals, with a safe migration for existing enabled/disabled rows, exact source-time boundary semantics and retention compatible with delayed feed rows and stale caches. Use the source row's full clock/tie-breaking contract, not the second-rounded public timestamp. A lone replacement of CreatedAt with last-enabled-at would discard pre-disable, undispatched eligible events; it is insufficient for this target. Close automatic-disable intervals as well. Preserve already admitted notification rows rather than re-filtering them against a new activation time.

Keep this specific to webhook fan-out/admission. `store.TailFeed` has exactly two production callers: webhook dispatch and push notifications; push eligibility and generic cursor behavior remain unchanged. Keep batching, transactional enqueue/cursor advancement, unique endpoint/event dedupe, fairness/caps, bounded retry evidence and cleanup. Coordinate toggle/admission races and two stale worker caches with the durable eligibility decision; do not infer success from a currently enabled snapshot alone.

### Callers, aliases and resource family

The exhaustive production census found one ListEnabledWebhookEndpoints invocation (RunOnce), one service call to SetWebhookEndpointEnabled, and one to UpdateWebhookEndpoint; the store also disables endpoints from CompleteWebhookAttempt at `store/webhooks.go:1250`. Creation is `webhooks/service.go:449–494`.

- Dashboard Status uses GraphQL `setWebhookEndpointEnabled` → Service.SetEnabled (`service.go:623`).
- GraphQL `updateWebhookEndpoint(enabled:)`, REST `PATCH /v1/webhooks/{id}` and MCP `update_webhook_endpoint` use Service.Update (`:795`). Create variants also accept enabled; initially disabled creation needs the same first-enable rule.
- Canonical REST create/list/read/delete/history/resend are in `webhooks/rest.go:140–265`; RegisterREST defines no legacy endpoint route aliases. Inbound `/v1/webhooks/git` is a different component. REST uses `eventFilter`; GraphQL/MCP use `eventTypes`; retain that dialect.
- Web, static, cron, worker, private, Postgres and Key Value source events share this dispatcher. Only Free web deploy-ended was exercised live. Type projection and payload vocabulary stay unchanged; other families need regression coverage without a new per-family mechanism.

No dashboard pre-settle fix is required: this is accepted backend state and later dispatch, while the UI correctly renders each immutable attempt. Keep the Status control pending until its mutation settles and preserve existing errors. Failed/forbidden/unauthenticated/not-found toggles must not change interval state; DB/transient failures keep existing error behavior. No new resource existence distinction or permission change is required.

## Dedupe and parity

Whole-board open/blocked/done searches for disabled replay, re-enable, watermark and backfill found no owner. Reviewed all 78 active milestone titles (excluding evidence directories), DO_NOT_DO, the last 40 dashboard/lego commits, target-file history and origin/main through `c84c348b0e4a2248ee0f80a09cc15165275e14d7`.

Related guarantees remain: w3/done/m11's CreatedAt guard excludes pre-creation events, not disabled intervals; w1/done/m58 owns worker dedupe; w2/done/m70 owns the event/filter contract; w8/done/m25 owns immutable attempts/resend; w4/done/m83 owns fair bounded admission. None records this interval behavior. The unrelated ServerRestarted observation was rejected against w4/done/m122's explicit store-less-only design. No anti-goal matches.

[Render's current webhook documentation](https://render.com/docs/webhooks) supports 2xx/15-second acknowledgment and retry/re-enable behavior. It does not settle disabled-interval replay; no authenticated Render comparison was run. This finding establishes a bex consistency defect, not an unsupported claim of exact Render semantics.

**Unverified:** REST/MCP toggle mutations live; initially disabled creation; automatic-disable recovery; pre-disable undispatched backlog; concurrent updates; multiple live worker replicas; late facts and other resource types; Chinese UI. These are explicit implementation/verification work, not observed failures. Signature verification, emails and exhausting the 33-hour retry schedule were not exercised.
