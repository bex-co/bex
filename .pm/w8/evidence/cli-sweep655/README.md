# CLI sweep 655 — one-off jobs support gate (live, 2026-10-19)

Disposable free web service `qa655-web` (`srv-dbbbdelfsg2s73ako7mg`). Deleted and verified absent (REST 404).

- `bex jobs list` / `jobs create` on a synthetic service id → 404 `NOT_FOUND`.
- `POST /services/{id}/jobs` with a real command → 410 `ONE_OFF_JOBS_UNSUPPORTED: one-off jobs are not supported on bex; run the command in the service itself (bex ssh <service>), as a pre-deploy command, or as a cron job — existing jobs remain listable`. By design; `jobs list` on the real service returned `[]`.
- Missing `startCommand` (`{}`) → 400 `invalid request body at /startCommand`. Explicit `"startCommand":""` → bare 400 `bad request` (source: `core.ErrBadRequest` at `lego/backend/internal/jobs/rest.go:128`, checked before the 410 gate in `Create`).

The bare-400-on-empty-string is recorded as a minor wart on an endpoint that is Gone by design, not filed: the actionable path already refuses with a coded, explanatory 410, and the missing-field path names the field. No bug filed.
