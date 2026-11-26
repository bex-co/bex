# Render's default environment variables on production

Captured 2026-11-17 for w32/m425. It records what a bex service actually reads, at build and at runtime, for every variable [render.com/docs/environment-variables](https://render.com/docs/environment-variables) lists (fetched 2026-11-17), by service type and by source. Re-run it with [`examples/default-env-probe`](../../examples/default-env-probe/README.md) after any change to a default variable.

- **Production pin:** `55c120858af0` (`9c79201b4`). `bex-controller-manager` and `bex-api` ran `ghcr.io/bex-co/bex-operator@sha256:fe4ebdc061d0…`, the digest that commit pins.
- **When and where:** 09:23–09:42 UTC, workspace `bex-canary`, as the QA user over REST. Builds cloned `bex-co/bex@1c92455c2` (root directory `examples/default-env-probe`) and `render-examples/phoenix_hello@d95c60fd4aa2`.
- **Method:** `probe.sh` runs first in each build command and each start command. It prints the 43 documented names plus `PORT`, by name only, as `DEFAULT_ENV <phase> NAME=value` or `… NAME unset`. The Docker fixture declares each name as an `ARG`, so its build shows which defaults arrive as build args. The image fixture prints the same names with `env | grep`. The lines were read back with `GET /v1/logs?type=build|app&text=DEFAULT_ENV`. No fixture set variables of its own, except the Elixir one's throwaway `SECRET_KEY_BASE`, which was never printed.
- **Cleanup:** every fixture was deleted at 09:42 UTC, and its by-id `GET` answered `404`.
- **The three cells w32/m424 changed** (Python's `GUNICORN_CMD_ARGS`, Ruby's `PASSENGER_PORT` and `GEM_PATH`) were read on the next pin, `6984548a30f8` (`336b647b1`), which carries w32/m424's `f1cdd052b`. From 10:38:43 UTC `bex-controller-manager` and `bex-api` ran `ghcr.io/bex-co/bex-operator@sha256:4b03796361d1…`, and the fixtures' build Jobs resolved their native runtime on the same digest. Two fresh probe fixtures, `py` (`srv-dbu2uug4f0dc73876r30`) and `rb` (`srv-dbu2uuo4f0dc73876r50`), built `bex-co/bex` `main` with the same root directory (unchanged since `c0cb02045`), were read 10:45–10:47 UTC during w32/m424's production walk, and were deleted at 11:00 UTC with by-id `GET` `404`.

**Legend:** ✅ matches Render's page. ✖ mismatch, with its follow-up. — not comparable, with the reason. A pair such as ✅ ✅ is build, then runtime.

## Fixtures

All web fixtures are on the Free plan (0.1 CPU); the private service, worker and cron job are on Starter (0.5 CPU).

| Fixture | Service id | Source | Type | Result |
| --- | --- | --- | --- | --- |
| `node` | `srv-dbu1q1likg6s738laua0` | native Node 24 | web | live; listens only on 3000 (w8/202 below) |
| `py` | `srv-dbu1q2dikg6s738lauc0` | native Python 3.13 | web | live |
| `rb` | `srv-dbu1q2dikg6s738laue0` | native Ruby 3.4 | web | live |
| `go` | `srv-dbu1q2mc1l2s739d46lg` | native Go | web | live |
| `rs` | `srv-dbu1q2tikg6s738laug0` | native Rust | web | live |
| `ex` | `srv-dbu1q3ec1l2s739d46tg` | native Elixir 1.18, `phoenix_hello` | web | live, but answered `502` (w32/037) |
| `dk` | `srv-dbu1q2uc1l2s739d46ng` | Dockerfile (`busybox:1.37.0`) | web | live |
| `img` | `srv-dbu1q36c1l2s739d46pg` | prebuilt `docker.io/library/busybox:1.37.0` | web | live; no build |
| `ps` | `srv-dbu1q45ikg6s738laui0` | native Node | private | live |
| `wk` | `srv-dbu1q4ec1l2s739d46vg` | native Node | worker | live |
| `cr` | `srv-dbu1q4ec1l2s739d471g` | native Node | cron | one triggered run, `successful` |
| `st` | `srv-dbu1q36c1l2s739d46rg` | native static build | static | live; build only |

## Every-runtime variables, by source

Web services. The native columns read the same names and values; only `RENDER_GIT_*` differs on the Elixir fixture, which builds Render's repository.

| Variable | Render's page | Native (Node, Python, Ruby, Go, Rust, Elixir) | Docker | Image | Observed |
| --- | --- | --- | --- | --- | --- |
| `IS_PULL_REQUEST` | `false` outside a preview | ✅ ✅ | ✅ ✅ | ✅ | `false` |
| `RENDER` | `true` | ✅ ✅ | ✅ ✅ | ✅ | `true` |
| `RENDER_CPU_COUNT` | the plan's CPU, e.g. `0.5` | ✅ ✅ | ✅ ✅ | ✅ | `0.1` (Free) |
| `RENDER_DISCOVERY_SERVICE` | `$RENDER_SERVICE_NAME-discovery` | ✅ ✅ | ✅ ✅ | ✅ | `qa-20261117-m425-node-discovery` |
| `RENDER_EXTERNAL_HOSTNAME` | the platform host, web and static | ✅ ✅ | ✅ ✅ | ✅ | `qa-20261117-m425-node.onbex.co` |
| `RENDER_EXTERNAL_URL` | the platform URL, web and static | ✅ ✅ | ✅ ✅ | ✅ | `https://qa-20261117-m425-node.onbex.co` |
| `RENDER_GIT_BRANCH` | the deploy's branch | ✅ ✅ | ✅ ✅ | — no repository | `main`; `master` for Elixir |
| `RENDER_GIT_COMMIT` | the deploy's commit | ✅ ✅ | ✅ ✅ | — no repository | `1c92455c27ad…`; `d95c60fd4aa2…` for Elixir |
| `RENDER_GIT_REPO_SLUG` | `$username/$reponame` | ✅ ✅ | ✅ ✅ | — no repository | `bex-co/bex`; `render-examples/phoenix_hello` |
| `RENDER_INSTANCE_ID` | the current instance | — ✅ | — ✅ | ✅ | runtime `<service id>-<suffix>`, one per instance; unset at build (see Unverified) |
| `RENDER_SERVICE_ID` | the service id | ✅ ✅ | ✅ ✅ | ✅ | `srv-…` |
| `RENDER_SERVICE_NAME` | the service's slug | ✅ ✅ | ✅ ✅ | ✅ | `qa-20261117-m425-node` |
| `RENDER_SERVICE_TYPE` | `web`, `pserv`, `cron`, `worker`, `static` | ✅ ✅ | ✅ ✅ | ✅ | `web` |
| `RENDER_WEB_CONCURRENCY` | web and private, runtime only | ✅ ✅ | ✅ ✅ | ✅ | unset at build; `1` at runtime |
| `WEB_CONCURRENCY` | as above, services created after 2025-12-08 | ✅ ✅ | ✅ ✅ | ✅ | unset at build; `1` at runtime |
| `PORT` | a web service's port, default `10000` | — ✅ | — ✅ | ✅ | unset at build; `10000` at runtime, also on the Node fixture that listens on 3000 |

The Docker column's build cells are build args: each name reached the build because the Dockerfile declared it as an `ARG`, the way Render passes a service's environment to a Docker build.

## Every-runtime variables, by type

The native Node source on each type. Variables not listed (`IS_PULL_REQUEST`, `RENDER`, `RENDER_GIT_*`, `RENDER_SERVICE_ID`, `RENDER_SERVICE_NAME`) read the same as on a web service, ✅ in every phase of every type.

| Variable | Web | Private | Worker | Cron | Static (build) |
| --- | --- | --- | --- | --- | --- |
| `RENDER_SERVICE_TYPE` | ✅ ✅ `web` | ✅ ✅ `pserv` | ✅ ✅ `worker` | ✅ ✅ `cron` | ✅ `static` |
| `RENDER_CPU_COUNT` | ✅ ✅ `0.1` | ✅ ✅ `0.5` | ✅ ✅ `0.5` | ✅ ✅ `0.5` | — `0.1`; a static site has no compute plan, and the page does not say what its build reads |
| `RENDER_EXTERNAL_HOSTNAME`, `_URL` | ✅ ✅ set | ✅ ✅ unset | ✅ ✅ unset | ✅ ✅ unset | ✅ set |
| `RENDER_DISCOVERY_SERVICE` | ✅ ✅ | ✅ ✅ | ✅ ✅ | — unset; a cron run has no instances to discover | — unset, as above |
| `RENDER_INSTANCE_ID` | — ✅ | — ✅ | — ✅ | — ✅ (the run's own) | — unset |
| `RENDER_WEB_CONCURRENCY`, `WEB_CONCURRENCY` | ✅ ✅ `1` at runtime | ✅ ✅ `1` at runtime | ✅ ✅ unset | ✅ ✅ unset | ✅ unset |
| `PORT` | — ✅ `10000` | — ✅ `10000` | — — `10000` | — — `10000` | — unset |
| `NODE_ENV` | ✅ ✅ | ✅ ✅ | ✅ ✅ | ✅ ✅ | ✅ unset (runtime only) |

Render's page says `RENDER_EXTERNAL_*` and the two concurrency variables are "empty" where they do not apply; bex leaves them unset. Code that reads them as strings sees the same empty value either way.

## Per-runtime defaults

Each runtime's web fixture. Every one of these names was unset on every other runtime's fixture, on the Docker and image fixtures, and on the Node private, worker, cron and static fixtures (other than Node's own), so no runtime's defaults reach another. Render's page lists no defaults for Docker.

| Runtime | Variable | Render's value | Build | Runtime | Observed |
| --- | --- | --- | --- | --- | --- |
| Elixir | `MIX_ENV` | `prod` | ✅ | ✅ | `prod` |
| Elixir | `RELEASE_DISTRIBUTION` | `name` | ✅ | ✅ | `name` (w8/272 below) |
| Go | `GO111MODULE` | `on` | ✅ | ✅ | `on` |
| Go | `GOPATH` | `/opt/render/project/go` | ✅ | ✅ | same |
| Node | `NODE_ENV` | `production`, runtime only | ✅ | ✅ | unset at build; `production` at runtime on all four Node services |
| Node | `NODE_MODULES_CACHE` | `true` | ✖ w8/m161 | ✖ w8/m161 | unset: bex has no dependency cache for it to switch |
| Python | `CI` | `true`, build only | ✅ | ✅ | `true` at build; unset at runtime |
| Python | `FORWARDED_ALLOW_IPS` | `*` | ✅ | ✅ | `*` |
| Python | `GUNICORN_CMD_ARGS` | `--preload --access-logfile - --bind=0.0.0.0:10000` | ✅ | ✅ | same, on pin `6984548a30f8`; `55c120858af0` read `--preload --access-logfile -`, without the `--bind` w32/m424 added |
| Python | `PIPENV_YES` | `true` | ✅ | ✅ | `true` |
| Python | `VENV_ROOT` | `/opt/render/project/src/.venv` | ✅ | ✅ | same; the virtualenv itself is w8/274's production check |
| Ruby | `BUNDLE_APP_CONFIG` | `/opt/render/project/.gems` | ✅ | ✅ | same |
| Ruby | `BUNDLE_BIN` | `/opt/render/project/.gems/bin` | ✅ | ✅ | same |
| Ruby | `BUNDLE_DEPLOYMENT` | `true` | ✅ | ✅ | `true` |
| Ruby | `BUNDLE_PATH` | `/opt/render/project/.gems` | ✅ | ✅ | same |
| Ruby | `GEM_PATH` | `/opt/render/project/.gems` | ✅ | ✅ | same, on pin `6984548a30f8`; unset on `55c120858af0` |
| Ruby | `MALLOC_ARENA_MAX` | `2` | ✅ | ✅ | `2` |
| Ruby | `PASSENGER_ENGINE` | `builtin` | ✅ | ✅ | `builtin` |
| Ruby | `PASSENGER_ENVIRONMENT` | `production` | ✅ | ✅ | `production` |
| Ruby | `PASSENGER_PORT` | `10000` | ✅ | ✅ | `10000`, on pin `6984548a30f8`; unset on `55c120858af0` |
| Ruby | `PIDFILE` | `/tmp/puma-server.pid` | ✅ | ✅ | same |
| Ruby | `RAILS_ENV` | `production` | ✅ | ✅ | `production` |
| Ruby | `RAILS_SERVE_STATIC_FILES` | `true` | ✅ | ✅ | `true` |
| Ruby | `RAILS_LOG_TO_STDOUT` | `true` | ✅ | ✅ | `true` |
| Rust | `CARGO_HOME` | `/opt/render/project/.cargo` | ✅ | ✅ | same |
| Rust | `ROCKET_ENV` | `prod` | ✅ | ✅ | `prod` |
| Rust | `ROCKET_PORT` | `10000`, runtime only | ✅ | ✅ | unset at build; `10000` at runtime; a service's own value wins (w8/273, sweep `81af`) |
| Rust | `RUSTUP_HOME` | `/opt/render/project/.rustup` | ✅ | ✅ | same |

`GUNICORN_CMD_ARGS`, `GEM_PATH` and `PASSENGER_PORT` were read on pin `6984548a30f8`, as the header says. There too, `py` read `PASSENGER_PORT` and `GEM_PATH` unset, and `rb` read `GUNICORN_CMD_ARGS` unset. No other fixture was re-read on that pin: `f1cdd052b` puts the three in the native Python and Ruby images only, and the operator sets its serving-port value of `GUNICORN_CMD_ARGS` only on native Python and of `PASSENGER_PORT` only on native Ruby (`NativeServingPortEnv`, `lego/operator/internal/build/native.go`), so none of them reaches the other fixtures.

## Beyond the page

Render's page does not list this one, but its native environment sets it, and the probe prints it since w9/m493.

| Runtime | Variable | Render's value | Build | Runtime | Observed |
| --- | --- | --- | --- | --- | --- |
| every native runtime, static builds included | `XDG_CACHE_HOME` | `/opt/render/.cache`, at build and at runtime; the directory persists between builds only | ✅ | ✅ | `/opt/render/.cache` at build and at runtime, read on native Python on pin `4b2cebd0bd03` (w9/m493 t004, 2026-11-19): a marker written in one build was read in the next, the running service found the directory empty, and a clear-cache deploy started it empty |

Render's value comes from a Render staff answer on community.render.com ("XDG_CACHE_HOME is writable, but is only persisted between builds") and a printed Render runtime environment (`XDG_CACHE_HOME: '/opt/render/.cache'`). The Docker fixture declares it as an `ARG`, so its build line shows whether a Docker build receives it; Render's Docker builds are not documented to.

## Acceptance checks owed elsewhere

- **w8/272 (Elixir long node name): passes.** With no clustering configuration, `bin/phoenix_hello rpc 'IO.puts(node())'` in the instance printed `phoenix_hello@srv-dbu1q3ec1l2s739d46tg-u39333opr9djbj8gvt12.nodes.tea-daif693dqjvc73e7as3g.svc.cluster.local`. After the service set `RELEASE_DISTRIBUTION=sname` and redeployed (`dep-dbu1vbec1l2s739d481g`, live 09:40:14), it printed `phoenix_hello@srv-dbu1q3ec1l2s739d46tg-miso1bl374a6oni9dpda`, and the node ran with `-sname phoenix_hello`.
- **A separate defect on the same fixture:** the release's `epmd` (4369) opened about a minute before Phoenix bound `PORT` on the Free plan, and port detection adopted 4369 (`serviceDetails.port: 4369`). The deploy closed `live`, and the URL answered `502` while Phoenix served on 10000 inside the pod. Filed as w32/037.
- **w8/202 (a detected port on the default `PORT`): passes again.** The `node` fixture's process listened only on 3000 and set no `PORT`. The deploy went live at 09:28:06, `serviceDetails.port` read `3000` with `openPorts: [3000]`, `GET /` answered `200 ok`, and the process still read `PORT=10000`, as on Render. The first walk passed on 2026-10-28 (Render's `hapi-quick-start`, pin `838feec85fdd`).

## Unverified on Render

bex has no Render account (w10/blocked/002), so these — cells rest on the page's wording alone. A walk with an account should print the same names on Render and file any drift.

- `RENDER_INSTANCE_ID` at build: the page says its variables are available at build and runtime unless noted, and it notes nothing for this one. A bex build has no instance and leaves it unset.
- `PORT` at build, and on workers and cron jobs: the page documents `PORT` as a web service's port. bex sets it on every running service and on no build.
- `RENDER_DISCOVERY_SERVICE` on a cron job or static site, and `RENDER_CPU_COUNT` in a static build.
- `RENDER_DISCOVERY_SERVICE` and a resolvable `<name>-discovery` hostname on a background worker. The environment-variables page lists the variable for a scaled service without naming types, while the private-network page gives a discovery hostname to web and private services only. bex sets both on a worker, and its connections are refused (w6/m640, [ADR018](../ADR018/services-lifecycle.md)'s private-network row).
- Whether "empty" means set to an empty string or unset.
