# Registry credential service binding

Captured 2026-07-15 from Render's current [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json) and the unmodified official [`render-oss/cli`](https://github.com/render-oss/cli) at `72b3fbd59068ae84d024ec2ded9df6b27dc8dd68`.

## Render contract

- A prebuilt-image create/update carries `image: {imagePath, registryCredentialId}`. The `image` schema requires `imagePath` and `ownerId`; `registryCredentialId` is an optional, non-nullable string. Its owner must match the image/service owner.
- A repository Docker build carries the same optional string under `serviceDetails.envSpecificDetails.registryCredentialId` (`dockerDetailsPOST` / `dockerDetailsPATCH`).
- A service read reports `registryCredential: {id,name}` plus `imagePath`; the id is an input field, not a top-level Render service-output field.
- The public schema does not define a clear operation. The official CLI trims an empty `--registry-credential` to omission, so it can set/change but cannot express clear. bex defines explicit `""` as clear consistently on REST, GraphQL, and MCP; omission preserves existing intent.

The official CLI's `BuildCreateRequest` puts `--registry-credential` in `image.registryCredentialId` whenever `--image` is supplied. For Docker runtimes it instead builds the nested `dockerDetailsPOST` form. Update follows the same split and requires `--registry-credential` to be paired with `--image` unless the existing runtime is Docker.

## bex coverage after w6/m34

| Surface | Create | Set/change/clear |
| --- | --- | --- |
| REST | Prebuilt images use `image.registryCredentialId`; repository Docker builds use Render's `serviceDetails.envSpecificDetails.registryCredentialId` | `PATCH /v1/services/{id}` accepts the matching source-specific object; empty clears. Reads echo the Docker-build id under `envSpecificDetails` and include Render's `registryCredential: {id,name}` summary plus bex's top-level id extension |
| GraphQL | `createService(registryCredentialId:)` for either source context | `setRegistryCredential(id:, registryCredentialId:)` |
| MCP | `create_web_service` / `create_cron_job` argument for either source context | `set_registry_credential` |
| Dashboard | Shared credential picker for Existing Image and Git-backed Docker runtime, including a credential-settings link when the workspace list is empty | Eligible existing-image and repository-Docker service settings show the current binding and can change or clear it; the service header resolves the bound credential's human name and links back to credential settings. “No credential — public image” sends explicit empty/no-auth rather than omission/legacy host matching |

Every adapter reaches the same App verb. Unknown id is 404; an existing credential in another workspace is 403; image-host mismatch is 400. Native/buildpack/static repository sources reject the field with a named 400. The exact id is persisted in Postgres and `App.spec.registryCredentialId`, while `App.spec.externalRegistryPullSecret` points at the materialized `kubernetes.io/dockerconfigjson` Secret.

For a Dockerfile build, the operator copies the explicitly bound external-registry credential into a deterministic build-namespace Secret used only by BuildKit to resolve private base images. The platform output credential is a different, per-App repository credential mounted only into the later Skopeo push and optional Cosign phases. BuildKit exports an OCI archive and never receives output-repository write auth or the signing key; there is no same-host credential merge. Neither credential becomes a build arg, BuildKit `--secret`, tenant-step environment variable, or tenant-visible mount. Derived Secrets are removed by the App finalizer. An unset binding never guesses a credential from an unknown `FROM` host.

The w2/m59 closure was verified live on 2026-07-19 with private Git, a private base image, and an adversarial Dockerfile. The resulting signed image reached `Running`; the 38/38 matrix also proved that the per-App output credential could access its own repository but not the private-base repository, and that unsigned and post-signature-tampered images were denied. See [`verify-build-isolation.sh`](../../scripts/verify-build-isolation.sh) and [ADR039](../ADR039-operator-audit-and-platform-reuse.md).

This closes the repository-Docker divergence recorded by w6/m31. bex still accepts arbitrary registry hosts instead of Render's closed provider enum.

## Docker-build live proof

The historical pre-m59 2026-07-15 CAPD app-cluster run used an authenticated `registry:2` fixture and a repository whose first Dockerfile line referenced a private base image in that registry. With the bound credential, BuildKit logged credential sharing, resolved the private manifest, pushed `m34-positive:gen-4`, and the generated workload reached `Running`. The same source with an intentionally wrong bound credential failed its manifest `HEAD` with `401 Unauthorized`. The then-current generated Job referenced only `bld-m34-positive-registry-auth` at `/docker-config`; its daemon config also selected the platform's already-declared plain-HTTP development registry for source resolution. The Apps, Secrets, registry namespace, host processes, and local image tag were removed after the check. This remains contract evidence but was superseded as build-boundary evidence by the phase-separated 2026-07-19 proof above.

## Dashboard live proof

The 2026-07-15 dev-5 run exercised the shipped dashboard against the live GraphQL and OpenBao-backed credential services. An Existing Image create selected credential A, and a fresh `server(id)` query returned A. Service Settings then changed the binding to credential B; another fresh query returned B, and the read-only service header resolved B's human name. Selecting “No credential — public image” produced the clear-success state, a fresh query returned the explicit empty string, and the header fact disappeared. The temporary service, both credential records, their OpenBao secrets, and the Kratos identity were removed after the check. The authenticated private-image pull itself remains covered by the official-CLI leg below, while the repository-Docker private-base path is covered by the BuildKit proof above.

## Reproducible official-CLI leg

`scripts/cli-compat.sh registry-credential-verify` drives the unmodified CLI against a live bex-api. It first launches an anonymous, `imagePullPolicy: Always` control Pod and requires `ErrImagePull`/`ImagePullBackOff`, proving the supplied image is private. It then creates a throwaway credential, runs `services create --image … --registry-credential …`, asserts the canonical REST summary, durable App intent, docker-config Secret, Deployment `imagePullSecrets`, kubelet `Pulled` event, and running Pod, then cleans up every resource on exit.

The final 2026-07-15 dev-6 image-backed run used CLI `72b3fbd` and an auth-enabled `registry:2` instance. Anonymous `/v2/`, manifest, direct-node, and control-Pod pulls were all refused; the exact credential then produced service `srv-d9c38chjg4r0f1h0jq2g`, the kubelet pulled the same private image, and the App reached `Running`. The run also proved canonical `GET /v1/registrycredentials/{id}` lookup, `registryCredential: {id,name}` readback, Postgres/CR intent, deterministic `kubernetes.io/dockerconfigjson` Secret binding, Deployment wiring, and zero service/credential/App/probe residue after cleanup. The repository-Docker CLI request form is pinned by the REST adapter tests; its build-plane behavior is covered by the separate live proof above.
