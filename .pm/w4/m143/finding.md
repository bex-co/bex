# Major — first service-page group creation submits a stale Workspace scope

Why: an ordinary first attempt fails even though the service's real environment is known and the dialog claims no service is selected.

## Reproduction and durable evidence

2026-10-02 07:21–07:29 UTC. In `bex` / `tea-d98210cbbpdc73dcrkvg`, create project `qa-20261002-muse-r1`, environment `qa-20261002-stage-r1`, and web service `qa-20261002-web-r1`. Hard-load `/services/srv-davlmi6o5s1c7398lhug/env`, then top **Create group**. It displays Workspace (no Environment), no link candidates, but retains the current service ID internally. Submit `qa-20261002-group-r1`: HTTP 200 GraphQL failure with ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH. Reproduced after another hard load. Cancel and reopen once: correct environment/current service appear and the exact same name creates successfully.

Expected: first open reflects the service scope and visible checked service. Actual: initial null scope survives asynchronous scope loading and the controlled open; a hidden service ID is submitted against null environment.

The local screenshot `.playwright-mcp/qa-env-r1-1.png` and JSON were verified on disk. Full request/response (browser-session POST `https://api.bex.co/graphql`, JSON Content-Type, credentials included) and scope proof follow; the snapshot is the failure's accessible state. These fixture IDs have been deleted.

```json
{
  "date": "2026-10-02T07:22:36.581Z",
  "scopeProbe": {
    "request": {
      "query": "query QaScope { environment(id:\"env-davlm9oaijhc73cn2qfg\") { id projectId name serviceIds } project(id:\"prj-davlm5mo5s1c7398lhtg\") { id name serviceIds } }"
    },
    "status": 200,
    "body": {
      "data": {
        "environment": {
          "id": "env-davlm9oaijhc73cn2qfg",
          "name": "qa-20261002-stage-r1",
          "projectId": "prj-davlm5mo5s1c7398lhtg",
          "serviceIds": ["srv-davlmi6o5s1c7398lhug"]
        },
        "project": {
          "id": "prj-davlm5mo5s1c7398lhtg",
          "name": "qa-20261002-muse-r1",
          "serviceIds": ["srv-davlmi6o5s1c7398lhug"]
        }
      }
    }
  },
  "request": [
    {
      "operationName": "CreateEnvGroup",
      "variables": {
        "name": "qa-20261002-group-r1",
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "envVars": [],
        "secretFiles": [],
        "serviceIds": ["srv-davlmi6o5s1c7398lhug"],
        "environmentId": null
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation CreateEnvGroup($name: String!, $ownerId: String, $envVars: [EnvGroupVarInput!], $secretFiles: [EnvGroupSecretFileInput!], $serviceIds: [String!], $environmentId: String) {\n  createEnvGroup(\n    name: $name\n    ownerId: $ownerId\n    envVars: $envVars\n    secretFiles: $secretFiles\n    serviceIds: $serviceIds\n    environmentId: $environmentId\n  ) {\n    id\n    name\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "createEnvGroup": null
      },
      "errors": [
        {
          "message": "linked services must have the same Environment scope as the environment group",
          "locations": [
            {
              "line": 2,
              "column": 3
            }
          ],
          "path": ["createEnvGroup"],
          "extensions": {
            "code": "ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH",
            "serviceEnvironmentId": "env-davlm9oaijhc73cn2qfg",
            "serviceId": "srv-davlmi6o5s1c7398lhug",
            "targetEnvironmentId": ""
          }
        }
      ]
    }
  ],
  "snapshot": "- region \"Notifications alt+T\":\n  - list:\n    - listitem: Couldn't create qa-20261002-group-r1 linked services must have the same Environment scope as the environment group\n- dialog \"Create environment group\":\n  - heading \"Create environment group\" [level=2]\n  - paragraph: Add variables, secret files, and service links now or leave them empty.\n  - text: Group name\n  - textbox \"Group name\":\n    - /placeholder: shared-production\n    - text: qa-20261002-group-r1\n  - heading \"Environment Variables\" [level=3]\n  - paragraph: Set literal values or generate strong secrets during creation.\n  - button \"Add Environment Variable\"\n  - button \"Import from .env\"\n  - heading \"Secret Files\" [level=3]\n  - paragraph: Files are encrypted and mounted under /etc/secrets.\n  - button \"Add Secret File\"\n  - heading \"Linked Services\" [level=3]\n  - paragraph: Selected services receive this group as soon as it is created.\n  - text: Environment\n  - combobox \"Environment\": Workspace (no Environment)\n  - paragraph: No services live in this Environment. Services can only be linked to a group in their own Environment.\n  - button \"Cancel\"\n  - button \"Create Environment Group\"\n  - button \"Close\""
}
```

After cancel/reopen the same mutation with `environmentId:"env-davlm9oaijhc73cn2qfg"` returns HTTP 200:

```json
[
  {
    "data": {
      "createEnvGroup": {
        "__typename": "EnvGroup",
        "id": "evg-davloomo5s1c7398li3g",
        "name": "qa-20261002-group-r1"
      }
    }
  }
]
```

Backend negative control, POST `https://api.bex.co/v1/env-groups`, JSON Content-Type, browser credentials included:

```json
{
  "name": "qa-20261002-rest-mismatch-r1",
  "ownerId": "tea-d98210cbbpdc73dcrkvg",
  "serviceIds": ["srv-davlmi6o5s1c7398lhug"]
}
```

HTTP 409, complete response:

```json
{
  "code": "ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH",
  "error": "linked services must have the same Environment scope as the environment group",
  "id": "conflict",
  "message": "linked services must have the same Environment scope as the environment group",
  "params": {
    "serviceEnvironmentId": "env-davlm9oaijhc73cn2qfg",
    "serviceId": "srv-davlmi6o5s1c7398lhug",
    "targetEnvironmentId": ""
  }
}
```

No group was created by either failed attempt. The later success reused the failed name. List-page controls succeeded for both an explicitly selected QA environment and a workspace-scoped native service.

## Root cause

- `dashboard/src/features/env-groups/components/new-env-group-dialog.tsx:104-105` initializes selected IDs and scope with useState once; default environment is null.
- `dashboard/src/features/env-groups/hooks/use-env-group-scope-index.ts:28-82` loads scope asynchronously. `dashboard/src/features/services/components/env-groups-panel.tsx:76-81` initially computes null, later the actual environment, and keeps the dialog mounted at 220–231.
- Both external service-page controls open through controlled state: panel at `env-groups-panel.tsx:148`, top toolbar at `dashboard/src/routes/services.$serviceId.env.tsx:41-45`.
- The dialog resets in `handleOpenChange` (124–131), which is not invoked when its parent simply changes the controlled open prop. Cancel invokes reset with the now-current scope (114–120); this predicts the successful reopen.
- Candidate filtering at 169–176 hides the preselected service under the stale Workspace scope; submission at 216–217 still sends the retained ID and null environment. The hidden ID in the raw capture is a key part of the mechanism.

The installed dependencies were inspected: Radix Dialog 1.1.15 `dist/index.mjs:24-50` uses controllable state; `@radix-ui/react-use-controllable-state` 1.2.2 `dist/index.mjs:5-46` calls onChange through the setter, not for external controlled-prop changes. React DOM 19.2.3 `react-dom-client.development.js:8264-8297` stores the initial state once. This is caller lifecycle wiring, not a Radix or React defect.

Backend `lego/backend/internal/envgroups/service.go:484-489` validates before allocating/writing, via `prepareCreateServices:554-582` and `validateGroupServiceEnvironment:1277-1289`. Its rejection is correct under the current bex contract.

## Fix, scope and adjacent classes

Initialize scope and selection atomically for each controlled open after the service scope is actually known. Before lookup settles, present a loading/disabled-submit state instead of treating unknown as Workspace. Failed/unauthorized/unavailable lookups need an honest error/retry state; never infer a scope or weaken backend authorization. Once a user edits an open draft, later polling must not reset it. Never submit selected IDs that are hidden because they are incompatible with the chosen scope.

Exhaustive component grep found **two production NewEnvGroupDialog consumers**: `routes/env-groups.tsx:119` (uncontrolled list dialog) and `features/services/components/env-groups-panel.tsx:220` (controlled dialog). The service flow has two external Create group actions. `ServiceEnvPage` is reached through `services.$serviceId.env.tsx` and the `static.$serviceId.env.tsx:13` alias. Web/private/worker/cron share service routing; static has the alias; Postgres/Key Value do not use this component. Apply a consistent dialog lifecycle contract, with regression checks on the currently working uncontrolled/list and workspace branches.

**Unverified this sweep:** static/private/worker/cron live group creation; MCP negative create; injected timeout/authorization cases; polling while a user edits. These belong in t002/t005, not as observed guarantees.

## Dedupe and complete m111 acceptance review

The issue is a regression/incomplete acceptance from [w4/m111](../done/m111/README.md), fix `23b518049`, still present on HEAD. Its full DoD was walked:

1. Workspace list filters: **pass**. The workspace-scoped native control appears; the environment-scoped Docker control does not.
2. In-environment list create: **pass**. Pick QA environment → current service appears → `qa-20261002-group-list-r1` created with correct scope/link.
3. Service-page create: **fails on first controlled open**, passes after cancel/reopen. Workspace service first-open control creates `qa-20261002-group-workspace-r1` successfully.
4. Backend mismatch enforcement: **GraphQL and REST pass** (above). MCP was not live-probed and remains required verification.

No open milestone covers the cold-open lifecycle. No later targeted history fixes it. No anti-goal applies. An empty-group link causing a rollout without a deploy row was separately checked against w2/m94's deliberate design and was not filed.

[Render's environment-variable documentation](https://render.com/docs/configure-environment-variables) describes environment-scoped groups and also permits workspace-level groups to link across environments. The m111 parity task's claim that Render has no environments is incorrect. Bex currently enforces exact scope equality. Preserve that backend rule in this UI fix and explicitly record the parity difference; changing it is separate product work.

## Hunt cleanup

The three created groups, project, environment and both services were deleted; all seven exact-ID REST reads returned 404 and the overview no longer listed them. The Kratos session was revoked and its local cookie files removed. A Kubernetes finalizer residue for the failed Docker fixture is documented separately in [m142](../m142/finding.md); no group API resource remains.
