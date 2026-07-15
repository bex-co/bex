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
| Other surfaces | GraphQL `setNotificationsToSend`, MCP `set_notifications_to_send`, and the Service Settings selector all use the same four-state policy. |
| `default` | Defers to each member's deploy-started/succeeded/failed preferences. A missing member-settings row defaults to failure-only. |
| `none` | Suppresses every deploy lifecycle email for the service. |
| `failure` | Sends only failed-deploy email to every resolvable workspace member. |
| `all` | Sends deploy-started, succeeded, and failed email to every resolvable workspace member. |
| Legacy compatibility | Existing CRs with no `notificationsToSend` retain the narrow `notifyOnFail` behavior: `ignore`/`notify` affect failures only, while started/succeeded still defer to member preferences. The legacy setter clears the richer field; richer-policy writes maintain a read-side `notifyOnFail` projection. |
| Default migration | Schema defaults and absent settings rows are failure-only. Rows still equal to the former all-enabled default are migrated to failure-only; customized rows are not overwritten. |
| Unknown value | `core.ErrBadRequest` (named 400), matching bex's `SetPlan`/`normalizeType` enum-validation convention |

## Divergences from Render, stated

- Workspace defaults remain member-scoped instead of Render's owner-wide email/Slack configuration.
- Slack and preview-environment notification behavior are not implemented. The service endpoint exposes `previewNotificationsEnabled: default` only.
- bex includes deploy-started email in `all`; Render's public enum does not enumerate individual event types.
