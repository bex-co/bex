# Render maintenance mode contract

**Captured:** 2026-07-15
**Counterpart:** tenant-facing Maintenance Mode for a web service, not Render's platform-scheduled `/maintenance` runs.

This record pins the public Render contract used by bex's implementation. The primary sources are Render's current [Maintenance Mode documentation](https://render.com/docs/maintenance-mode), [Blueprint reference](https://render.com/docs/blueprint-spec), [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), [webhook catalog](https://render.com/docs/webhooks), and [audit-log catalog](https://render.com/docs/audit-logs). Claims not stated by one of those sources are labeled as observations, inferences, or bex policy; they are not presented as captured Render behavior.

## Product and serving contract

- Eligibility is limited to paid web services. A service remains running, is unreachable from the public internet, and remains reachable over Render's private network and SSH.
- Dashboard confirmation takes effect immediately. Disabling the switch restores public traffic immediately; neither action is described as a deploy.
- Every public request returns `503 Service Unavailable` with either Render's default maintenance page or the configured custom page.
- A custom page is an absolute URL and must not be a URL of the service being maintained. Render recommends hosting it on a static site.
- Render fetches the custom content: the visitor is not redirected to the custom URL. A failing custom origin is returned as that error instead of silently falling back to the default page.

The documentation does not specify exact default-page HTML, headers beyond the status, method-specific behavior, fetch caching, redirect limits, timeout, body limit, DNS/IP policy, or whether every header from a custom origin is forwarded. The dated dashboard/default-page images establish appearance and information architecture, not a byte-stable HTML contract.

### bex serving decisions where Render is silent

bex applies maintenance routing to every path and method on both the platform hostname and all custom domains. The public Ingress is repointed to the existing activator responder; the tenant Deployment, pod template, Kubernetes Service, readiness probe, and private DNS path are not changed. Because a standard Ingress backend cannot name a Service in another namespace, an App outside `bex-system` receives an App-owned `ExternalName` routing alias to the fixed platform responder. Traefik accepts these operator-controlled aliases; tenants cannot select their target. The default response is a small neutral HTML page with status 503, `Content-Type: text/html; charset=utf-8`, and `Cache-Control: no-store`.

For custom pages, a successful 2xx/3xx origin body is served with status 503 and its content type. A 4xx/5xx origin status and body pass through. Fetches have a five-second timeout, five-redirect maximum, and 1 MiB response limit. Credentials in URLs, non-HTTP(S) schemes, the maintained service at any redirect hop, and loopback/private/link-local/multicast/unspecified IPs are rejected. Fetch failure, timeout, or oversize returns an explicit 502 rather than the default page. These limits are intentional SSRF/resource-exhaustion hardening where Render publishes no counterpart behavior.

## REST contract

Render's live OpenAPI defines the same referenced schema on `webServiceDetails`, `webServiceDetailsPOST`, and `webServiceDetailsPATCH`:

```json
{
  "maintenanceMode": {
    "enabled": true,
    "uri": ""
  }
}
```

The `maintenanceMode` object requires both `enabled` (boolean) and `uri` (string). An empty URI selects the default page. Consequently a canonical web service read always includes both keys, including the disabled/default state:

```json
{
  "serviceDetails": {
    "maintenanceMode": {
      "enabled": false,
      "uri": ""
    }
  }
}
```

Create and PATCH use that exact nesting. For example:

```http
PATCH /v1/services/srv-example
Content-Type: application/json

{
  "serviceDetails": {
    "maintenanceMode": {
      "enabled": true,
      "uri": "https://status.example.com/maintenance.html"
    }
  }
}
```

The OpenAPI does not publish maintenance-specific HTTP error status/body examples. bex uses its established Render-compatible 400 body for a missing required key, free/non-web placement, malformed URL, credentials, or a URL owned by the same service. A downgrade to `free` is rejected while maintenance is enabled; a PATCH that disables it and downgrades together is accepted in that order. A free-to-paid PATCH applies the plan first, then maintenance.

## Blueprint contract

Render documents:

```yaml
services:
  - type: web
    name: my-service
    maintenanceMode:
      enabled: true
```

`uri` is optional in a Blueprint; omission selects the default page. Setting `enabled: false` disables the mode. Render's public docs do not state whether removing the entire field from a later Blueprint sync resets a dashboard edit. bex follows its established non-destructive Blueprint ownership rule: omission preserves the current state, while an explicit object replaces both values. Maintenance-only syncs patch the App without opening a deploy or changing `restartedAt`. When a manifest uses maintenance mode without a plan, bex selects `starter`, matching Render's documented paid-only example while preserving bex's historical free default for manifests that omit this feature.

## Dashboard capture

Render's 2026-07-15 docs capture places a **Maintenance Mode** section in a web service's Settings. It shows an enabled/disabled status and explanatory text, a toggle that opens a confirmation before public traffic is blocked/restored, and an optional **Custom Maintenance Page** URL editor. The documentation says the feature is available only for paid web services. bex mirrors this section for web services and disables its controls on the free plan with an upgrade explanation. The service header also shows a local status banner while enabled; this is an additional visibility aid, not a claimed Render dashboard capture.

## Activity, audit, and webhooks

Render's webhook catalog contains:

- `maintenance_mode_enabled`
- `maintenance_mode_uri_updated`

Render's audit catalog distinguishes `MaintenanceModeEnabledEvent`, whose metadata `to` is `true` on enable and `false` on disable, from `MaintenanceModeURIUpdatedEvent` for a URL change. A combined two-field update therefore has deterministic URI-then-enabled effects in bex. A no-op emits nothing.

bex maps the two successful Core verbs to those two service-event and outbound webhook type strings. Audit output uses `MaintenanceModeEnabledEvent` with `metadata.to` for both enable and disable, and `MaintenanceModeURIUpdatedEvent` for URI edits. The store adds one typed nullable `maintenance_mode_to` boolean rather than a generic details object, so arbitrary verb arguments—and especially secret values—remain structurally unable to enter the audit trail.

## Lifecycle interactions

Render does not publish maintenance-mode behavior during suspension, plan downgrade, deploy, or auto-sleep. bex pins these rules:

- Deploys proceed and maintenance state persists; a maintenance-only change is not a deploy.
- Maintenance routing has precedence over the auto-sleep activator. In valid API state this is mostly defensive because maintenance is paid-only and bex auto-sleep is a free-plan feature.
- Suspending scales the workload to zero but leaves the public maintenance page reachable. Resuming restores the workload without clearing maintenance mode.
- Downgrading to free requires maintenance to be disabled, including in the same PATCH as described above.

## Executable bex evidence

- CRD/default and exact REST/Blueprint/Core rules: `lego/backend/internal/apps/maintenance_mode_test.go`
- Ingress-only routing and unchanged workload resources: `lego/operator/internal/controller/maintenance_mode_test.go`
- Default/custom responder, origin errors, self redirects, limits, and IP safety: `lego/operator/cmd/activator/main_test.go`
- Dashboard interaction and route gating: `dashboard/src/features/services/components/__tests__/maintenance-mode-section.test.tsx` and `dashboard/src/routes/__tests__/services.$serviceId.settings.test.tsx`

A real Chromium run against the dashboard, a host-run `bex-api`, and the CAPD app cluster exercised enable, confirmation, custom-URL edit, reload persistence, a rejected same-service URL, disable, and a second reload. Only Kratos/Hydra authentication was replaced by a deterministic local harness; GraphQL requests reached the real backend and patched the real Kubernetes App. Reloads read the persisted CR state, and the invalid save displayed the backend error without mutating it. The dated enabled-state capture is stored at the gitignored `.playwright-mcp/maintenance-mode.png`.

Mock-cluster verification on 2026-07-15 used the CAPD app cluster and the repository's host-run controller loop after the deployed manager exposed an unrelated KeyValue secret-informer RBAC failure. A paid `web_service` with platform-equivalent and custom-host routing proved:

- enabling changed both `m37-live-audit.onbex.co` and `m37-live-custom.test` from the App Service to `bex-activator`, while Deployment UID `390f3505-4cf2-4b49-9482-6e0153aa00d7`, generation `1`, pod UID `d1dcc3ff-226e-4d91-b5c5-b506c25539bf`, and template hash `859c667f7d` remained unchanged, and direct Service traffic continued returning the same whoami pod response;
- GET, POST, PUT, and DELETE on arbitrary paths of both hosts returned 503 and `Cache-Control: no-store`; `https://example.com/` returned its fetched body as 503; and a loopback custom origin was rejected with the explicit 502. Automated responder tests additionally pin origin-error pass-through, timeout, oversize, DNS/IP safety, and redirect-to-self behavior;
- suspend scaled the workload to zero while the maintenance responder remained reachable; resume restored one replica without clearing maintenance; and disable restored the original Ingress backend immediately;
- the temporary browser namespace and live App were deleted afterward, and the pre-existing controller deployment was restored to one Ready replica.

A final cross-namespace audit used the production-default `default` App namespace against the responder in `bex-system`. The operator created `bex-maintenance-m37-cross-audit` as an App-owned alias to `bex-activator.bex-system.svc.cluster.local:8888`; both Ingress hosts selected that alias, and the shared responder found the App and returned the default 503. Disabling restored both rules to the tenant Service while Deployment UID `d89c34af-a3f5-43fb-bce3-cde6021b2145`, generation `1`, pod UID `4f12f1a6-5d7c-4089-86e6-c8e097eef660`, and template hash `79b499fcc4` remained unchanged. The App and alias were then deleted and the deployed controller restored Ready.

The official Render CLI exposes no maintenance-mode command or generic service PATCH flag, so there is no unmodified CLI case to run for this counterpart.
