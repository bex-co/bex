# Built static rollback reads today's publish directory

Why: rollback must restore an earlier published version when the current output-directory setting is broken.

**Severity:** major. Functional hosting QA, pass a79 on 2026-10-10. The action dispatches and chooses the correct old image, but cannot restore its content until the user repairs today's saved publish path.

## Actual journey and controls

Owned service `srv-db588q4b8ulc73efu990`, `qa-20261010-a79-build`, public URL `https://qa-20261010-a79-build.onbex.co`. Public Git `https://github.com/bex-co/bex`, main, root `examples/static-site`, auto-deploy off, no environment inputs or grouping. All seven deploys used source commit `7b95b4608add606804c0a085837b2d146c4dedb8`; differing inline build commands generated the content.

| Time UTC | Action / terminal result | Public result |
| --- | --- | --- |
| 18:31:05 | Create with publish directory dist and build `mkdir -p dist && printf 'qa-a79-v1\\n' > dist/index.html` | First deploy `dep-db588q4b8ulc73efu99g` Live at 18:33:25 |
| 18:33:52 | Read initial serving version | 200, `qa-a79-v1\\n` |
| 18:34:23 | Save `printf 'qa-a79-intentional-build-failure\\n'; exit 1` | `dep-db58abs6flus738pnnm0` build_failed; v1 still 200, service Running |
| 18:36:52 | Save corrected build writing `qa-a79-v2\\n` to dist/index.html | `dep-db58bh4b8ulc73efu9b0` Live; v2 served |
| 18:39:29 | Save publish directory `qa-a79-missing-output` | `dep-db58cocb8ulc73efu9cg` update_failed with missing-directory extraction reason; v2 still 200 |
| 18:41:13 | UI rollback to exact first v1 deploy | HTTP 200; `dep-db58dicb8ulc73efu9eg` selects gen-1 image but update_failed by 18:41:43, same current-directory reason; v2 still 200 |
| 18:42:09 | Restore saved publish directory dist | `dep-db58e04b8ulc73efu9fg` Live; v2 served |
| 18:43:37 | Repeat UI rollback to the same first target | `dep-db58emc6flus738pnnq0` Live at 18:43:55; 200 v1 by 18:43:58 |
| 18:44:37–58 | Delete owned service and log out | Delete true, REST 404, baseline IDs restored, old whoami 401, cookies 0 |

Actual target button: “Roll back to 7b95b46 (dep-db588q4b8ulc73efu99g)”. Dialog says the site will be published again from that deploy, saved settings stay unchanged, and auto-deploy will be turned off. Both rollbacks select the identical gen-1 image digest. The successful control keeps the saved v2 build command, so it did not restore settings or rebuild v1 from the current command.

Expected: restoring v1 is independent of the current publish-directory draft. Actual: the accepted rollback tries to extract `qa-a79-missing-output/.` from the v1 image and fails. The deliberately bad directory's normal config deployment is an expected input failure; the independently requested restore of a previously published target is the finding.

## Durable receipts

The following are owned-fixture UI requests and observed responses, not reconstructed commands. The REST deploy read distinguishes created from finished; the public reads verify content.

```json
{
  "failedRollback": {
    "at": "2026-10-10T18:41:13.814Z",
    "request": [
      {
        "operationName": "RollbackService",
        "variables": {
          "serviceId": "srv-db588q4b8ulc73efu990",
          "deployId": "dep-db588q4b8ulc73efu99g"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation RollbackService($serviceId: String!, $deployId: String!, $confirm: String) {\n  rollbackService(\n    serviceId: $serviceId\n    deployId: $deployId\n    disableAutoDeploy: true\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "rollbackService": {
            "__typename": "Deploy",
            "createdAt": "2026-10-10T18:41:13.673403Z",
            "id": "dep-db58dicb8ulc73efu9eg",
            "image": "zot.bex-registry.svc:5000/tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261010-a79-build:gen-1@sha256:aafd81c61bfe2da69bd42a45b5cce8da7a76709f963a0efcac852b77b45d00ce",
            "rollbackOf": "dep-db588q4b8ulc73efu99g",
            "status": "created",
            "trigger": "rollback"
          }
        }
      }
    ],
    "observedAt": "2026-10-10T18:41:43.579Z",
    "deploysHTTP": 200,
    "deploy": {
      "id": "dep-db58dicb8ulc73efu9eg",
      "serviceId": "srv-db588q4b8ulc73efu990",
      "status": "update_failed",
      "trigger": "rollback",
      "bexTrigger": "rollback",
      "image": {
        "ref": "zot.bex-registry.svc:5000/tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261010-a79-build:gen-1@sha256:aafd81c61bfe2da69bd42a45b5cce8da7a76709f963a0efcac852b77b45d00ce"
      },
      "rollbackOf": "dep-db588q4b8ulc73efu99g",
      "commit": {
        "id": "7b95b4608add606804c0a085837b2d146c4dedb8",
        "message": "docs(pm): file plan-picker loading layout gap (w4/m196)"
      },
      "createdAt": "2026-10-10T18:41:13.673403Z",
      "updatedAt": "2026-10-10T18:41:39.13952Z",
      "startedAt": "2026-10-10T18:41:25.145862Z",
      "finishedAt": "2026-10-10T18:41:39.139519Z",
      "failureReason": "extract: cp: cannot stat 'qa-a79-missing-output/.': No such file or directory (exit 1)"
    },
    "public": {
      "status": 200,
      "body": "qa-a79-v2\n"
    }
  },
  "sameTargetControl": {
    "at": "2026-10-10T18:43:37.311Z",
    "request": [
      {
        "operationName": "RollbackService",
        "variables": {
          "serviceId": "srv-db588q4b8ulc73efu990",
          "deployId": "dep-db588q4b8ulc73efu99g"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation RollbackService($serviceId: String!, $deployId: String!, $confirm: String) {\n  rollbackService(\n    serviceId: $serviceId\n    deployId: $deployId\n    disableAutoDeploy: true\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "rollbackService": {
            "__typename": "Deploy",
            "createdAt": "2026-10-10T18:43:37.220399Z",
            "id": "dep-db58emc6flus738pnnq0",
            "image": "zot.bex-registry.svc:5000/tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261010-a79-build:gen-1@sha256:aafd81c61bfe2da69bd42a45b5cce8da7a76709f963a0efcac852b77b45d00ce",
            "rollbackOf": "dep-db588q4b8ulc73efu99g",
            "status": "created",
            "trigger": "rollback"
          }
        }
      }
    ],
    "observedAt": "2026-10-10T18:43:58.010Z",
    "deploysHTTP": 200,
    "deploy": {
      "id": "dep-db58emc6flus738pnnq0",
      "serviceId": "srv-db588q4b8ulc73efu990",
      "status": "live",
      "trigger": "rollback",
      "bexTrigger": "rollback",
      "image": {
        "ref": "zot.bex-registry.svc:5000/tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261010-a79-build:gen-1@sha256:aafd81c61bfe2da69bd42a45b5cce8da7a76709f963a0efcac852b77b45d00ce"
      },
      "rollbackOf": "dep-db588q4b8ulc73efu99g",
      "commit": {
        "id": "7b95b4608add606804c0a085837b2d146c4dedb8",
        "message": "docs(pm): file plan-picker loading layout gap (w4/m196)"
      },
      "createdAt": "2026-10-10T18:43:37.220399Z",
      "updatedAt": "2026-10-10T18:43:55.311085Z",
      "startedAt": "2026-10-10T18:43:38.963063Z",
      "finishedAt": "2026-10-10T18:43:55.311084Z"
    },
    "public": {
      "status": 200,
      "body": "qa-a79-v1\n"
    },
    "savedSettings": {
      "request": {
        "operationName": "QaA79Final",
        "variables": {
          "id": "srv-db588q4b8ulc73efu990"
        },
        "query": "query QaA79Final($id:String!){server(id:$id){id name type phase runtime buildCommand publishPath revision}}"
      },
      "status": 200,
      "response": {
        "data": {
          "server": {
            "buildCommand": "mkdir -p dist && printf 'qa-a79-v2\\n' > dist/index.html",
            "id": "srv-db588q4b8ulc73efu990",
            "name": "qa-20261010-a79-build",
            "phase": "Running",
            "publishPath": "dist",
            "revision": "rev-7",
            "runtime": "",
            "type": "static_site"
          }
        }
      }
    }
  },
  "cleanup": {
    "delete": {
      "at": "2026-10-10T18:44:37.921Z",
      "request": [
        {
          "operationName": "DeleteService",
          "variables": {
            "id": "srv-db588q4b8ulc73efu990"
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
    "gone": {
      "status": 404,
      "body": "{\"code\":\"NOT_FOUND\",\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\",\"params\":null}\n"
    },
    "baselineVerified": {
      "blueprints": true,
      "databases": true,
      "envGroups": true,
      "keyValues": true,
      "projects": true,
      "services": true
    },
    "logout": {
      "at": "2026-10-10T18:44:58.142Z",
      "oldWhoami": 401,
      "cookies": 0,
      "url": "about:blank"
    }
  }
}
```

## Source trace and proposed fix

- `lego/backend/internal/deploys/service.go:991–1066` validates the target, handles commit-only direct static targets, or passes the resolved image through targetReleaseConfig and openSelectedRelease.
- `lego/backend/internal/deploys/rollback_config.go:44–99` selects the old image and retained runtime configuration without reverting saved App settings. Missing runtime records deliberately fall back to image-only selection.
- `lego/types/v1alpha1/releasesnapshots.go:96–144` records runtime settings, not the published static revision/prefix. Existing ReleaseConfigReference requires a nonempty image; direct static publication cannot be represented by adding a prefix to an otherwise image-empty reference.
- `lego/operator/internal/controller/app_controller.go:2391–2407` sends static sites to reconcileStaticSite before normal selected-runtime projection. `:3674–3707` constructs publisher options from the selected image plus the current App PublishPath. `:3802` is its single production caller.
- `lego/operator/internal/publish/publish.go:443–451` supplies the current PublishPath as PUBLISH_PATH to `cp -a "$PUBLISH_PATH"/.`. This actual container consumer produces the observed error.
- `publish.go:103–152,312–331,390–415` has image/repo modes, validates exactly one of them and a nonempty path, and uploads to a newly requested revision prefix. No retained-prefix mode exists. `identity/identity.go:148–160` distinguishes workspace-scoped and legacy prefixes; `controller/release_identity.go:440` supplies rev-N.
- `app_controller.go:3732–3754,3849–3872` retains old serving content on failure and switches StaticPrefix only after publication succeeds. Preserve these semantics.

Record the successful static publication's actual prefix/revision as immutable target metadata keyed by the App and stored release generation. Bind that metadata to a new static rollback selection without changing saved build/repository/directory settings. Use an appropriate internal static-publication selection shape; the existing image-required runtime reference alone is insufficient.

Extend the existing bounded publishing Job with a retained-prefix-to-new-prefix copy mode. Update Options validation, mode-specific required fields, job identity/idempotency and the observing reconciler together. The existing uploader uses pinned `amazon/aws-cli:2.22.35` with its digest and `s3 sync`; AWS documents S3-to-S3 sync, so this is a supported tool family rather than an assumed new executable. Verify the actual pinned client against the configured object store, preserve object metadata, and explicitly detect unavailable/empty retained artifacts before claiming success. [AWS CLI sync reference](https://docs.aws.amazon.com/cli/latest/reference/s3/sync.html).

For old targets, resolve a historical prefix only where generation and layout make it provable; otherwise refuse the unavailable target with a named reason before creating the rollback. Do not guess from today's path, silently rebuild from today's configuration, change ordinary publishes, or affect non-static image rollback.

## Shared blast radius — separate t003 work

Exhaustive source search found three production calls to Service.Rollback: GraphQL `graphql.go:266`, REST `rest.go:293`, and MCP `mcp.go:182` (`rollback_deploy`). Preserve their existing payloads and dashboard-vs-API auto-deploy behavior. Static Native/image and Direct/no-build branches share reconcileStaticSite and its one publishStaticRevision call. New copy mode must cover both appropriately.

The dashboard's shared DeployActions exposes the canonical `/static/<id>/deploys` list and detail route; the generic service entry canonicalizes to static. Audit both entry points and any existing legacy service aliases rather than inventing another route. Verify exact target names, capability/refusal agreement and completion states.

Allowlist the new publication selection to static sites. Web/private/worker/cron use existing runtime selection; Postgres and Key Value have no App rollback publication. Test those controls and shared type/codegen consumers separately. Direct static, missing/legacy prefixes, absent retained content, and non-GraphQL adapters were not live-probed in this pass.

## Dedupe and completed guarantees

Read .pm/DO_NOT_DO.md and all 59 unfinished milestone READMEs (11 ordinary open, 48 blocked); search the whole board's inbox/completed material for static rollback/publish-directory overlap. No identical open item was found. Recent dashboard/lego commits and targeted rollback-selection history contain no fix bypassing today's static PublishPath; main source matches the observed error, so this is not an already-fixed deployment lag.

**w4/m141** is related, but its original “no mutation was sent” failure does not recur. Walk of all three DoD clauses: (1) current UI dispatches 200 and creates an exact-target rollback, then publication fails — the built-static recovery promise remains incomplete; the original no-build success was not replayed. (2) enabled target and dispatch agree; an independent DeployActions API vector was not captured, so that full clause is unverified. (3) target-naming clause passes: actual button, dialog and rollbackOf identify the selected first deploy. m141 explicitly recorded the image-producing static case as unexercised. Classify this as an original completion gap in that case, not a demonstrated later regression.

**w4/m155** is a serving-state control: failed build and failed publication keep the old public 200 and Running state; update_failed names the extraction cause; repairing the directory succeeds; owned deletion/session cleanup passes. Suspend/resume under failed publication, Kubernetes UID retirement and a post-delete public check were not replayed, so do not claim its entire DoD reverified.

w4/m180 concerns non-static Save-and-deploy artifact reuse, and w4/m160 browser cache revalidation is a different failure. Neither schedules this rollback-selection publication fix.

Render describes rollback as restoring a retained build artifact while saved service settings remain current; static headers/routes also remain current. Applying that retained-artifact principle to the saved publish directory is the inference behind this proposed behavior, not a claim that Render explicitly documents that particular field. [Render rollback documentation](https://render.com/docs/rollbacks).

## Evidence and limits

Local ignored ledger: `.playwright-mcp/qa-build-a79-ledger.json`, final cleanupVerified true. Supporting screenshot `.playwright-mcp/qa-static-rollback-a79-failed.png` exists and was viewed at 1440×1000. Console captured zero errors and warnings before logout. Receipts above are durable in the board; the ignored screenshot is supplemental.

The production object-store prefix itself was not inspected; source establishes the mechanism, and the same-target/image control establishes the directory dependency. No live direct/no-build, legacy-retention, copy-failure or non-static replay is claimed. Early readiness/locator mistakes were excluded, and no paid tier or preexisting resource was changed.
