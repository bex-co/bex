# A native-to-image source switch saves a Docker Command that never runs

Why: the supported source migration accepts a valid command but the operator launches the image default, leaving the service unavailable.

**Severity:** major. **Worker:** worker4. **Source:** functional live QA a311, 2026-10-12 Pacific / 2026-10-13 UTC. Production `https://dashboard.bex.co`, normal QA workspace `bex` / `tea-d98210cbbpdc73dcrkvg`. Diagnosis HEAD `624669b0349173ab035e58dd468d4dcdd56afb61`; deployed operator/backend revision was not read. No cybersecurity work or credential probes.

## Reproduction and control

1. Create Free web service `qa-20261012-a311-recovery` from public `https://github.com/bex-co/bex`, branch main, root `examples/hello-node`, runtime Node, port 3000, auto-deploy off, and MESSAGE=`qa-a311-recovered`. The intentionally failing build command was `printf 'qa-a311-intentional-build-failure\\n'; exit 7`. **That command never executed:** the actual build log reported waiting for a slot with 2/2 workspace builds running. The owned initial deploy was canceled before switching source. A queued build is not this finding.
2. On `/services/srv-db6ngh95i38c73fd5ob0/settings`, Source → Edit → Existing Image → `docker.io/library/busybox:1.37` → Update source succeeds. It clears repo/branch and stages the image without a deploy. The Docker Command editor initially retains `node main.js`.
3. Replace that command with the exact harmless command below and Save changes, including its rollout confirmation. Actual SetStartCommand succeeds at **00:21:27.781 UTC**; its carrying `config_change` deploy is `dep-db6nj1h2s0sc73a8hkp0`.
4. The image is selected, but the release remains `update_in_progress`, service Deploying, no revision, and its public URL returns **503 / service unavailable**. The later durable stall reason reports repeated container exits with **last exit code 0**. There is no application log output in the bounded first-release query.
5. Fresh-load at 390×844, cancel that release, and choose Manual Deploy → Deploy latest image. Actual TriggerDeploy succeeds at **00:24:50.432 UTC**, producing `dep-db6nkk92s0sc73a8hkr0`. This independent release reproduces the same exit-0 loop and HTTP 503. A fresh `/web/...` alias resolves to the canonical detail route; the settled mobile page still shows the stall at **00:27:45.448 UTC**.
6. Independently create Free Existing Image web control `qa-20261012-a311-control` / `srv-db6nksh5i38c73fd5ogg` with **the identical image, command, port 3000 and MESSAGE value**. Its first deploy `dep-db6nksh5i38c73fd5oh0` becomes **Live at 00:25:34.495886 UTC**, service Running/rev-1. At **00:26:21.108 UTC** the switched fixture returns 503 and the control returns **200 / `qa-a311-recovered\\n`**.

Exact command:

```sh
sh -c 'mkdir -p /tmp/qa-a311-www; printf "qa-a311-recovered\n" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'
```

The readbacks retain `runtime: node`, `builder: native`, `rootDir: examples/hello-node` on the switched image, with `repo: ""`. The successful control has `runtime: image`, `builder: auto`, empty repo/rootDir. No resource is shown as falsely Live; the failure is accepted command execution after the transition. Neither faulty release was allowed to reach the 15-minute readiness / 18-minute observation deadline, so no terminal failure verdict or timeout bug is claimed.

## Producer, consumer and correction

- **Source persistence:** `lego/backend/internal/apps/service.go:4100–4175,4203–4208` stages the four source fields through SetSourceAndRegistryCredential/sourceFields.applyTo, preserving runtime and builder. `effectiveRuntime:990–1006` also returns retained explicit runtime first. This explains the captured Node metadata; changing display text alone cannot launch the command.
- **Command write/UI:** `dashboard/src/features/services/components/service-source-card.tsx:177–184` dispatches setImage. The image DeployCard uses the optional Docker Command editor; `deploy-card.tsx:119–149`, `hooks/use-start-command.ts:13–25` send the exact command to SetStartCommand. The complete mutation/readback below proves the text was saved.
- **Cause:** `lego/operator/internal/controller/deployment_projection.go:228–235` sets `container.Command=[/bin/sh,-c,StartCommand]` only if effectiveBuilder is neither native nor buildpack. `app_controller.go:1476–1496` picks native for the retained Node runtime without considering that the configured source is now an image. It therefore skips the valid override. It is not a root-directory WorkingDir error: appContainer creates no WorkingDir; the actual local projection below also leaves the command nil.
- **Sibling:** `app_controller.go:4581–4597` applies the same builder exclusion when falling back from an empty cron Command to StartCommand. The local switched-node and switched-buildpack shapes omit both Deployment and cron command overrides. Cron's explicit nonempty Command still wins; preserve that precedence.
- **Actual framework:** pinned `k8s.io/api v0.35.0`, `core/v1/types.go:2912–2935`, permits the Command slice and uses the image defaults when Command/Args are absent. [Kubernetes documentation](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) confirms command overrides. The exit-0/no-output loop is consistent with the default BusyBox process rather than the configured HTTP server. Direct live Pod/OCI inspection was not performed, so the precise executed argv remains inferred from the source and local projection.
- **Target:** an ordinary deployment of an explicitly configured prebuilt source (`spec.image != "" && spec.repo == ""`) must apply a nonempty saved StartCommand as its Docker Command, independently of retained native/buildpack metadata. Genuine native/buildpack repository artifacts keep their baked startup command; empty image commands keep image defaults. Use one narrow artifact-aware launch decision at the two command projections, including the selected-record rule below. Preserve saved build/runtime/root fields for a later switch back to Git. Do not reset runtime, rewrite the whole effectiveBuilder mapping, alter source writes, or fix this with UI retries.
- **Selection boundary:** `release_config_selection.go:292–307` changes selected command/environment fields without rewriting saved repo/image/runtime/builder. `types/v1alpha1/releasesnapshots.go:151–192` carries a separate releaseConfig.Image. Ordinary native artifact reuse can remain repo-backed, but after a staged source switch **Restart/rollback can select an older native artifact while the saved source is already an image**. Thus neither saved source alone nor deploymentParams.image (every service has one) is sufficient for selected releases. For a selection with a retained record, preserve that record's app-container Command/Args launch mode when projecting the selected command; native records keep their baked wrapper, opaque/image records keep their shell override. Thread this mode to both command projections from the selected record/template, without restoring unrelated plan/disk settings. For an ordinary unselected release, derive the mode from the explicit source plus effective builder. `runtimeConfigRecord.template` already holds the authoritative container command; `applyServingDeployment:648–665` presently calls selectedRuntimeApp and reprojects it, with a verbatim restore reserved for cancellation. Verify the legacy image-only selection separately; when origin cannot be proved, preserve the existing fallback rather than inventing an origin.

## Local diagnostic and blast radius

An ignored Go overlay ran the actual existing controller projections on current main, with **no product edits**, `go test -overlay ../../.playwright-mcp/qa-image-source-a311-overlay.json ./internal/controller -run '^TestQaA311CommandProjectionProbe$' -count=1 -v`, exit0. This prints current behavior; it is neither a fix nor a regression asserting corrected behavior.

| Shape | Deployment command | Cron fallback command |
| --- | --- | --- |
| image + retained node/native | null | null |
| image + retained buildpack | null | null |
| image/auto control | /bin/sh -c saved-command | /bin/sh -c saved-command |
| genuine native repository | null (baked control) | null (baked control) |
| genuine buildpack repository | null (baked control) | null (baked control) |
| Dockerfile repository | /bin/sh -c saved-command | /bin/sh -c saved-command |

Artifacts exist: `.playwright-mcp/qa-image-source-a311-probe_test.go`, `qa-image-source-a311-overlay.json`, `qa-image-source-a311-probe.log`, `qa-image-source-a311-probe.json`; these are supplemental ignored files. Runtime proof is embedded below.

Census: effectiveBuilder has **four production callers**: build dispatch (`app_controller.go:933`), release/artifact identity (`release_identity.go:137`), Deployment command (`deployment_projection.go:233`) and cron fallback (`app_controller.go:4591`). Change the two command predicates, preserve the other two mappings, and review identity classification as a neighbor. appContainer has one production caller in projectDeployment (`deployment_projection.go:495`); applyDeploymentSpec has one production caller in applyServingDeployment (`release_config_snapshot.go:658`). cronPodSpec has one production caller (`app_controller.go:4127`); that shared template feeds scheduled/manual cron jobs.

Resource family: web/private/worker use the shared Deployment command; cron has the separate fallback; static publication has no application container command, and Postgres/Key Value use separate controllers. **Live failure/control coverage is Node web only.** Buildpack and cron are actual local projections, private/worker are source-traced; no live failure is claimed for those siblings.

Dashboard source edit is the ordinary service Settings card, excluding static/cron's legacy source cards. Canonical `/services/:id/{settings,events,deploys/:deployId}` and Render aliases `/web/*`, `/pserv/*`, `/worker/*`, `/cron/*` reach the shared service routes; the web alias was exercised. REST PATCH /v1/services/:id, GraphQL setImage/setRepo and MCP update_service reach the existing source setter; no request-contract change is needed. Their direct live source writes were not all replayed.

Pre-settle: retain the current truthful Deploying/In Progress state until actual readiness; preserve already-served artifacts during a staged source save, cancel, or selected release. This is not an optimistic status fix. Bad shell/command, missing image, crash, readiness timeout and queued build retain their normal verdicts. Source validation and session/permission failures retain their existing errors; no new read/error taxonomy is proposed.

## Dedupe and parity

Searched **8,850 Markdown files**, including done/blocked, and scanned **77 open README records**. The targeted native/image/command census has 45 matching paths. No matching live command-execution fix was found on current main; `git log -40 -- dashboard lego` and targeted history retain the offending predicate.

- **w8/088** is the reverse image-to-Git **runtime read/CLI clone** problem. Its fix intentionally derives Docker for retained image metadata on a Git source; that cannot repair native-to-image container command projection. Link the work, preserve its scope, do not duplicate its runtime projection change.
- **w5/m76** shipped source swaps and save-without-deploy, with a healthy Dockerfile-repo/image round trip. This is the previously uncovered **native metadata + nonempty image override** case. Its DoD is placed item by item: repo/branch display and public/image dialog options observed; accepted public repo and image saves observed; save-without-deploy/control fields source-traced (no dedicated immediate full history read); image-to-repo/different repo/branch not replayed; inaccessible-repo refusal not replayed; REST/GraphQL parity and suites not rerun. **No full historical DoD or regression claim.**
- **w4/m166** provides the image Docker Command UI, **w4/m194** clears cron overrides, **w4/226** omits Git fields from _create_ requests. Preserve these controls; this successful post-create source switch is distinct.
- **w4/m180** selects serving native artifacts without rebuilding; the selected-artifact distinction above preserves it. **w4/m177** reports missing-shell startup errors; BusyBox includes the shell and this stall is exit0, not StartError.
- The first-deploy cancellation copy passes **w4/103**; its generic later-no-live limitation remains that note's existing residual, not a new finding here.
- No roadmap anti-goal applies. No paid action, account reconfiguration, product fix or production infrastructure change.

[Render's Docker docs](https://render.com/docs/docker) describe Docker Command as overriding the image default, and [prebuilt-image deployment](https://render.com/docs/deploying-an-image) describes explicit image deployments. ADR026 records bex's intentional save-first source dialog, which this fix preserves. The same native-to-image experiment was **not** run against authenticated Render production.

## Cleanup and limits

Initial build and both faulty image releases canceled by recorded deploy ID. Both exact services deleted: switched **00:28:35.760 UTC**, control **00:28:42.125 UTC**. At **00:29:15.327 UTC**, both REST details and public URLs return404; all seven baseline ID sets match. Logout succeeds; the prior session returns401 at **00:29:34.819 UTC**; browser cookies0, one blank page, 1440px; only stat-matched own session files removed. No baseline or other worker resource mutated. Kubernetes descendant teardown was not independently inspected.

Settled mobile screenshot `.playwright-mcp/qa-image-source-command-a311-mobile.png` (83,414 bytes) was viewed; it supports the stall state, not command equality. Ignored complete ledger: `.playwright-mcp/qa-image-source-command-a311-ledger.json`. Durable JSON below retains exact relevant UI mutation requests, complete bodies, comparison reads, public responses, and cleanup. Credentials are absent. No implementation is claimed.

## Durable wire and observation proof

```json
{
  "at": "2026-10-13",
  "command": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'",
  "receipts": {
    "Create": {
      "at": "2026-10-13T00:16:05.958Z",
      "request": {
        "operationName": "CreateService",
        "variables": {
          "repo": "https://github.com/bex-co/bex",
          "branch": "main",
          "name": "qa-20261012-a311-recovery",
          "type": "web_service",
          "rootDir": "examples/hello-node",
          "runtime": "node",
          "buildCommand": "printf 'qa-a311-intentional-build-failure\\n'; exit 7",
          "startCommand": "node main.js",
          "plan": "free",
          "autoDeploy": false,
          "port": 3000,
          "envVars": [
            {
              "key": "MESSAGE",
              "value": "qa-a311-recovered"
            }
          ],
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
            "id": "srv-db6ngh95i38c73fd5ob0",
            "latestDeployId": "dep-db6ngh95i38c73fd5obg",
            "name": "qa-20261012-a311-recovery",
            "phase": "",
            "projectId": null,
            "registryCredentialId": null,
            "type": "web_service"
          }
        }
      }
    },
    "Cancel": {
      "at": "2026-10-13T00:19:33.038Z",
      "request": {
        "operationName": "CancelDeploy",
        "variables": {
          "serviceId": "srv-db6ngh95i38c73fd5ob0",
          "deployId": "dep-db6ngh95i38c73fd5obg"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation CancelDeploy($serviceId: String!, $deployId: String!) {\n  cancelDeploy(serviceId: $serviceId, deployId: $deployId) {\n    id\n    status\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "cancelDeploy": {
            "__typename": "Deploy",
            "id": "dep-db6ngh95i38c73fd5obg",
            "status": "canceled"
          }
        }
      }
    },
    "SetImage": {
      "at": "2026-10-13T00:20:47.993Z",
      "request": {
        "operationName": "SetImage",
        "variables": {
          "id": "srv-db6ngh95i38c73fd5ob0",
          "image": "docker.io/library/busybox:1.37",
          "registryCredentialId": null
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation SetImage($id: String!, $image: String!, $registryCredentialId: String, $confirm: String) {\n  setImage(\n    id: $id\n    image: $image\n    registryCredentialId: $registryCredentialId\n    confirm: $confirm\n  ) {\n    id\n    repo\n    branch\n    imagePath\n    registryCredentialId\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "setImage": {
            "__typename": "Service",
            "branch": "",
            "id": "srv-db6ngh95i38c73fd5ob0",
            "imagePath": "docker.io/library/busybox:1.37",
            "registryCredentialId": null,
            "repo": ""
          }
        }
      }
    },
    "RetainedCommand": {
      "at": "2026-10-13T00:21:02.012Z",
      "command": "node main.js"
    },
    "ImageCommandSave": {
      "at": "2026-10-13T00:21:27.781Z",
      "request": {
        "operationName": "SetStartCommand",
        "variables": {
          "id": "srv-db6ngh95i38c73fd5ob0",
          "command": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation SetStartCommand($id: String!, $command: String!, $confirm: String) {\n  setStartCommand(id: $id, command: $command, confirm: $confirm) {\n    id\n    startCommand\n    phase\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "setStartCommand": {
            "__typename": "Service",
            "id": "srv-db6ngh95i38c73fd5ob0",
            "phase": "Canceled",
            "startCommand": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'"
          }
        }
      }
    },
    "SourceState1": {
      "at": "2026-10-13T00:22:31.010Z",
      "request": {
        "operationName": "QaA311SourceState",
        "query": "query QaA311SourceState($id:String!){service(id:$id){id name type phase revision url repo branch imagePath rootDir runtime builder buildCommand startCommand plan}deploys(serviceId:$id,limit:10){id status trigger createdAt updatedAt startedAt finishedAt failureReason stallReason cancelReason}}",
        "variables": {
          "id": "srv-db6ngh95i38c73fd5ob0"
        }
      },
      "status": 200,
      "body": {
        "data": {
          "deploys": [
            {
              "cancelReason": "",
              "createdAt": "2026-10-13T00:21:26.623216Z",
              "failureReason": "",
              "finishedAt": "",
              "id": "dep-db6nj1h2s0sc73a8hkp0",
              "stallReason": "",
              "startedAt": "2026-10-13T00:21:34.464392Z",
              "status": "update_in_progress",
              "trigger": "config_change",
              "updatedAt": "2026-10-13T00:21:34.464393Z"
            },
            {
              "cancelReason": "",
              "createdAt": "2026-10-13T00:16:05.061862Z",
              "failureReason": "",
              "finishedAt": "2026-10-13T00:19:32.940537Z",
              "id": "dep-db6ngh95i38c73fd5obg",
              "stallReason": "",
              "startedAt": "",
              "status": "canceled",
              "trigger": "create",
              "updatedAt": "2026-10-13T00:19:32.940538Z"
            }
          ],
          "service": {
            "branch": "",
            "buildCommand": "printf 'qa-a311-intentional-build-failure\\n'; exit 7",
            "builder": "native",
            "id": "srv-db6ngh95i38c73fd5ob0",
            "imagePath": "docker.io/library/busybox:1.37",
            "name": "qa-20261012-a311-recovery",
            "phase": "Deploying",
            "plan": "free",
            "repo": "",
            "revision": "",
            "rootDir": "examples/hello-node",
            "runtime": "node",
            "startCommand": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'",
            "type": "web_service",
            "url": "https://qa-20261012-a311-recovery.onbex.co"
          }
        }
      }
    },
    "ImageState2": {
      "at": "2026-10-13T00:23:32.317Z",
      "request": {
        "operationName": "QaA311SourceState",
        "query": "query QaA311SourceState($id:String!){service(id:$id){id name type phase revision url repo branch imagePath rootDir runtime builder buildCommand startCommand plan}deploys(serviceId:$id,limit:10){id status trigger createdAt updatedAt startedAt finishedAt failureReason stallReason cancelReason}}",
        "variables": {
          "id": "srv-db6ngh95i38c73fd5ob0"
        }
      },
      "status": 200,
      "body": {
        "data": {
          "deploys": [
            {
              "cancelReason": "",
              "createdAt": "2026-10-13T00:21:26.623216Z",
              "failureReason": "",
              "finishedAt": "",
              "id": "dep-db6nj1h2s0sc73a8hkp0",
              "stallReason": "container exited shortly after start and is restarting repeatedly (last exit code 0) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped).",
              "startedAt": "2026-10-13T00:21:34.464392Z",
              "status": "update_in_progress",
              "trigger": "config_change",
              "updatedAt": "2026-10-13T00:21:34.464393Z"
            },
            {
              "cancelReason": "",
              "createdAt": "2026-10-13T00:16:05.061862Z",
              "failureReason": "",
              "finishedAt": "2026-10-13T00:19:32.940537Z",
              "id": "dep-db6ngh95i38c73fd5obg",
              "stallReason": "",
              "startedAt": "",
              "status": "canceled",
              "trigger": "create",
              "updatedAt": "2026-10-13T00:19:32.940538Z"
            }
          ],
          "service": {
            "branch": "",
            "buildCommand": "printf 'qa-a311-intentional-build-failure\\n'; exit 7",
            "builder": "native",
            "id": "srv-db6ngh95i38c73fd5ob0",
            "imagePath": "docker.io/library/busybox:1.37",
            "name": "qa-20261012-a311-recovery",
            "phase": "Deploying",
            "plan": "free",
            "repo": "",
            "revision": "",
            "rootDir": "examples/hello-node",
            "runtime": "node",
            "startCommand": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'",
            "type": "web_service",
            "url": "https://qa-20261012-a311-recovery.onbex.co"
          }
        }
      }
    },
    "CancelImage": {
      "at": "2026-10-13T00:24:27.455Z",
      "request": {
        "operationName": "CancelDeploy",
        "variables": {
          "serviceId": "srv-db6ngh95i38c73fd5ob0",
          "deployId": "dep-db6nj1h2s0sc73a8hkp0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation CancelDeploy($serviceId: String!, $deployId: String!) {\n  cancelDeploy(serviceId: $serviceId, deployId: $deployId) {\n    id\n    status\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "cancelDeploy": {
            "__typename": "Deploy",
            "id": "dep-db6nj1h2s0sc73a8hkp0",
            "status": "canceled"
          }
        }
      }
    },
    "Retry": {
      "at": "2026-10-13T00:24:50.432Z",
      "request": {
        "operationName": "TriggerDeploy",
        "variables": {
          "serviceId": "srv-db6ngh95i38c73fd5ob0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation TriggerDeploy($serviceId: String!, $commitId: String, $deployMode: String, $clearCache: String, $confirm: String) {\n  triggerDeploy(\n    serviceId: $serviceId\n    commitId: $commitId\n    deployMode: $deployMode\n    clearCache: $clearCache\n    confirm: $confirm\n  ) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "triggerDeploy": {
            "__typename": "Deploy",
            "createdAt": "2026-10-13T00:24:49.214059Z",
            "id": "dep-db6nkk92s0sc73a8hkr0",
            "image": "docker.io/library/busybox:1.37",
            "rollbackOf": "",
            "status": "created",
            "trigger": "api"
          }
        }
      }
    },
    "ControlCreate": {
      "at": "2026-10-13T00:25:23.534Z",
      "request": {
        "operationName": "CreateService",
        "variables": {
          "image": "docker.io/library/busybox:1.37",
          "name": "qa-20261012-a311-control",
          "type": "web_service",
          "registryCredentialId": "",
          "runtime": "image",
          "startCommand": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'",
          "plan": "free",
          "port": 3000,
          "envVars": [
            {
              "key": "MESSAGE",
              "value": "qa-a311-recovered"
            }
          ],
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
            "id": "srv-db6nksh5i38c73fd5ogg",
            "latestDeployId": "dep-db6nksh5i38c73fd5oh0",
            "name": "qa-20261012-a311-control",
            "phase": "",
            "projectId": null,
            "registryCredentialId": "",
            "type": "web_service"
          }
        }
      }
    },
    "Comparison1": {
      "at": "2026-10-13T00:26:18.731Z",
      "states": [
        {
          "request": {
            "operationName": "QaA311SourceState",
            "query": "query QaA311SourceState($id:String!){service(id:$id){id name type phase revision url repo branch imagePath rootDir runtime builder buildCommand startCommand plan}deploys(serviceId:$id,limit:10){id status trigger createdAt updatedAt startedAt finishedAt failureReason stallReason cancelReason}}",
            "variables": {
              "id": "srv-db6ngh95i38c73fd5ob0"
            }
          },
          "status": 200,
          "body": {
            "data": {
              "deploys": [
                {
                  "cancelReason": "",
                  "createdAt": "2026-10-13T00:24:49.214059Z",
                  "failureReason": "",
                  "finishedAt": "",
                  "id": "dep-db6nkk92s0sc73a8hkr0",
                  "stallReason": "container exited shortly after start and is restarting repeatedly (last exit code 0) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped).",
                  "startedAt": "2026-10-13T00:24:51.76815Z",
                  "status": "update_in_progress",
                  "trigger": "api",
                  "updatedAt": "2026-10-13T00:24:51.76815Z"
                },
                {
                  "cancelReason": "",
                  "createdAt": "2026-10-13T00:21:26.623216Z",
                  "failureReason": "",
                  "finishedAt": "2026-10-13T00:24:23.660435Z",
                  "id": "dep-db6nj1h2s0sc73a8hkp0",
                  "stallReason": "",
                  "startedAt": "2026-10-13T00:21:34.464392Z",
                  "status": "canceled",
                  "trigger": "config_change",
                  "updatedAt": "2026-10-13T00:24:23.660436Z"
                },
                {
                  "cancelReason": "",
                  "createdAt": "2026-10-13T00:16:05.061862Z",
                  "failureReason": "",
                  "finishedAt": "2026-10-13T00:19:32.940537Z",
                  "id": "dep-db6ngh95i38c73fd5obg",
                  "stallReason": "",
                  "startedAt": "",
                  "status": "canceled",
                  "trigger": "create",
                  "updatedAt": "2026-10-13T00:19:32.940538Z"
                }
              ],
              "service": {
                "branch": "",
                "buildCommand": "printf 'qa-a311-intentional-build-failure\\n'; exit 7",
                "builder": "native",
                "id": "srv-db6ngh95i38c73fd5ob0",
                "imagePath": "docker.io/library/busybox:1.37",
                "name": "qa-20261012-a311-recovery",
                "phase": "Deploying",
                "plan": "free",
                "repo": "",
                "revision": "",
                "rootDir": "examples/hello-node",
                "runtime": "node",
                "startCommand": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'",
                "type": "web_service",
                "url": "https://qa-20261012-a311-recovery.onbex.co"
              }
            }
          }
        },
        {
          "request": {
            "operationName": "QaA311SourceState",
            "query": "query QaA311SourceState($id:String!){service(id:$id){id name type phase revision url repo branch imagePath rootDir runtime builder buildCommand startCommand plan}deploys(serviceId:$id,limit:10){id status trigger createdAt updatedAt startedAt finishedAt failureReason stallReason cancelReason}}",
            "variables": {
              "id": "srv-db6nksh5i38c73fd5ogg"
            }
          },
          "status": 200,
          "body": {
            "data": {
              "deploys": [
                {
                  "cancelReason": "",
                  "createdAt": "2026-10-13T00:25:22.603621Z",
                  "failureReason": "",
                  "finishedAt": "2026-10-13T00:25:34.495886Z",
                  "id": "dep-db6nksh5i38c73fd5oh0",
                  "stallReason": "",
                  "startedAt": "",
                  "status": "live",
                  "trigger": "create",
                  "updatedAt": "2026-10-13T00:25:34.495887Z"
                }
              ],
              "service": {
                "branch": "",
                "buildCommand": "",
                "builder": "auto",
                "id": "srv-db6nksh5i38c73fd5ogg",
                "imagePath": "docker.io/library/busybox:1.37",
                "name": "qa-20261012-a311-control",
                "phase": "Running",
                "plan": "free",
                "repo": "",
                "revision": "rev-1",
                "rootDir": "",
                "runtime": "image",
                "startCommand": "sh -c 'mkdir -p /tmp/qa-a311-www; printf \"qa-a311-recovered\\n\" > /tmp/qa-a311-www/index.html; exec httpd -f -p 3000 -h /tmp/qa-a311-www'",
                "type": "web_service",
                "url": "https://qa-20261012-a311-control.onbex.co"
              }
            }
          }
        }
      ]
    },
    "HttpComparison1": {
      "at": "2026-10-13T00:26:21.108Z",
      "results": [
        {
          "url": "https://qa-20261012-a311-recovery.onbex.co/",
          "status": 503,
          "body": "service unavailable\n"
        },
        {
          "url": "https://qa-20261012-a311-control.onbex.co/",
          "status": 200,
          "body": "qa-a311-recovered\n"
        }
      ]
    },
    "FreshMobileStall": {
      "at": "2026-10-13T00:23:58.136Z",
      "main": "- main:\n  - button \"Toggle Sidebar\"\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261012-a311-recovery\"\n  - button \"Search\"\n  - button \"New\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261012-a311-recovery\" [level=1]\n  - text: Service Deploying\n  - 'link \"Latest deploy: In Progress\"':\n    - /url: /services/srv-db6ngh95i38c73fd5ob0/deploys/dep-db6nj1h2s0sc73a8hkp0\n    - text: Latest deploy In Progress\n  - link \"Free\":\n    - /url: /services/srv-db6ngh95i38c73fd5ob0/plan\n  - text: Runtime node\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db6ngh95i38c73fd5ob0\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261012-a311-recovery.onbex.co\":\n    - /url: https://qa-20261012-a311-recovery.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261012-a311-recovery\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: —\n  - term: Created\n  - definition:\n    - time: 7m\n  - text: Deploy status In Progress Config Change dep-db6nj1h2s0sc73a8hkp0\n  - button \"Cancel deploy dep-db6nj1h2s0sc73a8hkp0\" [disabled]: Cancel\n  - paragraph: \"container exited shortly after start and is restarting repeatedly (last exit code 0) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped).\"\n  - paragraph: docker.io/library/busybox:1.37\n  - term: \"Created:\"\n  - definition: October 12, 2026 at 5:21 PM\n  - term: \"Updated:\"\n  - definition: October 12, 2026 at 5:21 PM\n  - term: \"Started:\"\n  - definition: October 12, 2026 at 5:21 PM\n  - text: Status timeline\n  - list:\n    - listitem:\n      - paragraph: Deploy created\n      - time: October 12, 2026 at 5:21 PM\n    - listitem:\n      - paragraph: Deploy started\n      - time: October 12, 2026 at 5:21 PM\n    - listitem:\n      - paragraph: In Progress\n      - time: October 12, 2026 at 5:21 PM\n  - button \"Log type\": All logs\n  - textbox \"Search logs…\"\n  - button \"Log options\"\n  - button \"Maximize\"\n  - text: Loading logs…",
      "width": {
        "inner": 390,
        "doc": 390
      }
    },
    "FreshRetrySettled": {
      "at": "2026-10-13T00:27:45.448Z",
      "main": "- main:\n  - button \"Toggle Sidebar\"\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261012-a311-recovery\"\n  - button \"Search\"\n  - button \"New\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261012-a311-recovery\" [level=1]\n  - text: Service Deploying\n  - 'link \"Latest deploy: In Progress\"':\n    - /url: /services/srv-db6ngh95i38c73fd5ob0/deploys/dep-db6nkk92s0sc73a8hkr0\n    - text: Latest deploy In Progress\n  - link \"Free\":\n    - /url: /services/srv-db6ngh95i38c73fd5ob0/plan\n  - text: Runtime node\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db6ngh95i38c73fd5ob0\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261012-a311-recovery.onbex.co\":\n    - /url: https://qa-20261012-a311-recovery.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261012-a311-recovery\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: —\n  - term: Created\n  - definition:\n    - time: 11m\n  - text: Deploy status In Progress Manual Deploy dep-db6nkk92s0sc73a8hkr0\n  - button \"Cancel deploy dep-db6nkk92s0sc73a8hkr0\": Cancel\n  - paragraph: \"container exited shortly after start and is restarting repeatedly (last exit code 0) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped).\"\n  - paragraph: docker.io/library/busybox:1.37\n  - term: \"Created:\"\n  - definition: October 12, 2026 at 5:24 PM\n  - term: \"Updated:\"\n  - definition: October 12, 2026 at 5:24 PM\n  - term: \"Started:\"\n  - definition: October 12, 2026 at 5:24 PM\n  - text: Status timeline\n  - list:\n    - listitem:\n      - paragraph: Deploy created\n      - time: October 12, 2026 at 5:24 PM\n    - listitem:\n      - paragraph: Deploy started\n      - time: October 12, 2026 at 5:24 PM\n    - listitem:\n      - paragraph: In Progress\n      - time: October 12, 2026 at 5:24 PM\n  - button \"Log type\": All logs\n  - textbox \"Search logs…\"\n  - button \"Log options\"\n  - button \"Maximize\"\n  - text: 05:24:49 PM\n  - button \"Filter logs by instance srv-db6ngh95i38c73fd5ob0-c0ea0h58408qcjc4cujk\": \"[c0ea0]\"\n  - text: ==> Deploy queued 05:24:51 PM\n  - button \"Filter logs by instance srv-db6ngh95i38c73fd5ob0-c0ea0h58408qcjc4cujk\": \"[c0ea0]\"\n  - text: ==> Deploying image docker.io/library/busybox:1.37"
    },
    "CancelRetry": {
      "at": "2026-10-13T00:28:10.621Z",
      "request": {
        "operationName": "CancelDeploy",
        "variables": {
          "serviceId": "srv-db6ngh95i38c73fd5ob0",
          "deployId": "dep-db6nkk92s0sc73a8hkr0"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation CancelDeploy($serviceId: String!, $deployId: String!) {\n  cancelDeploy(serviceId: $serviceId, deployId: $deployId) {\n    id\n    status\n    __typename\n  }\n}"
      },
      "status": 200,
      "body": {
        "data": {
          "cancelDeploy": {
            "__typename": "Deploy",
            "id": "dep-db6nkk92s0sc73a8hkr0",
            "status": "canceled"
          }
        }
      }
    },
    "Delete": {
      "at": "2026-10-13T00:28:35.760Z",
      "request": {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db6ngh95i38c73fd5ob0"
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
    "ControlDelete": {
      "at": "2026-10-13T00:28:42.125Z",
      "request": {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db6nksh5i38c73fd5ogg"
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
      "at": "2026-10-13T00:29:15.327Z",
      "request": {
        "operationName": "QaA311Inventory",
        "query": "query QaA311Inventory($ownerId:String!){services(ownerId:$ownerId){id name type} databases(ownerId:$ownerId){id name} keyValues(ownerId:$ownerId){id name} projects(ownerId:$ownerId){id name} envGroups(ownerId:$ownerId){id name} blueprints(ownerId:$ownerId){id name} webhookEndpoints(ownerId:$ownerId){id name url eventTypes enabled}}",
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
          "url": "https://api.bex.co/v1/services/srv-db6ngh95i38c73fd5ob0",
          "status": 404
        },
        {
          "url": "https://api.bex.co/v1/services/srv-db6nksh5i38c73fd5ogg",
          "status": 404
        },
        {
          "url": "https://qa-20261012-a311-recovery.onbex.co/",
          "status": 404
        },
        {
          "url": "https://qa-20261012-a311-control.onbex.co/",
          "status": 404
        }
      ]
    },
    "SessionCleanup": {
      "at": "2026-10-13T00:29:34.819Z",
      "ownersStatus": 401,
      "cookies": 0,
      "pages": 1,
      "url": "about:blank"
    }
  },
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
