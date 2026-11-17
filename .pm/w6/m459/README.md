# w6 · m459 — Plan pickers and the Key Value instance card show each plan's connection limit

**Worker:** worker6 **Goal:** as on Render, a user choosing or reviewing a Key Value plan sees its connection limit beside its RAM, read from bex-api's catalog, and the Postgres pickers show `max_connections` if Render's do. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Which Render surfaces show the connection limit](t001.md) | 30m | — |
| t002 | [GraphQL instance types carry the limit](t002.md) | 30m | t001, w6/m456/t001, w6/134 |
| t003 | [The Key Value create cards, instance card and plan picker show the limit](t003.md) | 1h | t002 |
| t004 | [The Postgres plan pickers show `max_connections`, if Render's do](t004.md) | 30m | t002 |
| t005 | [Every locale, both widths, skeletons unchanged](t005.md) | 30m | t003, t004 |
| t006 | [Render parity](t006.md) | 30m | t005 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- The Key Value create page's plan cards, the Key Value detail page's instance card and its plan picker show each plan's connection limit ("50 connections" on free), read from `keyValueInstanceTypes`, matching w6/m456's table.
- The Postgres create and plan pickers show `max_connections` if t001 found Render's do, and t001's finding is recorded either way.
- Every locale carries the copy, and pending and ready states match at desktop and narrow-mobile widths.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 5, filed by the user's `/pm them to w6`. Render's Key Value create page shows "Free — $0/mo, 25 MB RAM, 50 connections", and its detail page's Key Value Instance card shows the plan's name, RAM and connection limit ([key-value.md](../../../docs/render-artifacts/key-value.md), captured 2026-07-09).
- **bex today:** `KeyValueInstanceType` (`lego/backend/internal/keyvalue/graphql.go:74`) carries no limit, and the dashboard shows none.
- **Goal linkage:** ADR008 § Render parity first, with Render's dashboard as the UI reference.
- **Expected outcome:** users see the limit where they pick a plan, so a connection refused after w6/m456 is explainable.
- **Why now:** w6/m456 starts enforcing 50 and 250 connections; a limit the user cannot see before choosing reads as an outage.
- **Render parity task:** included. The plan catalog is a GraphQL-only bex extension because Render has no public catalog API; t006 confirms that stays true.
