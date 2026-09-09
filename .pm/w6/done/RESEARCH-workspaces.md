# Render workspace lifecycle — verified research (2026-07-08)

Deep-research report (100-agent harness: 5 search angles → 18 primary sources → 86 claims extracted → 25 adversarially verified, 24 confirmed / 1 refuted). Basis for the w6 milestones. All quotes verified against live pages on 2026-07-08.

## Confirmed findings

1. **Plan lineup (critical framing).** On 2026-04-23 Render replaced legacy per-seat plans (Professional $19/member/mo, Organization $29/member/mo) with flat-rate **Hobby ($0) · Pro ($25/mo flat) · Scale ($499/mo flat) · Enterprise (custom)**; paid plans include **unlimited members**. Legacy workspaces force-migrate 2026-08-01. Clone the new lineup, not Hobby/Professional/Organization. — render.com/docs/new-workspace-plans, changelog/updated-plans-for-render-workspaces, /pricing
2. **Creation flow.** Dashboard workspace dropdown (top of left pane) → **+ New Workspace** → pick plan type + payment method → **Create Workspace**. Payment step likely skippable for Hobby (docs say "including", not "requires"; Hobby needs no card). — render.com/docs/team-members
3. **Switcher & multi-workspace.** Same dropdown switches workspaces. Cap: **five free Hobby workspaces per user**, unlimited paid. On Scale/Enterprise a member can hold a **different role per workspace**. — docs/team-members
4. **Hobby constraints.** Free (usage-only billing), exactly **1 member** (invites require Pro+), **25 services max** per workspace (incl. suspended). — docs/new-workspace-plans, docs/platform-features-by-plan
5. **Roles are plan-gated.** Pro: Admin, Developer. Scale/Enterprise: + Contributor, Viewer, Billing, plus org-level Owner/Member/Guest (orgs exist only on Scale+). Hobby: no roles (single member). — /pricing, docs/team-members, docs/organizations
6. **Billing linkage.** Flat subscription hangs off the workspace + usage (bandwidth, per-second compute). Subscription fee **waived in months with no services and no activity**. — /pricing, docs/faq
7. **REST models workspaces as `owners`.** `GET /v1/owners` = "List workspaces" (filter by name/email); `GET /v1/owners/{ownerId}` = "Retrieve workspace"; `GET /v1/owners/{ownerId}/members` = "List workspace members". Workspace IDs prefix **`tea-`**. Passing a **user ID returns that user's default workspace** (no error). — api-docs.render.com/reference/list-owners, retrieve-owner, retrieve-owner-members
8. **User-ID prefix is `own-`, not `usr-`** (medium confidence — single doc page; cross-check the OpenAPI spec). — api-docs/reference/retrieve-owner
9. **No workspace-mutation API.** The REST surface (and the OpenAPI-generated official CLI client) has **no POST/PATCH/DELETE on /owners** — workspace create/rename/delete is dashboard-only. Parity ⇒ bex keeps REST owners **read-only**; lifecycle mutations live in the dashboard GraphQL. — render.com/docs/api, github.com/render-oss/cli
10. **API keys are user-scoped** (one key reaches all the user's workspaces; created in Account Settings, not per workspace); every resource object carries **`ownerId`** for workspace scoping. — api-docs/reference/authentication, render.com/docs/api

MCP (from fetch stage, tool shapes consistent across repo/docs/registries): official server ships **`list_workspaces`** (no params), **`select_workspace`** (required `ownerID` string), **`get_selected_workspace`** (no params); selection is stateful and scopes subsequent tool calls. — github.com/render-oss/render-mcp-server, render.com/docs/mcp-server

## Refuted

- "Pro allows only 1 workspace; Scale/Enterprise unlimited" — **killed 1-2**. The only confirmed workspace-count limit is 5 Hobby/user; do not assume per-plan paid workspace counts.

## Open questions (feed into w6/m1 t001, w6/m2 t001)

1. **Delete/rename semantics unverified** — what happens to services/datastores on delete, confirmation guards, grace period, ownership transfer. Dashboard-only surface; needs live capture.
2. **Owner object schema** beyond `id` (name, email, type `user|team`, twoFactorAuthEnabled?) — enumerate from the OpenAPI spec; also confirms `own-` vs `usr-`.
3. MCP selected-workspace persistence details (where stored, per-session vs per-config).
4. Per-plan paid-workspace counts (see refuted claim).

## Caveats

Research lands mid-transition (new plans 2026-04-23, forced migration 2026-08-01) — live dashboards may still show legacy plans on old workspaces. render.com/pricing was JS-shell; quotes corroborated via search index + docs pages.
