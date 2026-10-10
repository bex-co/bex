# Exact health-mode probes and complete responses

These authenticated requests were sent through the QA browser to production. Session cookies are supplied by the browser and are deliberately absent. Every response below is complete for its exact request. GraphQL uses String IDs; REST/MCP use their actual trigger vocabulary. Screenshots and the broader control/cleanup ledger are local, so the API evidence is preserved here for replay.

## a33FinalCorrect

```json
{
  "at": "2026-10-10T06:08:57.890Z",
  "request": {
    "operationName": "QaA33Final",
    "query": "query QaA33Final($serviceId:String!,$deployId:String!){service(id:$serviceId){id name phase revision healthCheckPath undeployedChanges} deploy(serviceId:$serviceId,deployId:$deployId){id status createdAt startedAt finishedAt trigger stallReason failureReason cancelReason} deploys(serviceId:$serviceId){id status createdAt startedAt finishedAt trigger}}",
    "variables": {
      "serviceId": "srv-db4ejbr93q6c73at02pg",
      "deployId": "dep-db4ek0b4am4s73f0969g"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploy": {
        "cancelReason": "Superseded by a newer release",
        "createdAt": "2026-10-09T13:20:01.594376Z",
        "failureReason": "",
        "finishedAt": "2026-10-09T13:38:13.578021Z",
        "id": "dep-db4ek0b4am4s73f0969g",
        "stallReason": "",
        "startedAt": "",
        "status": "canceled",
        "trigger": "config_change"
      },
      "deploys": [
        {
          "createdAt": "2026-10-09T13:20:01.594376Z",
          "finishedAt": "2026-10-09T13:38:13.578021Z",
          "id": "dep-db4ek0b4am4s73f0969g",
          "startedAt": "",
          "status": "canceled",
          "trigger": "config_change"
        },
        {
          "createdAt": "2026-10-09T13:18:39.352618Z",
          "finishedAt": "2026-10-09T13:18:57.489929Z",
          "id": "dep-db4ejbr93q6c73at02q0",
          "startedAt": "2026-10-09T13:18:43.53932Z",
          "status": "live",
          "trigger": "create"
        }
      ],
      "service": {
        "healthCheckPath": "/",
        "id": "srv-db4ejbr93q6c73at02pg",
        "name": "qa-20261009-loop-a33-health",
        "phase": "Running",
        "revision": "rev-1",
        "undeployedChanges": false
      }
    }
  }
}
```

## a33BRootLiveAndClear

```json
{
  "at": "2026-10-10T06:11:29.514Z",
  "request": {
    "operationName": "QaA33BRootControl",
    "query": "query QaA33BRootControl($serviceId:String!){service(id:$serviceId){id name phase revision healthCheckPath undeployedChanges} deploys(serviceId:$serviceId){id status createdAt startedAt finishedAt trigger}}",
    "variables": {
      "serviceId": "srv-db4td5j93q6c73at05jg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "createdAt": "2026-10-10T06:10:54.707578Z",
          "finishedAt": "2026-10-10T06:11:13.601292Z",
          "id": "dep-db4tdrj93q6c73at05mg",
          "startedAt": "2026-10-10T06:10:57.513844Z",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "createdAt": "2026-10-10T06:10:00.946056Z",
          "finishedAt": "2026-10-10T06:10:13.632877Z",
          "id": "dep-db4tde393q6c73at05l0",
          "startedAt": "",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "createdAt": "2026-10-10T06:09:26.383468Z",
          "finishedAt": "2026-10-10T06:09:43.703538Z",
          "id": "dep-db4td5j93q6c73at05k0",
          "startedAt": "2026-10-10T06:09:27.720569Z",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "service": {
        "healthCheckPath": "/",
        "id": "srv-db4td5j93q6c73at05jg",
        "name": "qa-20261010-loop-a33-reverse",
        "phase": "Running",
        "revision": "rev-3",
        "undeployedChanges": false
      }
    }
  },
  "public": {
    "status": 200,
    "body": "QA_A33R_HTTP_OK\n"
  },
  "clear": {
    "at": "2026-10-10T06:11:31.834Z",
    "request": [
      {
        "operationName": "SetHealthCheckPath",
        "variables": {
          "id": "srv-db4td5j93q6c73at05jg",
          "path": ""
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
            "healthCheckPath": "",
            "id": "srv-db4td5j93q6c73at05jg"
          }
        }
      }
    ]
  }
}
```

## a33BReverseObserve

```json
{
  "at": "2026-10-10T06:12:10.081Z",
  "request": {
    "operationName": "QaA33Reverse",
    "query": "query QaA33Reverse($serviceId:String!){service(id:$serviceId){id name phase revision healthCheckPath undeployedChanges} deploys(serviceId:$serviceId){id status createdAt startedAt finishedAt trigger cancelReason failureReason stallReason}}",
    "variables": {
      "serviceId": "srv-db4td5j93q6c73at05jg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:11:31.802609Z",
          "failureReason": "",
          "finishedAt": "",
          "id": "dep-db4te4r4am4s73f098s0",
          "stallReason": "",
          "startedAt": "",
          "status": "created",
          "trigger": "config_change"
        },
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:10:54.707578Z",
          "failureReason": "",
          "finishedAt": "2026-10-10T06:11:13.601292Z",
          "id": "dep-db4tdrj93q6c73at05mg",
          "stallReason": "",
          "startedAt": "2026-10-10T06:10:57.513844Z",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:10:00.946056Z",
          "failureReason": "",
          "finishedAt": "2026-10-10T06:10:13.632877Z",
          "id": "dep-db4tde393q6c73at05l0",
          "stallReason": "",
          "startedAt": "",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:09:26.383468Z",
          "failureReason": "",
          "finishedAt": "2026-10-10T06:09:43.703538Z",
          "id": "dep-db4td5j93q6c73at05k0",
          "stallReason": "",
          "startedAt": "2026-10-10T06:09:27.720569Z",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "service": {
        "healthCheckPath": "",
        "id": "srv-db4td5j93q6c73at05jg",
        "name": "qa-20261010-loop-a33-reverse",
        "phase": "Running",
        "revision": "rev-3",
        "undeployedChanges": false
      }
    }
  },
  "public": {
    "status": 200,
    "body": "QA_A33R_HTTP_OK\n"
  }
}
```

## a33BReverseCrossSurface

```json
{
  "at": "2026-10-10T06:12:28.213Z",
  "public": {
    "status": 200,
    "body": "QA_A33R_HTTP_OK\n"
  },
  "rest": {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db4td5j93q6c73at05jg/deploys/dep-db4te4r4am4s73f098s0"
    },
    "status": 200,
    "response": {
      "id": "dep-db4te4r4am4s73f098s0",
      "serviceId": "srv-db4td5j93q6c73at05jg",
      "status": "created",
      "trigger": "service_updated",
      "bexTrigger": "config_change",
      "image": {
        "ref": "docker.io/library/busybox:1.37"
      },
      "createdAt": "2026-10-10T06:11:31.802609Z",
      "updatedAt": "2026-10-10T06:11:31.802609Z"
    }
  },
  "mcp": {
    "request": {
      "method": "POST",
      "url": "https://api.bex.co/mcp",
      "headers": {
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18"
      },
      "body": {
        "jsonrpc": "2.0",
        "id": 33,
        "method": "tools/call",
        "params": {
          "name": "get_deploy",
          "arguments": {
            "serviceId": "srv-db4td5j93q6c73at05jg",
            "deployId": "dep-db4te4r4am4s73f098s0"
          }
        }
      }
    },
    "status": 200,
    "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":33,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"bexTrigger\\\":\\\"config_change\\\",\\\"createdAt\\\":\\\"2026-10-10T06:11:31.802609Z\\\",\\\"id\\\":\\\"dep-db4te4r4am4s73f098s0\\\",\\\"image\\\":{\\\"ref\\\":\\\"docker.io/library/busybox:1.37\\\"},\\\"serviceId\\\":\\\"srv-db4td5j93q6c73at05jg\\\",\\\"status\\\":\\\"created\\\",\\\"trigger\\\":\\\"service_updated\\\",\\\"updatedAt\\\":\\\"2026-10-10T06:11:31.802609Z\\\"}\"}],\"structuredContent\":{\"bexTrigger\":\"config_change\",\"createdAt\":\"2026-10-10T06:11:31.802609Z\",\"id\":\"dep-db4te4r4am4s73f098s0\",\"image\":{\"ref\":\"docker.io/library/busybox:1.37\"},\"serviceId\":\"srv-db4td5j93q6c73at05jg\",\"status\":\"created\",\"trigger\":\"service_updated\",\"updatedAt\":\"2026-10-10T06:11:31.802609Z\"}}}\n\n"
  }
}
```

## a33BModeLogs

```json
{
  "at": "2026-10-10T06:12:44.757Z",
  "request": {
    "operationName": "QaA33ModeLogs",
    "query": "query QaA33ModeLogs($serviceId:String!,$start:String!,$end:String!){logs(resource:$serviceId,startTime:$start,endTime:$end,limit:50,text:\"QA_A33R_START\",type:\"app\"){logs{timestamp message instance type level} hasMore nextStartTime nextEndTime}}",
    "variables": {
      "serviceId": "srv-db4td5j93q6c73at05jg",
      "start": "2026-10-10T06:09:00Z",
      "end": "2026-10-10T06:12:44.384Z"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db4td5j93q6c73at05jg-ctqabte3tatjrtidmumj",
            "level": "unknown",
            "message": "QA_A33R_START",
            "timestamp": "2026-10-10T06:09:30.512777428Z",
            "type": "app"
          },
          {
            "instance": "srv-db4td5j93q6c73at05jg-uqalqgq18ved6g513n2f",
            "level": "unknown",
            "message": "QA_A33R_START",
            "timestamp": "2026-10-10T06:10:03.269461075Z",
            "type": "app"
          },
          {
            "instance": "srv-db4td5j93q6c73at05jg-r6mmbjdp2lse68v8c44j",
            "level": "unknown",
            "message": "QA_A33R_START",
            "timestamp": "2026-10-10T06:10:56.600168103Z",
            "type": "app"
          },
          {
            "instance": "srv-db4td5j93q6c73at05jg-8sotj9gk9jse6cqub4os",
            "level": "unknown",
            "message": "QA_A33R_START",
            "timestamp": "2026-10-10T06:11:35.794175355Z",
            "type": "app"
          }
        ],
        "nextEndTime": "2026-10-10T06:09:30.512777427Z",
        "nextStartTime": "2026-10-10T06:09:00Z"
      }
    }
  }
}
```

## a33ARepeatSet

```json
{
  "at": "2026-10-10T06:14:58.609Z",
  "request": [
    {
      "operationName": "SetHealthCheckPath",
      "variables": {
        "id": "srv-db4ejbr93q6c73at02pg",
        "path": "/"
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
          "healthCheckPath": "/",
          "id": "srv-db4ejbr93q6c73at02pg"
        }
      }
    }
  ]
}
```

## a33ARepeatObserve

```json
{
  "at": "2026-10-10T06:15:16.481Z",
  "request": {
    "operationName": "QaA33Repeat",
    "query": "query QaA33Repeat($serviceId:String!){service(id:$serviceId){id name phase revision healthCheckPath undeployedChanges} deploys(serviceId:$serviceId){id status createdAt startedAt finishedAt trigger cancelReason failureReason stallReason} logs(resource:$serviceId,startTime:\"2026-10-10T06:13:00Z\",endTime:\"2026-10-10T06:20:00Z\",limit:30,text:\"QA_A33_START\",type:\"app\"){logs{timestamp message instance type level} hasMore nextStartTime nextEndTime}}",
    "variables": {
      "serviceId": "srv-db4ejbr93q6c73at02pg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:14:58.544332Z",
          "failureReason": "",
          "finishedAt": "",
          "id": "dep-db4tfoj4am4s73f0990g",
          "stallReason": "",
          "startedAt": "",
          "status": "created",
          "trigger": "config_change"
        },
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:14:08.791174Z",
          "failureReason": "",
          "finishedAt": "2026-10-10T06:14:27.660192Z",
          "id": "dep-db4tfc34am4s73f098v0",
          "stallReason": "",
          "startedAt": "2026-10-10T06:14:13.778912Z",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "cancelReason": "",
          "createdAt": "2026-10-10T06:13:09.045352Z",
          "failureReason": "",
          "finishedAt": "2026-10-10T06:13:27.626724Z",
          "id": "dep-db4tetb93q6c73at05pg",
          "stallReason": "",
          "startedAt": "2026-10-10T06:13:13.596137Z",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "cancelReason": "Superseded by a newer release",
          "createdAt": "2026-10-09T13:20:01.594376Z",
          "failureReason": "",
          "finishedAt": "2026-10-09T13:38:13.578021Z",
          "id": "dep-db4ek0b4am4s73f0969g",
          "stallReason": "",
          "startedAt": "",
          "status": "canceled",
          "trigger": "config_change"
        },
        {
          "cancelReason": "",
          "createdAt": "2026-10-09T13:18:39.352618Z",
          "failureReason": "",
          "finishedAt": "2026-10-09T13:18:57.489929Z",
          "id": "dep-db4ejbr93q6c73at02q0",
          "stallReason": "",
          "startedAt": "2026-10-09T13:18:43.53932Z",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db4ejbr93q6c73at02pg-arpb1hh1oapvb7bt9tm6",
            "level": "unknown",
            "message": "QA_A33_START",
            "timestamp": "2026-10-10T06:13:10.860650759Z",
            "type": "app"
          },
          {
            "instance": "srv-db4ejbr93q6c73at02pg-ruq9s6juplfh79uk44eo",
            "level": "unknown",
            "message": "QA_A33_START",
            "timestamp": "2026-10-10T06:14:10.627776082Z",
            "type": "app"
          },
          {
            "instance": "srv-db4ejbr93q6c73at02pg-bc3486adrjhb0lvkj0ve",
            "level": "unknown",
            "message": "QA_A33_START",
            "timestamp": "2026-10-10T06:15:00.432243923Z",
            "type": "app"
          }
        ],
        "nextEndTime": "2026-10-10T06:13:10.860650758Z",
        "nextStartTime": "2026-10-10T06:13:00Z"
      },
      "service": {
        "healthCheckPath": "/",
        "id": "srv-db4ejbr93q6c73at02pg",
        "name": "qa-20261009-loop-a33-health",
        "phase": "Running",
        "revision": "rev-4",
        "undeployedChanges": false
      }
    }
  },
  "public": {
    "status": 200,
    "body": "QA_A33_HTTP_OK\n"
  }
}
```

The first fixture's terminal observation precedes its later recovery/repetition: its complete two-row history shows no newer deploy at the time of the false cancellation. The reverse probe's START logs prove replacement, not the raw Kubernetes probe shape or CR generation. The fresh forward repeat includes prior Live controls and a new instance. Both repeats were canceled deliberately before deletion; no natural timeout is claimed for those rows.
