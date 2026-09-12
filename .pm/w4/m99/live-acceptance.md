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
