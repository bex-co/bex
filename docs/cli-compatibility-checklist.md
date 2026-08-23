# Render CLI compatibility checklist

The full command surface of the official [Render CLI](https://render.com/docs/cli) (`render-oss/cli` **v2.24.0**), enumerated from `render --help` and **graded against a live bex** by driving the unmodified CLI through its REST API. Every command, subcommand, and flag is a dedicated checklist item.

Legend: `[x]` verified working · `[~]` works with a documented limitation · `[ ]` not working / not verifiable here · `[-]` deliberate bex non-goal.

## Native `bex` launcher (w9/m59)

`lego/cli/` imports (rather than forks) Render CLI **v2.24.0**, pinned to `fe8a6188119ee1a53dcf3e5c19f6a5302e840c3f`, and starts its exported command runner after mapping Bex defaults. With no `RENDER_*` input, it targets `https://api.bex.co/v1/` and lets the upstream config library persist its normal YAML schema as `~/.bex/cli.yaml`; `BEX_HOST`, `BEX_CLI_CONFIG_{DIR,PATH}`, `BEX_WORKSPACE`, `BEX_OUTPUT`, and a temporary `BEX_ACCESS_TOKEN` are supported Bex inputs. Hermetic launcher tests cover that mapping, process-level authenticated REST behavior, and Render-config isolation. `scripts/bex-cli-auth-e2e.sh` is the live device-login/refresh/ logout evidence runner; it must be run against a Bex auth environment before a release tag. The launcher overlays Bex help chrome (`Use`, Short/Long/examples, `bex docs`, version line) via `lego/cli/internal/branding` without forking; residual runtime strings (login TUI, `ErrLogin`, User-Agent prefix, skills state path) remain upstream.

This launcher record does not replace the unmodified-v2.24.0 server oracle below: that client is the broad command/API compatibility baseline after the w2/m79 re-baseline (prior baselines used v2.21.0 / launcher v2.22.0). Known server-side non-goals are still recorded as `[-]` in this checklist, including workflows, ephemeral SSH, and `ea` objects; importing a newer client does not claim those operations as implemented.

## How this pass was verified

- **Human device-flow op-class pass (2026-08-20):** after fixing the login → _"you are not allowed to take this action"_ regression (the `cliauth` device-grant requested no granular capability, so every authenticated call 403'd at the dispatch scope matrix — see the **`login`** row), the unmodified `bex` binary was driven against a locally-run real bex-api (control-plane store on a throwaway Postgres, real `app.bex.co` CRDs, a fake Hydra introspecting the platform-marked **human** device-flow token that now carries `bex.read bex.write bex.sensitive`). ~90 invocations were exercised with **zero authorization refusals**: 60 client-side (`--version`, `help`, `completion` ×4, `docs`, and the full `--help` tree for every command/subcommand) plus ~30 authenticated across all four op-classes — `whoami`/`workspaces`/`workspace current`; `services`/`postgres`/`keyvalues`/`projects` list + get; `services create` ×4 (web/cron/static/`--env-var`), `postgres`/`keyvalues create`; `services`/`postgres update`; `restart`, `postgres suspend`/`resume`; `deploys list`/`create`, `jobs list`, `services instances`; `services`/`postgres delete`; `environments` (correct 404); `logs`. Residual non-passes were all non-auth environment artifacts (the same `namespaces "tea-…" not found` control-plane provisioning gap of `.pm/w2/026.md`, KeyValue datastore-namespace resolution in the hand-seeded tenant, interactive confirm prompts). **Note:** `cli-compat.sh verify` authenticates with `client_credentials` tokens, which are capability-**exempt**, so it structurally cannot exercise this human-token op-class path — only the device-flow login (`scripts/bex-cli-auth-e2e.sh`, which runs `workspaces` + `services`) does. Guards: `TestDeviceGrantScopeCoversEveryOpClass`, `TestRenderCLITokenClearsEveryOpClassEndToEnd`, and the env-gated live `TestRenderCLIBinaryAgainstLiveServer`.
- **CLI:** `render-oss/cli` **v2.24.0** (`fe8a6188119ee1a53dcf3e5c19f6a5302e840c3f`), built unmodified from the Go-module pin, driven through [`scripts/cli-compat.sh`](../scripts/cli-compat.sh) against **dev-2** on **2026-08-19** (w2/m79). Auth/list/env legs passed (`login`, `whoami`, `workspace current`, `workspaces`, `projects`, missing-resource not-found shapes, environments). Create legs (`services`/`postgres`/`keyvalues`) failed with harness `namespaces "tea-…" not found` — control-plane tenant-namespace provisioning on this CAPD stack, **not** a CLI wire regression (filed `.pm/w2/026.md`). Prior production/dev supplements below remain historical evidence under older pins unless restated.
- **Upstream surface delta v2.22.0 → v2.24.0 (help-graded):** new root `completion` (bash/zsh/fish/powershell) — `[x]` local help/smoke; `RENDER_CLI_CONFIG_DIR` preferred over `RENDER_CLI_CONFIG_PATH` — launcher already maps `BEX_CLI_CONFIG_DIR`; Windows archive ships `render.exe` (release note only); `kv create|update --persistence-mode` and existing `pg` disk flags unchanged in contract. No Bex-native command-name collisions (`code`/`glm`/`muse`/`kimi`/`deepseek`). `isRootVersionRequest` body unchanged vs `lego/cli/internal/update`.
- **CLI:** (historical) `render-oss/cli` v2.21.0, built unmodified from a separate pinned upstream checkout (or a checksum-verified release binary), driven only through [`scripts/cli-compat.sh`](../scripts/cli-compat.sh) (Hydra `client_credentials` token exchange per call, `RENDER_HOST` → local bex-api). That dev-9 baseline never touched production; the named supplements below are separately scoped. The repository's `lego/cli/` path is the distinct import-based `bex` launcher, not an upstream checkout.
- **Target:** the isolated local **dev-9** environment (`.pm/w9/dev-9`, bex-api at `:54090`), current `lego/backend` HEAD, on **2026-07-18**.
- **Method:** the maintained regression suite `scripts/cli-compat.sh verify` (whole-shape `checkFields` assertions over the core families), the cleanup-safe `services-parity-verify` baseline/configured legs, and a full sweep of the remaining command tree. Each item was graded on the unmodified CLI's exit status and the wire shape read back via `-o json` and raw REST `GET`. The exact service flag contract and redacted POST/PATCH captures are in [`cli-services-create-update.md`](render-artifacts/cli-services-create-update.md).
- **Operator-correctness schema supplement (w2/m60):** the installed, unmodified v2.21.0 CLI's `services update --help` and `postgres update --help` were re-read on **2026-07-19**. Service update has no `--type`; Postgres update has both `--plan` and `--disk-size-gb`. Focused backend regression sends the CLI's exact `PATCH {"diskSizeGB":…}` wire shape: growth persists, a lower value returns the named 400 envelope, and the resource remains at its accepted size. Operator/envtest separately proves a plan downgrade preserves that size. This is a command-schema plus API regression supplement, not a new live dev-9/production CLI run; the previously graded create/update/delete baseline remains the live-client evidence.
- **Service-disk supplement (w1/m86, 2026-08-23):** ADR082 shipped persistent service disks on REST/GraphQL/MCP/dashboard, and the unmodified v2.24.0 CLI's `--help` tree was re-read for a matching surface: **there is none.** Render's own CLI has no `disks` command and no service-level disk flag — `--disk-size-gb` exists only on `postgres create|update` (the datastore's own storage, already graded above) and `services create|update` has no disk flag at all. So this feature adds **no** row to this checklist: there is nothing in the client to grade, and the absence is Render's, not a bex gap. The parity boundary for disks is the REST shape the CLI _would_ call if it ever grows one — `/v1/disks` in Render's documented request/response shapes, covered by the backend suite. Re-check this bullet when the CLI pin moves: a future `render disks` command would make this a real row.

- **Durable-delete and Key Value credential supplement (w2/m61):** the unmodified v2.21.0 CLI delete commands remain plain service/Postgres/Key Value id-or-name operations with no bex-only flag or cleanup handshake. Focused REST/GraphQL/MCP regressions preserve their existing acknowledgement shapes while subsequent reads show `deleting` until the finalizer has proved required cleanup absent, then NotFound. Key Value connection-info returns conflict rather than a split credential during rollout. This is an operator/backend regression supplement, not a new live production CLI run; the previously graded unmodified-CLI delete rows remain the live-client evidence.
- **OpenAPI request-boundary supplement (w7/m49):** the existing unmodified-v2.21.0 request fixtures across services, deploys, Postgres, Key Value, environment groups, projects, custom domains, jobs, registry credentials, and logs run through the authenticated 119-operation Render-contract intersection. Focused boundary tests also replay the accepted CLI-shaped bodies, compatibility defaults, and bex `dryRun`/`confirm` extensions while proving invalid input has no side effect. This is repository regression evidence, not a new live CLI run; the live grades below are unchanged. Bex deliberately rejects undeclared query/body input more strictly than Render's open schemas, as documented in [the OpenAPI boundary contract](render-artifacts/openapi-request-validation.md).
- **Canonical-route retirement supplement (w1/m55):** the official CLI's existing request fixtures already use Render's canonical `/v1/services`, `/v1/postgres`, `/v1/key-value`, and `/v1/registrycredentials` routes, so their focused backend regressions remain green after bex removed its non-Render public aliases. Dedicated negative tests now require those retired aliases to return `404`; the internal control-plane listener's unrelated `/v1/apps` route remains available only on port `8091`. This is repository regression evidence, not a new live CLI run; the live grades below are unchanged.
- **Nested-image/delete-convergence correction (w5/m49, production accepted 2026-07-21 UTC):** production proved that v2.21.0 serializes `image.ownerId:""` on image create and update. The strict REST adapter now accepts and authorizes that Render-declared field after OpenAPI validation. First-build deletion now interrupts synchronous build waits, inventories with the required build-namespace RBAC, repairs least-privilege registry authority, proves repository absence before revocation, and quiesces Ingress/cert-manager TLS producers before Secret deletion. The deployed unmodified CLI passed image create/update/delete and immediate repo-build delete to raw GET 404 plus official-CLI list absence inside the five-minute bound. The original stuck fixture and the final acceptance prefix both had zero Kubernetes/build/TLS/credential residue; Zot htpasswd/ACL/config and a builder-authenticated catalog also had zero matches. Sanitized evidence is in [the m49 diagnosis](../.pm/w5/done/m49/evidence/2026-07-20-diagnosis.md).
- **Configured service pass:** a disposable local OpenBao plus auth-enabled persistent Zot augmented dev-9 for the second service run. It proved CLI env vars, secret files, create/update registry-credential binding, a genuinely private kubelet pull, and native cron commands.
- **Postgres create supplement (w6/m38):** [`scripts/postgres-create-cli-smoke.sh`](../scripts/postgres-create-cli-smoke.sh) drove the same unmodified CLI against isolated **dev-6** and a real local CNPG cluster on **2026-07-18**. It proved both custom names, database-only and user-only independent defaults, the CR intent, generated Secret metadata, and `current_database()`/`current_user` inside PostgreSQL. It also proved that unsupported Datadog create/update flags fail with a named 400 and leave no resource behind.
- **Postgres psql supplement (w7/m46):** [`scripts/psql-compat-verify.sh`](../scripts/psql-compat-verify.sh) drove Render's checksum-verified, unmodified v2.21.0 release against isolated **dev-7**, real CNPG, a configured `BEX_DB_DOMAIN`, and pg-sni-proxy on **2026-07-18**. Both opaque id and exact name executed `SELECT 1 AS bex_psql_probe;` through real `psql` 18.4 over the external TLS/SNI route; the caller's public `/32` passed the CLI's own allow-list gate, the absent-IP negative case was refused before connect, and the disposable database and local credential state were removed. Redacted markers and release hashes are in [the psql CLI artifact](render-artifacts/psql-cli.md).
- **Production SSH supplement (w6/020):** [`scripts/ssh-verify.sh`](../scripts/ssh-verify.sh) drove the checksum-verified, unmodified v2.21.0 release through `https://api.bex.co` and the public `ssh.bex.co:22` gateway in a real PTY on **2026-07-18**. Service name, service id with the interactive **Any instance** selection, and complete instance id all reached the asserted running container and runtime environment; the verifier also re-proved the pinned host key, raw any/exact OpenSSH, PTY resize, exit status, restart/redeploy closure, suspended/free/stale/shell-less/unknown/deleted denial, and deleted its disposable key, service, and Scale workspace. The broader viewer/foreign/static/cron denial matrix remains captured in the prior [production acceptance](../.pm/w2/done/m39/evidence/2026-07-17-production-acceptance.md).
- **Production Key Value CLI supplement (w2/m57):** [`scripts/cli-compat.sh kv-cli-verify`](../scripts/cli-compat.sh) drove the unmodified v2.21.0 release through `https://api.bex.co` and the public `*.kv.bex.co:6379` TLS/SNI edge on **2026-07-18**. Under an automated PTY, opaque-id `PING`/`SET` and display-name `GET`/`DEL` all passed through the CLI's own list/item/connection-info path with an exact source `/32`; the verifier removed its key and Key Value, then revoked its API key and deleted its workspace. Sanitized results are in the [production acceptance evidence](../.pm/w2/m57/evidence/2026-07-18-kv-cli-acceptance.md).
- **Production durable-logs supplement:** [`scripts/cli-logs-verify.sh`](../scripts/cli-logs-verify.sh) drove the unmodified v2.21.0 CLI through `https://api.bex.co` against the deployed Loki + log-shipper on **2026-07-18**, closing the store-only `logs` filter rows the Loki-less dev-9 baseline could only grade `503`. A disposable busybox web-service fixture planted one JSON error line, one JSON info line, and one plaintext line at boot, then served a `200` on `/` and a `404` on a nonce path through the public edge; `render logs -o json` then proved every filter with a positive match AND a negative exclusion — the `app`/`request` split clean both ways, `--level error` isolating exactly the planted line (`critical` an honest empty), `--path`/`--host` matching only the probe values, `--method GET` positive with `POST` empty, and `--status-code` exact `404`/`200` exclusion both ways plus the `4xx` class shorthand. The fixture is deleted on exit; the raw REST/MCP halves of the same durable-store paths remain covered by [`scripts/logs-verify.sh`](../scripts/logs-verify.sh).
- **Base dev-9 does not run** (so paths needing them are marked accordingly, distinguishing a real bex gap from an environment limit): build infra (kpack/zot), OpenBao (env-vars/secret-file store), the registry-credential store, Loki (durable log store — but see the production durable-logs supplement above), Prometheus, cert-manager, OpenFGA (authz is allow-all here), `BEX_DB_DOMAIN`/`BEX_KV_DOMAIN` (external datastore connect), and the SSH gateway.

### Real gaps found this pass (bex-side, worth filing)

- **(w2/m79)** Create-path `namespaces "tea-…" not found` on CAPD **dev-2** during `cli-compat.sh verify` with v2.24.0 — auth/list/env passed; service/Postgres/Key Value creates 500'd before any wire-shape assert. Filed `.pm/w2/026.md` (harness/control-plane, not a CLI pin bug).
- ~~**`keyvalues update --memory-policy` / `--ip-allow-list` / `--clear-ip-allow-list` are silent no-ops**~~ — **fixed (w7/m45)**: `KeyValuePatch` now carries `MaxmemoryPolicy` + `IPAllowList` and `handleUpdateKeyValue` decodes them, so the CLI's update flags mutate the store; GraphQL `setKeyValueMaxmemoryPolicy` + MCP `update_key_value` (w1/m71's fold of `set_key_value_maxmemory_policy`/`set_key_value_ip_allow_list`) bring the programmatic surfaces to parity. Dashboard memory-policy editing after create has also since shipped (`cbf49951` / `d2b1de3e`: `KeyValueMaxmemoryPolicySection` edit-in-place on the Key Value detail page, w7/007).
- _(upstream CLI, not bex)_ **The v2.21.0 SSH instance picker discards an exact-instance selection**: both callbacks pass the service id instead of their `instanceID` argument. **Any instance** therefore works through the picker, and the supported complete-instance-id command argument works for exact targeting; the verifier does not patch the CLI or falsely credit the broken menu path.
- _(upstream CLI, not bex)_ **`skills list` / `skills update` panic** (nil-pointer) in every non-TTY output mode.

### Intended divergences (not bugs)

- `--region` is accepted but **platform-stamped** from `BEX_REGION` (`local-capd` in dev-9, `fsn1` in production); the submitted hint is not persisted. Both truthful installation values sit outside the CLI's closed Render-region enum, so a bare `services create --from` clone fails client-side and needs an explicit `--region`.
- `--previews` is rejected platform-wide (`400 "not supported by this platform"`).
- Postgres Datadog forwarding is not implemented. Supplying `--datadog-api-key` or `--datadog-site` on create or update returns a named 400; bex never accepts or persists the credential.
- `services update --runtime` is rejected inside the unmodified CLI by design (`cannot switch runtimes via the CLI`) before any bex request.
- `autoDeployTrigger: checksPass` (deploy only after CI checks pass) is rejected with a named 400 (w5/m53). bex maps its boolean `spec.autoDeploy` onto Render's `autoDeployTrigger` `commit`/`off` (and the legacy `autoDeploy` `yes`/`no`), accepting either on create/update with `autoDeployTrigger` taking precedence; it has no GitHub-checks integration, so the checks-gated trigger is a documented non-goal, not a silent no-op.

> Regenerate the command tree with `render <subcommand> --help`. Re-run the graded baseline with `scripts/cli-compat.sh verify`. Grouping mirrors the CLI's own `render --help` sections.

The interactive-only Key Value client has a separate, opt-in full-edge verifier: set the production-equivalent `BEX_API_URL`, `HYDRA_PUBLIC_URL`, `CLI_KEY_ENV`, and the runner's explicit `BEX_KV_VERIFY_ALLOW_CIDR`, then run `scripts/cli-compat.sh kv-cli-verify`. It provisions and always deletes one source-restricted public Key Value. The verifier launches the unmodified v2.21.0 CLI under an automated pseudo-terminal with explicit `--output interactive`; one-shot Redis arguments after `--` let the child and TUI exit without human input. This is deliberately excluded from the dev-9 baseline because it requires public Key Value DNS, TLS, and the SNI proxy. Production acceptance on 2026-07-18 exposed and repaired three real edge defects in sequence: the datastore wildcard was Cloudflare-proxied instead of DNS-only, Hetzner PROXY protocol was absent so exact source allowlists saw the load balancer, and `redis-cli` needed an explicit `--sni` hostname. The final opaque-id and display-name legs passed end to end; [`scripts/datastore-dns-cloudflare.sh`](../scripts/datastore-dns-cloudflare.sh) and the proxy tests retain the deployed prerequisites.

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
    - [x] payment-required failures remain an ordinary Render API error (HTTP 402 with `id`/`message`); the actionable message names `create_billing_checkout_session`, so the unmodified client does not receive an undecodable body
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
  - [x] `keyvalues delete <id|name>` — unchanged CLI contract; durable cleanup remains internal (w2/m61)
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
  - [x] `--level <levels>` — durable-logs supplement: a planted JSON `error` line is isolated exactly; an unmatched level is an honest empty. (The CLI's own `--level` enum has no `unknown`, so bex's honest plaintext bucket is reachable over REST only — upstream flag shape, not a bex gap.) Dev-9 (no Loki) answers `503`
  - [x] `--type <types>` — `app` works live even without the store; durable-logs supplement proved the `app`/`request` split clean in both directions
  - [x] `--host <hosts>` — durable-logs supplement: matches only the probe host; an absent host is an honest empty. Dev-9 (no Loki) answers `503`
  - [x] `--status-code <codes>` — durable-logs supplement: exact `404`/`200` exclude each other's probe lines, and the `4xx` class shorthand matches. Dev-9 (no Loki) answers `503`
  - [x] `--method <methods>` — durable-logs supplement: the GET probes match; a never-sent method is an honest empty. Dev-9 (no Loki) answers `503`
  - [x] `--path <paths>` — durable-logs supplement: matches only the probe path; an absent path is an honest empty. Dev-9 (no Loki) answers `503`
  - [x] `--task-id <ids>` — accepted as a filter
  - [x] `--task-run-id <ids>` — accepted as a filter
- [x] **`postgres`** (alias `pg`) — manage Render Postgres databases
  - [x] `postgres create` — id/name/ipAllowList wire shape correct (description persists)
    - [x] payment-required failures use the same CLI-decodable 402 envelope as Service and Key Value creates
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
    - [x] `--plan` — compute changes independently; an operator regression proves downgrade cannot reduce the accepted disk high-water mark
    - [x] `--disk-size-gb` — grow succeeds; shrink returns a named 400 and leaves intent/state unchanged (w2/m60 exact PATCH regression)
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
  - [x] `postgres delete <id|name>` — unchanged CLI contract; backup purge is terminally observed (w2/m61)
- [x] **`restart <resourceID>`** — restarts by typed id (`restart <resourceID>` is the CLI's only documented form)
- [x] **`services`** — list services and datastores; bare `services` lists them
  - [x] `-e, --environment-ids <ids>` — filter list by environment IDs
  - [x] `--include-previews` — accepted list flag
  - [x] `services create` — returns the `{service,deployId}` record; raw `dashboardUrl` is `…/<type>/<srv-id>` (`/web/`, `/cron/`, `/static/`); a cardless paid create uses Render's declared 402 `error` schema and keeps the checkout instruction in `message`
    - [x] `--name`
    - [x] `--type` — `web_service`, `cron_job`, `static_site` all accepted
    - [x] `--runtime` — round-trips (build not exercised in dev-9)
    - [x] `--repo` — round-trips (build not exercised)
    - [x] `--branch`
    - [x] `--image` — exact generated `image.ownerId:""` passes the composed server and the deployed production API (w5/m49)
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
    - [x] `--env-var KEY=VALUE` — exact configured App readback
    - [x] `--secret-file NAME:PATH` — exact configured OpenBao readback; content never logged
    - [x] `--registry-credential <cred>` — exact credential metadata plus authenticated private-image pull to `Running`
    - [x] `--ip-allow-list cidr=…,description=…` — ordered CIDR **and description** round-trip
    - [x] `--build-filter-path <path>`
    - [x] `--build-filter-ignored-path <path>`
    - [x] `--maintenance-mode` — requires a paid plan (`400` on free)
    - [x] `--maintenance-mode-uri <uri>`
    - [x] `--max-shutdown-delay <seconds>`
    - [x] `--environment-id <id>`
    - [~] `--from <serviceID>` — clones fine, but needs an explicit `--region` (CLI re-validates the source's `local-capd`)
  - [x] `services update <service>`
    - [x] workload type is intentionally not an update flag — current CLI schema matches Render's immutable-type contract; bex Blueprint and CRD admission reject the same transition
    - [x] `--name` — rename
    - [x] `--plan`
    - [~] `--runtime` — upstream CLI guard; exits before any request (`cannot switch runtimes via the CLI`)
    - [x] `--repo`
    - [x] `--branch`
    - [x] `--image` — exact generated `image.ownerId:""` PATCH passes locally and against the deployed production API (w5/m49)
    - [x] `--build-command`
    - [x] `--start-command`
    - [x] `--pre-deploy-command`
    - [x] `--cron-command` — exact configured native-cron replacement
    - [x] `--cron-schedule`
    - [x] `--health-check-path`
    - [x] `--auto-deploy`
    - [-] `--previews` — `400 "not supported by this platform"`
    - [x] `--publish-directory` — on `static_site`
    - [x] `--root-directory` — on repo services (image services correctly `400`)
    - [x] `--registry-credential` — distinct credential A→B replacement plus authenticated rollout
    - [x] `--ip-allow-list cidr=…,description=…` — ordered CIDR **and description** replacement
    - [x] `--build-filter-path <path>`
    - [x] `--build-filter-ignored-path <path>`
    - [x] `--maintenance-mode`
    - [x] `--maintenance-mode-uri <string>`
    - [x] `--max-shutdown-delay <int>`
  - [x] `services instances <serviceID>` — decodes live pod ids; suspended service returns `[]`; unknown id fails not-found
  - [x] `services delete <serviceID>` — acknowledgement shape is unchanged; production image deletion and immediate first-build deletion reached GET 404 + official-CLI list absence inside five minutes, with zero cluster/Zot residue (w5/m49)
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

- [x] **`login`** — recognizes `RENDER_API_KEY` (already-authenticated short-circuit); full browser/device flow verified separately in production. **w8/m27 compatibility:** the official launcher stays unmodified and platform-marked (`bex.co/platform-client`). The dispatch-time scope matrix (w8/m27, [docs/ADR069-security-review-round14.md](ADR069-security-review-round14.md)) requires **every** human OAuth token — platform clients included — to carry a granular capability; a token with no granular scope is refused with `403`, which the CLI surfaces as _"you are not allowed to take this action."_ So bex's own device-flow adapter (`internal/cliauth`, **not** a CLI fork) requests `openid offline_access bex.read bex.write bex.sensitive` on the unmodified CLI's behalf — the exact granular set the Render-CLI Hydra client is provisioned to allow with `skip_consent` (`scripts/auth-bootstrap-client.sh`) — so the minted platform-client token satisfies read/write/sensitive/mint op classes. `bex.api` remains a compatibility alias only and is never requested. `TestDeviceGrantScopeCoversEveryOpClass` pins the coverage; do not trim the adapter's requested scope back to identity-only. Do not fork the CLI.
- [x] **`logout`** — drives the OAuth revoke path; the credential's `client_credentials` grant fails afterward
- [x] **`whoami`** — reports the key-minting user's email (Kratos-admin lookup)
- [x] **`workspace`** — manage the CLI's active workspace
  - [x] `workspace current` — returns the active tenant
  - [x] `workspace set <id>` — set→persist→read round-trip verified

## Session

- [x] **`kv-cli [id|name]`** — automated PTY plus `scripts/cli-compat.sh kv-cli-verify` proved public `rediss://` by opaque id (`PING`/`SET`) and display name (`GET`/`DEL`) through the unmodified v2.21.0 CLI; see the [2026-07-18 production evidence](../.pm/w2/m57/evidence/2026-07-18-kv-cli-acceptance.md)
- [x] **`pgcli [id|name]`** — a bounded pseudo-terminal crosses the real interactive-only guard, resolves one disposable public bex Postgres by opaque id and exact name, preserves passthrough flags, and runs `SELECT 1 AS bex_pgcli_probe;` through real pgcli 4.5.0 twice; the negative piped guard, redacted shim handoff, SQL markers, and cleanup proof are recorded in [the pgcli CLI artifact](render-artifacts/pgcli-cli.md)
- [x] **`psql [id|name]`** — the checksum-verified, unmodified v2.21.0 CLI resolved one disposable public Postgres by opaque id and exact name, passed its client-side public `/32` allow-list gate, consumed the external `sslmode=require` connection string through pg-sni-proxy, and returned `SELECT 1 AS bex_psql_probe;` through real `psql` 18.4 twice. Unknown-name, absent-allowlist, endpoint-convergence, credential-redaction, and cleanup behavior are asserted by the live verifier and its hermetic regression ([the psql CLI artifact](render-artifacts/psql-cli.md))
  - [x] `-c, --command <SQL>` — both id and exact-name selectors returned the asserted probe row through the real non-TTY client path
- [x] **`ssh [serviceID|serviceName|instanceID]`** — live-proven through the public production gateway with the unmodified v2.21.0 CLI: service name, service id → **Any instance**, and complete instance id all reached the asserted running container; the upstream exact-instance picker bug is documented above, while the direct instance-id form proves exact targeting ([ADR035](ADR035-ssh.md))
  - [-] `-e, --ephemeral` — deliberate bex non-goal (flag parses)
  - [-] `--plan <string>` — tied to `--ephemeral` (non-goal)

## Management

- [~] **`blueprints`** — manage Blueprints (infrastructure as code)
  - [x] `blueprints validate <render.yaml>` — the unmodified v2.21.0 CLI successfully decoded both a valid `hello-go/render.yaml` response and an invalid `autoDeployTrigger: checksPass` response from freshly deployed `api.bex.co` on 2026-08-03 (the latter carried `services[0].autoDeployTrigger` plus source line/column). **w8/m19 production re-grade (2026-08-16):** after the m19 image rollout, the same unmodified CLI and the raw multipart response accepted one custom `.yaml` fixture containing static `buildCommand`, monorepo `dockerContext`, and private-image `image.creds.fromRegistryCreds.name`; the response was `valid:true`. This closes the CLI validate transport/decoding grade. It does not turn the Blueprint feature's overall partial row into blanket parity: the capability registry still intentionally rejects its documented subset, and the separate live apply legs are not evidence for this CLI command.
- [x] **`environments <projectID>`** — decodes the RC15 cursor envelope into real values; unknown project fails not-found
- [x] **`projects`** — lists projects in the active workspace

## Additional commands

- [x] **`docs`** — unmodified `render docs` opens `render.com/docs` (client-side; no bex call). The `bex` launcher overrides this command to open the Bex CLI guide on GitHub (`lego/cli/internal/branding.DocsURL`); that is a launcher branding overlay, not a change to the unmodified-CLI grade above.
- [x] **`completion [bash|zsh|fish|powershell]`** — **w2/m79 / v2.24.0:** generates shell autocompletion scripts client-side (no bex call); help + local smoke on the unmodified pin
- [~] **`ea`** — early-access surfaces; `ea sandbox` create/list/stop (w3/m32) **and exec** (w3/m33) all ship on the gVisor substrate, `ea objects` still out of the compatibility target
  - [-] `ea objects list` — bex `/v1/objects` → `404` (`--local` works client-side)
  - [-] `ea objects put` — `404` (`--local` works client-side)
  - [-] `ea objects get` — `404` (`--local` works client-side)
  - [-] `ea objects delete` — `404` (`--local` works client-side)
  - [x] `ea sandbox create` — verified with the real CLI (v2.21.0): `POST /v1/sandboxes` → `201`. The CLI sends only `--plan` (no template flag), so an empty template resolves to `base`, and it sends `ownerId` in the body (bex reads body-or-query). `plan`/`region`/`timeoutSeconds` and the effective `{default:"deny-all"}` policy round-trip from reserved runtime metadata. `allow-all` is a deliberate named 400 (`SANDBOX_NETWORK_POLICY_UNSUPPORTED`) because bex will not claim an unenforced policy. Runs on the gVisor substrate in the caller's `<ws>-sandbox` namespace.
  - [x] `ea sandbox exec` — verified on prod with the real CLI (w3/m33): `POST /v1/sandboxes/{id}/exec` streams the command's stdout/stderr + exit as SSE (`event: output`/`exit`). `render ea sandbox exec <id> -- sh -c 'uname -r'` returned `4.19.0-gvisor` from inside the gVisor sandbox. Runs via k8s `pods/exec` confined to the isolated ssh-gateway (bex-api authorizes + reverse-proxies, never gains `pods/exec`); execd's gRPC API and the CLI's older two-step token flow are both unused.
  - [x] `ea sandbox list` — verified with the real CLI: `GET /v1/sandboxes` → `200`, returned as the CLI's cursor-paginated `[{sandbox, cursor}]` envelope. Results are additionally filtered to the caller's durable owner metadata; workspace admins are the explicit override.
  - [x] `ea sandbox stop` — verified with the real CLI: `POST /v1/sandboxes/{id}/terminate` → `204`; a foreign/ownerless id returns the same `SANDBOX_NOT_FOUND` 404 as an absent id.
- [~] **`skills`** — manage Render agent skills for AI coding tools (client-side; no bex dependency)
  - [x] `skills install` — works; `--dry-run` reports intended changes without writing
  - [ ] `skills list` — **upstream CLI panic** (nil-pointer) in every non-TTY output mode
  - [ ] `skills update` — same upstream non-TTY panic; no `--dry-run`
  - [x] `skills remove` — works
- [x] **`help [command]`** — CLI builtin, no bex call
