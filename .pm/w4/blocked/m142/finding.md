# Blocker — new Docker web deploys are rejected by the live workload policy

Why: the enabled image-compatibility canary cannot complete a basic Docker hosting journey.

## Reproduction and evidence

2026-10-02 07:20–07:35 UTC, dashboard.bex.co, workspace `bex` / `tea-d98210cbbpdc73dcrkvg` (Scale, admin). No paid plan was selected. Create `qa-20261002-web-r1`, free Docker web, public repo `bex-co/bex@main`, root `examples/hello-go`, port 3000, auto-deploy disabled, `MESSAGE=qa-20261002-initial-r1`, in the QA project/environment. Initial deploy fails. Hard-load its Deploys page and choose Manual Deploy → Deploy latest commit: the retry fails with the same admission rejection. Expected: Live, Running, a serving public URL. Actual: two `update_failed` deploys, service Failed, no active revision.

Screenshot `.playwright-mcp/qa-deploys-r1-1.png` records the failure; it and the JSON capture were checked on disk. The complete durable probe below is a browser-session POST to `https://api.bex.co/graphql` with JSON Content-Type and credentials included. IDs identify deleted fixtures; recreate equivalent QA fixtures when replaying.

```json
{
  "date": "2026-10-02T07:25:50.249Z",
  "request": {
    "query": "query QaDeployComparison { docker: server(id:\"srv-davlmi6o5s1c7398lhug\") { id phase revision } dockerDeploys: deploys(serviceId:\"srv-davlmi6o5s1c7398lhug\",limit:5) { id status failureReason createdAt startedAt finishedAt } native: server(id:\"srv-davlofoaijhc73cn2qi0\") { id phase revision } nativeDeploys: deploys(serviceId:\"srv-davlofoaijhc73cn2qi0\",limit:5) { id status failureReason createdAt startedAt finishedAt } envGroup(id:\"evg-davloomo5s1c7398li3g\") { id name environmentId serviceLinks } }"
  },
  "status": 200,
  "body": {
    "data": {
      "docker": {
        "id": "srv-davlmi6o5s1c7398lhug",
        "phase": "Failed",
        "revision": ""
      },
      "dockerDeploys": [
        {
          "createdAt": "2026-10-02T07:24:10.525346Z",
          "failureReason": "deployments.apps \"tea-d98210cbbpdc73dcrkvg-qa-20261002-web-r1\" is forbidden: ValidatingAdmissionPolicy 'bex-operator-workloads' with binding 'bex-operator-workloads' denied request: operator-authored containers outside the execution boundary must be restricted (no added capabilities, no privilege escalation, RuntimeDefault seccomp; disk snapshot/restore Jobs may add only their DAC-bypass set)",
          "finishedAt": "2026-10-02T07:25:23.607286Z",
          "id": "dep-davlo6mo5s1c7398li1g",
          "startedAt": "2026-10-02T07:24:23.532295Z",
          "status": "update_failed"
        },
        {
          "createdAt": "2026-10-02T07:20:40.684067Z",
          "failureReason": "deployments.apps \"tea-d98210cbbpdc73dcrkvg-qa-20261002-web-r1\" is forbidden: ValidatingAdmissionPolicy 'bex-operator-workloads' with binding 'bex-operator-workloads' denied request: operator-authored containers outside the execution boundary must be restricted (no added capabilities, no privilege escalation, RuntimeDefault seccomp; disk snapshot/restore Jobs may add only their DAC-bypass set)",
          "finishedAt": "2026-10-02T07:22:53.421411Z",
          "id": "dep-davlmi6o5s1c7398lhv0",
          "startedAt": "2026-10-02T07:21:53.476217Z",
          "status": "update_failed"
        }
      ],
      "envGroup": {
        "environmentId": "env-davlm9oaijhc73cn2qfg",
        "id": "evg-davloomo5s1c7398li3g",
        "name": "qa-20261002-group-r1",
        "serviceLinks": ["srv-davlmi6o5s1c7398lhug"]
      },
      "native": {
        "id": "srv-davlofoaijhc73cn2qi0",
        "phase": "Building",
        "revision": ""
      },
      "nativeDeploys": [
        {
          "createdAt": "2026-10-02T07:24:47.47289Z",
          "failureReason": "",
          "finishedAt": "",
          "id": "dep-davlofoaijhc73cn2qig",
          "startedAt": "",
          "status": "queued"
        }
      ]
    }
  }
}
```

The native Go control's initial deploy reached Live at 07:27:23Z. After saving MESSAGE through the Environment editor, `dep-davlqu0aijhc73cn2qk0` reached Live at 07:31:40Z; the header showed Running, rev-3. `curl -sS --max-time 20 -i https://qa-20261002-go-r1.onbex.co/qa-20261002-control-r1` at 07:34:12Z returned HTTP/2 200, Content-Type text/plain, Content-Length 22 and body `qa-20261002-updated-r1`.

Read-only production Kubernetes checks independently found the Docker App's `spec.containerPolicy=image-v1` and `status.phase=Failed`; the native App had the legacy empty policy, and its ready Deployment had `allowPrivilegeEscalation:false`, `capabilities.drop:[ALL]`, no added capabilities, and RuntimeDefault seccomp. Reading the live `bex-operator-workloads` ValidatingAdmissionPolicy confirmed the repo's no-added-capabilities predicate below. No cluster mutation was performed.

## Root cause and target behavior

- `lego/backend/internal/apps/service.go:2226-2248`: `configureNewImageCompatibility` selects image-v1 for allowlisted, newly created Docker/image Apps, excluding static. Its two production callers are at 1892 and 2097 (ordinary creation and blueprint creation).
- `lego/operator/config/prod/kustomization.yaml:106-113`: the canary workspace is already enabled by `BEX_IMAGE_COMPATIBILITY_WORKSPACES`.
- `lego/types/v1alpha1/app_types.go:303-309` permits persisted strict-v1/image-v1; `app_identity.go:50` includes the policy in release identity. A changed creation flag does not migrate existing Apps.
- `lego/operator/internal/controller/container_policy.go:27-32`: image-v1 adds exactly CHOWN, DAC_OVERRIDE, SETUID, SETGID over the strict base.
- `deploy/gitops/base/operator-workload-admission.yaml:143-159`: outside the execution namespace boundary the consumer requires `capabilities.add.size()==0`, except the distinct disk snapshot/restore Job allowance. This rejects image-v1 before the workload is admitted. The live expression matched HEAD.

**Fix target:** preserve ADR089's bounded compatibility profile and implement its admission counterpart, with an operator-owned discriminator derived from the persisted App policy and validated workload provenance. The admission expression cannot read App.spec directly: producer projection and policy consumer must be designed together. Admit only the documented image-v1 capability set on eligible App workloads; retain the existing operator identity, canonical namespace, no privileged/host access, no privilege escalation and RuntimeDefault requirements. Existing strict Apps, platform workloads and managed data must retain their protections. Do not globally remove the capability restriction or assume turning off the create flag repairs persisted Apps. Complete the ADR's required isolation verification before broadening rollout.

## Blast radius and adjacent classes

Exhaustive production Go grep found **three appSecCtx call sites**: `deployment_projection.go:219` (web/private/worker Deployment), `app_controller.go:4177` (cron pod template, scheduled/manual runs), and `app_controller.go:5183` (predeploy Job). The shared admission policy covers **four workload kinds**: Deployment, StatefulSet, Job and CronJob (lines 27–36). Its operator-service-account match is at 39 and namespace constraints at 55–64.

The unchanged `tenantSecCtx` has **13 production call sites**: app compatibility (1), disk backup (1), database controller (1), database exports (3), Key Value backup (5), Key Value controller (2). Static sites are excluded at creation and served by static-server; Postgres and Key Value do not call appSecCtx. t002 must verify the complete family: web, static, cron, worker, private, Postgres and Key Value. This is a bounded workload-profile exception, not a global capability allowance.

Unauthorized and wrong-namespace requests must still fail before admission; forbidden host/privileged/capability shapes remain refused. Invalid App policy values remain schema failures. A real build failure remains build_failed; admission and runtime failures must remain distinguishable from success across deploy APIs/UI. No auth or existence-disclosure behavior should change.

**Unverified this sweep:** private/worker/cron/predeploy live deployments, static/datastore policy controls, MCP deploy representation, broader workspace behavior, isolation negative tests. Earlier cron failures are cited as w1/m163 evidence, not as this sweep's probes.

## Dedupe and Render

Open and done board items and targeted history were searched. [w1/m163](../../../w1/done/m163/README.md) records this exact cause as an unscheduled residual; its suspend/resume DoD is separate and stays closed. No open item covered admission for image-v1. The current producer/policy code still conflicts, so this is not deploy lag. No anti-goal applies.

[Render Docker documentation](https://render.com/docs/docker) supports Dockerfile and prebuilt-image services. It does not specify this capability set; bex's isolation contract comes from ADR089.

## Hunt cleanup

**Resolved at 07:53:17Z:** the exact Docker App lookup returned no objects. The operator's normal finalizer retry completed; no forced removal or platform change was used. All seven API fixtures and both App resources are now absent.

All seven API fixtures (two services, three groups, one environment and one project) were deleted and each exact-ID REST read returned 404; the overview no longer listed them. The native App disappeared. At filing time the failed Docker App still has a deletion timestamp and finalizer. Its 07:36:07Z operator error names registry tag cleanup: `zot.bex-registry.svc:5000` refused the connection. Zot subsequently showed Ready, but the App had not yet disappeared on the next check. This infrastructure residue is still being monitored by the continuing hunt; no finalizer was forced away and no unrelated resource was changed. The run's Kratos session was successfully revoked and its local cookie files removed.
