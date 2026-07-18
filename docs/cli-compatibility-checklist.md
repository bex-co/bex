# Render CLI compatibility checklist

The full command surface of the official [Render CLI](https://render.com/docs/cli) (`render-oss/cli` **v2.21.0**), enumerated from `render --help` and **graded against a live bex** by driving the unmodified CLI through its REST API. Every command, subcommand, and flag is a dedicated checklist item.

Legend: `[x]` verified working · `[~]` works with a documented limitation · `[ ]` not working / not verifiable here · `[-]` deliberate bex non-goal.

## How this pass was verified

- **CLI:** `render-oss/cli` v2.21.0, built unmodified from `./cli`, driven only through [`scripts/cli-compat.sh`](../scripts/cli-compat.sh) (Hydra `client_credentials` token exchange per call, `RENDER_HOST` → local bex-api). Production was never touched.
- **Target:** the isolated local **dev-9** environment (`.pm/w9/dev-9`, bex-api at `:54090`), current `lego/backend` HEAD, on **2026-07-18**.
- **Method:** the maintained regression suite `scripts/cli-compat.sh verify` (whole-shape `checkFields` assertions over the core families) **plus** a six-agent parallel sweep of the entire flag matrix and every command `verify` doesn't cover. Each item was graded on the unmodified CLI's exit status and the wire shape read back via `-o json` and raw REST `GET`.
- **Postgres create supplement (w6/m38):** [`scripts/postgres-create-cli-smoke.sh`](../scripts/postgres-create-cli-smoke.sh) drove the same unmodified CLI against isolated **dev-6** and a real local CNPG cluster on **2026-07-18**. It proved both custom names, database-only and user-only independent defaults, the CR intent, generated Secret metadata, and `current_database()`/`current_user` inside PostgreSQL. It also proved that unsupported Datadog create/update flags fail with a named 400 and leave no resource behind.
- **dev-9 does not run** (so paths needing them are marked accordingly, distinguishing a real bex gap from an environment limit): build infra (kpack/zot), OpenBao (env-vars/secret-file store), the registry-credential store, Loki (durable log store), Prometheus, cert-manager, OpenFGA (authz is allow-all here), `BEX_DB_DOMAIN`/`BEX_KV_DOMAIN` (external datastore connect), and the SSH gateway.

### Real gaps found this pass (bex-side, worth filing)

- ~~**`keyvalues update --memory-policy` / `--ip-allow-list` / `--clear-ip-allow-list` are silent no-ops**~~ — **fixed (w7/m45)**: `KeyValuePatch` now carries `MaxmemoryPolicy` + `IPAllowList` and `handleUpdateKeyValue` decodes them, so the CLI's update flags mutate the store; GraphQL `setKeyValueMaxmemoryPolicy` + MCP `set_key_value_maxmemory_policy`/`set_key_value_ip_allow_list` bring the programmatic surfaces to parity. (Dashboard memory-policy editing after create is a tracked follow-up.)
- _(upstream CLI, not bex)_ **`skills list` / `skills update` panic** (nil-pointer) in every non-TTY output mode.

### Intended divergences (not bugs)

- `--region` is accepted but **platform-stamped** (`local-capd`); the value submitted is not persisted. This is what makes a bare `services create --from` clone fail — the CLI re-validates the source's `local-capd` region against its own enum — so clones need an explicit `--region`.
- `--previews` is rejected platform-wide (`400 "not supported by this platform"`).
- Postgres Datadog forwarding is not implemented. Supplying `--datadog-api-key` or `--datadog-site` on create or update returns a named 400; bex never accepts or persists the credential.
- `services update --runtime` is rejected via the CLI by design ("cannot switch runtimes via the CLI"); `services --ip-allow-list` keeps the CIDR but drops per-entry descriptions (Postgres/Key Value do persist them).

> Regenerate the command tree with `render <subcommand> --help`. Re-run the graded baseline with `scripts/cli-compat.sh verify`. Grouping mirrors the CLI's own `render --help` sections.

The interactive-only Key Value client has a separate, opt-in full-edge verifier: set the production-equivalent `BEX_API_URL`, `HYDRA_PUBLIC_URL`, `CLI_KEY_ENV`, and the runner's explicit `BEX_KV_VERIFY_ALLOW_CIDR`, then run `scripts/cli-compat.sh kv-cli-verify`. It provisions and always deletes one source-restricted public Key Value. The verifier launches the unmodified v2.21.0 CLI under an automated pseudo-terminal with explicit `--output interactive`; one-shot Redis arguments after `--` let the child and TUI exit without human input. This is deliberately excluded from the dev-9 baseline because it requires public Key Value DNS, TLS, and the SNI proxy. The first production run reached the CLI's spawned `redis-cli` but exposed a deployed DNS defect: `*.kv.bex.co` was Cloudflare-proxied even though raw TCP requires DNS-only records. [`scripts/datastore-dns-cloudflare.sh`](../scripts/datastore-dns-cloudflare.sh) now reconciles and checks that prerequisite. The row remains open until the DNS repair is applied and the full external-edge run passes.

## Core

- [x] **`deploys`** — list, create, and cancel deploys
  - [x] `deploys list <serviceID>` — carries Render's nested `{image:{ref,…}}` shape
  - [x] `deploys create <serviceID>` — returns a deploy id; accepts the CLI's `clearCache` string enum
    - [x] `--clear-cache`
    - [x] `--commit <id>`
    - [x] `--image <url>`
    - [x] `--wait`
  - [x] `deploys cancel <serviceID> <deployID>` — resolves and cancels an open deploy
- [x] **`jobs`** — create and manage one-off jobs
  - [x] `jobs list <serviceID>` — lists jobs with status/planId/startCommand
  - [x] `jobs create <serviceID>` — returns a `job-…` record
    - [x] `--start-command <cmd>`
    - [x] `--plan-id <id>` — `starter`/`free` accepted, echoed back
  - [x] `jobs cancel <serviceID> <jobID>` — cancels a running job
- [x] **`keyvalues`** (alias `kv`) — manage Render Key Value instances
  - [x] `keyvalues create` — nested owner/options, opaque `red-<xid>` id, underscore `maxmemoryPolicy`
    - [x] `--name`
    - [x] `--plan <free|starter|standard|pro|pro_plus>`
    - [~] `--region` — accepted but echoes `local-capd` (single fixed region)
    - [x] `--memory-policy <cache|queue|raw policy>` — `cache`→`allkeys_lru` alias and raw policies round-trip
    - [x] `--ip-allow-list cidr=…,description=…` — CIDR **and** description round-trip
    - [x] `--workspace <id|name>`
    - [~] `--project <id|name>` — flag parsed; needs an existing project (none creatable via CLI)
    - [~] `--environment <id|name>` — flag parsed; needs an existing project/environment
  - [x] `keyvalues list`
  - [x] `keyvalues get <id|name>`
  - [x] `keyvalues update <id|name>` — resolves by opaque id; core fields apply
    - [x] `--name` — rename; opaque `red-` id stays stable
    - [x] `--plan`
    - [x] `--memory-policy` — mutates `maxmemoryPolicy` on read-back (fixed w7/m45)
    - [x] `--ip-allow-list` — replaces the allow-list; CIDR + description return (fixed w7/m45)
    - [x] `--clear-ip-allow-list` — empties the allow-list (fixed w7/m45)
  - [x] `keyvalues suspend <id|name>`
  - [x] `keyvalues resume <id|name>`
  - [x] `keyvalues delete <id|name>`
- [x] **`logs`** — view logs for services and datastores (single command)
  - [x] query mode — resolves a service by name; empty windows return stable cursors (no parse crash)
  - [x] `--tail` — streams live pod-log JSON
  - [x] `-r, --resources <ids>` — required in non-interactive mode; honored
  - [x] `--instance <ids>` — in live-pod mode the instance label is the pod name
  - [x] `--start <time>`
  - [x] `--end <time>`
  - [x] `--direction <backward|forward>`
  - [x] `--limit <count>`
  - [x] `--text <query>` — filter genuinely applied (empty result on no match)
  - [~] `--level <levels>` — `503`, needs the durable store (`BEX_LOKI_URL`)
  - [~] `--type <types>` — `app` works live; `request` needs the durable store
  - [~] `--host <hosts>` — `503`, needs the durable store
  - [~] `--status-code <codes>` — `503`, needs the durable store
  - [~] `--method <methods>` — `503`, needs the durable store
  - [~] `--path <paths>` — `503`, needs the durable store
  - [x] `--task-id <ids>` — accepted as a filter
  - [x] `--task-run-id <ids>` — accepted as a filter
- [x] **`postgres`** (alias `pg`) — manage Render Postgres databases
  - [x] `postgres create` — id/name/ipAllowList wire shape correct (description persists)
    - [x] `--name`
    - [x] `--plan <free|basic_*|pro_*|accelerated_*>`
    - [~] `--region <frankfurt|ohio|oregon|singapore|virginia>` — accepted; platform-stamped `local-capd`
    - [x] `--version <int>`
    - [x] `--disk-size-gb <int>`
    - [x] `--disk-autoscaling`
    - [x] `--high-availability` — persists to the CR spec (status flips once replicas are ready)
    - [x] `--read-replica <name>` — persists to the CR spec
    - [x] `--database-name <string>` — persisted as the immutable physical database name and live-proven through CNPG SQL identity
    - [x] `--database-user <string>` — persisted as the immutable owner role and live-proven through CNPG SQL identity; either custom-name flag may be omitted independently
    - [-] `--datadog-api-key <string>` — deliberate non-goal; named 400, credential never persisted
    - [-] `--datadog-site <string>` — deliberate non-goal; named 400
    - [x] `--workspace <id|name>`
    - [~] `--project <id|name>` — flag parsed; needs an existing project
    - [~] `--environment <id|name>` — flag parsed; needs an existing project/environment
  - [x] `postgres list` — resolves through the RC3 cursor envelope
  - [x] `postgres get <id|name>` — resolves by name; every field intact; `-o text` renders Workspace/Region
  - [x] `postgres update <id|name>`
    - [x] `--name` — rename; opaque `dpg-` id stays stable
    - [x] `--plan`
    - [x] `--disk-size-gb`
    - [x] `--disk-autoscaling` — flips true↔false
    - [x] `--high-availability`
    - [x] `--ip-allow-list cidr=…,description=…` — replaces list; description returns
    - [x] `--clear-ip-allow-list` — empties the list
    - [-] `--datadog-api-key <string>` — deliberate non-goal; named 400, credential never persisted
    - [-] `--datadog-site <string>` — deliberate non-goal; named 400
    - [~] `--project <id|name>` — flag parsed; needs an existing project
    - [~] `--environment <id|name>` — flag parsed; needs an existing environment
  - [x] `postgres suspend <id|name>`
  - [x] `postgres resume <id|name>`
  - [x] `postgres delete <id|name>`
- [x] **`restart <resourceID>`** — restarts by typed id (`restart <resourceID>` is the CLI's only documented form)
- [x] **`services`** — list services and datastores; bare `services` lists them
  - [x] `-e, --environment-ids <ids>` — filter list by environment IDs
  - [x] `--include-previews` — accepted list flag
  - [x] `services create` — returns the `{service,deployId}` record; raw `dashboardUrl` is `…/<type>/<srv-id>` (`/web/`, `/cron/`, `/static/`)
    - [x] `--name`
    - [x] `--type` — `web_service`, `cron_job`, `static_site` all accepted
    - [x] `--runtime` — round-trips (build not exercised in dev-9)
    - [x] `--repo` — round-trips (build not exercised)
    - [x] `--branch`
    - [x] `--image`
    - [x] `--plan`
    - [~] `--region` — accepted; bex overwrites to `local-capd` (CLI enum rejects `local-capd` itself)
    - [x] `--num-instances`
    - [x] `--build-command` — round-trips (build not exercised)
    - [x] `--start-command`
    - [x] `--pre-deploy-command`
    - [x] `--cron-command` — on `cron_job`
    - [x] `--cron-schedule` — on `cron_job`
    - [x] `--health-check-path`
    - [x] `--auto-deploy`
    - [-] `--previews` — `400 "not supported by this platform"` (deliberate non-goal)
    - [x] `--publish-directory` — on `static_site`
    - [x] `--root-directory` — on repo services
    - [~] `--env-var KEY=VALUE` — accepted; round-trip needs OpenBao (`503` in dev-9)
    - [~] `--secret-file NAME:PATH` — needs OpenBao (`503` in dev-9)
    - [~] `--registry-credential <cred>` — bex resolves it (workspace-scoped "no credential found"); cred store not configured in dev-9
    - [x] `--ip-allow-list cidr=…,description=…` — CIDR round-trips; description dropped (Postgres/KV keep it)
    - [x] `--build-filter-path <path>`
    - [x] `--build-filter-ignored-path <path>`
    - [x] `--maintenance-mode` — requires a paid plan (`400` on free)
    - [x] `--maintenance-mode-uri <uri>`
    - [x] `--max-shutdown-delay <seconds>`
    - [x] `--environment-id <id>`
    - [~] `--from <serviceID>` — clones fine, but needs an explicit `--region` (CLI re-validates the source's `local-capd`)
  - [x] `services update <service>`
    - [x] `--name` — rename
    - [x] `--plan`
    - [ ] `--runtime` — bex rejects runtime switch via CLI by design ("use the API directly")
    - [x] `--repo`
    - [x] `--branch`
    - [x] `--image`
    - [x] `--build-command`
    - [x] `--start-command`
    - [x] `--pre-deploy-command`
    - [ ] `--cron-command` — native-runtime only; not buildable in dev-9 (image cron rejected)
    - [x] `--cron-schedule`
    - [x] `--health-check-path`
    - [x] `--auto-deploy`
    - [-] `--previews` — `400 "not supported by this platform"`
    - [x] `--publish-directory` — on `static_site`
    - [x] `--root-directory` — on repo services (image services correctly `400`)
    - [~] `--registry-credential` — reaches bex; `503` cred store not configured in dev-9
    - [x] `--ip-allow-list cidr=…,description=…` — CIDR round-trips
    - [x] `--build-filter-path <path>`
    - [x] `--build-filter-ignored-path <path>`
    - [x] `--maintenance-mode`
    - [x] `--maintenance-mode-uri <string>`
    - [x] `--max-shutdown-delay <int>`
  - [x] `services instances <serviceID>` — decodes live pod ids; suspended service returns `[]`; unknown id fails not-found
  - [x] `services delete <serviceID>` — returns the deleted record
- [-] **`workflows`** — Render Workflows (deliberate bex non-goal; `GET /v1/workflows` is a `200 []` stub, everything else `404`/`405`/TTY-blocked)
  - [-] `workflows list` — returns empty from the stub (exit 0)
  - [-] `workflows create` — `405 Method Not Allowed`
  - [-] `workflows init` — client-side scaffolding works, no bex call
  - [-] `workflows dev` — client-side dev server, no bex call
  - [-] `workflows start` / `workflows cancel` — shortcuts; `404` / TTY-blocked
  - [-] `workflows tasks` — `tasks list` / `tasks runs {start,list,show,cancel}` all `404` / TTY-blocked
  - [-] `workflows versions` — `versions list` / `versions release` `404`
- [x] **`workspaces`** — lists the caller's real `tea-…` workspace

## Auth

- [x] **`login`** — recognizes `RENDER_API_KEY` (already-authenticated short-circuit); full browser/device flow verified separately in production
- [x] **`logout`** — drives the OAuth revoke path; the credential's `client_credentials` grant fails afterward
- [x] **`whoami`** — reports the key-minting user's email (Kratos-admin lookup)
- [x] **`workspace`** — manage the CLI's active workspace
  - [x] `workspace current` — returns the active tenant
  - [x] `workspace set <id>` — set→persist→read round-trip verified

## Session

- [ ] **`kv-cli [id|name]`** — automated PTY harness reaches the interactive CLI without a human TTY; full public `rediss://` id/name acceptance is pending (`scripts/cli-compat.sh kv-cli-verify`)
- [ ] **`pgcli [id|name]`** — interactive-only client guard; not verifiable headlessly
- [~] **`psql [id|name]`** — reaches bex, resolves the target by id **and** name, then stops at the Render-parity IP-allow-list gate (no `BEX_DB_DOMAIN`/allow-list in dev-9 — the same block Render imposes)
  - [~] `-c, --command <SQL>` — flag parses, required in non-TTY, drives the full path to the same gate
- [~] **`ssh [serviceID|serviceName|instanceID]`** — command present and parses; interactive-only + no SSH gateway in dev-9, so not exercisable headlessly here (the running-instance path has separate production evidence, [ADR035](ADR035-ssh.md))
  - [-] `-e, --ephemeral` — deliberate bex non-goal (flag parses)
  - [-] `--plan <string>` — tied to `--ephemeral` (non-goal)

## Management

- [x] **`blueprints`** — manage Blueprints (infrastructure as code)
  - [x] `blueprints validate <render.yaml>` — accepts the multipart request, returns `{plan,valid:true}`
- [x] **`environments <projectID>`** — decodes the RC15 cursor envelope into real values; unknown project fails not-found
- [x] **`projects`** — lists projects in the active workspace

## Additional commands

- [x] **`docs`** — opens `render.com/docs` client-side (no bex call)
- [-] **`ea`** — early-access surfaces, not implemented by bex (outside the compatibility target)
  - [-] `ea objects list` — bex `/v1/objects` → `404` (`--local` works client-side)
  - [-] `ea objects put` — `404` (`--local` works client-side)
  - [-] `ea objects get` — `404` (`--local` works client-side)
  - [-] `ea objects delete` — `404` (`--local` works client-side)
  - [-] `ea sandbox create` — `/v1/sandboxes` → `404`
  - [-] `ea sandbox exec` — `404`
  - [-] `ea sandbox list` — `404`
  - [-] `ea sandbox stop` — `404`
- [~] **`skills`** — manage Render agent skills for AI coding tools (client-side; no bex dependency)
  - [x] `skills install` — works; `--dry-run` reports intended changes without writing
  - [ ] `skills list` — **upstream CLI panic** (nil-pointer) in every non-TTY output mode
  - [ ] `skills update` — same upstream non-TTY panic; no `--dry-run`
  - [x] `skills remove` — works
- [x] **`help [command]`** — CLI builtin, no bex call
