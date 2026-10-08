# Pre-deploy pods receive serving traffic and return intermittent HTTP 502

Why: a normal migration must not take a healthy previous release offline. The migration pod is currently published as a ready serving endpoint while it runs, including after its rollout is canceled.

**Severity:** major. **Source:** live `qa-find-bugs` loop cycle 7, 2026-10-07 local / 2026-10-08 UTC. **Workspace:** bex, `tea-d98210cbbpdc73dcrkvg`. **Checkout:** `378a83586`. Production Kubernetes reported `v1.34.9`; the running controller-manager used `ghcr.io/bex-co/bex-operator@sha256:ae4dc73385537cae9b3b9f313f5c9809c5c9583380bd9098f3b9ebc9c0a798f1`. No claim that this digest is the checkout's exact source build is needed: the observed selector and pod labels reproduce the current code's mechanism, and no selector fix is present on main.

## Reproduction and controls

Fixture: `qa-20261007-c7-web-a63b`, service `srv-db3jmfs3a6ns73b87ol0`, App `tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b` in namespace `tea-d98210cbbpdc73dcrkvg`.

1. Create an Existing Image Free Web Service on port 3000, image `busybox:1.37`, Docker Command:

   ```sh
   sh -c 'mkdir -p /tmp/qa-c7; printf "qa-c7-runtime-a63b\n" > /tmp/qa-c7/index.html; busybox 2>&1 | head -n 1 >> /tmp/qa-c7/index.html; exec httpd -f -p 3000 -h /tmp/qa-c7'
   ```

   First deploy `dep-db3jmfs3a6ns73b87olg` reached Live, public GET returned HTTP 200 and BusyBox 1.37. A short exit-23 pre-deploy failed correctly, retained revision 1, and its marker appeared exactly once in All/Build/Application logs after fresh load. Clearing the command deployed successfully. These status/log checks are passing controls, not new findings.

2. Stage BusyBox 1.36 through Settings → Source, then save a pre-deploy command that prints a marker, sleeps 90 seconds and succeeds. Cancel during Pre-Deploy In Progress. The row closes canceled and the previous image is retained. Initial public requests returned 502, then hibernation 503, then the prior image: that mixed sample alone was not enough to blame cancellation.

3. Set Idle timeout to **1 hr**, clear the command and wait for the saved BusyBox 1.36 release to go Live (`dep-db3jpoc3a6ns73b87ou0`, revision 7). Begin regular external requests, stage BusyBox 1.37, and save:

   ```sh
   printf 'qa-c7-cancel-awake-start-a63b\n'; sleep 90; printf 'qa-c7-cancel-awake-finish-a63b\n'
   ```

   Cancel `dep-db3jq1shm7os73dpdb20` through its confirmation dialog while the step runs. The sampler from 06:49:41Z to 06:52:40Z made **69 GETs: 52 HTTP 200 with BusyBox 1.36.1, 17 HTTP 502 with `Bad Gateway`**. Failures occur during the running step, before and after Cancel. They stop after the step ends. No hibernation responses occur in this awake replay.

4. From a fresh Settings page, save a separate bounded failure:

   ```sh
   printf 'qa-c7-routing-start-a63b\n'; sleep 90; printf 'qa-c7-routing-fail-a63b\n'; exit 23
   ```

   On deploy `dep-db3jrichm7os73dpdb8g`, inspect live pods and EndpointSlices while the step runs. The ready application pod is `10.244.128.184`, image BusyBox 1.36, revision 7. The ready Job-owned pre-deploy pod is `10.244.90.134`, image BusyBox 1.37, with no HTTP-serving command. The primary Service's EndpointSlice contains **both** as `ready: true`, `serving: true`, port 3000. Both stable Services have the same app-name-only selector.

   Twelve public GETs at 06:54:02–06:54:19Z returned **six 502s, five 200s with the prior image, and one eight-second connection timeout**. The timeout is recorded as an observation, not separately attributed to this selector without further evidence. The job fails with exit 23; deploy status is `pre_deploy_failed`, step `failed`, service `Running`, revision 7. Afterward the EndpointSlice contains only the app pod, and **10/10 GETs return 200 with BusyBox 1.36.1**.

**Expected:** only runtime pods receive serving traffic; the healthy prior release answers throughout the pre-deploy step and its failure/cancel. **Actual:** an auxiliary Job pod joins the serving endpoint set and requests intermittently fail. The new image does not incorrectly become the serving application; the failure is traffic sent to the migration pod.

## Root cause and proposed fix

- `lego/operator/internal/controller/image_network.go:96–98`: `privateServiceProjection` starts with `{labelApp: app.Name}`. The legacy configured-port fixture has no additional revision selector.
- `lego/operator/internal/controller/app_controller.go:3014–3022`: `applyClusterIPService` assigns that selector to both stable Services. `reconcileSlugService` delegates to the same projection (`:3058`).
- `lego/operator/internal/predeploy/predeploy.go:157–160`: the Job and its pod use `execution.PodLabels`, preserving the App's app-name label. `lego/operator/internal/execution/security.go:73–76` includes `app.bex.co/app` with the same value used by the Service. The separate `component: predeploy` label does not help a Service that never selects a component. The shared label is needed by logging/isolation; removing it is the wrong layer to change.
- `lego/operator/internal/controller/deployment_projection.go:173–181`: runtime pods already have `bex.co/app-id`, falling back to the App name for legacy Apps. The captured migration pod has no such label. `appIDOrName` (`app_controller.go:3432–3434`) already implements that identity rule.

**Target fix:** add the existing runtime identity (`bex.co/app-id: appIDOrName(app)`) to the serving ClusterIP selector alongside app name, retaining the image-network revision constraint and ports. Use the shared projection for both Service names. Verify the caller/type matrix before landing. Do not add labels to immutable Deployment selectors, roll every application to introduce a new role label, alter the execution-label helper, stop migrations on Cancel, or mask the 502 with changed UI statuses.

**Library mechanism, checked at the actual server version:** Kubernetes [`v1.34.9` EndpointSlice controller](https://github.com/kubernetes/kubernetes/blob/v1.34.9/pkg/controller/endpointslice/endpointslice_controller.go#L392) lists same-namespace pods matching the Service selector. Its [reconciler](https://github.com/kubernetes/kubernetes/blob/v1.34.9/staging/src/k8s.io/endpointslice/reconciler.go#L194) forms endpoints for eligible selected pods, and [`podToEndpoint`](https://github.com/kubernetes/kubernetes/blob/v1.34.9/staging/src/k8s.io/endpointslice/utils.go#L38) derives serving/ready from the Pod's Ready condition. A Job owner does not exempt a selected running pod. The same file's `FindPort` accepts numeric target ports directly, so the lack of declared container ports is not a routing exclusion. The live EndpointSlice independently confirms these facts. Official [Service](https://kubernetes.io/docs/concepts/services-networking/service/) and [EndpointSlice](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/) documentation describe the selector/readiness mechanism.

## Blast radius, aliases and adjacent behavior

Exhaustive production grep at the recorded checkout (tests excluded): `privateServiceProjection` has **one caller**, `applyClusterIPService:3015`. `applyClusterIPService` has **three call sites**: normal primary convergence (`app_controller.go:2491`), slug alias (`:3058`), and image-network promotion (`image_network.go:135`). The proposed change is global within this serving-Service projection, not global across all pod labels/selectors.

- **Web/private:** both use these Services. The configured-port image web path was live-probed. Native/buildpack/Docker web and private services, including discovered multiports/revision selection, need t002 controls; none was live-probed here.
- **Static:** shared static-server routing, not this app-runtime selector. **Worker:** no addressable Service. **Cron:** no serving Service; cron pods also carry `bex.co/app-id`, so that label alone is not a universal role discriminator. Preserve non-addressable/type-change Service cleanup. **Postgres/Key Value:** separate resource controllers/Service projections, outside the fix.
- **Unchanged consumers:** the immutable Deployment selector, job ownership, log discovery, admission, workspace/environment network isolation and metrics labels. Selectorless platform ExternalName aliases (`app_controller.go:3944–3956`) are separate and remain unchanged.
- **Compatibility:** preserve ready old/new runtime pods during ordinary rolling deploys, public-ID fallback for hand-applied Apps, alias owner-conflict refusal, and existing image-network revision/port promotion. Existing `TestImageNetworkLeavesLegacyServicesUnchanged` pins `len(selector)==1` (`image_network_test.go:202–207`); replace that implementation assertion with preserved legacy ports and correct endpoint membership.
- **First release/no previous pod:** excluding the migration must not invent a serving endpoint; normal startup/activator behavior remains. Genuine unhealthy application pods stay subject to existing readiness. Sleep/suspend remain parked lifecycle states, rather than a reason to admit an auxiliary job.
- **Cancel:** w5/085 deliberately lets an in-flight migration finish; the deploy can be canceled while its step later succeeds. This behavior passed twice and must remain. The correction excludes that running job from traffic while preserving its outcome/logs.
- **Authorization:** routing selection is operator-internal, not an API existence/error-taxonomy change. Keep namespaced ownership and existing unauthorized/forbidden/missing-resource behavior. Do not broaden label or cross-namespace matches.
- **Before settle:** retain the healthy prior runtime during Pending/Running pre-deploy; update only the Service's eligible set. No new dashboard cache/polling predicate or API result shape is proposed.

Entrypoints to audit: REST service create/PATCH `preDeployCommand` (top-level and `serviceDetails`); GraphQL `createService` and `setPreDeployCommand`; MCP `create_web_service` and `update_service`; Blueprint field wiring; dashboard Settings. Retired MCP `set_pre_deploy_command` is not a current registration. All create an App consumed by this operator; no adapter-specific routing fix is needed. Only GraphQL writes and reads plus dashboard/public HTTP were exercised in the failing path; REST/MCP/Blueprint pre-deploy journeys remain unverified.

The two stable Services' identical selectors and ports were captured directly. Only the primary Service's EndpointSlice was captured while the job ran; membership in the slug Service's EndpointSlice is inferred from that identical projection and the checked Kubernetes controller, and must be independently checked in closeout.

## Dedupe and original guarantee

Searched open/done/blocked notes and open milestone READMEs across every workstream for pre-deploy with selector/endpoint/traffic/502, and reviewed targeted code history. Re-read `.pm/DO_NOT_DO.md`: no anti-goal permits migration pods receiving traffic. No `selector[labelAppID]` fix is present on main; `privateServiceProjection` still returns the observed app-only selector on this path, so this is not a deploy-lag filing.

- **w1/done/m33:** regression of its observable availability guarantee, rather than duplicating its original feature implementation. Entire DoD audit: the command runs against the new image (observed Job BusyBox 1.37 while the app serves 1.36); rollout blocks and exit 23 is recorded (passes); prior revision stays live **and serving** (revision retention passes, availability fails); step verdict/logs visible across REST/GraphQL/MCP/dashboard (GraphQL/dashboard and log-bucket controls pass, full REST/MCP replay remains t003 work). A successful gated promotion also remains t005 verification work.
- **w1/done/m156 and w1/done/m172:** held/failed release runtime convergence and restoring a previous template are distinct; this fixture has a ready prior pod and correct retained revision. Neither narrows the app-only Service selector. Their restore/park controls are preserved by t002.
- **w5/done/085:** continuing a canceled migration and reporting its eventual outcome are intentional. Not a second cancellation-status finding.
- **w4/blocked/180:** its source-membership log-filter bug did not reproduce; marker present once in all three buckets after fresh load. Not duplicated.
- **w4/blocked/m171 / w5/blocked/m114:** suspend/rollout terminal and release-restoration semantics are separate from a running job entering endpoints. Suspend was not replayed in this cycle and their remaining DoDs are not closed by this finding.
- **w4/blocked/m162/m164:** scanner-driven Free wake/traffic accounting are different. The decisive replay had `idleTTLSeconds: 3600`, regular GETs and no hibernation responses. The mixed first-cancel sample is excluded from the causal proof.

**Render:** official [deploy docs](https://render.com/docs/deploys#pre-deploy-command) put pre-deploy before starting the replacement and describe zero-downtime handoff. This filing enforces bex's already documented preservation of the prior service. No fresh authenticated Render migration or claim about its internal selector implementation. Render/bex plan availability differences are not this bug.

## Local evidence and cleanup

Verified local artifacts: `.playwright-mcp/qa-c7-routing-pods.json`, `qa-c7-routing-services.json`, `qa-c7-routing-endpoints.json`, `qa-c7-routing-endpoints-after.json`, `qa-c7-cancel-awake-samples.json`, `qa-c7-routing-failure-samples.json`, `qa-c7-routing-after-failure-samples.json`, `qa-c7-routing-state.json`, `qa-c7-log-buckets.json`, `qa-c7-fresh-app-logs.json`, and `qa-c7-routing-failed-detail.png`. The screenshot was inspected: it supports correct terminal/retained-service UI and logs, not the HTTP 502 count. Console capture reports zero errors; captured aborted polling/navigation requests and prior-cycle contract probes are not separate bugs. Durable request/response and Kubernetes projections follow below, so the finding does not depend on gitignored screenshots.

UI deletion succeeded; REST service and public URL returned 404; fresh Overview contains no fixture name. A names-only inventory of Apps/Deployments/ReplicaSets/Pods/Services/Ingresses/Jobs/ConfigMaps/Secrets in the fixture namespace first showed its draining app pod and subsequently showed none with this cycle's fixture prefix. The isolated Kratos session was revoked successfully and its jars removed. No product code was edited.

## Durable probes

Kubernetes blocks preserve the complete selected metadata/status/selector/endpoints projection; pod environment and credentials were never included. The HTTP sample command was `curl --silent --show-error --max-time 10 --write-out "\n%{http_code}" https://qa-20261007-c7-web-a63b.onbex.co/` for the cancel sampler (two-second pause), and the same command with `--max-time 8` and a 0.3-second pause for the twelve failing and ten control requests. `status: 000` is curl's timeout, not an HTTP response.

Repeat the metadata reads with the new fixture's own names/namespace:

```sh
kubectl --context hetzner-prod -n <fixture-namespace> get svc <slug> <app-name> -o json
kubectl --context hetzner-prod -n <fixture-namespace> get pods -l app.bex.co/app=<app-name> -o json
kubectl --context hetzner-prod -n <fixture-namespace> get endpointslices -l kubernetes.io/service-name=<app-name> -o json
```

### Complete GraphQL pre-deploy setting write

```json
{
  "request": {
    "operationName": "SetPreDeployCommand",
    "variables": {
      "id": "srv-db3jmfs3a6ns73b87ol0",
      "command": "printf 'qa-c7-predeploy-fail-a63b\\n'; exit 23"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetPreDeployCommand($id: String!, $command: String!, $confirm: String) {\n  setPreDeployCommand(id: $id, command: $command, confirm: $confirm) {\n    id\n    preDeployCommand\n    phase\n    __typename\n  }\n}"
  },
  "response": {
    "data": {
      "setPreDeployCommand": {
        "__typename": "Service",
        "id": "srv-db3jmfs3a6ns73b87ol0",
        "phase": "Running",
        "preDeployCommand": "printf 'qa-c7-predeploy-fail-a63b\\n'; exit 23"
      }
    }
  }
}
```

### Complete awake-cancel GraphQL write

```json
{
  "request": {
    "operationName": "CancelDeploy",
    "variables": {
      "serviceId": "srv-db3jmfs3a6ns73b87ol0",
      "deployId": "dep-db3jq1shm7os73dpdb20"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation CancelDeploy($serviceId: String!, $deployId: String!) {\n  cancelDeploy(serviceId: $serviceId, deployId: $deployId) {\n    id\n    status\n    __typename\n  }\n}"
  },
  "response": {
    "data": {
      "cancelDeploy": {
        "__typename": "Deploy",
        "id": "dep-db3jq1shm7os73dpdb20",
        "status": "canceled"
      }
    }
  }
}
```

### Complete GraphQL terminal read after the separate failed rollout

```json
{
  "request": {
    "operationName": "QaC7RoutingState",
    "query": "query QaC7RoutingState($id:String!){service(id:$id){id phase revision imagePath idleTTLSeconds preDeployCommand} deploys(serviceId:$id,limit:6){id status image preDeployStatus cancelReason failureReason finishedAt}}",
    "variables": {
      "id": "srv-db3jmfs3a6ns73b87ol0"
    }
  },
  "response": {
    "data": {
      "deploys": [
        {
          "cancelReason": "",
          "failureReason": "the pre-deploy command exited with code 23; check the pre-deploy logs",
          "finishedAt": "2026-10-08T06:54:34.893348Z",
          "id": "dep-db3jrichm7os73dpdb8g",
          "image": "busybox:1.37",
          "preDeployStatus": "failed",
          "status": "pre_deploy_failed"
        },
        {
          "cancelReason": "",
          "failureReason": "",
          "finishedAt": "2026-10-08T06:50:05.461642Z",
          "id": "dep-db3jq1shm7os73dpdb20",
          "image": "busybox:1.37",
          "preDeployStatus": "succeeded",
          "status": "canceled"
        },
        {
          "cancelReason": "",
          "failureReason": "",
          "finishedAt": "2026-10-08T06:49:22.04597Z",
          "id": "dep-db3jpoc3a6ns73b87ou0",
          "image": "busybox:1.36",
          "preDeployStatus": "",
          "status": "live"
        },
        {
          "cancelReason": "",
          "failureReason": "",
          "finishedAt": "2026-10-08T06:47:42.028026Z",
          "id": "dep-db3joks3a6ns73b87oqg",
          "image": "busybox:1.36",
          "preDeployStatus": "succeeded",
          "status": "canceled"
        },
        {
          "cancelReason": "",
          "failureReason": "",
          "finishedAt": "2026-10-08T06:45:51.426487Z",
          "id": "dep-db3jo5khm7os73dpdar0",
          "image": "busybox:1.37",
          "preDeployStatus": "",
          "status": "deactivated"
        },
        {
          "cancelReason": "",
          "failureReason": "the pre-deploy command exited with code 23; check the pre-deploy logs",
          "finishedAt": "2026-10-08T06:44:04.321402Z",
          "id": "dep-db3jn8khm7os73dpdaq0",
          "image": "busybox:1.37",
          "preDeployStatus": "failed",
          "status": "pre_deploy_failed"
        }
      ],
      "service": {
        "id": "srv-db3jmfs3a6ns73b87ol0",
        "idleTTLSeconds": 3600,
        "imagePath": "busybox:1.37",
        "phase": "Running",
        "preDeployCommand": "printf 'qa-c7-routing-start-a63b\\n'; sleep 90; printf 'qa-c7-routing-fail-a63b\\n'; exit 23",
        "revision": "rev-7"
      }
    }
  },
  "status": 200
}
```

### Both stable Service projections

```json
[
  {
    "name": "qa-20261007-c7-web-a63b",
    "selector": {
      "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b"
    },
    "ports": [
      {
        "port": 3000,
        "protocol": "TCP",
        "targetPort": 3000
      }
    ]
  },
  {
    "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b",
    "selector": {
      "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b"
    },
    "ports": [
      {
        "port": 3000,
        "protocol": "TCP",
        "targetPort": 3000
      }
    ]
  }
]
```

### Captured pod metadata and status projection while the job ran

```json
[
  {
    "name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-5d4557b1wpnk2",
    "uid": "8e27e7f5-277b-4b65-a0f9-fe04fc867854",
    "labels": {
      "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b",
      "app.bex.co/app-uid": "2fba46dc-0aef-4cb2-8f6b-e3d45b62b8a8",
      "app.bex.co/component": "predeploy",
      "app.bex.co/container-policy": "image-v1",
      "app.bex.co/predeploy": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b",
      "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
      "batch.kubernetes.io/controller-uid": "419badc8-e9ea-4ba4-a1aa-fa35638b290e",
      "batch.kubernetes.io/job-name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-5d4557b155b8",
      "controller-uid": "419badc8-e9ea-4ba4-a1aa-fa35638b290e",
      "job-name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-5d4557b155b8"
    },
    "ownerReferences": [
      {
        "apiVersion": "batch/v1",
        "blockOwnerDeletion": true,
        "controller": true,
        "kind": "Job",
        "name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-5d4557b155b8",
        "uid": "419badc8-e9ea-4ba4-a1aa-fa35638b290e"
      }
    ],
    "phase": "Running",
    "ip": "10.244.90.134",
    "ready": [
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-08T06:52:59Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "Ready"
      }
    ],
    "containers": [
      {
        "name": "predeploy",
        "image": "busybox:1.37"
      }
    ]
  },
  {
    "name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-b77d5256kmqkt",
    "uid": "51a11b4a-0afb-47f1-bf1f-ae443ec05fe7",
    "labels": {
      "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b",
      "app.bex.co/app-uid": "2fba46dc-0aef-4cb2-8f6b-e3d45b62b8a8",
      "app.bex.co/component": "predeploy",
      "app.bex.co/container-policy": "image-v1",
      "app.bex.co/predeploy": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b",
      "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
      "batch.kubernetes.io/controller-uid": "33a9be16-c918-4428-9999-387b443f1b7e",
      "batch.kubernetes.io/job-name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-b77d5256faee",
      "controller-uid": "33a9be16-c918-4428-9999-387b443f1b7e",
      "job-name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-b77d5256faee"
    },
    "ownerReferences": [
      {
        "apiVersion": "batch/v1",
        "blockOwnerDeletion": true,
        "controller": true,
        "kind": "Job",
        "name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-b77d5256faee",
        "uid": "33a9be16-c918-4428-9999-387b443f1b7e"
      }
    ],
    "phase": "Succeeded",
    "ip": "10.244.90.222",
    "ready": [
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-08T06:51:15Z",
        "observedGeneration": 1,
        "reason": "PodCompleted",
        "status": "False",
        "type": "Ready"
      }
    ],
    "containers": [
      {
        "name": "predeploy",
        "image": "busybox:1.37"
      }
    ]
  },
  {
    "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b-755589f54lg4bb",
    "uid": "9f7abbe3-84b3-4ecb-8262-cf8e531e3808",
    "labels": {
      "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b",
      "app.bex.co/app-uid": "2fba46dc-0aef-4cb2-8f6b-e3d45b62b8a8",
      "app.bex.co/container-policy": "image-v1",
      "app.bex.co/revision": "rev-7",
      "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
      "bex.co/app-id": "srv-db3jmfs3a6ns73b87ol0",
      "pod-template-hash": "755589f545"
    },
    "ownerReferences": [
      {
        "apiVersion": "apps/v1",
        "blockOwnerDeletion": true,
        "controller": true,
        "kind": "ReplicaSet",
        "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b-755589f545",
        "uid": "f1dc782f-7152-4381-8fcf-096cdd6bb3a0"
      }
    ],
    "phase": "Running",
    "ip": "10.244.128.184",
    "ready": [
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-08T06:49:16Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "Ready"
      }
    ],
    "containers": [
      {
        "name": "app",
        "image": "busybox:1.36"
      }
    ]
  }
]
```

### Primary EndpointSlice projection while the job ran

```json
[
  {
    "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b-jghzt",
    "labels": {
      "endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
      "kubernetes.io/service-name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b"
    },
    "endpoints": [
      {
        "addresses": [
          "10.244.128.184"
        ],
        "conditions": {
          "ready": true,
          "serving": true,
          "terminating": false
        },
        "nodeName": "bex-tenant-0-bdn2q-8sn5q",
        "targetRef": {
          "kind": "Pod",
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b-755589f54lg4bb",
          "namespace": "tea-d98210cbbpdc73dcrkvg",
          "uid": "9f7abbe3-84b3-4ecb-8262-cf8e531e3808"
        },
        "zone": "fsn1-dc14"
      },
      {
        "addresses": [
          "10.244.90.134"
        ],
        "conditions": {
          "ready": true,
          "serving": true,
          "terminating": false
        },
        "nodeName": "bex-tenant-0-bdn2q-whp2m",
        "targetRef": {
          "kind": "Pod",
          "name": "predeploy-tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-5d4557b1wpnk2",
          "namespace": "tea-d98210cbbpdc73dcrkvg",
          "uid": "8e27e7f5-277b-4b65-a0f9-fe04fc867854"
        },
        "zone": "fsn1-dc14"
      }
    ],
    "ports": [
      {
        "name": "",
        "port": 3000,
        "protocol": "TCP"
      }
    ]
  }
]
```

### Primary EndpointSlice projection after the job failed

```json
[
  {
    "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b-jghzt",
    "endpoints": [
      {
        "addresses": [
          "10.244.128.184"
        ],
        "conditions": {
          "ready": true,
          "serving": true,
          "terminating": false
        },
        "nodeName": "bex-tenant-0-bdn2q-8sn5q",
        "targetRef": {
          "kind": "Pod",
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261007-c7-web-a63b-755589f54lg4bb",
          "namespace": "tea-d98210cbbpdc73dcrkvg",
          "uid": "9f7abbe3-84b3-4ecb-8262-cf8e531e3808"
        },
        "zone": "fsn1-dc14"
      }
    ],
    "ports": [
      {
        "name": "",
        "port": 3000,
        "protocol": "TCP"
      }
    ]
  }
]
```

### Complete awake-cancel GET sample records

```json
[
  {
    "at": "2026-10-08T06:49:41Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:49:44Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:49:47Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:49:49Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:49:52Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:49:55Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:49:57Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:00Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:02Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:05Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:08Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:10Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:13Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:16Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:18Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:21Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:23Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:26Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:29Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:31Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:34Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:37Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:39Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:42Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:44Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:47Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:50Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:52Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:55Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:50:57Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:00Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:03Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:05Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:08Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:11Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:13Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:16Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:18Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:21Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:24Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:26Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:29Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:32Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:34Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:37Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:39Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:42Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:45Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:47Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:50Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:53Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:55Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:51:58Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:00Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:03Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:06Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:08Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:11Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:13Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:16Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:19Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:21Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:24Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:27Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:29Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:32Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:34Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:37Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:52:40Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  }
]
```

### Complete separate-failure GET sample records

```json
[
  {
    "at": "2026-10-08T06:54:02Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:03Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:04Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:04Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:05Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:06Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:07Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:08Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:09Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:10Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:54:18Z",
    "status": "000",
    "body": "",
    "stderr": "curl: (28) Connection timed out after 8006 milliseconds\n"
  },
  {
    "at": "2026-10-08T06:54:19Z",
    "status": "502",
    "body": "Bad Gateway",
    "stderr": ""
  }
]
```

### Complete post-failure GET control records

```json
[
  {
    "at": "2026-10-08T06:55:56Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:55:57Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:55:58Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:55:59Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:56:00Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:56:01Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:56:02Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:56:02Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:56:03Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  },
  {
    "at": "2026-10-08T06:56:04Z",
    "status": "200",
    "body": "qa-c7-runtime-a63b\nBusyBox v1.36.1 (2023-05-18 22:34:17 UTC) multi-call binary.\n",
    "stderr": ""
  }
]
```
