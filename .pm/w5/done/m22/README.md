# w5 · m21 — Health Check Path in service Settings

**Worker:** worker5 **Goal:** Wire the `healthCheckPath` field end-to-end: operator ReadinessProbe (w1/m23/t001), backend SetHealthCheckPath verb (REST PATCH + GraphQL mutation + MCP), dashboard Settings row (web_service + private_service only). **Status:** DONE 2026-07-12 — all four surfaces complete; isWorker predicate added; typecheck clean.

## Tasks (in order)

| id   | title                                                                                                                       | est | depends_on | status |
| ---- | --------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Operator: wire `app.Spec.HealthCheckPath` into ReadinessProbe on web containers (w1/m23/t001)                               | 20m | —          | DONE   |
| t002 | Backend: `SetHealthCheckPath` service verb + REST PATCH + GraphQL mutation + MCP tool                                       | 40m | t001       | DONE   |
| t003 | Backend: `healthCheckPath` in AppView, `view()`, REST/GraphQL response shapes                                               | 20m | t002       | DONE   |
| t004 | GraphQL: `health-check-path.graphql` mutation + `healthCheckPath` field added to server.graphql                             | 15m | t003       | DONE   |
| t005 | Dashboard: `HealthCheckPathRow` component (inline pencil-edit, coerce blank to "/")                                         | 30m | t004       | DONE   |
| t006 | Dashboard: add `isWorker` predicate + wire `HealthCheckPathRow` in ServiceSettingsPage (web+private only)                   | 20m | t005       | DONE   |
| t007 | Locale: en + zh strings (label, hint, placeholder, success/error toasts)                                                    | 15m | t005       | DONE   |
| t008 | definitions.ts: hand-add SetHealthCheckPath Document types + healthCheckPath in ServerQuery + ServerDocument                 | 20m | t004       | DONE   |
| t009 | local-bex stub: SetHealthCheckPath handler + healthCheckPath seeded on SERVICE stub                                         | 10m | t008       | DONE   |

## Definition of done

Health Check Path is editable from Settings for web_service and private_service; hidden for cron_job, background_worker, static_site; the operator uses the saved path as the ReadinessProbe HTTP path (defaults "/" when empty); typecheck clean; local-bex stub reflects changes immediately.

## Source + Goal linkage

- **Source:** `w5/009.md` (inbox note 2026-07-12); gate `w1/m23/t001` materialized in this session.
- **Goal linkage:** pillar 1 (Render health-check parity) + platform reliability (liveness before traffic routing).
