# w1 · m673 — The image URL is checked against its registry before the service is created, as Render's create form does

**Worker:** worker1 **Goal:** while a user types an image URL, in the create form and in Settings' image source, bex reads the image from its registry with the chosen credential, as Render's `isDeployableImage` does. Each of Render's outcomes is shown under the field before the service exists: not found or private, invalid URL, credential refused, wrong platform, too large, or reachable. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's image check and settle the details the bundle leaves open | 45m | — |
| [t002](t002.md) | Backend: read an image from its registry behind the SSRF guard and classify it into Render's five failure codes | 2h | t001 |
| [t003](t003.md) | GraphQL `isDeployableImage(externalImageReference)` with its key scope and authorization | 1h15m | t002 |
| [t004](t004.md) | Dashboard: the image tab checks the URL as it is typed, in the create form and in Settings | 2h | t003 |
| [t005](t005.md) | Verify in a real browser and over GraphQL, on dev-1 and on production | 1h | t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 8h40m**.

## Definition of done

1. GraphQL `isDeployableImage(externalImageReference: { imageUrl, registryCredentialId })` answers `{ deployable, failureCode, failureReason }`. `failureCode` is one of Render's `does_not_exist`, `image_too_large`, `invalid_input_url`, `invalid_registry_credential`, `unsupported_platform`, or null when the image is deployable. Each code is covered by a backend test against an in-process registry, with and without a credential.
2. The platform rule follows bex's hosting node pool. bex-api checks the same platform the operator resolves (`BEX_IMAGE_PLATFORM`, unset = `linux/amd64` on production). A multi-platform index with an entry for that platform is deployable, and a single-platform image for another architecture is `unsupported_platform`.
3. The read cannot be turned against bex. Every dial goes through `netutil.SafeDialContext`, so loopback, private and link-local hosts are refused, through redirects and token realms too. A credential's secret is sent only to its own registry host. Manifest and config bodies are capped, the read has a timeout, and one workspace cannot run more than a few checks at once. A caller can use only a credential of a workspace it can read, and the key scope is `KeyWorkspaceRegistry`, as for `registryCredentials`.
4. In the dashboard, the image tab of the create form and of Settings' source card checks the URL 800 ms after typing stops, and at once on paste. The field shows a spinner while it checks, then a check mark or a caution icon. Each failure shows Render's message, and success shows "This is a public image that can be accessed without credentials." or "We were able to authorize access to this image with the selected credential.". The strings are in all eight locales. Create and Save stay disabled until the image is deployable. The pending field and its message line hold the ready state's height at 1440 and 390 px.
5. Walked on dev-1 and on production as the QA user in `bex-canary`, with disposable services. Walk a public image, a private image with and without its credential, a wrong credential, a malformed URL, a missing tag and an image for the wrong platform. Do this on the create form and in Settings.

## What Render does

Read 2026-11-26 from Render's public dashboard bundle (the entry chunk; find the code by the operation name and strings below, since chunk hashes change every deploy) and from render.com/docs. Not walked signed in.

- **The operation.** `query isDeployableImage($externalImageReference: ExternalImageReferenceInput!) { isDeployableImage(externalImageReference: $externalImageReference) { deployable failureCode failureReason } }`. Both call sites send `{ registryCredentialId: <id> | null, imageUrl: <the parsed image path> }`, with Apollo's `network-only` policy. The failure-code enum has `does_not_exist`, `image_too_large`, `invalid_input_url`, `invalid_registry_credential` and `unsupported_platform`. The operation is in no feature-flag check, and the ledger row is `gap`.
- **Docs.** [deploying-an-image](https://render.com/docs/deploying-an-image) says "Render verifies that it can access the image using any credentials you provide.", "The Docker image must be built for the `linux/amd64` platform." and "The Docker image's compressed size cannot exceed 10 GB.". It also says "Verify your image URL in the Image section of your service's Settings page.".
- **When it runs.** The field's change handler debounces 800 ms, and a paste runs at once. An empty field resets to idle. A path the client cannot parse shows "Please supply the full name of the image." and sends no query. Changing the credential runs the check again. While the check is in flight the field shows a spinner (`loading`). After it, the field shows a check icon (`success`) or a caution icon (`error`).
- **Messages** (the client maps `failureCode`; `failureReason` from the server is the fallback, then "Invalid image path or credential. Please try again."; with no code, "An error occurred."):
  - `does_not_exist`: "No public image found. It looks like you may be using a private image - please add a credential or check the image URL for mistakes.", or with a credential, "The provided image URL and registry credential are invalid."
  - `invalid_input_url`: "The provided image URL is invalid."
  - `invalid_registry_credential`: "The provided registry credential is invalid."
  - `unsupported_platform`: "The provided image URL points to an image with an invalid platform. Images must be built with the platform linux/amd64.", with "linux/amd64." linking to `/docs/deploying-an-image#linux-amd64-platform`.
  - `image_too_large`: "This Docker image's compressed size is larger than the allowed limit.", with "allowed limit." linking to `/docs/deploying-an-image#image-size`.
  - deployable: "This is a public image that can be accessed without credentials.", or with a credential, "We were able to authorize access to this image with the selected credential."
- **The form around it.** Two components use the check. One is the inline connect dialog of the create flow: "Image URL", helper "Deploy an image from a Docker registry", placeholder `docker.io/library/nginx:latest`. The other is the create/update form, where the Settings image source counts as an update when it has an initial URL. It has the heading "Image URL", "The image URL for your external image.", and "Credential (Optional)" with "If your service pulls private Docker images from a registry, specify a credential that can access those images. Manage your credentials in Settings.". The Connect button is disabled until `deployable` is true. The credential select is marked in error when the check fails with a credential chosen. It is disabled while the image is a deployable public image with no credential chosen (t001 confirms this reading). The field's icon reflects the registry the path names.

## bex today

Verified 2026-11-26 on main `ff791487b5`.

- **Dashboard.** The create page (`dashboard/src/features/services/pages/new-service-page.tsx`) and Settings' source card (`dashboard/src/features/services/components/service-source-card.tsx`) share `ServiceSourcePicker`'s image tab (`service-source-picker.tsx`) with `RegistryCredentialSelect`. The tab checks only syntax, length and private hosts (`dockerImageProblem` in `dashboard/src/common/lib/utils/docker-image.ts`). The server's refusal on submit is the only other message (`serverError`, w4/406).
- **Backend.** OCI reference syntax is validated on create, source update, Blueprint validation and deploy overrides (w8/m49; ADR018 § Services & lifecycle). Nothing reads a registry before create. The registry readers that exist are the sandbox image import in `lego/backend/internal/sandbox/images.go`, the operator's metadata read in `lego/operator/internal/registry/image_config.go`, and GitHub's public-commit resolve. The sandbox import uses go-containerregistry over `netutil.SafeDialContext`, with a credential, `linux/amd64`, capped manifests and a 10 GB cap. The operator's read runs at deploy, honors `BEX_IMAGE_PLATFORM`, and maps registry refusals to the deploy reasons `image_not_found`, `image_credential_refused` and `image_access_refused`. The backend cannot import the operator (`operator → types ← backend`).
- **Ledger.** The `isDeployableImage` row in [dashboard-operations.md](../../../docs/render-artifacts/dashboard-operations.md#services) is `gap` and names this milestone.

## Source + Goal linkage

- **Source:** [w1/153](../done/153.md), found by the w1/m645 dashboard ledger, 2026-11-26.
- **Goal linkage:** Render parity, the top product principle ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)). Deploying a prebuilt image is a core hosting path ([ADR004](../../../docs/ADR004-app-deployment.md)).
- **Expected outcome:** a mistyped tag, a private image without its credential, a wrong token or an arm64-only image is caught in the form in about a second. Today it is a service that exists and fails its first deploy.
- **Why now:** the w1/m645 ledger left this as one of its last `gap` rows in Services, and w4/734 (refuse an image over 10 GB at deploy) shares the size rule. The pre-create check is where Render shows that rule first.
- **Render parity task included:** the milestone adds a GraphQL read and changes the create form and Settings. Render has no REST or MCP form of the check: it is not in its pinned OpenAPI spec or in its MCP tools. So t006 checks GraphQL and the dashboard, and records REST and MCP as `—`, as on Render.
