# A configured port change retargets ready old pods to the new listener

- **Source / severity:** user-requested continuous live qa-find-bugs, w4, 2026-10-08 cycle 10, privately supplied muse.env credentials. **Major:** two supported Settings saves interrupt an already serving, disk-free Free web service for at least 12 seconds. Filing only; no product fixes in this hunt.
- **Fixture / repro:** workspace bex (`tea-d98210cbbpdc73dcrkvg`); owned `qa-20261008-c10-net-e5c1`, `srv-db3ktf4hm7os73dpdc30`, `busybox:1.37`, Free, one replica, no disk, empty/TCP health-check path. Existing Image create at port 3000; Docker Command below follows the platform's injected PORT and writes its value at process start. Wait for Live and public HTTP 200. Settings → Edit Port → 3001 → Save changes reaches Live/200 with body port=3001. Begin public GET sampling, change back to 3000, then repeat 3000→3001 after a fresh settings load. Both saves return HTTP 200 and eventually become Live, but fail public requests during the transition. No pre-deploy command, environment patch or custom domain was involved. The fixture has been deleted; recreate it to replay.

```sh
sh -c 'mkdir -p /tmp/qa-c10; printf "marker=qa-c10-net-e5c1\nport=%s\n" "$PORT" > /tmp/qa-c10/index.html; exec httpd -f -p "$PORT" -h /tmp/qa-c10'
```

- **Expected:** every request continues reaching a ready pod on its actual listening port, serving the old body until a new ready pod serves the new body. Ingress must retain a resolvable backend throughout, including asynchronous Service/Ingress/EndpointSlice updates. The supported port change still opens one user-visible config_change deploy and eventually exposes the requested port, correct injected PORT and a truthful Live state. Increasing a drain or showing Deploying does not restore availability.
- **Actual run A, 3001→3000:** accepted 08:10:17.458Z; 33 GETs, 13×200, 4×404 and 16×502. First failed sample 08:10:17.664Z; last 08:10:29.716Z; first recovery 08:10:30.292Z with port=3000. Failure-sample span 12.052 s, not a claimed precise outage boundary. New deploy `dep-db3kvqc3a6ns73b87png` subsequently Live, rev-7.
- **Actual fresh run B, 3000→3001:** accepted 08:14:35.319Z; 41 GETs, 20×200, 4×404 and 17×502. First failed sample 08:14:35.548Z; last 08:14:48.011Z; first recovery 08:14:48.587Z with port=3001. Failure-sample span 12.463 s. New config_change deploy reaches Live, rev-8.
- **Cluster evidence:** the selected snapshot at 08:14:40.657–43.656Z shows both CR-named and slug-named Services already exposing/targeting 3001 with the unchanged app-only selector. Old rev-7 pod is Running, Ready=True, not deleting, and declares containerPort 3000. New rev-8 pod declares 3001 and is Ready=False. The primary Service's EndpointSlice still admits the old pod as ready/serving but sets its destination port to 3001. The Ingress backend also names 3001. This directly explains the 502: an admitted old endpoint is dialed on a port it does not listen on. Only the primary EndpointSlice was captured; slug EndpointSlice/client behavior is unverified, although its Service spec has the same defect.
- **Capture limits / anomalies:** the four Kubernetes reads per snapshot are sequential, span roughly three seconds and are not an atomic view. Snapshot 0 straddles the mutation (old Service spec, then new endpoint port); use the stable later snapshot for causality. Ready/deletion timestamps are preserved in the evidence but are not used to pretend those reads were simultaneous. The exact Traefik configuration during the four 404 samples was not captured. A temporary unresolved numeric Service backend is the code-supported explanation for that part, explicitly an inference rather than an observed edge-config dump.
- **Passing same-path control:** Manual Deploy → Restart service → confirm at unchanged port 3001. Complete RestartServer response opens `dep-db3l47k3a6ns73b87pqg`; 41 GETs during the initial sampling window and 15 in a later window after Live all return 200 with port=3001. The API reports Running, rev-9, same port. These two windows have a gap; they are not a continuous 60-second post-Live proof. The numeric helper remains correct for these samples because old and new pods listen on the same port.
- **Root cause:** `lego/operator/internal/controller/image_network.go:96–98` produces a shared app selector and one numeric `Port`/`TargetPort` from the desired configured port. `resolveImageNetwork:36–38` uses Spec.EffectivePort for this web service. `app_controller.go:2386` updates the slug alias before dispatch, `:2495` rewrites the primary Service during rollout, `:2516–2532` picks/reconciles the desired numeric Ingress backend, and only later `:2895–2911` checks Deployment/pod rollout readiness. `appendIngressHostRoute:494–507` writes ServiceBackendPort.Number. The old pod's PORT/listener is fixed at creation; a ready old pod is not a ready endpoint for an arbitrary newly desired port.
- **Consumer / framework:** actual pinned [Kubernetes v1.34.9 endpointslice FindPort](https://github.com/kubernetes/kubernetes/blob/v1.34.9/staging/src/k8s.io/endpointslice/utils.go#L381) returns a numeric target directly, without consulting a selected pod's declaration; string targets instead resolve that pod's named container port and fail if absent. Actual pinned [Traefik v3.7.5 Ingress provider](https://github.com/traefik/traefik/blob/v3.7.5/pkg/provider/kubernetes/ingress/kubernetes.go#L575) matches an Ingress backend to the Service number or name; a missing port aborts that route's construction (`:586`, `:375–384`). Its default endpoint path (`:686–728`) matches the Service port name to EndpointSlice ports, uses that slice's numeric destination and Serving/Terminating conditions, and dials it. It cannot infer the old pod's listener. Bex pins the Traefik version in `deploy/gitops/base/values/traefik.values.yaml:79`; no dependency upgrade is needed.
- **Fix specification:** introduce one coherent serving-route projection for configured-port web/legacy-private releases, separate from the requested port. A supported mechanism is stable named Service/Ingress binding plus per-pod named target-port resolution, which allows ready old/new listeners to coexist. **Adoption is part of the fix:** current `deployment_projection.go:203–205` emits unnamed container ports; immediately changing targetPort to a string would exclude every old unnamed pod. Stage a compatibility transition that preserves numeric routing until named ports exist on the serving pods, then apply the requested-port rollout; retain accepted configuration and snapshots during that transition. Service port numbers must remain resolvable to both current and cached Ingress readers, including any temporary dual-port exposure needed for adoption. Verify the mechanism with real EndpointSlices/Traefik, not only a desired-object assertion. Do not require tenant code to listen on two ports, disable readiness, remove the old endpoint early, lengthen preStop, convert this disk-free workload to Recreate, or globally roll existing deployments on an operator upgrade.
- **Layer can express it:** corev1.ServicePort.TargetPort is IntOrString; ContainerPort.Name and Ingress ServiceBackendPort.Name already exist in the pinned Kubernetes API. A short valid name such as bex-http is representable. The shared route helper must accept a named normal-App backend while retaining numeric bindings for static-server/activator/maintenance. Preserve numeric pod probe ports and the requested PORT injection. Any needed explicit migration/serving state belongs in types plus operator codegen, not a backend import into operator. The wire API continues accepting the requested integer; desired configuration is not a claim that a listener is already serving.
- **Blast radius / aliases:** exhaustive production search: privateServiceProjection has **one production caller**, applyClusterIPService; that helper has **three production call sites** (ordinary primary Service `app_controller.go:2495`, slug alias `:3062`, private discovered-port promotion `image_network.go:135`). convergeSharedChildren has **two production calls**, ordinary dispatch `:2386` and canceled-release settling `:5754`. appendIngressHostRoute has **two production calls**, ordinary Ingress `:3487` and paired-host redirect Ingress `:3610`. Allowlist this correction to configured-port App serving routes; preserve the distinct private image-v1/configured-image branch, whose ActiveImageNetwork/ServingNetwork already describe a published revision/port set (`lego/types/v1alpha1/image_network.go:57–78`, `image_network.go:100–140`). That sibling guard is code-traced, not a live correctness claim. Keep both Service identities, immutable Deployment selector, app-id/job isolation (coordinate w4/m181), paired custom-host routing, activator wake, suspension and maintenance backend precedence. web/private are port-bearing; worker/cron have no inbound Service and static uses shared serving infrastructure. Postgres/Key Value use separate controllers. Dashboard canonical /services/:id/settings plus /web and /pserv aliases reach the same setter; REST PATCH, GraphQL SetPort and MCP update_service converge on the same Core setter. No new type or alias is added.
- **Adjacent states:** a pending/unready/failed/canceled candidate must not make a previously ready listener unreachable; migration errors retain the serving route and cannot mark the requested port Live. First deployment with no previous listener retains honest unavailable/startup handling. Manual suspension, intentional sleep and maintenance keep their existing precedence; disk-bearing Recreate downtime remains documented. Forbidden/not-found/unauthenticated/validation failures remain the existing neutral API refusals; this routing correction must not alter resource existence disclosure. Preserve rollout/drain, cancellation, rollback and snapshot semantics.
- **Dedupe / history:** open/done/blocked search and cross-workstream milestone scan found no configured-port cutover item. w4/done/m121 introduced edit/read/refusal controls and verified final port movement, not an uninterrupted transition. w1/done/m154 fixes old-process termination before endpoint drain; this failure starts while the old pod is Ready and not deleting. w4/m181 is job endpoint admission and w4/m180 is source artifact choice; neither fixes numeric listener retargeting. Targeted history and the recent code log show no fix on main; the latest pull through `8af3f8fe1` changed Postgres retirement and startup diagnostics, but kept this numeric Service/Ingress projection unchanged. Source coordinates above are updated to that checkout; the live probes began at `3d2579bec`. File as an uncovered port-transition case, not a claim that m154's SIGTERM fix reverted. Its two original DoD clauses (error-free env-only rollout through Live+60s; new value after Live) must remain verified in t002/t005; this cycle did not replay that exact env-only fixture/window. m121's whole port-family contract is carried into t002. DO_NOT_DO conflicts with no part of this bounded availability correction.
- **Unverified:** legacy private HTTP clients and slug EndpointSlices; discovered/multiple image ports; built/native web variants; HTTP health-check mode; actual failed/canceled port candidates; upgrade/adoption of old unnamed pods; paired/custom domains; replicas>1; disk-bearing Recreate; wake on a changed nondefault port; exact 404 edge config; continuous original m154 env-save window; REST/MCP port writes in this cycle. These belong in the compatibility tasks and are not extra live findings.
- **Other journey results:** CIDR deny blocks both normal and forged forwarding-header requests; exact own IPv4 /32 admits browser/terminal; UTF-8 descriptions and order survive fresh mobile reload with no horizontal overflow; invalid CIDR blocks save, clearing restores public access, and network edits mint no deploy. Port 80 is refused by client validation. Paid maintenance, autoscaling and unsupported/custom-domain mutations were not submitted. Three transient ERR_CONNECTION_CLOSED console entries from GraphQL were recorded; no reproducible separate mutation failure or 429 was observed, so none is filed from them.
- **Render:** [official deployment sequence](https://render.com/docs/deploys#zero-downtime-deploys) keeps the old instance receiving traffic until the new one is ready, then updates networking. A mutable configured service-port field is a bex extension; this comparison is with the deployment availability contract, not an authenticated Render port-edit experiment. ADR004's zero-downtime readiness+drain contract also applies to this disk-free fixture.
- **Cleanup:** owned resource REST/public URL 404, absent from fresh overview, and read-only cluster checks found no owned App/primary Service/Ingress/pods. Foreign baseline Postgres/group still return 200. Only this cycle's session revoked (whoami 401), its cookie jars removed.

## Durable probes and responses

For GraphQL: POST https://api.bex.co/graphql, Content-Type application/json, authenticated QA session. Each mutation below is the **complete matching request operation and complete matching response**, normalized out of Apollo's HTTP batch. It does not include unrelated batched operations. Read probes are complete standalone requests/responses. All fixture data is synthetic. Substituting a recreated service ID reproduces the probes.

### Run A accepted mutation and complete sampled public responses

Artifact: `.playwright-mcp/qa-c10-port-return-samples.json`.

```json
{
  "mutation": {
    "at": "2026-10-08T08:10:17.458Z",
    "request": {
      "operationName": "SetPort",
      "variables": {
        "id": "srv-db3ktf4hm7os73dpdc30",
        "port": 3000
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetPort($id: String!, $port: Int!) {\n  setPort(id: $id, port: $port) {\n    id\n    port\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "setPort": {
          "__typename": "Service",
          "id": "srv-db3ktf4hm7os73dpdc30",
          "port": 3000
        }
      }
    },
    "status": 200
  },
  "samples": [
    {
      "at": "2026-10-08T08:10:17.087Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:10:17.664Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:10:18.248Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:10:18.885Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:10:19.461Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:10:20.101Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:20.681Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:21.261Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:21.840Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:22.561Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:23.152Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:23.770Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:24.353Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:24.930Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:25.511Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:26.084Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:26.750Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:27.337Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:27.965Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:28.540Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:29.122Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:29.716Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:10:30.292Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:30.944Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:31.530Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:32.163Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:32.739Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:33.315Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:33.895Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:34.472Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:35.141Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:35.732Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:10:36.350Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    }
  ],
  "mainStart": "- main:\n  - navigation \"Breadcrumbs\":\n    - link \"Projects\":\n      - /url: /\n    - button \"qa-20261008-c10-net-e5c1\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261008-c10-net-e5c1\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Live\"':\n    - /url: /services/srv-db3ktf4hm7os73dpdc30/deploys/dep-db3kvqc3a6ns73b87png\n    - text: Latest deploy Live\n  - link \"Free\":\n    - /url: /services/srv-db3ktf4hm7os73dpdc30/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db3ktf4hm7os73dpdc30\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261008-c10-net-e5c1.onbex.co\":\n    - /url: https://qa-20261008-c10-net-e5c1.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261008-c10-net-e5c1\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-7\n  - term: Created\n  - definition:\n    - time: 4m\n  - navigation \"Settings sections\":\n    - link \"General\":\n      - /url: \"#general\"\n    - link \"Source\":\n      - /url: \"#source\"\n    - link \"Deploy\":\n      - /url: \"#deploy\"\n    - link \"Custom Domains\":\n      - /url: \"#domains\"\n    - link \"Networking\":\n      - /url: \"#networking\"\n    - link \"Image registry credential\":\n      - /url: \"#registry-credential\"\n    - link \"Notifications\":\n      - /url: \"#notifications\"\n    - link \"Port\":\n      - /url: \"#port\"\n    - link \"Health Checks\":\n      - /url: \"#h"
}
```

### Fresh run B accepted mutation and complete sampled public responses

Artifact: `.playwright-mcp/qa-c10-port-fresh-repeat.json`.

```json
{
  "mutation": {
    "at": "2026-10-08T08:14:35.319Z",
    "request": {
      "operationName": "SetPort",
      "variables": {
        "id": "srv-db3ktf4hm7os73dpdc30",
        "port": 3001
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetPort($id: String!, $port: Int!) {\n  setPort(id: $id, port: $port) {\n    id\n    port\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "setPort": {
          "__typename": "Service",
          "id": "srv-db3ktf4hm7os73dpdc30",
          "port": 3001
        }
      }
    },
    "status": 200
  },
  "samples": [
    {
      "at": "2026-10-08T08:14:34.967Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3000\n"
    },
    {
      "at": "2026-10-08T08:14:35.548Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:14:36.123Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:14:36.693Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:14:37.262Z",
      "status": 404,
      "body": "404 page not found\n"
    },
    {
      "at": "2026-10-08T08:14:37.887Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:38.460Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:39.099Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:39.669Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:40.238Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:40.810Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:41.386Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:41.961Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:42.601Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:43.177Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:43.822Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:44.401Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:44.976Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:45.556Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:46.143Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:46.797Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:47.376Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:48.011Z",
      "status": 502,
      "body": "Bad Gateway"
    },
    {
      "at": "2026-10-08T08:14:48.587Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:49.176Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:49.764Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:50.338Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:50.991Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:51.565Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:52.208Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:52.782Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:53.363Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:53.940Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:54.514Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:55.090Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:55.710Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:56.288Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:56.926Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:57.501Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:58.079Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:14:58.655Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    }
  ]
}
```

### Run B settled API state

Artifact: `.playwright-mcp/qa-c10-port-repeat-settled.json`.

```json
{
  "request": {
    "operationName": "QaC10PortRepeatSettled",
    "query": "query QaC10PortRepeatSettled($s:String!){service(id:$s){id phase revision port undeployedChanges} deploys(serviceId:$s){id status trigger}}",
    "variables": {
      "s": "srv-db3ktf4hm7os73dpdc30"
    }
  },
  "response": {
    "data": {
      "deploys": [
        {
          "id": "dep-db3l1qs3a6ns73b87pog",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3kvqc3a6ns73b87png",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3kv2c3a6ns73b87pmg",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3ktf4hm7os73dpdc3g",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "service": {
        "id": "srv-db3ktf4hm7os73dpdc30",
        "phase": "Running",
        "port": 3001,
        "revision": "rev-8",
        "undeployedChanges": false
      }
    }
  },
  "status": 200
}
```

### Same-port Restart control: mutation and complete sampled responses

Artifact: `.playwright-mcp/qa-c10-restart-control.json`.

```json
{
  "mutation": {
    "request": {
      "operationName": "RestartServer",
      "variables": {
        "serviceId": "srv-db3ktf4hm7os73dpdc30"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation RestartServer($serviceId: String!) {\n  restartServer(serviceId: $serviceId) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "restartServer": {
          "__typename": "Deploy",
          "createdAt": "2026-10-08T08:19:42.2837Z",
          "id": "dep-db3l47k3a6ns73b87pqg",
          "image": "busybox:1.37",
          "rollbackOf": "",
          "status": "created",
          "trigger": "api"
        }
      }
    },
    "status": 200
  },
  "samples": [
    {
      "at": "2026-10-08T08:19:40.932Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:41.510Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:42.150Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:42.725Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:43.297Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:43.870Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:44.450Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:45.025Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:45.652Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:46.226Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:46.866Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:47.447Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:48.019Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:48.593Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:49.172Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:49.753Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:50.370Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:50.946Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:51.583Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:52.156Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:52.731Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:53.305Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:53.881Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:54.455Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:55.088Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:55.659Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:56.302Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:56.881Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:57.453Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:58.035Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:58.607Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:59.179Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:19:59.806Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:00.382Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:01.027Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:01.605Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:02.179Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:02.753Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:03.327Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:03.898Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    },
    {
      "at": "2026-10-08T08:20:04.531Z",
      "status": 200,
      "body": "marker=qa-c10-net-e5c1\nport=3001\n"
    }
  ],
  "mainStart": "- main:\n  - navigation \"Breadcrumbs\":\n    - link \"Projects\":\n      - /url: /\n    - button \"qa-20261008-c10-net-e5c1\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261008-c10-net-e5c1\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: In Progress\"':\n    - /url: /services/srv-db3ktf4hm7os73dpdc30/deploys/dep-db3l47k3a6ns73b87pqg\n    - text: Latest deploy In Progress\n  - link \"Free\":\n    - /url: /services/srv-db3ktf4hm7os73dpdc30/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db3ktf4hm7os73dpdc30\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261008-c10-net-e5c1.onbex.co\":\n    - /url: https://qa-20261008-c10-net-e5c1.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261008-c10-net-e5c1\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-9\n  - term: Created\n  - definition:\n    - time: 13m\n  - text: Deploy status In Progress Manual Deploy dep-db3l47k3a6ns73b87pqg\n  - button \"Cancel deploy dep-db3l47k3a6ns73b87pqg\": Cancel\n  - paragraph: busybox:1.37\n  - term: \"Created:\"\n  - definition: October 8, 2026 at 1:19 AM\n  - term: \"Updated:\"\n  - definition: October 8, 2026 at 1:19 AM\n  - term: \"Started:\"\n  - definition: October 8, 2026 at 1:19 AM\n  - text: Status timeline\n  - list:\n    - listitem:\n      - paragraph: Deploy created\n      - time: October 8, 2026 at 1:19 AM\n    -"
}
```

### Restart reaches Live on the same port

Artifact: `.playwright-mcp/qa-c10-restart-settled.json`.

```json
{
  "request": {
    "operationName": "QaC10RestartSettled",
    "query": "query QaC10RestartSettled($s:String!){service(id:$s){id phase revision port undeployedChanges} deploys(serviceId:$s){id status trigger}}",
    "variables": {
      "s": "srv-db3ktf4hm7os73dpdc30"
    }
  },
  "response": {
    "data": {
      "deploys": [
        {
          "id": "dep-db3l47k3a6ns73b87pqg",
          "status": "live",
          "trigger": "api"
        },
        {
          "id": "dep-db3l1qs3a6ns73b87pog",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3kvqc3a6ns73b87png",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3kv2c3a6ns73b87pmg",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3ktf4hm7os73dpdc3g",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "service": {
        "id": "srv-db3ktf4hm7os73dpdc30",
        "phase": "Running",
        "port": 3001,
        "revision": "rev-9",
        "undeployedChanges": false
      }
    }
  },
  "status": 200
}
```

### Post-Live Restart control window

Artifact: `.playwright-mcp/qa-c10-restart-settled-http.json`.

```json
[
  {
    "at": "2026-10-08T08:20:47.569Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:48.206Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:48.779Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:49.356Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:49.930Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:50.511Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:51.089Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:51.717Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:52.291Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:52.924Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:53.500Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:54.075Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:54.648Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:55.228Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  },
  {
    "at": "2026-10-08T08:20:55.804Z",
    "status": 200,
    "body": "marker=qa-c10-net-e5c1\nport=3001\n"
  }
]
```

## Selected Kubernetes observation

Repeat the following read-only commands during the fresh port save. This is an explicitly **selected projection**, not a full raw Kubernetes API response: Service names/selectors/ports, pod names/labels/deletion/readiness/container ports, EndpointSlice fields and Ingress rules. The collector's start/end timestamps bound four sequential reads.

```sh
kubectl --context hetzner-prod get service -n tea-d98210cbbpdc73dcrkvg tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1 qa-20261008-c10-net-e5c1 -o json
kubectl --context hetzner-prod get pods -n tea-d98210cbbpdc73dcrkvg -l app.bex.co/app=tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1 -o json
kubectl --context hetzner-prod get endpointslices.discovery.k8s.io -n tea-d98210cbbpdc73dcrkvg -l kubernetes.io/service-name=tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1 -o json
kubectl --context hetzner-prod get ingress -n tea-d98210cbbpdc73dcrkvg tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1 -o json
```

```json
{
  "startedAt": "2026-10-08T08:14:40.657127+00:00",
  "services": [
    {
      "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1",
      "selector": {
        "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1"
      },
      "ports": [
        {
          "port": 3001,
          "protocol": "TCP",
          "targetPort": 3001
        }
      ]
    },
    {
      "name": "qa-20261008-c10-net-e5c1",
      "selector": {
        "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1"
      },
      "ports": [
        {
          "port": 3001,
          "protocol": "TCP",
          "targetPort": 3001
        }
      ]
    }
  ],
  "pods": [
    {
      "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1-67b5ffdfdll47",
      "labels": {
        "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1",
        "app.bex.co/app-uid": "bb2ec5b7-b894-44aa-bd4d-56162e30a264",
        "app.bex.co/container-policy": "image-v1",
        "app.bex.co/revision": "rev-8",
        "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
        "bex.co/app-id": "srv-db3ktf4hm7os73dpdc30",
        "pod-template-hash": "67b5ffdf96"
      },
      "deletionTimestamp": null,
      "phase": "Running",
      "ready": [
        {
          "lastProbeTime": null,
          "lastTransitionTime": "2026-10-08T08:14:35Z",
          "message": "containers with unready status: [app]",
          "observedGeneration": 1,
          "reason": "ContainersNotReady",
          "status": "False",
          "type": "Ready"
        }
      ],
      "containers": [
        {
          "name": "app",
          "ports": [
            {
              "containerPort": 3001,
              "protocol": "TCP"
            }
          ]
        }
      ]
    },
    {
      "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1-9c5b99fbjgxl8",
      "labels": {
        "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1",
        "app.bex.co/app-uid": "bb2ec5b7-b894-44aa-bd4d-56162e30a264",
        "app.bex.co/container-policy": "image-v1",
        "app.bex.co/revision": "rev-7",
        "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
        "bex.co/app-id": "srv-db3ktf4hm7os73dpdc30",
        "pod-template-hash": "9c5b99fb8"
      },
      "deletionTimestamp": null,
      "phase": "Running",
      "ready": [
        {
          "lastProbeTime": null,
          "lastTransitionTime": "2026-10-08T08:10:28Z",
          "observedGeneration": 1,
          "status": "True",
          "type": "Ready"
        }
      ],
      "containers": [
        {
          "name": "app",
          "ports": [
            {
              "containerPort": 3000,
              "protocol": "TCP"
            }
          ]
        }
      ]
    }
  ],
  "endpointSlices": [
    {
      "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1-ld7rt",
      "ports": [
        {
          "name": "",
          "port": 3001,
          "protocol": "TCP"
        }
      ],
      "endpoints": [
        {
          "addresses": [
            "10.244.128.138"
          ],
          "conditions": {
            "ready": true,
            "serving": true,
            "terminating": false
          },
          "nodeName": "bex-tenant-0-bdn2q-8sn5q",
          "targetRef": {
            "kind": "Pod",
            "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1-9c5b99fbjgxl8",
            "namespace": "tea-d98210cbbpdc73dcrkvg",
            "uid": "0efd162a-b211-4980-b10d-93918c94ddad"
          },
          "zone": "fsn1-dc14"
        },
        {
          "addresses": [
            "10.244.128.249"
          ],
          "conditions": {
            "ready": false,
            "serving": false,
            "terminating": false
          },
          "nodeName": "bex-tenant-0-bdn2q-8sn5q",
          "targetRef": {
            "kind": "Pod",
            "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1-67b5ffdfdll47",
            "namespace": "tea-d98210cbbpdc73dcrkvg",
            "uid": "623e7ea1-d7ff-4c6c-abde-e3c4e6a8d304"
          },
          "zone": "fsn1-dc14"
        }
      ]
    }
  ],
  "ingressRules": [
    {
      "host": "qa-20261008-c10-net-e5c1.onbex.co",
      "http": {
        "paths": [
          {
            "backend": {
              "service": {
                "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c10-net-e5c1",
                "port": {
                  "number": 3001
                }
              }
            },
            "path": "/",
            "pathType": "Prefix"
          }
        ]
      }
    }
  ],
  "finishedAt": "2026-10-08T08:14:43.656380+00:00"
}
```

Supplemental local artifacts (verified before filing): `.playwright-mcp/qa-c10-port-k8s-samples.json` (nine snapshots), `qa-c10-port-return-settled.json`, `qa-c10-baseline.json`, `qa-c10-deny-save.json`, `qa-c10-deny-http.json`, `qa-c10-allow-save.json`, `qa-c10-allow-http.json`, `qa-c10-reorder.json`, `qa-c10-network-mobile.json`, `qa-c10-clear.json`, `qa-c10-clear-http.json`, `qa-c10-port-save.json`, `qa-c10-port-console.txt`, `qa-c10-network.txt`, `qa-c10-cleanup.json`; pinned consumer sources are `qa-c10-traefik-ingress-source.go` and `qa-c10-endpointslice-utils.go`. These are gitignored supplements; the request/response records and selected cluster observation above survive handoff.
