# CLI sweep 649 — deploy trigger, supersede, and bad-image failure (live, 2026-10-19)

Disposable free web service `qa649-web` (`srv-dbbb93ivfpec73dm0j7g`). Deleted and verified absent (REST 404).

- `POST /services/{id}/deploys` with a nonexistent image tag: 201 `queued` (async by design). With `{"clearCache":"yes"}`: 400 `invalid request body at /clearCache`. With `{}`: 201, redeploys the current image.
- Supersede semantics: queueing a newer deploy canceled the still-queued bad-image deploy and the initial creation deploy; the newest deploy went `live`.
- Isolated bad-image deploy (nothing queued after it): `update_in_progress` for ~90s, then `update_failed` with `finishedAt` set. The previously live version kept serving throughout (public URL stayed HTTP 200).

Async acceptance, newest-wins cancellation, terminal failure with timestamp, and zero-downtime retention all behave correctly. No bug filed.
