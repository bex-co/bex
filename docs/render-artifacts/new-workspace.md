# Create workspace (`/new/workspace`) — Render vs bex

Pinned **2026-08-19** for `w9/m88`. Complements [workspace-lifecycle.md](workspace-lifecycle.md) (switcher / settings) and [workspace-plan-change.md](workspace-plan-change.md) (plan-card anatomy). Unauthenticated hits of both dashboards bounce to login with `next=/new/workspace` (`.playwright-mcp/render-new-workspace.png`, `bex-new-workspace.png`). This note records the **signed-in create form**, not that login wall.

> Playwright MCP was not connected in the implementation session, so a fresh authenticated screenshot pair was not captured here. The Render layout below is from the 2026-07-12 authenticated plan-change capture plus Render's documented create steps (plan type + payment method, then Create Workspace). The bex layout is the shipped `/new/workspace` page (`dashboard/src/routes/new.workspace.tsx`). When MCP is available, dump authenticated full-page shots as `.playwright-mcp/bex-new-workspace-auth.png` and `render-new-workspace-auth.png` (bare filenames).

## bex's shipped shape

- **Route:** `GET /new/workspace` (chrome on, `FormPageSkeleton` pending). Create still GraphQL `createWorkspace(name, plan)` only.
- **Heading:** page-level `h1` **Create a workspace** (not a nested settings card).
- **Slug:** labelled **Workspace slug**, helper that it is used in URLs/resource names, live `WORKSPACE_NAME_RE` (DNS label 1–30). Not Render's freeform display name.
- **Plans:** 2×2 radiogroup, Hobby preselected. Catalog fees from `lego/backend/internal/pricing/pricing.yaml` (ADR030, 30% off Render): **`$0/mo` / `$17.50/mo` / `$349.30/mo` / Custom terms**. Capability bullets from `store.LimitsFor` (Hobby: 1 member, 25 services, 5 Hobby/user; Pro/Scale unlimited members/services; Scale extra roles). Usage footnote: resource tiers billed separately. Same `PlanPicker` as the change-plan dialog.
- **Payment Method:** shown for Pro / Scale / Enterprise. Reuses `BillingOnboardingView` against the **current** workspace (the new `tea-*` does not exist yet). Create is disabled only when `paymentMethodRequired` (the `BEX_REQUIRE_PAYMENT_METHOD` gate, now on `workspaceBillingReadiness`) and that workspace is not `paymentMethodReady`. Hobby never gated. **No** licensed Stripe Price for $17.50 / $349.30 is attached on create (`BillableMeterNames` stays usage-only).
- **Footer:** Cancel → `/`, Create Workspace → select returned id → `/`. Failure stays on the form with an inline error.
- **Switcher:** Billing → `/billing`, Workspace Settings, name + plan sublabel, **+ New Workspace**.

## Comparison

| Topic | Render | bex | Verdict |
| --- | --- | --- | --- |
| Page `h1`, slug/name, large plan cards, Create/Cancel | Authenticated `/new/workspace` | Same anatomy | Match (shape) |
| Plan prices | `$0` / `$25` / `$499` / contact | `$0` / `$17.50` / `$349.30` / Custom terms | **Deliberate** — catalog 30% off, not a bug |
| Included bandwidth / custom domains / build minutes on cards | Render plan marketing | Omitted — those are usage meters, not workspace SKUs | Deliberate |
| Name | Freeform | DNS-label slug | Deliberate (App CR names); follow-up only if we add a display name |
| Payment panel | Card-on-file for paid plans; charges the workspace SKU | Panel + ADR046 card gate; **no** SKU collection | Shape match, collection follow-up (`.pm/w9/050.md`) |
| Enterprise | Disabled “Get in touch” | Selectable | Deliberate (no sales flow); follow-up in `.pm/w9/050.md` |
| Switcher | Billing, settings, name+plan, New Workspace | Same, Billing at `/billing` not `/w/{id}/billing` | Match enough; per-`tea` billing URL not invented |
| REST / MCP create | None | None (`/v1/owners` read-only; MCP `list_workspaces` only) | Match |

Do not treat the 30%-off sticker as drift to “fix” back to Render’s $25 / $499.
