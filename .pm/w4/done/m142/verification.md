# m142 verification — 2026-10-02

## Policy contract and rollout

The operator projects `app.bex.co/container-policy=image-v1` and the immutable App UID from the selected App into application pod labels. Deployment and cron use `appPolicyLabels`; predeploy carries the persisted policy through its existing options. Co-located predeploy Jobs now have a controlling App owner. Admission requires the marker, a matching controlling `app.bex.co/v1alpha1` App owner/name/UID, an eligible Deployment/CronJob/Job, and a sole `app` or identified `predeploy` container. StatefulSets cannot acquire the exception.

The four-capability ceiling remains CHOWN, DAC_OVERRIDE, SETUID and SETGID. Image-v1 requires drop ALL and explicit no privilege escalation. Init containers require drop ALL, no additions and explicit no escalation. All hosting containers must have effective RuntimeDefault seccomp: a container override takes precedence over a pod default. Host access, token mounting, namespace/identity restrictions and the separate disk DAC-bypass set remain enforced.

The exact operator service account is the policy issuer. CEL cannot read App.spec or prove an owner reference exists; ownership and labels establish consistency, not independent authorization. Other identities receive no privileges from a marker: tenant identities have no workload RBAC. Kubernetes creates scheduled child Jobs from the admitted CronJob template; manual cron Jobs are separately admitted operator writes.

Deploy the admission policy/binding first (existing Argo sync waves -3/-2), then the operator image. Old producers lack the marker and remain refused by the new policy. New producers are refused by the old policy. Neither partial rollout widens admission. Once both are present, existing persisted image-v1 Apps are reprojected on reconcile; no App policy migration or new creation flag is required. A historical failed deploy stays failed; a new manual deploy provides a new outcome. Recorded release templates without the marker remain refused on restore; redeploying that release through the current producer supplies the marker. Do not infer a policy for historical templates from their capability list.

The creation allowlist remains the previously enabled workspace `tea-d98210cbbpdc73dcrkvg`. No customer workspace was added. ADR089's full ClickHouse storage, peer-networking, cross-workspace, isolated-environment, node/metadata/platform-service isolation and reference-propagation gates remain open; this change does not satisfy those gates or authorize broader rollout.

## Caller matrix

| Family | Producer and verification |
| --- | --- |
| Web/private/worker | `applyServingDeployment` / `appContainer` / `appPolicyLabels`; actual legacy, strict-v1 and image-v1 Deployment output submitted to an isolated API server with the complete production policy. |
| Scheduled cron | `cronPodSpec` + `convergeCronRuntime`; actual CronJob templates in each policy. Kubernetes owns scheduled child Jobs. |
| Manual cron | `ensureManualRun` via `convergeCronRuntime`; actual App-owned Jobs in each policy. |
| Predeploy | `reconcilePreDeploy` + `predeploy.Job`; actual co-located App-owned Jobs in each policy. |
| Static | `publish.PublishJob` retains strict contexts; static serving uses shared static-server, not `appSecCtx`. Backend creation excludes static sites from image compatibility. |
| Postgres | Actual `exportJob` remains restricted. Database creation/exports still use the unchanged `tenantSecCtx`. |
| Key Value | Actual `reconcileKeyValueWorkload` StatefulSet remains restricted. Its backup helpers still use `tenantSecCtx`. |
| Disk snapshots/restores | Actual `diskBackupCronJobSpec` / `createDiskRestoreJob` keep their separate DAC_OVERRIDE/CHOWN/FOWNER exception. |

The three `appSecCtx` call sites remain Deployment, cron and predeploy. The 13 production `tenantSecCtx` calls remain application policy (1), disk backup (1), database controller (1), database exports (3), Key Value controller (2) and Key Value backup (5). There is no new common capability default.

## Surface parity

[Render Docker documentation](https://render.com/docs/docker), checked 2026-10-02, supports Dockerfile and prebuilt-image deployments, predeploy commands and private networking. It does not document an identical Linux capability policy; the bounded capability contract is Bex's ADR089 decision.

No API fields or state transitions change. `store/reconciler.go` maps a current Ready-condition refusal to `update_failed` and requires an active Running release for `live`; `deploys/service.go` copies status and failureReason into its view. REST and MCP share `toRenderDeploy`; GraphQL exposes the same fields; dashboard `deploy-status.ts` distinguishes failure from Live. Live adapter replay remains a separate acceptance step.

## Execution record

- Targeted projection checks and final `bash scripts/gitops-validate.sh` passed. The complete-policy API-server regression passed in 17.748s: `GOWORK=off go test ./internal/controller -run '^TestControllers$' -ginkgo.focus='Application container policy admission' -ginkgo.no-color -count=1`.
- An isolated archive of original producer and policy revision `1589e8459dafe3535c87229c9d71341a3d0dc226`, with only the new test copied in, failed precisely on CREATE of the actual image-v1 web Deployment with the original admission 403. This is a before/after reproduction of the filed bug, not a hand-written approximation of the predicate.
- The matrix verifies provenance, capabilities, effective seccomp, init-container restrictions, host access, token mounting, identity and canonical namespaces. Job pod-template UPDATE negatives are refused by Kubernetes immutability before admission; Job metadata updates, CREATE admission and Deployment/CronJob/StatefulSet UPDATE paths are separately exercised. Legacy invalid bodies can still be deleted within canonical namespaces.
- The initial full operator run passed all 180 existing controller specs and failed only the new test's incorrect matcher for a non-error return. That test issue and its two lint findings were fixed. Final `make test` passed (controller package 120.558s, 85.1% statement coverage); `make lint` passed all four modules and the dead-code gate. The simplified admission test passed again in 15.377s. No generated CRD/RBAC/deepcopy drift remained.
- Read-only production observation: the live `bex-operator-workloads` policy has no `imageCompatibilityWorkload` variable, and reports no type-check warnings. The current production policy therefore cannot accept the new producer contract yet.
- QA login succeeded in the existing bex workspace. No QA fixture or production Kubernetes resource was mutated by this implementation run. Its session was revoked (`ok logged-out`) and its local cookie jar removed. The live Docker/native initial-deploy, retry, environment-update and adapter replay remain pending the production release carrying both halves of the fix.

- `/simplify` completed its reuse, quality and efficiency reviews. The test now identifies metadata-only mutations explicitly and avoids 46 redundant API reads; its post-review regression passed. Skill-layout validation, repository Markdown formatting and `git diff --check` also passed.
