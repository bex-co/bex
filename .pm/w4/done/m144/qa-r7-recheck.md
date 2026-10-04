# m144 partial production recheck — sweep 7, 2026-10-02

The production memory budget is now applied and this QA fixture's CPU, memory and instance queries return data. This is partial acceptance evidence for the existing milestone, not a new metrics bug or full closeout.

## Observed

At approximately 09:20 UTC, monitoring/prometheus-server-5db4bfcd9f-lv2kj was Ready with zero container restarts. Its main container started at 08:35:59Z with memory request 1Gi and limit 2Gi. The Prometheus EndpointSlice reports ready=true and serving=true. The earlier read around 09:13 UTC showed the same pod, resources and zero restarts. This is about seven minutes between this sweep's observations, not the required measured 15-minute restart-validation window; uptime alone is not a replay-peak measurement.

The own disposable web service srv-davnajmde41s73cantfg displayed CPU, memory and instance samples on Metrics. Last 30 minutes generated the complete successful GraphQL responses below at 09:21:07Z. Its free-service sleep/wake also worked: Hibernated with Deployment replicas=0; first JSON request 503 with Retry-After: 5; follow-up 15.6 seconds later 200 / OK. [m147's capture](../../m147/finding.md) retains those exact responses and the separate activity-page bug.

The dashboard network card still showed Partial data for bandwidth and no latency samples in this fixture. This recheck does not equate a healthy Prometheus process with complete exporter, usage or billing coverage.

## Remaining acceptance

- Measure retained-data replay/runtime/query peaks and cardinality; verify a retained-data restart and stable restart count across a measured 15-minute observation. This QA sweep made no Kubernetes mutations.
- Repeat the original existing-service Scaling/hard-reload and REST CPU/MCP get_metrics/CPU_LIMIT controls. This sweep used the own web fixture's Metrics page and GraphQL; it does not replace those original probes.
- Complete the shared-consumer/resource-family matrix in verification.md: datastore disk/connections/lag/Key Value, usage collector, Grafana, rules and other workload families.
- Do not claim historical backfill, billing repair, or recovery of all egress sources.

The fixture was deleted through the dashboard, its REST read was 404, its exact App/workloads were absent, and the QA session was revoked. No source or production configuration changed in this recheck.

## Targeted production read output

Commands: read the named Prometheus pod's JSON, projecting its name, creation timestamp, per-container resources and containerStatuses; read EndpointSlices labelled kubernetes.io/service-name=prometheus-server with ready/serving conditions. Both used kubectl --context hetzner-prod --request-timeout=20s -n monitoring. Complete captured projection/output:

```text
{
  "name": "prometheus-server-5db4bfcd9f-lv2kj",
  "createdAt": "2026-10-02T08:35:57Z",
  "resources": {
    "prometheus-server-configmap-reload": {},
    "prometheus-server": {
      "limits": {
        "cpu": "500m",
        "memory": "2Gi"
      },
      "requests": {
        "cpu": "50m",
        "memory": "1Gi"
      }
    }
  },
  "status": [
    {
      "allocatedResources": {
        "cpu": "50m",
        "memory": "1Gi"
      },
      "containerID": "containerd://b32a7fc203c0eb6fa9b25fe16114339bf29caefa9b331e5da136d680d8b7db84",
      "image": "quay.io/prometheus/prometheus:v2.54.1",
      "imageID": "quay.io/prometheus/prometheus@sha256:f6639335d34a77d9d9db382b92eeb7fc00934be8eae81dbc03b31cfe90411a94",
      "lastState": {},
      "name": "prometheus-server",
      "ready": true,
      "resources": {
        "limits": {
          "cpu": "500m",
          "memory": "2Gi"
        },
        "requests": {
          "cpu": "50m",
          "memory": "1Gi"
        }
      },
      "restartCount": 0,
      "started": true,
      "state": {
        "running": {
          "startedAt": "2026-10-02T08:35:59Z"
        }
      },
      "user": {
        "linux": {
          "gid": 65534,
          "supplementalGroups": [
            65534
          ],
          "uid": 65534
        }
      },
      "volumeMounts": [
        {
          "mountPath": "/etc/config",
          "name": "config-volume"
        },
        {
          "mountPath": "/data",
          "name": "storage-volume"
        },
        {
          "mountPath": "/etc/prometheus/etcd-metrics-ca",
          "name": "etcd-metrics-ca",
          "readOnly": true,
          "recursiveReadOnly": "Disabled"
        },
        {
          "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
          "name": "kube-api-access-qqmjs",
          "readOnly": true,
          "recursiveReadOnly": "Disabled"
        }
      ]
    },
    {
      "containerID": "containerd://2ee7e8ba37b35fcc12572ad8941686f14dce635fc304eb426a147decc6148f12",
      "image": "quay.io/prometheus-operator/prometheus-config-reloader:v0.76.0",
      "imageID": "quay.io/prometheus-operator/prometheus-config-reloader@sha256:3ee47d8f6eae9e3997bd928525946c4eb06d5bb82bf1da69ca743169c331c6a0",
      "lastState": {},
      "name": "prometheus-server-configmap-reload",
      "ready": true,
      "resources": {},
      "restartCount": 0,
      "started": true,
      "state": {
        "running": {
          "startedAt": "2026-10-02T08:35:58Z"
        }
      },
      "user": {
        "linux": {
          "gid": 65534,
          "supplementalGroups": [
            65534
          ],
          "uid": 65534
        }
      },
      "volumeMounts": [
        {
          "mountPath": "/etc/config",
          "name": "config-volume",
          "readOnly": true,
          "recursiveReadOnly": "Disabled"
        },
        {
          "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
          "name": "kube-api-access-qqmjs",
          "readOnly": true,
          "recursiveReadOnly": "Disabled"
        }
      ]
    }
  ]
}
NAME                      READY   SERVING
prometheus-server-pmwtk   true    true
```

## Complete GraphQL metric probes

POST https://api.bex.co/graphql with the existing QA session. These are the selected complete operations/responses from the dashboard's batch (credentials omitted); bounds and filters are unchanged. CPU/memory observations from both pod lifetimes are retained.

```json
{
  "at": "2026-10-02T09:21:07.083Z",
  "status": 200,
  "operations": [
    {
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-davnajmde41s73cantfg"]
              }
            ],
            "name": "CPU",
            "start": "2026-10-02T08:51:06.484Z",
            "end": "2026-10-02T09:21:06.484Z",
            "resolution": 15
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
      "response": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "instance",
                  "value": "srv-davnajmde41s73cantfg-k2scelruvtajq5q1ja4n"
                },
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-davnajmde41s73cantfg"
                }
              ],
              "parameters": [],
              "unit": "cpu",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:51Z",
                  "value": 0.00007303227992883178
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:51Z",
                  "value": 0.00008315186708605567
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:21Z",
                  "value": 0.0001481076534903279
                }
              ]
            },
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "instance",
                  "value": "srv-davnajmde41s73cantfg-soroj7nabhi8a1o8vrg6"
                },
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-davnajmde41s73cantfg"
                }
              ],
              "parameters": [],
              "unit": "cpu",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:21Z",
                  "value": 0.000053930380781172896
                }
              ]
            }
          ]
        }
      }
    },
    {
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-davnajmde41s73cantfg"]
              }
            ],
            "name": "MEMORY",
            "start": "2026-10-02T08:51:06.484Z",
            "end": "2026-10-02T09:21:06.484Z",
            "resolution": 15
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
      "response": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "instance",
                  "value": "srv-davnajmde41s73cantfg-k2scelruvtajq5q1ja4n"
                },
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-davnajmde41s73cantfg"
                }
              ],
              "parameters": [],
              "unit": "bytes",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:21Z",
                  "value": 1519616
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:36Z",
                  "value": 1519616
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:51Z",
                  "value": 1613824
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:06Z",
                  "value": 1626112
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:21Z",
                  "value": 1626112
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:36Z",
                  "value": 1626112
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:51Z",
                  "value": 1753088
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:06Z",
                  "value": 1753088
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:21Z",
                  "value": 1753088
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:36Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:51Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:06Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:21Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:36Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:51Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:06Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:21Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:36Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:51Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:06Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:21Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:36Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:51Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:06Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:21Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:36Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:51Z",
                  "value": 1802240
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:21:06Z",
                  "value": 1802240
                }
              ]
            },
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "instance",
                  "value": "srv-davnajmde41s73cantfg-soroj7nabhi8a1o8vrg6"
                },
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-davnajmde41s73cantfg"
                }
              ],
              "parameters": [],
              "unit": "bytes",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:21Z",
                  "value": 1445888
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:36Z",
                  "value": 1691648
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:51Z",
                  "value": 1691648
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:06Z",
                  "value": 1720320
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:21Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:36Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:51Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:06Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:21Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:36Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:51Z",
                  "value": 1769472
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:21:06Z",
                  "value": 1769472
                }
              ]
            }
          ]
        }
      }
    },
    {
      "request": {
        "operationName": "Metrics",
        "variables": {
          "query": {
            "filters": [
              {
                "field": "RESOURCE",
                "values": ["srv-davnajmde41s73cantfg"]
              }
            ],
            "name": "INSTANCES",
            "start": "2026-10-02T08:51:06.484Z",
            "end": "2026-10-02T09:21:06.484Z",
            "resolution": 15
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
      "response": {
        "data": {
          "metrics": [
            {
              "__typename": "MetricSeries",
              "labels": [
                {
                  "__typename": "MetricLabel",
                  "field": "resource",
                  "value": "srv-davnajmde41s73cantfg"
                }
              ],
              "parameters": [],
              "unit": "count",
              "values": [
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:21Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:36Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:14:51Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:06Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:21Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:36Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:15:51Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:06Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:21Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:36Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:16:51Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:06Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:21Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:36Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:17:51Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:06Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:21Z",
                  "value": 2
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:36Z",
                  "value": 2
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:18:51Z",
                  "value": 2
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:06Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:21Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:36Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:19:51Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:06Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:21Z",
                  "value": 1
                },
                {
                  "__typename": "MetricValue",
                  "time": "2026-10-02T09:20:36Z",
                  "value": 1
                }
              ]
            }
          ]
        }
      }
    }
  ]
}
```
