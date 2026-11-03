# w2 · m251 — Render drift check round 5: 208 → 252 operations, recorded and pinned

**Worker:** worker2 **Goal:** every operation Render added since 2026-09-10 has an ADR018 row with a status and an owner, and bex validates against Render's current OpenAPI contract. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Diff Render's live OpenAPI against the pin: new operations plus changed schemas on the existing 208 | 45m | — |
| t002 | Diff the render-mcp-server tip and Render's changelog since 2026-09-10 | 30m | — |
| t003 | Assess the Render CLI v2.29.0 → v2.35.0 delta against bex-api | 45m | t001 |
| t004 | Refresh the OpenAPI pin and guards; unserved new routes answer a named 404 | 1h | t001 |
| t005 | ADR018 round-5 paragraph and rows | 45m | t001, t002, t003 |
| t006 | File follow-ups for every other gap the round found | 20m | t005 |
| t007 | Simplify | 15m | t004, t006 |
| t008 | Test coverage | 30m | t004, t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- `lego/backend/internal/api/openapi/render-public-api-1.json` equals Render's live OpenAPI (252 operations as of 2026-11-02, or whatever live is when the pin lands), with its hash updated; `TestMCPParity*` and the OpenAPI parity/validator guards are green on the new pin.
- Every new operation and every changed request/response schema on the existing 208 operations appears in `docs/ADR018-render-parity.md` (a round-5 paragraph plus rows), each `✖`/`◐` with an owning milestone or inbox note, or `—` with its rationale.
- New Render routes bex does not serve answer a named 404 (the `/v1/objects` pattern), never a generic router 404 or a 500.
- The CLI v2.29.0 → v2.35.0 delta is recorded in `docs/cli-compatibility-checklist.md` (assessment only, no pin bump).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #1. Live diff 2026-11-02: `api-docs.render.com` has 252 operations vs the pin's 208; new groups `/build-sources` (16), `/flows` + `/flow-runs` (8), `/sandboxes` + `/sandbox-groups` (20); `service`, `servicePATCH` and every `*DetailsPOST` gained `buildSourceId`. Upstream `render-oss/cli` latest v2.35.0 vs pinned v2.29.0 (`lego/cli/UPSTREAM_RENDER_CLI.md`). Previous round: round 4 (w6/m142, 2026-09-10).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** the parity ledger reflects Render as of November 2026 again; gaps have owners instead of being invisible.
- **Why now:** the ledger is 7 weeks stale and 44 operations behind; current Render CLI releases may already send `buildSourceId`; w2/m254 (Build Sources) needs the refreshed pin.
- **Render parity task omitted:** this milestone *is* the parity audit — behavior gaps it finds are filed as their own milestones/notes (t006), not fixed here.
