# w9 · m88 — Authenticated `/new/workspace` Render-shape + catalog prices

**Worker:** worker9 **Goal:** the signed-in `/new/workspace` page matches Render's create-workspace anatomy (page `h1`, slug field, large plan cards, payment-method panel for paid plans, Create/Cancel) while showing bex's catalog fees from `pricing.yaml` (Hobby $0 / Pro $17.50 / Scale $349.30 / Enterprise custom — 30% off Render). **Status:** done

## Tasks (in order)

| id   | title                                                                 | est | depends_on                                      |
| ---- | --------------------------------------------------------------------- | --- | ----------------------------------------------- |
| t001 | Un-card the page: `h1`, slug field, `FormPageSkeleton`                | 45m | —                                               — **DONE** |
| t002 | Large plan cards: LimitsFor bullets + catalog fees                    | 45m | [w9/m88/t001]                                   — **DONE** |
| t003 | Payment-method panel when Pro/Scale is selected                       | 40m | [w9/m88/t002]                                   — **DONE** |
| t004 | Switcher: `+ New Workspace`, plan sublabel, Billing                   | 30m | —                                               — **DONE** |
| t005 | Authenticated Playwright walk vs Render; pin the artifact             | 35m | [w9/m88/t001, w9/m88/t002, w9/m88/t003, w9/m88/t004] — **DONE** |
| t006 | Render parity                                                         | 25m | [w9/m88/t001, w9/m88/t002, w9/m88/t003, w9/m88/t004, w9/m88/t005] — **DONE** |
| t007 | Simplify                                                              | 20m | [w9/m88/t006]                                   — **DONE** |
| t008 | Test coverage                                                         | 40m | [w9/m88/t006]                                   — **DONE** |
| t009 | Closeout                                                              | 15m | [w9/m88/t008]                                   — **DONE** |

## Definition of done

A signed-in user on `https://dashboard.bex.co/new/workspace` (or equivalent local SSR) sees:

- a page-level **Create a workspace** heading (not a nested settings card), a **workspace slug** field with DNS-label helper text (constraint unchanged), Hobby preselected;
- four large plan cards whose price lines are **`$0/mo` / `$17.50/mo` / `$349.30/mo` / Custom terms** (lockstep with `lego/backend/internal/pricing/pricing.yaml`; never Render's $25 / $499) plus capability bullets from `store.LimitsFor`;
- a usage footnote (resource tiers billed separately);
- when Pro or Scale is selected and `BEX_REQUIRE_PAYMENT_METHOD=1`, a Payment Method panel that reuses `BillingOnboardingView` / current-workspace readiness and disables Create until a card can be bound — Hobby unchanged; **no** licensed Stripe Price is attached on create (catalog display only);
- switcher copy **+ New Workspace**, each row showing its plan, and a Billing item to `/billing`;
- Create still selects the returned `tea-*` and lands on `/`; failure stays on the form with an inline error;
- `FormPageSkeleton` is the route `pendingComponent`;
- an authenticated side-by-side capture is pinned under `docs/render-artifacts/` (or `.playwright-mcp/` referenced from that file).

`yarn typecheck && yarn lint && yarn test` green for the dashboard files this milestone touches.

## Source + Goal linkage

- **Source:** user request 2026-08-19 after a Playwright comparison of `https://dashboard.render.com/new/workspace` vs `https://dashboard.bex.co/new/workspace` (both unauthenticated hits bounced to login with `next=/new/workspace`). The authenticated-page gap list and the catalog fees (ADR030, 30% off Render workspace subscriptions) were decided in that session. Extends shipped `w6/m3` (create form exists) rather than duplicating it.
- **Goal linkage:** Render-parity dashboard (ADR008 / ADR018 workspace lifecycle UI). The create page is the first place a human picks a workspace plan; showing the catalog price is what makes ADR030's 30%-off policy visible.
- **Expected outcome:** creating a workspace feels like Render's flow (shape + payment-method *panel*), while the numbers on the cards are bex's ($17.50 / $349.30), not Render's ($25 / $499).
- **Why now:** the catalog rates just landed in `pricing.yaml` and the PlanPicker locales still sit on a cramped card that says little besides the fee; without this milestone the new prices stay a one-line label on the old layout. Sequence: prices first (done), then this UX so the numbers have a Render-shaped home.
- **Render parity — included:** the milestone *is* the UI/create-flow comparison against Render. REST/MCP stay read-only for workspace mutations (deliberate, ADR018). GraphQL `createWorkspace` is unchanged except any payment-gate interaction already owned by ADR046. Flag remaining drift (freeform names, licensed Stripe SKU collection, Scale orgs/SSO, Enterprise contact-sales disable) as follow-up, do not silently invent them.
