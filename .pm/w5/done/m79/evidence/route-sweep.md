# m79 route-skeleton sweep

Date: 2026-08-25

## Method

The generated `FileRoutesByFullPath` inventory contains 82 patterns: 59 rendering routes, 3 delegated layout/data-dependent routes, 16 immediate redirects, 3 server endpoints, and 1 not-found route. `dashboard/src/common/lib/route-skeleton-manifest.ts` classifies every pattern exactly once; the focused manifest test fails closed when the generated tree and ledger differ.

Two fixed viewports were used throughout: desktop 1440 × 900 and narrow mobile 390 × 844.

- **L — slow navigation:** authenticated `dashboard.bex.co` browser session, with the local working-tree assets fulfilled under the production origin so production auth/API/CSS behavior remained intact. Apollo was cleared, GraphQL was held deterministically, navigation was started through the real TanStack router, pending bounds/regions were measured, requests were released, and the ready frame was measured at the same viewport. The application `pageerror` list was empty. Expected Vite HMR websocket noise from this local-asset harness was excluded.
- **H — browser geometry harness:** actual pending component mounted with production CSS at both viewports and compared with the route's ready DOM/owning component. Used where the QA tenant has no record of that resource family, an authenticated session canonicalizes away from an auth screen, or the route has no blocking loader to hold. These are browser measurements, but they are not represented as live tenant-record walks.
- **D — disposition test:** redirect, delegated layout, server endpoint, or stable not-found route. Automated route tests assert that it has no artificial pending component and that the destination/response owns any loading UI.

`Pass` means the pending frame preserved the ready route's outer container, padding, max width, responsive columns, heading/action or tab slots, and always-present major regions. Representative data-dependent row counts were allowed. No row remained blank, duplicated dashboard/auth chrome, leaked resolved identifiers, overflowed the 390 px viewport, or remained indefinitely pending.

## Route matrix

| Route | Disposition / shape | Desktop | Mobile | Evidence |
| --- | --- | --- | --- | --- |
| `/` | render · `overview` | Pass | Pass | L |
| `/$` | stable not-found | N/A | N/A | D |
| `/agents` | render · `agents-list` | Pass | Pass | H |
| `/billing` | render · `billing` | Pass | Pass | L |
| `/blueprints` | render · `blueprints-list` | Pass | Pass | L |
| `/env-groups` | render · `env-groups-list` | Pass | Pass | L |
| `/healthz` | server response | N/A | N/A | D |
| `/invite` | render · `invite` | Pass | Pass | H |
| `/login` | redirect → `/auth/login` | N/A | N/A | D |
| `/notifications` | render · `notifications` | Pass | Pass | L |
| `/register` | redirect → `/auth/sign-up` | N/A | N/A | D |
| `/settings` | render · `account-settings` | Pass | Pass | H |
| `/usage` | redirect → `/billing` | N/A | N/A | D |
| `/webhooks` | render · `webhooks-list` | Pass | Pass | L |
| `/agents/$agentSessionId` | render · `agent-session-detail` | Pass | Pass | H — no QA session fixture |
| `/api/connected-agents` | server response | N/A | N/A | D |
| `/api/sessions` | server response | N/A | N/A | D |
| `/auth/consent` | render · `oauth-consent` | Pass | Pass | H — requires live Hydra challenge |
| `/auth/device` | layout delegate → `/auth/device/` | N/A | N/A | D |
| `/auth/forgot-password` | render · `auth-recovery` | Pass | Pass | H — authenticated session canonicalizes away |
| `/auth/login` | render · `auth-login` | Pass | Pass | H — authenticated session canonicalizes away |
| `/auth/logout` | render · `auth-logout` | Pass | Pass | H |
| `/auth/reset-password` | render · `account-settings` | Pass | Pass | H — authenticated alias uses the settings geometry |
| `/auth/sign-up` | render · `auth-registration` | Pass | Pass | H — authenticated session canonicalizes away |
| `/auth/verification` | render · `auth-verification` | Pass | Pass | H |
| `/blueprints/$blueprintId` | render · `blueprint-detail` | Pass | Pass | L · `blp-da52hdj8ptnc73bm4uo0` |
| `/blueprints/new` | render · `blueprint-create` | Pass | Pass | L ready + H deterministic pending |
| `/cron/$` | canonical service redirect | N/A | N/A | D |
| `/d/$` | canonical datastore redirect | N/A | N/A | D |
| `/databases/$databaseId` | render · `database-detail` | Pass | Pass | L · `dpg-da52hdb8ptnc73bm4ujg` |
| `/env-groups/$groupId` | render · `env-group-detail` | Pass | Pass | H — no QA environment-group fixture |
| `/keyvalue/$keyValueId` | render · `keyvalue-detail` | Pass | Pass | L · `red-da52hdb8ptnc73bm4uk0` |
| `/keyvalue/new` | render · `keyvalue-create` | Pass | Pass | L ready + H deterministic pending |
| `/new/database` | redirect → `/?new=database` | N/A | N/A | D |
| `/new/project` | redirect → `/?new=project` | N/A | N/A | D |
| `/new/redis` | redirect → `/keyvalue/new` | N/A | N/A | D |
| `/new/workspace` | render · `workspace-create` | Pass | Pass | H |
| `/project/$projectId` | render · active-child frame | Pass | Pass | H — no QA project fixture |
| `/pserv/$` | canonical private-service redirect | N/A | N/A | D |
| `/r/$` | canonical datastore redirect | N/A | N/A | D |
| `/services/$serviceId` | render · service shell/active tab | Pass | Pass | L · four QA service fixtures |
| `/services/new` | render · `service-create` | Pass | Pass | L ready + H deterministic pending |
| `/static/$serviceId` | render · static shell/active tab | Pass | Pass | H — no QA static-site fixture |
| `/static/new` | redirect → `/services/new?type=static_site` | N/A | N/A | D |
| `/u/$` | canonical user redirect | N/A | N/A | D |
| `/w/$` | render · workspace-alias destination | Pass | Pass | H — destination variants exercised |
| `/web/$` | canonical web-service redirect | N/A | N/A | D |
| `/webhook/$webhookId` | render · webhook shell/active tab | Pass | Pass | H — no QA webhook fixture |
| `/webhooks/new` | render · `webhook-create` | Pass | Pass | L ready + H deterministic pending |
| `/worker/$` | canonical worker redirect | N/A | N/A | D |
| `/workspace/settings` | render · `workspace-settings` | Pass | Pass | H |
| `/static/` | redirect → `/` | N/A | N/A | D |
| `/auth/device/success` | render · `oauth-device-success` | Pass | Pass | H — terminal success route has no reusable challenge |
| `/billing/$first/$` | billing/settings compatibility redirect | N/A | N/A | D |
| `/project/$projectId/settings` | render · `project-settings` | Pass | Pass | H — no QA project fixture |
| `/services/$serviceId/disk` | render · `service-disk` | Pass | Pass | L |
| `/services/$serviceId/env` | render · `service-environment` | Pass | Pass | L |
| `/services/$serviceId/events` | render · `service-events` | Pass | Pass | L |
| `/services/$serviceId/headers` | render · `static-edge-rules` | Pass | Pass | H — capability-gated on compute fixtures |
| `/services/$serviceId/logs` | render · `service-logs` | Pass | Pass | L |
| `/services/$serviceId/metrics` | render · `service-metrics` | Pass | Pass | L |
| `/services/$serviceId/plan` | render · `service-plan` | Pass | Pass | L |
| `/services/$serviceId/redirects` | render · `static-edge-rules` | Pass | Pass | H — capability-gated on compute fixtures |
| `/services/$serviceId/scaling` | render · `service-scaling` | Pass | Pass | L |
| `/services/$serviceId/settings` | render · `service-settings` | Pass | Pass | L |
| `/services/$serviceId/shell` | render · `service-shell` | Pass | Pass | L |
| `/static/$serviceId/env` | render · `service-environment` | Pass | Pass | H — no QA static-site fixture |
| `/static/$serviceId/events` | render · `service-events` | Pass | Pass | H — no QA static-site fixture |
| `/static/$serviceId/headers` | render · `static-edge-rules` | Pass | Pass | H — no QA static-site fixture |
| `/static/$serviceId/metrics` | render · `static-metrics` | Pass | Pass | H — no QA static-site fixture |
| `/static/$serviceId/redirects` | render · `static-edge-rules` | Pass | Pass | H — no QA static-site fixture |
| `/static/$serviceId/settings` | render · `static-settings` | Pass | Pass | H — no QA static-site fixture |
| `/webhook/$webhookId/settings` | render · `webhook-settings` | Pass | Pass | H — no QA webhook fixture |
| `/auth/device/` | render · `oauth-device-confirm` | Pass | Pass | H — requires live Hydra device challenge |
| `/project/$projectId/` | render · `project-overview` | Pass | Pass | H — no QA project fixture |
| `/services/$serviceId/` | data-dependent redirect → service destination | N/A | N/A | D |
| `/static/$serviceId/` | data-dependent redirect → static Events | N/A | N/A | D |
| `/webhook/$webhookId/` | render · `webhook-activity` | Pass | Pass | H — no QA webhook fixture |
| `/services/$serviceId/deploys/$deployId` | render · `deploy-detail` | Pass | Pass | H — no stable historical deploy fixture |
| `/static/$serviceId/deploys/$deployId` | render · `deploy-detail` | Pass | Pass | H — no QA static-site fixture |
| `/services/$serviceId/deploys/` | render · `deploys-list` | Pass | Pass | L |
| `/static/$serviceId/deploys/` | render · `deploys-list` | Pass | Pass | H — no QA static-site fixture |

## Representative measurements

Measurements are CSS pixels. Small height differences are bounded text/data variation; structural regions, widths, padding, and responsive breakpoints remain identical.

| Route/family | Desktop pending → ready | Mobile pending → ready | Result |
| --- | --- | --- | --- |
| Top-level list shell (`/blueprints`, `/env-groups`, `/billing`, `/notifications`, `/webhooks`) | outer frame `x256 y48 w1184 h852` → same | outer frame `x0 y48 w390 h796` → same | Exact shell/chrome bounds; no terminal skeleton |
| Blueprint create card | 824 → 826 | 902 → 902 | Exact narrow-mobile reservation; desktop within 2 px |
| Key Value create card | 868 → 862 | 1190 → 1190 | Exact narrow-mobile reservation; desktop within 6 px |
| Webhook create card | 927 → 893 | 973 → 973 | Exact narrow-mobile reservation; desktop keeps identical regions |
| Service create card (web service) | 1858 → 1802 | 2458 → 2458 | Exact narrow-mobile reservation; desktop within 3.2% |
| Postgres overview | 2703 → 2726 | `x16 y172 w358 h2997` → `x16 y168 w358 h3021` | Stable long-page rail/cards; 24 px data-height delta |
| Key Value overview | 1179 → 1188 | `x16 y160 w358 h1725` → `x16 y164 w358 h1729` | Stable long-page rail/cards; 4–9 px delta |
| Service Environment | representative 810 → 810 | 1142 → 1050/1142 by stored rows | Same two editor cards + group card; bounded row-dependent height |
| Service Disk | 390 → 386 | same responsive card structure | Exact disk-empty/add-card silhouette |
| Service Shell | 946 → 964 | 1084 → 1084 | Exact mobile terminal + SSH cards |
| Service Settings | 4394 → 4091 on desktop fixture | `x16 y349 w358 h5265` → `x16 y357 w358 h5389` | Same 10 sections; 2.3% mobile data-height delta; rail is `w390` in both and body/main `scrollWidth=390` |
| Datastore metrics | 768 → 770 | same compact chart structure | Within 2 px |

The create-card browser harness rechecked final mobile heights after responsive tuning: Blueprint 902 px, Key Value 1190 px, and Webhook 973 px, all at card width 358 px with document `scrollWidth=390`. Service settings' pending navigation measured `x0 w390`; the first placeholder was `x16 w112`, matching the ready horizontal scroll rail without page overflow.

Animation is grouped at the route frame rather than starting more than 100 independent pulses on long settings pages. Under normal motion, the frame computes to `animation-name: pulse` and descendant skeleton leaves compute to `none`. Under `prefers-reduced-motion: reduce`, both compute to `none`.

## Render parity record

The repository's current Render artifacts capture ready-state information architecture and geometry, but—with the exception of isolated loading-state notes—do not contain a reproducible capture of Render's transient route preloader. Therefore this milestone uses Render evidence to shape the destination regions and records the transient-loading comparison as unavailable rather than inferring undocumented Render behavior.

| Skeleton family | Current Render evidence | Comparison |
| --- | --- | --- |
| Overview/list/workspace | [`dashboard-routes.md`](../../../../../docs/render-artifacts/dashboard-routes.md), [`workspace-family-walk.md`](../../../../../docs/render-artifacts/workspace-family-walk.md), [`env-groups-live-parity.md`](../../../../../docs/render-artifacts/env-groups-live-parity.md) | Pending geometry follows the captured page header, controls, cards/tables, and workspace chrome. Render's transient preloader was not captured. |
| Create/forms | [`new-service-wizard.md`](../../../../../docs/render-artifacts/new-service-wizard.md), [`key-value.md`](../../../../../docs/render-artifacts/key-value.md), [`new-workspace.md`](../../../../../docs/render-artifacts/new-workspace.md), [`webhooks-ui.md`](../../../../../docs/render-artifacts/webhooks-ui.md) | Source picker, service-type cards, plan grid, project/environment fields, event picker, and action slots mirror captured ready forms. Render's transient preloader was not captured. |
| Service/static detail | [`dashboard-walk/services.md`](../../../../../docs/render-artifacts/dashboard-walk/services.md), [`service-events.md`](../../../../../docs/render-artifacts/service-events.md), [`service-environment-page.md`](../../../../../docs/render-artifacts/service-environment-page.md), [`metrics-page.md`](../../../../../docs/render-artifacts/metrics-page.md), [`disks.md`](../../../../../docs/render-artifacts/disks.md), [`deploy-detail-page.md`](../../../../../docs/render-artifacts/deploy-detail-page.md), [`static-site-page-walk.md`](../../../../../docs/render-artifacts/static-site-page-walk.md) | Header/facts, tabs, deploy tables/timeline/logs, editors, charts, disk card, settings rail, and static exclusions follow captured ready regions. Render's transient preloader was not captured. |
| Datastores | [`dashboard-walk/datastores.md`](../../../../../docs/render-artifacts/dashboard-walk/datastores.md), [`postgres-logs.md`](../../../../../docs/render-artifacts/postgres-logs.md), [`keyvalue-logs.md`](../../../../../docs/render-artifacts/keyvalue-logs.md), [`postgres-ha.md`](../../../../../docs/render-artifacts/postgres-ha.md) | Metadata, connection/networking, plan, HA/recovery, metrics, and logs reserve the captured ready sections. Render's transient preloader was not captured. |
| Billing/notifications/settings | [`billing-onboarding.md`](../../../../../docs/render-artifacts/billing-onboarding.md), [`workspace-plan-change.md`](../../../../../docs/render-artifacts/workspace-plan-change.md), [`notify-on-fail.md`](../../../../../docs/render-artifacts/notify-on-fail.md) | Long-page navigation and always-present cards/actions match the captured ready surfaces. Render's transient preloader was not captured. |
| Auth/OAuth/invite | No current route-loading capture in `docs/render-artifacts/` | Evidence unavailable. These skeletons follow the exact local Ory/Hydra widget, terminal, and action-card frames without claiming Render parity. |

No product-significant ready-state drift was introduced, so no follow-up parity task is required. Cosmetic animation timing is intentionally local and is not presented as a Render behavior match.

## Automated enforcement

- `route-skeleton-manifest.test.ts` parses the generated route interface and requires all 82 paths exactly once, real owner files, tailored rendering owners, skeleton-free non-rendering routes, and nonempty duplicate-free region contracts.
- `route-skeletons.test.tsx` exercises the list, form, auth, datastore, service/static, editor, long-page navigation, mobile-height, create-type, and service/static root-destination shapes.
- `route-pending-transition.test.tsx` holds a deterministic loader, asserts the production Billing skeleton and required regions, releases it, and verifies the ready component replaces it.
- `skeleton.test.tsx` pins pulse/reduced-motion classes, including grouped route-frame animation suppression.
