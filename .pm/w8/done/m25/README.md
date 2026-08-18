# w8 · m25 — Webhook immutable attempt history + manual Resend

**Worker:** worker8 **Goal:** preserve every automatic and manual delivery attempt as immutable evidence and give operators Render's request/response inspection and Resend workflow **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Split logical notifications from immutable delivery attempts — **DONE** | 75m | — |
| t002 | Rework dispatch, retry, and retention over the two-level model — **DONE** | 75m | t001 |
| t003 | Expose attempt-level history and exact evidence on every API surface — **DONE** | 60m | t001, t002 |
| t004 | Correct pending, successful, and failed attempt semantics — **DONE** | 45m | t003 |
| t005 | Add authorized manual Resend across REST, GraphQL, and MCP — **DONE** | 60m | t002, t003 |
| t006 | Build the attempt-detail and Resend dashboard experience — **DONE** | 75m | t003, t004, t005 |
| t007 | Add safe activity polling and new-attempt reconciliation — **DONE** | 45m | t006 |
| t008 | Harden Resend concurrency, auditing, limits, and cleanup — **DONE** | 45m | t004, t005 |
| t009 | Extend live verification for failure, retry, and Resend — **DONE** | 45m | t007, t008 |
| t010 | Render parity — **DONE** | 30m | t009 |
| t011 | Simplify — **DONE** | 30m | t010 |
| t012 | Test coverage — **DONE** | 60m | t010 |
| t013 | Closeout — **DONE** | 15m | t011, t012 |

## Definition of done

One source event creates one logical endpoint notification, while every automatic retry and authorized manual Resend creates an immutable attempt with its own attempt ID, exact send time, status/transport error, bounded response, and request payload reference. Retries retain the same source `eventId`, byte-identical body, and Standard Webhooks identity while minting the correct per-send timestamp/signature. REST history preserves Render's documented envelope; GraphQL/MCP expose equivalent attempt semantics and Bex retry diagnostics. Failed attempts appear immediately under Failed even while a later retry is pending. The dashboard lists every attempt, expands request JSON and response/error evidence, shows exact and relative time, polls without duplicating keyset pages, and offers Resend only when authorized and safe. Multi-replica workers, retention, endpoint deletion, auto-disable, audit, metrics, and migration/backfill are proven by real-Postgres and live-receiver tests.

## Source + Goal linkage

- **Source:** Authenticated Bex↔Render webhook audit on 2026-08-17. Render's dashboard describes Recent deliveries as every attempt, exposes request JSON and the endpoint response, and offers Resend. Bex currently keeps one mutable row per `(endpoint_id,event_id)`, overwrites the latest outcome, terminal-only filters failures, and exposes no replay verb.
- **Goal linkage:** ADR008 pillars 1 and 3: reliable automation needs inspectable, replayable delivery evidence through Render-compatible REST plus the same core semantics for GraphQL, MCP, and the dashboard.
- **Expected outcome:** Operators can explain and recover any failed webhook without database access; retries are an auditable sequence rather than a lossy counter; agents can perform the same recovery through an authorized API.
- **Why now:** m24 makes webhook IDs useful, but the current in-place retry model structurally prevents the live Render behavior and destroys forensic evidence. The schema must be corrected before more dashboard polish hardens around it. Render parity is included as t010 because the milestone changes storage, worker behavior, REST/GraphQL/MCP, and the dashboard.
- **Anti-goal boundary:** This milestone does not expand event vocabulary, re-return signing secrets, alter endpoint plan caps, or trigger service/deploy mutations merely to manufacture test events. Live tests use a caller-controlled HTTPS receiver and disposable resources.
