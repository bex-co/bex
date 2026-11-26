# Render's own templates, deployed unchanged on bex

Captured 2026-11-26 (UTC) for [w6/m644](../../.pm/w6/done/m644/README.md), which ran twenty templates, and [w6/m653](../../.pm/w6/done/m653/README.md), which ran the other 49. Validation alone (w5/m143 and the weekly `TestRenderExamplesCorpus` job from w33/001) says whether bex accepts a `render-examples` Blueprint. This page says whether it runs: every one of the 69 Blueprints that need no values was deployed unchanged on production, from its own repository, and checked the way its README or Render guide says to.

- **Production pins:** `2bd6e1f415d3` for rows 1–20 (`043827a8e8`) and `9b91284a4a2e` for rows 21–69 and the `clickhouse` rerun (`c767c1bbd9`). Each is the newest `chore(deploy): pin platform images to …` on `main` during its run.
- **Where and how:** workspace `bex-canary`, as the QA user with a run-minted API key that was revoked afterwards. Each template ran through [`scripts/render-examples-live.sh`](../../scripts/render-examples-live.sh). The script creates the Blueprint from `https://github.com/render-examples/<repo>` as Render's Deploy to Render button does (`POST /v1/blueprints` with `source.repo`, then it approves the planned sync). It waits for every resource's first deploy, runs the checks below, prints one line, and deletes what it created. Rerun any one with `bash scripts/render-examples-live.sh <repo>` given `BEX_API_URL` and an API key for the workspace in `BEX_API_TOKEN`.
- **Cleanup:** every Blueprint, resource, peer and project a run created was deleted, and its by-id `GET` answered `404`.

## How the templates were chosen

`bash scripts/render-examples-corpus.sh <dir>` fetched 185 `render.yaml` files on 2026-11-25. Counting the services and databases nested under `projects[].environments[]` as well as the top-level ones, 69 of them can be deployed without changing them: no `sync: false` variable (that asks for a value, usually a third-party key), no `type: workflow` service and no previews (both `DO_NOT_DO` non-goals). Across the 69, runtimes appear as docker 29, node 17, python 8, image 7, static 7, ruby 3 and go 2; service types as web 60, pserv 15, worker 6, Key Value 5 (`redis` 3, `keyvalue` 2) and cron 2; 24 files declare a database and 24 a disk. `POST /v1/blueprints/validate` on production accepted 68 of the 69. The one it refused, `pg_pinecone_render`, is in the sample because the weekly corpus guard accepts it (below).

w6/m644's twenty cover every runtime and every service type, Postgres, Key Value, disks and multi-service files, and prefer templates with a Render guide. One template per runtime and type was the floor, not the aim: the frameworks most often deployed (Django, Rails, Express, FastAPI) are each in it. w6/m653 fetched the corpus again on 2026-11-26 and applied the same rule. There were still 185 files, and the same 69 qualify, so rows 21–69 are the 49 that w6/m644 did not run, in alphabetical order.

## The templates and their checks

A check is what the template's README or Render guide says to look for:

- `http` fetches the service's `onbex.co` URL and expects a 2xx answer containing the text.
- `post` does the same for a POST.
- `log` reads the service's app log through `GET /v1/logs`.
- `cron` triggers a run and expects `successful`.
- `peer` checks a private service from another service, as its guide does.

Every check runs after each of the template's resources has reached `live` (a deploy) or `available` (a datastore). The script carries the same table.

A private service's guide checks it from a peer, by connecting to it over the private network. bex runs no one-off jobs (`POST /v1/services/{id}/jobs` answers that they are not supported), so the script creates the peer as an ordinary service: a background worker named `render-examples-live-peer-<repo>`, from the public `busybox` image, in the same workspace and in the private service's environment. Its start command connects to each private service the checks name, at `<internal host>:<port>`, every 10 seconds. It logs the printable bytes of each answer, and the check reads that line through `GET /v1/logs`. What the peer sends depends on the protocol, which defaults by port:

- MySQL (3306): nothing, because the server greets first with its version and authentication plugin.
- Postgres (5432): an SSLRequest, which the server answers `N` or `S`.
- Redis (6379): `PING`.
- MongoDB (27017): a legacy `isMaster`.
- Any other port: `GET <path> HTTP/1.0`, so the answer is the HTTP status line, headers and body.

`redis:10000` names the protocol when a template moves its port. The peer is deleted with everything else. `addr` (the internal address names the port the first deploy detected) remains only for a port the peer cannot open in one command: `temporal-frontend`'s gRPC.

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
| 10 | `clickhouse` | docker | pserv | standard | disk 10 GB | [deploy-clickhouse](https://render.com/docs/deploy-clickhouse) | `peer clickhouse 8123` "Ok." |
| 11 | `pg_pinecone_render` | image | pserv | 0.5c-512mb | disk 1 GB | README | `peer pg-pinecone 5432` |
| 12 | `n8n` | image | web | free | Postgres (free) | [deploy-n8n](https://render.com/docs/deploy-n8n) | `http n8n-service /healthz` "ok" |
| 13 | `umami` | image | web, Key Value, env group (in a project) | starter | Postgres (basic-256mb) | README | `http umami /api/heartbeat`; `post umami /api/auth/login` with the README's first login, expects "token" |
| 14 | `bun-docker` | docker | web | starter | — | README | `http bun /` "Welcome to Bun!" |
| 15 | `strapi-sqlite` | node | web | starter | disk 1 GB | [deploy-strapi](https://render.com/docs/deploy-strapi) | `http strapi /_health` |
| 16 | `redwoodjs` | node, static | web, static | default | Postgres (default) | [deploy-redwood](https://render.com/docs/deploy-redwood) | `http redwoodjs-web /`; `http redwoodjs-api /graphql/health` |
| 17 | `vuepress` | static | static | — | — | README | `http vuepress-example /` "Hello Vuepress" |
| 18 | `postgrest` | docker | web | free | Postgres (free) | README | `http postgrest /` "swagger" (the OpenAPI document) |
| 19 | `minio` | docker, image | web ×2 | default | disk | README | `http minio-server /minio/health/live`; `http minio-console /` "MinIO" |
| 20 | `wordpress` | docker | web, pserv (from `render-examples/mysql`) | standard | disk 20 GB ×2 | [deploy-wordpress](https://render.com/docs/deploy-wordpress) | `http wordpress /wp-admin/install.php` "WordPress" |
| 21 | `ackee` | docker | web, pserv (from `render-examples/mongodb`) | default | disk 10 GB | [deploy-ackee](https://render.com/docs/deploy-ackee) | `http ackee /` "&lt;title&gt;Ackee&lt;/title&gt;"; `peer ackee-mongodb 27017` "ismaster" |
| 22 | `adminer` | docker | web | free | — | [deploy-adminer](https://render.com/docs/deploy-adminer) | `http adminer /` "Login - Adminer" |
| 23 | `astro-ssr` | node | web | free | — | [deploy-astro](https://render.com/docs/deploy-astro) | `http astro-ssr /` "It works."; `http astro-ssr /api/hello?name=Astro` "Hello, Astro!" |
| 24 | `blitzjs` | node | web | starter | Postgres (basic-256mb) | [deploy-blitz](https://render.com/docs/deploy-blitz) | `http blitzapp /` "Your app is ready, including user sign-up and log-in." |
| 25 | `crawl4ai-url-to-markdown` | docker | web | free | — | README | `http url-to-markdown /health` `"status":"healthy"`; `http url-to-markdown /` "&lt;title&gt;URL to Markdown&lt;/title&gt;"; `post url-to-markdown /crawl` the README's `{"url":"https://example.com"}`, expects "Example Domain" |
| 26 | `elasticsearch` | docker | pserv | standard | disk 10 GB | [deploy-elasticsearch](https://render.com/docs/deploy-elasticsearch) | `peer elasticsearch 9200` "You Know, for Search" |
| 27 | `elysiajs-hello-world` | node (Bun) | web | free | — | [deploy-elysiajs](https://render.com/docs/deploy-elysiajs) | `http elysiajs-hello-world /` "Hello Elysia" |
| 28 | `fastapi-uv` | python (uv) | web | starter | — | README | `http fastapi-uv /` `{"message":"Hello World"}`; `http fastapi-uv /?name=Render` `{"message":"Hello Render"}` |
| 29 | `fastify-hello-world` | node | web | free | — | [deploy-node-fastify-app](https://render.com/docs/deploy-node-fastify-app) | `http fastify-hello-world-example /` "Hello from Render using Fastify!" |
| 30 | `ghost` | docker | web, pserv (from `render-examples/mysql`) | default, standard | disk 10 GB ×2 | [deploy-ghost](https://render.com/docs/deploy-ghost) | `http ghost /` `<meta name="generator" content="Ghost 5.14"`; `peer mysql 3306` "8.0.24" |
| 31 | `gitpod-openvscode-server` | docker | web | default | disk 1 GB | README | `http gitpod-openvscode-server-example /` "vscode-workbench-web-configuration" |
| 32 | `hasura-graphql-metadata-db-only` | docker | web | default | Postgres (default) | [deploy-hasura-graphql](https://render.com/docs/deploy-hasura-graphql) | `http hasura /healthz` "OK"; `http hasura /v1/version` "v2.0.2" |
| 33 | `hasura-graphql` | docker | web | default | Postgres (default) | [deploy-hasura-graphql](https://render.com/docs/deploy-hasura-graphql) | `http hasura /healthz` "OK" |
| 34 | `langfuse4-template` | image | web ×2, worker, pserv, Key Value (in a project, network-isolated environment) | standard ×2, pro, starter ×2 | Postgres (basic-1gb), disk 10 GB ×2 | README | `http langfuse /api/public/health` `"status":"OK"`; `http langfuse-minio /minio/health/live`; `log langfuse-worker` "Listening: http://"; `peer langfuse-clickhouse 8123/ping` "Ok." |
| 35 | `matomo` | docker | web, pserv (from `render-examples/mysql`) | standard ×2 | disk 20 GB ×2 | [deploy-matomo](https://render.com/docs/deploy-matomo) | `http matomo /` "Matomo is libre software"; `peer mysql-matomo 3306` "mysql_native_password" |
| 36 | `mattermost` | docker | web, pserv | starter, default | Postgres (basic-256mb), disk 1 GB ×2 | [deploy-mattermost](https://render.com/docs/deploy-mattermost) | `http mattermost /api/v4/system/ping` `"status":"OK"`; `peer mattermost-minio 9000/minio/health/live` "HTTP/1.1 200 OK" |
| 37 | `mcp-server-python` | python | web | free | — | README | `http my-mcp-server /health` `{"status":"ok"}` |
| 38 | `mcp-server-typescript` | node | web | free | — | README | `http my-mcp-server /health` `{"status":"ok"}` |
| 39 | `metabase` | docker | web | standard | Postgres (default) | [deploy-metabase](https://render.com/docs/deploy-metabase) | `http metabase /api/health` `{"status":"ok"}` |
| 40 | `mongodb` | docker | pserv | default | disk 10 GB | [deploy-mongodb](https://render.com/docs/deploy-mongodb) | `peer mongodb 27017` "maxWireVersion" |
| 41 | `multi-environments-blueprint` | node, python | web ×2, cron, env group ×2 (in a project, two environments) | standard, starter, default | Postgres (pro-4gb, basic-1gb) | README | `http portal-web-env-demo /` `"environment":"production"`; `http portal-web-env-demo-staging /` `"environment":"staging"`; `cron nightly-jobs`; `log nightly-jobs` "Nightly tasks completed successfully" |
| 42 | `mysql` | docker | pserv, env group | standard | disk 10 GB | [deploy-mysql](https://render.com/docs/deploy-mysql) | `peer mysql 3306` "mysql_native_password" |
| 43 | `nextjs-hello-world` | node | web | starter | — | [deploy-nextjs-app](https://render.com/docs/deploy-nextjs-app) | `http next-js /` "Hello World."; `http next-js /about` "About" |
| 44 | `nuxtjs-hello-world` | node | web | starter | — | [deploy-nuxtjs](https://render.com/docs/deploy-nuxtjs) | `http nuxt-js /` "Welcome!" |
| 45 | `paradedb` | image | pserv | standard | disk 10 GB | [deploy-paradedb](https://render.com/docs/deploy-paradedb) | `peer paradedb 5432` "N" (Postgres's answer to an SSLRequest when SSL is off) |
| 46 | `patient-api` | docker | web | starter | Postgres (basic-256mb) | README | `http patient-api /` `{"status":"ok"}` |
| 47 | `pocketbase` | docker | web (in a project) | starter | disk 5 GB | README | `http pocketbase /api/health` "API is healthy."; `log pocketbase` "/\_/#/pbinstall/" (the README's first-run link) |
| 48 | `postgresql-missing-trigger-index` | — | — (one database) | — | Postgres (basic-1gb) | README | the database reaches `available` |
| 49 | `prometheus` | docker | web | starter | disk 1 GB | [deploy-prometheus](https://render.com/docs/deploy-prometheus) | `http prometheus /graph` "Prometheus Time Series Collection and Processing Server"; `http prometheus /-/healthy` "Prometheus Server is Healthy."; `http prometheus /api/v1/query?query=prometheus_http_requests_total` `"job":"prometheus"` |
| 50 | `python-memray-profiling` | python (uv) | web | starter | disk 10 GB | README | `http render-memray-example /` `"service":"render-memray-example"` |
| 51 | `python-poetry` | python (Poetry) | web | starter | — | README | `http flask-poetry /` "Hello, World!" |
| 52 | `questdb` | docker | web | default | disk 20 GB | README | `http questdb /index.html` "QuestDB · Console"; `http questdb /exec?query=SELECT%201` `"dataset":[` |
| 53 | `rabbitmq` | docker | web | default | disk 10 GB | [deploy-rabbitmq](https://render.com/docs/deploy-rabbitmq) | `http rabbitmq /` "RabbitMQ Management" |
| 54 | `rails-6` | ruby | web | default | Postgres (default) | README | `http myrailssite /` "Hello Rails on Render!" |
| 55 | `redash` | docker | web, worker ×2, pserv (from `render-examples/redis`), env group ×2 | standard ×2, default ×2 | Postgres (default), disk 10 GB | [deploy-redash](https://render.com/docs/deploy-redash) | `http redash /ping` "PONG."; `log redash-scheduler` "Starting scheduler and 1 workers for queues: celery"; `log redash-worker` "Starting 2 workers for queues: queries,scheduled_queries,schemas" (`redash/redash:latest` is Redash 8); `peer redash-redis redis:10000` "PONG" |
| 56 | `redis` | docker | pserv | default | disk 10 GB | README | `peer redis redis:10000` "PONG" |
| 57 | `remix-postgres` | node | web | free | Postgres (free) | [deploy-remix](https://render.com/docs/deploy-remix) | `http remix /healthcheck` "OK"; `http remix /` "Indie Stack" |
| 58 | `render-tanstack-demo` | static | static | — | — | README | `http tanstack-playground /` "TanStack Playground"; `http tanstack-playground /deep/link` "TanStack Playground" (the SPA rewrite) |
| 59 | `retool` | docker | web | standard | Postgres (default) | [deploy-retool](https://render.com/docs/deploy-retool) | `http retool /api/checkHealth` `"version"` |
| 60 | `shynet` | docker | web | default | Postgres (default) | [deploy-shynet](https://render.com/docs/deploy-shynet) | `http shynet /healthz/?format=json` `"DatabaseBackend": "working"` |
| 61 | `strapi-postgres` | node | web | starter | Postgres (basic-256mb), disk 1 GB | [deploy-strapi](https://render.com/docs/deploy-strapi) | `http strapi /` "The server is running successfully."; `http strapi /_health` |
| 62 | `svelte` | static | static | — | — | [deploy-svelte](https://render.com/docs/deploy-svelte) | `http svelte-static /` "Svelte app"; `http svelte-static /build/bundle.js` |
| 63 | `sveltekit-static` | static | static | — | — | [deploy-sveltekit](https://render.com/docs/deploy-sveltekit) | `http sveltekit-static /` `id="svelte"`; `http sveltekit-static /about` |
| 64 | `sveltekit` | node | web | default | — | [deploy-sveltekit](https://render.com/docs/deploy-sveltekit) | `http sveltekit /` "SvelteKit app"; `http sveltekit /sverdle` "Sverdle" |
| 65 | `tanstack-start` | node | web | free | — | README | `http tanstack-start /health` `{"status":"ok"}`; `http tanstack-start /` "TanStack Start on Render"; `http tanstack-start /about` "How it works" |
| 66 | `temporal` | docker, go | pserv ×6, web, worker (from `render-examples/sample-temporal-app`), env group | Starter ×8 | Postgres (basic-256mb), disk 10 GB | [deploy-temporal](https://render.com/docs/deploy-temporal) | `http app-workflow-trigger /` "OK!"; `peer temporal-ui 8088` "200 OK"; `peer temporal-elasticsearch 9200` "missing authentication credentials"; `addr temporal-frontend 7233` (gRPC, which the peer cannot open); `http app-workflow-trigger /trigger-workflow` "is processing."; `log app-worker` "Depositing $54.990002 into account 002-002" |
| 67 | `vibe-code-demo-apps` | static | static ×11 (each in its own project) | — | — | README | `http <app> /` for each of the 11, expecting its `<title>` ("Stillwood", "Coastal-Modern Interiors", … "Synthwave Collective") |
| 68 | `wikijs` | image | web | standard | Postgres (basic-256mb) | README | `http wikijs /` "Wiki.js Setup" |
| 69 | `wrapped-for-linkedin` | static, docker | static, web (in a project) | default, standard | — | README | `http wrappedforlinkedin /` "Wrapped for LinkedIn"; `http wrappedforlinkedin /deep/link` "Wrapped for LinkedIn"; `http wrapped-for-linkedin-screenshots /healthz` "ok" |

## Results

### Rows 1–20 (w6/m644)

Run between 04:22 and 06:03 UTC on pin `2bd6e1f415d3`. **12 of the 20 went live unchanged and passed their checks.** Four failed for a cause bex has to fix, each filed. Four failed for a cause that fails on Render too, with the evidence below.

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

### Rows 21–69 and the `clickhouse` rerun (w6/m653)

Run between 09:17 and 13:48 UTC on pin `9b91284a4a2e`. **34 of the 49 went live unchanged and passed their checks, so 46 of the 69 run on bex.** Fifteen failed. Five failed for a cause bex has to fix, each with its item, and ten for a cause that fails on Render too, with the evidence below.

- **Pacing:** w6/185 was still open, so creates were paced at no more than 18 an hour (two or three runs at a time, at least 150 seconds apart), and the last seven at no more than 8 an hour. GitHub still refused some: three creates answered `400` "bex's GitHub app is not authorized to read this repository" for a public repository (`matomo` at 10:03, `render-tanstack-demo` at 12:09 and `retool` at 12:15). That is w6/185's misread secondary limit. Each was run again later.
- **The peer:** `clickhouse` was run again with `peer clickhouse 8123 Ok.` and went live, and its peer was deleted with it. A second run added `peer clickhouse 9999`, a port nothing listens on, and failed at `check` with "the peer never read an answer from clickhouse-…:9999; its last line: peer clickhouse-…:9999 got no answer". The peer check passed for every private service whose template reached the check stage: MySQL (`mysql`, and `ghost`'s), MongoDB, Redis on port 10000 (`redis`, and `redash`'s), ParadeDB's Postgres and ClickHouse.
- **Reruns for the run's own faults:**
  - `remix-postgres` was refused at the plan while another QA session's free Postgres was in `bex-canary`, and was run again once it had gone.
  - `redash`'s first run looked for the Redash 10 log lines "Starting RQ scheduler...", which `redash/redash:latest` (still Redash 8, on Celery) never prints. The table now has Redash 8's lines.
  - `wrapped-for-linkedin`'s first run could not find `wrapped-for-linkedin-screenshots`, because the Blueprint lists it under a shortened slug ([w6/190](../../.pm/w6/190.md)). The script now also matches a service's own name.
- **Failures kept for triage:** every failed run kept its resources (`--keep`), and they were deleted by ID once their logs were read.

| # | Template | Outcome | Failing stage | Item or evidence |
| --- | --- | --- | --- | --- |
| 21 | `ackee` | failed, Render too | deploy | `electerious/ackee:latest` (3.7.1, 2026-10-23) ships MongoDB's Node driver 7.7, which refuses the template's `mongo:4.2`: `MongooseServerSelectionError: Server at ackee-mongodb:27017 reports maximum wire version 8, but this version of the Node.js Driver requires at least 9 (MongoDB 4.4)`. The driver dropped MongoDB 4.2 in v7.6.0 (2026-08-21), and render-examples/mongodb#3 (open, 2025-12-29) says the image "is very out of date". |
| 22 | `adminer` | live | — | — |
| 23 | `astro-ssr` | live | — | — |
| 24 | `blitzjs` | failed, Render too | build | On bex the build stops at `blitz: command not found` (exit 127): bex does not put `node_modules/.bin` on `PATH`, which Render's `deploy-blitz` guide relies on ([w6/187](../../.pm/w6/187.md), to confirm against Render). With the binary on `PATH`, the same build on `node:24.22.1-bookworm` (Render's default Node on Debian 12) stops at `blitz prisma generate` with `Unknown binaryTarget debian-openssl-3.0.x`: Prisma 2.27 predates OpenSSL 3, as with `redwoodjs` (row 16). |
| 25 | `crawl4ai-url-to-markdown` | live | — | the README's `POST /crawl` of `https://example.com` answered "Example Domain" |
| 26 | `elasticsearch` | failed, bex | deploy | [w6/186](../../.pm/w6/186.md): the image runs as `1000:0` and cannot write its disk: `AccessDeniedException[/usr/share/elasticsearch/data/nodes]` |
| 27 | `elysiajs-hello-world` | live | — | — |
| 28 | `fastapi-uv` | live | — | — |
| 29 | `fastify-hello-world` | live | — | — |
| 30 | `ghost` | live | — | its MySQL (from `render-examples/mysql`) answered the peer with its 8.0.24 greeting |
| 31 | `gitpod-openvscode-server` | failed, Render too | deploy | The template's Dockerfile switches to `USER vscode-server`, which `gitpod/openvscode-server:latest` no longer has, so the container never starts. Locally, `docker run` of the built image answers `unable to find user vscode-server: no matching entries in passwd file`. bex reported `rollout did not become healthy within the progress deadline`. The template's issue #2 "Not working" has been open since 2022-07-24. |
| 32 | `hasura-graphql-metadata-db-only` | live | — | — |
| 33 | `hasura-graphql` | live | — | — |
| 34 | `langfuse4-template` | failed, bex | deploy | [w6/184](../../.pm/w6/184.md): the `langfuse` image service started at 11:12:32 while its `basic-1gb` Postgres was still provisioning, and its migrations exited with `P1001: Can't reach database server`; the deploy ended `update_failed` 43 seconds in |
| 35 | `matomo` | failed, Render too | deploy | The Blueprint's `healthCheckPath: /matomo.php` answers `503` "As Matomo is not installed yet, the Tracking API cannot proceed." on a fresh `matomo:apache` (Matomo 6.0.0, 2026-11-23), reproduced locally. So the first deploy's health check cannot pass until someone runs the installer. Matomo 5.5 answered `200` there. |
| 36 | `mattermost` | failed, Render too | build | `mattermost-minio` is `FROM minio/minio`, which Docker Hub no longer serves: the repository page answers `404` and an anonymous pull answers `pull access denied, repository does not exist` (2026-11-26) |
| 37 | `mcp-server-python` | failed, Render too | deploy | `requirements.txt` does not pin `mcp`, so pip installs `mcp` 2.3.0 (2026-10-02) on any platform. Its `from mcp.server.fastmcp import FastMCP` then raises `ModuleNotFoundError: … This is mcp 2.x, where FastMCP was renamed to MCPServer`. |
| 38 | `mcp-server-typescript` | live | — | — |
| 39 | `metabase` | live | — | — |
| 40 | `mongodb` | live | — | the peer's `isMaster` read `maxWireVersion` |
| 41 | `multi-environments-blueprint` | live | — | both environments answered with their `NODE_ENV`, and a triggered `nightly-jobs` run ended `successful` and logged its line |
| 42 | `mysql` | live | — | the peer read the `mysql_native_password` greeting |
| 43 | `nextjs-hello-world` | live | — | — |
| 44 | `nuxtjs-hello-world` | failed, Render too | check | `package.json` asks for `nuxt: latest` with no lockfile, so the build gets Nuxt 4.7.0 (2026-11-23) for Nuxt 2 code. Every page answers `500` with `ReferenceError: __VUE_OPTIONS_API__ is not defined`, reproduced locally on `node:24.22.1-bookworm`. |
| 45 | `paradedb` | live | — | the peer's SSLRequest read `N` |
| 46 | `patient-api` | live | — | its Docker build ran `prisma migrate dev` against the database |
| 47 | `pocketbase` | live | — | the app log showed the README's first-run `/_/#/pbinstall/` link |
| 48 | `postgresql-missing-trigger-index` | live | — | the database reached `available` |
| 49 | `prometheus` | failed, bex | deploy | [w6/186](../../.pm/w6/186.md): the image runs as `nobody` and cannot create `/var/data/prometheus` on its disk (`Failed to create directory for logging active queries`) |
| 50 | `python-memray-profiling` | live | — | — |
| 51 | `python-poetry` | live | — | — |
| 52 | `questdb` | failed, Render too | check | The service names no plan, so it runs on Starter (512 MB), Render's default for a Blueprint service. There `questdb/questdb:latest` (10.2.0) exits 55 at boot with `OutOfMemoryError: Java heap space`, also locally under `docker run -m 512m`. The deploy went live on its open port 9003 before the first crash, and the edge then answered `503`. |
| 53 | `rabbitmq` | failed, bex | deploy | [w6/186](../../.pm/w6/186.md): the image runs as `rabbitmq` and cannot write its disk at `/var/lib/rabbitmq/mnesia` ("Ra could not create its data directory") |
| 54 | `rails-6` | live | — | — |
| 55 | `redash` | live | — | the peer's `PING` to `redash-redis:10000` read `PONG`. The readers' first deploys ran with an empty `REDIS_HOSTPORT` until the port was detected ([w6/188](../../.pm/w6/188.md)). The web service and workers still need the README's `render-redash create_db` step. |
| 56 | `redis` | live | — | the peer's `PING` to port 10000 read `PONG` |
| 57 | `remix-postgres` | failed, Render too | deploy | `Unknown binaryTarget debian-openssl-3.0.x`: the lockfile's Prisma 3.11.1 predates OpenSSL 3 support (3.13.0), as with `redwoodjs` (row 16) |
| 58 | `render-tanstack-demo` | live | — | the SPA rewrite served `/deep/link` |
| 59 | `retool` | failed, Render too | check | `tryretool/backend:latest` exits at boot with `Failed to start up due to configuration issues. Errors: BASE_DOMAIN environment variable is required for security reasons`, and the Blueprint sets no `BASE_DOMAIN`. Its Render guide also needs a license key, which the file ships as `EXPIRED-LICENSE-KEY-TRIAL`. The deploy went live on an open port before the first exit, and the edge then answered `503`. |
| 60 | `shynet` | live | — | — |
| 61 | `strapi-postgres` | live | — | — |
| 62 | `svelte` | live | — | — |
| 63 | `sveltekit-static` | live | — | — |
| 64 | `sveltekit` | live | — | — |
| 65 | `tanstack-start` | live | — | — |
| 66 | `temporal` | failed, bex | deploy | [w6/189](../../.pm/w6/189.md): `temporal-worker` started before `temporal-frontend`'s auto-setup had created the schema (`relation "schema_version" does not exist`). Render orders it with a `fromService` variable whose comment says "The worker service needs to wait for the history service to be up first". `temporal-elasticsearch` then failed on its disk too ([w6/186](../../.pm/w6/186.md)), while `temporal-ui`, `temporal-history` and `temporal-matching` went live. |
| 67 | `vibe-code-demo-apps` | live | — | all 11 static sites, each in its own project |
| 68 | `wikijs` | live | — | — |
| 69 | `wrapped-for-linkedin` | live | — | — |

**Seen across templates:**

- All ten Render-too failures are templates that went stale: images whose `latest` tag moved on (`ackee`, `gitpod-openvscode-server`, `matomo`, `questdb`, `retool`), an image Docker Hub stopped serving (`mattermost`), unpinned dependencies that now resolve to new majors (`mcp-server-python`, `nuxtjs-hello-world`), and a Prisma older than OpenSSL 3 (`blitzjs`, `remix-postgres`).
- The five bex failures have three causes, all filed before this run except one. A non-root image cannot write its disk (w6/186: `elasticsearch`, `prometheus`, `rabbitmq`, and `temporal-elasticsearch`). An image service starts before its database is available (w6/184: `langfuse4-template`). A service starts before another service it reads is up (w6/189: `temporal`).
- Creating a Blueprint from a public repository still reads GitHub anonymously ([w6/185](../../.pm/w6/185.md)). At no more than 18 creates an hour, no create answered `503`, but three answered `400` "not authorized" for a public repository.

The deploy-detail lines above come from each failed release's `deploy_ended` `failureReason` or its first log line that reads like an error. The ids and `onbex.co` hostnames they name were hashed with `scripts/lib/public_log.py`.
