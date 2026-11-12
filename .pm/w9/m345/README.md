# w9 · m345 — Render event types for features bex already has: `edge_cache_enabled`/`_disabled`/`_purged` and `pipeline_minutes_exhausted`

**Worker:** worker9 **Goal:** the Events feed and outbound webhooks report edge-cache changes and builds refused for exhausted pipeline minutes with Render's event types, on every surface Render offers them. **Status:** in progress (t001–t007 done and deployed at `f018ce945`; the production check waits on w9/087, because every web service created since w4/m243 is in image-ports mode and never runs the edge cache)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Edge-cache changes and purges record `edge_cache_enabled`, `edge_cache_disabled` and `edge_cache_purged` — **DONE** | 60m | — |
| [t002](done/t002.md) | A build refused for exhausted pipeline minutes records `pipeline_minutes_exhausted` — **DONE** | 60m | — |
| [t003](done/t003.md) | Webhooks: the four types are subscribable; the dashboard picker and Events tab follow Render's sets — **DONE** | 60m | t001, t002 |
| [t004](done/t004.md) | Docs: correct the omission reasons and the ADR018 webhooks row — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity: the four events match Render on REST, GraphQL, MCP, webhooks and the dashboard — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

On production:

- Turning a service's edge cache on, off and purging it each records one event (`edge_cache_enabled`, `edge_cache_disabled`, `edge_cache_purged`) readable on REST, GraphQL and MCP, and a webhook subscribed to it receives a signed delivery.
- A build refused because the workspace used its included pipeline minutes records one `pipeline_minutes_exhausted` event on the service, also deliverable by webhook (verified live if a QA workspace can reach the limit, otherwise by real-Postgres test plus the refusal path on production).
- The dashboard picker lists `pipeline_minutes_exhausted` and leaves the `edge_cache_*` types API-only, as Render's picker does; the Events tab renders all four in en and zh.
- No doc or test excuse claims these types are unsupported.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9` (2026-11-11), item 4, filed by `/pmx` 2026-11-12.
  - `lego/backend/internal/events/vocabulary_test.go:54-55` excuses `SetCacheProfile`/`PurgeCache` because 'Render's event vocabulary has no edge-cache event', but Render's pinned enum lists all three (`docs/render-artifacts/webhooks-api.md:93-95`).
  - Pipeline minutes shipped in w6/m308 (`lego/backend/internal/buildminutes/buildminutes.go`), yet `webhooks-api.md:149` and `service-events.md:99` still say there is no producer.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); ADR018 'Outbound event webhooks' row.
- **Expected outcome:** Render webhook consumers that subscribe to these types receive them from bex unchanged.
- **Why now:** both underlying features have shipped, and the recorded reasons for leaving the events out are now false.
- **Parity:** included (t005), because the change touches REST, GraphQL, MCP, webhooks and the dashboard.

## Production check (2026-11-12, in progress)

- **Fixture:** `srv-dbqrqpg44das73c8s0cg` (`qa-20261112-m345-cache`, a Starter web service in `bex-canary` running `ealen/echo-server:0.9.2` with an explicit port 80) plus webhook `whk-dbqrr9nnb8qs73a4f1b0` subscribed to the three `edge_cache_*` types. Delete both at closeout.
- **What happened:** `setCacheProfile` to `origin-controlled`, then `origin-controlled-all`, then `no-cache` each saved and started a deploy. The purge in between answered `bad request: edge caching is not enabled for this service`, and no `edge_cache_*` event was recorded.
- **Why:** even with an explicit port, the service is in `web-image-v1` mode, so `ServesEdgeCache` is false. That is w9/087's bug, not m345's: m345 reports only caches that actually run. Re-run once 087 ships.
