# CLI sweeps 1300–1301 (2026-10-22)

Production `https://api.bex.co/v1/`, deployed pin `51e6dc76c8fe` (pin commit `368424106`, API rollout observed 01:48:57Z), workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device-login grant, HEAD-built pinned CLI (Render CLI v2.27.0, `a764810a7682`). Every fixture was ledgered before creation and deleted by ID (204 → 404).

## Sweep 1300 — real-world MCP server (render-examples/mcp-server-python)

`bex services create --type web_service --runtime python --repo https://github.com/render-examples/mcp-server-python --plan free --build-command 'pip install -r requirements.txt' --start-command 'python server.py' --health-check-path /health --env-var MCP_API_TOKEN=<redacted> --env-var PORT=10000` (`srv-dbcmn4a4bvhs73f7164g`).

- First deploy crash-looped: the repo's unpinned `mcp` now resolves to 2.x, which removed `mcp.server.fastmcp`. **Upstream repo defect**; it fails on Render the same way. bex's stall reason named the crash and pointed at the logs, and the deploy failed at Render's 15-minute start window.
- After `services update --build-command 'pip install -r requirements.txt "mcp<2"'` plus `deploys create`, the service went live. `GET /health` → 200; `POST /mcp` without a token → 401 JSON-RPC error; with the bearer, `initialize` and `tools/call hello` → 200 SSE. The example enables DNS-rebinding protection keyed on `RENDER_EXTERNAL_HOSTNAME`, so bex's injected hostname matches the Host the edge forwards. No finding.
- Overlapping deploys: the update's own deploy was superseded by the manual one, which waited for the in-progress (crash-looping) deploy to end. This matches Render's "Wait" policy (default for workspaces created on or after 2025-07-14). No finding.

Checked, not filed:

- An unpinned native Python service builds on 3.13 while Render's creation-date default is 3.14.8. This is recorded in ADR060 ("Python's default stays 3.13").
- `POST /v1/services` refuses `serviceDetails.port` (`unknown field "port"`); create accepts only the top-level `port`, while PATCH accepts both and reads report `serviceDetails.port`. It is a bex-only extension, Render clients never send a port, and the top-level spelling works.
- Waking the pre-existing sleeping `hello-go` with one GET answered 503, then 200 (`service_woken`). ADR007 records the bounded response as a deliberate divergence from Render's held request.
- Reconnecting a repo whose Blueprint was deleted returns the old Blueprint ID (`blp-dbca9be03oos73cqinu0`, deleted in sweep 1283, reissued here). This is the deliberate disconnected-row revival of w8/m37 and w4/120 (`store/blueprint_lifecycle.go` `AdmitBlueprintCreate`).

## Sweep 1301 — live acceptance of w8 fixes

| Item | Probe | Result |
| --- | --- | --- |
| [w8/153](../../done/153.md) | `bex services create --type static_site --repo https://github.com/mdn/beginner-html-site` with no `--publish-directory` | `publishPath: "public"`. The first deploy fails: `the publish directory "public" does not exist in the repository`. After `--build-command 'mkdir -p public && cp index.html public/'` it goes live; `GET /` → 200 "My test page". |
| [w8/154](../../done/154.md) | read-only `GET /v1/services/<hello-go>` | `dockerContext: "."`, `dockerfilePath: "./Dockerfile"` (was `""`). |
| [w8/164](../../done/164.md) | REST create `traefik/whoami:v1.10.3` (`FROM scratch`), `port: 8080`, `dockerCommand: /whoami --port 8080 --name qa1301` | live in 23 s; `GET /` shows `Name: qa1301`. A shell-syntax command (`env \| grep … ; echo … && exec python -m http.server 10000`) on `python:3.12-alpine` also runs. |
| [w8/165](../../done/165.md) | `bex blueprints validate` with `rootDir` spellings | `./`, `server/`, `./server/`, `/server`, `a//b` valid; `../x`, `a/../b` refused with the `..` message. |
| [w8/166](../../done/166.md) | `POST /v1/blueprints` for `Davender-1565/VisualSort` (the original trigger, Render export with `rootDir: ./`) | `201` with the Blueprint ID, `status: in_sync`, one free static site, live in 21 s, `GET /` → 200 "Sorting Visualizer". |
| [w8/168](../../done/168.md) | REST create `python:3.12-alpine`, no port, `dockerCommand` running `python -m http.server 10000` | `port: 10000`, `PORT=10000`, live in 43 s, `GET /` → 200. Pre-existing `hello-go` keeps `port: 3000` and still serves. |
| [w8/171](../../done/171.md) | same image printing its env (free plan) | `RENDER_CPU_COUNT=0.1`, `RENDER_WEB_CONCURRENCY=1`, `WEB_CONCURRENCY=1`. |

Fixtures: `srv-dbcn2akmjcus73ec3l1g` (pubdef), `blp-dbca9be03oos73cqinu0` + `srv-dbcn50vg16qs73dudru0` (VisualSort), the whoami, p10k and cpu services: all deleted, 204 → 404.

## Sweeps 1302–1303 — real-world native journeys

- `render-examples/elysiajs-hello-world` (Bun on the native Node runtime, `BUN_VERSION=1.3.0`, `bun install` / `bun start`): live, `GET /` → "Hello Elysia", app log `Elysia is running at localhost:10000`.
- `render-examples/go-web-server` (native Go): with `--branch main` the clone fails with `couldn't find remote ref main`, which is correct, because the repo's default branch is `master`. With `--branch` omitted, bex resolves `master` as Render does, builds, and serves "Hello from Render!".
- `render-examples/bun-docker` as a native Node service: live ("Welcome to Bun!"). `bex services update --runtime docker|node|image` is refused by the pinned CLI before any request ("cannot switch runtimes via the CLI; use the API directly"), which is upstream behavior. Runtime change stays REST/GraphQL/dashboard-only (w5/m152).
- `render-examples/fastapi-uv`'s `run.sh` starts `uvicorn --host 0.0.0.0 main:app` with no `--port`, so it binds 8000 and relies on Render's port detection. That is a recorded non-goal on bex (ADR018 row 104 divergence 2, ADR089 D3), so this official example would fail its health check on bex as published. Not filed, but noted as a data point for that decision.
- `bex logs --limit` above 100 is refused (`limit must be an integer between 1 and 100`), matching Render's logs API bound.

## Sweep 1304 — pre-deploy launch

Filed as [w8/172](../../172.md). Free image services on pin `51e6dc76c8fe`:

- `python:3.12-alpine` with `preDeployCommand: /bin/sh -c echo QA-PREDEPLOY-SHC-1304` → pre-deploy `succeeded`, but its window logged one empty line and no marker. After PATCHing to `echo QA-PREDEPLOY-PLAIN-1304`, the marker is logged.
- `traefik/whoami:v1.10.3` with `preDeployCommand: /whoami --help` → `pre_deploy_failed`: "exited with code 128; check the pre-deploy logs", with no pre-deploy logs. The same image's Docker Command `/whoami --port 10000 --name qa1304` is exec'd directly since w8/164.

Fixtures `srv-dbcnlcvg16qs73dudsj0` and `srv-dbcnmacmjcus73ec3lug`: deleted, 204 → 404.

## Sweep 1305 — Blueprint YAML forms (`bex blueprints validate`, no fixtures)

- Accepted: anchors and aliases on an `envVars` list, a `!!str` tag, numeric, boolean and float env `value`s (Render's astro-ssr example writes `value: 10000`), and two non-Latin service names, which get distinct names `resource-<hash>` (w4/385 confirmed on this pin). An emoji-prefixed name gets slug `site`.
- Refused, consistent with Render's pinned schema (`render.yaml.json`: `version` is `{"type":"string","const":"1"}`, `autoDeploy` boolean, env `value` string):
  - unquoted `version: 1` (3 of ~700 corpus files; 114 quote it);
  - `autoDeploy: yes` (a string in YAML 1.2; 1 corpus file);
  - `value:` (null).
- Refused with precise messages: a duplicate mapping key (line and column of the first declaration) and a multi-document file ("exactly one YAML document").
- A YAML merge key (`<<: *common`) is refused as "Blueprint mapping keys must be strings". No corpus file uses `<<:`, so not filed.

## Sweep 1306 — free-form names and CLI name selectors

- `bex services create --name 'Café Ünï 6efb4f'` → `name` kept as given, `slug: cafe-uni-6efb4f`. `GET /v1/services?name=` matches the exact display name, and the pinned CLI's name resolution (`pkg/service/repo.go:164–216`, exact `svc.Name` match) works for `services update` and `services delete` by that name.
- A second service named exactly `QA Dup 6efb4f` → `409 CONFLICT: name "QA Dup 6efb4f" maps to resource name "qa-dup-6efb4f", which is already in use`. Render's [Projects FAQ](https://render.com/docs/projects) says "All of a workspace's services must have unique names—even services that belong to different project environments", so this matches Render; the CLI's multiple-match branch is defensive. Not filed. Residual: names differing only in case or punctuation collide on the slug on bex.
- Fixtures `srv-dbcnp04mjcus73ec3m9g` and `srv-dbcnp0ng16qs73dudt60`: deleted (the second by CLI name), both 404.

## Sweep 1307 — runtime versions

Re-examined under AGENTS.md's parity rule (2026-10-20: a divergence stands only when ADR018 records it with the user's sign-off). ADR018 row 84 lists three runtime-version divergences from ADR060's D7 addendum with no sign-off. All three reproduce on pin `efdd8e2ff06b` with free native fixtures, each deleted 204 → 404:

| Fixture | Signal | Result | Render |
| --- | --- | --- | --- |
| `render-examples/express-hello-world` (`srv-dbco6h0u12fs73e9ai0g`) | `NODE_VERSION=20` | `build_failed`: "NODE_VERSION=20 is not a supported Node version; supported Node lines are 22, 24, 26" | installs 20 (its own default table assigns 20.x and 14.17.0 to older services) |
| `render-examples/fastapi` (`srv-dbco77vt87js73873a9g`) | `PYTHON_VERSION=3.9.18` | `build_failed`: "… supported Python lines are 3.10, 3.11, 3.12, 3.13, 3.14" | any version from 3.7.3 |
| `render-examples/fastapi` (`srv-dbco8l0u12fs73e9ai4g`, created 2026-10-22) | none | runs `Python 3.13.16` | 3.14.8 for services created since 2026-10-12 |
| `render-examples/fastapi` (`srv-dbcoah7t87js73873adg`) | `PYTHON_VERSION=3.11.9` | runs `Python 3.11.17` | exactly 3.11.9 |

Corpus reach (~700 `render.yaml`): 63 of 79 `NODE_VERSION` pins and 9 of 84 `PYTHON_VERSION` pins would be refused, plus 1 of 1 `ELIXIR_VERSION`. Render's own `rails-7`, `rails-6` and `sidekiq` examples pin Ruby 3.2.0, below bex's 3.3 line. Filed as [w8/173](../../173.md), [w8/174](../../174.md) and [w8/175](../../175.md).

## Sweep 1308 — port detection

Filed as [w8/177](../../177.md). On pin `efdd8e2ff06b`, with no port or `PORT` configured:

- `render-examples/fastapi-uv` as published (`./install.sh`, `./run.sh`, which runs `uvicorn --host 0.0.0.0 main:app`; `srv-dbcoct7t87js73873ang`). The app log shows `Uvicorn running on http://0.0.0.0:8000`. The deploy waits on "a TCP connect to port 10000", and `GET /` → first-deploy 503.
- `nginx:1.27-alpine` (`EXPOSE 80`; `srv-dbcohr7t87js73873av0`) reads back `port: 10000`, `openPorts: [10000]`, and stalls the same way after 52 s.

Corpus: 5 of 959 web services hardcode a non-10000 port without `PORT`. Image and Dockerfile ports are not visible to a `render.yaml` scan. Both fixtures deleted (204 → 404).

## Sweep 1310 — log search wildcards and regex

Filed as [w8/176](../../176.md). On the sweep 1309 fixture (`srv-dbcocsou12fs73e9aid0`, one app access line `"GET / HTTP/1.1" 200` and one request-log `GET /`):

- `--text 'HTTP/1.1'` → 1.
- `--text` `GET*200`, `*GET*200*`, `GET*`, `/GET.*200/`, `/GET/` and `/HTTP\/1\.[0-9]/` → 0 each, exit 0.
- `--path` `/` and `/*` and `*` → 1; `--path '/^\/$/'` → 0.

Render's pinned OpenAPI (`render-public-api-1.json`, `GET /logs`) says "Wildcards and regex are supported" for `text`, `path`, `host`, `method`, `statusCode` and `level`.

## Sweep 1309 — cold start

Filed as [w8/178](../../178.md). The free image service `srv-dbcocsou12fs73e9aid0` (`python -m http.server 10000`):

- Live at 03:44:19Z; warm `GET /` at 03:44:30Z; `service_hibernated` at 04:00:19Z.
- A curl-style `GET /` (`Accept: */*`) → `503` in 0.65 s with `Retry-After: 5` and `{"error":"service not ready","retryAfter":5}`.
- Retries at +1 s and +6 s → 503; at +12 s → 200.
- Deleted (204 → 404).

## Sweep 1311 — in-flight drain

Filed as [w8/180](../../180.md). A free `python:3.12-alpine` service whose `GET /slow` sleeps N seconds (the app runs as PID 1, so it ignores `SIGTERM`):

| Run | Fixture | Result |
| --- | --- | --- |
| a | 40 s request; image redeploy with unchanged digest | Request completed (200, 40.6 s). The pod may not have been replaced, so this run is inconclusive. |
| b | 40 s request; `PUT env-vars/QA_ROLL` 3 s later | Pod rolled (`…-799c6b788plzx` → `…-7c859d99298mx`, new pod live after ~32 s). Request completed (200, 40.6 s), inside the old pod's 10 s + 30 s window. |
| c (`srv-dbcp8cgu12fs73e9aj9g`) | Control 70 s request, no roll | 200 in 70.7 s. |
| c | 70 s request; env PUT 1 s later (new pod live at 04:44:34Z) | **502 Bad Gateway at 62.2 s**, about 37 s after the switch. |

Render's [deploys](https://render.com/docs/deploys) page says the old instance gets `SIGTERM` 60 s after the switch, then the shutdown delay. All fixtures were deleted (204 → 404). An earlier attempt with a mis-quoted command (`srv-dbcp238u12fs73e9airg`, crash-looped on a `SyntaxError`) was also deleted.

Unverified, not filed: bex's env-var `PUT` rolls the service immediately (ADR018 env-vars row: "legacy item/replace writes still roll immediately"). Render's docs say env changes take effect on the next deploy, but the pinned OpenAPI does not say whether the API write deploys. A Render capture is needed.

## Sweep 1312 — env-group scope

Filed as [w8/179](../../179.md). A project with one environment (`prj-dbcp76vt87js73873b4g` / `evm-dbcp76vt87js73873b50`), a static site created in that environment with the CLI's `--environment-id`, and a workspace env group (`evg-dbcp77ft87js73873b60`). Linking the group to the service → 409 `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`. `bex blueprints validate` accepts the equivalent Blueprint (a top-level group plus `fromGroup` from an environment service). All four fixtures were deleted (404).

## Sweep 1313 — Key Value allowlist

Filed as [w8/181](../../181.md).

- `bex keyvalues create --plan free` → `red-dbcpn37t87js73873b9g` with `ipAllowList: []` and `persistenceMode: journal_snapshot` (bex's free default).
- `bex keyvalues update --persistence-mode off` → exit 0.
- `bex keyvalues update --ip-allow-list 'cidr=203.0.113.0/24,…'` → exit 1, `400 KEY_VALUE_RESTART_CONFIRMATION_REQUIRED`. A Render-shaped REST `PATCH {"ipAllowList":[…]}` gets the same 400.

Deleted (204 → 404).

## Sweep 1314 — env-group and secret-file reads

Filed as [w8/182](../../182.md).

- Env group: `POST` and `GET /v1/env-groups/<id>` → `envVars: [{"key":"QA_EVG"}]` with no value; the per-key `GET` returns the value.
- Service: `GET /v1/services/<id>/secret-files` → `[{"secretFile":{"name":"qa1314.txt"}}]` with no `content`; the per-file `GET` returns it.
- `GET /v1/services/<id>/env-vars` returns values, which matches Render.

Fixtures (an env group and two free `traefik/whoami` services) were deleted (204 → 404).

## Sweep 1315 — static-site misses

Filed as [w8/183](../../183.md). Free static sites from `mdn/beginner-html-site` (publish directory `.`, no routes):

- Without `404.html`: `/no-such-page` and `/blog/missing-post` → 200 with the home page; `/missing.css` → 404 `not found`.
- With a build-generated `404.html` (`/404.html` → 200 "qa1315 custom 404"): `/no-such-page` → 200 with the home page; `/missing.css` and `/images/nope.png` → 404 plain `not found`. The custom page is never used.

Render (staff answer on community.render.com, and the redirects-rewrites docs): a miss with no matching rule gets 404, with the root `404.html` when present. Three fixtures were deleted (204 → 404).

## Sweep 1316 — external rewrites

Filed as [w8/184](../../184.md). `bex blueprints validate` on a static site with `rewrite /api/* → https://httpbin.org/anything/*` and `redirect /old → https://example.com/new` → exit 1, `routes[0].destination must be a path starting with /`. The redirect alone is accepted (w8/162). Render's docs ("Destination can be either a path or a full, publicly accessible URL") and staff answers in its community confirm external rewrites proxy content. No fixtures.

## Sweep 1317 — `/robots.txt` while asleep

Filed as [w8/185](../../185.md). Free image service `srv-dbcqnlvt87js73873bo0`:

- `service_hibernated` at 06:39:33Z.
- A Googlebot `GET /robots.txt` → 503 JSON `{"error":"service not ready","retryAfter":5}` in 0.69 s.
- `service_woken` at 06:40:09Z.
- Deleted (204 → 404).

Render's free docs: `/robots.txt` gets a "disallow all" response and does not trigger a spin-up. Related correction: [w8/178](../../178.md) now quotes Render's dashboard notice ("can delay requests by 50 seconds or more") and keeps the browser loading page, which Render also shows, out of scope.

## Sweep 1318 — maintenance mode and Postgres versions (no findings)

- `bex services update <free web> --maintenance-mode` → exit 1, `400 maintenanceMode requires a paid web service plan`, matching Render ("available only for paid web services"). A custom maintenance URI is served, not redirected, through the activator's SSRF-guarded fetcher (`validateCustomPageURL`/`fetchOrigin`), as Render serves it. [w8/184](../../184.md) now points at that fetcher as the reuse target for external rewrites.
- Postgres create accepts 13–18 (`postgres/service.go:49`). Render's pinned enum lists 11–18, but its docs say "Major versions 13 through 18 are available for all new instances" (11 and 12 only where the workspace already has one), so bex matches for new databases.
- The fixture was deleted (204 → 404).

## Sweep 1319 — Next.js and Nuxt (no findings)

Render's official `nextjs-hello-world` and `nuxtjs-hello-world`, created with the CLI as free native Node services using their `render.yaml` commands (`yarn; yarn build`, `yarn start`):

- Next.js went live in 366 s; `GET /` → 200.
- Nuxt went live in 260 s; `GET /` → 200.

Both were deleted (204 → 404).

## Sweep 1320 — env-var writes redeploy

Filed as [w8/186](../../186.md). On the free `traefik/whoami` service `srv-dbcs1o4ttgas739mhso0`, a bulk `PUT /env-vars`, a `PUT /env-vars/QA_B` and a `DELETE /env-vars/QA_B` each started a new deploy within a second (`env_vars_changed` followed by `deploy_started`, `envUpdated: true`). Render's API reference for the bulk endpoint: "Changes will **not** be deployed automatically…". Deleted (204 → 404).

Not filed: Render's request-log message format could not be found in any public source (searched docs, blog and community), so the raw-JSON request-log divergence noted in [w8/176](../../176.md) still needs a capture.

## Sweep 1321 — protected environments

Filed as [w8/187](../../187.md). As workspace owner, I created a project with a `protectedStatus: protected` environment and a free `traefik/whoami` service in it.

- Refused with `400 PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED`:
  - `bex deploys create --image …`;
  - REST rollback `{"deployId":…}`;
  - REST `DELETE` and `bex services delete --confirm`.
- A plain `bex deploys create` worked.
- Cleanup needed `DELETE …?confirm=sudo delete service <name>`. The service, project and environment are all 404.

Render's projects docs restrict a fixed list of actions to Admins, with no phrase, and leave rollback and commit/image deploys unrestricted.

## Sweep 1322 — regression on pin `21652364739c` (no findings)

The security-patch pin (deploy run for `216523647` succeeded at 08:18Z) was checked with three free fixtures, each deleted (204 → 404):

- `nginx:1.27-alpine` with `PORT=80`: live in 22 s, `GET /` 200 "Welcome to nginx!".
- Native Node `render-examples/express-hello-world`: live in 145 s, 200.
- Static site from `mdn/beginner-html-site`: live in 31 s, 200.

## Sweep 1323 — environment API

Filed as [w8/188](../../188.md).

- `POST /v1/environments/{id}/resources` (Render's route) → 404.
- bex's `PUT /v1/environments/{id}/service-links` → 200.
- `DELETE` on the then non-empty environment → 204, and the service was detached (`environmentId: null`, still in the project). Render's API answers 409 for a non-empty environment.

The static site, project and both environments are all 404.

## Sweep 1324 — health-check rotation

Filed as [w8/189](../../189.md). A free `python:3.12-alpine` service with `healthCheckPath: /health`, failing after `GET /fail`:

| Time after `/fail` | Observation |
| --- | --- |
| up to +33 s | `GET /` → 200 from the failing instance |
| ≈ +29 s | `server_failed` |
| +34 s | one TLS handshake timeout |
| +58 s | 503 readiness hold |
| +109 s | 200 from the restarted pod (`server_available`) |

Render's docs say an instance is removed after 15 s of failures and restarted at 60 s. Deleted (204 → 404).

## Sweep 1325 — bandwidth-sources

Filed as [w8/190](../../190.md). Read-only on the pre-existing `hello-go`:

- `GET /v1/metrics/bandwidth-sources?resource=…` → 501 `internal_error`, with a message pointing at `/v1/metrics/bandwidth` and GraphQL `monthToDateBandwidth`.
- `GET /v1/metrics/bandwidth` → 200.

Render's pinned spec serves hourly per-source series (`http`, `websocket`, `nat`, `privatelink`, `total`).

Dedupe follow-up: another workstream independently filed [w4/388](../../../w4/388.md) on the image declared-port half of [w8/177](../../177.md). w8/177 is rescoped to runtime port detection.

## Sweep 1326 — list defaults and override scope

Filed as [w8/191](../../191.md). Read-only.

- Unfiltered `GET /v1/notification-settings/overrides` → 6 overrides, all from one of the caller's three workspaces (per-workspace total 15).
- In bex-canary, the no-`limit` list returns 8 items in unsorted order. A cursor taken from its first item returns `[]`. A `limit=3` walk returns all 8 in sorted order.
- `/v1/services` with no `limit` is unsorted too, through the same `core.StablePage`.

## Sweep 1327 — live tail across a rollout (no findings)

A free `python:3.12-alpine` service printing a heartbeat every 5 s, tailed with `bex logs -r <id> --tail -o text`. An env `PUT` 25 s in rolled the pod.

- The tail showed 19 beats from the first pod, then 19 from its replacement, with no gap.
- `SIGINT` stopped the tail cleanly.
- Deleted (204 → 404).

Not re-run: [w8/m118](../../blocked/m118/README.md)'s remaining t005 step needs a custom domain we control.

## Sweep 1328 — restart semantics (no findings)

`bex restart <free whoami>` → exit 0. It emitted `server_restarted`, created a new deploy (trigger `other`) and deactivated the previous one, and served from a new pod within 40 s. Render's deploys docs say "a service restart is actually a special form of manual deploy", reusing the running commit and configuration with zero downtime, so this matches. Deleted (204 → 404).

## Sweep 1329 — instance IDs (known, not re-filed)

`bex services instances <id>` and REST `/instances` agree on `srv-dbcvvhjpgbls73dcpbf0-g49caq71a4cm5vom0c9u`, but inside the container `RENDER_INSTANCE_ID` (and `HOSTNAME`) is the raw pod name `tea-daif693dqjvc73e7as3g-qa-…-inst-7bc9fb98788cr4`. This is w4/386, which is open and blocked on an architecture decision. Reconfirmed on pin `21652364739c`. Deleted (204 → 404).

## Sweep 1330 — default env at build and runtime (no findings)

Native Node web (`express-hello-world`) and a static site printed `RENDER*` / `IS_PULL_REQUEST` / `WEB_CONCURRENCY` from their build and start commands.

- Build and runtime both carry:
  - `RENDER=true` and `IS_PULL_REQUEST=false`;
  - `RENDER_GIT_COMMIT` (full SHA), `RENDER_GIT_BRANCH=main` and `RENDER_GIT_REPO_SLUG=<owner>/<repo>`;
  - `RENDER_EXTERNAL_URL` and `RENDER_EXTERNAL_HOSTNAME`;
  - `RENDER_SERVICE_ID`, `RENDER_SERVICE_NAME` and `RENDER_SERVICE_TYPE` (`web` / `static`);
  - `RENDER_CPU_COUNT=0.1`, plus `RENDER_DISCOVERY_SERVICE` for web.
- Runtime adds `RENDER_INSTANCE_ID` (the pod name, which is w4/386) and `RENDER_WEB_CONCURRENCY` / `WEB_CONCURRENCY=1`.

Note, not filed: builds lack `RENDER_INSTANCE_ID`, and static builds lack `RENDER_DISCOVERY_SERVICE`. Render's [environment-variables](https://render.com/docs/environment-variables) page marks neither as runtime-only, but a build has no service instance and a static site has nothing to discover, so the target is ambiguous. Both fixtures were deleted (204 → 404).

## Sweep 1331 — workspace caps (no findings, open question)

`GET /v1/owners/<canary>/limits` reports `services 25`, `postgres 1` and `keyValues 1`. Render publishes a 25-service Hobby cap and one active **Free** Postgres plus one active **Free** Key Value per workspace. Whether bex's caps of 1 also count paid datastores (which would be stricter than Render) cannot be tested without a paid fixture. Not filed.

## Sweep 1332 — Docker build regression on pin `21652364739c` (no findings)

`bex services create --runtime docker --repo https://github.com/render-examples/bun-docker` (free) → reads back `dockerfilePath: ./Dockerfile` and `dockerContext: .` ([w8/154](../../done/154.md) holds on a new service). It went live in 112 s, and `GET /` → 200 "Welcome to Bun!". Deleted (204 → 404).
