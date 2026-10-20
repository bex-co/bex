# CLI sweep 724 — deploy rollback dialect and lifecycle (live, 2026-10-19)

Disposable free web service `qa724-web` (`srv-dbbc9ctfsg2s73akoa90`) with two successive live deploys. Deleted and verified absent (REST 404).

- The nested guess `POST /services/{id}/deploys/{dep}/rollback` → 404 (no such route). Correct dialect: `POST /services/{id}/rollback {"deployId": …}`.
- Rollback to the currently-live deploy → 409 `deploy "…" is already live — nothing to roll back to`. Missing `deployId` → 400 `invalid request body at /deployId`; synthetic deploy → 404 `deploy not found`.
- After a second deploy went live, rollback to the first → 201 with `trigger:"rollback"` and `rollbackOf` naming the target; it progressed `created → update_in_progress → live`, public URL 200 throughout.

No bug filed.
