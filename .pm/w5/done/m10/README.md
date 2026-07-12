# w5 · m10 — Trustworthy dev stub (multi-service) + custom-domain DNS instructions end to end

**Worker:** worker5 **Goal:** Kill the dev stub's phantom-service bug (any `/services/<id>` echoes the one hardcoded service — `nightly-report` showed `eden-cms-v2`'s data, including its custom domains) and close the last custom-domains gap: after adding a domain, the user sees exactly which DNS record to create (type + host + target, copyable), sourced from the backend across REST/GraphQL/MCP and rendered in the dashboard — Render's post-add flow, end to end. **Status:** DONE (2026-07-09)

## Tasks (in order)

| id   | title                                                                                | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | Fix `local-bex` stub: multi-service store, per-id resolvers, unknown id → null       | 45m | — — **DONE**     |
| t002 | Dashboard not-found state for unknown service id (no phantom data)                   | 30m | t001 — **DONE**  |
| t003 | Capture Render's live add-domain DNS-instructions flow (m7/m11.5 pattern)            | 30m | — — **DONE**     |
| t004 | Backend: per-domain DNS record target + verify verb (REST/GraphQL/MCP)               | 60m | t003 — **DONE**  |
| t005 | Dashboard: DNS instructions panel (add flow + per-row) with copy + status re-check   | 60m | t002, t004 — **DONE** |
| t006 | Render parity — REST/GraphQL/MCP/UI consistency for the domain surface               | 30m | t005 — **DONE**  |
| t007 | Simplify — `/simplify` over the m10 diff                                             | 20m | t006 — **DONE**  |
| t008 | Test coverage — stub resolvers, not-found route, DNS-instruction fields + panel      | 30m | t006 — **DONE**  |
| t009 | Closeout                                                                             | 10m | t008 — **DONE**  |

## Definition of done

With the dev stub running: `/services/<unknown-id>` renders a not-found state (no borrowed data), and two stub services render distinct overviews, custom domains, and logs. Adding a custom domain (real backend or stub) immediately shows the DNS record to create — record type per `domainType` (CNAME → platform host for subdomains; apex guidance), host, target, copy-to-clipboard — and the row's verification/serving status can be re-checked; the DNS-target data is served identically by REST, GraphQL, and MCP (same fields/semantics, verified against Render's captured flow). `make test` (backend), `yarn lint && yarn typecheck && yarn test` (dashboard) green.

## Source + Goal linkage

- **Source:** user report 2026-07-09 (`/services/nightly-report` showed `www.eden-cms.com` data — traced to `dashboard/scripts/local-bex.mjs:194` echoing the single hardcoded `SERVICE` for any id) + inbox note `w5/006` (post-add DNS/CNAME instructions, split out of `w1/m11.5`), surfaced as the ◐ "Verify / DNS instructions" row in `docs/ADR018-render-parity.md`.
- **Goal linkage:** pillar 1 (Render parity) — Render's add-domain flow shows the exact DNS record and re-checks until the cert issues; bex adds domains blind. The stub fix protects the verification workflow itself: a stub that fabricates data for any id makes Playwright/UI verification untrustworthy (it masked this exact class of bug on 2026-07-09).
- **Expected outcome:** a tenant adding `www.example.com` sees "create a CNAME from `www.example.com` to `<app>.onbex.co`" with copy buttons and live verification status; a developer hitting an unknown service id sees a 404 state, not phantom data.
- **Why now:** the phantom-service bug actively misleads anyone using the stub (it just misled a real verification session); the DNS-instructions gap is the one remaining ◐ on the custom-domains ledger row and blocks calling that surface Render-complete. The two ship together because the domain UI work is verified against the stub — the stub must be trustworthy first.
- **Render parity task included:** t004/t005 touch REST, GraphQL, MCP, and the dashboard — the standing parity check (t006) verifies the four surfaces agree and match Render's captured behavior.
