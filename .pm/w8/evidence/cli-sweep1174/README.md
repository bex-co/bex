# CLI sweep 1174 — dryRun validates and previews without persisting (live, 2026-10-20)

No fixture persisted; nothing to clean up. No new bug filed.

- `POST /services?dryRun=true` with a valid free web-service payload → 200 with a preview whose `id` is the would-be name (`qa1174-dry`), and a name search immediately after returns `[]`: the preview is not stored.
- The same dryRun with an invalid `type` → 400: validation runs in full during a dry run; it is not a rubber stamp.
