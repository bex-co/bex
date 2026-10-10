# w8 · m56 — Publish complete service settings before rolling a release

**Worker:** worker8 **Goal:** A successful compound settings update deploys only its complete requested configuration. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Publish complete CR-owned compound settings once | 80m | — |
| t002 | Audit table rows, adapters and persistence boundaries | 45m | t001 |
| t003 | Replay combined and single-field clears in both clients | 35m | t002 |
| t004 | Verify Render and cross-surface settings contracts | 25m | t003 |
| t005 | Simplify the changed code | 15m | t004 |
| t006 | Add meaningful publication and execution regressions | 45m | t004 |
| t007 | Close out after the observable outcome holds | 10m | t005, t006 |

## Definition of done

- Fresh owned Free Python web fixtures begin Live with `/healthz` and distinct harmless pre-deploy markers. Published Bex and actual unmodified pinned Render each run the original combined clear command successfully. Readbacks have both settings empty, a new current deploy is Live, and HTTP 200 serves the expected fixture marker.
- Neither client creates a new pre-deploy instance executing the removed marker after the combined clear. Initial marker capture, server timestamps, different instance IDs and time-bounded queries distinguish genuine execution from retained logs. Populated restore and single-field clear controls remain correct.
- A controlled observer of actual successful compound App writes never sees a release-triggering partial requested configuration. One accurate deploy-history row identifies the complete released configuration. Tests cover execution gates rather than only final row counts.
- All 22 settings rows / 28 fields and both ApplyServicePatch adapters have explicit ownership and compatibility dispositions. Single-field setters, save-only source changes, dry-run, deterministic invalid preflight, operational-only changes, conflict handling and truthful unexpected persistence failures retain their contracts.
- Attach dated post-fix live evidence. Delete only owned fixtures, verify detail/list/runtime absence and quota convergence with baseline IDs retained. Complete standing tasks and move the milestone into `w8/done/m56/` only when these outcomes are real.

## Source + Goal linkage

- **Source:** ongoing user-requested `$qa-find-bugs-cli` w8 loop, sweep 77, 2026-10-10 UTC. [Researched finding and durable evidence](finding.md). Filing only; no implementation or ship authorization.
- **Goal linkage:** ADR008 reliable hosting; ADR006 consistent API adapters; ADR018 faithful pinned Render CLI behavior; completed w6/m51's one-request/one-rollout promise.
- **Expected outcome:** removing a pre-deploy command in a successful compound save prevents that command from executing in the resulting release.
- **Why now:** both real clients execute the removed command on independent fresh production fixtures despite successful final empty readbacks. Current-main source observation confirms intermediate App publication, so coalesced history and invalid-request preflight do not resolve the defect.
- **Sizing:** seven tasks, 4h15m; configuration ownership spans 22 table rows and multiple persistence paths, with controller execution and live regression obligations.
- **Scope:** server/runtime configuration publication. Render parity is included because REST/MCP compound updates and shared GraphQL/dashboard setters expose these settings. Preserve the actual pinned client, existing source-save semantics and individual setter contracts.
- **Unverified:** actual Render backend, live MCP/GraphQL/browser, paid App kinds and direct production CR/Job generations. Source-only consumers are audit obligations, not passing claims.
