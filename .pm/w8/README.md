# w8 — Usage metering (worker8)

**Worker:** worker8 Created 2026-07-09 from `/pm-brainstorm w8` — owns `GOAL.md` #5's unowned half ("usage metering"; the multi-tenant half is w1/m9 + w6 + w4/m12). Meters **quantities** — instance-seconds by tier, egress bytes, build minutes: exactly Render's three meters (verified live 2026-07-09 vs render.com/pricing + docs). Payments/payment-collection stay out per w6's "no billing system" boundary; **m7** (2026-07-13) adds dollar *estimates* over these quantities (a price sheet, not a billing system — `.pm/FUTURE-MAYBE.md`'s "Pricing & spend estimation" trigger). Numbered **w8, not w7** — w7 was consumed as the old staging path for w1/m15; never reuse a number. Ordered by dependency: pipeline → API surface → dashboard.

## Milestones

- [x] **m1** — Metering pipeline: hourly usage rollups into the control-plane store (9 tasks) ← from `/pm-brainstorm w8` 2026-07-09
- [x] **m2** — Usage API: month-to-date usage over REST · GraphQL · MCP (9 tasks) ← from `/pm-brainstorm w8` 2026-07-09, needs m1
- [x] **m3** — Dashboard Usage page (workspace-scoped, Render-consistent) (8 tasks) ← from `/pm-brainstorm w8` 2026-07-09, needs m2
- [x] **m4** — Usage data retention: compact hourly detail into monthly aggregates (9 tasks) ← from `/pm-brainstorm think of new milestones for w8` 2026-07-09
- [x] **m5** — Meter managed Postgres & Key Value instance-seconds (9 tasks) ← from `/pm-brainstorm more milestones for w8` 2026-07-10
- [x] **m6** — Usage history: GraphQL period support + dashboard multi-month view (9 tasks) ← from `/pm-brainstorm more milestones for w8` 2026-07-10, needs m2
- [x] **m7** — Price sheet + estimated spend (Render-equivalent billing) (12 tasks) ← from `/pm-brainstorm for more` 2026-07-13 (user request fires `.pm/FUTURE-MAYBE.md`'s "Pricing & spend estimation" trigger; 30% off Render's compute/Postgres/KeyValue/build-minute prices, 90% off bandwidth; estimate-only, no payment collection — user-confirmed scope boundary)
- [x] **m8** — Service display name: rename without breaking the immutable resource id (8 tasks) ← from `/pm-brainstorm more` 2026-07-13 (`docs/ADR018-render-parity.md` "Change instance plan / type" row note — `name` PATCH field not editable). Originally proposed under `w2`, materialized under `w8` per user direction
- [x] **m9** — Meter managed Postgres & Key Value storage separately from compute (9 tasks) ← promotes `002` 2026-07-13 (Render drift follow-up from m5, named directly in `docs/ADR018-render-parity.md`'s usage-metering row)
- [ ] **m10** — Env vars: `generateValue` + cursor pagination (8 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (the ADR006/ADR018 env-vars row's two documented omissions; `w1/m35`'s generateValue prerequisite). Not usage work — placed under w8 for capacity per the m8 precedent
- [x] **m11** — Reliable usage windows: durable zeroes + gap-free per-meter cursors (7 tasks) ← prerequisite split from `001` 2026-07-14; cap enforcement stays gated — done 2026-07-14

## Inbox

- `001.md` — Usage-based plan enforcement (Hobby caps + approaching-limit notifications) — gate audited 2026-07-14: not ready; `m11` first makes collector coverage provable, then the note needs 28 days of real rows and an explicit decision on incomplete outbound-egress coverage

> `002.md` promoted to **m9** 2026-07-13; note moved to `done/`.
