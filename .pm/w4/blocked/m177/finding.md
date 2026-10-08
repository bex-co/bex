# Container startup failure is described as an application crash

Why: a tenant is directed to application crash output that was never produced, although the runtime already recorded the missing shell precisely.

- **Severity:** major; the deploy diagnosis misleads the user during a failed hosting journey. The shell requirement itself is not the finding.
- **Repro:** sign in to `https://dashboard.bex.co`; create a Free Existing Image web service with image `traefik/whoami:v1.11.0`, port 3000 and Docker Command `/whoami -port 3000`. Wait for the startup retries, open its deploy, then reload. Observed service `srv-db301astnshs73a156qg`, deploy `dep-db301astnshs73a156r0`; URL `/services/srv-db301astnshs73a156qg/deploys/dep-db301astnshs73a156r0`.
- **Expected / actual:** the diagnosis should say the container could not start because `/bin/sh` cannot be executed in this image, with the recovery described below. Instead, it says the container exited after starting and directs the user to crash logs plus a port-bind suggestion. The deploy's All logs showed only platform deploy banners; the service log page showed HTTP access records, not application output.
- **Evidence:** `.playwright-mcp/qa-start-error-20261007-1.png`; `.playwright-mcp/qa-start-error-20261007-api.json`; exact probes below. Browser console was clean at the failing deploy and no product request failed at the transport layer. Navigation-aborted requests were not treated as defects. A later manually supplied unsupported `reveal` parameter produced a 400 and is excluded as QA input error.
- **Root cause:** `lego/operator/internal/controller/deployment_projection.go:228-229` executes a non-native command override using `/bin/sh -c`. `lego/operator/internal/controller/app_controller.go:4873-4884` handles `CrashLoopBackOff` using only `LastTerminationState.Terminated.ExitCode`, discarding its `Reason: StartError` and `Message` before inventing the started/crashed description. Kubernetes v0.35.0's `core/v1/types.go:3274-3297` explicitly retains both fields and the start time; the observed epoch start time also contradicts the current prose.
- **Fix:** refine this diagnostic globally for the known runtime-start failure while retaining the existing `CrashLoopBackOff` condition reason unless the full consumer taxonomy is deliberately updated. For the observed missing-shell case, name `/bin/sh` as unavailable and tell the user to clear Docker Command to use the image's default program or choose an image with the required shell. Do not dump arbitrary OCI messages, command arguments, container ids or tenant secret values. Unknown startup failures need honest bounded startup copy rather than fabricated application output. Ordinary application crashes keep the current exit-code/log guidance. Do not change command parsing, security policy, rollout timers or lifecycle state.
- **Consumer trace:** operator `app_controller.go:3147` writes the in-flight Ready diagnosis; `:3206` uses it at rollout timeout. `lego/backend/internal/store/reconciler.go:1557-1581` reads the current failure reason and `:1590-1604` reads the in-flight message. `stallDiagnosis` at `:1626-1633` admits `CrashLoopBackOff`; retaining that reason permits the refined text to reach existing consumers. GraphQL `lego/backend/internal/deploys/graphql.go:66`, REST `deploys/rest.go:97`, MCP's `renderDeploy` response and dashboard `dashboard/src/features/deploys/components/deploy-header.tsx:105-119` already carry the message. Both public list and detail views must preserve it.
- **Blast radius:** exhaustive `rg 'stuckPodMessage\(' lego/operator -g '*.go'` found exactly **2 production call sites**, the in-flight rollout at `app_controller.go:3147` and terminal settlement at `:3206`; remaining calls are tests. Its pod scan is shared by Deployment-backed web/private/worker Apps; static sites, cron Jobs, Postgres and Key Value are separate runtime paths and were not claimed as affected. t002 owns the caller audit and regression controls. API aliases to verify: REST deploy list/detail; GraphQL `deploys`/`deploy`; MCP `list_deploys`/`get_deploy`; dashboard deploy history/detail.
- **Adjacent classes:** actual process exits (including nonzero and OOM), image-pull refusal, invalid image names, missing config dependencies, probe failure and ordinary rollout progress retain their separate explanations. Existing permission, missing-resource, unauthenticated and network-timeout responses remain at their current API boundaries; this is an authorized deploy's runtime diagnosis, not a new resource-existence classifier. Before the pod's termination state settles, retain ordinary progress UI; publish the specific cause only when current-generation evidence exists, and clear it on healthy recovery or terminal cancellation.
- **Unverified:** terminal timeout; REST/MCP at the failing instant; failure Events/email; genuine application crash/OOM; private/worker variants; initial Waiting `RunContainerError` without a recorded terminated state. Similar symptoms in those paths are verification work, not separately established findings.
- **Render:** docs above describe Docker Command, but exact runtime startup diagnostics were not reproduced on Render. Do not claim a verified Render implementation difference.
- **Estimate:** 45m implementation + 35m caller/control verification; closing tasks are listed in the milestone.
- **Dedupe:** searched open and done `.pm` for `StartError`, `RunContainerError`, OCI runtime errors, missing executable/shell and the crash wording; reviewed the matching security/executable-selection items and w7/m79. w7/m79 covers missing Secret/ConfigMap and public-routing dependencies, not this StartError misclassification. w6/m123 covers build failure tails/timestamps, not runtime start errors. w4/m166 adds Docker Command controls; it does not correct this diagnosis. Scanned the 71 open/blocked workstream/milestone READMEs, checked `.pm/DO_NOT_DO.md`, the ADR018 non-goals, recent commits and `git log -S 'stuckPodMessage'`; no matching open fix or undeployed remedy was found. Checkout at capture was `5f628ed0e`; after pulling current main (`32dc42d28`), the cited branch still discards the same fields, and a fresh search found no additional filing. Line citations above refer to that refreshed checkout. This is a gap in existing diagnosis coverage, not a regression of a guarantee those milestones implemented.

## Durable GraphQL probe

From a signed-in dashboard page, send `POST https://api.bex.co/graphql` with cookies, `Content-Type: application/json`, and this complete body:

```json
{"query":"{server(id:\"srv-db301astnshs73a156qg\"){id phase plan startCommand}deploys(serviceId:\"srv-db301astnshs73a156qg\"){id status stallReason failureReason}native:server(id:\"srv-db30214nmvbc73akdjn0\"){phase revision}nativeDeploys:deploys(serviceId:\"srv-db30214nmvbc73akdjn0\"){id status stallReason failureReason}}"}
```

HTTP 200; complete response captured before cancellation:

```json
{
  "data": {
    "deploys": [{
      "failureReason": "",
      "id": "dep-db301astnshs73a156r0",
      "stallReason": "container exited shortly after start and is restarting repeatedly (last exit code 128) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped).",
      "status": "update_in_progress"
    }],
    "native": {"phase":"Deploying","revision":""},
    "nativeDeploys": [{"failureReason":"","id":"dep-db30214nmvbc73akdjng","stallReason":"","status":"update_in_progress"}],
    "server": {"id":"srv-db301astnshs73a156qg","phase":"Deploying","plan":"free","startCommand":"/whoami -port 3000"}
  }
}
```

The native service was still deploying at this capture; it is not an application-crash control. Later it reached Live and served `qa-20261007-r1-native-before` with HTTPS 200.

## Runtime observation and healthy recovery

Read-only production Kubernetes inspection of this fixture's pod at 08:23 UTC selected `status.containerStatuses` from `kubectl --context hetzner-prod -n tea-d98210cbbpdc73dcrkvg get pods -o json`. Relevant complete `lastState.terminated` object (only the fixture was printed):

```json
{
  "containerID":"containerd://a4ebd9aa4cee632ac116a06112581cc3e10e38e1a112c708b8ad5d311e1c8c10",
  "exitCode":128,
  "finishedAt":"2026-10-07T08:22:58Z",
  "message":"failed to create containerd task: failed to create shim task: OCI runtime create failed: runc create failed: unable to start container process: error during container init: exec: \"/bin/sh\": stat /bin/sh: no such file or directory",
  "reason":"StartError",
  "startedAt":"1970-01-01T00:00:00Z"
}
```

The current state was Waiting `CrashLoopBackOff`, restart count 5, `ready:false`, `started:false`. The image digest was `sha256:200689790a0a0ea48ca45992e0450bc26ccab5307375b41c84dfc4f2475937ab`.

Local controls confirmed this cause rather than inferring it from exit 128: `docker image inspect traefik/whoami:v1.11.0` reports ENTRYPOINT `["/whoami"]`, CMD null; `docker run --rm traefik/whoami:v1.11.0 --help` exposes `-port`; forcing entrypoint `/bin/sh` reproduces the missing-file runtime error. This does not prove Render's override interpretation.

Cancel was confirmed through the dashboard and settled Canceled. Setting `WHOAMI_PORT_NUMBER=3000` with Save only, clearing Docker Command and confirming Save changes recovered the same fixture. The replacement deploy `dep-db3050knmvbc73akdjq0` reached `live`, service `Running`, `rev-2`, `startCommand:""`, `undeployedChanges:false`, and `stallReason:""`. External `curl -sSI https://qa-20261007-r1-web.onbex.co` returned HTTP/2 200 at 08:28:54 UTC. Resource cleanup is recorded in the pass ledger/report; the fixture ids above are evidence, not permanent repro resources.


## Hunt cleanup

Both owned services were deleted through their dashboard confirmations; the owned project was deleted through Project Settings, which also removed its environment. All four by-id REST reads returned 404. A fresh Overview reload contained no links to this pass's resources. Pre-existing resources, including earlier QA resources, were left untouched. Session revocation is tracked in the local pass report.
