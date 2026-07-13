# Render workspace plan change — live dashboard capture + bex's decided shape

Captured **2026-07-12** live from `dashboard.render.com` against a real authenticated session (`puncsky@gmail.com`), on the `stargately` workspace (`tea-c185th5c2rvvnhbfiltg`, **Professional (Legacy)** plan). Screenshot: `.playwright-mcp/render-change-plan.png` (the plan-picker page with **Pro** selected). This is the capture `w6/m12/t001` calls for; it follows the method and doc shape of `workspace-lifecycle.md` (w6/m5).

> **Scope note.** The flow was driven up to (but not through) the final submit — clicking it on a real production workspace would charge a real card / change real billing, which this capture must not do. Everything below "Submit — not captured" is inferred from the visible UI (button state, copy already on the page), not observed. The radio-selection step, page layout, and payment-method section **were** observed directly.

## bex's decided shape (read this first)

- **GraphQL only**, mirroring `renameWorkspace`'s exact shape: `changeWorkspacePlan(id: String!, plan: String!): Workspace!`, `can_manage`-scoped (`AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(id))`), delegating to a new `Service.ChangePlan(ctx, id, plan string) (WorkspaceView, error)` in `internal/workspaces/service.go`.
- **REST stays absent** — Render's REST `owners` surface has no plan-change endpoint (RESEARCH-workspaces.md finding 9: no POST/PATCH on `/owners` at all); bex's REST owners surface is deliberately read-only. Matching absence, not a gap.
- **MCP stays read-only** — Render's official MCP ships only `list_workspaces`/`select_workspace`/`get_selected_workspace`, no plan mutation; bex's workspace MCP tools gain nothing here.
- **No payment step** — Render inserts a "Payment Method" section (card-on-file, required for any paid plan) between plan selection and submit; bex ships no billing system yet ("Not in w6"), so the plan flips immediately on `changeWorkspacePlan`, exactly like `createWorkspace` already does.
- **Guard copy** follows the existing `core.ErrBadRequest`-wrapped, `%w: <specific reason>` convention `Create`'s per-user-cap check already uses (`internal/workspaces/service.go`), extended to three downgrade reasons:
  - member-count guard: `"%w: workspace has %d members, exceeds %s plan's limit of %d"`
  - service-count guard: `"%w: workspace has %d services, exceeds %s plan's limit of %d"`
  - per-user-cap guard (reuses `Create`'s exact message): `"%w: at most %d %s workspaces per user"`
  - role guard (t004): `"%w: workspace has members with roles not allowed on %s (%s); downgrade first"` naming the offending role(s)
  - A no-op (target == current plan) succeeds trivially — no guard, no write, matching Render's disabled-submit-until-changed UX (the submit button stayed disabled until a different plan was selected).

## Where the flow lives (observed)

The entry point is **not** the Settings page directly — Settings' General section shows a **Plan** row (plan name + tagline + an **Update Plan** link) that routes to a dedicated page: `/w/<tea-id>/billing/update-plan` (title: _"Change plan for &lt;workspace name&gt;"_). The same page is reachable from the **Billing** section and from a dismissible banner ("Better pricing for fast-growing teams" / "New plans, zero seat fees" — the mid-rollout 2026-04-23 pricing-transition banner, same one seen in `workspace-lifecycle.md`).

Page layout, top to bottom:

1. **Header** — `<h1>Change plan for &lt;workspace name&gt;</h1>` + the current plan name as a subtitle (e.g. "Professional (Legacy)").
2. **"Select a plan"** — a `radiogroup` of plan cards, one per plan, in ladder order: **Hobby (new)**, **Pro**, **Scale**, **Enterprise** (disabled — "Get in touch", links to `render.com/contact`). Each card shows: plan name (+ a **"New plan"** badge when it differs from the legacy name), one-line description, price (`$0/mo` / `$25/mo flat-fee` / `$499/mo flat-fee`, each "plus compute costs\*"), a bulleted feature list, and its own **Select plan** control — clicking anywhere on the card (not just a dedicated button) sets the radio. The currently-selected card's control reads "Plan selected" instead of "Select plan".
3. **"Payment Method"** — a warning banner ("This workspace plan requires a card for paid services and compute.") plus the card-on-file summary (brand + last 4) and a change-card control. Present regardless of which plan is selected (even Hobby, since compute/bandwidth overage still bills the card).
4. **Footer actions** — a primary submit button (disabled with no accessible label captured until a plan differing from the current one is selected, at which point it becomes enabled — exact label not read before backing out) and a **Cancel** link back to `/billing`.

No separate confirmation modal appears between selecting a plan and the (not-clicked) submit button — selection alone doesn't ask "are you sure"; whatever confirmation exists is presumably on/after submit (not captured).

## Downgrade guards (not captured live — inferred + design decision)

The live account had no workspace in a state that would trigger a downgrade guard (the only Hobby workspace available had 1 member, well under any cap), and submitting a real change was out of bounds for this capture. `RESEARCH-workspaces.md` finding 4 (Hobby: 1 member / 25 services) and finding 5 (roles plan-gated) give the caps; bex's own `store.LimitsFor` (whose `AllowedRoles` field holds the role catalog) is the enforcement point (see "bex's decided shape" above) — Render's exact downgrade-refusal copy is unverified and out of scope to chase further (no payment-free way to reach a violating downgrade on a real account).

## Parity implications for bex

| Render (captured live) | bex (decided) | Verdict |
| --- | --- | --- |
| Dedicated `/billing/update-plan` page, plan-card radiogroup | GraphQL mutation + a dialog on the existing settings page (t005) — no dedicated route | Deliberate simplification; bex has no billing app to house a full page for it. |
| Payment Method step gates any paid plan | No payment step | Deliberate deviation — "Not in w6" (billing out of scope), same as `createWorkspace`. |
| Submit disabled until a different plan is chosen | Mutation no-ops (still 200) on `plan == current` | Deliberate simplification — no client-only guard needed when the server is idempotent. |
| Guard/confirmation copy on submit | Unverified (not captured) | Documented gap — bex's guard copy is a fresh design (see above), not a clone; revisit if a future capture reaches it. |
| Enterprise plan requires "Get in touch" (disabled selection) | `store.NormalizePlan` accepts `enterprise` as a valid target (no contact-sales gate) | Deliberate simplification — bex has no sales flow; parity would mean disabling it, which contradicts "plan flips with no payment step." Flagged for `t006`'s parity sweep. |
