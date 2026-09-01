# w4 · m90 — Workspace-scoped billing email + payment method at creation

**Worker:** worker4 **Goal:** `/new/workspace` collects a required, account-prefilled billing email and a payment method for the workspace being created, so no new workspace inherits or merely checks another workspace's billing state **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persist billing email and resumable workspace-create attempts — **DONE** | 50m | — |
| t002 | Prepare and verify a pre-workspace Stripe SetupIntent — **DONE** | 1h | t001 |
| t003 | Finalize workspace creation exactly once over GraphQL — **DONE** | 1h | t001, t002 |
| t004 | Build the billing-email + Payment Element create form — **DONE** | 1h | t003 |
| t005 | Finish return, retry, cleanup, skeleton, and local-dev paths — **DONE** | 45m | t002, t003, t004 |
| t006 | Render parity — **DONE** | 35m | t001, t002, t003, t004, t005 |
| t007 | Simplify — **DONE** | 25m | t006 |
| t008 | Test coverage — **DONE** | 50m | t006 |
| t009 | Closeout — **DONE** | 15m | t007, t008 |

## Researched current state (2026-08-31)

Authenticated Playwright walks of both production routes established the exact gap:

- **Render:** `Workspace Details` contains `Name` and required `Billing Email`; the email is prefilled from the signed-in account. Hobby resets it to the account email and disables editing with “For hobby workspaces, billing email cannot be changed”; Pro and Scale leave it editable and validate it. `Payment Method` is always present and explicitly says billing is unique to each workspace. `Add Card` opens Stripe-hosted Elements for full name, country/address, and card details. Render's shipped form requires a card for paid plans and keeps it optional for Hobby (apart from an internal Render-account bypass), then submits the new workspace's card token/address with the create mutation. Official Render docs independently say workspace creation includes plan type and payment method.
- **bex:** Hobby shows only slug + plan cards. The payment panel appears only after selecting a paid plan and says a card on the **current workspace** is required. `createBlockedByPayment` and `useBillingOnboarding` read that current workspace; GraphQL remains `createWorkspace(name, plan)`. The backend likewise calls `RequirePlanBilling` on the caller's current workspace, inserts the new tenant, and returns it without a `billing_provider_mappings` row or payment marker. `StripeClient.EnsureCustomer` currently creates a Customer named only by `tea-*` and carries no billing email.
- **Prior-work gap:** `w9/m88` intentionally shipped only the Render-shaped cards/panel and its DoD expressly retained current-workspace readiness and omitted real collection. This milestone replaces that shortcut rather than duplicating finished work.

Stripe's recommended custom-flow primitive fits the required ordering: after the authorized attempt is prepared, the server creates the dedicated Customer and SetupIntent, and the browser mounts Payment Element with the returned client secret and calls `confirmSetup`. Card/PAN data stays inside Stripe.js/Elements. The server re-reads the successful SetupIntent and attached PaymentMethod before binding defaults and creating the authoritative workspace; a browser redirect or client assertion alone is not proof.

## Completion evidence (2026-08-31)

- Authenticated desktop and 390px-mobile comparisons were captured for Render and bex in `.playwright-mcp/`; `docs/render-artifacts/new-workspace.md` records the observed parity, intentional policy differences, and GraphQL-only mutation boundary.
- A real Stripe test-mode browser run created a dedicated Customer with `billing+success@example.com`, confirmed a Payment Element SetupIntent with Stripe's `4242` test card, retried safely after a deliberately missing catalog price caused a fail-closed provider error, and finalized one Pro workspace. Stripe reported an active, test-mode subscription with all 15 metered contract items and the reserved workspace/attempt metadata.
- The real Postgres state showed one finalized attempt, one `tea-*` tenant with the submitted billing email, one admin membership, one billing-provider mapping, and a non-null `payment_method_bound_at`; the canceled attempt remained `cleanup_pending` and created no tenant. Provider ids were inspected only in trusted CLI/SQL evidence, while no client secret or payment value was printed or persisted.
- The cancel/start-over path was exercised before the successful retry. All temporary Customers, SetupIntents, the successful test subscription, local processes/database, and temporary local-cluster CRDs were cleaned up afterward. The two missing required test-catalog meters/prices (`disk_gb_hours`, `sandbox_compute_seconds`) were provisioned and retained as billing configuration.
- Green gates: `make test` and `make lint` from `lego/operator`; `go test ./...` from `lego/backend`; the dedicated real-Postgres `TestWorkspaceCreationPG`; `go test ./...` from `lego/cli`; `yarn lint` and full `yarn test` from `dashboard`; seven Stripe installer tests; and `scripts/skill-layout-validate.sh`.
- `/simplify` ran as the required reuse, quality, and efficiency review. It consolidated authorization, validation, Stripe key-mode checks, search/email helpers, and derived form state; bounded cleanup was tightened to batches of 10 with 15-minute retry leases and 30-day terminal retention.

## Definition of done

- `/new/workspace` always shows a required **Billing Email**, prefilled from the Kratos session email. Hobby keeps Render's read-only/account-email behavior; paid plans allow a different valid billing email. Validation exists server-side as well as in the form.
- The Payment Method region describes billing as unique to the new workspace and never reads, copies, or gates on the current workspace's card. Payment details are collected by Stripe Payment Element/Stripe.js; bex never receives raw PAN/CVC and no Stripe secret reaches the browser.
- The server, not a plan-only client heuristic, reports the create policy: production `BEX_REQUIRE_PAYMENT_METHOD=all` requires a successfully bound method for Hobby/Pro/Scale; paid-intent mode requires it for Pro/Scale while Hobby remains optional like Render; billing-off self-hosted installs retain a usable create flow.
- A successful setup creates exactly one `tea-*`, owner-admin membership/OpenFGA grant, Stripe Customer/contract, billing-provider mapping, `payment_method_bound_at` marker, and stored billing email for that same workspace. The Customer email matches the submitted billing email.
- A failed, cancelled, expired, duplicated, or retried setup creates no visible/usable orphan workspace and cannot reuse another subject's attempt. Retrying the finalization is idempotent; abandoned Stripe/local attempt state has a bounded cleanup path.
- The new workspace is selected only after server-verified setup/finalization. Immediate card setups continue without a false success gap; redirect-based/SCA flows resume safely after return and survive refresh/back navigation.
- Existing `createWorkspace` callers have an explicit compatibility outcome: either the mutation keeps a documented billing-off path or receives a stable coded payment-required error directing interactive callers to the create flow. Workspace mutation stays GraphQL-only because Render exposes no REST/MCP workspace create; this absence is documented and tested.
- The pending skeleton matches the final desktop and narrow-mobile form, including Workspace Details, plan cards, billing email, Payment Method, and footer geometry.
- Real Stripe test-mode browser evidence covers at least one required-card success and one cancel/failure/retry; backend, real-Postgres, dashboard typecheck/lint/tests, and relevant script/config checks are green.

## Source + Goal linkage

- **Source:** user request 2026-08-31: “`https://dashboard.bex.co/new/workspace` should require user to input billing email (auto-filled) and payment method, just like `https://dashboard.render.com/new/workspace`”; authenticated Playwright comparison and public Render create-workspace docs performed in the handoff session. Extends `w9/m88` and the Stripe/payment-gate lineage (`w7/m51`, `w1/m62`, ADR046, ADR075).
- **Goal linkage:** ADR008's open-source Render-alternative goal and ADR018 workspace-lifecycle parity; ADR046/ADR075 require a workspace-local payment marker before usage, which the present create flow cannot establish for the workspace it returns.
- **Expected outcome:** a user leaves `/new/workspace` with one independently billable workspace whose billing email and card belong to it, rather than discovering a second payment wall or having eligibility inferred from an unrelated workspace.
- **Why now:** production runs `BEX_REQUIRE_PAYMENT_METHOD=all`, so creating an unbound workspace is immediately inconsistent with the next resource-create call and with the sign-up wall. The current UI also claims workspace-specific billing while operating on the old workspace. Closing the mismatch before external paid workspaces prevents ambiguous Customer ownership and cleanup migrations later.
- **Render parity — included:** this is a user-facing GraphQL/dashboard/Stripe change. The parity pass must preserve Render's billing-email behavior and new-workspace-specific card semantics, while explicitly recording bex's intentional stricter Hobby card requirement under `all` mode and its 30%-off catalog prices. REST/MCP workspace mutation remain absent to match Render; any new provider-neutral state/error exposed elsewhere must keep the usual REST/GraphQL/MCP semantics aligned.
