# Regression: Save and deploy runs a full source build

- **Severity:** major (configuration reaches the service, but the advertised no-build choice runs a new source build).
- **Repro:** create the fixture described in the milestone DoD. Its first Live deployment serves MESSAGE `qa-c1-first-6b2e`. Link owned environment group MESSAGE `qa-c1-group-6b2e`; Edit → delete the own MESSAGE → **Save and deploy** opens `dep-db3hvbkhm7os73dpda0g` and runs `go build` for 31.7 s. It reaches Live with the group's value. Reload `/services/srv-db3hshshm7os73dpd9u0/env`, add MESSAGE `qa-c1-repeat-6b2e`, choose **Save and deploy** again: `dep-db3i1h43a6ns73b87nbg` runs `go build` for 31.6 s and reaches Live (rev-4). The recorded request explicitly says `deploy`, not the rebuild option.
- **Expected:** one configuration release using the serving artifact with newly saved variables/files/group values. **Actual:** the correct new value reaches the process after a full BuildKit source build; the UI/API accept the no-build option without implementing it.
- **Controls:** Restart confirmation created `dep-db3i2uk3a6ns73b87nd0`, selected the prior `gen-4@sha256:4d8ce5e1f4073e22b707be212e8bd502d768e69f5802f4c73951d6940b6359bc` image and retained the same commit; it reached Live in about 24 s. Save-only returned `rolledOut:false`, saved `qa-c1-only-6b2e`, and left public GET serving `qa-c1-repeat-6b2e`. Cancel discarded the edit. Explicit rebuild then opened `dep-db3i484hm7os73dpda50`, ran `go build` for 33.7 s, reached Live/rev-6, and external curl at 04:58:52Z answered HTTP 200 `qa-c1-rebuild-6b2e`.

## Root cause and expressible fix

1. UI `dashboard/src/features/services/components/service-environment-editor.tsx:140` correctly maps deploy to the batch mutation and rebuild to save-only followed by the trigger. `use-environment-draft-save.ts:24` forwards the mode. Apollo 4.1.3 `BatchHttpLink` (configured `factory.client.ts:39`) wraps even a single operation in an array; the capture below is the real batch, not a mistaken trigger request.
2. `lego/backend/internal/secrets/batch.go:291` uses `rollApp`; `:301` removes pending save-only markers and invokes `bumpRestart` for deploy. `secrets/service.go:1084` writes `spec.RestartedAt`. `types/v1alpha1/app_identity.go:89` classifies this as both Artifact and Release.
3. `lego/operator/internal/controller/release_identity.go:125` includes RestartedAt in the artifact fingerprint, including Docker builds. `reusableArtifactImage` at `:377` can reuse an active generation-bound selection, but otherwise returns the prior artifact only if it did not change. The timestamp makes it change. `app_controller.go:792` tries reuse; `:823` dispatches `buildFromSource`; `:996` calls `build.EnsureBuild`. `internal/build/build.go` creates/adopts the real BuildKit Job, matching the actual RUN log below.
4. Existing `ReleaseConfigReference` (`types/v1alpha1/releasesnapshots.go:121`) can express an image-only selection: use the **currently serving immutable image**, generation bound to this new release, with source generation zero so `controller/release_config_snapshot.go:200` uses the newly saved configuration instead of restoring an old snapshot. Positive source generation would deploy the old secrets, an incorrect fix. Existing `deploys/rollback_config.go:71` and `:95` demonstrate generation-bound image selection and Restart's served-image preference.
5. `backend/internal/rollout/rollout.go:231` currently inserts deploy history with `Spec.Image`, empty for a source-built service. Ensure history carries the selected image too: `store/reconciler.go`'s `buildLifecycleFacts` suppresses build facts only for an image-bearing row. Skipping dispatch while keeping an empty row would still fabricate build history. Preserve selected commit provenance.

**Target / scope:** allowlist the deploying **batch** path to an image-only selection of the serving release while keeping repository source intent and applying the new saved config. Do not globally delete RestartedAt from artifact identity: manual deploy/latest commit, clear-cache and Git push need their build semantics; native/buildpack fingerprints also consume environment/build inputs. Lock/recheck the usable live artifact and generation inside the existing conflict-safe mutation. A service with no usable served artifact must be refused before secret writes (or have this choice disabled with a clear reason), rather than silently rebuilding. Static direct-publish/no-OCI policy is a t002 design/verification item, not a claim about an observed static failure.

**Blast radius / aliases:** production `rollApp` has five mutation sites: batch (`batch.go`), files (`files.go`) and three in `service.go`. Tracker consumers span three feature families: apps, secrets and environment groups. `desiredAppReleaseIdentity` has two production callers (settings fingerprint and release decision). REST `PATCH /v1/services/{id}/environment` (`secrets/rest.go:72`), GraphQL `patchServiceEnvironment` (`graphql.go:211`), MCP `patch_service_environment` (`mcp.go:110`) all call `PatchEnvironment`; they move together. Immediate single-var/replace-vars/file CRUD and group writes are sibling contracts to preserve, not aliases of this explicit batch mode. t002 must re-enumerate callers on its implementation HEAD.

**Adjacent classes:** invalid mode/missing usable artifact refusal changes nothing; unauthenticated, forbidden or hidden-resource reads retain existing policy with no existence oracle. Projection/store failure preserves current revision-aware rollback behavior; rollout failure is surfaced without a second source trigger or partial-success lie. Concurrent/superseded/deleted-and-recreated App writes must keep current UID/generation safeguards.

**Unverified:** REST/MCP executing the deploying batch, source-built native/buildpack/private/worker/cron/static, direct static publishing, prebuilt-image saves, file content inside the process, pre-deploy failure, rollback/cancel races and missing-artifact states. Shared code predicts exposure but this run's live failure is the Docker web journey only. Put these into t002/t005 compatibility work.

**Render:** [Render's environment-variable guide](https://render.com/docs/configure-environment-variables) distinguishes saving, deploying and rebuilding. The repository's authenticated capture `docs/render-artifacts/service-environment-page.md` and w5/m44/t006 specify deploy as one rollout of the current image with no build. No authenticated Render replay in this run.

**Dedupe / regression lineage:** searched open/blocked/done across workstreams and current source history after syncing main. No open owner or landed fix. This regresses `w5/done/m44`'s explicit no-build effect. Artifact RestartedAt inclusion came from `b58265e6c` after m44. Earlier `w1/done/m148`'s then-rebuild Restart decision is superseded for Restart by the newer serving-artifact flow; it did not cancel m44's explicit environment choice. Existing `w4/blocked/m156` owns no-build timestamps and is deploy lag; accepted link/unlink convergence at auto-deploy off is recorded in `w2/done/m94` and `w4/done/176.md`, excluded here. No DO_NOT_DO anti-goal applies.

## Original w5/m44 definition of done, clause by clause

| Original guarantee | This hunt | Required compatibility check |
| --- | --- | --- |
| Masked read-only until Edit; one staged editor | Passed on owned web fixture | Retain masking, file/variable shared draft and no implicit reveals |
| Add/update/remove multiple variables and files | One variable add/update/remove passed; multi/file unprobed | Verify mixed sparse patches once, preserve unrelated keys/files |
| Dotenv paste and file import | Unprobed | Replay both with multiline/duplicate/error cases |
| Secret-file upload | Unprobed | Verify staged upload and new file content in served release |
| Generated-secret preview | Unprobed | Preview stays local until explicit save; no secret logging |
| Copy/download fail-closed dotenv export | Unprobed | Both export paths preserve opaque-value and failure policy |
| Discard whole draft | Single-variable Cancel passed | Repeat with combined files/vars/imports |
| Store only; exactly zero rollout | Passed synthetic MESSAGE control | Verify files too, pending change survives reload |
| One rollout without rebuild | **Failed twice on Docker web** | Main target: serving artifact + newly saved config, zero source dispatch |
| One rebuild/deploy, no intermediate rollout | Passed single-variable control | Verify mixed draft and failure/retry orchestration |
| Unchanged masked values survive unfetched/unmodified | Unprobed | Regression test unchanged opaque values and files |
| Complete linked-group create/link, no destructive group delete on service page | Create/link/inheritance/shadowing passed; destructive-control audit partial | Complete create/unlink/relink flow; keep accepted group auto-deploy semantics |
| Desktop and 390px mobile no overflow | Breadcrumb screenshots only, editor mobile unprobed | Compare pending/ready and staged editor at both breakpoints |
| Keyboard/focus/screen-reader names in en/zh | Partial English role-based interaction | Full keyboard flow and both languages |
| Backend/dashboard suites and warning-free local-bex | Not run (filing only) | Implementation gate plus local fixtures; exclude our intentional CORS diagnostic from product warnings |
| Evidence doc + ADR018 describe shipped behavior/drift | Read baseline documents | Update only if final contract/limitations require it |

## Durable request and response

All values are synthetic QA data. No cookie/header/credential material is retained. POST `https://api.bex.co/graphql`, authenticated browser cookies, JSON body. Exact repeat request:

```json
[
  {
    "operationName": "PatchServiceEnvironment",
    "variables": {
      "serviceId": "srv-db3hshshm7os73dpd9u0",
      "envVars": [
        {
          "key": "MESSAGE",
          "value": "qa-c1-repeat-6b2e"
        }
      ],
      "secretFiles": [],
      "saveMode": "deploy"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation PatchServiceEnvironment($serviceId: String!, $envVars: [EnvironmentEnvVarPatchInput!], $secretFiles: [EnvironmentSecretFilePatchInput!], $saveMode: String!) {\n  patchServiceEnvironment(\n    serviceId: $serviceId\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n  ) {\n    envVarKeys\n    secretFileNames\n    rolledOut\n    __typename\n  }\n}"
  }
]
```

HTTP 200, complete response:

```json
[
  {
    "data": {
      "patchServiceEnvironment": {
        "__typename": "EnvironmentPatchResult",
        "envVarKeys": [
          "MESSAGE"
        ],
        "rolledOut": true,
        "secretFileNames": []
      }
    }
  }
]
```

Exact follow-up query and complete response when the repeat reached Live (pageText is separately excerpted below):

```json
{
  "request": {
    "query": "query { server(id:\"srv-db3hshshm7os73dpd9u0\") { id phase revision undeployedChanges } deploys(serviceId:\"srv-db3hshshm7os73dpd9u0\",limit:5) { id status trigger failureReason startedAt finishedAt } }"
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:50:34.336637Z",
          "id": "dep-db3i1h43a6ns73b87nbg",
          "startedAt": "2026-10-08T04:49:22.559402Z",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:46:04.499454Z",
          "id": "dep-db3hvbkhm7os73dpda0g",
          "startedAt": "2026-10-08T04:44:51.508526Z",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:40:51.416347Z",
          "id": "dep-db3hshshm7os73dpd9ug",
          "startedAt": "2026-10-08T04:39:34.626143Z",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "server": {
        "id": "srv-db3hshshm7os73dpd9u0",
        "phase": "Running",
        "revision": "rev-4",
        "undeployedChanges": false
      }
    }
  },
  "observedAt": "2026-10-08T04:51:22.723Z"
}
```

Matching deploy-detail log, from the same ID above:

```text
#10 [build 4/4] RUN CGO_ENABLED=0 go build -trimpath -o /app .
#10 DONE 31.6s
#12 exporting manifest sha256:4d8ce5e1f4073e22b707be212e8bd502d768e69f5802f4c73951d6940b6359bc
2026/10/08 04:50:24 hello-go listening on 3000: "qa-c1-repeat-6b2e"
==> Your service is live
```

Complete REST controls (GET deploy and GET saved MESSAGE):

```json
[
  {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db3hshshm7os73dpd9u0/deploys/dep-db3i2uk3a6ns73b87nd0"
    },
    "status": 200,
    "response": {
      "id": "dep-db3i2uk3a6ns73b87nd0",
      "serviceId": "srv-db3hshshm7os73dpd9u0",
      "status": "live",
      "trigger": "api",
      "bexTrigger": "api",
      "image": {
        "ref": "zot.bex-registry.svc:5000/tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261007-c1-web-6b2e:gen-4@sha256:4d8ce5e1f4073e22b707be212e8bd502d768e69f5802f4c73951d6940b6359bc"
      },
      "commit": {
        "id": "0f312a923ef4230615ad277a26fc5c86c3b42a89",
        "message": "chore(deploy): pin platform images to cd65ad10a064 [skip ci]"
      },
      "createdAt": "2026-10-08T04:52:10.721757Z",
      "updatedAt": "2026-10-08T04:52:34.334288Z",
      "startedAt": "2026-10-08T04:52:21.508165Z",
      "finishedAt": "2026-10-08T04:52:34.334287Z"
    }
  },
  {
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db3hshshm7os73dpd9u0/env-vars/MESSAGE"
    },
    "status": 200,
    "response": {
      "key": "MESSAGE",
      "value": "qa-c1-only-6b2e",
      "revision": "evr1_AAAAAAAAAAQ"
    }
  }
]
```

Complete save-only request/response:

```json
{
  "request": [
    {
      "operationName": "PatchServiceEnvironment",
      "variables": {
        "serviceId": "srv-db3hshshm7os73dpd9u0",
        "envVars": [
          {
            "key": "MESSAGE",
            "value": "qa-c1-only-6b2e"
          }
        ],
        "secretFiles": [],
        "saveMode": "save_only"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation PatchServiceEnvironment($serviceId: String!, $envVars: [EnvironmentEnvVarPatchInput!], $secretFiles: [EnvironmentSecretFilePatchInput!], $saveMode: String!) {\n  patchServiceEnvironment(\n    serviceId: $serviceId\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n  ) {\n    envVarKeys\n    secretFileNames\n    rolledOut\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "patchServiceEnvironment": {
          "__typename": "EnvironmentPatchResult",
          "envVarKeys": [
            "MESSAGE"
          ],
          "rolledOut": false,
          "secretFileNames": []
        }
      }
    }
  ]
}
```

Explicit rebuild control, exact query and complete response:

```json
{
  "observedAt": "2026-10-08T04:58:51.260Z",
  "request": {
    "query": "query { server(id:\"srv-db3hshshm7os73dpd9u0\") { id phase revision undeployedChanges } deploys(serviceId:\"srv-db3hshshm7os73dpd9u0\",limit:6) { id status trigger failureReason startedAt finishedAt } }"
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:56:21.514928Z",
          "id": "dep-db3i484hm7os73dpda50",
          "startedAt": "2026-10-08T04:55:04.265005Z",
          "status": "live",
          "trigger": "api"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:52:34.334287Z",
          "id": "dep-db3i2uk3a6ns73b87nd0",
          "startedAt": "2026-10-08T04:52:21.508165Z",
          "status": "deactivated",
          "trigger": "api"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:50:34.336637Z",
          "id": "dep-db3i1h43a6ns73b87nbg",
          "startedAt": "2026-10-08T04:49:22.559402Z",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:46:04.499454Z",
          "id": "dep-db3hvbkhm7os73dpda0g",
          "startedAt": "2026-10-08T04:44:51.508526Z",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "failureReason": "",
          "finishedAt": "2026-10-08T04:40:51.416347Z",
          "id": "dep-db3hshshm7os73dpd9ug",
          "startedAt": "2026-10-08T04:39:34.626143Z",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "server": {
        "id": "srv-db3hshshm7os73dpd9u0",
        "phase": "Running",
        "revision": "rev-6",
        "undeployedChanges": false
      }
    }
  }
}
```

**Local evidence, verified on disk:** `.playwright-mcp/qa-env-c1-save-rebuild-repeat.png`, `qa-env-c1-repeat-request.json`, `qa-env-c1-repeat-response.json`, `qa-env-c1-repeat-build.json`, `qa-env-c1-restart-control.json`, `qa-env-c1-saveonly-rest.json`, `qa-env-c1-saveonly-wire.json`, `qa-env-c1-rebuild-control.json`, `qa-env-c1-explicit-rebuild-logs.json` (all under `.playwright-mcp/`). Browser console/network captures are `qa-env-c1-console.log` and `qa-env-c1-network.log`; two failed cross-origin public fetch diagnostics were QA probes, and external curl confirmed the public response.

## Sweep and cleanup record

Journeys 1, 2, 3 (Restart), 4 and 16 were exercised this cycle. Remaining deploy actions and hosting families rotate into subsequent infinite-loop sweeps; no paid upgrade or unrelated production resource was changed. All five owned fixtures (service, two groups, environment and project) were deleted through their dashboard confirmations. Their exact REST by-ID reads each returned 404 at 05:05:29Z, the project environment showed zero resources, and the public service URL returned 404. The isolated Kratos session was revoked and its jar removed. Baseline resources were untouched. Cleanup evidence: `.playwright-mcp/qa-c1-cleanup.json` and `.playwright-mcp/qa-loop-c1-ledger.json`.
