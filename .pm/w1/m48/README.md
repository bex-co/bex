# w1 · m48 — Deploy progress lines: a never-silent deploy log feed

**Worker:** worker1 **Goal:** The deploy log feed always has something honest to say from the second a deploy is created — Render-style platform progress lines (`==> Build queued…`, `==> Building from <repo>…`, `==> Your service is live`) synthesized from the deploy lifecycle, visible identically on REST/GraphQL/MCP, the SSE tail, and the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's platform progress-line contract (vocabulary, log type, ids, CLI)         | 30m | —          |
| t002 | Core synthesis: deploy-lifecycle entries merged into windowed `type=build` history reads  | 45m | t001       |
| t003 | SSE tail: emit synthesized progress lines on deploy-status transitions while following    | 45m | t002       |
| t004 | Dashboard: verify the deploy page never renders the silent-window empty state             | 30m | t003       |
| t005 | Render parity: same lines on REST/GraphQL/MCP + official CLI `render logs --type build`   | 30m | t004       |
| t006 | Simplify: `/simplify` over the code this milestone changed                                | 20m | t005       |
| t007 | Test coverage: deterministic ids, window edges, no-duplicate merge, terminal-line cases   | 30m | t005       |
| t008 | Closeout: verify DoD, move milestone to done                                              | 10m | t007       |

## Definition of done

- Opening a deploy's page within seconds of creating it (cold node, build pod still Pending) shows a first platform line (e.g. `==> Build queued`) on the next poll/tail frame — never minutes of empty feed while only the previous revision's app lines scroll.
- The same synthesized lines are returned by REST `GET /v1/logs?type=build`, GraphQL `logs(type:"build")`, MCP `list_logs`, and streamed by `GET /v1/logs/subscribe?type=build` — one core verb, three adapters, no surface drift.
- The unmodified official Render CLI (`render logs --type build`) shows them against bex-api.
- Terminal outcomes produce a closing line (`==> Your service is live` / `==> Build failed`) inside the deploy window.
- Synthesized entries carry deterministic ids (deploy id + phase) so dedupe across the history/tail merge can never double-print them, and they never mask the `buildStoreUnavailable` state (a missing Loki still reports itself; synthesis is additive, not a cover).

## Source + Goal linkage

- **Source:** the 2026-07-17 production incident on `dep-d9d8r3h07a5s73dj32ag` (deploy page showed only old-revision service logs during a build) — evidence + Render recheck in `docs/render-artifacts/live-deploy-following.md` § 2026-07-17; the dead-tail/ingest-lag half shipped as `52882f22`.
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md`) and observability (`docs/ADR010-observability.md`) — Render's deploy feed is never silent because the platform itself speaks (`==> Cloning from…`, `==> Checking out commit…`, verified via render.com/docs/your-first-deploy and the live dashboard walk).
- **Expected outcome:** no silent window on cold-node builds; users watching a deploy always see the platform's own narration between deploy creation and the build pod's first real line, on every surface including the official CLI.
- **Why now:** `52882f22` fixed the tail dying and the ingest race, but a cold node still has a minutes-long window where *no* build line exists anywhere — the incident's remaining UX gap, with fresh captured evidence to build against. Render parity task included: this changes what the logs verbs return on REST/GraphQL/MCP and what the dashboard shows.
