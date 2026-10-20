# CLI sweep 731 — deploy-create options and cancel of a live deploy (live, 2026-10-19)

Disposable free web service `qa731-web` (`srv-dbbceglfsg2s73akob10`). Deleted and verified absent (REST 404).

- `POST /deploys {"clearCache":"do_not_clear"}` and `{"clearCache":"clear"}` → both 201 (Render's enum spellings accepted; the sweep-649 string-type probe remains refused). Three queued deploys superseded down to one live, per sweep 649.
- `{"imageUrl":""}` → 201 deploying the current image (empty reads as absent).
- Canceling the now-live deploy via the CLI → 409 `conflict: deploy "…" is already live` (terminal-state conflict, matching sweep 654's already-canceled shape).

No bug filed.
