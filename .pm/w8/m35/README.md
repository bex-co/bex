# w8 · m35 — Datastore deleting-read parity: fold Postgres/Key Value onto consistent-absence

**Worker:** worker8 **Goal:** a deleting Postgres or Key Value reads `not found` by id the instant deletion starts — the same contract services got in w3/m81, matching List and Render's 404 **Status:** todo

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Pin Render's deleted-datastore read evidence; confirm option 1           | 30m | —          |
| t002 | Postgres by-id reads gain `core.NotFoundIfDeleting`                      | 30m | t001       |
| t003 | Key Value by-id reads gain `core.NotFoundIfDeleting`                     | 30m | t001       |
| t004 | Cross-surface tests + finalizer-bound story + ADR closure                | 45m | t002, t003 |
| t005 | Render parity: cross-surface consistency check (REST/GraphQL/MCP/UI)     | 30m | t004       |
| t006 | Simplify: `/simplify` over the code this milestone changed               | 30m | t005       |
| t007 | Test coverage: meaningful tests for the behavior this milestone shipped  | 30m | t005       |
| t008 | Closeout: verify DoD, mark done, move milestone to done/                 | 15m | t007       |

## Definition of done

The instant a `Database` or `KeyValue` App carries a `DeletionTimestamp`, every by-id read surface (REST get + connection-info and sibling by-id reads, GraphQL, MCP) returns `not found`, consistent with `List` (which already omits deleting rows) and with the w3/m81 service contract — no family answers `deleting` by id while absent from its list. The finalizer window remains bounded/observable for datastores (w6/m129 `terminating` reconciliation confirmed, or a gap filed). ADR006/ADR009/ADR021 and the ADR018 rows record the decision with its evidence boundary.

## Source + Goal linkage

- **Source:** `w3/034` (filed by `w3/m81/t005` as the deliberately-scoped-out datastore half; retired to `w3/done/034.md` pointing here). Ratified as `/pm-brainstorm for w8` 2026-09-07 #3 (option 1 — fold onto consistent-absence), approved same day.
- **Goal linkage:** Render parity + one coherent cross-family read contract (ADR006). Render's datastore delete reaches `GET` 404; bex services already match, and the datastores are the odd ones out.
- **Expected outcome:** no tenant-observable services-vs-datastores split; the current unbounded, undocumented `deleting`-by-id + list-drop incoherence — the same shape as the m81 incident — is gone rather than latent.
- **Why now:** the fix shape and precedent (`core.NotFoundIfDeleting`, m81's call-site pattern) are fresh and cheap to apply; leaving it stands the incoherence on two more families as an accident, not a decision.
- **Render parity closing task included:** the change is wire-visible on datastore reads across REST/GraphQL/MCP, with the dashboard's deleting-state rendering affected.
- **Evidence boundary:** live Render capture is blocked — no `RENDER_API_KEY` exists (`w10/002`) — so t001 pins documented evidence (OpenAPI/docs) and records the boundary the way prior ledger rows do.
