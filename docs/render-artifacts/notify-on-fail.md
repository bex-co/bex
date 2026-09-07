# Render artifact — Per-service `notifyOnFail`

**Captured:** live OpenAPI spec fetch, 2026-07-14 (`https://api-docs.render.com/v1.0/openapi/render-public-api-1.json` — the same spec `docs/render-artifacts/*` and `render.go` cite elsewhere) + `render.com/docs/notifications` and the "Customizable Service Notifications" blog post (docs-fallback for the parts the OpenAPI spec doesn't cover — dashboard copy/composition rules).

## What Render ships

### The field, exactly as the spec defines it

`components.schemas.service` is **required** to carry `notifyOnFail`, typed `components.schemas.notifySetting`:

```json
"notifySetting": { "type": "string", "enum": ["default", "notify", "ignore"] }
```

It is a **top-level** field on the service object (sibling of `autoDeploy`, `suspended`, …), present on `GET /v1/services/{id}` for every service type (static site, web service, private service, background worker, cron job).

### It is read-only on the service create/update endpoints

Neither `POST /v1/services` (create) nor `PATCH /v1/services/{id}` (update) accepts `notifyOnFail` in their request bodies — confirmed by grepping the spec's `requestBody` schemas for both operations (`toCreateRequest`/PATCH `patchServiceRequest` in bex's own `rest.go` are structured the same way for the fields they DO own). Render's actual write path is a **separate** endpoint pair:

- `GET /notification-settings/overrides/services/{serviceId}` — retrieve a service's override
- `PATCH /notification-settings/overrides/services/{serviceId}` — body `{ previewNotificationsEnabled, notificationsToSend }`

`notificationsToSend` (the thing PATCHed) is its own richer enum: `default | none | failure | all`. `notifyOnFail` on the service object is best read as a **read-side convenience mirror** of that override, narrowed to the failure axis specifically (hence the name) — Render's dashboard/blog describes the same three-state shape in prose ("Use workspace default" / "Only failure notifications" / "All notifications" / "None"), and the owner (workspace) level has its own parallel settings (`GET`/`PATCH /notification-settings/owners/{ownerId}`: `slackEnabled`, `emailEnabled`, `previewNotificationsEnabled`, `notificationsToSend` — no `default` option at the owner level, since there's nothing above it to defer to).

### Composition (from the docs, since the OpenAPI spec doesn't encode precedence)

Two tiers: workspace/owner defaults, and a per-service override that can defer to them (`notificationsToSend: "default"`) or replace them (`none`/`failure`/`all`). Render's docs don't describe a third, per-member tier — Render's notification settings are workspace/service-scoped, not individual-member-scoped (unlike bex's own w3/m9 model, see below).

## bex parity decisions

bex now implements Render's service-level `notificationsToSend` policy and exact override endpoint while retaining the existing member-scoped email settings as the workspace-default layer:

| Decision | bex |
| --- | --- |
| Authoritative field | `App.spec.notificationsToSend`, values `default` \| `none` \| `failure` \| `all` |
| Render REST | `GET`/`PATCH /v1/notification-settings/overrides/services/{serviceId}`; PATCH accepts `notificationsToSend`. `previewNotificationsEnabled` is returned as `default`; non-default writes are rejected rather than ignored. |
| Other surfaces | GraphQL `setNotificationsToSend`, MCP `update_service(notificationsToSend:)` (w1/m71 folded `set_notifications_to_send` into it), and the Service Settings selector all use the same four-state policy. |
| `default` | Defers to each member's deploy-started/succeeded/failed preferences. A missing member-settings row defaults to failure-only. |
| `none` | Suppresses every deploy lifecycle email for the service. |
| `failure` | Sends only failed-deploy email to every resolvable workspace member. |
| `all` | Sends deploy-started, succeeded, and failed email to every resolvable workspace member. |
| Legacy compatibility | Existing CRs with no `notificationsToSend` retain the narrow `notifyOnFail` behavior: `ignore`/`notify` affect failures only, while started/succeeded still defer to member preferences. The legacy setter clears the richer field; richer-policy writes maintain a read-side `notifyOnFail` projection. |
| Default migration | Schema defaults and absent settings rows are failure-only. Rows still equal to the former all-enabled default are migrated to failure-only; customized rows are not overwritten. |
| Unknown value | `core.ErrBadRequest` (named 400), matching bex's `SetPlan`/`normalizeType` enum-validation convention |

## Email content (w7/m44)

The policy above decides _who_ is emailed; this section is _what_ the deploy email says. Render's captured deploy-failure email (2026-07-17) carries three things beyond a bare status line: an impact-framing sentence, the failing **commit message**, and a **View Logs** link.

> We encountered an error during the deploy process for backend-v2. This means your deploy didn't complete successfully and your latest changes may not be live. Commit: `<full commit subject + body>` View Logs

bex now matches that shape (`internal/notifications/deployEmail`). The failure body:

```
Subject: Deploy failed: backend-v2

We encountered an error during the deploy process for "backend-v2". This means
your deploy didn't complete successfully and your latest changes may not be live.

Commit:
<the deploy's commit subject + body>

View logs:
https://dashboard.bex.co/services/backend-v2/deploys/dep-…
```

- **Impact framing** matches Render's register per lifecycle kind: failed ("encountered an error … may not be live"), succeeded ("is live … now serving"), started ("has started …").
- **Failure reason** (w7/m79, a **bex extension over Render's captured email**) follows the framing on the failed path only: the operator's actionable diagnosis — the same string `deploys.failure_reason` and every API surface carry, e.g. a crash loop with the `$PORT` hint, an image-pull failure, or an unresolvable Secret/ConfigMap reference naming the missing object. Omitted for a failure the operator could not diagnose, so a blank reason leaves the email byte-identical to before. Rationale: this email is the first thing most people read when a deploy breaks, and it named the commit and linked the logs without ever saying what went wrong — the platform already knew.
- **Commit block** is included when the deploy has a commit (repo-backed builds — `Deploy.CommitMessage`); an image-backed deploy with no commit omits the block rather than printing an empty `Commit:` label.
- **View Logs** is the deploy-detail deep link (`<BEX_DASHBOARD_URL>/services/<service>/deploys/<deployId>`, which renders build/deploy logs), omitted when `BEX_DASHBOARD_URL` is unset — the same honest-omit the workspace-invite email uses for its link.
- **Consistency**: the reconcile-time failure/succeeded path carries commit + link (the deploy row is in hand); the request-time started path has no deploy row, so its email carries the framing only ("when available" — never a broken or half link).

## Dashboard override (w5/m60 verification)

The per-service notification override **is already wired in the dashboard** — the Settings → Notifications section renders `ServiceNotificationsRow`, a four-state edit-in-place select (`default | all | failure | none`, Render's captured vocabulary) backed by the authoritative `setNotificationsToSend` verb and reading `notificationsToSend`. So the w5/m60 miner's "`setNotifyOnFail` is dashboard-unconsumed" finding is accurate but **intentional**: `setNotifyOnFail` is the legacy narrow (`default | notify | ignore`) setter that _clears_ the richer `notificationsToSend` field (see the Legacy-compatibility row above), so wiring it in the UI would be a parity regression. The override closure was therefore already satisfied by the authoritative setter; the legacy verb stays deliberately unconsumed (available to REST/CLI clients that still send it). No dashboard change was needed for this closure.

## Divergences from Render, stated

- Workspace defaults remain member-scoped instead of Render's owner-wide email/Slack configuration.
- Slack and preview-environment notification behavior are not implemented. The service endpoint exposes `previewNotificationsEnabled: default` only.
- bex includes deploy-started email in `all`; Render's public enum does not enumerate individual event types.
- Email is **plain-text** (the shared `mailer.SMTP` relay), where Render's is HTML; the "View Logs" target is bex's own deploy-detail route. The service name is quoted (`"backend-v2"`) where Render's prose is unquoted — cosmetic.

## Live verification (2026-09-06)

On production, an owned disposable Web Service saved each service policy (`all`, `failure`, `none`, `default`) through Settings → Notifications. After each reload, both the visible option and REST `notificationsToSend` matched the selected value. Evidence: `.playwright-mcp/w5-notifications-result.json` and `w5-notifications-roundtrip.png`; fixture deleted afterward (GET 404). This verifies the shared service-policy control; historical cron trigger/detail evidence remains in `w5/done/029.md`.
