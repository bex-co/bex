# Create workspace (`/new/workspace`) — Render vs bex

Pinned **2026-08-31** for `w4/m90`. Complements [workspace-lifecycle.md](workspace-lifecycle.md) (switcher/settings) and [workspace-plan-change.md](workspace-plan-change.md) (plan-card anatomy).

Authenticated desktop and narrow-mobile captures were taken from both products:

- Render: `.playwright-mcp/render-new-workspace-auth.png` and `.playwright-mcp/render-new-workspace-auth-mobile.png`
- bex local parity build: `.playwright-mcp/bex-new-workspace-auth.png` and `.playwright-mcp/bex-new-workspace-auth-mobile.png`

The captures are local QA artifacts and intentionally remain outside source control.

## Render reference behavior

- **Workspace Details** contains required `Name` and `Billing Email` fields. Billing email is prefilled from the signed-in account.
- Hobby resets billing email to the account email and disables editing. Pro and Scale allow a different valid email.
- **Payment Method** is present for every plan and says billing is unique to each workspace.
- `Add Card` opens Stripe-hosted fields. The card is optional for Hobby and required for paid plans; the create action cannot complete a required-card plan without it.
- Workspace creation is dashboard-internal. Render's public REST API has read-only owner/workspace endpoints and its MCP surface has no workspace-create mutation.

## bex behavior after m90

- **Workspace Details** contains required `Workspace slug` and `Billing Email`. The latter comes from the authenticated Kratos identity. Hobby is read-only/account-email; paid plans accept a different valid address. The server repeats both email validation and Hobby ownership validation.
- The four plan cards retain the ADR030 catalog: **$0 / $17.50 / $349.30 / Custom terms**. These are intentionally 30% below Render's $0 / $25 / $499 / contact pricing.
- **Payment Method** is always present and belongs only to the reserved workspace. It never reads, copies, or gates on the current workspace's Customer or readiness marker.
- A server policy controls collection: `all` requires a verified payment method for every plan; `paid` requires one for Pro, Scale, and Enterprise while keeping Hobby optional; `off` leaves self-hosted creation usable without Stripe.
- When collection is requested, the server reserves an opaque attempt and `tea-*`, creates that workspace's Stripe Customer using the submitted email, and creates a SetupIntent. The browser receives only Stripe's publishable key and client secret, and renders the Payment Element. Raw PAN/CVC never enters bex state.
- Finalization re-reads the succeeded SetupIntent and Customer-bound PaymentMethod, binds the Customer/Subscription defaults, then atomically creates the tenant, owner membership, billing mapping, payment marker, and stored billing email. The workspace is selected only after that transaction returns.
- The attempt is subject-bound, expiring, resumable after redirect/refresh, and idempotent on finalization. Cancelled or abandoned attempts stay outside `tenants` and are reclaimed by a leased cleanup worker, so they are never visible as usable workspaces.
- The legacy `createWorkspace(name, plan)` GraphQL mutation remains the explicit billing-off compatibility path. When policy requires collection it returns stable `PAYMENT_REQUIRED`; clients must use `prepareWorkspaceCreation` followed by `finalizeWorkspaceCreation`. REST and MCP workspace mutations remain absent to match Render's public surface.

## Comparison

| Topic | Render | bex | Verdict |
| --- | --- | --- | --- |
| Required, account-prefilled billing email | Hobby read-only; paid editable | Same, with server validation | Match |
| Payment Method region | Always visible; workspace-specific | Always visible; reserved-workspace-specific | Match |
| Card policy | Optional Hobby, required paid | `paid` matches; production `all` also requires Hobby | Intentional stricter production policy |
| Provider collection | Stripe-hosted card fields | Stripe Payment Element + SetupIntent | Match in security boundary |
| Failure/retry | Form remains recoverable | Opaque resumable attempt; idempotent finalize; bounded cleanup | Match with explicit server guarantees |
| Plan prices | `$0` / `$25` / `$499` / contact | `$0` / `$17.50` / `$349.30` / Custom terms | Deliberate ADR030 difference |
| Name | Freeform | DNS-label slug | Deliberate App-CR naming constraint |
| Enterprise | Contact sales | Self-serve selectable | Known deliberate divergence (`.pm/w9/050.md`) |
| Licensed monthly workspace SKU | Render plan charge | No licensed monthly SKU; usage-meter contract only | Deliberate current billing model |
| REST / MCP create | None | None | Match |

Do not change bex's catalog to Render's price points as a parity fix, and do not reintroduce the pre-m90 current-workspace billing shortcut.
