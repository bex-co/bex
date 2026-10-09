# A stale projector snapshot deletes a successfully created service and recreates a partial spec

Why: a successful create can lose accepted settings seconds later and leave a customer with a broken service whose durable record still exists.

**Severity:** major. **Attribution:** proven server defect in current source; the live occurrence is consistent with it, but the exact CR-deletion actor was not observed. **Scheduled fix:** w8/m52. This hunt has not implemented or shipped it.

## Versions and context

Production https://api.bex.co/v1/, 2026-10-09 UTC (2026-10-08 America/Los_Angeles), workspace bex-canary / tea-daif693dqjvc73e7as3g, QA human device-flow grant, fresh non-TTY processes, JSON output. Checksum-verified official Bex v0.3.2, embedded VCS a6350da0d6e451e063143432e06c548ba7b5851b (vcs.modified=true in the released artifact), imports Render v2.27.0 / a764810a768202704e7206eb7b87a47211fcd98e. Private source probe: main 8e59fc0126f5971c9acc3b2e03f825991ba207a9. GitOps references image 465c725fea0b; the actual deployed revision is **unknown**, not assumed equal to HEAD. CLI source is unchanged from checkout comparison 3e08c3400.

## Live reproduction and complete safe responses

Use newly owned fixtures only, private isolated configuration, explicit BEX_HOST/BEX_WORKSPACE, verified free capacity and an owned-ID deletion path. Change the nonce for every create. Observed command:

```sh
bex services create --name qa-20261008-8ad415-pre --type web_service \
  --runtime image --image docker.io/mendhak/http-https-echo:35 \
  --region frankfurt --plan free --health-check-path / \
  --env-var HTTP_PORT=3000 \
  --env-var QA_PREDEPLOY_MARKER=qa-predeploy-8ad415-ok-λ \
  --pre-deploy-command 'printf "qa-predeploy-env=%s\n" "$QA_PREDEPLOY_MARKER"' \
  --confirm -o json
```

Exit 0 in 0.904 s, stderr empty. Complete **CLI-decoded service output**, not an intercepted raw POST response:

```json
{
  "autoDeploy": "no",
  "autoDeployTrigger": "off",
  "createdAt": "2026-10-09T04:06:40Z",
  "dashboardUrl": "https://dashboard.bex.co/web/srv-db46gjrfuh0c73ao9h50",
  "id": "srv-db46gjrfuh0c73ao9h50",
  "imagePath": "docker.io/mendhak/http-https-echo:35",
  "name": "qa-20261008-8ad415-pre",
  "notifyOnFail": "default",
  "ownerId": "tea-daif693dqjvc73e7as3g",
  "rootDir": "",
  "serviceDetails": {
    "env": "image",
    "envSpecificDetails": {
      "preDeployCommand": "printf \"qa-predeploy-env=%s\\n\" \"$QA_PREDEPLOY_MARKER\""
    },
    "healthCheckPath": "/",
    "internalAddress": "qa-20261008-8ad415-pre:3000",
    "maintenanceMode": {
      "enabled": false,
      "uri": ""
    },
    "maxShutdownDelaySeconds": 30,
    "numInstances": 1,
    "plan": "free",
    "port": 3000,
    "preDeployCommand": "printf \"qa-predeploy-env=%s\\n\" \"$QA_PREDEPLOY_MARKER\"",
    "region": "fsn1",
    "renderSubdomainPolicy": "enabled",
    "runtime": "image",
    "url": "https://qa-20261008-8ad415-pre.onbex.co"
  },
  "slug": "qa-20261008-8ad415-pre",
  "suspended": "not_suspended",
  "suspenders": [],
  "type": "web_service",
  "updatedAt": "2026-10-09T04:06:40Z"
}
```

The actual initial pre-deploy Job emitted its unique marker. `bex logs --resources srv-db46gjrfuh0c73ao9h50 --type build --limit 20 -o json` exited 0 in 6.324 s, stderr empty. Both decoded entries are below; concatenated-JSON framing is known upstream behavior.

```json
[
  {
    "id": "srv-db46gjrfuh0c73ao9h50-17irgjmti9npepkqqfmn-2026-10-09T04:06:39.713462Z-0506962f",
    "labels": [
      {
        "name": "type",
        "value": "build"
      },
      {
        "name": "resource",
        "value": "srv-db46gjrfuh0c73ao9h50"
      },
      {
        "name": "instance",
        "value": "srv-db46gjrfuh0c73ao9h50-17irgjmti9npepkqqfmn"
      },
      {
        "name": "container",
        "value": "platform"
      }
    ],
    "message": "==> Deploy queued",
    "timestamp": "2026-10-09T04:06:39.713462Z"
  },
  {
    "id": "srv-db46gjrfuh0c73ao9h50-55co6lpjqadkddrnheiv-2026-10-09T04:06:41.339592466Z-7d1bb8fd",
    "labels": [
      {
        "name": "type",
        "value": "build"
      },
      {
        "name": "resource",
        "value": "srv-db46gjrfuh0c73ao9h50"
      },
      {
        "name": "instance",
        "value": "srv-db46gjrfuh0c73ao9h50-55co6lpjqadkddrnheiv"
      },
      {
        "name": "container",
        "value": "predeploy"
      }
    ],
    "message": "qa-predeploy-env=qa-predeploy-8ad415-ok-λ",
    "timestamp": "2026-10-09T04:06:41.339592466Z"
  }
]
```

Subsequent actual CLI lists omit preDeployCommand, envSpecificDetails and healthCheckPath. A same-authority **diagnostic replay**, not intercepted CLI traffic, GET /v1/services/srv-db46gjrfuh0c73ao9h50 with User-Agent render-cli/2.27.0 (macOS - 26.5.1) and X-Bex-Workspace tea-daif693dqjvc73e7as3g returned 200. Relevant headers and complete response follow; only the account email is redacted. The durable ID is unchanged but the creation timestamp moves from **04:06:40Z to 04:06:53Z**, and the accepted settings disappear.

```json
{
  "headers": {
    "content-type": "application/json",
    "cf-ray": "a47a851cf98cde51-SJC"
  },
  "response": {
    "id": "srv-db46gjrfuh0c73ao9h50",
    "name": "qa-20261008-8ad415-pre",
    "immutableName": "qa-20261008-8ad415-pre",
    "slug": "qa-20261008-8ad415-pre",
    "displayName": "",
    "type": "web_service",
    "suspended": "not_suspended",
    "dashboardUrl": "https://dashboard.bex.co/web/srv-db46gjrfuh0c73ao9h50",
    "createdAt": "2026-10-09T04:06:53Z",
    "updatedAt": "2026-10-09T04:07:42Z",
    "owner": {
      "id": "tea-daif693dqjvc73e7as3g",
      "name": "bex-canary",
      "email": "[QA account email redacted]",
      "type": "team"
    },
    "serviceDetails": {
      "env": "image",
      "internalAddress": "qa-20261008-8ad415-pre:3000",
      "maintenanceMode": {
        "enabled": false,
        "uri": ""
      },
      "maxShutdownDelaySeconds": 30,
      "numInstances": 1,
      "plan": "free",
      "port": 3000,
      "region": "fsn1",
      "renderSubdomainPolicy": "enabled",
      "runtime": "image",
      "url": "https://qa-20261008-8ad415-pre.onbex.co"
    },
    "imagePath": "docker.io/mendhak/http-https-echo:35",
    "suspenders": [],
    "ownerId": "tea-daif693dqjvc73e7as3g",
    "phase": "Deploying",
    "replicas": 1,
    "idleTTLSeconds": 0,
    "autoDeploy": "no",
    "autoDeployTrigger": "off",
    "pushDeliveryMethod": "none",
    "notifyOnFail": "default",
    "notificationsToSend": "default"
  }
}
```

GET /v1/services/srv-db46gjrfuh0c73ao9h50/env-vars returned 200; both owned values still matched HTTP_PORT=3000 and the harmless Unicode QA marker. That proves store readback, not the process environment. Actual CLI app logs (same resource, --type app --limit 20 -o json; exit0) show: Listening on ports 8080 for http, and 8443 for https. The owned URL returned HTTP503 with the complete body below. Repeated detail/list reads continued to omit the settings. The first deploy dep-db46gjrfuh0c73ao9h5g remained update_in_progress at the final pre-deletion observation around 04:14Z. No eventual terminal state or direct container env read is claimed. Ignoring the stored port is consistent with losing the CR's environment-secret reference.

```text
service unavailable

```

One early deploys-list process exited1 in0.371s, stdout empty, stderr Error: unexpected response: 404 Not Found. Subsequent identical commands succeeded. This transient absence is supporting evidence, not a separately filed route bug.

**Fresh Bex control:** srv-db46inkct93s73fr3c50 / qa-20261008-384c0f-pre, equivalent flags and fresh marker. Settings and original createdAt 04:11:11Z survived; dep-db46inkct93s73fr3c5g reached live / preDeployStatus:succeeded; HTTP200 at /qa-control-384c0f. Changing the command to a harmless marker plus exit47 opened dep-db46k6ba2v3c73epktj0, which closed pre_deploy_failed / preDeployStatus:failed with failureReason: the pre-deploy command exited with code 47; check the pre-deploy logs. Its expected-failure marker is reachable with --type build, and the prior release still answers HTTP200 after a wake retry. The first request after a long pause returned {"error":"service not ready","retryAfter":5} while a new instance woke; this transient is **not** a failed-release outage finding.

**User-requested retry / unmodified same-pin Render control:** srv-db46tqsct93s73fr3cfg / qa-20261008-e7b036-pre, unmodified a764810a7682 CLI against **Bex**, separate 0600 config with this authorized human grant. Equivalent free image/port/predeploy flags and fresh marker. Create exited0 in0.781s; dep-db46tqsct93s73fr3cg0 reached live, original createdAt 04:34:51Z/settings remained, HTTP200 at /qa-render-control-e7b036. It did not reproduce the loss. One live failure and two successful controls establish intermittence, not a failure-rate estimate or Bex-only client defect. Never pointed at Render production.

## Producer → server → consumer

- Pinned cmd/servicecreate.go:158–162 validates/builds the body and sends it. pkg/service/create.go:45,95,161–162 carries env vars, health and predeploy; pkg/service/repo.go:82–96 consumes CreateServiceWithResponse and returns the 201 service. No client post-create clearing. POST request/status/headers were not captured: decoded successful output and actual Job marker are retained above.
- Backend apps/rest.go:1040–1051 calls shared Service.Create and returns service-and-deploy201. apps/service.go:2332–2353 allocates the durable row, then :2487 calls writeInitialApp; non-grouped creates publish the complete App through writeNewApp. The fields at issue do not live in store.App; they live in the complete CR/seeded references.
- store/reconciler.go:584 takes desired SQL rows **before** its CR list at :593. A row and complete CR can commit between those reads. The new CR enters byID while its new row is absent from this pass's seen set.
- The prune at store/reconciler.go:667–672 treats stale absence as deletion, calling Client.Delete without a fresh Store.GetApp check or explicit UID precondition. The existing control-plane identity guard cannot protect a new App of the same identity.
- Next pass :620 recreates it through projectApp/projectSpec (:1942,1972). That partial projection omits predeploy/health/runtime/env-secret references. Ordinary applyOwnedSpec preserves CR-only settings; a recreation has no complete CR to preserve. apps/service.go:1003–1004 reads the CR creation time, explaining the observed timestamp change if this was the live deletion path.

Current code is proven defective below. Exact live actor/UID/events were unavailable through ordinary QA APIs; deployed revision is unknown. Do not claim an observed Kubernetes deletion trace.

## Deterministic research probe and target fix

[Exact probe source](evidence/snapshot-probe_test.go.txt), no production credentials/resources, was added only to a private git-archive source copy. No product source changed. In an isolated exact-HEAD copy, place it as lego/backend/internal/store/qa_snapshot_probe_test.go and run:

```sh
cd lego/backend
GOWORK=off go test ./internal/store -run '^TestQAProbeNewAppSurvivesDesiredSnapshot$' -count=1 -v
```

The wrapper reads the desired snapshot, publishes a fake-store row and complete fake-client App, then returns the older snapshot. It invokes the actual ReconcileOnce, not a reimplementation. Observed exit1:

```text
=== RUN   TestQAProbeNewAppSurvivesDesiredSnapshot
fresh durable row srv-db46j91jg4r0p68ljo0g still exists, but projector deleted its full App between desired-row and CR snapshots
recreated App lost accepted configuration: predeploy="" health="" envFromSecret=""
--- FAIL: TestQAProbeNewAppSurvivesDesiredSnapshot (0.04s)
FAIL github.com/bex-co/bex/lego/backend/internal/store 0.793s
```

Three existing controls passed in the same source copy: TestPendingAppCreationWithholdsProjectionAndRetainsExistingCR, TestProjectorStillDeletesOwnAndLegacyApps, TestTwoProjectorsOnOneClusterDeleteOnlyTheirOwnApps. They do not exercise creation between snapshots.

**Target fix:** retain the identity guard and re-check durable existence with existing Store.GetApp(ctx,id) immediately before orphan deletion. Any existing row, including pending creation, prevents deletion. Delete only for confirmed store.ErrNotFound; timeout/connection/unknown errors retain the App and report a retryable reconciliation error. Bind deletion to the observed UID so a replacement cannot be removed by stale state. Preserve genuine orphan cleanup. No blanket prune-disable, timing grace as the correctness condition, CLI fork, or full-spec persistence migration. PGStore.GetApp (store.go:1030) directly reads/classifies the durable row.

**Blast radius:** one production ReconcileOnce scheduler (reconciler.go:424, PollWake), one App prune loop, five store-managed service types: web/private/worker/cron/static. Shared producers: REST, GraphQL graphql.go:1409, three MCP creates mcp.go:697,708,719, Blueprint createFromStack deploy.go:2855. Enumerate their exact reachability/count and native/secret-file effects in t002; live evidence is image-web only. Postgres/Key Value use separate datastore observation/placement, not this App prune. Namespace pruning is a sibling risk to inspect for matching evidence, not an assumed live namespace bug or automatic scope expansion.

## Dedupe, limits and cleanup

Searched all open/done .pm records for ListDesiredApps, stale desired/snapshot prune, new/recreated App and predeploy/config loss; inspected matches and history. w6/done/m39 + w3/done/017 fixed cross-control-plane ownership, whose guard remains. Pending creation / grouped Blueprint tests cover rows present in the desired snapshot, not absent from an older snapshot. w7/done/011 concerns an operator-config namespace toggle; w6/done/m46 type/exposure/generation convergence; w8/done/022 image row-first triggers; w2/done/042 image-clone read projection; w5/done/m100 predeploy log reachability. None owns this delete-by-stale-absence gap. git log -S 'seen[id]' on store/reconciler.go points to original aebbd4342; current code still lacks a fresh row guard. No matching new implementation/open repair found. Known upstream framing and pricing/SSH/sandbox non-goals remain separate.

All three exact owned IDs and recorded deploy dependents were deleted through released Bex. Detail/runtime404, CLI list absence, all six baseline IDs present, services quota used6/terminating0, PG0/KV0 verified. Copied Render config removed without revoking the original ongoing hunt grant. No surviving hosting fixture, paid changes, admin bypass, commit/push or external upstream submission. Session logout is reserved for the end of the infinite loop.

Unverified: live deletion actor/CR UID/events; deployed revision; same-pin failing occurrence; native/static/cron/private/worker/secret-file live variants; real-Postgres/apiserver concurrency; UI/MCP/GraphQL creates; infrastructure residue below normal user-visible deletion checks; eventual first-deploy terminal state (cleaned before terminal convergence).
