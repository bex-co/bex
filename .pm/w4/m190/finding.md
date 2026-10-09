# HTTP health probes reject a healthy hostname-restricted web app

Severity: **blocker** for the observed web-service HTTP-health deploy journey. Cycle32, 2026-10-09 UTC; production dashboard, English, workspace `bex` (`tea-d98210cbbpdc73dcrkvg`), credentials handled privately by qa-login.sh. This files work; it does not implement the fix.

## Reproduction and observed controls

1. Create the owned Free prebuilt web service `qa-20261009-loop-a32-health`, BusyBox `docker.io/library/busybox:1.37`, port 3000. Actual service `srv-db4e35393q6c73at029g`.
2. With TCP default, the first deploy reaches Live. HTTP paths `/healthz?check=a32` and literal `/qa health` also deploy Live. Padding the query-path draft with edge spaces normalizes to the saved path without another deploy.
3. Save the Docker Command below. Keep the working space-path health check for this save. Deploy `dep-db4e5cj4am4s73f095s0` reaches Live at 12:49:13.651575Z. Public `/cgi-bin/health` returns 200 and `QA_A32_HOST_OK\n`, in the browser and external curl.
4. Settings → Edit Health Check Path → `/cgi-bin/health` → Save changes. The accepted mutation opens `dep-db4e5tj93q6c73at02d0`, started 12:50:13.600367Z. Its pod repeatedly logs `QA_A32_PROBE_HOST=10.244.128.252:3000`; the fixture's exact branch below rejects that Host with 421. Public requests keep using the service hostname and return 200 from the prior release.
5. Fresh-load the deploy detail. At 12:55:04.689Z the dashboard, GraphQL, REST and MCP report startup-health waiting, not Live. The public endpoint still returns 200. Saved path and serving revision rev-4 differ intentionally while this deploy runs; that is not a separate pending-state bug.

The settled failure, recovery, fresh repeat and cleanup are recorded below.

### Fixture command

Use a new owned service name and substitute its actual platform hostname in the CGI allowlist. No environment variables, secrets, domain claims, project or environment were attached.

```sh
mkdir -p /tmp/qa-www; echo QA_A32_HTTP_OK > /tmp/qa-www/index.html; echo QA_A32_HEALTH_OK > /tmp/qa-www/healthz; echo QA_A32_SPACE_OK > "/tmp/qa-www/qa health"; mkdir -p /tmp/qa-www/cgi-bin; printf '%s\n' '#!/bin/sh' 'echo "QA_A32_PROBE_HOST=$HTTP_HOST" >&2' 'case "$HTTP_HOST" in' '  qa-20261009-loop-a32-health.onbex.co|qa-20261009-loop-a32-health.onbex.co:*)' '    printf '\''Status: 200 OK\r\nContent-Type: text/plain\r\n\r\nQA_A32_HOST_OK\n'\'' ;;' '  *)' '    printf '\''Status: 421 Misdirected Request\r\nContent-Type: text/plain\r\n\r\nQA_A32_HOST_REJECT host=%s\n'\'' "$HTTP_HOST" ;;' 'esac' > /tmp/qa-www/cgi-bin/health; chmod +x /tmp/qa-www/cgi-bin/health; echo QA_A32_START; exec httpd -f -p "$PORT" -h /tmp/qa-www
```

The command writes this CGI:

```sh
#!/bin/sh
echo "QA_A32_PROBE_HOST=$HTTP_HOST" >&2
case "$HTTP_HOST" in
  qa-20261009-loop-a32-health.onbex.co|qa-20261009-loop-a32-health.onbex.co:*)
    printf 'Status: 200 OK\r\nContent-Type: text/plain\r\n\r\nQA_A32_HOST_OK\n' ;;
  *)
    printf 'Status: 421 Misdirected Request\r\nContent-Type: text/plain\r\n\r\nQA_A32_HOST_REJECT host=%s\n' "$HTTP_HOST" ;;
esac

```

The 421 is inferred from this captured command and the actual incoming Host logs; no direct pod-IP HTTP response was independently captured. The public 200 is directly captured. A distinct new instance emitted ten-second Host records and the deployment reported startup-health rejection; the public route was accessible.

## Expected behavior and researched fix

For a **new tracked web-service release** with an HTTP health path, connect to that pod's IP and service port, but send one Host header containing a service hostname: prefer an admitted custom host (`Spec.Host`, then the first `Spec.Hosts` entry), otherwise the enabled canonical platform hostname. Managed pending domain claims are excluded from those fields. Do not pick `EffectiveHosts()[0]` blindly: that ordering puts the platform host before additional custom hosts. Do not synthesize from the display name or the tenant CR name when a canonical subdomain exists.

Add an explicit projection input to `deploymentParams`; the same `HTTPHeaders: []HTTPHeader{{Name:"Host", Value: selectedFQDN}}` must reach startup, readiness and liveness. Leave `HTTPGet.Host` empty: that field changes the network destination. Setting it to the public hostname would test ingress/the prior release rather than the new instance, and can induce public wake traffic or false readiness.

**Release safety is part of the fix.** Select and retain the Host for a release when its legitimate rollout is projected. Repeated reconciles and operational domain/subdomain changes within that release must preserve its existing probe Host, including an absent Host on a legacy template. Reuse the same-release applied/recorded template through the uncached release-record mechanism; the record contains the complete probe. A new normal deploy selects from current admitted routing hosts. Cancel and failed-rollout restoration retain the served template verbatim. A new rollback combines its historical health path with current domains, as ADR004 requires; do not restore historical domains by copying the whole historical template. Explicitly test Restart's retained configuration and record the chosen Host behavior. Do not silently roll existing services on operator upgrade or rewrite old records to manufacture a migration.

The public Render docs specify hostname choice, not the timing of reselection after an operational domain edit. Bex's existing domains-are-operational contract requires the above per-release retention unless a separately designed, recorded rollout policy is introduced. Record this timing limitation in parity documentation; do not claim it was live-verified against Render. No new API/custom-header feature is needed for this finding.

### Root cause and actual consumers

- `lego/operator/internal/controller/deployment_projection.go:298–306`: `healthCheckHandler(path, port)` creates HTTPGet with only Path and Port; HTTPHeaders and connection Host are absent.
- `:263–280`: **one production helper call**, copied onto startup, readiness and liveness. Timeout remains 5 seconds; startup and deadline remain 900 seconds; liveness stays 60 seconds. Changing all three through the helper prevents disagreement.
- `:202`, `:417`: **one production appContainer call**, from projectDeployment. `:155–171` params have no hostname/base-domain input.
- `lego/operator/internal/controller/app_controller.go:2461–2473`: **one production deploymentParams construction**; reconciler BaseDomain is available. Cron/static diverge earlier at :2408–2419; worker bypasses probes.
- `lego/types/v1alpha1/app_types.go:910–940`: canonical EffectiveHosts / PlatformSubdomain contract and public-type allowlist. It can supply the platform fallback; custom-first probe precedence differs from public-URL precedence.
- `lego/backend/internal/apps/domains.go:583–623`: verified-only Host/Hosts projection. Three calls to projectDomainClaims (:775, :1381, :1499), plus storeless add/delete direct patch (:1426, :1536). No operator database/DNS ownership lookup is appropriate.
- `lego/types/v1alpha1/app_identity.go:99–104`: Host/Hosts/subdomain fields are operational. `lego/backend/internal/rollout/rollout.go:126–145` uses SpecRollsRelease to open history; naive recomputation of Host in the pod template would bypass that guarantee.
- `lego/operator/internal/controller/release_config_snapshot.go:440–495,648–667`: applied templates are recorded and cancel restores them. `release_config_selection.go:272–317` restores historical health path but preserves current domain intent. Freeze the derived Host without broadening that historical-setting contract.
- `dashboard/src/features/services/components/health-check-path-row.tsx:28–44` and `hooks/use-health-check-path.ts:15–27` correctly submit the path; `api/health-check-path.graphql` returns the saved value. `lego/backend/internal/apps/service.go:4372–4384` authorizes operate, normalizes and patches through tracked settings. API acceptance is not proof the asynchronous release became Ready.

The installed `k8s.io/api v0.35.0` pin is `lego/operator/go.mod:19`. Its actual `core/v1/types.go:2636–2655` distinguishes connection Host from HTTPHeaders and can express this fix. Opened matching-version Kubernetes primary source: [request.go](https://github.com/kubernetes/kubernetes/blob/v1.35.0/pkg/probe/http/request.go#L34-L79) selects podIP when connection Host is empty, builds HTTPHeaders, and assigns the Host header to req.Host; [http.go](https://github.com/kubernetes/kubernetes/blob/v1.35.0/pkg/probe/http/http.go#L85-L114) accepts 200–399 and rejects 421. The production kubelet version was not independently inspected; incoming pod-IP Host logs directly confirm this mechanism on the live fixture.

## Shared blast radius and adjacent classes

Projection fix is allowlisted to HTTP probes of web services (including legacy empty type) with an admitted public hostname. One helper fans out to three probes. Private services retain their existing internal-host HTTP behavior; TCP mode remains header-free; workers have no probes; cron has Job/CronJob flow; static sites use the shared static-server; Postgres and Key Value have separate controllers. These siblings were traced, not live re-certified here. They need their own regression coverage in t003.

Create/update aliases: REST POST serviceDetails.healthCheckPath, REST PATCH top-level/nested healthCheckPath; GraphQL createService, setHealthCheckPath and patchService; MCP create_web_service/update_service; dashboard Create and Settings; Blueprint declaration; directly applied App CRs. All ultimately use the operator probe projection. This run mutated only the dashboard Settings path; API readers of the deploy were independently checked.

Do not add auth bypasses or disclose foreign-domain/resource existence. Existing unauthenticated, forbidden, not-found and invalid-path refusals remain in the API layer. A valid accepted change remains queued/in progress with the prior release serving until the pod passes startup/readiness; 4xx/5xx/timeouts still fail health. Errors reading retained configuration must remain explicit retry/failure, not a guessed healthy Host.

## Durable probes

Authenticated requests used the browser session; no credentials are in these records. Each JSON response below is complete for its exact request.

Accepted UI mutation:

```json
{
  "request": [
    {
      "operationName": "SetHealthCheckPath",
      "variables": {
        "id": "srv-db4e35393q6c73at029g",
        "path": "/cgi-bin/health"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation SetHealthCheckPath($id: String!, $path: String!) {\n  setHealthCheckPath(id: $id, path: $path) {\n    id\n    healthCheckPath\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "setHealthCheckPath": {
          "__typename": "Service",
          "healthCheckPath": "/cgi-bin/health",
          "id": "srv-db4e35393q6c73at029g"
        }
      }
    }
  ]
}
```

Bounded incoming probe-Host records and simultaneous public response:

```json
{
  "utc": "2026-10-09T13:00:26.847Z",
  "graphql": {
    "method": "POST",
    "url": "https://api.bex.co/graphql",
    "request": {
      "query": "query QaA32HostProof($id:String!){logs(resource:$id,type:\"app\",text:\"QA_A32_PROBE_HOST\",startTime:\"2026-10-09T12:50:08Z\",endTime:\"2026-10-09T12:50:30Z\",limit:10){hasMore nextStartTime nextEndTime logs{timestamp message instance type level}}}",
      "variables": {
        "id": "srv-db4e35393q6c73at029g"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "logs": {
          "hasMore": false,
          "logs": [
            {
              "instance": "srv-db4e35393q6c73at029g-l0o26ahp6t66aofoiu1l",
              "level": "unknown",
              "message": "QA_A32_PROBE_HOST=10.244.128.252:3000",
              "timestamp": "2026-10-09T12:50:09.073989533Z",
              "type": "app"
            },
            {
              "instance": "srv-db4e35393q6c73at029g-l0o26ahp6t66aofoiu1l",
              "level": "unknown",
              "message": "QA_A32_PROBE_HOST=10.244.128.252:3000",
              "timestamp": "2026-10-09T12:50:19.073801501Z",
              "type": "app"
            },
            {
              "instance": "srv-db4e35393q6c73at029g-l0o26ahp6t66aofoiu1l",
              "level": "unknown",
              "message": "QA_A32_PROBE_HOST=10.244.128.252:3000",
              "timestamp": "2026-10-09T12:50:29.073629747Z",
              "type": "app"
            }
          ],
          "nextEndTime": "2026-10-09T12:50:09.073989532Z",
          "nextStartTime": "2026-10-09T12:50:08Z"
        }
      }
    }
  },
  "public": {
    "method": "GET",
    "url": "https://qa-20261009-loop-a32-health.onbex.co/cgi-bin/health",
    "status": 200,
    "body": "QA_A32_HOST_OK\n"
  }
}
```

REST and MCP read the same deployment:

```json
{
  "utc": "2026-10-09T12:57:45.723Z",
  "rest": {
    "method": "GET",
    "path": "/v1/services/srv-db4e35393q6c73at029g/deploys/dep-db4e5tj93q6c73at02d0",
    "status": 200,
    "response": {
      "id": "dep-db4e5tj93q6c73at02d0",
      "serviceId": "srv-db4e35393q6c73at029g",
      "status": "update_in_progress",
      "trigger": "service_updated",
      "bexTrigger": "config_change",
      "image": {
        "ref": "docker.io/library/busybox:1.37"
      },
      "createdAt": "2026-10-09T12:49:58.885055Z",
      "updatedAt": "2026-10-09T12:50:13.600368Z",
      "startedAt": "2026-10-09T12:50:13.600367Z",
      "stallReason": "the container is running but its startup health check has not succeeded, so the rollout is waiting: GET /cgi-bin/health on port 3000. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening."
    }
  },
  "mcp": {
    "request": {
      "jsonrpc": "2.0",
      "id": 32,
      "method": "tools/call",
      "params": {
        "name": "get_deploy",
        "arguments": {
          "serviceId": "srv-db4e35393q6c73at029g",
          "deployId": "dep-db4e5tj93q6c73at02d0"
        }
      }
    },
    "status": 200,
    "response": {
      "jsonrpc": "2.0",
      "id": 32,
      "result": {
        "content": [
          {
            "type": "text",
            "text": "{\"bexTrigger\":\"config_change\",\"createdAt\":\"2026-10-09T12:49:58.885055Z\",\"id\":\"dep-db4e5tj93q6c73at02d0\",\"image\":{\"ref\":\"docker.io/library/busybox:1.37\"},\"serviceId\":\"srv-db4e35393q6c73at029g\",\"stallReason\":\"the container is running but its startup health check has not succeeded, so the rollout is waiting: GET /cgi-bin/health on port 3000. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening.\",\"startedAt\":\"2026-10-09T12:50:13.600367Z\",\"status\":\"update_in_progress\",\"trigger\":\"service_updated\",\"updatedAt\":\"2026-10-09T12:50:13.600368Z\"}"
          }
        ],
        "structuredContent": {
          "bexTrigger": "config_change",
          "createdAt": "2026-10-09T12:49:58.885055Z",
          "id": "dep-db4e5tj93q6c73at02d0",
          "image": {
            "ref": "docker.io/library/busybox:1.37"
          },
          "serviceId": "srv-db4e35393q6c73at029g",
          "stallReason": "the container is running but its startup health check has not succeeded, so the rollout is waiting: GET /cgi-bin/health on port 3000. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening.",
          "startedAt": "2026-10-09T12:50:13.600367Z",
          "status": "update_in_progress",
          "trigger": "service_updated",
          "updatedAt": "2026-10-09T12:50:13.600368Z"
        }
      }
    }
  }
}
```

Local ignored, verified evidence: `.playwright-mcp/qa-health-host-a32-ledger.json` and `.playwright-mcp/qa-health-host-a32-stalled.png` (viewed, 1440×1000). The screenshot shows the saved-change hint, startup-health stall and pod-IP Host records together. The complete requests above carry the contract evidence after handoff.

## Dedupe and prior DoD disposition

Read DO_NOT_DO; no anti-goal applies. Scanned every PM markdown for HTTPHeaders/healthCheckHandler and health/Host-header pairs, all **74 non-done READMEs** across workstreams, latest 40 repository/product commits, and targeted history. No scheduled fix or undeployed main fix matches. `git log -S healthCheckHandler` points to `7b9c43ebe` (w7/m80), which still lacks Host. w9/m89/m92 are unrelated auth/agent-session hunt precedents.

- **w7/done/m80 full DoD:** root-404 TCP default, slow-page five-second budget, eight-minute boot/timer relationship, and strict explicit HTTP success. This finding does not contradict those four outcomes; Host selection was omitted. TCP/query/space controls passed here, but a root-404 fixture, 2.5-second response and eight-minute successful boot were not replayed. This is a residual parity gap, not evidence those timer/default fixes regressed.
- **w7/done/m81 full DoD:** liveness restart, startup shielding, decided unconditional contract and unchanged worker shapes. All use the shared handler. Host omission reaches those consumers, while their timeout/threshold contracts remain in source; no live wedged-instance, slow-boot-success or worker comparison was performed here. Preserve all four in regression coverage.
- **w4/blocked/m162:** platform activator/static-server `/healthz` dispatch, not tenant App HTTP probe Host. This CGI path avoids its route exception. No reopening/closure claim.
- **w4/done/131:** latestDeployId null on ordinary Service reads is intentional; deploys was queried explicitly. No finding filed for it.
- Query strings and percent-encoded spaces are accepted by Kubernetes' URL construction and passed on this fixture. Whitespace normalization/no-op feedback is a control, not a second finding.

## Render, limitations and estimate

[Render Health Checks](https://render.com/docs/health-checks), checked 2026-10-09, documents verified custom-domain Host with platform-domain fallback. This is a concrete HTTP-health parity gap. Its private-service TCP-only behavior differs from Bex's existing private HTTP extension; this task does not remove that extension or change the deliberate readiness-removal timing.

Not live tested: verified/pending custom domains, subdomain disablement, private/worker/cron/static/datastore probes, rollback/Restart/cancel restoration after the future fix, steady-state restart, mobile/dark UI, foreign/unauthenticated refusals, or an authenticated Render fixture. Host policy and release-history safety need implementation tests rather than asserted live outcomes. No paid resource or billing action was submitted.

Estimate: 7 tasks, 270 minutes, including shared consumers and standing parity/simplify/coverage/live closeout.

## Settled state, fresh repeat and cleanup

- The first HTTP-health deploy finished **update_failed at 13:05:13.608177Z**, exactly 15 minutes after its recorded start. The final reason says startup health never succeeded in the rollout window. The service returned to Running on rev-4; public CGI remained 200. This is a terminal failure, not an indefinitely stuck deployment.
- Fresh deploy detail at 13:05:47.304Z showed Running / Latest deploy Failed, Duration 15m and the terminal startup-health diagnosis. Viewed screenshot: `.playwright-mcp/qa-health-host-a32-failed.png`.
- Settings → clear path recovered to Live (`dep-db4edeb4am4s73f09600`, finished 13:06:13.630046Z, rev-6), with the exact same command/public CGI still 200.
- Fresh Settings URL → re-enable the same CGI path opened `dep-db4edmr93q6c73at02ig`. A different new instance received **10.244.128.50:3000** as Host three times; the deploy reported startup-health waiting and public CGI remained 200. This shorter repeat was deliberately canceled at 13:08:39.435345Z rather than waiting for a second deadline. Running rev-6 and public 200 were checked after cancel; no second terminal failure is claimed.
- Deleted the owned service through its UI phrase at 13:09:08.812Z. At 13:09:53.902Z API and public CGI were both 404. All six baseline ID sets matched: services 5, databases 4, Key Values 2, projects 3, env groups 1, Blueprints 1. Other workers' QA-named resources were preserved.
- qa-login logout returned `ok logged-out`; old browser cookies gave whoami 401 at 13:10:06.598Z. Browser parked at about:blank, cookies cleared (7 → 0), own jar/state removed. No credential values were read or printed.
- Console before cleanup was clean. Recent GraphQL requests were 200; cleanup 404s and post-logout 401 are intentional. The cancel click first opened a confirmation dialog; a harness response-wait timeout before clicking Proceed was corrected without replaying a committed mutation. Missing guessed source paths and locator mistakes were harness errors, not filed product bugs.

Terminal exact GraphQL request and complete response:

```json
{
  "utc": "2026-10-09T13:06:54.021Z",
  "request": {
    "query": "query QaA32Terminal($serviceId:String!,$deployId:String!){deploy(serviceId:$serviceId,deployId:$deployId){id status startedAt finishedAt stallReason failureReason}}",
    "variables": {
      "serviceId": "srv-db4e35393q6c73at029g",
      "deployId": "dep-db4e5tj93q6c73at02d0"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploy": {
        "failureReason": "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /cgi-bin/health on port 3000. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening.",
        "finishedAt": "2026-10-09T13:05:13.608177Z",
        "id": "dep-db4e5tj93q6c73at02d0",
        "stallReason": "",
        "startedAt": "2026-10-09T12:50:13.600367Z",
        "status": "update_failed"
      }
    }
  }
}
```

Recovery exact request and complete response, with public control:

```json
{
  "utc": "2026-10-09T13:06:18.111Z",
  "request": {
    "query": "query QaA32Recovery($id:String!){service(id:$id){id phase revision healthCheckPath} deploys(serviceId:$id,limit:1){id status createdAt startedAt finishedAt trigger stallReason failureReason}}",
    "variables": {
      "id": "srv-db4e35393q6c73at029g"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "createdAt": "2026-10-09T13:06:01.807288Z",
          "failureReason": "",
          "finishedAt": "2026-10-09T13:06:13.630046Z",
          "id": "dep-db4edeb4am4s73f09600",
          "stallReason": "",
          "startedAt": "",
          "status": "live",
          "trigger": "config_change"
        }
      ],
      "service": {
        "healthCheckPath": "",
        "id": "srv-db4e35393q6c73at029g",
        "phase": "Running",
        "revision": "rev-6"
      }
    }
  },
  "public": {
    "status": 200,
    "body": "QA_A32_HOST_OK\n"
  }
}
```

Fresh-repeat exact request and complete response, with public control:

```json
{
  "utc": "2026-10-09T13:07:27.547Z",
  "url": "https://dashboard.bex.co/services/srv-db4e35393q6c73at029g/deploys/dep-db4edmr93q6c73at02ig",
  "request": {
    "query": "query QaA32RepeatProof($id:String!){service(id:$id){id phase revision healthCheckPath} deploys(serviceId:$id,limit:1){id status createdAt startedAt finishedAt trigger stallReason failureReason} logs(resource:$id,type:\"app\",text:\"QA_A32_PROBE_HOST\",startTime:\"2026-10-09T13:06:36Z\",endTime:\"2026-10-09T13:07:10Z\",limit:20){hasMore nextStartTime nextEndTime logs{timestamp message instance type level}}}",
    "variables": {
      "id": "srv-db4e35393q6c73at029g"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "createdAt": "2026-10-09T13:06:35.468547Z",
          "failureReason": "",
          "finishedAt": "",
          "id": "dep-db4edmr93q6c73at02ig",
          "stallReason": "the container is running but its startup health check has not succeeded, so the rollout is waiting: GET /cgi-bin/health on port 3000. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening.",
          "startedAt": "2026-10-09T13:06:43.547475Z",
          "status": "update_in_progress",
          "trigger": "config_change"
        }
      ],
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db4e35393q6c73at029g-54t4ic3n2bvm32e0f3fo",
            "level": "unknown",
            "message": "QA_A32_PROBE_HOST=10.244.128.50:3000",
            "timestamp": "2026-10-09T13:06:45.646370497Z",
            "type": "app"
          },
          {
            "instance": "srv-db4e35393q6c73at029g-54t4ic3n2bvm32e0f3fo",
            "level": "unknown",
            "message": "QA_A32_PROBE_HOST=10.244.128.50:3000",
            "timestamp": "2026-10-09T13:06:55.646794414Z",
            "type": "app"
          },
          {
            "instance": "srv-db4e35393q6c73at029g-54t4ic3n2bvm32e0f3fo",
            "level": "unknown",
            "message": "QA_A32_PROBE_HOST=10.244.128.50:3000",
            "timestamp": "2026-10-09T13:07:05.646673588Z",
            "type": "app"
          }
        ],
        "nextEndTime": "2026-10-09T13:06:45.646370496Z",
        "nextStartTime": "2026-10-09T13:06:36Z"
      },
      "service": {
        "healthCheckPath": "/cgi-bin/health",
        "id": "srv-db4e35393q6c73at029g",
        "phase": "Deploying",
        "revision": "rev-6"
      }
    }
  },
  "public": {
    "status": 200,
    "body": "QA_A32_HOST_OK\n"
  }
}
```

The ignored ledger contains the accepted clear/re-enable/cancel/delete mutations, complete six-family cleanup capture and logout verification. Global Markdown formatting and scoped PM formatting ran; PM paths, task count, worker, dependencies, status and 270-minute total validated.
