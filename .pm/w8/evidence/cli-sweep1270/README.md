# CLI sweep 1270 — 601 real `render.yaml` files: Render's own schema vs `bex blueprints validate` (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, HEAD-built pinned CLI, human device-login grant. Read-only validation; no fixtures.

## Method

1. GitHub code search (filename `render.yaml`) for `runtime`, `startCommand`, `fromDatabase`, `envVarGroups`, `healthCheckPath`, `staticPublishPath`, `fromService` and `buildFilter`. That gave 600 repo/path pairs and 601 distinct files, excluding sweep 1267's Render exports.
2. Each file was checked locally against Render's pinned Blueprint JSON schema (`lego/backend/internal/apps/schema/render.yaml.json`, jsonschema 4.26): 30 were not YAML, 353 failed Render's schema, and **218 passed**.
3. The 218 Render-schema-valid files went through `bex blueprints validate <file> -o json`, about 1.5 s apart.

## Results (218 Render-schema-valid files)

| Blocking categories | Files | Disposition |
| --- | --- | --- |
| valid | 63 | — |
| region only | 61 | w8/152 (decision pending) |
| `PORT` env var only | 19 (22 with any `PORT`) | deliberate, ADR018:137 / w2/m95 |
| workspace Postgres quota | 19 | QA workspace already holds its free database; environmental |
| name not a DNS label | 10 | deliberate, ADR020 / ADR049:356 |
| previews | 10 (+5 with region) | non-goal |
| YAML aliases | 2 | **filed [w8/156](../../156.md)** |
| other | 22 (+4 combined with the above) | see below |

"Other", reviewed one by one:

- `fromService.envVarKey: RENDER_EXTERNAL_URL` (3 files, the only blocker in each) was **filed as [w8/155](../../done/155.md)**. Across all 601 files, 26 reference `RENDER_*` defaults this way.
- Not filed:
  - `buildCommand: ""` with native Python (9 files, all `render.yaml.txt` templates).
  - Placeholder domains `YOUR_FRONTEND_DOMAIN`.
  - Disk `sizeGB` below Hetzner's 10 GB minimum (recorded).
  - `user: postgres` reserved.
  - Free worker (`background_worker requires a paid plan`, matching Render).
  - Autoscaling without a target percentage.
  - `autoDeploy: yes`: a YAML 1.1 boolean, which yaml.v3 reads as a string.
  - Invented `fromService.transform`/`prefix` keys.
  - `sync: false` inside an env group, which Render also forbids ("You can't define an environment variable with `sync: false` in an environment group").
  - Repos or databases outside the workspace.

The data behind w8/152: 61 of the 218 Render-schema-valid files would validate on bex if `region` were accepted, so validation would pass for 124 of 218 instead of 63.

## Minimal reproductions

- `default-ref.yaml`: two free image web services, the second copying the first's `RENDER_EXTERNAL_URL` and its own `RENDER_EXTERNAL_HOSTNAME`. It has 0 Render-schema errors; bex refuses it with `… which has no such plainly-defined variable` (line 15).
- `alias.yaml`: two free image web services sharing `envVars: &shared` / `*shared`. It has 0 Render-schema errors; bex refuses it with `YAML aliases are not supported in Blueprints` (line 17, column 14).
