# CLI sweep 1283 — free-form Blueprint names on the new pin, and what failed next (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `a3f66521f93d`, bex-canary, HEAD-built pinned CLI.

- w5/m150 is live: `Davender-1565/VisualSort`'s render.yaml (Render export header, `name: Visual_Sort`, static, `rootDir: ./`) now maps the name to `visual-sort`, so the name refusal from sweep 1267 is gone.
- The next refusal is `rootDir: ./` → `rootDirectory must be a relative path with no '..' components`, filed as [w8/165](../../165.md). The spelling table there comes from this sweep's validate probes. REST create with `rootDir: "./"` → the same 400.
- `POST /v1/blueprints` for that repo returned 400 but left Blueprint `blp-dbca9be03oos73cqinu0` (`status: error`, `autoSync: true`, failed sync `bsr-dbca9be03oos73cqinug`). Filed as [w8/166](../../166.md). The Blueprint was deleted (204 → 404); no service was created (`GET /v1/services?name=visual-sort` → `[]`).
- The full re-validation of the sweep 1267, 1270 and 1276 corpora on the new pin is running; results go in sweep 1284.
