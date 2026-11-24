# w9 · m620 — Billing details are for Admins, Developers and Billing members only, as on Render

**Worker:** worker9 **Goal:** who reads a workspace's billing details on bex matches Render's "View billing details" row: Admin, Developer and Billing, never Contributor or Viewer. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `can_view_billing` (developer, admin, billing): model, `RelCan`, audit CHECK migration, capability grant, ADR012's matrix — **DONE** | 45m | — |
| [t002](done/t002.md) | Usage, cost estimate, billing projection and billing readiness readable with `can_view_billing`; edits stay `can_manage_billing` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Dashboard: Billing and usage for Admin, Developer and Billing; a Contributor and a Viewer see no billing figures — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Role tests on REST, GraphQL and MCP, and a live walk on dev-9 per role — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

On dev-9, an Admin, a Developer and a Billing member read the workspace's usage, its estimated and real cost, its invoices and its billing readiness on REST, GraphQL, MCP and the dashboard. A Contributor and a Viewer get a 403 and see no billing figure anywhere. Payment method, Checkout, Portal and the spend limit stay with Admin and Billing. ADR012's matrix and ADR024 carry `can_view_billing`, and ADR018's members row states the behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w9` (2026-11-23), item 2, verified before filing. Render: role table (render.com/docs/team-members, read 2026-11-23): "View billing details" is ✅ for Admin, Developer and Billing and ❌ for Contributor and Viewer, and "Edit payment method" is Admin and Billing only. The Contributor's and Viewer's descriptions both say "Can't view sensitive fields (billing info, …)". bex: `lego/backend/internal/usage/service.go:288,321-327,381`, `lego/backend/internal/billing/service.go:262`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Developer, the everyday role on Pro, sees the invoices and real cost Render shows them. A Viewer and a Contributor stop seeing usage and estimated cost, which Render treats as sensitive.
- **Why now:** the mismatch runs both ways. Every Pro workspace's Developers miss billing details, and Scale workspaces' Viewers and Contributors (open signup since 2026-11-21) read figures Render hides. API keys follow their creator's role (w9/m481), so the same gap reaches keys.
- **Render parity closing task:** included, because REST, GraphQL, MCP and dashboard reads change.
