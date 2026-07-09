# w8 · m3 — Dashboard Usage page (workspace-scoped, Render-consistent)

**Worker:** worker8 **Goal:** the human half: a workspace-scoped Usage page in the dashboard showing month-to-date compute hours by service and tier, bandwidth, and build minutes — layout referenced live from the usage section of Render's billing page — as a GraphQL client of m2. Build _seconds_ are stored (m1) but displayed as **minutes**, the vocabulary Render users know ("pipeline minutes"). **Status:** todo (gated on w8/m2)

## Tasks (in order)

| id   | title                                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's billing/usage page live (Playwright) as the layout reference                           | 30m | —          |
| t002 | Apollo wiring for the m2 `usage` query + dev-stub data (coordinate with w5/m10's trustworthy stub)      | 30m | t001       |
| t003 | Usage page UI: month-to-date compute by service/tier, bandwidth, build minutes                          | 45m | t002       |
| t004 | Navigation: sidebar/workspace-settings entry, consistent with w6/m3's workspace IA                      | 30m | t003       |
| t005 | Render parity: UI matches the captured Render layout/semantics; drift flagged, not silent               | 30m | t004       |
| t006 | Simplify: `/simplify` over the code this milestone changed                                              | 30m | t005       |
| t007 | Test coverage: meaningful UI/query tests (loading, empty month, unit display)                           | 45m | t005       |
| t008 | Closeout: verify DoD, mark done, move milestone to `done/`                                              | 15m | t007       |

## Definition of done

A signed-in user opens the Usage page on dashboard.bex.co and sees their workspace's real month-to-date quantities — matching the m2 API's numbers exactly — live against the prod cluster; build time renders in minutes; the page layout is recognizably Render's usage view.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w8` 2026-07-09 (same provenance as m1/m2); Render's billing-page usage section captured live in t001.
- **Goal linkage:** V0 roadmap item 5; Render-parity dashboard surface (Render shows month-to-date usage on its Billing page).
- **Expected outcome:** the dashboard answers "what did this workspace consume this month?" without kubectl, curl, or Prometheus knowledge.
- **Why now:** completes the metering slice while m1/m2 context is fresh; the workspace IA it hangs off (w6/m3 switcher/settings) is being built in parallel — coordinating now avoids a later nav retrofit.
- **Render parity: included** (t005) — this is a user-facing UI surface; the reference is Render's own billing page, usage section only (the payment/invoice section is out of scope — see the pricing entry in `.pm/FUTURE-MAYBE.md`).
