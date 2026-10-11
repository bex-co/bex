# Restart chooses stale deploy history after runtime cutover

**Severity:** major functional configuration error. Production dashboard QA a209, 2026-10-11 UTC. Research/main pin `155fb3cb5`. Filing only. All actions used the ordinary signed-in dashboard and customer-visible reads on one owned service; no platform inspection or product changes.

## Reproduce the observed timing window

Create Free image web service `qa-20261011-a209-rollback` (`srv-db5sgu49vsac73cmev20`), image `python:3.13-alpine`, port 3000, and `QA_A209_MARKER=qa-a209-v1`. The complete create command below runs a Python HTTP server returning the marker in JSON. Keep that same image/working command for both marker configurations. Save V2, deploy it successfully, then use normal dashboard Rollback/Manual Deploy → Restart service.

| UTC time | Actual captured state/action |
| --- | --- |
| 17:57:13.971324 | Standard V2 deployment `dep-db5srochj6ks739l0nlg` reaches API Live. |
| 17:57:39.233 | Rollback to initial V1 deploy `dep-db5sgu49vsac73cmev2g` accepted; new row `dep-db5ss4shj6ks739l0nn0`. |
| 17:58:10.364 | Service Running, revision rev-7, public 200/V1; rollback row still update_in_progress. |
| 17:58:12.654 | Restart accepted as `dep-db5ssd4hj6ks739l0no0`. |
| 17:58:14.060085 | The already-serving rollback row closes canceled, Superseded by the Restart. |
| 17:58:28.878107 | Restart row reaches Live; 17:58:55.810 public read is 200/**V2**, not V1. |
| 17:59:44.029358 | Repeat rollback `dep-db5st0s9vsac73cmev7g` is allowed to reach API Live; public V1 confirmed at 18:00:04.288. |
| 18:00:41.842 | Restart after this settled rollback accepted as `dep-db5stic9vsac73cmev8g`. |
| 18:00:58.983399 | That Restart reaches Live; 18:01:20.637 read stays 200/**V1**, with V2 still saved/pending. |
| 18:02:00.329 | After a fresh Deploys page load, Trigger latest image opens `dep-db5su649vsac73cmeva0` using saved V2. |
| 18:02:22.694 | Public 200/**V2**. |
| 18:02:23.005 | Service Running, revision rev-11, no undeployedChanges; new row update_in_progress, latest SQL-Live row is the settled V1 Restart `dep-db5stic9vsac73cmev8g`. |
| 18:02:24.059 | Normal Restart accepted as `dep-db5subs9vsac73cmevb0`; confirmation promises the commit/image running now. |
| 18:02:28.862745 | V2 row canceled, Superseded by this Restart. |
| 18:02:44.389884 | Restart reaches Live; 18:03:12.227 public is 200/**V1**, revision rev-12, saved V2 now pending. |
| 18:03:58.816637 | Subsequent standard deploy `dep-db5sutc9vsac73cmevc0` reaches Live; 18:04:16.649 public again 200/**V2**, no pending changes. |

These are two opposite-direction configuration substitutions: serving rollback V1 → Restart V2, and serving standard V2 → Restart V1. Neither newly serving intermediate row is claimed to have reached terminal API Live: both later close canceled. The contradiction is precisely that the public runtime and revision had changed while that history projection still lagged.

Expected: Restart reuses the configuration/artifact actually serving when its serialized dispatch selects the release. It must not substitute an earlier historical row merely because the currently serving row has not yet been observed Live. Saved configuration remains saved, and the next standard deploy still applies it. A same-image configuration-only case is sufficient to fail this contract; no new image digest, data loss or public outage is claimed.

## Root, dispatch and consumer

`lego/backend/internal/deploys/rollback_config.go:134–152` calls ListDeploys with **Statuses=[live], Limit=1**, then selects that row's immutable configuration and opens a Restart. The query never compares the row's generation with the serving identity. `targetReleaseConfig:45–69` selects the record named by the chosen row's generation; `openSelectedRelease:112–130` copies that selection into the newly minted release. Thus a lagging SQL-Live A row can select A even when B is already serving. This predicts both opposite-direction captures and the settled positive control.

The serving identity exists: `lego/types/v1alpha1/app_types.go:1460–1466` defines Status.ActiveRevision and Status.Image as the current revision/image. `apps/service.go:1109` exposes ActiveRevision as the captured REST revision. Status.ReleaseGeneration describes the current attempt and must not be used blindly as the serving generation. Existing store contract `store.go:630,2350–2360` resolves a deploy by **app ID + generation**, and GetDeploy at 2322–2327 also scopes by app ID. This permits selecting the currently serving row even while its status is open, without weakening those scopes or inventing a new API field.

`deploys/service.go:245–279` takes the existing per-App dispatch lock and re-reads the App, then uses `rollout.PatchAppLocked`. But today's selected target/configuration are frozen **before** that lock/reread. `rollout/rollout.go:204–221` re-reads and repeats mutation on an optimistic conflict; it cannot correct a frozen selection by itself. Opened the actual pinned controller-runtime **v0.23.3** `pkg/client/patch.go:101–139`: MergeFromWithOptimisticLock includes the original resourceVersion in the patch. A status advance can therefore require selection again on retry. No nested acquisition of the same dispatch lock is proposed.

History lag is consistent with the actual projection: `store/reconciler.go:1308–1386` observes live only when PhaseRunning and releaseIsActive agree; `releaseIsActive:1556–1562` requires the row generation to match both ReleaseGeneration and ActiveRevision. `supersededDeployStatus:1427–1459` can close a skipped observation canceled after a newer release is adopted. This explains the captured terminal rows; do not rewrite them to fabricated Live states or claim every canceled row had served.

### Targeted correction and neighbouring cases

Allowlist the correction to **Restart's current-runtime selection**. Resolve the active `rev-N` identity and its artifact/configuration within the existing serialized dispatch, scoped to the same App, and re-resolve if the optimistic retry observes a changed serving identity. Prefer the serving row/record regardless of its still-open history status; bind its immutable configuration through the existing ReleaseConfigReference fields, with PreserveGroupValues=true. The type at `releasesnapshots.go:146–178` supports positive SourceGeneration, selected Image and the new release Generation. The selected artifact must be the active Status.Image/record artifact, not an in-flight build artifact. A currently serving row can still have empty ResolvedImage before its terminal projection; that must not send a modern current release down the old-row fallback.

Do not fall back to a different old SQL-Live release when an authoritative modern serving revision lacks a resolvable current record/artifact. Return a named retryable conflict before a new release is opened when that current selection cannot be established. Preserve and explicitly test existing **legacy** image-only/missing-record and commit-pinned fallback dispositions rather than promising exact configuration from metadata that never existed. Empty/malformed active revision, no served release, missing matching row, missing/evicted record and unresolved image each need a documented modern-versus-legacy outcome. Existing denied, unauthenticated, deleting/replaced resource, billing, unavailable-store, timeout and trigger-validation outcomes retain their current adapters; none was induced by this finding.

Before settlement, the UI may truthfully show Running/current revision while the newest history row remains In Progress. Restart must select the running configuration even in that state; disabling all Restart actions until history catches up is not the requested preservation fix. No new menu, pending-state policy, fabricated history, or saved-state rewrite.

### Counted shared scope and aliases

Exhaustive non-test search: restartSelectedRelease has **one** caller (Service.Restart at `service.go:556`); targetReleaseConfig and openSelectedRelease each have **two** callers (Restart and explicit Rollback at 1034/1040). liveCommit has **one** production caller in Restart's fallback (564). openRelease has **two** production callers: openSelectedRelease at rollback_config.go:114 and normal trigger dispatch at service.go:759. t001 owns their compatibility audit before any change and must keep ordinary triggers correct. Make the selection change local to Restart, leaving explicit historical Rollback target selection intact.

Entry points: REST `POST /v1/services/{id}/restart` (`apps/rest.go:781`) and MCP `restart_service` (`apps/mcp.go:788–790`) call apps.Service.Restart (`service.go:3542`), whose RestartDeploy callback is wired once in `api/server.go:874–880,915` to deploys.Service.Restart. GraphQL `restartServer(serviceId:)` calls that verb directly (`deploys/graphql.go:236–242`). Dashboard uses the shared document through two hooks: useTriggerDeploy and useServiceLifecycle. Their production callers are ManualDeployButton and the service environment editor for useTriggerDeploy (the latter only triggers a normal deploy), and index, project overview, service settings and service-detail layout for useServiceLifecycle. Carry both UI entry families. Bex CLI uses pinned upstream Render CLI module `v1.1.3-0.20260909214233-a764810a7682`; it is a REST client, not a new first-party implementation. REST/MCP/CLI Restart were source-traced, not invoked live here.

Resource-family disposition: web was reproduced; private/worker share App runtime selection and require regression coverage; cron Restart is explicitly refused at `deploys/service.go:552`; static skips snapshot Restart at `rollback_config.go:135` and retains its existing path; Postgres/Key Value use separate services. These siblings were not run live. Environment Save-and-deploy's ServedRelease lookup (w4/m180) is a separately traced latest-Live consumer, but its timing window was **not** exercised; preserve its existing acceptance and do not count it as this observed failure.

## History, prior guarantees and dedupe

Targeted history identifies `e5af115b3 fix(deploys): preserve saved settings across rollback` as the introduction of restartSelectedRelease; the current latest-Live selection remains on refreshed main, with no later fix in the last-40 product history. This is a residual violation of the completed **w5/m107 Restart preservation guarantee**, not a claim that a new recent commit introduced it. No completed milestone is reopened or declared fully replayed here.

Every original `w5/done/m107` DoD clause is dispositioned:

| Original clause | This pass and required preservation |
| --- | --- |
| A → saved/deployed B → rollback A retains saved B; next normal deploy runs B | Observed for the marker and working Docker command, including the final V2 recovery. Historical replicas/health/files and isolated dev-5 are not re-verified. Preserve the saved/runtime separation. |
| Historical configuration survives reconciliation, pod replacement, explicit Restart and operator restart, with no transient group/saved mutation | **Explicit Restart in the history-lag window fails twice.** Settled Restart passes. Pod replacement, operator restart and linked groups were not exercised and require regression coverage, not a claimed full closeout. |
| Researched target/current matrix, snapshots/cancellation protections and current plan/disks/domains | Same-image environment selection is observed. Cancellation protections, field matrix, health, replicas, pre-deploy, autoscaling, disks and domains remain unprobed; retain existing target/current policy. |
| REST/GraphQL/MCP/UI agreement, dashboard-vs-API auto-deploy, missing/evicted metadata, same-image targets, already-running guards and indicators | GraphQL mutation plus REST service/deploy reads and dashboard are captured. Pending indication follows the observed running/saved difference. Rollback's dashboard disable-auto-deploy option is captured, but the API option comparison, MCP/CLI, missing records and already-running-target guards were not exercised. t001/t004/t006 carry them. |
| Required suites/lint/dashboard checks and live cleanup | This is a filing; no product suite or old closeout is claimed. This pass's fixture/session cleanup is complete. Implementer runs the appropriate checks and bounded live replay. |

`w5/blocked/m105` owns native Settings command-edit artifact dispatch, not this selection from lagging history; it already notes that ready runtimes can precede terminal API history. `w5/blocked/m114` owns Suspend → Resume interruption/stamps; neither operation caused this finding. `w4/blocked/m180` explicitly left Restart's lookup unchanged. `w4/blocked/m159` fixes confirmation lifetime and preserves backend semantics; its open dialog survived here, but its full acceptance remains open. `w4/blocked/m184` owns applying-notice wording, not the wrong selected runtime. Completed `w1/m148` fixes the UI verb/branch-head dispatch: this pass sent the correct RestartServer verb. Completed `w1/m152` snapshots/cancel restoration is retained, not rolled back.

Open/blocked/done distinctive-term searches and the **50 active milestone README** scan found no existing task owning selection of an already-serving row before its Live projection. The two findings of this pass are independent; the Export screenshot is not evidence for this backend failure. DO_NOT_DO imposes no conflicting anti-goal.

## Governing contract and limits

[ADR004](../../../docs/ADR004-app-deployment.md) specifies selected runtime configuration separate from saved settings; its release snapshots and fallback limitations remain in force. [ADR018](../../../docs/ADR018-render-parity.md) and [ADR006](../../../docs/ADR006-bex-api.md) carry the hosting/API contract. [Render Restart documentation](https://render.com/docs/deploys#restarting-a-service), consulted 2026-10-11, requires the running instance's commit/configuration and excludes undeployed environment edits. This is documentation evidence; no authenticated Render timing-window reproduction or claim about its storage mechanism.

Earlier in this fixture, an intentionally failing start command exited 37 and the platform failed the deployment after its configured rollout budget while V1 kept serving. Restart recovered V1; restoring the command deployed V2. Rename/reset/search, log filters, mobile geometry, source-confirmation lifetime and legacy route controls were healthy within the recorded bounds. They do not prove all historical milestones complete.

The local public sampler covers **17:39:11.369259–17:59:06.308818**, 217 responses, all 200. Later timing repeats have discrete public reads in the durable evidence; continuous availability is not claimed after the sampler ends. No public failure, loss of persistent customer data, alternate image, scale-out, private/worker/static, database or MCP/CLI behavior was demonstrated.

## Complete probes and cleanup

The JSON below embeds the exact create/deploy/rollback/Restart GraphQL requests and complete responses, and complete REST service/deploy reads from both failed windows and the settled/final controls. The request host for GraphQL is `POST https://api.bex.co/graphql`; captured REST paths use `https://api.bex.co` with GET. Public reads use `GET https://qa-20261011-a209-rollback.onbex.co/`. Browser context supplied authentication; no cookie/header credential is included. Only irrelevant whole-page UI text is omitted where explicitly indicated; API response bodies are complete.

All local paths were checked: `.playwright-mcp/qa-startup-rollback-a209-ledger.json` retains the entire pass; `qa-a209-public-samples.json` retains the completed sampler. This timing issue has durable API/runtime evidence, with no dedicated screenshot claim.

UI deletion succeeded at **18:04:41.324**. REST service read returned 404 at **18:05:17.191**, public read returned 404 at **18:05:21.119**, and all six full inventories equal the before-pass baseline (5 services, 4 Postgres, 2 Key Value, 3 projects, 1 group, 1 blueprint). Own logout succeeded at **18:05:33.651**, owners returned 401, one blank page remains with zero cookies/cleared clipboard permissions, and exact own session files were removed. Zero console errors/warnings. No foreign resource was changed.

```json
{
  "a209Create": {
    "at": "2026-10-11T17:33:44.989Z",
    "status": 200,
    "request": {
      "operationName": "CreateService",
      "variables": {
        "image": "python:3.13-alpine",
        "name": "qa-20261011-a209-rollback",
        "type": "web_service",
        "registryCredentialId": "",
        "runtime": "image",
        "startCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'",
        "plan": "free",
        "port": 3000,
        "envVars": [
          {
            "key": "QA_A209_MARKER",
            "value": "qa-a209-v1"
          }
        ],
        "ownerId": "tea-d98210cbbpdc73dcrkvg"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation CreateService($name: String!, $ownerId: String, $environmentId: String, $type: String, $repo: String, $image: String, $registryCredentialId: String, $branch: String, $rootDir: String, $runtime: String, $buildCommand: String, $startCommand: String, $dockerfilePath: String, $buildFilter: BuildFilterInput, $plan: String, $autoDeploy: Boolean, $schedule: String, $command: String, $publishPath: String, $port: Int, $envVars: [EnvVarInput], $secretFiles: [SecretFileInput]) {\n  createService(\n    name: $name\n    ownerId: $ownerId\n    environmentId: $environmentId\n    type: $type\n    repo: $repo\n    image: $image\n    registryCredentialId: $registryCredentialId\n    branch: $branch\n    rootDir: $rootDir\n    runtime: $runtime\n    buildCommand: $buildCommand\n    startCommand: $startCommand\n    dockerfilePath: $dockerfilePath\n    buildFilter: $buildFilter\n    plan: $plan\n    autoDeploy: $autoDeploy\n    schedule: $schedule\n    command: $command\n    publishPath: $publishPath\n    port: $port\n    envVars: $envVars\n    secretFiles: $secretFiles\n  ) {\n    id\n    name\n    type\n    phase\n    projectId\n    environmentId\n    registryCredentialId\n    latestDeployId\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "createService": {
          "__typename": "Service",
          "environmentId": null,
          "id": "srv-db5sgu49vsac73cmev20",
          "latestDeployId": "dep-db5sgu49vsac73cmev2g",
          "name": "qa-20261011-a209-rollback",
          "phase": "",
          "projectId": null,
          "registryCredentialId": "",
          "type": "web_service"
        }
      }
    }
  },
  "a209RestoreCommand": {
    "at": "2026-10-11T17:56:49.637Z",
    "status": 200,
    "request": {
      "operationName": "SetStartCommand",
      "variables": {
        "id": "srv-db5sgu49vsac73cmev20",
        "command": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetStartCommand($id: String!, $command: String!, $confirm: String) {\n  setStartCommand(id: $id, command: $command, confirm: $confirm) {\n    id\n    startCommand\n    phase\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "setStartCommand": {
          "__typename": "Service",
          "id": "srv-db5sgu49vsac73cmev20",
          "phase": "Running",
          "startCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
        }
      }
    }
  },
  "a209V2Ready": {
    "at": "2026-10-11T17:57:14.240Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T17:57:00Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "replicas": 1,
          "revision": "rev-6",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys?limit=6",
        "status": 200,
        "body": [
          {
            "deploy": {
              "id": "dep-db5srochj6ks739l0nlg",
              "serviceId": "srv-db5sgu49vsac73cmev20",
              "status": "live",
              "trigger": "service_updated",
              "bexTrigger": "config_change",
              "image": {
                "ref": "python:3.13-alpine"
              },
              "createdAt": "2026-10-11T17:56:49.540333Z",
              "updatedAt": "2026-10-11T17:57:13.971332Z",
              "startedAt": "2026-10-11T17:56:59.05014Z",
              "finishedAt": "2026-10-11T17:57:13.971324Z"
            },
            "cursor": "dep-db5srochj6ks739l0nlg"
          },
          {
            "deploy": {
              "id": "dep-db5sq24hj6ks739l0nk0",
              "serviceId": "srv-db5sgu49vsac73cmev20",
              "status": "deactivated",
              "trigger": "api",
              "bexTrigger": "api",
              "image": {
                "ref": "python:3.13-alpine"
              },
              "createdAt": "2026-10-11T17:53:12.267911Z",
              "updatedAt": "2026-10-11T17:57:13.974324Z",
              "startedAt": "2026-10-11T17:53:14.005774Z",
              "finishedAt": "2026-10-11T17:53:28.854526Z"
            },
            "cursor": "dep-db5sq24hj6ks739l0nk0"
          },
          {
            "deploy": {
              "id": "dep-db5siashj6ks739l0ndg",
              "serviceId": "srv-db5sgu49vsac73cmev20",
              "status": "update_failed",
              "trigger": "service_updated",
              "bexTrigger": "config_change",
              "image": {
                "ref": "python:3.13-alpine"
              },
              "createdAt": "2026-10-11T17:36:43.058555Z",
              "updatedAt": "2026-10-11T17:51:58.886037Z",
              "startedAt": "2026-10-11T17:36:44.186986Z",
              "finishedAt": "2026-10-11T17:51:58.886035Z",
              "failureReason": "container exited shortly after start and is restarting repeatedly (last exit code 37) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped)."
            },
            "cursor": "dep-db5siashj6ks739l0ndg"
          },
          {
            "deploy": {
              "id": "dep-db5sgu49vsac73cmev2g",
              "serviceId": "srv-db5sgu49vsac73cmev20",
              "status": "deactivated",
              "trigger": "other",
              "bexTrigger": "create",
              "image": {
                "ref": "python:3.13-alpine"
              },
              "createdAt": "2026-10-11T17:33:44.609015Z",
              "updatedAt": "2026-10-11T17:53:28.862292Z",
              "finishedAt": "2026-10-11T17:33:58.949131Z"
            },
            "cursor": "dep-db5sgu49vsac73cmev2g"
          }
        ]
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v2\", \"path\": \"/\"}"
    }
  },
  "a209Rollback": {
    "at": "2026-10-11T17:57:39.233Z",
    "status": 200,
    "request": {
      "operationName": "RollbackService",
      "variables": {
        "serviceId": "srv-db5sgu49vsac73cmev20",
        "deployId": "dep-db5sgu49vsac73cmev2g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation RollbackService($serviceId: String!, $deployId: String!, $confirm: String) {\n  rollbackService(\n    serviceId: $serviceId\n    deployId: $deployId\n    disableAutoDeploy: true\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "rollbackService": {
          "__typename": "Deploy",
          "createdAt": "2026-10-11T17:57:39.137694Z",
          "id": "dep-db5ss4shj6ks739l0nn0",
          "image": "python:3.13-alpine",
          "rollbackOf": "dep-db5sgu49vsac73cmev2g",
          "status": "created",
          "trigger": "rollback"
        }
      }
    }
  },
  "a209RollbackReady": {
    "at": "2026-10-11T17:58:10.364Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T17:57:59Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "undeployedChanges": true,
          "replicas": 1,
          "revision": "rev-7",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5ss4shj6ks739l0nn0",
        "status": 200,
        "body": {
          "id": "dep-db5ss4shj6ks739l0nn0",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "update_in_progress",
          "trigger": "rollback",
          "bexTrigger": "rollback",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "rollbackOf": "dep-db5sgu49vsac73cmev2g",
          "createdAt": "2026-10-11T17:57:39.137694Z",
          "updatedAt": "2026-10-11T17:57:44.893438Z",
          "startedAt": "2026-10-11T17:57:44.893437Z"
        }
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v1\", \"path\": \"/\"}"
    },
    "url": "https://dashboard.bex.co/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5ss4shj6ks739l0nn0"
  },
  "a209RestartAfterRollback": {
    "at": "2026-10-11T17:58:12.654Z",
    "status": 200,
    "request": {
      "operationName": "RestartServer",
      "variables": {
        "serviceId": "srv-db5sgu49vsac73cmev20"
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
          "createdAt": "2026-10-11T17:58:12.561409Z",
          "id": "dep-db5ssd4hj6ks739l0no0",
          "image": "python:3.13-alpine",
          "rollbackOf": "",
          "status": "queued",
          "trigger": "api"
        }
      }
    }
  },
  "a209RestartAfterRollbackReady": {
    "at": "2026-10-11T17:58:55.810Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T17:58:23Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "replicas": 1,
          "revision": "rev-8",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5ss4shj6ks739l0nn0",
        "status": 200,
        "body": {
          "id": "dep-db5ss4shj6ks739l0nn0",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "canceled",
          "trigger": "rollback",
          "bexTrigger": "rollback",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "rollbackOf": "dep-db5sgu49vsac73cmev2g",
          "createdAt": "2026-10-11T17:57:39.137694Z",
          "updatedAt": "2026-10-11T17:58:14.060103Z",
          "startedAt": "2026-10-11T17:57:44.893437Z",
          "finishedAt": "2026-10-11T17:58:14.060085Z",
          "cancelReason": "Superseded by dep-db5ssd4hj6ks739l0no0"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5ssd4hj6ks739l0no0",
        "status": 200,
        "body": {
          "id": "dep-db5ssd4hj6ks739l0no0",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "live",
          "trigger": "api",
          "bexTrigger": "api",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "createdAt": "2026-10-11T17:58:12.561409Z",
          "updatedAt": "2026-10-11T17:58:28.878124Z",
          "startedAt": "2026-10-11T17:58:14.070472Z",
          "finishedAt": "2026-10-11T17:58:28.878107Z"
        }
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v2\", \"path\": \"/\"}"
    }
  },
  "a209RollbackSettledRetry": {
    "at": "2026-10-11T17:59:31.786Z",
    "status": 200,
    "request": {
      "operationName": "RollbackService",
      "variables": {
        "serviceId": "srv-db5sgu49vsac73cmev20",
        "deployId": "dep-db5sgu49vsac73cmev2g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation RollbackService($serviceId: String!, $deployId: String!, $confirm: String) {\n  rollbackService(\n    serviceId: $serviceId\n    deployId: $deployId\n    disableAutoDeploy: true\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "rollbackService": {
          "__typename": "Deploy",
          "createdAt": "2026-10-11T17:59:31.660017Z",
          "id": "dep-db5st0s9vsac73cmev7g",
          "image": "python:3.13-alpine",
          "rollbackOf": "dep-db5sgu49vsac73cmev2g",
          "status": "created",
          "trigger": "rollback"
        }
      }
    }
  },
  "a209RollbackSettledRetryPoll1": {
    "at": "2026-10-11T18:00:04.288Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T17:59:43Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "undeployedChanges": true,
          "replicas": 1,
          "revision": "rev-9",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5st0s9vsac73cmev7g",
        "status": 200,
        "body": {
          "id": "dep-db5st0s9vsac73cmev7g",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "live",
          "trigger": "rollback",
          "bexTrigger": "rollback",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "rollbackOf": "dep-db5sgu49vsac73cmev2g",
          "createdAt": "2026-10-11T17:59:31.660017Z",
          "updatedAt": "2026-10-11T17:59:44.029358Z",
          "finishedAt": "2026-10-11T17:59:44.029358Z"
        }
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v1\", \"path\": \"/\"}"
    }
  },
  "a209RestartSettledControl": {
    "at": "2026-10-11T18:00:41.842Z",
    "status": 200,
    "request": {
      "operationName": "RestartServer",
      "variables": {
        "serviceId": "srv-db5sgu49vsac73cmev20"
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
          "createdAt": "2026-10-11T18:00:41.6674Z",
          "id": "dep-db5stic9vsac73cmev8g",
          "image": "python:3.13-alpine",
          "rollbackOf": "",
          "status": "created",
          "trigger": "api"
        }
      }
    }
  },
  "a209RestartSettledControlReady": {
    "at": "2026-10-11T18:01:20.637Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T18:00:52Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "undeployedChanges": true,
          "replicas": 1,
          "revision": "rev-10",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5stic9vsac73cmev8g",
        "status": 200,
        "body": {
          "id": "dep-db5stic9vsac73cmev8g",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "live",
          "trigger": "api",
          "bexTrigger": "api",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "createdAt": "2026-10-11T18:00:41.6674Z",
          "updatedAt": "2026-10-11T18:00:58.9834Z",
          "startedAt": "2026-10-11T18:00:44.038027Z",
          "finishedAt": "2026-10-11T18:00:58.983399Z"
        }
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v1\", \"path\": \"/\"}"
    }
  },
  "a209TimingDeploy": {
    "at": "2026-10-11T18:02:00.329Z",
    "status": 200,
    "request": {
      "operationName": "TriggerDeploy",
      "variables": {
        "serviceId": "srv-db5sgu49vsac73cmev20"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation TriggerDeploy($serviceId: String!, $commitId: String, $deployMode: String, $clearCache: String, $confirm: String) {\n  triggerDeploy(\n    serviceId: $serviceId\n    commitId: $commitId\n    deployMode: $deployMode\n    clearCache: $clearCache\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "triggerDeploy": {
          "__typename": "Deploy",
          "createdAt": "2026-10-11T18:02:00.202631Z",
          "id": "dep-db5su649vsac73cmeva0",
          "image": "python:3.13-alpine",
          "rollbackOf": "",
          "status": "created",
          "trigger": "api"
        }
      }
    }
  },
  "a209TimingRepeat": {
    "samples": [
      {
        "at": "2026-10-11T18:02:22.694Z",
        "status": 200,
        "body": {
          "fixture": "qa-a209",
          "value": "qa-a209-v2",
          "path": "/"
        }
      }
    ],
    "pre": {
      "at": "2026-10-11T18:02:23.005Z",
      "rows": [
        {
          "path": "/v1/services/srv-db5sgu49vsac73cmev20",
          "status": 200,
          "body": {
            "id": "srv-db5sgu49vsac73cmev20",
            "name": "qa-20261011-a209-rollback",
            "immutableName": "qa-20261011-a209-rollback",
            "slug": "qa-20261011-a209-rollback",
            "displayName": "",
            "type": "web_service",
            "suspended": "not_suspended",
            "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
            "createdAt": "2026-10-11T17:33:44Z",
            "updatedAt": "2026-10-11T18:02:20Z",
            "owner": {
              "id": "tea-d98210cbbpdc73dcrkvg",
              "name": "bex",
              "email": "puncsky@gmail.com",
              "type": "team"
            },
            "serviceDetails": {
              "env": "image",
              "envSpecificDetails": {
                "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
              },
              "internalAddress": "qa-20261011-a209-rollback:3000",
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
              "url": "https://qa-20261011-a209-rollback.onbex.co"
            },
            "imagePath": "python:3.13-alpine",
            "suspenders": [],
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "phase": "Running",
            "replicas": 1,
            "revision": "rev-11",
            "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
            "idleTTLSeconds": 0,
            "autoDeploy": "no",
            "autoDeployTrigger": "off",
            "pushDeliveryMethod": "none",
            "notifyOnFail": "default",
            "notificationsToSend": "default"
          }
        },
        {
          "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5su649vsac73cmeva0",
          "status": 200,
          "body": {
            "id": "dep-db5su649vsac73cmeva0",
            "serviceId": "srv-db5sgu49vsac73cmev20",
            "status": "update_in_progress",
            "trigger": "api",
            "bexTrigger": "api",
            "image": {
              "ref": "python:3.13-alpine"
            },
            "createdAt": "2026-10-11T18:02:00.202631Z",
            "updatedAt": "2026-10-11T18:02:14.003122Z",
            "startedAt": "2026-10-11T18:02:14.003121Z"
          }
        },
        {
          "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys?limit=6",
          "status": 200,
          "body": [
            {
              "deploy": {
                "id": "dep-db5su649vsac73cmeva0",
                "serviceId": "srv-db5sgu49vsac73cmev20",
                "status": "update_in_progress",
                "trigger": "api",
                "bexTrigger": "api",
                "image": {
                  "ref": "python:3.13-alpine"
                },
                "createdAt": "2026-10-11T18:02:00.202631Z",
                "updatedAt": "2026-10-11T18:02:14.003122Z",
                "startedAt": "2026-10-11T18:02:14.003121Z"
              },
              "cursor": "dep-db5su649vsac73cmeva0"
            },
            {
              "deploy": {
                "id": "dep-db5stic9vsac73cmev8g",
                "serviceId": "srv-db5sgu49vsac73cmev20",
                "status": "live",
                "trigger": "api",
                "bexTrigger": "api",
                "image": {
                  "ref": "python:3.13-alpine"
                },
                "createdAt": "2026-10-11T18:00:41.6674Z",
                "updatedAt": "2026-10-11T18:00:58.9834Z",
                "startedAt": "2026-10-11T18:00:44.038027Z",
                "finishedAt": "2026-10-11T18:00:58.983399Z"
              },
              "cursor": "dep-db5stic9vsac73cmev8g"
            },
            {
              "deploy": {
                "id": "dep-db5st0s9vsac73cmev7g",
                "serviceId": "srv-db5sgu49vsac73cmev20",
                "status": "deactivated",
                "trigger": "rollback",
                "bexTrigger": "rollback",
                "image": {
                  "ref": "python:3.13-alpine"
                },
                "rollbackOf": "dep-db5sgu49vsac73cmev2g",
                "createdAt": "2026-10-11T17:59:31.660017Z",
                "updatedAt": "2026-10-11T18:00:58.988369Z",
                "finishedAt": "2026-10-11T17:59:44.029358Z"
              },
              "cursor": "dep-db5st0s9vsac73cmev7g"
            },
            {
              "deploy": {
                "id": "dep-db5ssd4hj6ks739l0no0",
                "serviceId": "srv-db5sgu49vsac73cmev20",
                "status": "deactivated",
                "trigger": "api",
                "bexTrigger": "api",
                "image": {
                  "ref": "python:3.13-alpine"
                },
                "createdAt": "2026-10-11T17:58:12.561409Z",
                "updatedAt": "2026-10-11T17:59:44.033355Z",
                "startedAt": "2026-10-11T17:58:14.070472Z",
                "finishedAt": "2026-10-11T17:58:28.878107Z"
              },
              "cursor": "dep-db5ssd4hj6ks739l0no0"
            },
            {
              "deploy": {
                "id": "dep-db5ss4shj6ks739l0nn0",
                "serviceId": "srv-db5sgu49vsac73cmev20",
                "status": "canceled",
                "trigger": "rollback",
                "bexTrigger": "rollback",
                "image": {
                  "ref": "python:3.13-alpine"
                },
                "rollbackOf": "dep-db5sgu49vsac73cmev2g",
                "createdAt": "2026-10-11T17:57:39.137694Z",
                "updatedAt": "2026-10-11T17:58:14.060103Z",
                "startedAt": "2026-10-11T17:57:44.893437Z",
                "finishedAt": "2026-10-11T17:58:14.060085Z",
                "cancelReason": "Superseded by dep-db5ssd4hj6ks739l0no0"
              },
              "cursor": "dep-db5ss4shj6ks739l0nn0"
            },
            {
              "deploy": {
                "id": "dep-db5srochj6ks739l0nlg",
                "serviceId": "srv-db5sgu49vsac73cmev20",
                "status": "deactivated",
                "trigger": "service_updated",
                "bexTrigger": "config_change",
                "image": {
                  "ref": "python:3.13-alpine"
                },
                "createdAt": "2026-10-11T17:56:49.540333Z",
                "updatedAt": "2026-10-11T17:58:28.889343Z",
                "startedAt": "2026-10-11T17:56:59.05014Z",
                "finishedAt": "2026-10-11T17:57:13.971324Z"
              },
              "cursor": "dep-db5srochj6ks739l0nlg"
            }
          ]
        }
      ]
    },
    "dialog": "- alertdialog \"Restart qa-20261011-a209-rollback?\":\n  - heading \"Restart qa-20261011-a209-rollback?\" [level=2]\n  - paragraph: qa-20261011-a209-rollback restarts on the commit or image it is running now. Commits pushed since are not deployed. New instances replace the old ones once they are healthy.\n  - status\n  - button \"Cancel\"\n  - button \"Restart\"",
    "receipt": {
      "at": "2026-10-11T18:02:24.059Z",
      "status": 200,
      "request": {
        "operationName": "RestartServer",
        "variables": {
          "serviceId": "srv-db5sgu49vsac73cmev20"
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
            "createdAt": "2026-10-11T18:02:23.97683Z",
            "id": "dep-db5subs9vsac73cmevb0",
            "image": "python:3.13-alpine",
            "rollbackOf": "",
            "status": "queued",
            "trigger": "api"
          }
        }
      }
    }
  },
  "a209TimingRepeatOutcome": {
    "at": "2026-10-11T18:03:12.227Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T18:02:34Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "undeployedChanges": true,
          "replicas": 1,
          "revision": "rev-12",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5su649vsac73cmeva0",
        "status": 200,
        "body": {
          "id": "dep-db5su649vsac73cmeva0",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "canceled",
          "trigger": "api",
          "bexTrigger": "api",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "createdAt": "2026-10-11T18:02:00.202631Z",
          "updatedAt": "2026-10-11T18:02:28.862746Z",
          "startedAt": "2026-10-11T18:02:14.003121Z",
          "finishedAt": "2026-10-11T18:02:28.862745Z",
          "cancelReason": "Superseded by dep-db5subs9vsac73cmevb0"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5subs9vsac73cmevb0",
        "status": 200,
        "body": {
          "id": "dep-db5subs9vsac73cmevb0",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "live",
          "trigger": "api",
          "bexTrigger": "api",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "createdAt": "2026-10-11T18:02:23.97683Z",
          "updatedAt": "2026-10-11T18:02:44.389886Z",
          "startedAt": "2026-10-11T18:02:28.880161Z",
          "finishedAt": "2026-10-11T18:02:44.389884Z"
        }
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v1\", \"path\": \"/\"}"
    },
    "omitted": "Whole-page UI text only; API response bodies complete"
  },
  "a209FinalDeploy": {
    "at": "2026-10-11T18:03:33.632Z",
    "status": 200,
    "request": {
      "operationName": "TriggerDeploy",
      "variables": {
        "serviceId": "srv-db5sgu49vsac73cmev20"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation TriggerDeploy($serviceId: String!, $commitId: String, $deployMode: String, $clearCache: String, $confirm: String) {\n  triggerDeploy(\n    serviceId: $serviceId\n    commitId: $commitId\n    deployMode: $deployMode\n    clearCache: $clearCache\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "triggerDeploy": {
          "__typename": "Deploy",
          "createdAt": "2026-10-11T18:03:33.520003Z",
          "id": "dep-db5sutc9vsac73cmevc0",
          "image": "python:3.13-alpine",
          "rollbackOf": "",
          "status": "created",
          "trigger": "api"
        }
      }
    }
  },
  "a209FinalDeployReady": {
    "at": "2026-10-11T18:04:16.649Z",
    "rows": [
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20",
        "status": 200,
        "body": {
          "id": "srv-db5sgu49vsac73cmev20",
          "name": "qa-20261011-a209-rollback",
          "immutableName": "qa-20261011-a209-rollback",
          "slug": "qa-20261011-a209-rollback",
          "displayName": "",
          "type": "web_service",
          "suspended": "not_suspended",
          "dashboardUrl": "https://dashboard.bex.co/web/srv-db5sgu49vsac73cmev20",
          "createdAt": "2026-10-11T17:33:44Z",
          "updatedAt": "2026-10-11T18:03:44Z",
          "owner": {
            "id": "tea-d98210cbbpdc73dcrkvg",
            "name": "bex",
            "email": "puncsky@gmail.com",
            "type": "team"
          },
          "serviceDetails": {
            "env": "image",
            "envSpecificDetails": {
              "dockerCommand": "exec python -u -c 'exec(\"import os,json\\nfrom http.server import BaseHTTPRequestHandler,HTTPServer\\nclass Handler(BaseHTTPRequestHandler):\\n def do_GET(self):\\n  payload=json.dumps({\\\"fixture\\\":\\\"qa-a209\\\",\\\"value\\\":os.environ.get(\\\"QA_A209_MARKER\\\"),\\\"path\\\":self.path}).encode()\\n  self.send_response(200);self.send_header(\\\"Content-Type\\\",\\\"application/json\\\");self.send_header(\\\"Content-Length\\\",str(len(payload)));self.end_headers();self.wfile.write(payload)\\n def do_HEAD(self):\\n  self.send_response(200);self.end_headers()\\n def log_message(self,*args):pass\\nHTTPServer((\\\"0.0.0.0\\\",int(os.environ[\\\"PORT\\\"])),Handler).serve_forever()\\n\")'"
            },
            "internalAddress": "qa-20261011-a209-rollback:3000",
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
            "url": "https://qa-20261011-a209-rollback.onbex.co"
          },
          "imagePath": "python:3.13-alpine",
          "suspenders": [],
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "phase": "Running",
          "replicas": 1,
          "revision": "rev-13",
          "urls": ["https://qa-20261011-a209-rollback.onbex.co"],
          "idleTTLSeconds": 0,
          "autoDeploy": "no",
          "autoDeployTrigger": "off",
          "pushDeliveryMethod": "none",
          "notifyOnFail": "default",
          "notificationsToSend": "default"
        }
      },
      {
        "path": "/v1/services/srv-db5sgu49vsac73cmev20/deploys/dep-db5sutc9vsac73cmevc0",
        "status": 200,
        "body": {
          "id": "dep-db5sutc9vsac73cmevc0",
          "serviceId": "srv-db5sgu49vsac73cmev20",
          "status": "live",
          "trigger": "api",
          "bexTrigger": "api",
          "image": {
            "ref": "python:3.13-alpine"
          },
          "createdAt": "2026-10-11T18:03:33.520003Z",
          "updatedAt": "2026-10-11T18:03:58.816639Z",
          "finishedAt": "2026-10-11T18:03:58.816637Z"
        }
      }
    ],
    "public": {
      "status": 200,
      "body": "{\"fixture\": \"qa-a209\", \"value\": \"qa-a209-v2\", \"path\": \"/\"}"
    }
  }
}
```
