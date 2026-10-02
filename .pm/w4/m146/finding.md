# Empty Blueprint environment lists cause repeat deployments

Severity: **major**. An unchanged valid manifest with `envVars: []` repeatedly plans an environment update and creates a real deployment. Live static-site revisions advanced **rev-1 → rev-2 → rev-3** on two identical applies. No outage or data loss was observed; the defect wastes publish/build work and makes the change preview unreliable.

Found by `qa-find-bugs` on 2026-10-02, sweep 6, using `muse.env` without printing its contents. Workspace `bex` (`tea-d98210cbbpdc73dcrkvg`), admin, Scale. QA services were free. The API image matched `deploy/gitops/base/bex.yaml` digest `sha256:7bc6535cde1da49bcdc406781e2b2cfcd9e8a99a8bc8ba212d8418b84ae0951c`, pinned by `2790aaffe` to source `e97ca42273a9`. The faulty helper is identical at that source, local HEAD `b3b85f995`, and the earlier `0064b675f` list-normalization fix. A refreshed origin added `194a2fdc4` (datastore region filtering), which does not fix this.

## Repro and observed controls

1. Submit the manifest below to authenticated `POST https://api.bex.co/v1/blueprints/deploy`, with JSON `{ownerId, bexYaml}`. Do not supply a top-level repository: that would create a separate Blueprint connection. Wait for the free static site to serve HTTP 200 and the initial deploy to become live.
2. Validate the identical manifest through GraphQL `validateBlueprint`, REST multipart `POST /v1/blueprints/validate` (ownerId + file named render.yaml), and MCP `validate_bex_yml`. All report `update` and changed path `envVars`.
3. Reapply the original manifest. Wait for terminal live. Reload the service page and reapply it again. Each apply creates a new deploy on the same service ID, bumps generation/restartedAt and publishes a new revision.
4. Remove just `envVars: []`. The plan becomes `noop`. Applying this control leaves the deploy list at three and the site on rev-3.
5. Without applying, change only `staticPublishPath: .` to `dist` in the omitted-env control: it reports `update` naming only `staticPublishPath`. A new QA name correctly reports `create`.

```yaml
services:
  - name: qa-20261002-static-r6
    type: web
    runtime: static
    repo: https://github.com/bex-co/bex
    branch: main
    rootDir: examples/static-site
    staticPublishPath: .
    autoDeployTrigger: off
    envVars: []
```

Owned service: `srv-davn1dc5o9vs73dt7q9g`; page `https://dashboard.bex.co/static/srv-davn1dc5o9vs73dt7q9g/events`.

| Observation | Result |
| --- | --- |
| Initial deployment | `dep-davn1dc5o9vs73dt7qa0`, live 08:52:25Z, rev-1; generation 1; env absent; restartedAt absent |
| First identical apply, 08:55:17Z | `dep-davn2t45o9vs73dt7qb0`, live 08:55:55Z, rev-2; generation 2; env still absent; restartedAt 08:55:17Z |
| Fresh-page second identical apply, 08:56:20Z | `dep-davn3d6de41s73cant60`, live 08:56:37Z, rev-3; generation 3; env still absent; restartedAt 08:56:20Z |
| Omitted-env apply, 08:57:30Z | Same ID, same three deploys at 08:58:14Z, same rev-3; generation 3 and restartedAt unchanged in cluster read |
| Group-only web manifest | A separately created web service `srv-davmvvude41s73cansu0` with only `fromGroup` also plans service `update/envVars` on an identical re-plan; actual group-only reapply was not tested |

The env-group action's own conservative `update` is expected, separately documented at `blueprint_state_plan.go:31–35,247–253`. It is not the finding.

## Mechanism and concrete fix

- `lego/backend/internal/apps/blueprint_plan.go:247–249` applies every declared service envVars block via `mergeBlueprintEnv`.
- `mergeBlueprintEnv` at **:322–342**, specifically **:327**, always allocates a non-nil result, even when current and declared environment lists are empty. `ApplyBlueprintServiceSpec` at **:316** compares the before/after specs with `reflect.DeepEqual`.
- `lego/types/v1alpha1/app_types.go:553` declares `Env []EnvVar` with `json:"env,omitempty"`. The live CR has no env field before and after each apply. The declared empty list and group-only declaration both yield nil literals in `classifyServiceEnv` (`deploy.go:2322–2373`); the merge introduces the different in-memory representation.
- Dependency code was opened: Go 1.26.8 `src/reflect/deepequal.go:98–101` explicitly rejects differing slice nilness; `src/encoding/json/encode.go:351–354,735–738` omits zero-length slices. The repository container uses the Go 1.26 line pinned by digest; the local workspace defaults to Go 1.27, so the 1.26 source was checked separately. Controller-runtime **v0.23.3** `pkg/client/patch.go:119–138` marshals original and modified objects before constructing the patch.
- The false spec change reaches `deploy.go:2741`, bypasses the idempotent return at **:2777**, and reaches `patchChangedStackService`. Its second DeepEqual at **:2876** also treats nil/empty as different; **:2906** creates a deployment and **:2916** writes restartedAt before the merge patch at **:2923**. Thus the empty env difference itself disappears on the wire but the restart timestamp is a real mutation. `recordBlueprintRedeploy:2937–2945` records the next generation.
- The planner consumes the same applier at `blueprint_state_plan.go:197,206`; it correctly reports the helper's false result as an envVars update. The UI consumer `dashboard/src/features/blueprints/components/blueprint-plan-summary.tsx:128` counts non-noop operations and renders the provided changed paths, so a frontend suppression would leave the actual redeploy defect intact.
- The omitted-field control never invokes the envVars applier (`blueprint_plan.go:286–290`) and reaches the early return in apply. The genuine-path-change control confirms the planner still distinguishes real changes.

**Target:** canonicalize a semantically empty merged env list to nil at the shared merge boundary, preserving nonempty values, upsert semantics, stable ordering and source ownership. Reuse the existing empty-list convention (`canonicalSlice`, **blueprint_plan.go:106–123**) without adding an unnecessary second clone of an already-built result. Then unchanged empty/group-only declarations report service noop with no changed paths and apply creates zero deploys, changes no generation/restartedAt, and leaves the serving revision untouched. Do not suppress all envVars differences, erase undeclared values, or change the conservative env-group plan.

## Callers, aliases and adjacent behavior

Exhaustive production Go search (excluding \*\_test.go):

- `mergeBlueprintEnv`: **1 caller**, envVars field applier.
- `ApplyBlueprintServiceSpec`: **3 call sites** — the main plan probe and per-field probe (`blueprint_state_plan.go:197,206`), plus `applyBlueprintServiceSpec` wrapper (`deploy.go:3015`).
- The wrapper has **1 caller**, `applyCreateWithFields:2741`. Compiled `applyBlueprintCreate` has **2 callers**, normal service pass **:955** and deferred-reference second pass **:1001**. The fields-nil legacy path uses `applyCreateToSpec`, not this merge helper.
- `blueprintActionPlan` has **5 callers**: direct stack preflight (`deploy.go:634`), validation (`blueprint.go:692`), create preflight (**:960**), sync preparation (**:1155**) and sync fallback (**:1331**).
- Validation aliases: REST multipart `POST /v1/blueprints/validate`, GraphQL `validateBlueprint`, MCP `validate_bex_yml`. Repository preview aliases: REST `POST /v1/blueprints/preview`, GraphQL `blueprintPreview`, MCP `preview_blueprint`, via `PreviewBlueprint → blueprintValidationFor`.
- Apply aliases: REST `POST /v1/blueprints/deploy` and MCP `deploy`; connected create through REST `POST /v1/blueprints`, GraphQL `createBlueprint`, MCP `create_blueprint`; sync through REST `POST /v1/blueprints/{id}/sync`, GraphQL `syncBlueprint`, MCP `sync_blueprint`. Git auto-sync enters the shared admitted-sync apply path. `render.yaml` and legacy filename `bex.yml` use the same grammar.
- UI plan summary has **2 production JSX consumers**: `routes/blueprints.new.tsx:369` and `routes/blueprints.$blueprintId.tsx:642`. Valid Git-file create and missing-file previews were exercised; a Git-hosted fixture containing this exact empty-env case was not.
- Type family: web, static, cron, worker and private share the service applier; only static actual repeat applies and web group-only planning were observed. Postgres and Key Value have separate appliers, with canonicalSlice already used for their list fields (**:395,398,423**). Env groups retain their own conservative policy. No claim of live regression on the unprobed siblings.
- Global fix at the shared empty-env merge; no service-type allowlist. Add controls for nonempty literals, undeclared mutable keys, secret references, fromGroup-only, sync:false/generateValue-only and deferred fromService/fromDatabase sources. The latter forms may have additional planning constraints; do not assume all are demonstrated by this empty result.
- Validation/authz/payment/protected-environment refusal, missing repository, timeout, and create vs existing matching retain their existing meanings. No permission broadening, value disclosure or changed error taxonomy is proposed.

## Dedupe and earlier guarantees

The full board (open, blocked and done) was searched for `mergeBlueprintEnv`, empty envVars, nil/empty and Blueprint idempotence. All 57 open/blocked/workstream/dev README files were scanned; matching open milestones concern unrelated HA, datastore placement, allowlists, local stacks, environment CAS, CLI, native Settings deployment and mobile events. No open item covers this merge defect. `DO_NOT_DO.md` and prior hunt precedents w9/m89 and w9/m92 were reviewed. No anti-goal applies. `git log -40 -- dashboard lego` and targeted helper history show no pending fix.

This is an **uncovered empty-env failure of the existing idempotent-apply guarantee**, not evidence that the earlier domains/IP-allowlist corrections reverted. The faulty allocation existed before and after `0064b675f`. The previous broad “every list applier” closure missed this helper. Preserve the complete earlier guarantees:

| Previous milestone / entire DoD | This sweep's disposition |
| --- | --- |
| w1/done/m24 — multi-service + DB convergence; upfront all-or-nothing validation; DB-before-dependent ordering; working secretRef-backed fromDatabase; repeat deploy no-op; parity ledger | Repeat no-op is disproved for empty-env static services. No DB/worker stack, invalid mixed-stack atomicity, DB ordering, DB query, or secretRef disclosure probe was run here; retain their existing tests and explicitly recheck them in the shared-callers task. Parity ledger review belongs to the closing task. |
| w6/done/m125 — existing is not create | Pass for the two created services; static correctly resolves the same ID as update/noop. |
| w6/m125 — changed plan differs | Pass: omitted-env control noop vs staticPublishPath-only update. |
| w6/m125 — changed apply preserves ID and writes changed command | Not exercised as that acceptance flow; repeat empty-env apply preserves ID but incorrectly adds a deployment. Keep the original changed-command regression. |
| w6/m125 — new service create | Pass, initial creates and plan-only new QA name. |
| w6/m125 — DB/KV plans and duplicate-name conflicts | Not re-probed; retain tests. |
| w4/done/m124 — export web/DB/KV → noop | Not re-probed. Export of a service without env omits the envVars field (`blueprint_generate.go:344–350`), explaining why that guard misses an explicitly empty block. |
| w4/m124 — domain-carrying stability | Not re-probed; preserve both Host/Hosts layouts. |
| w4/m124 — KV empty ipAllowList stability | Not re-probed; its canonicalSlice implementation remains present. |
| w4/m124 — exact changedFields and real changes still detected | Pass on the staticPublishPath control. Existing empty env still falsely contributes envVars; other resource field variations not re-probed. |
| w4/m124 — exportable-kind round-trip guard | Existing `TestGeneratedBlueprintRePlansAsNoop` checks export → plan, not explicit-empty apply → serialized read → plan. Extend at the appropriate apply boundary; preserve its seven service/datastore fixtures and documented env-group conservative exemption. |
| w4/done/m138 — sync history refresh | Not re-probed; no Git connection created here. |
| w4/m138 — unchanged dialog says no change | Exact empty-env Git fixture not probed; API gives the consumer a false update. Preserve the existing passing dialog case without envVars. |
| w4/m138 — create/update distinction | UI create preview passed; changed-field API control passed. Empty-env UI update not directly captured. |
| w4/m138 — reviewed source copy | Not re-probed; retain commit-pinning tests. |

The earlier m124 outcome called spurious updates merely noisy; this live apply experiment establishes concrete deployment side effects for this case.

## Render and unverified scope

Render documents applying changes to affected services and retaining environment variables not overwritten by a Blueprint: [Blueprint lifecycle](https://render.com/docs/infrastructure-as-code), [YAML reference](https://render.com/docs/blueprint-spec), checked 2026-10-02. No Render account apply was executed. Bex's explicit unchanged-apply contract is also in MCP `deploy` documentation (`mcp.go:782`) and `deploy.go:2704–2708`; preserve that contract.

Unverified: live worker/private/cron repeats, DB/KV controls, Git create/sync/auto-sync for this manifest, actual group-only reapply, sync:false/generateValue/deferred references, protected-environment effects and generated exports with group refs. These are test work, not additional observed bugs. No application fixes or regression tests were implemented in this QA filing.

## Evidence and cleanup

Local screenshot `.playwright-mcp/qa-r6-static-unchanged-rev3.png` shows the rev-3 header after both redundant applies. Its event feed still shows the first redundant deployment; use the complete API captures for the three-row deployment history. Complete request/response records below are durable; the ignored screenshot is supplementary. Broader local captures: `.playwright-mcp/qa-r6-evidence-progress.json`, `.playwright-mcp/qa-muse-r6-ledger.json`.

Cleanup completed before shipping: deleted web `srv-davmvvude41s73cansu0` at 09:05:19Z, static `srv-davn1dc5o9vs73dt7q9g` at 09:05:31Z and group `evg-davmvu6de41s73canstg` at 09:06:16Z through their dashboard confirmation dialogs. At 09:06:17.726Z, authenticated GET of `/v1/services/<each-service-id>` and `/v1/env-groups/<group-id>` returned HTTP 404 with `{"error":"not found","id":"not_found","message":"not found"}` for each. The cluster scan after service deletion found no matching App, Deployment, Pod, Job, Service, Ingress, Secret or PVC. This run's Kratos logout returned `ok logged-out`; its cookie jar/storage state were removed. Final local API/UI evidence: `.playwright-mcp/qa-r6-evidence-final.json`. No QA resources remain from this sweep.

## Complete request/response captures

All GraphQL captures used authenticated POST `https://api.bex.co/graphql` with JSON content type; apply captures used POST `https://api.bex.co/v1/blueprints/deploy`. MCP used POST `https://api.bex.co/mcp`, JSON content type and Accept `application/json, text/event-stream`. Cookies are intentionally excluded. Each response below is complete for the exact captured request, not an invented projection. The initial REST validation JSON experiment returned a schema 400 because Render requires multipart; it was a harness input error and was corrected before the REST captures below.

### Capture 1: Initial creation (2026-10-02T08:52:06.224Z)

```json
{
  "at": "2026-10-02T08:52:06.224Z",
  "label": "Initial creation",
  "request": {
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "bexYaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n"
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "name": "qa-20261002-static-r6",
        "slug": "qa-20261002-static-r6",
        "displayName": "",
        "type": "static_site",
        "phase": "",
        "undeployedChanges": false,
        "url": "https://qa-20261002-static-r6.onbex.co",
        "urls": null,
        "image": "",
        "builder": "auto",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "",
        "createdAt": "2026-10-02T08:52:06Z",
        "updatedAt": "2026-10-02T08:52:06.10893327Z",
        "dashboardUrl": "https://dashboard.bex.co/static/srv-davn1dc5o9vs73dt7q9g",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/static-site",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "publishPath": ".",
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        },
        "latestDeployId": "dep-davn1dc5o9vs73dt7qa0"
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": null
  }
}
```

### Capture 2: static-and-web-status (2026-10-02T08:54:54.343Z)

```json
{
  "at": "2026-10-02T08:54:54.343Z",
  "label": "static-and-web-status",
  "request": {
    "query": "query{static:service(id:\"srv-davn1dc5o9vs73dt7q9g\"){id phase revision} staticDeploys:deploys(serviceId:\"srv-davn1dc5o9vs73dt7q9g\",limit:20){id status startedAt finishedAt failureReason} web:service(id:\"srv-davmvvude41s73cansu0\"){id phase revision} webDeploys:deploys(serviceId:\"srv-davmvvude41s73cansu0\",limit:5){id status startedAt finishedAt failureReason}}"
  },
  "status": 200,
  "response": {
    "data": {
      "static": {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "phase": "Running",
        "revision": "rev-1"
      },
      "staticDeploys": [
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:52:25.967761Z",
          "id": "dep-davn1dc5o9vs73dt7qa0",
          "startedAt": "2026-10-02T08:52:07.782846Z",
          "status": "live"
        }
      ],
      "web": {
        "id": "srv-davmvvude41s73cansu0",
        "phase": "Building",
        "revision": ""
      },
      "webDeploys": [
        {
          "failureReason": "",
          "finishedAt": "",
          "id": "dep-davmvvude41s73cansug",
          "startedAt": "2026-10-02T08:54:26.012607Z",
          "status": "build_in_progress"
        }
      ]
    }
  }
}
```

### Capture 3: unchanged-empty-env (2026-10-02T08:54:54.343Z)

```json
{
  "at": "2026-10-02T08:54:54.343Z",
  "label": "unchanged-empty-env",
  "request": {
    "query": "query($yaml:String!,$owner:String!){validateBlueprint(bexYaml:$yaml,ownerId:$owner){valid errors plan{mode totalActions actions{operation kind name resourceId changedFields{path}}}}}",
    "variables": {
      "yaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n",
      "owner": "tea-d98210cbbpdc73dcrkvg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "validateBlueprint": {
        "errors": [],
        "plan": {
          "actions": [
            {
              "changedFields": [
                {
                  "path": "envVars"
                }
              ],
              "kind": "service",
              "name": "qa-20261002-static-r6",
              "operation": "update",
              "resourceId": "srv-davn1dc5o9vs73dt7q9g"
            }
          ],
          "mode": "current_state",
          "totalActions": 1
        },
        "valid": true
      }
    }
  }
}
```

### Capture 4: omitted-env (2026-10-02T08:54:54.343Z)

```json
{
  "at": "2026-10-02T08:54:54.343Z",
  "label": "omitted-env",
  "request": {
    "query": "query($yaml:String!,$owner:String!){validateBlueprint(bexYaml:$yaml,ownerId:$owner){valid errors plan{mode totalActions actions{operation kind name resourceId changedFields{path}}}}}",
    "variables": {
      "yaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n",
      "owner": "tea-d98210cbbpdc73dcrkvg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "validateBlueprint": {
        "errors": [],
        "plan": {
          "actions": [
            {
              "changedFields": [],
              "kind": "service",
              "name": "qa-20261002-static-r6",
              "operation": "noop",
              "resourceId": "srv-davn1dc5o9vs73dt7q9g"
            }
          ],
          "mode": "current_state",
          "totalActions": 1
        },
        "valid": true
      }
    }
  }
}
```

### Capture 5: identical-empty-env-reapply-1 (2026-10-02T08:55:17.273Z)

```json
{
  "at": "2026-10-02T08:55:17.273Z",
  "label": "identical-empty-env-reapply-1",
  "request": {
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "bexYaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n"
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "name": "qa-20261002-static-r6",
        "slug": "qa-20261002-static-r6",
        "displayName": "",
        "type": "static_site",
        "phase": "Running",
        "undeployedChanges": false,
        "url": "https://qa-20261002-static-r6.onbex.co",
        "urls": ["https://qa-20261002-static-r6.onbex.co"],
        "image": "",
        "builder": "auto",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "rev-1",
        "createdAt": "2026-10-02T08:52:06Z",
        "updatedAt": "2026-10-02T08:55:17.178987395Z",
        "dashboardUrl": "https://dashboard.bex.co/static/srv-davn1dc5o9vs73dt7q9g",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/static-site",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "publishPath": ".",
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        }
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": null
  }
}
```

### Capture 6: fresh-page-identical-empty-env-reapply-2 (2026-10-02T08:56:20.785Z)

```json
{
  "at": "2026-10-02T08:56:20.785Z",
  "label": "fresh-page-identical-empty-env-reapply-2",
  "before": {
    "request": {
      "query": "query{service(id:\"srv-davn1dc5o9vs73dt7q9g\"){id phase revision} deploys(serviceId:\"srv-davn1dc5o9vs73dt7q9g\",limit:20){id status startedAt finishedAt failureReason}}"
    },
    "status": 200,
    "response": {
      "data": {
        "deploys": [
          {
            "failureReason": "",
            "finishedAt": "2026-10-02T08:55:55.928417Z",
            "id": "dep-davn2t45o9vs73dt7qb0",
            "startedAt": "2026-10-02T08:55:26.333546Z",
            "status": "live"
          },
          {
            "failureReason": "",
            "finishedAt": "2026-10-02T08:52:25.967761Z",
            "id": "dep-davn1dc5o9vs73dt7qa0",
            "startedAt": "2026-10-02T08:52:07.782846Z",
            "status": "deactivated"
          }
        ],
        "service": {
          "id": "srv-davn1dc5o9vs73dt7q9g",
          "phase": "Running",
          "revision": "rev-2"
        }
      }
    }
  },
  "request": {
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "bexYaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n"
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "name": "qa-20261002-static-r6",
        "slug": "qa-20261002-static-r6",
        "displayName": "",
        "type": "static_site",
        "phase": "Running",
        "undeployedChanges": false,
        "url": "https://qa-20261002-static-r6.onbex.co",
        "urls": ["https://qa-20261002-static-r6.onbex.co"],
        "image": "",
        "builder": "auto",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "rev-2",
        "createdAt": "2026-10-02T08:52:06Z",
        "updatedAt": "2026-10-02T08:56:20.69953218Z",
        "dashboardUrl": "https://dashboard.bex.co/static/srv-davn1dc5o9vs73dt7q9g",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/static-site",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "publishPath": ".",
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        }
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": null
  }
}
```

### Capture 7: MCP validate (2026-10-02T08:57:09.198Z)

```json
{
  "at": "2026-10-02T08:57:09.198Z",
  "surface": "MCP validate",
  "request": {
    "jsonrpc": "2.0",
    "id": 6,
    "method": "tools/call",
    "params": {
      "name": "validate_bex_yml",
      "arguments": {
        "bexYaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n"
      }
    }
  },
  "status": 200,
  "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":6,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"estimatedPricing\\\":{\\\"lines\\\":[],\\\"totalUsd\\\":\\\"0.00\\\",\\\"variable\\\":[]},\\\"plan\\\":{\\\"actions\\\":[{\\\"changedFields\\\":[{\\\"path\\\":\\\"envVars\\\"}],\\\"kind\\\":\\\"service\\\",\\\"name\\\":\\\"qa-20261002-static-r6\\\",\\\"operation\\\":\\\"update\\\",\\\"resourceId\\\":\\\"srv-davn1dc5o9vs73dt7q9g\\\",\\\"sourcePath\\\":\\\"#/services/0\\\"}],\\\"mode\\\":\\\"current_state\\\",\\\"services\\\":[\\\"qa-20261002-static-r6\\\"],\\\"totalActions\\\":1},\\\"valid\\\":true}\"}],\"structuredContent\":{\"estimatedPricing\":{\"lines\":[],\"totalUsd\":\"0.00\",\"variable\":[]},\"plan\":{\"actions\":[{\"changedFields\":[{\"path\":\"envVars\"}],\"kind\":\"service\",\"name\":\"qa-20261002-static-r6\",\"operation\":\"update\",\"resourceId\":\"srv-davn1dc5o9vs73dt7q9g\",\"sourcePath\":\"#/services/0\"}],\"mode\":\"current_state\",\"services\":[\"qa-20261002-static-r6\"],\"totalActions\":1},\"valid\":true}}}\n\n"
}
```

### Capture 8: GQL status after second identical apply (2026-10-02T08:57:09.198Z)

```json
{
  "at": "2026-10-02T08:57:09.198Z",
  "surface": "GQL status after second identical apply",
  "request": {
    "query": "query{service(id:\"srv-davn1dc5o9vs73dt7q9g\"){id phase revision} deploys(serviceId:\"srv-davn1dc5o9vs73dt7q9g\",limit:20){id status startedAt finishedAt failureReason} web:service(id:\"srv-davmvvude41s73cansu0\"){id phase revision} webDeploys:deploys(serviceId:\"srv-davmvvude41s73cansu0\",limit:5){id status startedAt finishedAt failureReason}}"
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:56:37.501088Z",
          "id": "dep-davn3d6de41s73cant60",
          "startedAt": "2026-10-02T08:56:25.957056Z",
          "status": "live"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:55:55.928417Z",
          "id": "dep-davn2t45o9vs73dt7qb0",
          "startedAt": "2026-10-02T08:55:26.333546Z",
          "status": "deactivated"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:52:25.967761Z",
          "id": "dep-davn1dc5o9vs73dt7qa0",
          "startedAt": "2026-10-02T08:52:07.782846Z",
          "status": "deactivated"
        }
      ],
      "service": {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "phase": "Running",
        "revision": "rev-3"
      },
      "web": {
        "id": "srv-davmvvude41s73cansu0",
        "phase": "Building",
        "revision": ""
      },
      "webDeploys": [
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:55:37.492486Z",
          "id": "dep-davmvvude41s73cansug",
          "startedAt": "2026-10-02T08:54:26.012607Z",
          "status": "canceled"
        }
      ]
    }
  }
}
```

### Capture 9: empty-env (2026-10-02T08:57:30.435Z)

```json
{
  "at": "2026-10-02T08:57:30.435Z",
  "label": "empty-env",
  "request": {
    "method": "POST",
    "url": "https://api.bex.co/v1/blueprints/validate",
    "contentType": "multipart/form-data",
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "file": {
      "name": "render.yaml",
      "content": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n"
    }
  },
  "status": 200,
  "response": {
    "valid": true,
    "plan": {
      "mode": "current_state",
      "services": ["qa-20261002-static-r6"],
      "totalActions": 1,
      "actions": [
        {
          "operation": "update",
          "kind": "service",
          "name": "qa-20261002-static-r6",
          "sourcePath": "#/services/0",
          "resourceId": "srv-davn1dc5o9vs73dt7q9g",
          "changedFields": [
            {
              "path": "envVars"
            }
          ]
        }
      ]
    },
    "estimatedPricing": {
      "totalUsd": "0.00",
      "lines": [],
      "variable": []
    }
  }
}
```

### Capture 10: omitted-env (2026-10-02T08:57:30.435Z)

```json
{
  "at": "2026-10-02T08:57:30.435Z",
  "label": "omitted-env",
  "request": {
    "method": "POST",
    "url": "https://api.bex.co/v1/blueprints/validate",
    "contentType": "multipart/form-data",
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "file": {
      "name": "render.yaml",
      "content": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n"
    }
  },
  "status": 200,
  "response": {
    "valid": true,
    "plan": {
      "mode": "current_state",
      "services": ["qa-20261002-static-r6"],
      "totalActions": 1,
      "actions": [
        {
          "operation": "noop",
          "kind": "service",
          "name": "qa-20261002-static-r6",
          "sourcePath": "#/services/0",
          "resourceId": "srv-davn1dc5o9vs73dt7q9g"
        }
      ]
    },
    "estimatedPricing": {
      "totalUsd": "0.00",
      "lines": [],
      "variable": []
    }
  }
}
```

### Capture 11: omitted-env-apply-control (2026-10-02T08:57:30.435Z)

```json
{
  "at": "2026-10-02T08:57:30.435Z",
  "label": "omitted-env-apply-control",
  "request": {
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "bexYaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n"
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "name": "qa-20261002-static-r6",
        "slug": "qa-20261002-static-r6",
        "displayName": "",
        "type": "static_site",
        "phase": "Running",
        "undeployedChanges": false,
        "url": "https://qa-20261002-static-r6.onbex.co",
        "urls": ["https://qa-20261002-static-r6.onbex.co"],
        "image": "",
        "builder": "auto",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "rev-3",
        "createdAt": "2026-10-02T08:52:06Z",
        "updatedAt": "2026-10-02T08:56:36Z",
        "dashboardUrl": "https://dashboard.bex.co/static/srv-davn1dc5o9vs73dt7q9g",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/static-site",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "publishPath": ".",
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        }
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": null
  }
}
```

### Capture 12: real-path-change-control (2026-10-02T08:58:14.465Z)

```json
{
  "at": "2026-10-02T08:58:14.465Z",
  "label": "real-path-change-control",
  "request": {
    "query": "query($yaml:String!,$owner:String!){validateBlueprint(bexYaml:$yaml,ownerId:$owner){valid errors plan{mode totalActions actions{operation kind name resourceId changedFields{path}}}}}",
    "variables": {
      "yaml": "services:\n  - name: qa-20261002-static-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: dist\n    autoDeployTrigger: off\n",
      "owner": "tea-d98210cbbpdc73dcrkvg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "validateBlueprint": {
        "errors": [],
        "plan": {
          "actions": [
            {
              "changedFields": [
                {
                  "path": "staticPublishPath"
                }
              ],
              "kind": "service",
              "name": "qa-20261002-static-r6",
              "operation": "update",
              "resourceId": "srv-davn1dc5o9vs73dt7q9g"
            }
          ],
          "mode": "current_state",
          "totalActions": 1
        },
        "valid": true
      }
    }
  }
}
```

### Capture 13: genuinely-new-name-control (2026-10-02T08:58:14.465Z)

```json
{
  "at": "2026-10-02T08:58:14.465Z",
  "label": "genuinely-new-name-control",
  "request": {
    "query": "query($yaml:String!,$owner:String!){validateBlueprint(bexYaml:$yaml,ownerId:$owner){valid errors plan{mode totalActions actions{operation kind name resourceId changedFields{path}}}}}",
    "variables": {
      "yaml": "services:\n  - name: qa-20261002-plan-only-r6\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/static-site\n    staticPublishPath: .\n    autoDeployTrigger: off\n    envVars: []\n",
      "owner": "tea-d98210cbbpdc73dcrkvg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "validateBlueprint": {
        "errors": [],
        "plan": {
          "actions": [
            {
              "changedFields": [],
              "kind": "service",
              "name": "qa-20261002-plan-only-r6",
              "operation": "create",
              "resourceId": ""
            }
          ],
          "mode": "current_state",
          "totalActions": 1
        },
        "valid": true
      }
    }
  }
}
```

### Capture 14: after-omitted-env-apply-control (2026-10-02T08:58:14.465Z)

```json
{
  "at": "2026-10-02T08:58:14.465Z",
  "label": "after-omitted-env-apply-control",
  "request": {
    "query": "query{service(id:\"srv-davn1dc5o9vs73dt7q9g\"){id phase revision} deploys(serviceId:\"srv-davn1dc5o9vs73dt7q9g\",limit:20){id status startedAt finishedAt failureReason}}"
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:56:37.501088Z",
          "id": "dep-davn3d6de41s73cant60",
          "startedAt": "2026-10-02T08:56:25.957056Z",
          "status": "live"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:55:55.928417Z",
          "id": "dep-davn2t45o9vs73dt7qb0",
          "startedAt": "2026-10-02T08:55:26.333546Z",
          "status": "deactivated"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-02T08:52:25.967761Z",
          "id": "dep-davn1dc5o9vs73dt7qa0",
          "startedAt": "2026-10-02T08:52:07.782846Z",
          "status": "deactivated"
        }
      ],
      "service": {
        "id": "srv-davn1dc5o9vs73dt7q9g",
        "phase": "Running",
        "revision": "rev-3"
      }
    }
  }
}
```

### Capture 15: fromGroup re-plan (2026-10-02T08:49:25.640Z)

```json
{
  "at": "2026-10-02T08:49:25.640Z",
  "request": {
    "query": "query($yaml:String!,$owner:String!){validateBlueprint(bexYaml:$yaml,ownerId:$owner){valid errors errorDetails{line column path error} plan{mode totalActions actions{operation kind name resourceId changedFields{path} message}}}}",
    "variables": {
      "yaml": "envVarGroups:\n  - name: qa-20261002-group-r6\n    envVars:\n      - key: MESSAGE\n        value: qa-20261002-blueprint-v1\nservices:\n  - name: qa-20261002-web-r6\n    type: web\n    runtime: go\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/hello-go\n    plan: free\n    buildCommand: go build -o app .\n    startCommand: ./app\n    autoDeployTrigger: off\n    healthCheckPath: /\n    envVars:\n      - fromGroup: qa-20261002-group-r6\n",
      "owner": "tea-d98210cbbpdc73dcrkvg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "validateBlueprint": {
        "errorDetails": [],
        "errors": [],
        "plan": {
          "actions": [
            {
              "changedFields": [
                {
                  "path": "envVars"
                },
                {
                  "path": "name"
                }
              ],
              "kind": "env_var_group",
              "message": "",
              "name": "qa-20261002-group-r6",
              "operation": "update",
              "resourceId": "evg-davmvu6de41s73canstg"
            },
            {
              "changedFields": [
                {
                  "path": "envVars"
                }
              ],
              "kind": "service",
              "message": "",
              "name": "qa-20261002-web-r6",
              "operation": "update",
              "resourceId": "srv-davmvvude41s73cansu0"
            }
          ],
          "mode": "current_state",
          "totalActions": 2
        },
        "valid": true
      }
    }
  }
}
```
