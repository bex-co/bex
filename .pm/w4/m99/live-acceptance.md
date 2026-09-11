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
