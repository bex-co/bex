# w5 · m12 — Key Value dashboard (create / list / detail, Render-consistent)

**Worker:** worker5 **Goal:** Give dashboard.bex.co the Key Value product surface Render has at dashboard.render.com: a sidebar entry, a list page, a create flow matching `/new/redis` (name + plan picker `free`/`starter`/`standard` + the captured options), and a detail page with status and connection-info reveal + copy (internal `redis://`, external when public, CLI command) — mirroring the m8 Databases page pattern, as the GraphQL client of w2/m7. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's KV dashboard live (`/new/redis`, list, detail) via Playwright | 30m | —          |
| t002 | GraphQL documents + hooks + `local-bex` stub fixtures for KV                   | 45m | t001       |
| t003 | Key Value list page + sidebar navigation entry                                 | 45m | t002       |
| t004 | Create flow at `/new/redis` parity (name, plan picker, captured options)       | 60m | t002       |
| t005 | Detail page — status, plan, connection-info reveal + copy                      | 60m | t003       |
| t006 | Render parity — UI vs captured Render pages; ledger UI cell                    | 30m | t004, t005 |
| t007 | Simplify — `/simplify` over the m12 diff                                       | 20m | t006       |
| t008 | Test coverage — pages, create validation, connection-info reveal               | 45m | t006       |
| t009 | Closeout                                                                       | 10m | t008       |

## Definition of done

On dashboard.bex.co (or the local stub): a "Key Value" sidebar entry lists KV stores with Render-consistent columns and empty state; "New Key Value" walks the captured `/new/redis` flow and creates a store that appears and reaches Available; its detail page shows status/plan and reveals the 3-part connection info with working copy buttons (password masked until revealed, external string only when public); i18n en+zh; `yarn lint && yarn typecheck && yarn test` green. Verified against the fixed multi-service stub (w5/m10) offline and against a real backend once w2/m7 is deployed.

## Source + Goal linkage

- **Source:** user parity report 2026-07-09 — "I don't see it on dashboard.bex.co… are you sure about feature parity with dashboard.render.com/new/redis?" The mechanism shipped (w1/m14, prod-verified) but no surface exists; the dashboard half pairs with w2/m7 (API surface, promoted from `w2/003`).
- **Goal linkage:** pillar 1 (Render parity) — the ledger's Key Value row UI cell is ✖; this flips it. The Databases page (w5/m8) proved the pattern; KV is its sibling resource type.
- **Expected outcome:** a tenant can create, monitor, and get connection credentials for a managed Valkey entirely from dashboard.bex.co, exactly as on Render.
- **Why now:** the user has flagged the gap explicitly; the mechanism is live in prod with nothing consuming it; w2/m7 defines the GraphQL contract this milestone binds to — sequencing directly after it (t002 onward can develop against the stub in parallel once t001+w2/m7-t001 fix the shapes).
- **Render parity task included:** t006 — this is user-facing surface work by definition.
