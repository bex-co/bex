# Registry credential service binding

Captured 2026-07-15 from Render's current [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json) and the unmodified official [`render-oss/cli`](https://github.com/render-oss/cli) at `72b3fbd59068ae84d024ec2ded9df6b27dc8dd68`.

## Render contract

- A prebuilt-image create/update carries `image: {imagePath, registryCredentialId}`. The `image` schema requires `imagePath` and `ownerId`; `registryCredentialId` is an optional, non-nullable string. Its owner must match the image/service owner.
- A repository Docker build carries the same optional string under `serviceDetails.envSpecificDetails.registryCredentialId` (`dockerDetailsPOST` / `dockerDetailsPATCH`).
- A service read reports `registryCredential: {id,name}` plus `imagePath`; the id is an input field, not a top-level Render service-output field.
- The public schema does not define a clear operation. The official CLI trims an empty `--registry-credential` to omission, so it can set/change but cannot express clear. bex defines explicit `""` as clear consistently on REST, GraphQL, and MCP; omission preserves existing intent.

The official CLI's `BuildCreateRequest` puts `--registry-credential` in `image.registryCredentialId` whenever `--image` is supplied. For Docker runtimes it instead builds the nested `dockerDetailsPOST` form. Update follows the same split and requires `--registry-credential` to be paired with `--image` unless the existing runtime is Docker.

## bex coverage after w6/m31

| Surface | Create | Set/change/clear |
| --- | --- | --- |
| REST | `POST /v1/services` → `image.registryCredentialId` | `PATCH /v1/services/{id}` → same object; empty clears. Reads include Render's `registryCredential: {id,name}` summary plus bex's top-level id extension. Credential lookup/CRUD uses canonical `/v1/registrycredentials` plus the older hyphenated alias |
| GraphQL | `createService(registryCredentialId:)` | `setRegistryCredential(id:, registryCredentialId:)` |
| MCP | `create_web_service` / `create_cron_job` argument | `set_registry_credential` |
| Dashboard | Existing Image credential picker | Create flow; “None (public image)” sends explicit empty/no-auth rather than omission/legacy host matching. Settings still manages credential records |

Every adapter reaches the same App verb. Unknown id is 404; an existing credential in another workspace is 403; image-host mismatch is 400. The exact id is persisted in Postgres and `App.spec.registryCredentialId`, while `App.spec.externalRegistryPullSecret` points at the materialized `kubernetes.io/dockerconfigjson` Secret.

## Recorded divergence

bex rejects a registry credential on a repository-backed Docker service with a named 400. Supporting Render's nested Docker-build form requires a separate BuildKit authentication boundary for private base images; silently accepting it would claim authentication the build Job cannot perform. The follow-up is `.pm/w6/017.md`.

## Reproducible official-CLI leg

`scripts/cli-compat.sh registry-credential-verify` drives the unmodified CLI against a live bex-api. It first launches an anonymous, `imagePullPolicy: Always` control Pod and requires `ErrImagePull`/`ImagePullBackOff`, proving the supplied image is private. It then creates a throwaway credential, runs `services create --image … --registry-credential …`, asserts the canonical REST summary, durable App intent, docker-config Secret, Deployment `imagePullSecrets`, kubelet `Pulled` event, and running Pod, then cleans up every resource on exit.

The final 2026-07-15 dev-6 run used CLI `72b3fbd` and an auth-enabled `registry:2` instance. Anonymous `/v2/`, manifest, direct-node, and control-Pod pulls were all refused; the exact credential then produced service `srv-d9c38chjg4r0f1h0jq2g`, the kubelet pulled the same private image, and the App reached `Running`. The run also proved canonical `GET /v1/registrycredentials/{id}` lookup, `registryCredential: {id,name}` readback, Postgres/CR intent, deterministic `kubernetes.io/dockerconfigjson` Secret binding, Deployment wiring, and zero service/credential/App/probe residue after cleanup.
