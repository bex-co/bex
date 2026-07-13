# w8 — Usage metering (worker8)

**Worker:** worker8 Created 2026-07-09 from `/pm-brainstorm w8` — owns `GOAL.md` #5's unowned half ("usage metering"; the multi-tenant half is w1/m9 + w6 + w4/m12). Meters **quantities** — instance-seconds by tier, egress bytes, build minutes: exactly Render's three meters (verified live 2026-07-09 vs render.com/pricing + docs). Payments/payment-collection stay out per w6's "no billing system" boundary; **m7** (2026-07-13) adds dollar *estimates* over these quantities (a price sheet, not a billing system — `.pm/FUTURE-MAYBE.md`'s "Pricing & spend estimation" trigger). Numbered **w8, not w7** — w7 was consumed as the old staging path for w1/m15; never reuse a number. Ordered by dependency: pipeline → API surface → dashboard.

## Milestones

- [x] **m1** — Metering pipeline: hourly usage rollups into the control-plane store (9 tasks) ← from `/pm-brainstorm w8` 2026-07-09
- [x] **m2** — Usage API: month-to-date usage over REST · GraphQL · MCP (9 tasks) ← from `/pm-brainstorm w8` 2026-07-09, needs m1
- [x] **m3** — Dashboard Usage page (workspace-scoped, Render-consistent) (8 tasks) ← from `/pm-brainstorm w8` 2026-07-09, needs m2
- [x] **m4** — Usage data retention: compact hourly detail into monthly aggregates (9 tasks) ← from `/pm-brainstorm think of new milestones for w8` 2026-07-09
- [ ] **m5** — Meter managed Postgres & Key Value instance-seconds (9 tasks) ← from `/pm-brainstorm more milestones for w8` 2026-07-10
- [ ] **m6** — Usage history: GraphQL period support + dashboard multi-month view (9 tasks) ← from `/pm-brainstorm more milestones for w8` 2026-07-10, needs m2
- [ ] **m7** — Price sheet + estimated spend (Render-equivalent billing) (12 tasks) ← from `/pm-brainstorm for more` 2026-07-13 (user request fires `.pm/FUTURE-MAYBE.md`'s "Pricing & spend estimation" trigger; 30% off Render's compute/Postgres/KeyValue/build-minute prices, 90% off bandwidth; estimate-only, no payment collection — user-confirmed scope boundary)

## Inbox

- `001.md` — Usage-based plan enforcement (Hobby caps + approaching-limit notifications) — gated on m1 producing ~a month of real data
