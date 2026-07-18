# w1 · m51 — Egress correctness: count HTTPS bytes + record billable hours

**Worker:** worker1 **Goal:** make egress numbers true end to end — the router resolver counts Traefik's `websecure-` TLS sibling (where essentially all real traffic flows) so the chart, month-to-date, health source, and billing meter stop measuring only 80→443 redirects; and the usage rollup records each hour's healthy sources instead of deferring the whole hour forever, so prod finally writes `egress_bytes` rows. **Status:** done — **Done 2026-07-18**: prod-verified on the `fdbfb173` roll — 1h interactive read within 0.4% of a trusted reset-free Prometheus increase (1424.3 vs 1418.3 MiB; pre-fix chart averaged ~2 KB/s), month-to-date 83.1 MB → 84.6 GB (degraded-annotated long window), and the rollup cursor unfroze from 2026-07-16 09:00 to the last closed hour on its first pass writing 44 non-zero hours (69.75 GB) for the reference service while the frozen row and all instance/storage totals stayed byte-identical; cross-surface consistency automatic through the one shared resolver; evidence in ADR023.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Ground-truth Traefik's dual-router naming (prod config, not one observation) — **DONE** | 30m | — |
| t002 | Resolver fix: TraefikRouterNames includes the TLS sibling names (one fix point) — **DONE** | 45m | t001 |
| t003 | Usage rollup: per-source hour gating — record healthy sources, defer on transport — **DONE** | 45m | t001 |
| t004 | Prod verification: HTTPS volume in chart/month-to-date + egress_bytes rows land — **DONE** | 30m | t002, t003 |
| t005 | Render parity — cross-surface consistency + ADR018/ADR023 notes — **DONE** | 30m | t004 |
| t006 | Simplify — `/simplify` over the changed code — **DONE** | 30m | t005 |
| t007 | Test coverage — resolver siblings, per-source rollup fixtures — **DONE** | 45m | t005 |
| t008 | Closeout — verify DoD, sync status, move to done — **DONE** | 15m | t006, t007 |

## Definition of done

On production after the deploy:

- The reference service's Outbound Bandwidth chart and `monthToDateBandwidth` include HTTPS traffic: month-to-date rises well above the redirect-only 83 MB figure and is order-of-magnitude consistent with a short trusted-window Prometheus increase over the `websecure-` router (short window ⇒ no flap-reset inflation).
- Within ~2 hours of the deploy, the usage surface shows non-zero `egress_bytes` rows for at least one serving app in the current period — July previously had **zero** such rows.
- Historical usage rows are unchanged (undercounted history is noted, not rewritten).
- A transport failure (Prometheus unreachable) still defers the hour; a health-term failure no longer defers — pinned by tests.
- Backend suite + lint green; ADR023 prose matches the shipped behavior.

## Source + Goal linkage

- **Source:** inbox notes `w1/034` + `w1/035` (both filed 2026-07-18 from w1/m50/t006's prod sanity check): the API's 83.1 MB month figure matched the plaintext `default-` router's Prometheus increase (83.4 MiB) exactly while `websecure-` series carried the real volume; and July's usage table held zero `egress_bytes` rows because `queryEgressSources` defers any hour where any source fails a health term — and some term fails nearly every prod hour.
- **Goal linkage:** ADR023's metering integrity (bandwidth that is _true_, not merely never-invented) and Render-parity credibility of the bandwidth surfaces; direct continuation of w1/m50.
- **Expected outcome:** bandwidth surfaces report real HTTPS traffic for the first time; the billing meter produces rows on prod at all (undercounting only per-source per-hour instead of undercounting to zero).
- **Why now:** both defects are live on prod and every serving hour that passes is an unrecorded billing hour; m50 just built the machinery (per-source health, degraded-as-data) this milestone composes with; the two notes fix ends of the same pipe — shipping one without the other leaves egress still wrong.
- **Render parity task included:** the REST/GraphQL/MCP/dashboard bandwidth and usage numbers change semantics (grow to include HTTPS); shapes stay identical.

## Non-goals

- Correcting historical usage rows (they undercounted, never overcounted — consistent with "never invent bytes"; note it in ADR023, don't rewrite data).
- Changing m50's interactive-read semantics (shipped).
- Reset-tolerant http _accounting_ beyond per-source skipping — hours where the websecure counter flaps stay excluded from billing by the per-source health gate (its `increase()` inflation is exactly what the gate exists to block).
