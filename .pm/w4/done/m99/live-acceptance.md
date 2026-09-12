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
{ service(id:"srv-daie61jdqjvc73e7ap9g") { id phase autoDeploy autoDeployTrigger idleTTLSeconds } deploys(serviceId:"srv-daie61jdqjvc73e7ap9g") { id status } }
```
HTTP 200, complete response:
```json
{"data": {"deploys": [{"id": "dep-daie61jdqjvc73e7apa0", "status": "live"}], "service": {"autoDeploy": false, "autoDeployTrigger": "off", "id": "srv-daie61jdqjvc73e7ap9g", "idleTTLSeconds": 0, "phase": "Running"}}}
```

```sh
bex services create --name q923ee6 --type web_service --runtime image --image docker.io/traefik/whoami:v1.11.0 --env-var WHOAMI_PORT_NUMBER=3000 --env-var QA_MARKER=m99 --plan free --region frankfurt --num-instances 1 --health-check-path / --confirm -o json
```

Exit 0 in 0.47s, empty stderr; service `srv-daie61kmg29s73d1ukc0` created `2026-09-12T05:28:38Z`, `autoDeploy:no`, `autoDeployTrigger:off`. Initial deploy `dep-daie61kmg29s73d1ukcg` (trigger `create`) reached `live` at `2026-09-12T05:29:02.744783Z`. `GET https://q923ee6.onbex.co/` returned HTTPS 200, serving hostname `tea-d98210cbbpdc73dcrkvg-q923ee6-76578b77b4-tj6jm`.

GraphQL request to `https://api.bex.co/graphql`:
```graphql
{ service(id:"srv-daie61kmg29s73d1ukc0") { id phase autoDeploy autoDeployTrigger idleTTLSeconds } deploys(serviceId:"srv-daie61kmg29s73d1ukc0") { id status } }
```
HTTP 200, complete response:
```json
{"data": {"deploys": [{"id": "dep-daie61kmg29s73d1ukcg", "status": "live"}], "service": {"autoDeploy": false, "autoDeployTrigger": "off", "id": "srv-daie61kmg29s73d1ukc0", "idleTTLSeconds": 0, "phase": "Running"}}}
```

The short control deliberately uses a seven-character service name so its platform alias remains below the truncation boundary. The earlier 2026-09-11 evidence above already proves the hashed long-name alias and sleep/wake. Both new fixtures ran in tenant namespace `tea-d98210cbbpdc73dcrkvg`. Their activator aliases were `ExternalName`, target `bex-activator.bex-system.svc.cluster.local`, port/targetPort 8888, with `controller:true` App owners (`app.bex.co/v1alpha1`):

| Alias | App owner | Owner UID |
| --- | --- | --- |

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
{"name": "qa-20260912-923ee6-invalid", "ownerId": "tea-d98210cbbpdc73dcrkvg", "type": "web_service", "autoDeploy": "sometimes", "image": {"imagePath": "docker.io/traefik/whoami:v1.11.0"}, "serviceDetails": {"runtime": "image", "plan": "free", "region": "frankfurt", "numInstances": 1, "healthCheckPath": "/"}, "envVars": [{"key": "WHOAMI_PORT_NUMBER", "value": "3000"}]}
```
HTTP 400, complete response:
```json
{"error": "invalid request body at /autoDeploy", "id": "bad_request", "message": "invalid request body at /autoDeploy"}
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
    "services": [
      "qa-20260912-923ee6-bp"
    ],
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
