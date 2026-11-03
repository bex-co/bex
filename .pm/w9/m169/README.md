# w9 · m169 — ADR095 phase 2 — attachments in the `/agents` dashboard

**Worker:** worker9 **Goal:** a dashboard user can paste, drop or pick files when starting or steering a session and sees them in the conversation. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Upload client and attachment chips in both composers | 90m | — |
| t002 | Render user-turn attachments: thumbnails, file cards, lightbox | 60m | t001 |
| t003 | Live dashboard verification at desktop and narrow mobile | 40m | t002 |
| t004 | Render parity (Devin reference) | 25m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Both composers accept picker, drag-and-drop and pasted clipboard images; chips show progress, errors in the server's words and removal; send is disabled while uploading; limits are read from capabilities.
- User turns show raster thumbnails (lightbox) and download cards for every other type; SVG/HTML/PDF never render inline.
- Verified live at desktop and narrow-mobile widths with screenshots in `evidence/`.
- Dashboard `yarn test`, typecheck and lint green.

## Source + Goal linkage

- **Source:** [ADR095](../../../docs/ADR095-agent-attachments.md) (accepted for implementation by user 2026-11-03: "/pm ADR095 to w9"); Devin gap list in [ADR094](../../../docs/ADR094-coding-agent-mcp.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 (cloud coding agents); Devin parity for agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** the "paste a screenshot and say fix this" flow works in the product UI, matching Devin's web app.
- **Why now:** depends on m168's API; ADR095 D12 phase 2.
- **Dependency:** m168 (REST upload, GraphQL attachment fields, capabilities.attachments).
- **Parity:** included: dashboard surface over the m168 API; Devin web app is the reference.
