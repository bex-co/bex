# w8 — project workstream (worker8)

**Worker:** worker8. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Local dev environment

Develop against `.pm/w8/dev-8/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-8` gets its own Kratos + Hydra + Mailpit (namespace `dev-8-auth`) and app namespace (`dev-8`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=8 (`dev-8/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash scripts/dev-env.sh 8 up` — bring it up (idempotent — safe to re-run)
- `bash scripts/dev-env.sh 8 status` — health check (processes, pods, HTTP) + a pass/fail verification inventory
- `bash scripts/dev-env.sh 8 down` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)
- `bash scripts/dev-env.sh 8 clean` — reclaim `logs/` and `bin/` (refuses while the environment is up)

`up` prints the dashboard command to point at it once bex-api is running. One shared
implementation serves every workstream since `w1/m72`; `.pm/w8/dev-8/` keeps only
`ports.env` (a generated record of the derivation), this README, and `.gitignore`.

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
- [x] **m13** — Datastore list pagination: Postgres + Key Value (7 tasks) ← from `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (`core.PageParams` in `apps/rest.go:309` but nowhere in `postgres/rest.go`/`keyvalue/rest.go`; Render's `GET /postgres` + `GET /key-value` both page); datastore-family placement per the m12 precedent
- [x] **m14** — Postgres disk autoscaling (8 tasks) ← from `/pm-brainstorm` round 7, 2026-07-14 (systematic field-diff: `enableDiskAutoscaling`/`diskAutoscalingEnabled`, zero hits; the control loop between grow-only `storageGB` and w3/m10's already-scraped kubelet volume stats) — done 2026-07-15
- [x] **m15** — Complete outbound-bandwidth accounting: HTTP + WebSocket + direct + datastore TCP (14 tasks) ← prerequisite split from `001` 2026-07-14; replaces the HTTP-only counter before bandwidth caps can be promoted
- [x] **m16** — Managed-datastore polish chores: create ipAllowList parity + pg_stat_statements backfill + KV version assessment (8 tasks; t008 added by round 19 — Render's inline PATCH `parameterOverrides` silently ignored) ← groups round-14 consistency find (postgres create `ipAllowList` is REST-only while keyvalue has all three surfaces) with notes `003` + `006`, 2026-07-15 (the w7/m37 chores pattern); coordinates with w4/m24 (descriptions) and w9/m38 (error bodies)
- [x] **m17** — Disk-autoscaling hardening: loud sample-failure signal + single-sourced 16 TB cap (6 tasks) ← from `/pm-brainstorm` round 16, 2026-07-15 (consistency-mine over the day-old m14 diff: `database_autoscale.go:176-179` swallows sample-unavailable at Info level and silently no-ops — the feature's whole purpose defeated with no Event/condition; the 16 TB cap literal duplicated across operator, dashboard, MCP description, and docs) — done 2026-07-16
- [x] **m18** — Blueprint estimated pricing panel (Render blueprint/new parity) (8 tasks) ← from deep-research session 2026-08-04 (bex `blueprints/new` review shows names only vs Render's Estimated pricing panel; server-side estimate from the m7 price sheet through the ADR049 preview payload)
- [x] **m19** — Blueprint spec-parity round 2: registry sweep, custom paths, schema drift (11 tasks) ← from blueprint Render-parity review 2026-08-15 (the m63 audit-baseline `unsupported` set never swept: static `buildCommand`/`dockerContext`/`registryCredential` block real-world yamls; Render's 2026-02-09 custom Blueprint paths; schema pin aging; ADR018 Blueprint row ◐ pending CLI evidence)
- [x] **m20** — Blueprint grouping hardening: transactional writes, quota, audit, disconnect reclaim (8 tasks) ← promotes `w1/049` #5 residuals 2026-08-16 (security-scan round 7 deferred register; fix-shape precedent `w7/done/m72/t004`); sequenced after m19 in the same workstream since both touch `blueprint.go`
- [x] **m21** — Blueprint dashboard completion: sync:false prompts, pre-sync diff, settings edit (7 tasks) ← from blueprint lifecycle-semantics verification 2026-08-16 (backend `envVarValues` channel complete but the dashboard's `CreateBlueprint` mutation lacks the variable, so dashboard-created blueprints deploy secret placeholders empty; detail-page Sync applies blindly; name/path PATCHable but not editable in UI)
- [x] **m22** — Generate Blueprint: export existing resources as render.yaml (7 tasks) ← from blueprint lifecycle-semantics verification 2026-08-16 (Render's select-services→download render.yaml has no bex equivalent; the m19 registry allowlist + verified adoption-by-name make it a serialization exercise with a no-op round trip)
- [x] **m23** — Blueprint resource ownership: stop silent cross-blueprint overwrite (6 tasks) ← from blueprint lifecycle-semantics verification 2026-08-16 (no ownership marker — two blueprints naming the same resource silently clobber each other; Render warns "last sync wins", bex refuses with takeover confirmation); best sequenced after m21/m22, which reuse the same review/confirmation surfaces
- [x] **m24** — Webhook event hydration: retrieve a payload's `data.id` (9 tasks) ← from authenticated Bex↔Render webhook parity audit 2026-08-17; closes the event-detail residual after w2/m70
- [x] **m25** — Webhook immutable attempt history + manual Resend (13 tasks) ← from authenticated Bex↔Render webhook parity audit 2026-08-17, needs m24
- [x] **m26** — Webhook management UX + drift-proof dashboard parity (10 tasks) ← from authenticated Bex↔Render webhook parity audit 2026-08-17, needs m25
- [x] **m27** — Granular OAuth capability scopes and authorization-decision audit (9 tasks) ← proposal 1 from `/pm-brainstorm for w8`, selected by user 2026-08-18
- [x] **m28** — Polish `/agents` as a prompt-first workspace (8 tasks) ← designer review of `dashboard.bex.co/agents` 2026-08-18, user handoff to w8
- [ ] **m29** — bex CLI help chrome: strip Render branding without forking (7 tasks) ← from CLI branding research handoff 2026-08-19; Layer-1 `RootCmd` overlay only — DO_NOT_DO no-fork honored

## Inbox

No open inbox notes.

> `001.md` retired 2026-08-16 after its production gate found only 1–2 represented workspaces, zero build minutes, a 1.46 TiB sole complete egress sample, and remaining hourly gaps — no evidence-safe cap can be selected. `007.md` retired after its one useful artifact was captured: the production unmodified-CLI validation re-grade is now in `docs/cli-compatibility-checklist.md`; the note mixed that finished evidence chore with disposable deployment experiments and an unrelated dev-host suggestion, so it is not retained as roadmap work. `008.md` implemented and moved to `done/` (env groups in `resources[]` + action plans). `009.md` implemented and moved to `done/` (ADR018 deliberate divergences). `002.md` promoted to **m9** 2026-07-13; note moved to `done/`. `004.md` (KeyValue `maxmemoryPolicy` underscore-vs-hyphen, filed by `w9/m2`'s Render CLI compatibility walk) fixed 2026-07-15 (`dfff3034`), re-verified live end to end (create/list/get/update/suspend/resume/delete) — note moved to `done/`. `005.md` (Postgres owner/options wire-shape, filed by `w9/m2`) retired 2026-07-15 — a parallel session independently found the same gap across Postgres/Service/KeyValue and filed it as `w6/016`; `005.md` moved to `done/` pointing there rather than duplicating it.
