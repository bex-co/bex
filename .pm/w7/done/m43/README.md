# w7 · m43 — Scaling page parity: Manual Scaling card + Recent Metrics

**Worker:** worker7 **Goal:** bex's Scaling tab matches Render's live `/scaling` page structure — a **Manual Scaling** card (instance slider + input + Save) sits on the Scaling page and is shown exactly when autoscaling is off, and a **Recent Metrics** section (48h average CPU/memory utilization + total instances, with a "View all metrics" link) grounds the scaling decision in data. The API surface is already at full parity (REST `POST /scale` + `GET/PUT/DELETE /autoscaling`, GraphQL `scaleService`/`setAutoscaling`/`disableAutoscaling`, MCP `scale_service`/`get_autoscaling`/`set_autoscaling`/`disable_autoscaling` — w2/m12 + w1/m20); this milestone is dashboard-only UI parity plus small copy/range corrections. **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Manual Scaling card on the Scaling page (slider 1–100 + input + Save; hidden while autoscaling on) — **DONE** | 45m | —          |
| t002 | Recent Metrics section: 48h avg CPU/memory utilization + total instances — **DONE** | 45m | —          |
| t003 | Copy + range corrections: en "Render"→"bex", slider max 25→100, default target 60, disable-confirm messaging — **DONE** | 30m | —          |
| t004 | Render parity — verify Scaling page + API consistency vs Render's live page; update ADR018 + manual-scaling.md — **DONE** | 20m | t001, t002, t003 |
| t005 | Simplify — `/simplify` over the milestone's diff — **DONE** | 15m | t004       |
| t006 | Test coverage — scaling-page composition + manual-scale card + metrics section tests — **DONE** | 30m | t004       |
| t007 | Closeout — move to `done/` when the DoD holds — **DONE** | 15m | t006       |

## Definition of done

On the Scaling tab (`/services/<name>/scaling`) of a web service:

1. **Manual Scaling card** appears when autoscaling is off — heading + Render's explanatory copy, an instance slider (1–100) with a numeric input, and a Save button enabled only when the draft differs from the live count; saving calls the existing `scaleService` mutation and the header's Instances fact reflects the new count after refetch. The card is hidden while autoscaling is on (Render's mutual exclusion), and the Settings page's "Instance count" stepper is removed (Render's Settings has no instance control — the w5/m16 placement followed a docs-fallback guess that the live capture corrected).
2. **Recent Metrics section** renders below the scaling controls: average CPU utilization and average memory utilization across all instances plus total instances for the past 48 hours (the existing `metrics` GraphQL query with CPU/MEMORY percentage mode + INSTANCE_COUNT), an explanatory "past 48 hours" note, a "View all metrics" link to the Metrics tab, and honest empty states when no data is captured (Render's "No data captured in the past 48 hours").
3. **Copy and ranges corrected:** the en autoscaling hint says "bex scales the number of instances…" (`en.ts:329` currently says "Render"; zh is already correct); the min/max instance sliders allow up to 100 (currently 25; backend already validates 1–100); the default target percent on first enable is 60 (Render's default; currently 75); disabling autoscaling surfaces Render's "your service will run the fixed number of instances specified under Manual Scaling" messaging in the existing confirm/save flow.
4. Cron jobs / static sites keep their existing type-aware behavior (no replica concept ⇒ no manual card); nothing changes on REST/GraphQL/MCP.

## Source + Goal linkage

- **Source:** User request 2026-07-16 — live Playwright comparison of Render's `/web/srv-…/scaling` (beancount-dashboard-ssr, autoscaling toggled on/off to capture both states, then restored) against bex's `/services/nodejs-starter1/scaling` on dev-7. Render's page = Autoscaling card (mutual-exclusive with) Manual Scaling card + Recent Metrics (48h avg memory/CPU utilization, total instances, "View all metrics"). bex's page = autoscaling card only; the manual stepper lives on Settings (w5/m16, placed from the docs-fallback reconstruction in [docs/render-artifacts/manual-scaling.md](../../../docs/render-artifacts/manual-scaling.md) — "Captured: docs-fallback (render.com login required…)"; today's live capture corrects the placement). Backend survey confirmed full API parity: REST `rest.go:1354,1360-1362`, GraphQL `graphql.go` (`scaleService`/`setAutoscaling`/`disableAutoscaling`/`autoscalingConfig`), MCP `mcp.go` (4 tools), metrics `/v1/metrics/{cpu,memory,instance-count}` all exist.
- **Goal linkage:** Render dashboard parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md) rows "Manual scale (instance count)" and "Autoscaling config" — both marked UI ✅, but the manual-scale UI cell cites the Settings stepper, which the live capture shows is the wrong page); GOAL.md elasticity pillar (w1/m20's autoscaling + w2/m12's scale verb get their Render-shaped front door).
- **Expected outcome:** A bex user lands on Scaling and sees Render's exact page structure: pick manual instance count or enable autoscaling in one place, with the last 48h of utilization data right there to inform the choice — no detour through Settings for instance count or Metrics for utilization.
- **Why now:** w7 is the live-comparison polish workstream (m41 static-site create, m42 logs UX — same user-research pattern); the en-locale "Render" copy bug ships in production today and the misplaced stepper contradicts the parity ledger's UI ✅ claim; all the backend pieces exist, so this is low-risk dashboard-only work.
- **Render parity:** included (t004) — UI-surface work touching the Scaling page; the check verifies the page against the live Render capture, confirms no REST/GraphQL/MCP drift (none expected — dashboard-only), and updates the ADR018 manual-scale row's UI evidence plus the manual-scaling.md artifact's placement note.
