# CLI sweep 977 — deploy trigger and cancel discipline (live, 2026-10-20)

Two disposable free web services created; both deleted (204s). No new bug filed.

- `POST /services/{id}/deploys {clearCache:"do_not_clear"}` → 201, deploy `queued` with `trigger: api`; a bogus `clearCache` value → 400.
- Cancel with an (empty) JSON body → coded 400 `request body is not allowed for this operation`; cancel with no body → 200 and the deploy reads back `status: canceled` with `finishedAt` set. Same strict no-body discipline as restart (sweep 959), coded on both sides.
- The first fixture's deploy was still `queued` when its service was deleted; the delete succeeded and took the deploy with it.
