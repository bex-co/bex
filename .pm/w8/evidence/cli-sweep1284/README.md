# CLI sweep 1284 — corpus re-validation on pin `a3f66521f93d` (read-only, 2026-10-21)

The three Blueprint corpora were re-run through `bex blueprints validate` on the new pin and compared with their earlier results.

| Corpus | Files | Valid before → now | Newly valid | Regressions |
| --- | --- | --- | --- | --- |
| Render exports (sweep 1267) | 100 | 4 → 7 | 3 | 0 |
| Render-schema-valid public files (sweep 1270) | 218 | 63 → 79 | 16 | 0 |
| Official `render-examples` (sweep 1276) | 150 | 39 → 52 | 13 | 0 |

What changed:

- The `fromService.envVarKey: RENDER_*` refusals (w8/155) are gone. The four official examples now fail only on the QA workspace's Postgres quota.
- Half of the 22 `PORT`-refused files now validate.
- w5/m150 lets free-form names through. One of those files now fails on `rootDir: ./`, filed as w8/165.

Remaining non-decision refusals:

- `sizeGB` below 10: 11 official examples and 10 public files. This is the ADR082 Hetzner floor, deliberate (ADR082:121). Render's documented minimum is 1.
- Native Python with `buildCommand: ""` (9 `.txt` templates).
- `fromDatabase` pointing at a database absent from the workspace (8).
- Autoscaling without a target percentage (3).
- `sync: false` in an env group (1), which Render also forbids.
- `pro-4gb` (1, w8/160).
- `dockerFilePath` with a capital F (1, `render-examples/twingate-example`). Render's schema does not define it either.
- `cgr.dev` registry (1, `render-examples/minio`) → **filed as [w8/167](../../done/167.md)**, with a six-registry probe table.

w8/164 was upgraded in this sweep. Render's documented Docker Command form, `/bin/sh -c <unquoted commands>`, runs on bex with the first command's arguments silently dropped, proven live with `srv-dbcagfu03oos73cqir60` (deleted, 404). See the addendum in 164.
