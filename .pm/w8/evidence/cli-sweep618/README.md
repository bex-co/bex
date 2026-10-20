# CLI sweep 618 — service events pagination: cursor walk, limit bounds, malformed cursor

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `f31004f7c`, 2026-10-19. Owned fixture `qa618-web` (`srv-dbban5abpdss73ci0or0`, free hello-app) driven through suspend/resume ×2 + restart to mint six events, then deleted: REST 404, zero `qa618` names. REST boundary (the CLI has no events verb).

- **Cursor walk is exact:** paging with `limit=2` took 4 pages to collect 6 events, all ids unique, none repeated or skipped. `limit=100` returned the same 6 in one page, newest-first (`deploy_started, suspender_removed, suspender_added, …`).
- **Limit bounds are named:** `limit=0`, `limit=-1`, and `limit=9999` all → `400 limit must be an integer between 1 and 100`; `limit=abc` → `400 invalid query parameter "limit"`. No clamping, no 500s.
- **Malformed cursor:** `cursor=garbage!!` → `400 bad request: malformed cursor` — refused, not silently restarted from the first page.

## Unrun

- Cursor behavior when events are inserted mid-walk (the walk above raced only my own lifecycle verbs, already settled).
- Events pagination on datastores (`dpg-…` event streams).
