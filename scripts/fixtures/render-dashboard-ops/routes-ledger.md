# Fixture URL ledger

A fixture shaped like `docs/render-artifacts/dashboard-routes.md`: prose tables the check does not read, and the URL table it does.

| Resource    | Render path     | bex canonical route    |
| ----------- | --------------- | ---------------------- |
| Web service | `/web/{srv-id}` | `/services/$serviceId` |

## Render's URL inventory

| Render URL | bex | Disposition | Pointer |
| --- | --- | --- | --- |
| `/:param/:param/deploys/:param` | `web.$` and the other service aliases | matched | `dashboard/src/routes/web.$.tsx` |
| `/:param/new` | `/web/new` and the other create pages | matched | `dashboard/src/routes/_serviceCreate.tsx` |
| `/billing` | `/usage` | partial | ADR018 § Billing & invoicing: Usage, not billing |
| `/billing/update-plan` | — | gap | `w3/m708` |
| `/env-group/:param` | — | gap | `w3/m705` |
| `/env-groups` | `/env-groups` | matched | `dashboard/src/routes/env-groups.index.tsx` |
| `/sandboxes` | — | flagged | `sandbox_shell` |
| `/static/new` | `/static/new` | matched | `dashboard/src/routes/_serviceCreate.static.new.tsx` |
| `/w/:param/billing/update-plan-preview` | — | gap | `w3/m708` |
| `/w/:param/settings` | `w.$` alias | matched | `dashboard/src/routes/w.$.tsx` |
| `/w/:param/settings#team-members` | — | gap | `w3/m707` |
| `/w/:param/settings/*` | `w.$` alias | matched | `dashboard/src/routes/w.$.tsx` |
| `/web/:param/*` | `web.$` alias | matched | `dashboard/src/routes/web.$.tsx` |
| `/web/:param/events` | `web.$` alias | matched | `dashboard/src/routes/web.$.tsx` |
| `/web/:param/metrics/:param` | — | gap | `w3/m707` |
| `/web/new` | `/web/new` | matched | `dashboard/src/routes/_serviceCreate.web.new.tsx` |
