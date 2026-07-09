# w1 · m11.5 — Custom-domains dashboard section (UI half of m11)

**Worker:** worker1 **Goal:** Add Render-parity custom-domains management to the service Settings tab in the bex dashboard, wired to the `customDomains`/`addCustomDomain`/`deleteCustomDomain` GraphQL surface shipped in m11. **Status:** done

## Tasks (in order)

| id   | title                                                                | est | depends_on          | status     |
| ---- | -------------------------------------------------------------------- | --- | ------------------- | ---------- |
| t001 | GraphQL codegen for custom-domain queries and mutations              | 20m | —                   | — **DONE** |
| t002 | Custom Domains table in Settings (list, status badges, empty state)  | 40m | t001                | — **DONE** |
| t003 | Add Custom Domain dialog (FQDN input → addCustomDomain mutation)     | 30m | t002                | — **DONE** |
| t004 | Delete (per-row menu) + platform subdomain display section           | 30m | t003                | — **DONE** |
| t005 | Render parity — verify table, badges, add, delete match render.com   | 20m | t004                | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed          | 20m | t005                | — **DONE** |
| t007 | Test coverage — meaningful tests for the custom-domains UI           | 30m | t005                | — **DONE** |
| t008 | Closeout                                                             | 10m | t006, t007          | — **DONE** |

## Definition of done

Navigating to a service's Settings tab shows the Custom Domains section: existing domains listed with Name (external link), Verified Status badge, and Certificate Status badge; "Add Custom Domain" button opens a dialog, submits to bex-api, and the new domain appears in the table; per-row kebab menu offers "Delete" with a confirmation dialog that removes the domain; a "Platform Subdomain" read-only section below the table shows the `.onbex.co` URL as always-active. All three status paths (pending/verified/active) are visually distinguishable. The section matches render.com's Settings → Custom Domains layout (live-captured 2026-07-09).

## Source + Goal linkage

- **Source:** promoted from `w5/003.md` (dashboard inbox note, 2026-07-08); unblocked when `w1/m11` shipped the backend API (2026-07-09). Live Render capture performed 2026-07-09: table with Name link + "Verified Status" + "Certificate Status" columns, per-row kebab Menu, "Add Custom Domain" button, "Render Subdomain" toggle below the table.
- **Goal linkage:** Render parity (pillar-1 API-first: API shipped in m11, UI follows immediately); dashboard completeness — Settings tab currently shows only the plan picker.
- **Expected outcome:** a user can manage custom domains for a service entirely from the dashboard, on par with render.com.
- **Why now:** m11 unblocked this (the `customDomains`/`addCustomDomain`/`deleteCustomDomain` GraphQL surface exists and is tested); the dashboard Settings tab is visibly incomplete without it. Render parity task (t005) is included because this is a user-facing surface change.

## Outcome (2026-07-09)

Shipped. The service Settings tab now renders the Custom Domains section (table with Name external-link + Verified Status + Certificate Status badges, "Add Custom Domain" dialog, per-row kebab delete with confirmation) plus a read-only Platform Subdomain section. Wired to the m11 GraphQL surface. Verified end-to-end against the `dev:local` stub with Playwright (list → add → delete round-trip, status badges, platform URL). Green: `tsc -b`, `eslint`, 417 dashboard tests (9 new for these components).

Intentional Render deviations (parity notes, not gaps):

- **Certificate Status shows "Active"/"Pending", not Render's "Unknown".** bex derives it from real TLS/serving state (`serverStatus` = cert issued and not suspended), which is more honest than Render's capture showed. Documented in `CustomDomainView`'s type comment.
- **Platform subdomain is a read-only "Always enabled" section, not Render's toggle.** bex always keeps the `.onbex.co` hostname reachable (no opt-out), so there is no switch to render.
- **No post-add DNS/CNAME instructions panel** (Render shows the CNAME target after adding). Deferred — the mechanism half (per-host cert issuance) already works; the instructions UI is a follow-up recorded as `w5/006.md`.

`/simplify` (t006) applied: removed dead `domainType` plumbing, dropped a redundant `saving` state (the hook's `busy` already gates it), simplified the dialog's `onOpenChange`, added a shared `success` Badge variant, and extracted a shared `CenteredState` (dedup with the env-vars panel). Skipped as low-value/out-of-scope: memoizing `mapDomains` (matches the sibling convention), a shared `TableSkeleton`/`ExternalLink`/mutation-with-toast helper (broader refactors touching working code).
