# CLI sweep 1273 — a real three-resource Render Blueprint (Postgres, Key Value, Docker web) end to end (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, workspace `tian-personal` (bex-canary's free Postgres slot is held), human device-login grant, HEAD-built pinned CLI.

**Repo:** `D-Atharv/feature-flag-service` (213 KB, Go, distroless image). It defines:

- free Postgres 16 `ffs-postgres`, with no `ipAllowList`
- free Key Value `ffs-keyvalue`, with `ipAllowList: []` and `noeviction`
- free Docker web `ffs-api` with `healthCheckPath: /healthz`, `DATABASE_URL` from `fromDatabase … connectionString` and `REDIS_ADDR` from `fromService keyvalue … connectionString`

The repo's own `scripts/render-migrate.sh` runs migrations from outside using the External Database URL.

| Step | Result |
| --- | --- |
| `POST /v1/blueprints` (blueprint `blp-dbc61165u6js73eso45g`, autoSync PATCHed to false) | 201; resources `dpg-dbc61165u6js73eso46g`, `red-dbc611e5u6js73eso47g`, `srv-dbc611e5u6js73eso490` in 5 s |
| Postgres and Key Value | both `available`. Postgres `ipAllowList: []`, `public: false` (**w8/157**) |
| First deploy | built in about 3 min, then crash-looped `relation "api_keys" does not exist`. It reached `update_failed` at the 15-minute rollout budget (06:53:42 → 07:08:42), and the pod that first started cleanly at 07:09:12 was shut down at 07:09:23 by that failure, as on Render (a failed first deploy needs a new deploy) |
| `bex psql <dpg> -c 'select 1'` | `Error: IP address (<client-ip>) not in allow list for ffs-postgres` |
| `bex pg update <dpg> --ip-allow-list cidr=<v4>/32 --ip-allow-list cidr=<v6>/128 --confirm` | applied; `public: true` |
| Migrations through `bex psql -c` (goose `Up` sections of `001_init.sql`, `002_flag_audit.sql`) | The first attempt, about a minute after the allow-list update, failed over IPv6 (`SSL error: unexpected eof while reading`). Three `select` probes and the retried migration then succeeded. Tables: `api_keys, environments, flag_audit, flags` |
| `bex deploys create srv-dbc611e5u6js73eso490 --wait` | `live` in 2 m 54 s. `GET /` 200, `/healthz` 200, `/readyz` 200 `{"status":"ready"}`, and `/evaluate/…` without a key returns the app's 401 problem document. Logs show `redis connected addr=red-….svc:6379` |

The one transient IPv6 failure right after the allow-list change was not reproduced in four later connections, so it is not filed. The `fromDatabase`/`fromService` connection strings worked.

**Cleanup:** Blueprint 204 → 404; Postgres, Key Value and service each 204 → 404.

**Corpus side check (sweep 1270 files):** 13 Docker services set `rootDir` with `dockerfilePath`. In 12 of them the Dockerfile exists only under `rootDir`, not at the repo root. That supports w8/m128's rootDir-relative reading and is consistent with Render's monorepo guide.
