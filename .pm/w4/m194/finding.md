# Command clearing revives a hidden Blueprint cron command

- **Severity:** major. An ordinary successful command-clear save is misleading: the next cron executes the old override while Settings and GraphQL say Command is empty. All changes used the owned Free fixture.
- **Source:** user-requested functional qa-find-bugs loop, 2026-10-10 UTC pass a48, muse profile, workspace `tea-d98210cbbpdc73dcrkvg`. Research HEAD `4c94bfce2`. Filing only.
- **Expected:** a prebuilt-image cron reports its configured runtime override in the editable Command field. Explicitly clearing it removes that override, including any legacy StartCommand fallback, so subsequent jobs use the image's own entrypoint/CMD.
- **Actual:** the Blueprint command is stored as StartCommand while Command is empty. Replacing Command works. Clearing it successfully restores the still-stored initial command through the runtime fallback.

## Reproduction

Owned Free image cron **qa-20261010-loop-a48-cron**, `srv-db515jsb8ulc73efu3p0`, BusyBox 1.37, schedule `0 0 1 1 *` (next scheduled run 2027-01-01 UTC), auto-deploy off. Created through the normal inline Blueprint apply endpoint, without a connected Blueprint record.

1. Apply the manifest in the complete creation transaction below. Wait for the initial deploy to become Live. Its Docker Command is `sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'`.
2. Open **Settings → Deploy**. Command is blank and its hint says leaving it blank runs the image's own command. Entering Edit shows Save changes disabled because the displayed value is already empty. Repeat after a fresh reload: still blank. GraphQL has `command:""` and the original nonempty `startCommand`. REST also omits the cron command/env-specific override.
3. Edit Command to a harmless replacement and Save changes. The response and field show the replacement. Edit again, clear it, and Save changes. The captured mutation correctly sends **`command:""`** and returns empty; reload keeps the field blank. Wait for the clear's config-change deployment to be Live, then trigger a run.
4. First failing run `crr-rh20rpvuehdfdkd6o4mp`: after the clear at **10:34:20.696Z**, the run begins **10:34:42Z**, logs the original **qa-a48-start** marker at **10:34:44.030303217Z**, and finishes successful at **10:35:46Z** (64 seconds).
5. Fresh-page repeat: save another nonempty replacement at **10:36:18.964Z**, clear it at **10:36:19.308Z**, and reload. At **10:37:01.653Z**, the clear's deploy is Live, the service is Running / rev-10, Command is empty, and StartCommand still holds the original script. Trigger run `crr-8k83n52d7gte0ikfbn9r` at **10:37:04.127Z**. It logs **qa-a48-start** and **qa-a48-complete**, then finishes successful at **10:38:08Z**, again 64 seconds.
6. Strong control: save `sh -c 'echo qa-a48-replacement-control'`, wait for its deploy to be Live, then trigger `crr-v902sutmgggq54mso5d6`. The original StartCommand still exists, but the nonempty Command wins: this run logs only **qa-a48-replacement-control** and succeeds from **10:39:33Z→10:39:37Z**. Its log query has `hasMore:false`.

The second repeat's short-lived replacement deployment is superseded by the immediate clear; the evidence explicitly waits for the clear deployment to be Live before running. This is not an old serving release temporarily executing during rollout.

## Durable transactions

Each HTTP capture below contains the complete request body and response for its named transaction. GraphQL dashboard operations are arrays with the actual Apollo extension. Authentication headers and cookies are omitted. UI snapshots are separate; graphQL-only extracts below omit those snapshot strings, not response fields. All commands and log markers are harmless QA fixtures.

Creation, POST `https://api.bex.co/v1/blueprints/deploy`:

```json
{
  "time": "2026-10-10T10:26:23.711Z",
  "request": {
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "bexYaml": "services:\n  - type: cron\n    name: qa-20261010-loop-a48-cron\n    runtime: image\n    image:\n      url: docker.io/library/busybox:1.37\n    plan: free\n    autoDeployTrigger: off\n    schedule: '0 0 1 1 *'\n    dockerCommand: \"sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'\"\n"
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-db515jsb8ulc73efu3p0",
        "name": "qa-20261010-loop-a48-cron",
        "slug": "qa-20261010-loop-a48-cron",
        "displayName": "",
        "type": "cron_job",
        "phase": "",
        "undeployedChanges": false,
        "undeployedChangesApplying": false,
        "url": "",
        "urls": null,
        "image": "",
        "sourceImage": "docker.io/library/busybox:1.37",
        "runtime": "image",
        "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'",
        "builder": "auto",
        "replicas": 1,
        "suspended": false,
        "schedule": "0 0 1 1 *",
        "nextRunAt": "2027-01-01T00:00:00Z",
        "plan": "free",
        "revision": "",
        "createdAt": "2026-10-10T10:26:23Z",
        "updatedAt": "2026-10-10T10:26:23.588551967Z",
        "dashboardUrl": "https://dashboard.bex.co/cron/srv-db515jsb8ulc73efu3p0",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "",
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        },
        "latestDeployId": "dep-db515jsb8ulc73efu3pg"
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": null
  }
}
```

Fresh original state through GraphQL, and complete REST `GET https://api.bex.co/v1/services/srv-db515jsb8ulc73efu3p0` response:

```json
{
  "time": "2026-10-10T10:32:02.512Z",
  "graphql": {
    "request": {
      "query": "query($id:String!){server(id:$id){id name runtime startCommand command schedule phase revision}}",
      "variables": {
        "id": "srv-db515jsb8ulc73efu3p0"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "server": {
          "command": "",
          "id": "srv-db515jsb8ulc73efu3p0",
          "name": "qa-20261010-loop-a48-cron",
          "phase": "Running",
          "revision": "rev-1",
          "runtime": "image",
          "schedule": "0 0 1 1 *",
          "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
        }
      }
    }
  },
  "rest": {
    "status": 200,
    "response": {
      "id": "srv-db515jsb8ulc73efu3p0",
      "name": "qa-20261010-loop-a48-cron",
      "immutableName": "qa-20261010-loop-a48-cron",
      "slug": "qa-20261010-loop-a48-cron",
      "displayName": "",
      "type": "cron_job",
      "suspended": "not_suspended",
      "dashboardUrl": "https://dashboard.bex.co/cron/srv-db515jsb8ulc73efu3p0",
      "createdAt": "2026-10-10T10:26:23Z",
      "updatedAt": "2026-10-10T10:31:37Z",
      "owner": {
        "id": "tea-d98210cbbpdc73dcrkvg",
        "name": "bex",
        "email": "puncsky@gmail.com",
        "type": "team"
      },
      "serviceDetails": {
        "env": "image",
        "lastSuccessfulRunAt": "2026-10-10T10:30:09Z",
        "nextRunAt": "2027-01-01T00:00:00Z",
        "numInstances": 1,
        "plan": "free",
        "region": "fsn1",
        "runtime": "image",
        "schedule": "0 0 1 1 *"
      },
      "imagePath": "docker.io/library/busybox:1.37",
      "suspenders": [],
      "ownerId": "tea-d98210cbbpdc73dcrkvg",
      "phase": "Running",
      "replicas": 1,
      "revision": "rev-1",
      "schedule": "0 0 1 1 *",
      "runs": [
        {
          "id": "crr-j7vmq43j8vu7uobtrrb3",
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261010-loop-a48-cron-run-71c98662",
          "startedAt": "2026-10-10T10:29:05Z",
          "finishedAt": "2026-10-10T10:30:09Z",
          "status": "successful"
        },
        {
          "id": "crr-f35r26vuaeibv9nndnun",
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261010-loop-a48-cron-run-95f51894",
          "startedAt": "2026-10-10T10:28:03Z",
          "finishedAt": "2026-10-10T10:28:40.98503361Z",
          "status": "canceled"
        },
        {
          "id": "crr-q9t6q01ghr37vtri4d98",
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261010-loop-a48-cron-run-2a900e94",
          "startedAt": "2026-10-10T10:27:12Z",
          "finishedAt": "2026-10-10T10:27:32.806089416Z",
          "status": "canceled"
        }
      ],
      "lastSuccessfulRunAt": "2026-10-10T10:30:09Z",
      "nextRunAt": "2027-01-01T00:00:00Z",
      "idleTTLSeconds": 0,
      "autoDeploy": "no",
      "autoDeployTrigger": "off",
      "pushDeliveryMethod": "none",
      "notifyOnFail": "default",
      "notificationsToSend": "default"
    }
  },
  "commandValue": ""
}
```

First clear after a replacement:

```json
{
  "capture": {
    "time": "2026-10-10T10:34:20.696Z",
    "request": [
      {
        "operationName": "UpdateCronJob",
        "variables": {
          "id": "srv-db515jsb8ulc73efu3p0",
          "schedule": "0 0 1 1 *",
          "command": ""
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation UpdateCronJob($id: String!, $schedule: String!, $command: String, $confirm: String) {\n  updateCronJob(\n    id: $id\n    schedule: $schedule\n    command: $command\n    confirm: $confirm\n  ) {\n    id\n    schedule\n    command\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "updateCronJob": {
            "__typename": "Service",
            "command": "",
            "id": "srv-db515jsb8ulc73efu3p0",
            "schedule": "0 0 1 1 *"
          }
        }
      }
    ]
  },
  "time": "2026-10-10T10:34:23.182Z",
  "value": ""
}
```

First post-clear trigger:

```json
{
  "time": "2026-10-10T10:34:42.211Z",
  "dialog": "- alertdialog \"Trigger a run now?\":\n  - heading \"Trigger a run now?\" [level=2]\n  - paragraph: This runs the job's command immediately, outside its schedule. Only one run can be active at a time.\n  - button \"Go Back\"\n  - button \"Trigger Run\"",
  "capture": {
    "request": [
      {
        "operationName": "RunCronJob",
        "variables": {
          "id": "srv-db515jsb8ulc73efu3p0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation RunCronJob($id: String!) {\n  runCronJob(id: $id) {\n    id\n    status\n    startedAt\n    finishedAt\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "runCronJob": {
            "__typename": "CronRun",
            "finishedAt": "",
            "id": "crr-rh20rpvuehdfdkd6o4mp",
            "startedAt": "",
            "status": "pending"
          }
        }
      }
    ]
  }
}
```

First post-clear runtime read; this capture is before completion and proves the initial marker, not both markers:

```json
{
  "time": "2026-10-10T10:35:24.404Z",
  "request": {
    "query": "query($id:String!,$end:String!){server(id:$id){id command startCommand phase revision} cronJobRuns(serviceId:$id,limit:20){id status startedAt finishedAt} logs(resource:$id,type:\"app\",startTime:\"2026-10-10T10:34:23Z\",endTime:$end,limit:100){hasMore logs{timestamp message instance type}}}",
    "variables": {
      "id": "srv-db515jsb8ulc73efu3p0",
      "end": "2026-10-10T10:35:24.048Z"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "cronJobRuns": [
        {
          "finishedAt": "",
          "id": "crr-rh20rpvuehdfdkd6o4mp",
          "startedAt": "2026-10-10T10:34:42Z",
          "status": "pending"
        },
        {
          "finishedAt": "2026-10-10T10:30:09Z",
          "id": "crr-j7vmq43j8vu7uobtrrb3",
          "startedAt": "2026-10-10T10:29:05Z",
          "status": "successful"
        },
        {
          "finishedAt": "2026-10-10T10:28:40.98503361Z",
          "id": "crr-f35r26vuaeibv9nndnun",
          "startedAt": "2026-10-10T10:28:03Z",
          "status": "canceled"
        },
        {
          "finishedAt": "2026-10-10T10:27:32.806089416Z",
          "id": "crr-q9t6q01ghr37vtri4d98",
          "startedAt": "2026-10-10T10:27:12Z",
          "status": "canceled"
        }
      ],
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db515jsb8ulc73efu3p0-njbvovsc3k2qqsi1e6mc",
            "message": "qa-a48-start",
            "timestamp": "2026-10-10T10:34:44.030303217Z",
            "type": "app"
          }
        ]
      },
      "server": {
        "command": "",
        "id": "srv-db515jsb8ulc73efu3p0",
        "phase": "Running",
        "revision": "rev-7",
        "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
      }
    }
  }
}
```

The first run's completed state before starting the fresh repeat:

```json
{
  "time": "2026-10-10T10:36:16.274Z",
  "request": {
    "query": "query($id:String!){cronJobRuns(serviceId:$id,limit:20){id status startedAt finishedAt} deploys(serviceId:$id,limit:10){id status trigger} server(id:$id){id command startCommand phase revision}}",
    "variables": {
      "id": "srv-db515jsb8ulc73efu3p0"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "cronJobRuns": [
        {
          "finishedAt": "2026-10-10T10:35:46Z",
          "id": "crr-rh20rpvuehdfdkd6o4mp",
          "startedAt": "2026-10-10T10:34:42Z",
          "status": "successful"
        },
        {
          "finishedAt": "2026-10-10T10:30:09Z",
          "id": "crr-j7vmq43j8vu7uobtrrb3",
          "startedAt": "2026-10-10T10:29:05Z",
          "status": "successful"
        },
        {
          "finishedAt": "2026-10-10T10:28:40.98503361Z",
          "id": "crr-f35r26vuaeibv9nndnun",
          "startedAt": "2026-10-10T10:28:03Z",
          "status": "canceled"
        },
        {
          "finishedAt": "2026-10-10T10:27:32.806089416Z",
          "id": "crr-q9t6q01ghr37vtri4d98",
          "startedAt": "2026-10-10T10:27:12Z",
          "status": "canceled"
        }
      ],
      "deploys": [
        {
          "id": "dep-db519b46flus738pnio0",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "id": "dep-db5194s6flus738pnin0",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db515jsb8ulc73efu3pg",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "server": {
        "command": "",
        "id": "srv-db515jsb8ulc73efu3p0",
        "phase": "Running",
        "revision": "rev-7",
        "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
      }
    }
  }
}
```

Fresh-page repeated clear; the preceding nonempty save is retained in the local ledger:

```json
{
  "time": "2026-10-10T10:36:19.308Z",
  "request": [
    {
      "operationName": "UpdateCronJob",
      "variables": {
        "id": "srv-db515jsb8ulc73efu3p0",
        "schedule": "0 0 1 1 *",
        "command": ""
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation UpdateCronJob($id: String!, $schedule: String!, $command: String, $confirm: String) {\n  updateCronJob(\n    id: $id\n    schedule: $schedule\n    command: $command\n    confirm: $confirm\n  ) {\n    id\n    schedule\n    command\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "updateCronJob": {
          "__typename": "Service",
          "command": "",
          "id": "srv-db515jsb8ulc73efu3p0",
          "schedule": "0 0 1 1 *"
        }
      }
    }
  ]
}
```

Authoritative Live deployment control before the repeated trigger, and its complete trigger response:

```json
{
  "before": {
    "time": "2026-10-10T10:37:01.653Z",
    "request": {
      "query": "query($id:String!){server(id:$id){id command startCommand phase revision} deploys(serviceId:$id,limit:5){id status trigger}}",
      "variables": {
        "id": "srv-db515jsb8ulc73efu3p0"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "deploys": [
          {
            "id": "dep-db51a8s6flus738pniqg",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "id": "dep-db51a8kb8ulc73efu3ug",
            "status": "canceled",
            "trigger": "config_change"
          },
          {
            "id": "dep-db519b46flus738pnio0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db5194s6flus738pnin0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db515jsb8ulc73efu3pg",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "server": {
          "command": "",
          "id": "srv-db515jsb8ulc73efu3p0",
          "phase": "Running",
          "revision": "rev-10",
          "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
        }
      }
    }
  },
  "time": "2026-10-10T10:37:04.127Z",
  "capture": {
    "request": [
      {
        "operationName": "RunCronJob",
        "variables": {
          "id": "srv-db515jsb8ulc73efu3p0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation RunCronJob($id: String!) {\n  runCronJob(id: $id) {\n    id\n    status\n    startedAt\n    finishedAt\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "runCronJob": {
            "__typename": "CronRun",
            "finishedAt": "",
            "id": "crr-8k83n52d7gte0ikfbn9r",
            "startedAt": "",
            "status": "pending"
          }
        }
      }
    ]
  }
}
```

Completed repeated run and both original markers, while Command remains empty:

```json
{
  "time": "2026-10-10T10:38:34.911Z",
  "request": {
    "query": "query($id:String!,$end:String!){server(id:$id){id command startCommand phase revision} cronJobRun(serviceId:$id,runId:\"crr-8k83n52d7gte0ikfbn9r\"){id status startedAt finishedAt} logs(resource:$id,type:\"app\",startTime:\"2026-10-10T10:37:04Z\",endTime:$end,limit:100){hasMore logs{timestamp message instance type}}}",
    "variables": {
      "id": "srv-db515jsb8ulc73efu3p0",
      "end": "2026-10-10T10:38:34.653Z"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "cronJobRun": {
        "finishedAt": "2026-10-10T10:38:08Z",
        "id": "crr-8k83n52d7gte0ikfbn9r",
        "startedAt": "2026-10-10T10:37:04Z",
        "status": "successful"
      },
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db515jsb8ulc73efu3p0-vcmh50ti1lio5vevrmh0",
            "message": "qa-a48-start",
            "timestamp": "2026-10-10T10:37:05.99494307Z",
            "type": "app"
          },
          {
            "instance": "srv-db515jsb8ulc73efu3p0-vcmh50ti1lio5vevrmh0",
            "message": "qa-a48-complete",
            "timestamp": "2026-10-10T10:38:05.998767106Z",
            "type": "app"
          }
        ]
      },
      "server": {
        "command": "",
        "id": "srv-db515jsb8ulc73efu3p0",
        "phase": "Running",
        "revision": "rev-10",
        "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
      }
    }
  }
}
```

Working nonempty replacement save:

```json
{
  "time": "2026-10-10T10:38:56.513Z",
  "request": [
    {
      "operationName": "UpdateCronJob",
      "variables": {
        "id": "srv-db515jsb8ulc73efu3p0",
        "schedule": "0 0 1 1 *",
        "command": "sh -c 'echo qa-a48-replacement-control'"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation UpdateCronJob($id: String!, $schedule: String!, $command: String, $confirm: String) {\n  updateCronJob(\n    id: $id\n    schedule: $schedule\n    command: $command\n    confirm: $confirm\n  ) {\n    id\n    schedule\n    command\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "updateCronJob": {
          "__typename": "Service",
          "command": "sh -c 'echo qa-a48-replacement-control'",
          "id": "srv-db515jsb8ulc73efu3p0",
          "schedule": "0 0 1 1 *"
        }
      }
    }
  ]
}
```

Live replacement deployment before the control trigger:

```json
{
  "time": "2026-10-10T10:39:33.317Z",
  "ready": true,
  "before": {
    "request": {
      "query": "query($id:String!){server(id:$id){id command startCommand phase revision} deploys(serviceId:$id,limit:1){id status trigger}}",
      "variables": {
        "id": "srv-db515jsb8ulc73efu3p0"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "deploys": [
          {
            "id": "dep-db51bg46flus738pnit0",
            "status": "live",
            "trigger": "config_change"
          }
        ],
        "server": {
          "command": "sh -c 'echo qa-a48-replacement-control'",
          "id": "srv-db515jsb8ulc73efu3p0",
          "phase": "Running",
          "revision": "rev-12",
          "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
        }
      }
    }
  },
  "capture": {
    "request": [
      {
        "operationName": "RunCronJob",
        "variables": {
          "id": "srv-db515jsb8ulc73efu3p0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation RunCronJob($id: String!) {\n  runCronJob(id: $id) {\n    id\n    status\n    startedAt\n    finishedAt\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "runCronJob": {
            "__typename": "CronRun",
            "finishedAt": "",
            "id": "crr-v902sutmgggq54mso5d6",
            "startedAt": "",
            "status": "pending"
          }
        }
      }
    ]
  }
}
```

Completed control run, complete narrow log selection, and retained legacy StartCommand:

```json
{
  "time": "2026-10-10T10:40:18.240Z",
  "request": {
    "query": "query($id:String!,$end:String!){server(id:$id){id command startCommand phase revision} cronJobRun(serviceId:$id,runId:\"crr-v902sutmgggq54mso5d6\"){id status startedAt finishedAt} logs(resource:$id,type:\"app\",startTime:\"2026-10-10T10:39:33Z\",endTime:$end,limit:100){hasMore logs{timestamp message instance type}}}",
    "variables": {
      "id": "srv-db515jsb8ulc73efu3p0",
      "end": "2026-10-10T10:40:17.976Z"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "cronJobRun": {
        "finishedAt": "2026-10-10T10:39:37Z",
        "id": "crr-v902sutmgggq54mso5d6",
        "startedAt": "2026-10-10T10:39:33Z",
        "status": "successful"
      },
      "logs": {
        "hasMore": false,
        "logs": [
          {
            "instance": "srv-db515jsb8ulc73efu3p0-edm6v1ic08nlhr4g7a0i",
            "message": "qa-a48-replacement-control",
            "timestamp": "2026-10-10T10:39:34.996831381Z",
            "type": "app"
          }
        ]
      },
      "server": {
        "command": "sh -c 'echo qa-a48-replacement-control'",
        "id": "srv-db515jsb8ulc73efu3p0",
        "phase": "Running",
        "revision": "rev-12",
        "startCommand": "sh -c 'echo qa-a48-start; sleep 60; echo qa-a48-complete'"
      }
    }
  }
}
```

## Root cause and target behavior

1. `lego/backend/internal/apps/deploy.go:2227–2251` resolves the manifest command through `manifestStartAndAutoDeploy:2347–2355` and puts it into CreateRequest.StartCommand. `apps/service.go:2893` stores StartCommand; `applyOptionalCreateSpec:3134–3136` fills cron Command only from CreateRequest.Command. Initial Blueprint crons intentionally keep that representation: `blueprint_plan.go:128–143` preserves an unchanged fallback to avoid turning native images' baked Bash command into a runtime shell override. The existing create/serialized tests explicitly expect Command empty and StartCommand retained.
2. `apps/service.go:1090–1095` reads both raw fields into AppView, returning Command only from Spec.Command. `apps/graphql.go:384` exposes that Command. `apps/render.go:374,416,522–523,559,589` uses AppView.Command for cron metadata and runtime-specific projections. For this image cron it therefore omits the override even though the runtime uses StartCommand. The dashboard faithfully displays the empty value: `routes/services.$serviceId.settings.tsx` passes service.command to CronDeploySection; `cron-deploy-section.tsx:93–105` renders it and forwards explicit empty. The actual empty-string request above reaches the backend correctly.
3. `apps/service.go:4307–4312` SetCronJob assigns **only Spec.Command** on a supplied command. Its sibling SetCommands at **4057–4061** does the same for cron start-command aliases. Neither removes the older StartCommand on clear.
4. `lego/operator/internal/controller/app_controller.go:4564–4569` uses Command first, then StartCommand for builders other than native/buildpack. Empty Command therefore revives the original image override. The runtime selection is important: `app_controller.go:4093–4100` calls selectedRuntimeApp before cronPodSpec; `release_config_selection.go:272–290` projects the selected release's StartCommand and optional Command. Both fields need the correct cleared value in the new carrying release, not just a UI change.
5. Existing Blueprint clearing already handles the correct pair: `blueprint_plan.go:137–139` clears StartCommand when Command is explicitly cleared. That correct sibling is source-traced, not a claimed live Blueprint-clear control in this pass.

**Target:** allowlist this correction to cron runtime overrides. For image/Dockerfile crons, expose the configured fallback as the effective saved override when Command is empty, and make every explicit command-clear entrypoint remove both runtime override slots before the new release is materialized. A later job must have no platform command override and run the image default. Preserve a genuine nonempty Command, omitted/null command's keep behavior, and schedule-only updates.

**Native/buildpack boundary:** preserve their baked startup/build configuration and Bash behavior. Do not globally copy StartCommand into Command or globally erase native StartCommand. Use the actual build-strategy decisions, not a runtime-name guess: backend `build_strategy.go:49–67`, operator `effectiveBuilder` and the cronPodSpec native/buildpack exclusion were opened. Existing AppSpecs created before a fix must become readable and clearable too; correcting only future Blueprint creation leaves this observed fixture class broken.

**Types and framework:** AppSpec.Command and StartCommand are string fields with omitempty (`lego/types/v1alpha1/app_types.go:355,532`); release snapshots carry StartCommand and optional Command (`releasesnapshots.go:100–103`). No schema widening is required. `gqlutil.StrPtr:400–406` preserves a supplied empty string; pinned **graphql-go/graphql v0.8.1** actual `values.go:299–321` does not classify it as null, and `scalars.go:307–324` serializes it unchanged. The live **Apollo 4.1.3** request also preserves it. The defect is neither the old empty-to-null dashboard conversion nor transport coercion. The existing tracked rollout path remains responsible for the carrying deployment; preserve its conflict/error behavior.

## Shared callers, aliases and verification scope

Exhaustive non-test searches find **two SetCronJob call sites**: GraphQL updateCronJob and the settings registry. **Three SetCommands call sites**: the registry, GraphQL setBuildCommand and GraphQL setStartCommand; the build-only call supplies no start change. The registry's two speculative mutations at `settings.go:323,390` must model the same field-pair transition as the applied writers, so preflight validation does not reason about a different resulting spec.

The AppView producer has **12 production `s.view(...)` call sites**: service.go 8, deploy.go 2, settings.go 1, placement.go 1. A read correction is allowlisted to cron runtime overrides, rather than changing all service types. REST rendering and GraphQL service objects share it; MCP get_service/update_service reuse the render/get/update paths. The shared-write/projection audit is its own milestone task.

| Family / route | Relevant behavior | Live status this pass |
| --- | --- | --- |
| Image cron, /services/:id/settings; /cron/:id/settings redirect | Command editor and shared run template | Both clears, fresh read and nonempty control verified |
| Dockerfile cron | Same fallback branch; REST accepts both cron command spellings | Source traced, no live build/run |
| Native/buildpack cron | Baked startup configuration; no non-native runtime fallback | Source traced; Bash/native execution unverified |
| Web, private, worker | Own StartCommand, not the cron Command slot | Source traced; no live command changes |
| Static | No running cron container | No runtime claim |
| Postgres / Key Value | Separate resource types and APIs | No command-write claim |
| REST PATCH /v1/services/:id | top-level command, serviceDetails.command, and envSpecificDetails start/Docker-command aliases enter the shared registry | Read tested; these write aliases unverified live |
| GraphQL | updateCronJob; setStartCommand alias; setBuildCommand preserves start when absent | updateCronJob tested; other writes source-traced |
| MCP | get_service / update_service command and startCommand inputs | Source traced, not live |

Also unverified: scheduled execution (the schedule was safely in January), native image default after a clear, failed/canceled clear rollout and rollback, concurrent conflicting writes, protected retry, and CLI round-trip/clone. Cancellation/trigger/activity controls were exercised earlier in this pass; their success does not certify immediate process termination. One canceled process completed during its termination grace before its replacement started; no separate cancellation finding is asserted.

## Dedupe and prior acceptance walk

Open, blocked and done searches covered cron command, blank/empty/default, legacy fallback, Blueprint create/read/clear and the shared writers. The full anti-goals file, all eight open milestone titles across workstreams, the last 40 dashboard/lego commits and targeted command history were checked. Prior live hunts w9/done/m89 and m92 were used as filing precedent.

- **w4/done/137** fixes the dashboard's empty-to-null conversion. Its full DoD passes for this wire/readback portion: both fresh-page clears send empty, return empty, keep an empty row, preserve the schedule, and nonempty replacement is persisted/displayed. Its text explicitly excludes executing the image default and does not own the remaining runtime fallback. Existing protected/failed-save/schedule controls remain verification work, not tested claims here.
- **w4/done/174** fixes unchanged Blueprint reapply and Blueprint explicit clear. The full completion record includes a later native-shell preservation correction; its earlier creation-time Command copy is superseded. Unchanged serialized repeat identity/spec/resourceVersion/restart timestamp/no-deploy, genuine update/one deploy, omitted command preservation, and explicit Blueprint clear of both slots are source/test-traced; none was live-replayed here. It does not change SetCronJob/SetCommands clearing. Current main has the correct Blueprint-clear helper and the still-incomplete API writers.
- **w9/done/m165** fixes CLI Docker cron create/read/update/clone. Full DoD walk: Docker CLI create carrying command, REST/GraphQL/MCP read/clone projections, CLI update readback, native control, and verifier drift detection were read; none is claimed live-tested by this image/Blueprint dashboard journey. Its closeout expressly does not claim image-default execution after empty Command. This is an uncovered fallback-input/runtime-clear case, not a rollback of the tested direct-CLI command projection.
- **w8/blocked/072** is the upstream image-cron CLI update builder refusal. This dashboard sends a valid request and receives success; this backend fallback defect is independent. Keep that upstream blocker and do not fork the CLI.

Target history: `a29e8a321` clears legacy fallbacks **in Blueprint apply**; `18958459c` preserves native initial commands; `503706776` accepts image dockerCommand; later create guards remain. Current main still has the exact SetCronJob/SetCommands and read/runtime mismatch. No deployed-build-versus-HEAD contradiction or already-landed fix was found.

[Render's official cron setup](https://render.com/docs/cronjobs#setup) distinguishes Docker image startup commands from explicit Docker Command overrides. This finding enforces bex's own visible blank-to-image-default promise; it does not assume identical authenticated Render dialog labels or declare a CLI-builder fix.

## Artifacts and cleanup

Inspected `.playwright-mcp/qa-cron-command-a48-blank.png` shows the empty optional Command and its image-default hint. `.playwright-mcp/qa-cron-controls-a48-ledger.json` contains full captures, controls, baseline and cleanup. `qa-cron-controls-a48-console-final.txt` has zero errors and warnings before logout. Network history is retained in `qa-cron-controls-a48-network.txt`; it spans earlier passes, so prior teardown responses are not this finding. Screenshots and ledgers are ignored session evidence; the requests and responses above are durable.

Typed UI deletion and the complete following REST read:

```json
{
  "delete": {
    "time": "2026-10-10T10:40:42.683Z",
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db515jsb8ulc73efu3p0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "deleteService": true
        }
      }
    ]
  },
  "read": {
    "time": "2026-10-10T10:40:52.136Z",
    "url": "https://api.bex.co/v1/services/srv-db515jsb8ulc73efu3p0",
    "status": 404,
    "response": {
      "code": "NOT_FOUND",
      "error": "not found",
      "id": "not_found",
      "message": "not found",
      "params": null
    }
  }
}
```

At **2026-10-10T10:40:52Z**, the six-family inventory exactly matched baseline IDs: five services, four Postgres, two Key Values, three projects, one pre-existing group and one Blueprint. No connected Blueprint was created. Other workers' QA-named resources were preserved. The owned session was revoked; old-cookie whoami was **401 at 10:41:32.203Z**, cookies zero, viewport restored to 1440×1000, browser about:blank, and owned jar/state removed. Cleanup is complete.
