# CLI sweep 1225 — blueprint sync history reads empty (live, 2026-10-20)

Read-only sweep; no sync was triggered and the baseline blueprint was never mutated. No new bug filed.

- `GET /blueprints/{baseline}/syncs?limit=3` → 200 `[]` (no recorded sync runs for the stored-manifest baseline).
- A correct-form but unknown blueprint id on the syncs route → 404, complementing the malformed-id 400 from sweep 1157: form gates first, existence second.
