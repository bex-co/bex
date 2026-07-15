# w8 — Usage metering (worker8)

**Worker:** worker8 Created 2026-07-09 from `/pm-brainstorm w8` — owns `GOAL.md` #5's unowned half ("usage metering"; the multi-tenant half is w1/m9 + w6 + w4/m12). Meters **quantities** — instance-seconds by tier, egress bytes, build minutes: exactly Render's three meters (verified live 2026-07-09 vs render.com/pricing + docs). Payments/payment-collection stay out per w6's "no billing system" boundary; **m7** (2026-07-13) adds dollar _estimates_ over these quantities (a price sheet, not a billing system — `.pm/FUTURE-MAYBE.md`'s "Pricing & spend estimation" trigger). Numbered **w8, not w7** — w7 was consumed as the old staging path for w1/m15; never reuse a number. Ordered by dependency: pipeline → API surface → dashboard.

## Local dev environment

Develop against `.pm/w8/dev-8/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-8` gets its own Kratos + Hydra + Mailpit (namespace `dev-8-auth`) and app namespace (`dev-8`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=8 (`dev-8/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash .pm/w8/dev-8/up.sh` — bring it up (idempotent — safe to re-run)
- `bash .pm/w8/dev-8/status.sh` — health check (processes, pods, HTTP)
- `bash .pm/w8/dev-8/down.sh` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)

`up.sh` prints the dashboard command to point at it once bex-api is running.

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
- [x] **m11** — Reliable usage windows: durable zeroes + gap-free per-meter cursors (7 tasks) ← prerequisite split from `001` 2026-07-14; cap enforcement stays gated — done 2026-07-14
- [x] **m12** — Managed Postgres major-version upgrade (9 tasks) ← from `/pm-brainstorm more milestones for each worker` round 2, 2026-07-14 (`database_types.go:35` — `Version` exists at create, no upgrade verb anywhere; Render ships version upgrades as a first-class flow; rides CNPG's declarative major-upgrade path, verified first by t002). Placed under w8 for capacity per the m8 precedent; numbered m12 not m11 — a concurrent session claimed m11 mid-rebase — done 2026-07-15
- [ ] **m13** — Datastore list pagination: Postgres + Key Value (7 tasks) ← from `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (`core.PageParams` in `apps/rest.go:309` but nowhere in `postgres/rest.go`/`keyvalue/rest.go`; Render's `GET /postgres` + `GET /key-value` both page); datastore-family placement per the m12 precedent
- [ ] **m14** — Postgres disk autoscaling (8 tasks) ← from `/pm-brainstorm` round 7, 2026-07-14 (systematic field-diff: `enableDiskAutoscaling`/`diskAutoscalingEnabled`, zero hits; the control loop between grow-only `storageGB` and w3/m10's already-scraped kubelet volume stats)
- [ ] **m15** — Complete outbound-bandwidth accounting: HTTP + WebSocket + direct + datastore TCP (14 tasks) ← prerequisite split from `001` 2026-07-14; replaces the HTTP-only counter before bandwidth caps can be promoted

## Inbox

- `001.md` — Usage-based plan enforcement (Hobby caps + approaching-limit notifications) — gate audited 2026-07-14: not ready; `m11` first makes collector coverage provable, then the note needs 28 days of real rows and an explicit decision on incomplete outbound-egress coverage
- `003.md` — Key Value (Valkey) version upgrade assessment: does Render support KV version changes at all? If yes, mirror the m12 pattern; if no, record and close ← from `/pm-brainstorm` round 3, 2026-07-14

> `002.md` promoted to **m9** 2026-07-13; note moved to `done/`. `004.md` (KeyValue `maxmemoryPolicy` underscore-vs-hyphen, filed by `w9/m2`'s Render CLI compatibility walk) fixed 2026-07-15 (`dfff3034`), re-verified live end to end (create/list/get/update/suspend/resume/delete) — note moved to `done/`. `005.md` (Postgres owner/options wire-shape, filed by `w9/m2`) retired 2026-07-15 — a parallel session independently found the same gap across Postgres/Service/KeyValue and filed it as `w6/016`; `005.md` moved to `done/` pointing there rather than duplicating it.
