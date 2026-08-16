# w8 · m18 — Blueprint estimated pricing panel (Render blueprint/new parity)

**Worker:** worker8 **Goal:** the `blueprints/new` review step (and the existing-Blueprint sync view) shows a Render-style "Estimated pricing" panel — per-resource `(PlanName) $X / month` rows, a `Total $X per month` footer, variable-cost exclusions — computed server-side from bex's own price sheet. **Status:** done

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Forward monthly estimator in `internal/pricing` (declared plans → $/month) — **DONE** | 45m | — |
| t002 | Attach `estimatedPricing` to the `BlueprintPreview` core payload — **DONE** | 45m | t001 |
| t003 | Expose `estimatedPricing` over GraphQL · REST · MCP preview adapters — **DONE** | 30m | t002 |
| t004 | Dashboard "Estimated pricing" panel on the blueprints/new review step — **DONE** | 45m | t003 |
| t005 | Render parity check (REST/GraphQL/MCP/UI vs dashboard.render.com/blueprint/new) — **DONE** | 30m | t004 |
| t006 | Simplify (`/simplify` over the changed code) — **DONE** | 30m | t005 |
| t007 | Test coverage (estimator table tests + preview payload + golden render.yaml) — **DONE** | 45m | t005 |
| t008 | Closeout — **DONE** | 15m | t007 |

## Definition of done

Previewing a blueprint with paid plans (e.g. `bex-co/discourse_docker` `_infra/bex/bex-beancount.yaml`: `standard` web + `standard` keyvalue + `basic-1gb` database) on `blueprints/new` renders an Estimated pricing panel with one row per paid resource showing plan label + bex monthly price, database rows including provisioned-storage cost, a correct monthly total, and variable costs (autoscaling / multi-instance / cron) listed but excluded from the total with a footnote. An all-free blueprint renders no panel. The same `estimatedPricing` object is returned by the REST and MCP preview verbs. All prices come from `lego/backend/internal/pricing/pricing.yaml` via the API — nothing hardcoded in the dashboard.

## Source + Goal linkage

- **Source:** deep-research session 2026-08-04 (user request: bex `blueprints/new` shows only resource names while `dashboard.render.com/blueprint/new` shows an Estimated pricing panel totaling $177.50 for the same stack). Research decoded Render's shipped dashboard bundles (`IACSync-*.js`): plan catalogs fetched per resource type + client constants ($0.30/GB Postgres storage, HA ratio 1, tier-default disk sizes, free plans filtered, cron/autoscaling excluded as "variable"). bex side: the ADR049 blueprint compiler already parses `Plan`/`NumInstances`/`DiskSizeGB`/`HighAvailability`/`ReadReplicas` (`lego/backend/internal/apps/deploy.go:221-317`) with Render-matching defaults (omitted plan → `starter`, omitted DB plan → `basic-256mb`), and the w8/m7 price sheet (`internal/pricing/pricing.yaml`, ADR030: 30% off Render) has every rate — but `blueprintValidationPlanFromIR` (`blueprint.go:848-871`) strips the preview to name-only lists, so nothing priceable reaches the dashboard.
- **Goal linkage:** Render parity (docs/ADR018-render-parity.md — Blueprint review surface) + the ADR030 pricing pillar; the visible 30%-below-Render total turns a parity gap into a selling point at the exact moment a user decides to create paid resources.
- **Expected outcome:** blueprint users see what a stack costs per month before creating it, on bex's prices, across all preview surfaces.
- **Why now:** directly follows the just-landed ADR049 blueprint compiler (w1/m63) whose parsed stack makes the estimate a projection rather than new parsing, and reuses the w8/m7 price sheet — both dependencies are fresh; the review screen is the last blueprint step with a visible parity hole. Render parity task included: this is feature work changing GraphQL/REST/MCP payloads and the dashboard UI.
- **DO_NOT_DO constraints honored:** persistent service disks are a deliberate non-goal (ADR018 `—` row), so there is **no Disks group** for service `disk` blocks — only database provisioned storage (a supported, metered surface) is priced. Estimate-only remains the boundary: no payment collection changes.
