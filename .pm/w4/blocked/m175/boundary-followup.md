# a313 — Boundary buckets include requests outside the selected window

Why: the custom Metrics window can count traffic that its matching request-log window correctly excludes.

**Severity:** major. **Disposition:** existing m175 residual; no duplicate milestone. **Source:** continuous functional QA a313, 2026-10-12 Pacific / 2026-10-13 UTC, w4, normal QA login in workspace bex. Diagnosis source `3592fa39f` plus the queued PM commits; targeted metrics history retains the offending bounds code. Production backend/Loki versions were not inspected. Filing only; no Bex code changed.

## Live reproduction and controls

Owned Free BusyBox web `qa-20261012-a313-metrics` / `srv-db6o3515i38c73fd5p60` becomes Live at **00:56:04.681 UTC**. Exactly eight harmless requests are driven between **00:56:26.192 and 00:56:29.264 UTC**: alpha has three GET and one HEAD, beta two GET, and a missing path one GET/one HEAD returning404.

Metrics → Time range → Custom: enter local Start **2026-10-12 15:52**, End **17:56**. The actual wire is UTC **22:52 → 00:56**, 124 minutes, automatically chosen resolution **62s**. Enter Path `/qa-a313-alpha` and press Enter.

At **00:59:48.729 UTC**, the desktop shows **4 requests** and a latency line even though every alpha request occurred after the selected end. The actual filtered HTTP_REQUESTS response has one point at **00:56:50 = 4**; HTTP_LATENCY returns **0.0074900019 seconds**. The identical log window returns **zero entries / hasMore:false** at **01:00:42.539 UTC**. Its containing minute **00:56 → 00:57** returns exactly four records at **00:56:26.698244900, 27.029882432, 27.357671096, 27.711878095**.

A fresh **390×844 reload**, reapplying only the page-local path filter, reproduces at **01:01:50.908 UTC**. The URL-owned custom bounds survive. Actual UI reads also show the unfiltered count wrongly including all eight requests; unfiltered latency remains empty on its separate Prometheus path.

Direct valid boundary probes:

- Alpha **00:56:40 → 00:57:20**, step60: all four requests are before start, yet count is4.
- Alpha **00:55:20 → 00:56:20**, step60: all four requests are after end, yet count is4.
- Containing minute step15: alpha4, beta2, missing path with status4xx2.
- All eight m175 preset span/step pairs, PATH alpha, containing traffic: each counts4, including 24h/2d/7d/14d. The original newest-bucket repair remains valuable.
- Normal UI custom **22:56 → 00:56**, resolution60: zero series and honest No data in range at **01:04:29.995 UTC**.
- Normal UI custom **22:57 → 00:57**, resolution60: exactly4 at **01:04:31.391 UTC**.

The distinction is alignment, not new traffic, input validation, request-path matching or a missing log store.

## Root cause and concrete correction

`lego/backend/internal/metrics/lokisource.go:62–70,87–103` rounds both bounds up, then samples count_over_time over a whole step. `lokiRequestQueryFor:117–149` builds count and filtered-latency aggregations; `lokiRequestPipeline:154–169` applies host/path filters but no entry-time bounds. For the UI case, end00:56:00 rounds to00:56:50 at step62; the whole bucket includes the four later records. The first rounded bucket similarly reaches before an unaligned start.

The previous regression test `metrics_test.go:1256–1325` places its out-of-window request **two whole steps before start**, outside even the widened bucket. It verifies that the newest bucket is included, but cannot detect this within-bucket contamination.

Framework checked at the project pin **Loki 3.6.7**, used by the existing m175 investigation and log-delivery fixture:

- [StepAlignMiddleware](https://github.com/grafana/loki/blob/v3.6.7/pkg/querier/queryrange/queryrangebase/step_align.go) floors both range bounds to the step.
- [Timestamp template functions](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/log/fmt.go) expose the entry's own timestamp; [label-filter grammar](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/syntax/syntax.y) accepts numeric comparisons, parentheses, AND/OR and label dropping.
- [NumericLabelFilter](https://github.com/grafana/loki/blob/v3.6.7/pkg/logql/log/label_filter.go) parses float64. Comparing a full Unix-nanosecond integer would lose low-bit precision. Compare the **integer seconds and nanosecond remainder separately**, each exactly representable at these bounds.

Target: retain the end-containing evaluation point, but include only entries with **start ≤ entry timestamp ≤ end**, once each, before count/quantile aggregation. Use the Loki entry timestamp, which the logs query also filters, rather than an application timestamp or client clock. Add a private timestamp-bound pipeline helper called by the existing pipeline with normalized RequestMetricsRequest.Start/End. Generate integer bounds, format temporary seconds/nanoseconds labels, apply the inclusive tuple condition, and drop both labels before aggregation.

The exact expression is in the local proof below. It uses `unixEpoch (__timestamp__)` and `(__timestamp__).Nanosecond`, followed by the two inclusive tuple comparisons and `drop`. No new persisted labels, API fields, Loki config, query endpoint or dashboard count adjustment is required.

Do **not** discard whole returned points because their evaluation timestamp lies beyond end: that loses valid newest partial-bucket data and undoes t001. This correction clips contributing entries; the existing bucket timestamp representation stays unchanged. The MetricPoint timestamp/value shape (`service.go:91–104`), Render serializer (`render.go:56–62`) and dashboard chart conversion remain compatible.

## Actual local validation

A cached **grafana/loki:3.6.7** image, actual buildinfo3.6.7, ran in an isolated owned local container/network with step alignment enabled, loopback-only ephemeral port and synthetic data. No production Loki was queried directly or seeded.

Eight records: immediately before start, exactly start, start+1ns, middle, end−1ns, exactly end, end+1ns, and later. The expected inclusive count is5.

| Query | Actual result |
| --- | --- |
| existing unbounded whole-bucket count | 8 |
| count with timestamp clipping | 5 |
| unaggregated label check after clipping | 5, no temporary timestamp labels |
| unbounded p90 with synthetic outlier durations | 0.9213 seconds |
| clipped p90 of eligible durations | 0.0046 seconds |

All five queries return200. This proves expression syntax, real execution, endpoint precision and label removal at the pin; it is **research**, not a Bex implementation or deployed fix. Local container/network are removed and the port is closed.

Ignored artifacts exist: `.playwright-mcp/qa-metrics-boundary-a313-local-driver.py`, `qa-a313-loki-config.yaml`, `qa-metrics-boundary-a313-local-loki.json`, `qa-metrics-boundary-a313-local-cleanup.json`. The full local requests/bodies are embedded below. The isolated readiness endpoint initially held its normal startup grace while ingestion/query already worked; that local harness state is not a product finding.

## Shared scope, adjacent states and dedupe

One production NewLokiRequestMetricsSource factory at `cmd/api/main.go:569`; one query builder call at `lokisource.go:56`; two pipeline calls, count and latency (`:133,141`). Service selection has two Loki branches: host/path request metrics (`service.go:1007–1020`) and preferred unfiltered exact count (`:1042–1045`). The unfiltered latency and bandwidth Prometheus branches stay unchanged, as do approximate count fallback and upstream error behavior.

REST `/v1/metrics/http-requests` / `http-latency`, GraphQL HTTP_REQUESTS/HTTP_LATENCY and MCP `get_metrics` (http_request_count/http_requests/http_latency; resourceId/resource, resolution/resolutionSeconds, httpHost/host, httpPath/path aliases) consume the same core. The NetworkMetricsCard has one shared JSX mount in ServiceMetricsPage, hosted by service/static routes. Source families: web/private/worker/cron/static App metrics reach the common source when request streams exist; absence of a public/request stream remains empty. Postgres/Key Value datastore metrics use a separate source. Live coverage here is web; siblings and REST/MCP are not live-probed.

Quiet/empty valid windows remain empty. Inverted/overlong/invalid queries retain their existing validation; a real upstream error stays an error or the existing allowed approximation fallback, not a manufactured zero. Temporary timestamp labels must be dropped before grouping/quantiles; preserve status/method/host/path grouping, units and error labels. Check execution cost on a realistic window before landing; the eight-record prototype establishes no production performance bound.

Open/blocked/done searches for lokiStepBounds, outside-window counts and boundary buckets point to **this open m175** and completed m119/m108. This is precisely m175's existing requirement to keep start exact and count only its selected window. No new milestone is filed. DO_NOT_DO does not exclude this hosting correction.

Original DoD placement:

- Every range / same with filters: all8 PATH presets pass with N=4; original 100GET+25POST/unfiltered matrix not repeated.
- Last point: returned preset points cover the newest traffic; coarse timestamp behavior retained.
- Dashboard: custom desktop/fresh-mobile failure and aligned controls observed; original 24h/7d UI acceptance not replayed.
- REST/GraphQL/MCP agreement and all m119 bullets: not fully replayed. GraphQL and log reconciliation are captured; leave the remaining acceptance open.

No full historical regression or closeout claim. Render production was not tested; the correction restores Bex's stated exactness, with cross-surface/Render parity still owned by t002.

## Cleanup

Own DeleteService succeeds **01:05:08.086 UTC**. At **01:05:38.108 UTC**, REST detail/public URL return404 and all seven baseline ID sets match. Own logout succeeds; old session owners401 at **01:05:54.730 UTC**; cookies0, one blank 1440px page; observer and only stat-matched own session files removed. Local container/network absent and port closed. No baseline resource changed.

Viewed bounded mobile screenshot `.playwright-mcp/qa-metrics-boundary-a313-mobile.png` (34,343 bytes) supports the wrong excluded-window chart. Complete ignored ledger `.playwright-mcp/qa-metrics-boundary-a313-ledger.json`. No paid action, infrastructure change or product implementation.

## Complete live wire and observation proof

```json
{
  "resource": "srv-db6o3515i38c73fd5p60",
  "receipts": {
    "Create": {
      "at": "2026-10-13T00:55:49.151Z",
      "request": {
        "operationName": "CreateService",
        "variables": {
          "image": "docker.io/library/busybox:1.37",
          "name": "qa-20261012-a313-metrics",
          "type": "web_service",
          "registryCredentialId": "",
          "runtime": "image",
          "startCommand": "sh -c 'mkdir -p /tmp/qa-a313-www; printf \"qa-a313-root\\n\" > /tmp/qa-a313-www/index.html; printf \"qa-a313-alpha\\n\" > /tmp/qa-a313-www/qa-a313-alpha; printf \"qa-a313-beta\\n\" > /tmp/qa-a313-www/qa-a313-beta; exec httpd -f -p 3000 -h /tmp/qa-a313-www'",
          "plan": "free",
          "port": 3000,
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
      "status": 200,
      "body": {
        "data": {
          "createService": {
            "__typename": "Service",
            "environmentId": null,
            "id": "srv-db6o3515i38c73fd5p60",
            "latestDeployId": "dep-db6o3515i38c73fd5p6g",
            "name": "qa-20261012-a313-metrics",
            "phase": "",
            "projectId": null,
            "registryCredentialId": "",
            "type": "web_service"
          }
        }
      }
    },
    "InitialState": {
      "at": "2026-10-13T00:56:25.176Z",
      "request": {
        "operationName": "QaA313Runtime",
        "query": "query QaA313Runtime($id:String!){service(id:$id){id name type phase revision url imagePath undeployedChanges undeployedChangesApplying latestDeployId} deploys(serviceId:$id,limit:10){id status image createdAt updatedAt startedAt finishedAt failureReason}}",
        "variables": {
          "id": "srv-db6o3515i38c73fd5p60"
        }
      },
      "status": 200,
      "body": {
        "data": {
          "deploys": [
            {
              "createdAt": "2026-10-13T00:55:48.572367Z",
              "failureReason": "",
              "finishedAt": "2026-10-13T00:56:04.68101Z",
              "id": "dep-db6o3515i38c73fd5p6g",
              "image": "docker.io/library/busybox:1.37",
              "startedAt": "2026-10-13T00:55:51.752576Z",
              "status": "live",
              "updatedAt": "2026-10-13T00:56:04.681012Z"
            }
          ],
          "service": {
            "id": "srv-db6o3515i38c73fd5p60",
            "imagePath": "docker.io/library/busybox:1.37",
            "latestDeployId": null,
            "name": "qa-20261012-a313-metrics",
            "phase": "Running",
            "revision": "rev-1",
            "type": "web_service",
            "undeployedChanges": false,
            "undeployedChangesApplying": false,
            "url": "https://qa-20261012-a313-metrics.onbex.co"
          }
        }
      }
    },
    "Traffic": {
      "startedAt": "2026-10-13T00:56:26.192Z",
      "finishedAt": "2026-10-13T00:56:29.264Z",
      "requests": [
        {
          "at": "2026-10-13T00:56:26.771Z",
          "method": "GET",
          "path": "/qa-a313-alpha",
          "status": 200,
          "body": "qa-a313-alpha\n"
        },
        {
          "at": "2026-10-13T00:56:27.098Z",
          "method": "GET",
          "path": "/qa-a313-alpha",
          "status": 200,
          "body": "qa-a313-alpha\n"
        },
        {
          "at": "2026-10-13T00:56:27.432Z",
          "method": "GET",
          "path": "/qa-a313-alpha",
          "status": 200,
          "body": "qa-a313-alpha\n"
        },
        {
          "at": "2026-10-13T00:56:27.779Z",
          "method": "HEAD",
          "path": "/qa-a313-alpha",
          "status": 200
        },
        {
          "at": "2026-10-13T00:56:28.126Z",
          "method": "GET",
          "path": "/qa-a313-beta",
          "status": 200,
          "body": "qa-a313-beta\n"
        },
        {
          "at": "2026-10-13T00:56:28.460Z",
          "method": "GET",
          "path": "/qa-a313-beta",
          "status": 200,
          "body": "qa-a313-beta\n"
        },
        {
          "at": "2026-10-13T00:56:28.784Z",
          "method": "GET",
          "path": "/qa-a313-missing",
          "status": 404
        },
        {
          "at": "2026-10-13T00:56:29.111Z",
          "method": "HEAD",
          "path": "/qa-a313-missing",
          "status": 404
        }
      ]
    },
    "MetricsMatrix": {
      "at": "2026-10-13T00:57:57.605Z",
      "results": [
        {
          "label": "alpha-contained",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-13T00:56:00Z",
                "end": "2026-10-13T00:57:00Z",
                "resolution": 15
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:56:30Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "beta-contained",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-beta"]
                  }
                ],
                "start": "2026-10-13T00:56:00Z",
                "end": "2026-10-13T00:57:00Z",
                "resolution": 15
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:56:30Z",
                      "value": 2
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "missing-contained-4xx",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-missing"]
                  },
                  {
                    "field": "STATUS_CODE",
                    "values": ["4xx"]
                  }
                ],
                "start": "2026-10-13T00:56:00Z",
                "end": "2026-10-13T00:57:00Z",
                "resolution": 15
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:56:30Z",
                      "value": 2
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "alpha-before-window",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-13T00:56:40Z",
                "end": "2026-10-13T00:57:20Z",
                "resolution": 60
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:57:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "alpha-after-window",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-13T00:55:20Z",
                "end": "2026-10-13T00:56:20Z",
                "resolution": 60
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:57:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        }
      ]
    },
    "LogCorpus": {
      "at": "2026-10-13T01:00:42.539Z",
      "results": [
        {
          "label": "excluded-ui-window",
          "request": {
            "operationName": "QaA313LogCorpus",
            "query": "query QaA313LogCorpus($resource:String!,$startTime:String!,$endTime:String!){logs(resource:$resource,type:\"request\",path:\"/qa-a313-alpha\",startTime:$startTime,endTime:$endTime,limit:100){hasMore nextStartTime nextEndTime logs{timestamp message type method statusCode}}}",
            "variables": {
              "resource": "srv-db6o3515i38c73fd5p60",
              "startTime": "2026-10-12T22:52:00Z",
              "endTime": "2026-10-13T00:56:00Z"
            }
          },
          "status": 200,
          "body": {
            "data": {
              "logs": {
                "hasMore": false,
                "logs": [],
                "nextEndTime": "2026-10-13T00:56:00Z",
                "nextStartTime": "2026-10-12T22:52:00Z"
              }
            }
          }
        },
        {
          "label": "contained-control",
          "request": {
            "operationName": "QaA313LogCorpus",
            "query": "query QaA313LogCorpus($resource:String!,$startTime:String!,$endTime:String!){logs(resource:$resource,type:\"request\",path:\"/qa-a313-alpha\",startTime:$startTime,endTime:$endTime,limit:100){hasMore nextStartTime nextEndTime logs{timestamp message type method statusCode}}}",
            "variables": {
              "resource": "srv-db6o3515i38c73fd5p60",
              "startTime": "2026-10-13T00:56:00Z",
              "endTime": "2026-10-13T00:57:00Z"
            }
          },
          "status": 200,
          "body": {
            "data": {
              "logs": {
                "hasMore": false,
                "logs": [
                  {
                    "message": "{\"ClientAddr\":\"162.224.81.143:61368\",\"ClientHost\":\"162.224.81.143\",\"ClientPort\":\"61368\",\"ClientUsername\":\"-\",\"DownstreamContentSize\":14,\"DownstreamStatus\":200,\"Duration\":8240430,\"KubernetesIngressName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesIngressNamespace\":\"tea-d98210cbbpdc73dcrkvg\",\"KubernetesServiceName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesServicePort\":\"3000\",\"OriginContentSize\":14,\"OriginDuration\":7474008,\"OriginStatus\":200,\"Overhead\":766422,\"RequestAddr\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestContentSize\":0,\"RequestHost\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestMethod\":\"GET\",\"RequestPath\":\"/qa-a313-alpha\",\"RequestPort\":\"-\",\"RequestProtocol\":\"HTTP/1.1\",\"RequestScheme\":\"https\",\"RetryAttempts\":0,\"ServiceName\":\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics-3000@kubernetes\",\"StartLocal\":\"2026-10-13T00:56:26.689558513Z\",\"StartUTC\":\"2026-10-13T00:56:26.689558513Z\",\"TLSCipher\":\"TLS_AES_128_GCM_SHA256\",\"TLSVersion\":\"1.3\",\"entryPointName\":\"websecure\",\"level\":\"info\",\"msg\":\"\",\"time\":\"2026-10-13T00:56:26Z\"}",
                    "method": "GET",
                    "statusCode": "200",
                    "timestamp": "2026-10-13T00:56:26.6982449Z",
                    "type": "request"
                  },
                  {
                    "message": "{\"ClientAddr\":\"162.224.81.143:61368\",\"ClientHost\":\"162.224.81.143\",\"ClientPort\":\"61368\",\"ClientUsername\":\"-\",\"DownstreamContentSize\":14,\"DownstreamStatus\":200,\"Duration\":4574004,\"KubernetesIngressName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesIngressNamespace\":\"tea-d98210cbbpdc73dcrkvg\",\"KubernetesServiceName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesServicePort\":\"3000\",\"OriginContentSize\":14,\"OriginDuration\":4329459,\"OriginStatus\":200,\"Overhead\":244545,\"RequestAddr\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestContentSize\":0,\"RequestHost\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestMethod\":\"GET\",\"RequestPath\":\"/qa-a313-alpha\",\"RequestPort\":\"-\",\"RequestProtocol\":\"HTTP/1.1\",\"RequestScheme\":\"https\",\"RetryAttempts\":0,\"ServiceName\":\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics-3000@kubernetes\",\"StartLocal\":\"2026-10-13T00:56:27.024912471Z\",\"StartUTC\":\"2026-10-13T00:56:27.024912471Z\",\"TLSCipher\":\"TLS_AES_128_GCM_SHA256\",\"TLSVersion\":\"1.3\",\"entryPointName\":\"websecure\",\"level\":\"info\",\"msg\":\"\",\"time\":\"2026-10-13T00:56:27Z\"}",
                    "method": "GET",
                    "statusCode": "200",
                    "timestamp": "2026-10-13T00:56:27.029882432Z",
                    "type": "request"
                  },
                  {
                    "message": "{\"ClientAddr\":\"162.224.81.143:61368\",\"ClientHost\":\"162.224.81.143\",\"ClientPort\":\"61368\",\"ClientUsername\":\"-\",\"DownstreamContentSize\":14,\"DownstreamStatus\":200,\"Duration\":4393429,\"KubernetesIngressName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesIngressNamespace\":\"tea-d98210cbbpdc73dcrkvg\",\"KubernetesServiceName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesServicePort\":\"3000\",\"OriginContentSize\":14,\"OriginDuration\":4132264,\"OriginStatus\":200,\"Overhead\":261165,\"RequestAddr\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestContentSize\":0,\"RequestHost\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestMethod\":\"GET\",\"RequestPath\":\"/qa-a313-alpha\",\"RequestPort\":\"-\",\"RequestProtocol\":\"HTTP/1.1\",\"RequestScheme\":\"https\",\"RetryAttempts\":0,\"ServiceName\":\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics-3000@kubernetes\",\"StartLocal\":\"2026-10-13T00:56:27.35295793Z\",\"StartUTC\":\"2026-10-13T00:56:27.35295793Z\",\"TLSCipher\":\"TLS_AES_128_GCM_SHA256\",\"TLSVersion\":\"1.3\",\"entryPointName\":\"websecure\",\"level\":\"info\",\"msg\":\"\",\"time\":\"2026-10-13T00:56:27Z\"}",
                    "method": "GET",
                    "statusCode": "200",
                    "timestamp": "2026-10-13T00:56:27.357671096Z",
                    "type": "request"
                  },
                  {
                    "message": "{\"ClientAddr\":\"162.224.81.143:61368\",\"ClientHost\":\"162.224.81.143\",\"ClientPort\":\"61368\",\"ClientUsername\":\"-\",\"DownstreamContentSize\":0,\"DownstreamStatus\":200,\"Duration\":5739003,\"KubernetesIngressName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesIngressNamespace\":\"tea-d98210cbbpdc73dcrkvg\",\"KubernetesServiceName\":\"tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics\",\"KubernetesServicePort\":\"3000\",\"OriginContentSize\":0,\"OriginDuration\":5226612,\"OriginStatus\":200,\"Overhead\":512391,\"RequestAddr\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestContentSize\":0,\"RequestHost\":\"qa-20261012-a313-metrics.onbex.co\",\"RequestMethod\":\"HEAD\",\"RequestPath\":\"/qa-a313-alpha\",\"RequestPort\":\"-\",\"RequestProtocol\":\"HTTP/1.1\",\"RequestScheme\":\"https\",\"RetryAttempts\":0,\"ServiceName\":\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261012-a313-metrics-3000@kubernetes\",\"StartLocal\":\"2026-10-13T00:56:27.705474376Z\",\"StartUTC\":\"2026-10-13T00:56:27.705474376Z\",\"TLSCipher\":\"TLS_AES_128_GCM_SHA256\",\"TLSVersion\":\"1.3\",\"entryPointName\":\"websecure\",\"level\":\"info\",\"msg\":\"\",\"time\":\"2026-10-13T00:56:27Z\"}",
                    "method": "HEAD",
                    "statusCode": "200",
                    "timestamp": "2026-10-13T00:56:27.711878095Z",
                    "type": "request"
                  }
                ],
                "nextEndTime": "2026-10-13T00:56:26.698244899Z",
                "nextStartTime": "2026-10-13T00:56:00Z"
              }
            }
          }
        }
      ]
    },
    "PresetControls": {
      "at": "2026-10-13T01:02:34.374Z",
      "results": [
        {
          "label": "30m",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-13T00:32:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 15
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:56:30Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "1h",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-13T00:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 30
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:56:30Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "4h",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-12T21:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 120
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T00:58:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "12h",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-12T13:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 300
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T01:00:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "24h",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-12T01:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 720
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T01:00:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "2d",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-11T01:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 1440
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T01:12:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "7d",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-10-06T01:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 5040
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T01:48:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        },
        {
          "label": "14d",
          "request": {
            "operationName": "QaA313Metrics",
            "query": "query QaA313Metrics($query:MetricsQueryInput!){metrics(query:$query){unit labels{field value}values{time value}parameters{quantile}}}",
            "variables": {
              "query": {
                "name": "HTTP_REQUESTS",
                "filters": [
                  {
                    "field": "RESOURCE",
                    "values": ["srv-db6o3515i38c73fd5p60"]
                  },
                  {
                    "field": "PATH",
                    "values": ["/qa-a313-alpha"]
                  }
                ],
                "start": "2026-09-29T01:02:00.000Z",
                "end": "2026-10-13T01:02:00.000Z",
                "resolution": 10080
              }
            }
          },
          "status": 200,
          "body": {
            "data": {
              "metrics": [
                {
                  "labels": [
                    {
                      "field": "resource",
                      "value": "srv-db6o3515i38c73fd5p60"
                    }
                  ],
                  "parameters": [],
                  "unit": "count",
                  "values": [
                    {
                      "time": "2026-10-13T03:12:00Z",
                      "value": 4
                    }
                  ]
                }
              ]
            }
          }
        }
      ]
    },
    "UiOutsideWindowApplied": {
      "at": "2026-10-13T00:59:33.041Z",
      "url": "https://dashboard.bex.co/services/srv-db6o3515i38c73fd5p60/metrics?range=custom&rangeStart=2026-10-12T22%3A52%3A00.000Z&rangeEnd=2026-10-13T00%3A56%3A00.000Z",
      "path": "/qa-a313-alpha"
    },
    "UiOutsideWindowSettled": {
      "at": "2026-10-13T00:59:48.729Z",
      "url": "https://dashboard.bex.co/services/srv-db6o3515i38c73fd5p60/metrics?range=custom&rangeStart=2026-10-12T22%3A52%3A00.000Z&rangeEnd=2026-10-13T00%3A56%3A00.000Z",
      "main": "- main:\n  - navigation \"Breadcrumbs\":\n    - link \"Projects\":\n      - /url: /\n    - button \"qa-20261012-a313-metrics\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261012-a313-metrics\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Live\"':\n    - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n    - text: Latest deploy Live\n  - link \"Free\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db6o3515i38c73fd5p60\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261012-a313-metrics.onbex.co\":\n    - /url: https://qa-20261012-a313-metrics.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261012-a313-metrics\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 3m\n  - combobox \"Filter events\": All events\n  - combobox \"Time range\": Custom…\n  - button \"Edit custom range\"\n  - button \"Show event timeline\"\n  - text: Application Metrics Instances\n  - listbox \"Instances\":\n    - option \"srv-db6o3515i38c73fd5p60-f000i\"\n  - tablist \"Aggregation\":\n    - tab \"Raw\" [selected]\n    - tab \"Min\"\n    - tab \"Max\"\n    - tab \"Avg\"\n  - tablist:\n    - tab \"Percentage\" [selected]\n    - tab \"Total\"\n  - text: Memory\n  - link \"Limit\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: 512 MiB Percentages unavailable — no trustworthy limit for this window CPU\n  - link \"Limit\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: 0.100 cores No data in range Total Instances\n  - link \"Manage scaling\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/scaling\n  - text: \"1\"\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 1 data points\": 0 0.5 1\n  - text: Network Metrics Aggregated across all instances\n  - combobox \"Status Code\": Status Code All\n  - combobox \"Host\": Host All\n  - textbox \"Path\":\n    - /placeholder: Path…\n    - text: /qa-a313-alpha\n  - button \"Clear path filter\"\n  - text: Total Requests 4 requests\n  - combobox \"Group by\": Group by All requests\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Bar chart with 1 data points\": 0 2 4\n  - text: Response Times\n  - combobox \"Percentile\": Percentile p90\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 1 data points\": 0ms 4ms 7ms\n  - text: Outbound Bandwidth Partial data\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 121 data points\": 0 B 0 B 0 B\n  - paragraph: 0 B used this month *"
    },
    "FreshMobileSettled": {
      "at": "2026-10-13T01:01:50.908Z",
      "main": "- main:\n  - button \"Toggle Sidebar\"\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261012-a313-metrics\"\n  - button \"Search\"\n  - button \"New\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261012-a313-metrics\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Live\"':\n    - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n    - text: Latest deploy Live\n  - link \"Free\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db6o3515i38c73fd5p60\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261012-a313-metrics.onbex.co\":\n    - /url: https://qa-20261012-a313-metrics.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261012-a313-metrics\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 5m\n  - combobox \"Filter events\": All events\n  - combobox \"Time range\": Custom…\n  - button \"Edit custom range\"\n  - button \"Show event timeline\"\n  - text: Application Metrics Instances\n  - listbox \"Instances\":\n    - option \"srv-db6o3515i38c73fd5p60-f000i\"\n  - tablist \"Aggregation\":\n    - tab \"Raw\" [selected]\n    - tab \"Min\"\n    - tab \"Max\"\n    - tab \"Avg\"\n  - tablist:\n    - tab \"Percentage\" [selected]\n    - tab \"Total\"\n  - text: Memory\n  - link \"Limit\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: 512 MiB Percentages unavailable — no trustworthy limit for this window CPU\n  - link \"Limit\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: 0.100 cores No data in range Total Instances\n  - link \"Manage scaling\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/scaling\n  - text: \"1\"\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 1 data points\": 0 0.5 1\n  - text: Network Metrics Aggregated across all instances\n  - combobox \"Status Code\": Status Code All\n  - combobox \"Host\": Host All\n  - textbox \"Path\":\n    - /placeholder: Path…\n    - text: /qa-a313-alpha\n  - button \"Clear path filter\"\n  - text: Total Requests 4 requests\n  - combobox \"Group by\": Group by All requests\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Bar chart with 1 data points\": 0 2 4\n  - text: Response Times\n  - combobox \"Percentile\": Percentile p90\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 1 data points\": 0ms 4ms 7ms\n  - text: Outbound Bandwidth Partial data\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 121 data points\": 0 B 0 B 0 B\n  - paragraph: 0 B used this month *",
      "screenshotCaptured": true
    },
    "AlignedExcluded": {
      "at": "2026-10-13T01:04:28.977Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:56:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 60
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": []
        }
      },
      "url": "https://dashboard.bex.co/services/srv-db6o3515i38c73fd5p60/metrics?range=custom&rangeStart=2026-10-12T22%3A56%3A00.000Z&rangeEnd=2026-10-13T00%3A56%3A00.000Z"
    },
    "AlignedExcludedUI": {
      "at": "2026-10-13T01:04:29.995Z",
      "main": "- main:\n  - button \"Toggle Sidebar\"\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261012-a313-metrics\"\n  - button \"Search\"\n  - button \"New\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261012-a313-metrics\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Live\"':\n    - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n    - text: Latest deploy Live\n  - link \"Free\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db6o3515i38c73fd5p60\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261012-a313-metrics.onbex.co\":\n    - /url: https://qa-20261012-a313-metrics.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261012-a313-metrics\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 8m\n  - combobox \"Filter events\": All events\n  - combobox \"Time range\": Custom…\n  - button \"Edit custom range\"\n  - button \"Show event timeline\"\n  - text: Application Metrics Instances\n  - listbox \"Instances\":\n    - option \"srv-db6o3515i38c73fd5p60-f000i\"\n  - tablist \"Aggregation\":\n    - tab \"Raw\" [selected]\n    - tab \"Min\"\n    - tab \"Max\"\n    - tab \"Avg\"\n  - tablist:\n    - tab \"Percentage\" [selected]\n    - tab \"Total\"\n  - text: Memory\n  - link \"Limit\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: 512 MiB Percentages unavailable — no trustworthy limit for this window CPU\n  - link \"Limit\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/plan\n  - text: 0.100 cores No data in range Total Instances\n  - link \"Manage scaling\":\n    - /url: /services/srv-db6o3515i38c73fd5p60/scaling\n  - text: \"1\"\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 1 data points\": 0 0.5 1\n  - text: Network Metrics Aggregated across all instances\n  - combobox \"Status Code\": Status Code All\n  - combobox \"Host\": Host All\n  - textbox \"Path\":\n    - /placeholder: Path…\n    - text: /qa-a313-alpha\n  - button \"Clear path filter\"\n  - text: Total Requests\n  - combobox \"Group by\": Group by All requests\n  - text: No data in range Response Times\n  - combobox \"Percentile\": Percentile p90\n  - text: No data in range Outbound Bandwidth Partial data\n  - list \"Service events\":\n    - listitem:\n      - link \"Deploy started\":\n        - /url: /services/srv-db6o3515i38c73fd5p60/deploys/dep-db6o3515i38c73fd5p6g\n  - img \"Line chart with 121 data points\": 0 B 0 B 0 B\n  - paragraph: 0 B used this month *"
    },
    "AlignedIncluded": {
      "at": "2026-10-13T01:04:31.391Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:57:00.000Z",
            "end": "2026-10-13T00:57:00.000Z",
            "resolution": 60
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [],
              "unit": "count",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:57:00Z",
                  "value": 4
                }
              ]
            }
          ]
        }
      },
      "url": "https://dashboard.bex.co/services/srv-db6o3515i38c73fd5p60/metrics?range=custom&rangeStart=2026-10-12T22%3A57%3A00.000Z&rangeEnd=2026-10-13T00%3A57%3A00.000Z"
    },
    "Delete": {
      "at": "2026-10-13T01:05:08.086Z",
      "request": {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db6o3515i38c73fd5p60"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "deleteService": true
        }
      }
    },
    "Cleanup": {
      "at": "2026-10-13T01:05:38.108Z",
      "request": {
        "operationName": "QaA313Inventory",
        "query": "query QaA313Inventory($ownerId:String!){services(ownerId:$ownerId){id name type} databases(ownerId:$ownerId){id name} keyValues(ownerId:$ownerId){id name} projects(ownerId:$ownerId){id name} envGroups(ownerId:$ownerId){id name} blueprints(ownerId:$ownerId){id name} webhookEndpoints(ownerId:$ownerId){id name url eventTypes enabled}}",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg"
        }
      },
      "status": 200,
      "body": {
        "data": {
          "blueprints": [
            {
              "id": "blp-d9nqg95cavls73fp8m10",
              "name": "discourse_docker"
            }
          ],
          "databases": [
            {
              "id": "dpg-d9nqg95cavls73fp8m20",
              "name": "beancount-forum-db"
            },
            {
              "id": "dpg-d9rrkoc4h4mc73edurp0",
              "name": "tianpan-forum-db"
            },
            {
              "id": "dpg-d9rs3ee0ccis738kc7c0",
              "name": "blockeden-forum-db"
            },
            {
              "id": "dpg-db34l9bor55s73cqpq9g",
              "name": "qa-20261007-r16-owner-f208ab"
            }
          ],
          "envGroups": [
            {
              "id": "evg-db0s94c48ccs739jikr0",
              "name": "qa-20261003-keys-r85"
            }
          ],
          "keyValues": [
            {
              "id": "red-d9p49kdrtmes73c34ovg",
              "name": "beancount-forum-redis"
            },
            {
              "id": "red-da4086iii7bs73drbqh0",
              "name": "blockeden-forum-redis"
            }
          ],
          "projects": [
            {
              "id": "prj-d9dgeo0bd9nc73a0vh1g",
              "name": "bex.co"
            },
            {
              "id": "prj-d9e5qct5qe4s73b1mjn0",
              "name": "beancount.io"
            },
            {
              "id": "prj-d9sgfnbjghus73cg6hg0",
              "name": "forums"
            }
          ],
          "services": [
            {
              "id": "srv-d9bkcspg9s7c73d0n8ug",
              "name": "agentmarketcap-1",
              "type": "web_service"
            },
            {
              "id": "srv-d9bj8s3eg85c7390eb9g",
              "name": "beancount-cms-v2",
              "type": "web_service"
            },
            {
              "id": "srv-d9nqg9dcavls73fp8m2g",
              "name": "beancount-forum",
              "type": "web_service"
            },
            {
              "id": "srv-d9ndt8hmcglc739fkp50",
              "name": "eden-dash-v3",
              "type": "web_service"
            },
            {
              "id": "srv-d9e40ei9086p3l1jri30",
              "name": "eden-cms-v2",
              "type": "web_service"
            }
          ],
          "webhookEndpoints": []
        }
      },
      "absence": [
        {
          "url": "https://api.bex.co/v1/services/srv-db6o3515i38c73fd5p60",
          "status": 404
        },
        {
          "url": "https://qa-20261012-a313-metrics.onbex.co/",
          "status": 404
        }
      ]
    },
    "SessionCleanup": {
      "at": "2026-10-13T01:05:54.730Z",
      "ownersStatus": 401,
      "cookies": 0,
      "pages": 1,
      "url": "about:blank"
    }
  },
  "uiTransports": [
    {
      "at": "2026-10-13T00:59:33.781Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:52:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 62
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [],
              "unit": "count",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:56:50Z",
                  "value": 4
                }
              ]
            }
          ]
        }
      }
    },
    {
      "at": "2026-10-13T00:59:33.781Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_LATENCY",
            "start": "2026-10-12T22:52:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 62,
            "parameters": [
              {
                "quantile": 0.9
              }
            ]
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [
                {
                  "__typename": "MetricSeriesParameter",
                  "quantile": 0.9
                }
              ],
              "unit": "seconds",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:56:50Z",
                  "value": 0.0074900019
                }
              ]
            }
          ]
        }
      }
    },
    {
      "at": "2026-10-13T01:01:02.657Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:52:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 62
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [],
              "unit": "count",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:56:50Z",
                  "value": 4
                }
              ]
            }
          ]
        }
      }
    },
    {
      "at": "2026-10-13T01:01:02.657Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_LATENCY",
            "start": "2026-10-12T22:52:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 62,
            "parameters": [
              {
                "quantile": 0.9
              }
            ]
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [
                {
                  "__typename": "MetricSeriesParameter",
                  "quantile": 0.9
                }
              ],
              "unit": "seconds",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:56:50Z",
                  "value": 0.0074900019
                }
              ]
            }
          ]
        }
      }
    },
    {
      "at": "2026-10-13T01:01:03.783Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:52:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 62
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [],
              "unit": "count",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:56:50Z",
                  "value": 8
                }
              ]
            }
          ]
        }
      }
    },
    {
      "at": "2026-10-13T01:01:03.783Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              }
            ],
            "name": "HTTP_LATENCY",
            "start": "2026-10-12T22:52:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 62,
            "parameters": [
              {
                "quantile": 0.9
              }
            ]
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": []
        }
      }
    },
    {
      "at": "2026-10-13T01:04:28.976Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:56:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 60
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": []
        }
      }
    },
    {
      "at": "2026-10-13T01:04:28.976Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_LATENCY",
            "start": "2026-10-12T22:56:00.000Z",
            "end": "2026-10-13T00:56:00.000Z",
            "resolution": 60,
            "parameters": [
              {
                "quantile": 0.9
              }
            ]
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": []
        }
      }
    },
    {
      "at": "2026-10-13T01:04:31.390Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_REQUESTS",
            "start": "2026-10-12T22:57:00.000Z",
            "end": "2026-10-13T00:57:00.000Z",
            "resolution": 60
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [],
              "unit": "count",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:57:00Z",
                  "value": 4
                }
              ]
            }
          ]
        }
      }
    },
    {
      "at": "2026-10-13T01:04:31.390Z",
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-db6o3515i38c73fd5p60"]
              },
              {
                "field": "PATH",
                "values": ["/qa-a313-alpha"]
              }
            ],
            "name": "HTTP_LATENCY",
            "start": "2026-10-12T22:57:00.000Z",
            "end": "2026-10-13T00:57:00.000Z",
            "resolution": 60,
            "parameters": [
              {
                "quantile": 0.9
              }
            ]
          }
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query Metrics($query: MetricsQueryInput!) {\n  metrics(query: $query) {\n    unit\n    labels {\n      field\n      value\n      __typename\n    }\n    values {\n      time\n      value\n      __typename\n    }\n    parameters {\n      quantile\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-db6o3515i38c73fd5p60"
                }
              ],
              "parameters": [
                {
                  "__typename": "MetricSeriesParameter",
                  "quantile": 0.9
                }
              ],
              "unit": "seconds",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-13T00:57:00Z",
                  "value": 0.0074900019
                }
              ]
            }
          ]
        }
      }
    }
  ],
  "inventoryMatches": {
    "blueprints": true,
    "databases": true,
    "envGroups": true,
    "keyValues": true,
    "projects": true,
    "services": true,
    "webhookEndpoints": true
  }
}
```

## Complete synthetic local Loki proof

```json
{
  "version": {
    "status": 200,
    "body": {
      "version": "3.6.7",
      "revision": "7e1daf3a",
      "branch": "release-3.6.x",
      "buildUser": "root@buildkitsandbox",
      "buildDate": "2026-02-23T09:20:38Z",
      "goVersion": ""
    }
  },
  "startNs": "1791852980123456789",
  "endNs": "1791853000987654321",
  "push": {
    "request": {
      "streams": [
        {
          "stream": {
            "namespace": "qa-a313-local",
            "app": "qa-a313-local",
            "type": "request",
            "status": "200",
            "method": "GET"
          },
          "values": [
            [
              "1791852980123456788",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 999000000, \"marker\": \"before\"}"
            ],
            [
              "1791852980123456789",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 1000000, \"marker\": \"at-start\"}"
            ],
            [
              "1791852980123456790",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 2000000, \"marker\": \"start-plus-1ns\"}"
            ],
            [
              "1791852990000000000",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 3000000, \"marker\": \"middle\"}"
            ],
            [
              "1791853000987654320",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 4000000, \"marker\": \"end-minus-1ns\"}"
            ],
            [
              "1791853000987654321",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 5000000, \"marker\": \"at-end\"}"
            ],
            [
              "1791853000987654322",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 888000000, \"marker\": \"end-plus-1ns\"}"
            ],
            [
              "1791853010000000000",
              "{\"RequestPath\": \"/qa-a313-alpha\", \"RequestHost\": \"qa-a313.invalid\", \"Duration\": 777000000, \"marker\": \"after\"}"
            ]
          ]
        }
      ]
    },
    "response": {
      "status": 204,
      "body": null
    }
  },
  "records": [
    {
      "timestampNs": "1791852980123456788",
      "marker": "before",
      "expectedIncluded": false
    },
    {
      "timestampNs": "1791852980123456789",
      "marker": "at-start",
      "expectedIncluded": true
    },
    {
      "timestampNs": "1791852980123456790",
      "marker": "start-plus-1ns",
      "expectedIncluded": true
    },
    {
      "timestampNs": "1791852990000000000",
      "marker": "middle",
      "expectedIncluded": true
    },
    {
      "timestampNs": "1791853000987654320",
      "marker": "end-minus-1ns",
      "expectedIncluded": true
    },
    {
      "timestampNs": "1791853000987654321",
      "marker": "at-end",
      "expectedIncluded": true
    },
    {
      "timestampNs": "1791853000987654322",
      "marker": "end-plus-1ns",
      "expectedIncluded": false
    },
    {
      "timestampNs": "1791853010000000000",
      "marker": "after",
      "expectedIncluded": false
    }
  ],
  "queries": [
    {
      "label": "original-count",
      "request": {
        "method": "GET",
        "path": "/loki/api/v1/query_range",
        "params": {
          "query": "sum(count_over_time({namespace=\"qa-a313-local\",app=\"qa-a313-local\",type=\"request\"} | json request_path=\"RequestPath\" | request_path=\"/qa-a313-alpha\" [60s]))",
          "start": "1791853020000000000",
          "end": "1791853020000000000",
          "step": "60"
        }
      },
      "status": 200,
      "body": {
        "status": "success",
        "data": {
          "resultType": "matrix",
          "result": [
            {
              "metric": {},
              "values": [[1791853020, "8"]]
            }
          ],
          "stats": {
            "summary": {
              "bytesProcessedPerSecond": 104110,
              "linesProcessedPerSecond": 871,
              "totalBytesProcessed": 956,
              "totalLinesProcessed": 8,
              "execTime": 0.009182584,
              "queueTime": 0.000036667,
              "subqueries": 0,
              "totalEntriesReturned": 1,
              "splits": 0,
              "shards": 1,
              "totalPostFilterLines": 8,
              "totalStructuredMetadataBytesProcessed": 64
            },
            "querier": {
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 0,
                  "headChunkLines": 0,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 0,
                  "headChunkStructuredMetadataBytes": 0,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 0,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "ingester": {
              "totalReached": 1,
              "totalChunksMatched": 1,
              "totalBatches": 2,
              "totalLinesSent": 8,
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 956,
                  "headChunkLines": 8,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 8,
                  "headChunkStructuredMetadataBytes": 64,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 136417,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "cache": {
              "chunk": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "index": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "result": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "statsResult": {
                "entriesFound": 0,
                "entriesRequested": 1,
                "entriesStored": 1,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 2,
                "downloadTime": 15083,
                "queryLengthServed": 0
              },
              "volumeResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "seriesResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "labelResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "instantMetricResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              }
            },
            "index": {
              "totalChunks": 0,
              "postFilterChunks": 0,
              "shardsDuration": 0,
              "usedBloomFilters": false
            }
          }
        }
      }
    },
    {
      "label": "clipped-count",
      "request": {
        "method": "GET",
        "path": "/loki/api/v1/query_range",
        "params": {
          "query": "sum(count_over_time({namespace=\"qa-a313-local\",app=\"qa-a313-local\",type=\"request\"} | label_format qa_ts_s=\"{{ unixEpoch (__timestamp__) }}\", qa_ts_ns=\"{{ (__timestamp__).Nanosecond }}\" | (qa_ts_s > 1791852980 or (qa_ts_s == 1791852980 and qa_ts_ns >= 123456789)) and (qa_ts_s < 1791853000 or (qa_ts_s == 1791853000 and qa_ts_ns <= 987654321)) | drop qa_ts_s, qa_ts_ns | json request_path=\"RequestPath\" | request_path=\"/qa-a313-alpha\" [60s]))",
          "start": "1791853020000000000",
          "end": "1791853020000000000",
          "step": "60"
        }
      },
      "status": 200,
      "body": {
        "status": "success",
        "data": {
          "resultType": "matrix",
          "result": [
            {
              "metric": {},
              "values": [[1791853020, "5"]]
            }
          ],
          "stats": {
            "summary": {
              "bytesProcessedPerSecond": 298625,
              "linesProcessedPerSecond": 2498,
              "totalBytesProcessed": 956,
              "totalLinesProcessed": 8,
              "execTime": 0.003201333,
              "queueTime": 0.000056083,
              "subqueries": 0,
              "totalEntriesReturned": 1,
              "splits": 0,
              "shards": 1,
              "totalPostFilterLines": 5,
              "totalStructuredMetadataBytesProcessed": 64
            },
            "querier": {
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 0,
                  "headChunkLines": 0,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 0,
                  "headChunkStructuredMetadataBytes": 0,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 0,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "ingester": {
              "totalReached": 1,
              "totalChunksMatched": 1,
              "totalBatches": 2,
              "totalLinesSent": 5,
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 956,
                  "headChunkLines": 8,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 5,
                  "headChunkStructuredMetadataBytes": 64,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 140583,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "cache": {
              "chunk": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "index": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "result": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "statsResult": {
                "entriesFound": 1,
                "entriesRequested": 1,
                "entriesStored": 0,
                "bytesReceived": 227,
                "bytesSent": 0,
                "requests": 1,
                "downloadTime": 4708,
                "queryLengthServed": 60000000000
              },
              "volumeResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "seriesResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "labelResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "instantMetricResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              }
            },
            "index": {
              "totalChunks": 0,
              "postFilterChunks": 0,
              "shardsDuration": 0,
              "usedBloomFilters": false
            }
          }
        }
      }
    },
    {
      "label": "clipped-label-check",
      "request": {
        "method": "GET",
        "path": "/loki/api/v1/query_range",
        "params": {
          "query": "count_over_time({namespace=\"qa-a313-local\",app=\"qa-a313-local\",type=\"request\"} | label_format qa_ts_s=\"{{ unixEpoch (__timestamp__) }}\", qa_ts_ns=\"{{ (__timestamp__).Nanosecond }}\" | (qa_ts_s > 1791852980 or (qa_ts_s == 1791852980 and qa_ts_ns >= 123456789)) and (qa_ts_s < 1791853000 or (qa_ts_s == 1791853000 and qa_ts_ns <= 987654321)) | drop qa_ts_s, qa_ts_ns | json request_path=\"RequestPath\" | request_path=\"/qa-a313-alpha\" [60s])",
          "start": "1791853020000000000",
          "end": "1791853020000000000",
          "step": "60"
        }
      },
      "status": 200,
      "body": {
        "status": "success",
        "data": {
          "resultType": "matrix",
          "result": [
            {
              "metric": {
                "app": "qa-a313-local",
                "detected_level": "unknown",
                "method": "GET",
                "namespace": "qa-a313-local",
                "request_path": "/qa-a313-alpha",
                "service_name": "qa-a313-local",
                "status": "200",
                "type": "request"
              },
              "values": [[1791853020, "5"]]
            }
          ],
          "stats": {
            "summary": {
              "bytesProcessedPerSecond": 227910,
              "linesProcessedPerSecond": 1907,
              "totalBytesProcessed": 956,
              "totalLinesProcessed": 8,
              "execTime": 0.004194625,
              "queueTime": 0.000041833,
              "subqueries": 0,
              "totalEntriesReturned": 1,
              "splits": 0,
              "shards": 1,
              "totalPostFilterLines": 5,
              "totalStructuredMetadataBytesProcessed": 64
            },
            "querier": {
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 0,
                  "headChunkLines": 0,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 0,
                  "headChunkStructuredMetadataBytes": 0,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 0,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "ingester": {
              "totalReached": 1,
              "totalChunksMatched": 1,
              "totalBatches": 2,
              "totalLinesSent": 5,
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 956,
                  "headChunkLines": 8,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 5,
                  "headChunkStructuredMetadataBytes": 64,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 91208,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "cache": {
              "chunk": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "index": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "result": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "statsResult": {
                "entriesFound": 1,
                "entriesRequested": 1,
                "entriesStored": 0,
                "bytesReceived": 227,
                "bytesSent": 0,
                "requests": 1,
                "downloadTime": 9292,
                "queryLengthServed": 60000000000
              },
              "volumeResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "seriesResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "labelResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "instantMetricResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              }
            },
            "index": {
              "totalChunks": 0,
              "postFilterChunks": 0,
              "shardsDuration": 0,
              "usedBloomFilters": false
            }
          }
        }
      }
    },
    {
      "label": "original-latency",
      "request": {
        "method": "GET",
        "path": "/loki/api/v1/query_range",
        "params": {
          "query": "quantile_over_time(0.9, {namespace=\"qa-a313-local\",app=\"qa-a313-local\",type=\"request\"} | json request_path=\"RequestPath\" | request_path=\"/qa-a313-alpha\" | json latency_ns=\"Duration\" | unwrap latency_ns [60s]) by () / 1e9",
          "start": "1791853020000000000",
          "end": "1791853020000000000",
          "step": "60"
        }
      },
      "status": 200,
      "body": {
        "status": "success",
        "data": {
          "resultType": "matrix",
          "result": [
            {
              "metric": {},
              "values": [[1791853020, "0.9213"]]
            }
          ],
          "stats": {
            "summary": {
              "bytesProcessedPerSecond": 753571,
              "linesProcessedPerSecond": 6306,
              "totalBytesProcessed": 956,
              "totalLinesProcessed": 8,
              "execTime": 0.001268625,
              "queueTime": 0.000032916,
              "subqueries": 0,
              "totalEntriesReturned": 1,
              "splits": 0,
              "shards": 1,
              "totalPostFilterLines": 8,
              "totalStructuredMetadataBytesProcessed": 64
            },
            "querier": {
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 0,
                  "headChunkLines": 0,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 0,
                  "headChunkStructuredMetadataBytes": 0,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 0,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "ingester": {
              "totalReached": 1,
              "totalChunksMatched": 1,
              "totalBatches": 2,
              "totalLinesSent": 8,
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 956,
                  "headChunkLines": 8,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 8,
                  "headChunkStructuredMetadataBytes": 64,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 61667,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "cache": {
              "chunk": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "index": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "result": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "statsResult": {
                "entriesFound": 1,
                "entriesRequested": 1,
                "entriesStored": 0,
                "bytesReceived": 227,
                "bytesSent": 0,
                "requests": 1,
                "downloadTime": 3333,
                "queryLengthServed": 60000000000
              },
              "volumeResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "seriesResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "labelResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "instantMetricResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              }
            },
            "index": {
              "totalChunks": 0,
              "postFilterChunks": 0,
              "shardsDuration": 0,
              "usedBloomFilters": false
            }
          }
        }
      }
    },
    {
      "label": "clipped-latency",
      "request": {
        "method": "GET",
        "path": "/loki/api/v1/query_range",
        "params": {
          "query": "quantile_over_time(0.9, {namespace=\"qa-a313-local\",app=\"qa-a313-local\",type=\"request\"} | label_format qa_ts_s=\"{{ unixEpoch (__timestamp__) }}\", qa_ts_ns=\"{{ (__timestamp__).Nanosecond }}\" | (qa_ts_s > 1791852980 or (qa_ts_s == 1791852980 and qa_ts_ns >= 123456789)) and (qa_ts_s < 1791853000 or (qa_ts_s == 1791853000 and qa_ts_ns <= 987654321)) | drop qa_ts_s, qa_ts_ns | json request_path=\"RequestPath\" | request_path=\"/qa-a313-alpha\" | json latency_ns=\"Duration\" | unwrap latency_ns [60s]) by () / 1e9",
          "start": "1791853020000000000",
          "end": "1791853020000000000",
          "step": "60"
        }
      },
      "status": 200,
      "body": {
        "status": "success",
        "data": {
          "resultType": "matrix",
          "result": [
            {
              "metric": {},
              "values": [[1791853020, "0.0046"]]
            }
          ],
          "stats": {
            "summary": {
              "bytesProcessedPerSecond": 391515,
              "linesProcessedPerSecond": 3276,
              "totalBytesProcessed": 956,
              "totalLinesProcessed": 8,
              "execTime": 0.002441791,
              "queueTime": 0.000055875,
              "subqueries": 0,
              "totalEntriesReturned": 1,
              "splits": 0,
              "shards": 1,
              "totalPostFilterLines": 5,
              "totalStructuredMetadataBytesProcessed": 64
            },
            "querier": {
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 0,
                  "headChunkLines": 0,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 0,
                  "headChunkStructuredMetadataBytes": 0,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 0,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "ingester": {
              "totalReached": 1,
              "totalChunksMatched": 1,
              "totalBatches": 2,
              "totalLinesSent": 5,
              "store": {
                "totalChunksRef": 0,
                "totalChunksDownloaded": 0,
                "chunksDownloadTime": 0,
                "queryReferencedStructuredMetadata": false,
                "queryUsedV2Engine": false,
                "chunk": {
                  "headChunkBytes": 956,
                  "headChunkLines": 8,
                  "decompressedBytes": 0,
                  "decompressedLines": 0,
                  "compressedBytes": 0,
                  "totalDuplicates": 0,
                  "postFilterLines": 5,
                  "headChunkStructuredMetadataBytes": 64,
                  "decompressedStructuredMetadataBytes": 0
                },
                "chunkRefsFetchTime": 105708,
                "congestionControlLatency": 0,
                "pipelineWrapperFilteredLines": 0,
                "dataobj": {
                  "prePredicateDecompressedRows": 0,
                  "prePredicateDecompressedBytes": 0,
                  "prePredicateDecompressedStructuredMetadataBytes": 0,
                  "postPredicateRows": 0,
                  "postPredicateDecompressedBytes": 0,
                  "postPredicateStructuredMetadataBytes": 0,
                  "postFilterRows": 0,
                  "pagesScanned": 0,
                  "pagesDownloaded": 0,
                  "pagesDownloadedBytes": 0,
                  "pageBatches": 0,
                  "totalRowsAvailable": 0,
                  "totalPageDownloadTime": 0
                }
              }
            },
            "cache": {
              "chunk": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "index": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "result": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "statsResult": {
                "entriesFound": 1,
                "entriesRequested": 1,
                "entriesStored": 0,
                "bytesReceived": 227,
                "bytesSent": 0,
                "requests": 1,
                "downloadTime": 3291,
                "queryLengthServed": 60000000000
              },
              "volumeResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "seriesResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "labelResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              },
              "instantMetricResult": {
                "entriesFound": 0,
                "entriesRequested": 0,
                "entriesStored": 0,
                "bytesReceived": 0,
                "bytesSent": 0,
                "requests": 0,
                "downloadTime": 0,
                "queryLengthServed": 0
              }
            },
            "index": {
              "totalChunks": 0,
              "postFilterChunks": 0,
              "shardsDuration": 0,
              "usedBloomFilters": false
            }
          }
        }
      }
    }
  ]
}
```
