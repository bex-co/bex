# w5 · m12 — Key Value dashboard (create / list / detail, Render-consistent)

**Worker:** worker5 **Goal:** Give dashboard.bex.co the Key Value product surface Render has at dashboard.render.com: a sidebar entry, a list page, a create flow matching `/new/redis` (name + plan picker `free`/`starter`/`standard` + the captured options), and a detail page with status and connection-info reveal + copy (internal `redis://`, external when public, CLI command) — mirroring the m8 Databases page pattern, as the GraphQL client of w2/m7. **Status:** done (2026-07-09)

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's KV dashboard live (`/new/redis`, list, detail) via Playwright — **DONE** (live authenticated session; `docs/render-artifacts/key-value.md`) | 30m | —          |
| t002 | GraphQL documents + hooks + `local-bex` stub fixtures for KV — **DONE** (hand-mirrored codegen, `graphql`-parse round-trip verified byte-identical against existing generated documents; `yarn lint && yarn typecheck` green) | 45m | t001       |
| t003 | Key Value list page + sidebar navigation entry — **DONE** (`/keyvalue` route, sidebar entry after Databases, stat cards + table; verified live against the stub) | 45m | t002       |
| t004 | Create flow at `/new/redis` parity (name, plan picker, captured options) — **DONE** (full-page form per capture, plan-tier cards sourced from `keyValueInstanceTypes`; create→creating→available verified live) | 60m | t002       |
| t005 | Detail page — status, plan, connection-info reveal + copy — **DONE** (connect panel gated behind Reveal, suspend/resume with confirm-gated suspend, delete typed-confirm; verified live) | 60m | t003       |
| t006 | Render parity — UI vs captured Render pages; ledger UI cell — **DONE** (`docs/render-parity.md` Key Value row UI ✖→✅; gaps — maxmemoryPolicy/persistenceMode, IP allowlist, metrics tab — listed in-row, not faked) | 30m | t004, t005 |
| t007 | Simplify — `/simplify` over the m12 diff — **DONE** (4-agent review; applied: shared `isValidDnsLabel`, `MetadataList`, `ConnectionField` extracted to `common/components/` and adopted by both databases *and* keyvalue; simplified `use-key-value-lifecycle`'s dispatch. Declined: `keyvalue`→`key-value` dir rename (mandated by the milestone's own file spec) and plan-picker→Select swap (t004 explicitly asked for tier cards matching the live capture, not databases' dialog-era Select). Re-verified live on stub after refactor; `yarn lint && yarn typecheck && yarn test` green, 467/467) | 20m | t006       |
| t008 | Test coverage — pages, create validation, connection-info reveal — **DONE** (29 new tests: status lib, list/create/detail hooks, connection-info-panel security assertions incl. public/internal-only variants, row-actions typed-confirm, all 3 routes; 496/496 green) | 45m | t006       |
| t009 | Closeout — **DONE** (full create→list→detail→connection-info→delete journey re-verified live on the stub, incl. standard/public plan + suspend/resume + copy; statuses flipped, milestone moved to `done/`) | 10m | t008       |

## Definition of done

On dashboard.bex.co (or the local stub): a "Key Value" sidebar entry lists KV stores with Render-consistent columns and empty state; "New Key Value" walks the captured `/new/redis` flow and creates a store that appears and reaches Available; its detail page shows status/plan and reveals the 3-part connection info with working copy buttons (password masked until revealed, external string only when public); i18n en+zh; `yarn lint && yarn typecheck && yarn test` green. **Verified against the fixed multi-service stub (w5/m10) offline, live via Playwright, 2026-07-09** — create (standard plan + public) → creating→Available convergence → connection-info reveal (internal + external + CLI) → copy → suspend/resume → typed-confirm delete, all working; 496/496 tests green. Prod verification against a real backend is deferred until w2/m7's own live-deploy closeout (`w2/done/m7/t010`, currently deploy-gated) lands — this milestone's dashboard code is complete and stub-verified independent of that.

## Source + Goal linkage

- **Source:** user parity report 2026-07-09 — "I don't see it on dashboard.bex.co… are you sure about feature parity with dashboard.render.com/new/redis?" The mechanism shipped (w1/m14, prod-verified) but no surface exists; the dashboard half pairs with w2/m7 (API surface, promoted from `w2/003`).
- **Goal linkage:** pillar 1 (Render parity) — the ledger's Key Value row UI cell is ✖; this flips it. The Databases page (w5/m8) proved the pattern; KV is its sibling resource type.
- **Expected outcome:** a tenant can create, monitor, and get connection credentials for a managed Valkey entirely from dashboard.bex.co, exactly as on Render.
- **Why now:** the user has flagged the gap explicitly; the mechanism is live in prod with nothing consuming it; w2/m7 defines the GraphQL contract this milestone binds to — sequencing directly after it (t002 onward can develop against the stub in parallel once t001+w2/m7-t001 fix the shapes).
- **Render parity task included:** t006 — this is user-facing surface work by definition.
