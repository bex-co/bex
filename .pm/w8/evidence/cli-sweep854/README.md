# CLI sweep 854 — create `dryRun` writes nothing (live, 2026-10-20)

No fixture persisted (both dry-runs verified by empty name-filter lists). No new bug filed.

- `POST /services?dryRun=true` → 200 (not 201) with the resolved service object (name-as-id placeholder, since nothing is minted); `GET /services?name=qa854-dry` afterwards → `[]`.
- The body form `{"dryRun": true, …}` behaves identically (200, nothing persisted) — the w2/m29 preview contract on both spellings.
