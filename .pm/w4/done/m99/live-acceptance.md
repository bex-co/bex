# m99 live acceptance — 2026-09-11 UTC

Implementation: `818de80b8797ba095faef0132ff432cf376b9f59`. Installed executable `/opt/homebrew/bin/bex`, Bex v0.2.1 / Render CLI v2.27.0. Human QA device credentials were isolated in a private config; workspace `tea-d98210cbbpdc73dcrkvg`. Only ledger-owned free fixtures were mutated.

## Long name, initial deployment, and sleep/wake — passed

The production policy's variables, validations, match conditions and failure policy match the shipped source, apart from additional blank lines introduced by YAML rendering (observed resourceVersion `95271420`). This observation does not claim the new backend image was deployed: Blueprint false validation still failed at this point.

```sh
BEX_HOST=https://api.bex.co/v1/ BEX_WORKSPACE=tea-d98210cbbpdc73dcrkvg \
BEX_CLI_CONFIG_PATH=<private-config> BEX_NO_UPDATE_NOTIFIER=1 \
bex services create --name qa-20260911-ba5a8ffb-control \
  --type web_service --runtime image --image docker.io/traefik/whoami:v1.11.0 \
  --env-var WHOAMI_PORT_NUMBER=3000 --env-var QA_MARKER=m99 \
  --plan free --region frankfurt --num-instances 1 --health-check-path / \
  --confirm -o json
```

Create: exit 0, 1.28s, empty stderr, `srv-dahrfta7uadc739e9atg`, disabled `autoDeploy:no` / `autoDeployTrigger:off`, created `08:12:37Z`. `bex deploys list srv-dahrfta7uadc739e9atg -o json` exited 0 and returned initial deploy `dep-dahrfta7uadc739e9au0`, trigger `create`, started `08:12:38.111442Z`, finished `08:12:59.278552Z`, status `live`.

The tenant App `tea-d98210cbbpdc73dcrkvg-qa-20260911-ba5a8ffb-control` had UID `151c9523-535c-45b8-9e4f-154f60f5e0d4`, phase `Running`, and Ready reason `Deployed` (`1/1 replicas ready`). Its actual 63-character alias was:

```text
name: bex-activator-tea-d98210cbbpdc73dcrkvg-qa-20260911-ba5-9fdc7041
type: ExternalName
target: bex-activator.bex-system.svc.cluster.local
port: 8888
controller owner: app.bex.co/v1alpha1 App
owner name: tea-d98210cbbpdc73dcrkvg-qa-20260911-ba5a8ffb-control
owner UID: 151c9523-535c-45b8-9e4f-154f60f5e0d4
```

`GET https://qa-20260911-ba5a8ffb-control.onbex.co/` returned HTTPS 200 at `08:13:27Z`, with hostname `tea-d98210cbbpdc73dcrkvg-qa-20260911-ba5a8ffb-control-6fd5r29br`. The read-only GraphQL query `service(id:"srv-dahrfta7uadc739e9atg") { id phase autoDeploy autoDeployTrigger idleTTLSeconds }` plus `deploys(serviceId:"srv-dahrfta7uadc739e9atg") { id status }` returned HTTP 200, no errors, `Running`, `false`, `off`, and the same initial `live` deploy.

For sleep/wake, same-authority REST PATCH on this owned ID set only `serviceDetails.idleTTLSeconds:30` (HTTP 200). No platform or pre-existing resource setting was changed. After the idle window, Kubernetes reported `Hibernated`, Deployment replicas `0`, ready replicas `0`; last-active was `08:13:37Z`, observed at `08:14:28Z`. Public requests then returned:

| UTC | HTTPS | Observation |
| --- | --- | --- |
| 08:14:53 | 503 | `Retry-After: 5`, `{"error":"service hibernated","retryAfter":5}` |
| 08:14:59 | 503 | Same retry response |
| 08:15:04 | 200 | New hostname `tea-d98210cbbpdc73dcrkvg-qa-20260911-ba5a8ffb-control-6fd54kn46` |

GraphQL again reported `Running` and the original deploy still `live`. Collected events were normal scale-up, scale-down and certificate creation events; no admission-denied event was present.

Cleanup: `bex services delete srv-dahrfta7uadc739e9atg --confirm -o json` exited 0 in 1.97s. Detail returned 404 immediately; instances initially still listed the draining pod, then returned 404 on the next bounded check. Final CLI list omitted the ID; the owned Service/Pod selector returned no objects. The transient instance list was not mistaken for completed teardown.

## Still required

Short-name post-fix initial deployment and HTTPS control; explicit `--auto-deploy=false` against the deployed backend; post-fix enabled/invalid controls; final baseline reconciliation and isolated session cleanup. The replacement deployment run `34577793564` at `d0f17feb3ece1ce20c8c85384e8ee68ff4b2e1d3` includes the implementation commit. The original queued run `34577179164` was superseded, not restarted. Do not close the milestone until all remaining live checks pass.

## Deployment-gate timing failure

Run `34577793564` failed operator job `103195228834` in the existing `TestWakeRestoresAppServiceAsIngressBackend`, before the Ginkgo suite (all 158 Ginkgo specs passed). The test's one-second idle TTL combined with its whole-second RFC3339 activity stamp let the App expire during the wake assertions. Local `go test ./internal/controller -run '^TestWakeRestoresAppServiceAsIngressBackend$' -count=200` reproduced four failures in 6.54 seconds with the identical wrong activator-backend result.

The test now uses the same ordinary 300-second window as its active fixture sibling. Its initial hour-old activity still forces sleep; a fresh stamp remains active through the routing assertions. Production expiration logic is unchanged. This removes the timing assumption rather than retrying or suppressing the failure.

After the test correction, all 200 targeted repetitions passed (14.51s), and the complete operator `GOWORK=off make test` suite passed.

## Final acceptance and closeout — 2026-09-12 UTC

Completed using installed `/opt/homebrew/bin/bex` v0.2.1 / pinned Render CLI v2.27.0, a fresh isolated human device login, and workspace `tea-d98210cbbpdc73dcrkvg` (`bex`). The credential helper used the existing QA configuration from the sibling checkout because this checkout has no `.env`; no credentials were copied into the repository. Workspace limits returned HTTP 200: services used 5 / limit 100, terminating 0. Only the two recorded free fixtures below were created or deleted.

The production `bex-api` Deployment had two ready replicas and image `ghcr.io/bex-co/bex-operator@sha256:831c50645511e5eb31dd6ccc0f60d79a40393f969eb9f996bd65413316e53f91`; operator, activator, gateway and static-server used the same digest. Kubernetes access was read-only. The earlier backend deployment blocker is superseded by the observed successful requests below.

### CLI create, deployment, HTTPS, and GraphQL

Commands used per-process `BEX_HOST=https://api.bex.co/v1/`, `BEX_WORKSPACE=tea-d98210cbbpdc73dcrkvg`, an isolated `BEX_CLI_CONFIG_PATH=<private-config>`, and disabled update notices/analytics. Inherited Bex/Render overrides were removed.

```sh
bex services create --name qa-20260912-923ee6-false --type web_service --runtime image --image docker.io/traefik/whoami:v1.11.0 --env-var WHOAMI_PORT_NUMBER=3000 --env-var QA_MARKER=m99 --plan free --region frankfurt --num-instances 1 --health-check-path / --confirm -o json --auto-deploy=false
```

Exit 0 in 0.45s, empty stderr; service `srv-daie61jdqjvc73e7ap9g` created `2026-09-12T05:28:38Z`, `autoDeploy:no`, `autoDeployTrigger:off`. Initial deploy `dep-daie61jdqjvc73e7apa0` (trigger `create`) reached `live` at `2026-09-12T05:29:02.63358Z`. `GET https://qa-20260912-923ee6-false.onbex.co/` returned HTTPS 200, serving hostname `tea-d98210cbbpdc73dcrkvg-qa-20260912-923ee6-false-c76dfd45t2nxn`.

GraphQL request to `https://api.bex.co/graphql`:

```graphql
{
  service(id: "srv-daie61jdqjvc73e7ap9g") {
    id
    phase
    autoDeploy
    autoDeployTrigger
    idleTTLSeconds
  }
  deploys(serviceId: "srv-daie61jdqjvc73e7ap9g") {
    id
    status
  }
}
```

HTTP 200, complete response:

```json
{
  "data": {
    "deploys": [{ "id": "dep-daie61jdqjvc73e7apa0", "status": "live" }],
    "service": {
      "autoDeploy": false,
      "autoDeployTrigger": "off",
      "id": "srv-daie61jdqjvc73e7ap9g",
      "idleTTLSeconds": 0,
      "phase": "Running"
    }
  }
}
```

```sh
bex services create --name q923ee6 --type web_service --runtime image --image docker.io/traefik/whoami:v1.11.0 --env-var WHOAMI_PORT_NUMBER=3000 --env-var QA_MARKER=m99 --plan free --region frankfurt --num-instances 1 --health-check-path / --confirm -o json
```

Exit 0 in 0.47s, empty stderr; service `srv-daie61kmg29s73d1ukc0` created `2026-09-12T05:28:38Z`, `autoDeploy:no`, `autoDeployTrigger:off`. Initial deploy `dep-daie61kmg29s73d1ukcg` (trigger `create`) reached `live` at `2026-09-12T05:29:02.744783Z`. `GET https://q923ee6.onbex.co/` returned HTTPS 200, serving hostname `tea-d98210cbbpdc73dcrkvg-q923ee6-76578b77b4-tj6jm`.

GraphQL request to `https://api.bex.co/graphql`:

```graphql
{
  service(id: "srv-daie61kmg29s73d1ukc0") {
    id
    phase
    autoDeploy
    autoDeployTrigger
    idleTTLSeconds
  }
  deploys(serviceId: "srv-daie61kmg29s73d1ukc0") {
    id
    status
  }
}
```

HTTP 200, complete response:

```json
{
  "data": {
    "deploys": [{ "id": "dep-daie61kmg29s73d1ukcg", "status": "live" }],
    "service": {
      "autoDeploy": false,
      "autoDeployTrigger": "off",
      "id": "srv-daie61kmg29s73d1ukc0",
      "idleTTLSeconds": 0,
      "phase": "Running"
    }
  }
}
```

The short control deliberately uses a seven-character service name so its platform alias remains below the truncation boundary. The earlier 2026-09-11 evidence above already proves the hashed long-name alias and sleep/wake. Both new fixtures ran in tenant namespace `tea-d98210cbbpdc73dcrkvg`. Their activator aliases were `ExternalName`, target `bex-activator.bex-system.svc.cluster.local`, port/targetPort 8888, with `controller:true` App owners (`app.bex.co/v1alpha1`):

| Alias | App owner | Owner UID |
| ----- | --------- | --------- |

| `bex-activator-tea-d98210cbbpdc73dcrkvg-q923ee6` | `tea-d98210cbbpdc73dcrkvg-q923ee6` | `16d1335b-39eb-4b92-90b5-4cb17e264680` |

| `bex-activator-tea-d98210cbbpdc73dcrkvg-qa-20260912-923ee6-false` | `tea-d98210cbbpdc73dcrkvg-qa-20260912-923ee6-false` | `df1331b1-c741-4287-95df-54df086d72bd` |

All 53 retained events for this run’s fixture names were normal; none was an admission denial.

### Negative controls and Blueprint validation

The explicit-enabled control used the first create command with fresh name `qa-20260912-923ee6-enabled` and `--auto-deploy=true`. Exit 1, empty stdout, complete stderr:

```text
Error: received response code 400: bad request: prebuilt image services cannot declare autoDeploy; remove it or deploy from repo instead
```

Same-human-authority REST `POST /v1/services`, complete non-secret request:

```json
{
  "name": "qa-20260912-923ee6-invalid",
  "ownerId": "tea-d98210cbbpdc73dcrkvg",
  "type": "web_service",
  "autoDeploy": "sometimes",
  "image": { "imagePath": "docker.io/traefik/whoami:v1.11.0" },
  "serviceDetails": {
    "runtime": "image",
    "plan": "free",
    "region": "frankfurt",
    "numInstances": 1,
    "healthCheckPath": "/"
  },
  "envVars": [{ "key": "WHOAMI_PORT_NUMBER", "value": "3000" }]
}
```

HTTP 400, complete response:

```json
{
  "error": "invalid request body at /autoDeploy",
  "id": "bad_request",
  "message": "invalid request body at /autoDeploy"
}
```

These negative probes created no resources. The pinned CLI accepts a Boolean flag; the invalid legacy string was tested directly through REST, not represented as CLI coverage.

`bex blueprints validate <fixture.yaml> -o json` tested:

```yaml
services:
  - type: web
    name: qa-20260912-923ee6-bp
    runtime: image
    image:
      url: docker.io/traefik/whoami:v1.11.0
    plan: free
    autoDeploy: false
```

With `autoDeploy: false`, exit 0, empty stderr, complete stdout:

```json
{
  "plan": {
    "services": ["qa-20260912-923ee6-bp"],
    "totalActions": 1
  },
  "valid": true
}
```

Changing only line 8 to `autoDeploy: true` returned exit 1, empty stderr, complete stdout:

```json
{
  "errors": [
    {
      "column": 17,
      "error": "prebuilt image services cannot declare autoDeploy; remove it or deploy from repo instead",
      "line": 8,
      "path": "services[0].autoDeploy"
    }
  ],
  "valid": false
}
```

### Cleanup and reconciliation

- `bex services delete srv-daie61jdqjvc73e7ap9g --confirm -o json`: exit 0. `GET /v1/services/srv-daie61jdqjvc73e7ap9g` returned 404 (`app not found`). Instance listing initially showed the draining pod; the subsequent `bex services instances srv-daie61jdqjvc73e7ap9g -o json` exited 1 with `404 Not Found`. Final resource list omitted the ID.

- `bex services delete srv-daie61kmg29s73d1ukc0 --confirm -o json`: exit 0. `GET /v1/services/srv-daie61kmg29s73d1ukc0` returned 404 (`app not found`). Instance listing initially showed the draining pod; the subsequent `bex services instances srv-daie61kmg29s73d1ukc0 -o json` exited 1 with `404 Not Found`. Final resource list omitted the ID.

- Kubernetes readback found zero matching App, Pod, Service or Deployment objects. No owned fixture survives.

- Baseline: 11 resources; final list: 12. Missing baseline ID: `srv-daie4tjdqjvc73e7ap3g` (a pre-existing QA fixture, never mutated by this run). New non-owned IDs: `dpg-daie64kmg29s73d1uke0`, `red-daie6hrdqjvc73e7apc0`. This shared workspace had concurrent external activity: the private command/ownership ledger contains only this run’s two creates and two deletes, plus rejected create controls and read/validation requests. No baseline resource was targeted for mutation or restoration. Therefore the final list is **not** claimed identical to the baseline.

- Isolated `bex logout --confirm -o json` exited 0. The isolated browser session was logged out, `/sessions/whoami` returned 401, and its separate browser context was closed. Private credentials, captures, kubeconfig and owned loopback processes were removed after evidence extraction. No personal CLI configuration or pre-existing browser context was changed.

### Scope of verification

This final pass supplies the missing deployed CLI/REST, Blueprint, short-name runtime, GraphQL and cleanup observations. The earlier long-name/sleep-wake live evidence and implementation checkpoint remain the evidence for those completed checks and the real API-server admission controls across static/maintenance/activator families. No new code was needed. Static/maintenance production fixture journeys and production negative admission writes were not repeated; their constructor/binding/actor/owner/target controls were exercised in the recorded actual API-server regression suite. Full suites were not rerun for this board-only closeout.

All milestone DoD checks are now supported by the combined evidence. Closed through the PM workflow; no commit or push was requested.

## Subsequent CLI QA — 2026-09-12 05:24–05:33 UTC

Requested `$qa-find-bugs-cli` sweep filed to w4, 2026-09-11 PDT. Installed `/opt/homebrew/bin/bex` v0.2.1, Render v2.27.0 / `a764810a768202704e7206eb7b87a47211fcd98e`; installed build reports `423855d5512091bb349585641937260582fab77d`, `vcs.modified=true`. Checkout `0af34ef9f8943a9c7d129cefcf6be48c1c34fb94`, initially clean main. Platform deployed revision unknown. Human device OAuth, non-TTY JSON, isolated 0700 directory and 0600 config; inherited Render/Bex overrides stripped, updates/analytics disabled. Workspace `bex` / `tea-d98210cbbpdc73dcrkvg` selected because it was the authenticated QA dashboard's active workspace. Baseline: five services, three Postgres, two Key Values; resource limits 100/25/25, zero terminating; all creates explicitly free.

### Explicit disabled image-create now passes

Common command environment: `BEX_HOST=https://api.bex.co/v1/`, `BEX_WORKSPACE=tea-d98210cbbpdc73dcrkvg`, `BEX_CLI_CONFIG_PATH=<private-config>`, `BEX_NO_UPDATE_NOTIFIER=1`.

```sh
bex services create --name qa-20260911-6a6f10-control \
  --type web_service --runtime image \
  --image docker.io/traefik/whoami:v1.11.0 \
  --plan free --region oregon --num-instances 1 \
  --env-var WHOAMI_PORT_NUMBER=3000 --env-var QA_MARKER=qa-20260911-6a6f10 \
  --health-check-path /health --auto-deploy=false --confirm -o json
```

Exit 0, 1.23s, empty stderr. Returned `srv-daie4tjdqjvc73e7ap3g`, created `2026-09-12T05:26:14Z`, `autoDeploy:"no"`, `autoDeployTrigger:"off"`, `runtime:"image"`, `plan:"free"`, `internalAddress:"qa-20260911-6a6f10-control:3000"`. Raw HTTP request/response was not captured; these are actual CLI-decoded values, not an invented wire transcript.

`bex deploys list srv-daie4tjdqjvc73e7ap3g -o json` returned initial deploy `dep-daie4tjdqjvc73e7ap40`, trigger `create`, `startedAt:"2026-09-12T05:26:17.1434Z"`, `finishedAt:"2026-09-12T05:26:32.465672Z"`, `status:"live"`. `curl --max-time 15 https://qa-20260911-6a6f10-control.onbex.co/health` returned HTTPS 200 with empty body. This proves the initial runtime, not only the create acknowledgment.

An earlier fresh process also accepted explicit false for `qa-20260911-6a6f10-web` / `srv-daie47bdqjvc73e7aoug` (exit 0, 0.55s, 05:24:45Z). That fixture incorrectly bound whoami to 8080 while Bex routes image services to 3000, so it is **not positive runtime evidence**. Its app log confirmed port 8080; it was deleted before the corrected fixture. This is the documented `w9/done/011.md` port convention, not a new bug.

Disabled image Blueprint validation also passed:

```yaml
services:
  - type: web
    name: qa-20260911-6a6f10-validate
    runtime: image
    image:
      url: docker.io/traefik/whoami:v1.11.0
    plan: free
    autoDeploy: false
```

`bex blueprints validate <file> -o json`: exit 0, 0.35s, empty stderr, complete stdout:

```json
{
  "plan": {
    "services": ["qa-20260911-6a6f10-validate"],
    "totalActions": 1
  },
  "valid": true
}
```

Validation only; no Blueprint apply or resources were created from this file. A separate invalid-runtime file returned exit 1, `valid:false`, `path:"services[0].runtime"`, line 4, column 14. Enabled auto-deploy and invalid auto-deploy enum controls remain untested in this sweep; the earlier “Still required” list is historical and is narrowed by this section. No short-name alias control was performed, so m99 remains open.

### Other supported journeys

- Human device login completed after the Kratos challenge required reauthentication. Password handling stayed inside a private adaptation of `scripts/qa-login.sh`; no admin credentials or machine tokens were used. `whoami`, `workspaces`, and isolated workspace set/current all succeeded. Initial unauthenticated `whoami` failed with exit 1.
- `projects -o json` listed three projects. `environments prj-d9dgeo0bd9nc73a0vh1g -o json` returned `null`, exit 0, for an empty project; a synthetic missing project returned exit 1 / 404. Empty CLI `null` was not misfiled as a server failure.
- `bex services update qa-20260911-6a6f10-control --health-check-path / --max-shutdown-delay 45 --confirm -o json` succeeded (1.72s), preserving the ID and returning both changed values. Its config-change deploy reached live before the next redeploy.
- `bex deploys create srv-daie4tjdqjvc73e7ap3g --wait --confirm -o json` succeeded after 26.59s. Deploy `dep-daie5ibdqjvc73e7ap6g` finished at `05:28:02.469476Z`, `status:"live"`; diagnostics stayed on stderr, final deploy JSON on stdout. HTTPS `/health` still returned 200. Previous deploys became `deactivated`.
- `bex logs -r srv-daie4tjdqjvc73e7ap3g --text 'Starting up on port' --limit 2 -o json` returned two matching startup entries; `--text qa-no-match-6a6f10` returned no output, exit 0. A synthetic missing service failed 404. Multiple JSON log objects are the pinned client's stream format, not an asserted single JSON document. No live-tail interrupt check was performed.
- Free Postgres create with custom database/user `qa_probe`, 1GB disk and this run's exact IPv4 `/32` returned `dpg-daie64kmg29s73d1uke0`; name lookup retained custom identities, and missing ID lookup failed. `bex psql <id-or-name> -c 'SELECT 1 AS qa_probe, current_database(), current_user;' -o text -- --no-psqlrc --csv -q` did **not** complete a SQL query. First failure was the known missing private server CA (`w4/done/m95`); authenticated connection-info successfully supplied the certificate. With `PGSSLROOTCERT=<private-CA-file>`, both selectors reached TLS but returned `unexpected eof while reading` over IPv6. The allowed source was IPv4-only; the subsequent IPv4 control was interrupted by OAuth rejection below. No TLS-verification downgrade, connectivity success, or new database defect is claimed.
- Free Key Value create returned `red-daie6hrdqjvc73e7apc0`, `maxmemoryPolicy:"allkeys_lru"`; name-based `--memory-policy queue` update returned a real diff to `noeviction`, and get retained it. `redis-cli` and `pgcli` were unavailable, so their sessions were skipped. No Key Value connection credentials were read.
- Static/native builds, cron/worker/private service variants, jobs, paid SSH, sandbox runtime, deploy cancel/rollback, live log tail and runtime secret-marker verification remain uncovered. No paid fixture, domain, key, project, environment, workspace or pre-existing resource was mutated.
- Two local help omissions reproduce in installed and HEAD binaries and are scheduled together in [w4/063](../063.md). No hosting implementation fix was made by this sweep.

### Auth interruption — observed, cause unverified

After earlier successes, fresh `whoami`, Key Value delete, and psql invocations returned 401 at about 05:30–05:31 UTC. The stored expiry was **2026-09-19T05:23:13Z**, and the access credential did not change locally. Exact diagnostic request: `GET https://api.bex.co/v1/users`, `User-Agent: render-cli/2.27.0`, `Authorization: Bearer <REDACTED>`. Response at `Sat, 12 Sep 2026 05:31:47 GMT`: HTTP 401, `Content-Type: application/json`, `WWW-Authenticate: Bearer resource_metadata="https://api.bex.co/.well-known/oauth-protected-resource"`, complete body:

```json
{ "error": "unauthorized", "id": "unauthorized", "message": "unauthorized" }
```

Changing only this run's local expiry to enter the pinned client's normal refresh branch did not recover access. A direct same-contract refresh diagnostic (`POST /v1/token/refresh/`, JSON `{"grant_type":"refresh_token","refresh_token":"<REDACTED>"}`) returned HTTP 400, complete body:

```json
{
  "error": "invalid_grant",
  "error_description": "The provided authorization grant (e.g., authorization code, resource owner credentials) or refresh token is invalid, expired, revoked, does not match the redirection URI used in the authorization request, or was issued to another client. The refresh token is malformed or not valid."
}
```

This does **not** prove early expiry. The pinned `pkg/command/wrapper.go` translates 401 to “token is expired”; `pkg/client/client.go` refreshes only within 24h of stored expiry and clears the local refresh token after a non-timeout refresh failure. `internal/api/auth.go` admits only active introspected grants, and `internal/cliauth/service.go:296` passes the refresh failure through from Hydra. ADR012 explicitly documents subject+client consent-chain logout (`cliauth/service.go:191`), so an independent logout/revocation could explain both failures. No revoking actor, Hydra deployment change, or actual expiry cause was observed. No new implementation bug is filed from this uncertain attribution, and no account-wide logout was invoked by this sweep.

The first diagnostic above used a shortened User-Agent. A credential-free loopback capture of the actual installed binary established `render-cli/2.27.0 (macOS - 26.5.1)`. Repeating `GET /v1/users` with that exact User-Agent at `Sat, 12 Sep 2026 05:39:05 GMT` still returned 401 and the identical complete JSON body above; a Python-default edge block is excluded.

### Cleanup proved

Every fixture had a persisted unique-name intent and returned-ID creation proof, absent from baseline. The two services were CLI-deleted by recorded ID; detail and instances subsequently returned 404, and their HTTPS endpoints returned 404. Datastore CLI cleanup hit the rejected OAuth session, so the same QA browser authority performed only these exact fallback calls after verifying the owning workspace and empty Postgres read replicas:

```text
DELETE /v1/key-value/red-daie6hrdqjvc73e7apc0 -> 204
DELETE /v1/postgres/dpg-daie64kmg29s73d1uke0 -> 204
```

Subsequent detail reads returned 404. Workspace limits returned services `used:5, terminating:0`; Postgres `used:3, terminating:0`; Key Value `used:2, terminating:0`. Final same-workspace lists matched all ten baseline IDs exactly:

```text
srv-d9bkcspg9s7c73d0n8ug
srv-d9bj8s3eg85c7390eb9g
srv-d9nqg9dcavls73fp8m2g
srv-d9ndt8hmcglc739fkp50
srv-d9e40ei9086p3l1jri30
dpg-d9nqg95cavls73fp8m20
dpg-d9rrkoc4h4mc73edurp0
dpg-d9rs3ee0ccis738kc7c0
red-d9p49kdrtmes73c34ovg
red-da4086iii7bs73drbqh0
```

Zero surviving run-owned resources and zero missing baseline resources. Datastore DNS/connection teardown was not separately measured; API finalization and zero terminating counts were observed. Access and refresh credentials were already refused; the current isolated Kratos browser session was logged out through its own logout URL (200), and its isolated context was closed. Private configs, credentials, captures and helper processes were removed. No broad CLI logout was sent because ADR012's subject+client scope could affect another session. Board changes remain uncommitted.

## Continued CLI QA — 2026-09-12 06:38 UTC onward

Second sweep requested with “continue”; same installed Bex v0.2.1 and Render v2.27.0 pin as above, fresh isolated human device login, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`, nonce `b83d2f`. Checkout advanced concurrently to `939cac901`; the production revision was not inferred from it. All ten baseline IDs above remained present. No new implementation defect is inferred from the passing journeys below.

### Postgres SQL, rename, suspend, resume — passed

Created free 1 GB Postgres 18 `dpg-daif6ojdqjvc73e7as5g` at `2026-09-12T06:38:26Z`, name `qa-20260911-b83d2f-pg`, database/user `qa_probe`, exact caller IPv4 `/32` allowlist, no replicas, pool or HA. Persisted intent and full returned-ID creation proof before further mutations. Command:

```sh
bex postgres create --name qa-20260911-b83d2f-pg --plan free \
  --disk-size-gb 1 --database-name qa_probe --database-user qa_probe \
  --ip-allow-list 'cidr=<QA_IPV4>/32,description=qa-b83d2f' --confirm -o json
bex psql dpg-daif6ojdqjvc73e7as5g \
  -c 'SELECT 1 AS qa_probe, current_database(), current_user;' \
  -o text -- --no-psqlrc --csv -q
bex psql qa-20260911-b83d2f-pg -c 'SELECT 2 AS qa_name_probe;' \
  -o text -- --no-psqlrc --csv -q
bex postgres update qa-20260911-b83d2f-pg \
  --name qa-20260911-b83d2f-renamed --confirm -o json
bex postgres suspend dpg-daif6ojdqjvc73e7as5g --confirm -o json
bex psql dpg-daif6ojdqjvc73e7as5g -c 'SELECT 4 AS after_suspend;' \
  -o text -- --no-psqlrc --csv -q
bex postgres resume qa-20260911-b83d2f-renamed --confirm -o json
bex psql qa-20260911-b83d2f-renamed -c 'SELECT 5 AS resumed_probe;' \
  -o text -- --no-psqlrc --csv -q
bex postgres delete dpg-daif6ojdqjvc73e7as5g --confirm -o json
```

Connection prerequisites: `PGSSLROOTCERT=<private per-database CA file>` acquired from this fixture's same-QA `/connection-info` response; `PGHOSTADDR=<resolved public IPv4>` retains the original hostname for TLS verification. No TLS downgrade; inherited PG variables removed. Connection URLs and credentials were not published. This certifies IPv4 with `sslmode=verify-full`; default address selection / IPv6 is not certified.

Initial SQL while API status was `creating` returned SSL EOF. CLI logs showed “database system is ready to accept connections” at `06:39:34.808Z`; subsequent ID query exited 0 in 2.08s, empty stderr, CSV `qa_probe,current_database,current_user` / `1,qa_probe,qa_probe`. Name query exited 0 in 1.99s, CSV `qa_name_probe` / `2`. Rename exited 0 in 1.03s, stable ID/database/user.

First suspend failed during its initial GET with a TLS handshake timeout (exit 1, 10.07s). A read confirmed it had not suspended before retry. Successful suspend exited 0 in 0.79s at `06:40:56.67041137Z`, `suspended:"suspended"`. After convergence the SQL negative control exited 1 in 1.23s with `Error: exit status 2: psql: error: connection to server at "<public IPv4>", port 5432 failed: SSL error: unexpected eof while reading`; stdout empty. A separate earlier query ran before successful suspension and is not evidence of a suspend failure.

Resume by the renamed selector exited 0 in 1.16s, `status:"creating"`, `suspended:"not_suspended"`, updated `06:43:29Z`. Logs showed ready at `06:43:52.316Z`. After convergence, resumed SQL exited 0 in 2.26s, CSV `resumed_probe` / `5`, empty stderr. Delete exited 0 in 1.32s, `meta.deleted:true`; subsequent CLI get exited 1, `No Postgres database with ID 'dpg-daif6ojdqjvc73e7as5g'.` Same-QA REST detail returned 404; Postgres quota returned used 3 / terminating 0.

### Static-site publish, negative flags, deletion preview — passed

```sh
bex services create --name qa-20260911-b83d2f-static --type static_site \
  --repo https://github.com/bex-co/bex --branch main \
  --root-directory examples/static-site --publish-directory . \
  --auto-deploy=false --confirm -o json
bex deploys list srv-daifaa3dqjvc73e7asf0 -o json
bex services update qa-20260911-b83d2f-static --maintenance-mode --confirm -o json
bex services delete qa-20260911-b83d2f-static -o json
bex services delete srv-daifaa3dqjvc73e7asf0 --confirm -o json
```

An initial create with `--plan free` was rejected before HTTP (exit 1, 0.13s, `failed to parse command: --plan is not supported for static_site`); no resource created. Removing that upstream-forbidden flag produced exit 0 in 5.57s, empty stderr: `srv-daifaa3dqjvc73e7asf0`, created `06:46:01Z`, owning workspace as above, `plan:"free"`, `autoDeploy:"no"`, `autoDeployTrigger:"off"`, root directory `examples/static-site`, publish path `.`, empty build command. This was a no-build static publish, with no paid plan/add-on.

Initial deploy `dep-daifaa3dqjvc73e7asfg` progressed from `update_in_progress` to `live`, created `06:46:00.596393Z`, finished `06:46:32.452486Z`; source commit `232fb116f80dff61e029edfc2631c1a6c72fae98`. HTTPS `https://qa-20260911-b83d2f-static.onbex.co/` progressed from 404 to 200 with title `bex static site` and heading `Hello from bex`. The unsupported maintenance update exited 1 in 1.37s with `--maintenance-mode is not supported for static_site` and empty stdout. These flag rejections match the pinned upstream consumer, not a Bex server defect.

Name-selector deletion preview exited 0 in 1.17s, `meta.deleted:false`, `message:"re-run with --confirm to delete"`. HTTPS still returned 200 afterward. Confirmed delete by ledger ID exited 0 in 2.19s, `meta.deleted:true`. HTTPS and REST service detail subsequently returned 404. The `/instances` diagnostic returned 200 with `[]`; static sites use the shared server, not a dedicated instance. Full cleanup status is recorded below.

Raw HTTP mutation transcripts were not captured; JSON fields above are actual CLI-decoded output. An accidental `services get` invocation was excluded from detail coverage: the pinned help exposes no such subcommand and its parent command simply lists resources. No new bug is filed for an invented command. Other uncovered journeys from the first sweep remain uncovered, including Key Value PING, SSH, jobs, sandbox and stream interruption.

### Cleanup exception — filed as w4/064

Static deletion is blocked, not merely delayed. Read-only operator logs at `06:48:42Z`, `06:50:04Z` and `06:52:48Z` reject the exact fixture's purge Job because its name is 64 characters. The fallback in `publish.PurgeJob` budgets 45 parent characters where only 44 fit. Private-overlay tests of the actual checkout builder reproduce failures at parent lengths 49, 50 and 63, with length 48 passing. [w4/064](../../064.md) contains the exact log, source diagnosis, regression acceptance and recovery ledger.

Survivor: API `srv-daifaa3dqjvc73e7asf0`, CR `tea-d98210cbbpdc73dcrkvg-qa-20260911-b83d2f-static`, UID `15c840e0-ff4c-46bf-aec6-b7de5dfa78e9`, namespace/workspace `tea-d98210cbbpdc73dcrkvg`; deletion timestamp `2026-09-12T06:47:35Z`, finalizer retained. Its endpoint is 404 and its store row is gone, but the quota slot remains terminating. No further fixtures were created; no forced cleanup or production mutations were used for diagnosis. The existing controller needs the naming fix to admit its cleanup Job. Postgres is fully finalized and all ten baseline IDs remain present.

The normal read-only kubeconfig helper failed on its missing default SSH key and two changed host keys. It did not bypass those checks. Using the existing Bex key against the third node with `StrictHostKeyChecking=yes` succeeded; the private diagnostic kubeconfig was used only for exact-fixture metadata/Jobs, filtered operator logs and the operator image digest. Observed operator image: `sha256:831c50645511e5eb31dd6ccc0f60d79a40393f969eb9f996bd65413316e53f91`.

Run-specific refresh and access tokens were revoked through the issuer's advertised `https://oauth.bex.co/oauth2/revoke` endpoint (both 200, empty body), using the public client's ID and only this private config's tokens. The revoked access token was then refused by `/v1/users` with 401. No subject-wide CLI logout was sent. This run's Kratos session logout returned 200 and subsequent whoami 401; its isolated browser context was closed. Private credentials, diagnostic kubeconfig, captures and helper processes were removed; the sanitized surviving-resource ledger is retained in 064. The new filing and evidence consolidation remain uncommitted.
