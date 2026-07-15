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

bex's existing notification system (w3/m9, `docs/ADR018-render-parity.md` § Notifications) is **member-scoped**, not workspace-scoped: every member has their own `deploySucceeded`/`deployFailed` booleans, and `NotifyDeploy` emails whichever members opted in. Render's `notificationsToSend`/owner-settings richness (Slack, "all" vs "failure-only", preview-environment toggle) has no bex equivalent and is out of scope here — this milestone ships exactly the field the task names, `notifyOnFail`, with Render's exact name and enum, layered on top of bex's existing member-preference model rather than replacing it with Render's separate-endpoint/two-tier design:

| Decision | bex |
| --- | --- |
| Field name + enum | `notifyOnFail`, values `default` \| `notify` \| `ignore` — byte-identical to Render's `notifySetting` |
| Placement | Top-level on the service object (`spec.notifyOnFail`), matching Render's service-object placement — **not** a separate overrides endpoint (Render's `notificationsToSend`/`previewNotificationsEnabled` richness is not modeled) |
| Settable via | Create (`POST /v1/services` body, `serviceDetails`/create request) and `PATCH /v1/services/{id}` (top-level, alongside `autoDeploy`) — a deliberate simplification of Render's real write path (the separate `/notification-settings/overrides/services/{id}` endpoint), consistent with this task's own instruction to wire it through the existing create/PATCH surface rather than add a new endpoint family |
| Scope | **Failure only** — the field's own name says so, and it composes with bex's per-member `deployFailed` preference, never `deploySucceeded`. A success email is unaffected by `notifyOnFail`; it always follows each member's own `deploySucceeded` preference, matching w3/m9. |
| `default` | Defers entirely to bex's existing member-preference behavior (each opted-in member gets the failure email) — the composition rule the milestone's DoD calls out |
| `ignore` | Suppresses the failure email for this service for **every** member, regardless of individual `deployFailed` preference — the DoD's "mute one flaky cron" case |
| `notify` | Forces the failure email to every workspace member with a resolvable email, regardless of individual `deployFailed` opt-out — the symmetric override to `ignore` (Render's `notificationsToSend: "all"` plays the analogous force-on role at its layer) |
| Unknown value | `core.ErrBadRequest` (named 400), matching bex's `SetPlan`/`normalizeType` enum-validation convention |

## Divergences from Render, stated

- No `notificationsToSend`/`previewNotificationsEnabled` owner- or service-level richness, no Slack channel, no separate overrides endpoint — bex's per-member preference model (w3/m9) already covers the workspace-default tier in its own way (opt-in per person, not per workspace), and `notifyOnFail` is the one additional per-service knob this milestone's DoD asks for.
- Render's `notifyOnFail` is documented as read-only on the service object and driven by a separate endpoint; bex makes it directly settable on create/PATCH for surface consistency with every other spec field (`autoDeploy`, `healthCheckPath`, …) and because bex has no equivalent to Render's separate notification-overrides endpoint family to route through instead. A client reading `GET /v1/services/{id}` sees the same field name/enum Render returns; a client trying to PATCH it against real Render would 400 (Render silently ignores unknown PATCH fields, per `rest.go`'s own `patchServiceRequest` doc comment) — noted as a divergence, not hidden.
