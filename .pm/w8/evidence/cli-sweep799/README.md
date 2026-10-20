# CLI sweep 799 — deploy list/get/trigger cycle (live, 2026-10-20)

Disposable free image-backed web service created, deployed, deleted (204, then GET 404). No new bug filed.

- Initial deploy appears in `GET /services/{id}/deploys` as `update_in_progress` and reaches `live` on single GET within ~25s, with `startedAt`/`finishedAt` populated.
- `POST /services/{id}/deploys {"clearCache":"do_not_clear"}` → 201, new `dep-…` with `trigger:"api"`.
- Bogus `clearCache` value → 400 `invalid request body at /clearCache` (enum validated by name, per the w2/m30 contract in [ADR018](../../../docs/ADR018-render-parity.md)).
