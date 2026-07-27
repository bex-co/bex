# w5 · m53 — Auto-Deploy trigger parity: `autoDeployTrigger` across REST/GraphQL/MCP/UI

**Worker:** worker5 **Goal:** Auto-Deploy is presented and wired the way Render does it — a disabled select showing the trigger value ("On Commit" / "Off") with pencil-edit in the UI, and `autoDeployTrigger` accepted/emitted on the Render-compatible REST surface alongside legacy `autoDeploy` — mapped onto bex's existing boolean, with "After CI Checks Pass" recorded as a conscious divergence. **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | REST: accept + emit `autoDeployTrigger` mapped onto `spec.autoDeploy`        | 45m | —          |
| t002 | GraphQL + MCP: expose the trigger consistently with REST                     | 30m | t001       |
| t003 | Dashboard: Render-style disabled trigger select replaces the bare switch     | 30m | t002       |
| t004 | Document the `checksPass` divergence (ADR018 + CLI checklist)                | 20m | t001       |
| t005 | Render parity — cross-surface consistency check                              | 30m | t003, t004 |
| t006 | Simplify — run `/simplify` over the changed code                             | 20m | t005       |
| t007 | Test coverage — mapping matrix + UI select tests                             | 30m | t005       |
| t008 | Closeout — verify DoD, mark done, move milestone                             | 15m | t007       |

## Definition of done

`GET /v1/services/{id}` emits `autoDeployTrigger: "commit"|"off"` consistent with `autoDeploy: "yes"|"no"`; PATCH accepts either field (trigger wins when both sent, matching Render precedence), `checksPass` is rejected with a Render-shaped validation error whose message names the divergence; GraphQL and MCP read/write the same vocabulary; the dashboard's Deploy card renders Auto-Deploy as a disabled select showing "On Commit"/"Off" with the m50 pencil-edit flow (no bare switch); ADR018 + `docs/cli-compatibility-checklist.md` record `checksPass` as a conscious divergence. Backend + dashboard suites green.

## Source + Goal linkage

- **Source:** user request 2026-07-26 — live Render walk 2026-07-26/27: Render renders `autoDeployTrigger` as a disabled select "On Commit" with pencil-edit (Cancel/"Save changes" verified live); Render's public API v1 carries `autoDeployTrigger: off|commit|checksPass` alongside legacy `autoDeploy: yes|no`. bex: bare switch over boolean `spec.autoDeploy` (w2/m9).
- **Goal linkage:** Render parity pillar — both the wire contract (`docs/ADR006-bex-api.md`: Render-compatible REST) and the UI presentation.
- **Expected outcome:** Render clients (including the official CLI) sending `autoDeployTrigger` work against bex-api; dashboard users see Render's exact control.
- **Why now:** Surfaced by the same settings walk driving m50–m52; the UI half consumes m50/t001's select variant, so it sequences naturally after m50. `checksPass` needs GitHub checks integration — out of scope by decision, documented not built.
- **Render parity task included:** yes — REST/GraphQL/MCP/UI all change; the parity task exercises all four plus the official CLI's service-update path.
