# A canceled deploy's default log window hides the pre-deploy step's later output

Why: a migration that finishes after Cancel can print its most useful success or error line after the deploy's cancellation time. The deploy page reports the final step outcome but its default log window omits that line and searching reports “No matching logs.”

**Severity:** major (misleading diagnostic results, with a wider-range workaround). **Estimate:** seven tasks, ~2h 40m. Functional `qa-find-bugs` pass a147, 2026-10-10 local / 2026-10-11 UTC; QA credentials were privately loaded from `muse.env`. Workspace `tea-d98210cbbpdc73dcrkvg`. Fixture `srv-db5glu8k3t2s73e5vb2g`, `qa-20261010-a147-predeploy`, deleted; session revoked. Filing only.

## Reproduction

1. Create an owned Free web service with Existing Image `docker.io/library/busybox:1.37`, port 3000, and the command below. Wait for Live and public HTTP 200. Stage `docker.io/library/busybox:1.36` with Settings → Source → Edit → Update source; source changes wait for a carrying deploy.
2. Settings → Edit Pre-Deploy Command → `printf 'qa-a147-cancel-start\n'; sleep 45; printf 'qa-a147-cancel-complete\n'` → Save changes. This save starts a configuration deploy. Wait for “Pre-deploy command running.”
3. Open that deploy, Cancel → Proceed while the command sleeps. Let the existing step finish, without triggering another release. The deploy remains `canceled`; the step eventually reads `succeeded`.
4. Fresh-load the deploy at desktop 1440×1000 or mobile 390×844. Search `qa-a147-cancel-complete` in All logs with the default Deploy window: “No matching logs.” Log options → Last 15 minutes shows the exact completion line. Switching back to Deploy window hides it again.
5. Repeat with `printf 'qa-a147-cancel-failure-start\n'; sleep 35; printf 'qa-a147-cancel-failure-end\n'; exit 23`, cancel while it runs, and wait for its verdict. The canceled deploy's step reads `failed`; the default search hides the final error marker, and Last 15 minutes reveals it. The desktop replay and a fresh mobile replay both reproduced this.

Fixture command (only synthetic marker/version output):

```sh
sh -c 'mkdir -p /tmp/qa-a147; { printf "qa-a147-runtime\n"; busybox | head -n 1; } > /tmp/qa-a147/index.html; exec httpd -f -p "$PORT" -h /tmp/qa-a147'
```

## Observed timing and controls

| Case | Deploy | Cancel / deploy finishedAt | Step output after cancel | Matching step-end fact | Result |
| --- | --- | --- | --- | --- | --- |
| Successful step | `dep-db5gnf0k3t2s73e5vb5g` | `04:09:01.5668Z` | completion `04:09:14.773261156Z` | `04:09:17Z` | canceled deploy, succeeded step; default output missing |
| Error step | `dep-db5gq372659s73bbul30` | `04:14:23.441207Z` | error marker `04:14:40.414394093Z` | `04:14:42Z` | canceled deploy, failed step; default output missing |

All instants are 2026-10-11 UTC. The successful step printed its start at `04:08:29.772991661Z` and finished 45 seconds later. The error step printed its start at `04:14:05.414283981Z` and the final marker 35 seconds later. Each start is before the projected deploy startedAt (`04:08:41.494919Z` and `04:14:12.413098Z`), so keep the existing createdAt lower bound.

- Two uncanceled exit-23 deployments closed `pre_deploy_failed` with the actionable exit-code reason. The service remained Running on the original release. All, Build, and Application log buckets displayed the first failed step's markers, preserving the w4/180 source-membership control.
- All 20 public samples during/after the staged-image failure and all 25 samples during/after the successful cancellation returned HTTP 200 with BusyBox 1.37.0, the old image. These samples do not prove EndpointSlice membership or close w4/m181.
- After the success cancellation, the step's API verdict was `succeeded` while an earlier page sample still said canceled. This candidate was **dropped**: a later screenshot taken before reload already said succeeded. `useLatestDeploy` polls DeploysDocument at the baseline cadence; the same normalized Deploy carries preDeployStatus into the detail read. No permanently stale verdict is claimed.
- Clearing Pre-Deploy Command started recovery `dep-db5gr5f2659s73bbul5g`, Live at `04:16:41.493695Z`; public HTTP 200 served BusyBox 1.36.1. Both pending-change flags cleared. The six deploy rows were two normal failed steps, two canceled releases with correct final step verdicts, the original deactivated release, and recovery Live.
- Console capture: zero errors/warnings. Expected navigation aborts are not findings. The data queries below all returned HTTP 200.

## Durable probes

Browser POST to `https://api.bex.co/graphql`, JSON content type, normal signed-in QA cookies supplied by the browser. Credentials are deliberately absent. These exact requests and complete responses establish the cutoff independently of screenshots.

### Successful step: cancellation-bound query

```json
{
  "at": "2026-10-11T04:12:27.962Z",
  "status": 200,
  "request": {
    "operationName": "QaA147CanceledWindow",
    "query": "query QaA147CanceledWindow($id:String!,$dep:String!,$start:String!,$end:String!){deploy(serviceId:$id,deployId:$dep){id status preDeployStatus createdAt updatedAt startedAt finishedAt} serviceEvents(serviceId:$id,startTime:$start,limit:100){id type timestamp details{deployId deployStatus}} build:logs(resource:$id,type:\"build\",text:\"qa-a147-cancel\",startTime:$start,endTime:$end,limit:100){hasMore logs{timestamp message type instance}} predeploy:logs(resource:$id,type:\"predeploy\",text:\"qa-a147-cancel\",startTime:$start,endTime:$end,limit:100){hasMore logs{timestamp message type instance}}}",
    "variables": {
      "id": "srv-db5glu8k3t2s73e5vb2g",
      "dep": "dep-db5gnf0k3t2s73e5vb5g",
      "start": "2026-10-11T04:08:28.610523Z",
      "end": "2026-10-11T04:09:01.5668Z"
    }
  },
  "response": {
    "data": {
      "build": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-nvej7lbkin5snjptmhao",
            "message": "qa-a147-cancel-start",
            "timestamp": "2026-10-11T04:08:29.772991661Z",
            "type": "build"
          }
        ]
      },
      "deploy": {
        "createdAt": "2026-10-11T04:08:28.610523Z",
        "finishedAt": "2026-10-11T04:09:01.5668Z",
        "id": "dep-db5gnf0k3t2s73e5vb5g",
        "preDeployStatus": "succeeded",
        "startedAt": "2026-10-11T04:08:41.494919Z",
        "status": "canceled",
        "updatedAt": "2026-10-11T04:09:58.252713Z"
      },
      "predeploy": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-nvej7lbkin5snjptmhao",
            "message": "qa-a147-cancel-start",
            "timestamp": "2026-10-11T04:08:29.772991661Z",
            "type": "predeploy"
          }
        ]
      },
      "serviceEvents": [
        {
          "details": {
            "deployId": "dep-db5gnf0k3t2s73e5vb5g",
            "deployStatus": ""
          },
          "id": "evt-sagp2f8t5g64q0ugvs7j",
          "timestamp": "2026-10-11T04:09:17Z",
          "type": "pre_deploy_ended"
        },
        {
          "details": {
            "deployId": "dep-db5gnf0k3t2s73e5vb5g",
            "deployStatus": "canceled"
          },
          "id": "evt-fs7mqtuottjkd48cjd04",
          "timestamp": "2026-10-11T04:09:01Z",
          "type": "deploy_ended"
        },
        {
          "details": {
            "deployId": "",
            "deployStatus": ""
          },
          "id": "evt-1o48l0kn7d0e6i6tvs0f",
          "timestamp": "2026-10-11T04:08:28Z",
          "type": "pre_deploy_command_changed"
        },
        {
          "details": {
            "deployId": "dep-db5gnf0k3t2s73e5vb5g",
            "deployStatus": ""
          },
          "id": "evt-j70ngjj3q94qu43hnl89",
          "timestamp": "2026-10-11T04:08:28Z",
          "type": "deploy_started"
        }
      ]
    }
  }
}
```

### Successful step: finite end bound moved to its matching step-end fact

```json
{
  "at": "2026-10-11T04:12:47.194Z",
  "status": 200,
  "request": {
    "operationName": "QaA147CanceledWindowExtended",
    "query": "query QaA147CanceledWindowExtended($id:String!,$dep:String!,$start:String!,$end:String!){deploy(serviceId:$id,deployId:$dep){id status preDeployStatus createdAt updatedAt startedAt finishedAt} serviceEvents(serviceId:$id,startTime:$start,limit:100){id type timestamp details{deployId deployStatus}} build:logs(resource:$id,type:\"build\",text:\"qa-a147-cancel\",startTime:$start,endTime:$end,limit:100){hasMore logs{timestamp message type instance}} predeploy:logs(resource:$id,type:\"predeploy\",text:\"qa-a147-cancel\",startTime:$start,endTime:$end,limit:100){hasMore logs{timestamp message type instance}}}",
    "variables": {
      "id": "srv-db5glu8k3t2s73e5vb2g",
      "dep": "dep-db5gnf0k3t2s73e5vb5g",
      "start": "2026-10-11T04:08:28.610523Z",
      "end": "2026-10-11T04:09:17Z"
    }
  },
  "response": {
    "data": {
      "build": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-nvej7lbkin5snjptmhao",
            "message": "qa-a147-cancel-start",
            "timestamp": "2026-10-11T04:08:29.772991661Z",
            "type": "build"
          },
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-nvej7lbkin5snjptmhao",
            "message": "qa-a147-cancel-complete",
            "timestamp": "2026-10-11T04:09:14.773261156Z",
            "type": "build"
          }
        ]
      },
      "deploy": {
        "createdAt": "2026-10-11T04:08:28.610523Z",
        "finishedAt": "2026-10-11T04:09:01.5668Z",
        "id": "dep-db5gnf0k3t2s73e5vb5g",
        "preDeployStatus": "succeeded",
        "startedAt": "2026-10-11T04:08:41.494919Z",
        "status": "canceled",
        "updatedAt": "2026-10-11T04:09:58.252713Z"
      },
      "predeploy": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-nvej7lbkin5snjptmhao",
            "message": "qa-a147-cancel-start",
            "timestamp": "2026-10-11T04:08:29.772991661Z",
            "type": "predeploy"
          },
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-nvej7lbkin5snjptmhao",
            "message": "qa-a147-cancel-complete",
            "timestamp": "2026-10-11T04:09:14.773261156Z",
            "type": "predeploy"
          }
        ]
      },
      "serviceEvents": [
        {
          "details": {
            "deployId": "dep-db5gnf0k3t2s73e5vb5g",
            "deployStatus": ""
          },
          "id": "evt-sagp2f8t5g64q0ugvs7j",
          "timestamp": "2026-10-11T04:09:17Z",
          "type": "pre_deploy_ended"
        },
        {
          "details": {
            "deployId": "dep-db5gnf0k3t2s73e5vb5g",
            "deployStatus": "canceled"
          },
          "id": "evt-fs7mqtuottjkd48cjd04",
          "timestamp": "2026-10-11T04:09:01Z",
          "type": "deploy_ended"
        },
        {
          "details": {
            "deployId": "",
            "deployStatus": ""
          },
          "id": "evt-1o48l0kn7d0e6i6tvs0f",
          "timestamp": "2026-10-11T04:08:28Z",
          "type": "pre_deploy_command_changed"
        },
        {
          "details": {
            "deployId": "dep-db5gnf0k3t2s73e5vb5g",
            "deployStatus": ""
          },
          "id": "evt-j70ngjj3q94qu43hnl89",
          "timestamp": "2026-10-11T04:08:28Z",
          "type": "deploy_started"
        }
      ]
    }
  }
}
```

### Error step: cancellation-bound and later-output controls

```json
{
  "at": "2026-10-11T04:16:16.996Z",
  "status": 200,
  "request": {
    "operationName": "QaA147CanceledFailureWindow",
    "query": "query QaA147CanceledFailureWindow($id:String!,$dep:String!,$start:String!,$end:String!){deploy(serviceId:$id,deployId:$dep){id status preDeployStatus createdAt updatedAt startedAt finishedAt} serviceEvents(serviceId:$id,type:\"pre_deploy_ended\",startTime:$start,limit:100){id type timestamp details{deployId deployStatus}} bounded:logs(resource:$id,type:\"build\",text:\"qa-a147-cancel-failure\",startTime:$start,endTime:$end,limit:100){hasMore logs{timestamp message type instance}} unbounded:logs(resource:$id,type:\"build\",text:\"qa-a147-cancel-failure\",startTime:$start,limit:100){hasMore logs{timestamp message type instance}}}",
    "variables": {
      "id": "srv-db5glu8k3t2s73e5vb2g",
      "dep": "dep-db5gq372659s73bbul30",
      "start": "2026-10-11T04:14:04.170318Z",
      "end": "2026-10-11T04:14:23.441207Z"
    }
  },
  "response": {
    "data": {
      "bounded": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-ff9r16kq48p5bh8s1f37",
            "message": "qa-a147-cancel-failure-start",
            "timestamp": "2026-10-11T04:14:05.414283981Z",
            "type": "build"
          }
        ]
      },
      "deploy": {
        "createdAt": "2026-10-11T04:14:04.170318Z",
        "finishedAt": "2026-10-11T04:14:23.441207Z",
        "id": "dep-db5gq372659s73bbul30",
        "preDeployStatus": "failed",
        "startedAt": "2026-10-11T04:14:12.413098Z",
        "status": "canceled",
        "updatedAt": "2026-10-11T04:14:58.242833Z"
      },
      "serviceEvents": [
        {
          "details": {
            "deployId": "dep-db5gq372659s73bbul30",
            "deployStatus": ""
          },
          "id": "evt-su4fhv4sghiftsoq5dtr",
          "timestamp": "2026-10-11T04:14:42Z",
          "type": "pre_deploy_ended"
        }
      ],
      "unbounded": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-ff9r16kq48p5bh8s1f37",
            "message": "qa-a147-cancel-failure-start",
            "timestamp": "2026-10-11T04:14:05.414283981Z",
            "type": "build"
          },
          {
            "instance": "srv-db5glu8k3t2s73e5vb2g-ff9r16kq48p5bh8s1f37",
            "message": "qa-a147-cancel-failure-end",
            "timestamp": "2026-10-11T04:14:40.414394093Z",
            "type": "build"
          }
        ]
      }
    }
  }
}
```

## Root cause and target behavior

- `dashboard/src/features/deploys/components/deploy-detail-page.tsx:97–106` supplies `startTime = deploy.createdAt` and `endTime = deploy.finishedAt` to the log panel for the default window. Cancellation closes the deploy before its pre-deploy step finishes, by ADR004's deliberate migration-completion policy.
- `dashboard/src/features/deploys/hooks/use-deploy-logs.ts:151–172` fans that **same** window into separate build, predeploy and app reads. The ingestion grace at 108–125 re-reads the same closed bound; waiting or extending its timer cannot recover a later timestamp.
- `lego/backend/internal/logs/service.go:487–502` drops a live-source row after q.End; the durable build leg also respects the requested window. The complete bound/unbound captures show correct filtering, not data loss or an Apollo cache bug.
- `lego/backend/internal/store/store.go:2477–2498` correctly settles the canceled row's step verdict and records `pre_deploy_ended` for that deploy, without replacing deploy finishedAt. `store/reconciler.go:1183–1224` gives that fact an explicit deployId and the step's own completion time. Keep these semantics.
- `lego/backend/internal/events/graphql.go:168–177` already exposes serviceEvents with type, startTime, endTime, cursor and limit; event details expose deployId. The event is available **after** cancellation (shown above), so querying this fact with endTime = cancellation would repeat the original defect.

**Preferred bounded fix:** keep this in the deploy feature. For a canceled deploy whose step has a final succeeded/failed verdict, resolve its matching `pre_deploy_ended` through the existing typed/paged events API. Use a frozen event-search ceiling after the step outcome was observed, retain cursor consistency, and match both service scope and details.deployId. Fetch cursor pages until the matching fact is found or the window is exhausted; do not use whichever step ended most recently, or stop after a capped first page.

The default diagnostic end becomes the later of deploy finishedAt and **that deploy's** step-end fact. Preserve createdAt as the start. Account for the event timestamp's wire precision when choosing an inclusive log bound, so same-second fractional log timestamps are not clipped. Use existing events rather than inferring completion from browser time, a fixed extra 15 minutes, or arbitrary updatedAt; updatedAt may serve as a fixed event-search ceiling, not the displayed completion bound. No database migration or new public API field is required by this proposed fix.

Deploy finishedAt, displayed duration, status, image choice, cancel reason, generation ownership, Job completion, serving configuration and cancellation policy remain correct. Last 15 minutes and the other `?r=` overrides retain their existing open-range behavior. Ordinary live/failed/deactivated/no-step and actually-stopped/superseded-step cases keep their normal bounds.

**Before the end fact settles:** preserve the canceled status and correct step verdict, show a checking log-window state while resolving the fact, and do not flash an authoritative empty result using the known-truncated cancellation bound. A missing legacy fact or a read failure is an explicit limitation/retry state with the existing range option; do not silently claim complete step output or poll indefinitely for a step that was actually stopped.

**Actual dependency inspected:** live mutation metadata pins Apollo Client 4.1.3. Its installed react/hooks/useQuery.js:176–200 reobserves when variables change; QueryManager.js:728–737 applies the selected fetch policy/cache write behavior. The actual request endTime is already too early, and moving it in an independent browser query returns the row. Aim regression tests at the caller's window and real query responses, not a mocked list that already includes the missing row.

## Counted scope, aliases and verification boundaries

Exhaustive production grep (tests excluded): one `useDeployLogs` caller, DeployLogPanel; one panel mount, DeployDetailPage; two detail route mounts, `services.$serviceId.deploys.$deployId.tsx:29` and `static.$serviceId.deploys.$deployId.tsx:24`. One `useDeploy` caller and one `useDeployTimeline` caller. This change is allowlisted to canceled deploys with an evidenced continued step; the common log mapping/dedupe/filter primitives need no global change.

The service route family covers web/private/worker/cron, and the specific static detail route is a sibling. Pre-deploy runs only for web/private/worker; cron/static have no such step. Postgres and Key Value have different viewers and are controls, not live affected cases. Only Free web was exercised here. `common/lib/render-alias.ts` maps /web, /pserv, /worker, /cron and legacy static catch-all shims to /services; the specific /static detail route also mounts the shared page. These aliases must converge on the same bounded detail behavior. The helper and web/worker shims were opened in source; alias URLs were not live-probed.

Existing read adapters to retain: GraphQL deploy/deploys, serviceEvents and the separate typed logs reads; REST GET `/v1/services/{id}/deploys/{deployId}`, deploy list, `/v1/services/{id}/events`, and `/v1/logs`; MCP get_deploy/list_deploys/list_events/list_logs, with list_service_events as the legacy service-events alias (different default lookback and envelope, same service facts). The preferred change consumes existing fields, so the adapters' error handling, time filters and event cursor semantics stay intact. REST/MCP equivalents were traced in source but not called in this pass.

**Unverified:** suspended-step cancellation, first-release cancellation, actual superseding/stopped Job cases, historical facts behind >100 typed events, log persistence after Job TTL, paid worker/private fixtures, cron/static/datastore UI controls, the legacy alias URLs, Chinese UI, and authenticated Render UI. Carry these into compatibility/tests; do not turn them into additional observed failures. Timeline's missing post-cancel event is a look-alike surface and is not filed here.

## Dedupe and precedent

- w5/done/085.md was read in full. Its implemented guarantee is that an already-running canceled step can finish, the final verdict/fact belongs to the canceled generation, and no serving rollout/failure follows. This pass exercised successful and error outcomes, retained the canceled deploy and old image, and recovered normally. Suspend and first-release test classes were not replayed live. Its backend verdict/fact fix is deployed and working; the log-window consumer remained unchanged. This is the additional integration gap, not a claim its backend fix regressed.
- w4/done/187.md settles superseded running verdicts. It does not widen a continued step's log bounds. Its actual-stopped/superseded classes must remain terminal.
- w9/done/m1 README and t003 were opened: createdAt..finishedAt was the original deploy viewer contract before the post-cancel completion integration. Preserve its normal windows, per-deploy route, API filters and error behavior; do not describe that original implementation as a new failed backend guarantee.
- w4/blocked/180.md is cross-source bucket dedupe; all default buckets already contain the missing row there. This pass's ordinary failed-step bucket control passed. Widening time bounds was correctly excluded from that separate fix.
- w4/blocked/m181 is serving endpoint membership, with a 90-second/EndpointSlice closeout. This pass's HTTP controls passed but did not inspect endpoints or cover its whole guarantee. No duplicate availability filing or milestone closure.
- w4/blocked/m156 concerns unknown deploy starts; its existing createdAt-versus-startedAt distinction is preserved.
- Targeted searches covered open/done board items and all 55 open/blocked milestone titles. DO_NOT_DO was re-read; no anti-goal conflict. Recent 40 product commits and targeted history were inspected after fetch/pull; HEAD/origin main `db3fd8e8a` still has the too-early caller bound. No already-landed window fix/deploy-lag finding.

Render's [deployment documentation](https://render.com/docs/deploys#pre-deploy-command), checked 2026-10-11, describes the separate pre-deploy instance and preservation of the prior successful deploy after failure. It does not establish this exact authenticated post-cancel log-window behavior. bex's continued-migration policy is ADR004's own contract; no exact Render UI match is claimed. A guessed `/docs/pre-deploy-commands` URL returned 404, so it is not a cited source.

## Local evidence and cleanup

Verified present and viewed: `.playwright-mcp/qa-a147-canceled-step-output-desktop.png` (succeeded step + missing completion search) and `.playwright-mcp/qa-a147-canceled-step-output-mobile.png` (error-step search omitted in default window). The mobile screenshot is scrolled to the missing result; use the durable API capture for the final verdict. The earlier `qa-a147-canceled-step-stale-mobile.png` shows succeeded and is **not** evidence of a stale verdict.

Ledger: `.playwright-mcp/qa-predeploy-window-a147-ledger.json`. Public samplers: `.playwright-mcp/qa-a147-fail-public.json` (20 rows) and `.playwright-mcp/qa-a147-cancel-public.json` (25 rows). Artifacts are gitignored; the requests/responses above are the durable handoff.

DeleteService returned true at `04:17:29.902Z`; REST GET returned 404 at `04:17:44.280Z`. Services, databases, Key Value, projects, groups and Blueprints all matched the starting ID sets. Own logout succeeded; whoami 401 at `04:18:01.786Z`, browser cookies zero, about:blank, and only own session handles removed.
