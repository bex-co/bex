# w7 · m6 — Custom domain ownership verification + cross-App collision guard

**Worker:** worker7 **Goal:** Adding a custom domain no longer routes or requests a TLS cert until the tenant proves DNS ownership, and no App (any tenant) can claim a hostname already verified elsewhere or a reserved platform host — closing a live cross-tenant/cross-platform hostname-hijack path on the shipped custom-domain flow. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Add domain verification state to the control-plane store (pending/verifying/verified + challenge token)    | 45m | —            |
| t002 | Implement the DNS ownership check in the verify verb; mark verified only when the DNS proof passes          | 45m | t001         |
| t003 | Gate routing on verified state — patch `spec.hosts[]` (⇒ Ingress + cert) only after verification            | 45m | t002         |
| t004 | Cross-App + reserved-host collision guard — reject already-verified hosts and platform/base-domain hosts     | 45m | t002         |
| t005 | Surface verification status + DNS challenge record across REST · GraphQL · MCP + dashboard                   | 45m | t003, t004   |
| t006 | Render parity — verification state + challenge record shape consistent across REST · GraphQL · MCP + dashboard, vs Render | 30m | t005 |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                  | 20m | t006         |
| t008 | Test coverage — meaningful tests for verification gating, DNS proof, and collision/reserved-host rejection    | 30m | t006         |
| t009 | Closeout — DoD verified, milestone moved to `done/`                                                          | 15m | t008         |

## Definition of done

Adding a custom domain returns a challenge record and a `pending` state; the host is **not** routed and **no** cert-manager cert is requested until the verify verb confirms the DNS proof; a second App (any tenant) cannot verify a host already verified elsewhere; platform / base-domain hosts (`BEX_BASE_DOMAIN`, the dashboard host) are rejected; verification state + challenge record round-trip on REST, GraphQL, and MCP create/verify/read and render in the dashboard DNS-instructions panel. Parity with Render's custom-domain verification is checked across the three adapters + UI.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-12). Verified 2026-07-12: `lego/backend/internal/apps/domains.go:207-235` (`AddDomain`) appends any hostname to `App.spec.hosts[]` with **zero** cross-App uniqueness check and **no** reserved/platform-host guard (only a same-App duplicate check); `VerifyDomain` (domains.go:200-202) is an idempotent `GetDomain` re-read, not an ownership proof. `docs/ADR005-custom-domain.md:50` explicitly defers the verify-before-route state machine (`pending → verifying → active, owned-by-tenant`) as control-plane work.
- **Goal linkage:** GOAL.md V0 #7 (security review) + pillar 1 (Render-compatible custom-domain flow); extends the tenant-isolation axis of the w7 track to the ingress/routing layer.
- **Expected outcome:** cross-tenant and cross-platform hostname hijacking on the shipped custom-domain path is closed; the parity ledger's Verify/DNS row (`docs/ADR018-render-parity.md:59`), which currently claims ✅ but only does an idempotent status re-read, becomes truthful (routing actually gated on ownership).
- **Why now:** it is a **live hijack** on already-shipped, prod-running code — any authenticated tenant can claim `dashboard.bex.co`, a `*.onbex.co` platform host, or another tenant's active custom domain, and bex builds the Ingress + races an ACME challenge; cheapest to close before real multi-tenant custom domains exist to migrate.
- **Render parity: included** — t005 changes the custom-domain verify flow on REST/GraphQL/MCP + the dashboard DNS-instructions panel, so t006 checks the `pending/verifying/verified` state + challenge-record shape/semantics across the three adapters and against Render's custom-domain verification. Pure control-plane state + operator routing-gate are the mechanism; the surfaced state is the parity-bearing part.
