# Render audit-logs API — captured response schema

Captured 2026-07-15 (w4/m26) from Render's current [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), corroborated by the unmodified official [`render-oss/cli`](https://github.com/render-oss/cli) at `72b3fbd59068ae84d024ec2ded9df6b27dc8dd68` (generated client types in `pkg/client/types_gen.go` match the spec; the CLI ships **no** user-facing audit-log command). When bex's audit surface was built (w4/m10, 2026-06 era) this schema was not resolvable from public docs — api-docs.render.com's reference page then documented only the request parameters. The spec has since been extended; this capture closes ADR018's last evidence-blocked ◐.

A live capture against a real Render account was not performed in this session (no `RENDER_API_KEY` credential surfaced in the environment; the w5/m32 real-account workflow can add one later) — the OpenAPI schema below is normative (required fields + closed enums) and two independent artifacts agree on it.

## Endpoints

- `GET /v1/owners/{ownerId}/audit-logs` — operation `list-owner-audit-logs` (bex's surface; workspace-scoped)
- `GET /v1/organizations/{orgId}/audit-logs` — operation `list-organization-audit-logs` (org layer; out of bex scope — bex has no org above workspace)

Query params: `startTime`/`endTime` (RFC3339), `direction` (shared `$ref` with `/logs`: enum `forward|backward`, default `backward` = most recent first), `cursor`, and an **audit-specific** `limit` (`auditLogLimitParam`: default 20, **maximum 1000** — not the general pagination cap of 100).

## Response: array of `auditLogWithCursor`

```json
[{ "cursor": "…", "auditLog": { … } }]
```

Both members required. `auditLog` (all six fields **required**):

| field | type | notes |
| --- | --- | --- |
| `id` | string | example `aud-123456789` |
| `timestamp` | string (date-time) | ISO 8601 |
| `event` | string, closed enum | PascalCase `…Event` names, e.g. `CreateServerEvent`, `SuspendServiceEvent`, `UpdateIPAllowListEvent` — 75 values in the capture, including org/team/SSO/SCIM events bex has no counterpart for |
| `status` | string, enum `success\|error` | binary |
| `actor` | object | `{type (required, enum user\|rest_api\|system), email?, id?}` — example id `usr-123456789` |
| `metadata` | object, `additionalProperties: string` | free string map; spec example `{"service": "srv-123456789", "field": "env_vars"}` |

## bex alignment (w4/m26)

`internal/audit/rest.go` now emits exactly this shape: `event` (1:1 verb→event mapping for the unambiguous lifecycle verbs, bex `<feature>.<Verb>` passthrough otherwise — a documented extension of the enum), `status` `success|error` with a denial mapped to `error` **and** preserved as `metadata.outcome: "denied"` (bex records authorization denials, which Render's binary enum cannot distinguish from runtime errors), `actor` `{type, id}` (`session`→`user`, `oauth2`→`rest_api`, system/unattributed→`system`; `email` omitted — the audit row deliberately stores only the subject), and always-present string-map `metadata` carrying the audited target keyed by kind (`service`/`database`/`keyvalue`/`envGroup`) plus the maintenance-mode `to` flag. **Payment method (w2/067, 2026-11-03):** a completed setup Checkout (the dashboard's Add/Update payment method) writes `billing.UpdatePaymentMethod` → `UpdateWorkspacePaymentMethodEvent` from the Stripe webhook, attributed to the member who opened the session (`bex_actor`/`bex_actor_method` session metadata), one row per session however often Stripe redelivers; no card details are stored. A default-card change made in the Stripe Billing Portal (w2/070, 2026-11-04) arrives as `customer.updated` and writes the same verb once per event, attributed to the member who last opened the portal (`bex_portal_actor` Customer metadata, within an hour) or else `system`; the Checkout's own change is not recorded twice (`bex_bound_payment_method`). **Reads and one row per action (w4/m321, 2026-11-11):** `ViewEnvVarValuesEvent` (any read returning env var values, `metadata.service` or `metadata.envGroup`) and `ViewConnectionInfoEvent` (Postgres/Key Value connection info, `metadata.database`/`metadata.keyvalue`) are recorded once per successful request; reads gated by a write relation that change nothing (billing status, the audit log itself, invites, key lists, agent-session reads) record denials only; the create/delete/move/webhook/disk/Blueprint/build-source verbs map to Render's names, and a delete or move records one row. See ADR018's audit row for the full list and the env var write decision. `TestRenderEventNamesAreInThePinnedEnum` checks every mapped name against the pinned enum. The store's limit cap was raised to Render's 1000. `resource` (the OpenFGA object authorized against) remains a bex extra field. GraphQL `auditLogs` keeps bex's own flat dialect (`actor` string, `action`, `status: denied`) for the dashboard — Render has no public GraphQL to diverge from.
