# Render's own templates, deployed unchanged on bex

Captured 2026-11-26 (UTC) for [w6/m644](../../.pm/w6/done/m644/README.md). Validation alone (w5/m143 and the weekly `TestRenderExamplesCorpus` job from w33/001) says whether bex accepts a `render-examples` Blueprint. This page says whether it runs: twenty of those Blueprints were deployed unchanged on production, from their own repositories, and checked the way their README or Render guide says to.

- **Production pin:** `2bd6e1f415d3` (`043827a8e8`, the newest `chore(deploy): pin platform images to …` on `main` during the run).
- **Where and how:** workspace `bex-canary`, as the QA user with a run-minted API key that was revoked afterwards. Each template ran through [`scripts/render-examples-live.sh`](../../scripts/render-examples-live.sh), which creates the Blueprint from `https://github.com/render-examples/<repo>` as Render's Deploy to Render button does (`POST /v1/blueprints` with `source.repo`, then approve the planned sync), waits for every resource's first deploy, runs the checks below, prints one line, and deletes what it created. Rerun any one with `bash scripts/render-examples-live.sh <repo>` given `BEX_API_URL` and an API key for the workspace in `BEX_API_TOKEN`.
- **Cleanup:** every Blueprint, resource and project a run created was deleted, and its by-id `GET` answered `404`.

## How the twenty were chosen

`bash scripts/render-examples-corpus.sh <dir>` fetched 185 `render.yaml` files on 2026-11-25. Counting the services and databases nested under `projects[].environments[]` as well as the top-level ones, 69 of them can be deployed without changing them: no `sync: false` variable (that asks for a value, usually a third-party key), no `type: workflow` service and no previews (both `DO_NOT_DO` non-goals). Across the 69, runtimes appear as docker 29, node 17, python 8, image 7, static 7, ruby 3 and go 2; service types as web 60, pserv 15, worker 6, Key Value 5 (`redis` 3, `keyvalue` 2) and cron 2; 24 files declare a database and 24 a disk. `POST /v1/blueprints/validate` on production accepted 68 of the 69. The one it refused, `pg_pinecone_render`, is in the sample because the weekly corpus guard accepts it (below).

The sample covers every runtime and every service type, Postgres, Key Value, disks and multi-service files, and prefers templates with a Render guide. One template per runtime and type is the floor, not the aim: the frameworks most often deployed (Django, Rails, Express, FastAPI) are each in it.

## The templates and their checks

A check is what the template's README or Render guide says to look for. `http` fetches the service's `onbex.co` URL and expects a 2xx answer containing the text, `post` does the same for a POST, `log` reads the service's app log through `GET /v1/logs`, `cron` triggers a run and expects `successful`, and `addr` reads a private service's internal address and expects the port its first deploy detected. Every check runs after each of the template's resources has reached `live` (a deploy) or `available` (a datastore). The script carries the same table.

A private service's guide checks it from a peer, by connecting to it over the private network. bex runs no one-off jobs (`POST /v1/services/{id}/jobs` answers that they are not supported), so the script cannot start that peer; `addr` stands in, and the deploy reaching `live` already means a port opened.

| # | Template | Runtime | Types | Plans | DB / disk | Guide | Check |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `express-hello-world` | node | web | free | — | [deploy-node-express-app](https://render.com/docs/deploy-node-express-app) | `http express-hello-world /` "Hello from Render!" |
| 2 | `fastapi` | python | web | free | — | [deploy-fastapi](https://render.com/docs/deploy-fastapi) | `http fastapi-example /` `{"message":"Hello World"}` |
| 3 | `go-web-server` | go | web | free | — | README | `http go /` "Hello from Render!" |
| 4 | `django` | python | web | free | Postgres (free) | [deploy-django](https://render.com/docs/deploy-django) | `http mysite /` "Hello Django on Render!" |
| 5 | `rails-7` | ruby | web | default | Postgres (default) | [deploy-rails](https://render.com/docs/deploy-rails) | `http myrailssite /` "Hello Rails on Render!" |
| 6 | `sidekiq` | ruby | web, worker, Key Value | default | — | [deploy-sidekiq-worker](https://render.com/docs/deploy-sidekiq-worker) | `http sinatra-web /` "Sinatra + Sidekiq Example"; `log sidekiq-worker` "Starting processing" |
| 7 | `celery` | python | web ×2, worker, Key Value | free, default, starter | — | [deploy-celery](https://render.com/docs/deploy-celery) | `http app /` "Asynchronous addition using Celery!"; `http flower /` "Flower"; `log celery-worker` "ready." |
| 8 | `ioredis` | node | worker, Key Value | default | — | [connecting-to-redis-with-ioredis](https://render.com/docs/connecting-to-redis-with-ioredis) | `log ioredis-worker` "Result for key animal: cat" |
| 9 | `webhook-playground` | node | web, cron, pre-deploy | standard | Postgres (basic-256mb) | README | `http webhook-playground /` "Webhook Playground"; `cron webhook-playground-cleanup` |
| 10 | `clickhouse` | docker | pserv | standard | disk 10 GB | [deploy-clickhouse](https://render.com/docs/deploy-clickhouse) | `addr clickhouse 8123` |
| 11 | `pg_pinecone_render` | image | pserv | 0.5c-512mb | disk 1 GB | README | `addr pg-pinecone 5432` |
| 12 | `n8n` | image | web | free | Postgres (free) | [deploy-n8n](https://render.com/docs/deploy-n8n) | `http n8n-service /healthz` "ok" |
| 13 | `umami` | image | web, Key Value, env group (in a project) | starter | Postgres (basic-256mb) | README | `http umami /api/heartbeat`; `post umami /api/auth/login` with the README's first login, expects "token" |
| 14 | `bun-docker` | docker | web | starter | — | README | `http bun /` "Welcome to Bun!" |
| 15 | `strapi-sqlite` | node | web | starter | disk 1 GB | [deploy-strapi](https://render.com/docs/deploy-strapi) | `http strapi /_health` |
| 16 | `redwoodjs` | node, static | web, static | default | Postgres (default) | [deploy-redwood](https://render.com/docs/deploy-redwood) | `http redwoodjs-web /`; `http redwoodjs-api /graphql/health` |
| 17 | `vuepress` | static | static | — | — | README | `http vuepress-example /` "Hello Vuepress" |
| 18 | `postgrest` | docker | web | free | Postgres (free) | README | `http postgrest /` "swagger" (the OpenAPI document) |
| 19 | `minio` | docker, image | web ×2 | default | disk | README | `http minio-server /minio/health/live`; `http minio-console /` "MinIO" |
| 20 | `wordpress` | docker | web, pserv (from `render-examples/mysql`) | standard | disk 20 GB ×2 | [deploy-wordpress](https://render.com/docs/deploy-wordpress) | `http wordpress /wp-admin/install.php` "WordPress" |

## Results

Run between 04:22 and 06:03 UTC. **12 of the 20 went live unchanged and passed their checks.** Four failed for a cause bex has to fix, each filed. Four failed for a cause that fails on Render too, with the evidence below.

Each template ran once to its outcome. A rerun replaced the first run where the failure was the run's own, not the template's. That happened in four cases:

- Three runs (`fastapi`, `go-web-server`, `rails-7`) broke when the script file changed under them mid-run. `run.sh` now runs a copy.
- `rails-7` went live, but its check fetched the database that shares the service's name. The script now looks up services only.
- `django`, `n8n` and `postgrest` were refused at the plan while another QA session's free Postgres was in `bex-canary`. Render also allows one free database per workspace.
- Seventeen creates of six templates were refused with `503` "GitHub is rate-limiting bex" between 04:51:53 and 05:11:41 UTC ([w6/185](../../.pm/w6/185.md)), and were retried until the hour's budget reset.

The `umami`, `django`, `celery`, `minio` and `wordpress` failures were rerun with `--keep` to read their logs, and those fixtures were then deleted by ID.

| # | Template | Outcome | Failing stage | Item or evidence |
| --- | --- | --- | --- | --- |
| 1 | `express-hello-world` | live | — | — |
| 2 | `fastapi` | live | — | — |
| 3 | `go-web-server` | live | — | — |
| 4 | `django` | failed, Render too | build | `migrate` dies on `psycopg2/_psycopg…so: undefined symbol: _PyInterpreterState_Get`: the file pins `psycopg2-binary==2.9.9`, which cannot run on Python 3.14. Render's default for a new service is 3.14.10 ([python-version](https://render.com/docs/python-version)), and the template's open PR #15 (by Render, 2026-03-24) is "Switching to psycopg v3 to be compatible with Python 3.14". |
| 5 | `rails-7` | live | — | — |
| 6 | `sidekiq` | failed, Render too | deploy | `sinatra-web` listens on `127.0.0.1:10000` and `[::1]:10000` only, and the deploy times out at the 15-minute budget. Sinatra 3.2 binds `localhost` unless `APP_ENV` or `RACK_ENV` names another environment (`sinatra/base.rb`), and Render's Ruby defaults set `RAILS_ENV` but neither of those ([environment-variables](https://render.com/docs/environment-variables)), as w9/133 found. The template's PR #15 reports a passing Render run on 2026-11-20; nothing in Render's documentation explains how, so this row stands on the documentation until someone can repeat that run on render.com. |
| 7 | `celery` | failed, Render too | deploy | `gunicorn==20.1.0` imports `pkg_resources`, which a Python 3.14 virtual environment does not have, and `celery==5.2.6` does not import on Python 3.13 or later either: the template's open issue #25 ("Celery version needs to be upgraded", 2025-10-19). |
| 8 | `ioredis` | live | — | — |
| 9 | `webhook-playground` | live | — | the pre-deploy migration ran, and a triggered cron run ended `successful` |
| 10 | `clickhouse` | live | — | internal address `clickhouse-<suffix>:8123` |
| 11 | `pg_pinecone_render` | failed, bex | plan | [w6/183](../../.pm/w6/blocked/183.md): `spec.startCommand: Too long: may not be more than 4096 bytes` for a 4,717-byte `dockerCommand` |
| 12 | `n8n` | live | — | — |
| 13 | `umami` | failed, bex | deploy | [w6/184](../../.pm/w6/184.md): the web service starts before its Postgres is available and exits (`Can't reach database server`); a manual deploy once it is available goes live and the README's first login works |
| 14 | `bun-docker` | live | — | — |
| 15 | `strapi-sqlite` | live | — | built on `NODE_VERSION=~16.13.0` |
| 16 | `redwoodjs` | failed, Render too | deploy | `redwoodjs-api` exits with `Unknown binaryTarget debian-openssl-3.0.x`: Redwood 0.33's Prisma (the repository's last commit is from 2021-06-10) predates OpenSSL 3, and Render's native runtimes run Debian 12 with OpenSSL 3 ([native-runtimes](https://render.com/docs/native-runtimes)). |
| 17 | `vuepress` | live | — | — |
| 18 | `postgrest` | live | — | — |
| 19 | `minio` | failed, bex | deploy | [w6/186](../../.pm/w6/186.md): the Chainguard image runs as user `65532` and cannot create `/data/.minio.sys` on its disk; the template's PR #21 ran the same file on a Render Starter service with a 10 GB disk |
| 20 | `wordpress` | failed, bex | deploy | [w6/186](../../.pm/w6/186.md): the image runs as `www-data`, and its entrypoint's `tar` into the disk at `/var/www/html` fails with `Cannot mkdir: Permission denied`; `mysql-wordpress` (from `render-examples/mysql`) went live |

**Seen across templates:** creating a Blueprint from a public repository reads GitHub anonymously, on a 60-an-hour budget that every workspace shares. After about 25 creates in 35 minutes, every create answered `503` for the next 20 minutes ([w6/185](../../.pm/w6/185.md)). One create at that moment answered `400` "bex's GitHub app is not authorized to read this repository" for a public repository that the next create read.

The deploy-detail lines above come from each failed release's `deploy_ended` `failureReason` or its first log line that reads like an error. The ids and `onbex.co` hostnames they name were hashed with `scripts/lib/public_log.py`.
