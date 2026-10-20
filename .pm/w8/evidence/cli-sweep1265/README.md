# CLI sweep 1265 — Blueprint manual sync reverts CLI drift (live, 2026-10-20)

The `render-examples/fastapi` Blueprint was connected again (`autoSync` then PATCHed to false). Its `fastapi-example` service went live in 223 s. Both were deleted afterwards (Blueprint 204 → 404, service 204 → 404). No new bug filed.

- Drift: `bex services update <id> --start-command 'uvicorn … --log-level debug'` saved the new start command. `POST /v1/blueprints/{id}/sync` → 200, and the start command was restored to the manifest's `uvicorn main:app --host 0.0.0.0 --port $PORT`. Render's contract: a sync overwrites Blueprint-managed fields changed outside the file.
- Observation, deliberate: reconnecting the same repo/branch/path revived the Blueprint deleted in sweep 1264 (same `blp-…` id; the old sync `bsr-…ig` from 19:26 is listed alongside the new ones). Disconnect is a soft detach and connect revives the row by design (w8/m37; `createdAt` was reset by w4/120, `store/blueprint_lifecycle_pg_test.go:567–572`). Not filed.
