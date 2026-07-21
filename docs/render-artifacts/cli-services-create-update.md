# Official CLI service create/update contract

This artifact freezes the `render-oss/cli` v2.21.0 contract exercised by `scripts/cli-services-parity-verify.sh`. The CLI checkout in `cli/` was built and run unmodified. No first-party CLI, preview-environment implementation, or synthetic mapping from a bex region to a Render region is part of this work.

## Evidence

- **Date:** 2026-07-18.
- **Baseline target:** isolated local dev-9, current `lego/backend` and current App CRD, reached through `scripts/cli-compat.sh`. The maintained baseline created, read, updated, cloned, and deleted representative web, native-cron, and static services.
- **Configured-capability target:** the same isolated API plus disposable OpenBao and auth-enabled persistent Zot. A private-image negative control failed without credentials; the official CLI then created a service with one registry credential, replaced it with a second credential through `services update`, and the kubelet pulled the image to a Running App. The configured leg also read back a CLI env var, an OpenBao secret file, and the native cron command. Secrets, bearer tokens, file contents, registry passwords, and kubeconfig contents were never captured.
- **Production m49 target:** `https://api.bex.co/v1/`, final operator digest `sha256:03db349ef81942dad2904827185bf17912bd9658783d67c9b7b42ea878c96987`, on 2026-07-21 UTC. The checksum-pinned, unmodified v2.21.0 CLI passed the full baseline, including image create/update/delete and immediate delete during a native service's first repository build. Every explicit and trap cleanup reached raw GET 404 plus official-CLI list absence; a separate cluster/Zot audit found zero matching residue.
- **Version-skew note:** dev-9's installed operator image predates the new structured App field. It was paused while current API/CRD wire assertions ran and resumed for the in-cluster private-image rollout. Current operator projection, legacy fallback, and exact CIDR enforcement are covered by the operator/types suite.
- **Commands:** `scripts/cli-compat.sh services-parity-verify baseline`, `scripts/cli-compat.sh services-parity-verify configured`, and `scripts/cli-compat.sh services-parity-self-test`.

The configured run ended with these independent facts: all baseline assertions passed; CLI env-var and secret-file values round-tripped; create and update allowlist descriptions survived exactly; the explicit clone region worked; the bare-clone and runtime guards failed before a mutation; previews failed explicitly at bex; anonymous private-image pull failed; both create-time and update-time registry credential IDs read back exactly; and the credentialed App reached `Running`. The verifier's cleanup trap was also observed after success and deliberate assertion failures.

### 2026-07-20 nested-image and deletion correction (production accepted)

The production OpenAPI rollout exposed two assumptions the 2026-07-18 pass did not retain as exact-wire/deletion-convergence gates:

- The generated v2.21.0 `client.Image` model has required `imagePath` and `ownerId` fields. `BuildCreateRequest` and `BuildUpdateRequest` populate `imagePath` but not the nested owner, so the unmodified CLI serializes these redacted fragments exactly:

  ```json
  {
    "ownerId": "<workspace-id>",
    "image": { "imagePath": "<image-ref>", "ownerId": "" }
  }
  ```

  ```json
  { "image": { "imagePath": "<replacement-image-ref>", "ownerId": "" } }
  ```

  The pinned Render OpenAPI accepted both. The later strict Go adapter rejected the nested key because its REST-only `imageRef` omitted `ownerId`. The adapter now treats a blank nested owner as inheritance, rejects conflicting non-blank owners by the field name `image.ownerId`, preserves membership-based 403 behavior, and keeps unknown sibling keys strict. A composition regression sends both fragments through authentication, the pinned OpenAPI middleware, and the real service handler.

- A production repo-backed service deleted three seconds after create remained visible in `Deleting`. The operator held its reconcile worker in a synchronous first-build poll for 20 minutes, then finalization hit missing build-namespace `list`/`delete` RBAC and attempted anonymous Zot cleanup because the configured shared push Secret was absent. The old finalizer also revoked the per-App credential before registry absence was proven, destroying its own retry authority. The shipped correction interrupts build polling on App deletion, grants only the missing namespaced inventory verbs, repairs/activates the least-privilege per-App credential, persists registry-absence proof, and revokes credentials only after execution/external stages are done.

The first post-rollout image delete found a second finalizer cycle: the App-owned Ingress and ingress-shim Certificate remained live while TLS Secret cleanup ran, so cert-manager recreated the Secret and held `externalPending=true` through the verifier's 300-second deadline. Finalization now observes an ordered Ingress → Certificate → Secret shutdown. The verifier also recognizes the official CLI's `{service: ...}` list wrapper; its immediate native-repo-delete fixture supplies the CLI-required build and start commands.

`scripts/cli-services-parity-verify.sh` adds image create/update/delete and immediate repo-delete legs. Every explicit delete and EXIT/INT/TERM cleanup waits up to five minutes for both raw GET 404 and absence from `render services`; DELETE acknowledgement alone cannot pass. Its self-test plants stuck-GET, stuck-list, failed-probe, overlong-name, redaction, wrapper-shape, and interrupted-cleanup failures. The final production run passed every baseline leg and all trap cleanup. The original stuck fixture and final `cp-051218-12572-*` fixtures were absent from the API and CLI; 19 cluster artifact classes, per-App credentials, Zot htpasswd/ACL/config, and a builder-authenticated catalog all had zero matching residue. Full sanitized evidence is in [the m49 diagnosis](../../.pm/w5/done/m49/evidence/2026-07-20-diagnosis.md).

## Create flag contract

`POST /v1/services` uses Render's create envelope. “Exact” below means the official CLI exited zero and a subsequent raw `GET /v1/services/{id}` matched the complete asserted value, not merely the command exit status.

| Official CLI flag | Emitted API meaning | Readback / disposition |
| --- | --- | --- |
| `--name` | top-level `name` | Exact. |
| `--type` | top-level `type` | Exact for `web_service`, `cron_job`, `static_site`, and the private-image `background_worker` control. |
| `--runtime` | `serviceDetails.runtime` | Exact for native web and cron services. |
| `--repo` | top-level `repo` | Exact. |
| `--branch` | top-level `branch` | Exact. |
| `--image` | `image.imagePath` plus generated `image.ownerId:""` | Exact in deterministic full-server regression and the final production v2.21.0 run. |
| `--plan` | `serviceDetails.plan` | Exact. |
| `--region` | `serviceDetails.region` | **Limited:** Render OpenAPI and the strict REST decoder accept the declared input, then core placement normalizes readback to the installation's truthful `BEX_REGION` (`fsn1` in production). bex does not pretend the submitted Render region controls placement. |
| `--num-instances` | `serviceDetails.numInstances` | Exact; service readback reports the requested replica count. |
| `--build-command` | native `serviceDetails.envSpecificDetails.buildCommand`, static `serviceDetails.buildCommand` | Exact for web, cron, and static. |
| `--start-command` | `serviceDetails.envSpecificDetails.startCommand` | Exact for native web. |
| `--pre-deploy-command` | `serviceDetails.preDeployCommand` | Exact. |
| `--cron-command` | cron `serviceDetails.envSpecificDetails.startCommand` | Exact in the configured native-cron leg. |
| `--cron-schedule` | `serviceDetails.schedule` | Exact. |
| `--health-check-path` | `serviceDetails.healthCheckPath` | Exact. |
| `--auto-deploy` | top-level `autoDeploy` (`yes`/`no` on read) | Exact in both directions. |
| `--previews` | `serviceDetails.previews.generation` | **Non-goal:** request reaches bex and gets `400 not supported by this platform`; never accepted as a no-op. |
| `--publish-directory` | static `serviceDetails.publishPath` | Exact. |
| `--root-directory` | top-level `rootDir` | Exact for repository services. |
| `--env-var` | create `envVars[]` | Exact in the configured leg; Kubernetes App readback matched the submitted literal without logging it. |
| `--secret-file` | create `secretFiles[]` | Exact in the configured OpenBao leg; authenticated REST readback matched the local file without logging it. |
| `--registry-credential` | `image.registryCredentialId` | Exact in the auth-enabled Zot leg; metadata readback matched the first credential ID. |
| `--ip-allow-list` | `serviceDetails.ipAllowList[]` as `{cidrBlock,description}` | Exact, ordered CIDR **and description**, for web and static. |
| `--build-filter-path` | `buildFilter.paths[]` | Exact. |
| `--build-filter-ignored-path` | `buildFilter.ignoredPaths[]` | Exact. |
| `--maintenance-mode` | `serviceDetails.maintenanceMode.enabled` | Exact on a paid plan. |
| `--maintenance-mode-uri` | `serviceDetails.maintenanceMode.uri` | Exact. |
| `--max-shutdown-delay` | `serviceDetails.maxShutdownDelaySeconds` | Exact. |
| `--environment-id` | top-level `environmentId` | Exact in the full dev-9 flag sweep; assignment uses the existing environment resolver. |
| `--from` | client GET of the source followed by a normal create body | **Limited:** exact clone with explicit `--region frankfurt`; see the client-only guard below for a source whose platform region is outside the CLI enum. |
| `--confirm`, `-o/--output` | client behavior only | Non-interactive confirmation and JSON decoding work; no service field is implied. |

## Update flag contract

`services update` emits `PATCH /v1/services/{id}`. The verifier changes values between create and update, so a dropped PATCH field cannot pass on the original value.

| Official CLI flag | Emitted API meaning | Readback / disposition |
| --- | --- | --- |
| `--name` | top-level `name` | Exact; opaque service ID remains stable. |
| `--plan` | `serviceDetails.plan` | Exact. |
| `--runtime` | no request | **Upstream CLI guard:** v2.21.0 exits with `cannot switch runtimes via the CLI`; bex is not contacted. |
| `--repo` | top-level `repo` | Exact replacement. |
| `--branch` | top-level `branch` | Exact replacement. |
| `--image` | image source update plus generated `image.ownerId:""` | Exact in deterministic full-server regression and production, including a credential-less public-image replacement. |
| `--build-command` | native/static build command | Exact replacement for web, cron, and static. |
| `--start-command` | native start command | Exact replacement. |
| `--pre-deploy-command` | `serviceDetails.preDeployCommand` | Exact replacement. |
| `--cron-command` | cron start command | Exact replacement in the native-cron leg. |
| `--cron-schedule` | `serviceDetails.schedule` | Exact replacement. |
| `--health-check-path` | `serviceDetails.healthCheckPath` | Exact replacement. |
| `--auto-deploy` | top-level `autoDeploy` | Exact replacement. |
| `--previews` | `serviceDetails.previews.generation` | **Non-goal:** bex returns the explicit platform rejection. |
| `--publish-directory` | static `serviceDetails.publishPath` | Exact replacement. |
| `--root-directory` | top-level `rootDir` | Exact replacement for repository services; image-only services reject it explicitly. |
| `--registry-credential` | credential bound to the image source | Exact replacement from credential A to distinct credential B; if PATCH dropped the field, the assertion would retain A and fail. |
| `--ip-allow-list` | replacement `serviceDetails.ipAllowList[]` | Exact ordered replacement, including IPv4, IPv6, and descriptions. |
| `--build-filter-path` | replacement `buildFilter.paths[]` | Exact. |
| `--build-filter-ignored-path` | replacement `buildFilter.ignoredPaths[]` | Exact. |
| `--maintenance-mode` | `serviceDetails.maintenanceMode.enabled` | Exact `true` to `false` replacement. |
| `--maintenance-mode-uri` | `serviceDetails.maintenanceMode.uri` | Exact replacement. |
| `--max-shutdown-delay` | `serviceDetails.maxShutdownDelaySeconds` | Exact replacement. |
| `--confirm`, `-o/--output` | client behavior only | Non-interactive confirmation and JSON decoding work. |

The official service-update command has no clear-allowlist flag. Supplying a new `--ip-allow-list` set replaces the old set; API/GraphQL/MCP/dashboard clear semantics use an explicit empty structured list.

## Captured wire shapes

The baseline was repeated through a local logging proxy that recorded method, path, and body only. Authorization headers were not recorded. Identifiers below are replaced with placeholders; the payload values are the actual non-secret fixtures used by the verifier.

Representative web create:

```json
{
  "autoDeploy": "no",
  "branch": "main",
  "buildFilter": { "ignoredPaths": ["docs/**"], "paths": ["cmd/**"] },
  "name": "<unique-web-name>",
  "ownerId": "<workspace-id>",
  "repo": "https://github.com/render-examples/go-gin.git",
  "rootDir": "cmd/api",
  "serviceDetails": {
    "envSpecificDetails": {
      "buildCommand": "go build ./...",
      "startCommand": "./server"
    },
    "healthCheckPath": "/healthz",
    "ipAllowList": [
      { "cidrBlock": "203.0.113.0/24", "description": "create-office" }
    ],
    "maintenanceMode": {
      "enabled": true,
      "uri": "https://status.example.test/maintenance"
    },
    "maxShutdownDelaySeconds": 41,
    "numInstances": 2,
    "plan": "starter",
    "preDeployCommand": "./server migrate",
    "region": "frankfurt",
    "runtime": "go"
  },
  "type": "web_service"
}
```

Representative web update:

```json
{
  "autoDeploy": "yes",
  "branch": "release",
  "buildFilter": {
    "ignoredPaths": ["examples/**"],
    "paths": ["services/**"]
  },
  "name": "<unique-web-name>-updated",
  "repo": "https://github.com/render-examples/go-echo.git",
  "rootDir": "services/api",
  "serviceDetails": {
    "envSpecificDetails": {
      "buildCommand": "go build ./cmd/...",
      "startCommand": "./api"
    },
    "healthCheckPath": "/ready",
    "ipAllowList": [
      { "cidrBlock": "198.51.100.0/24", "description": "update-office" },
      { "cidrBlock": "2001:db8::/32", "description": "update-v6" }
    ],
    "maintenanceMode": {
      "enabled": false,
      "uri": "https://status.example.test/ready"
    },
    "maxShutdownDelaySeconds": 42,
    "plan": "standard",
    "preDeployCommand": "./api migrate"
  }
}
```

Raw GET readback contained the same ordered allowlist objects. The cron capture placed `./job create`/`./job update` in `serviceDetails.envSpecificDetails.startCommand` and the schedules in `serviceDetails.schedule`. The static capture placed `npm run build` under `serviceDetails.buildCommand`, `dist` under `serviceDetails.publishPath`, and retained its description-carrying allowlist.

## Client-only and explicit-failure boundaries

- `services update --runtime python` exits inside `cli/pkg/types/service/serviceupdate.go`; the proxy recorded no PATCH.
- A bare `services create --from` first GETs the source. When its returned region is `local-capd` (or any value outside the CLI's closed enum), region validation fails before POST. Supplying an explicit official-CLI region makes the clone POST and round-trip normally. bex does not fabricate region data to evade this client behavior.
- Create and update `--previews manual` both reach bex and receive `400 not supported by this platform`; the proxy captured the preview body on both POST and PATCH.
- The configured registry negative control uses `imagePullPolicy: Always` and requires `ErrImagePull`/`ImagePullBackOff` before the positive credentialed leg can pass. This prevents a public or cached image from making the test vacuous.

## Cross-surface allowlist contract

The shared semantic value is an ordered list of `(CIDR, optional description)` entries. REST uses Render's `{cidrBlock,description}` names. GraphQL and MCP retain their legacy flat-CIDR aliases and add structured inputs/outputs; passing both aliases is an error rather than silent precedence. The dashboard editor reads and writes structured entries. Blueprint descriptions use the same Core request. The App CR retains legacy `spec.ipAllowList` and adds `spec.ipAllowListEntries`; structured entries win when present, while a flat-only legacy App synthesizes empty descriptions. The operator projects only effective CIDRs to Traefik, so descriptions never affect reachability.
