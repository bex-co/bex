# Render's Add Credential dialog and the Credential dropdown (w4/m670 t001)

**Read 2026-11-26** without sign-in from Render's public dashboard bundle: `https://dashboard.render.com/` loads `assets/index-D5V4ZgbE.js`, and following its imports fetches 459 chunks (the method of [dashboard-operations.md](dashboard-operations.md)). The dropdown and the dialog are components in `index-D5V4ZgbE.js`: find the dropdown by its `add-credential` item id and the dialog by its `Update Credential` / `Add Credential` title. The registry labels and the image-to-registry parser are in `RegistryIcon-BGh3RHbE.js`. The hashes change with every Render deploy. Nothing was submitted. Render's docs say the same in prose: [deploying-an-image § Credentials for private images](https://render.com/docs/deploying-an-image).

## The Credential dropdown

One component serves every page that picks a credential:

| Page | Label | Help text |
| --- | --- | --- |
| New service, Existing Image (inline and stepped forms) | **Credential (Optional)** | "If your service pulls private Docker images from a registry, specify a credential that can access those images. Manage your credentials in Settings." (stepped form) |
| New service, Advanced, a Docker build from a repository | **Registry Credential** | the same two sentences |
| Service Settings (image service, or Docker build) | **Registry Credential** | the same, behind Edit / Save Changes |
| Service Settings, the **Update Image** dialog of an image service | **Credential (Optional)** | the stepped form's two sentences: the dialog is that form's component, with the current image filled in |
| Build Source settings | **Registry Credential** | "If your build pulls private Docker images from a registry, …" |

- Its items, in order: **Add credential** (a New icon, an action item, with a separator after it), then the owner's credentials by name, each with its registry's icon.
- With nothing chosen it reads **No credential**, and a clear button returns to that.
- On the image forms (New service's Existing Image and the Update Image dialog) the list holds only the credentials whose registry matches the image URL's registry: `credentials.filter(c => !type || c.registryType === type)`, where `type` is `parsedRegistryType`. The parser takes the first path segment: `*pkg.dev*` is Google Artifact, `*dkr.ecr*` AWS ECR, `docker.io` or `*.docker.io` Docker, `ghcr.io` GitHub, any host containing `gitlab` GitLab. A name with no host (`nginx`, `acme/web`) is Docker. An image on any other host, and a blank field, show every credential. The other dropdowns pass no type. Re-read 2026-11-27 (w4/735): the same bundle hashes, and the parser was run from the chunk on the vectors `registries.test.ts` pins.
- The dropdown has no permission check of its own. The Manage link stays, as the "Manage your credentials in Settings." sentence in the help text.
- Choosing Add credential leaves the selection as it was and opens the dialog over the page. On success the caller sets the new credential's id as the selection (`onCompleted: e => e.createRegistryCredential && setId(e.createRegistryCredential.id)`), and the mutation's cache update puts the new credential first in the list. Nothing else in the form changes.

## The dialog

Title **Add Credential**. Description "Learn more about storing external [registry credentials.](https://render.com/docs/deploying-an-image#managing-credentials)". Fields, in order:

| # | Field | Render's copy and rules |
| --- | --- | --- |
| 1 | **Name** | placeholder `My credential`; required ("Required"), at most 100 characters, autofocused |
| 2 | **Registry** | a select, placeholder "Select a registry"; options in this order with these labels: GitHub (`GITHUB`), GitLab (`GITLAB`), Docker (`DOCKER`), Google Artifact (`GOOGLE_ARTIFACT`), AWS ECR (`AWS_ECR`); required ("Registry is required"). No other registry can be chosen. |
| — | per-registry help, under the select once a registry is chosen | GitHub: "You'll need to create a personal access token with the required `read:packages` permission in GitHub **Settings > Developer Settings**.", linking `https://github.com/settings/tokens/new?description=<name>&scopes=read:packages`. GitLab: "… with the required `read registry` permission in GitLab by navigating to **Edit Profile > Access Tokens**.", linking `https://gitlab.com/-/profile/personal_access_tokens?name=<name>&scopes=read_registry`. Docker: "Enter your Docker Hub username and password, or create an access token with `read` access permission to use in place of your password by visiting Docker Hub **Account settings > Security**.", linking `https://hub.docker.com/settings/security?generateToken=true`. Google Artifact: "You'll need to create a service account with the required `roles/artifactregistry.reader` permission in Google Cloud.", linking the console's create-service-account page. AWS ECR: "The Username is the AWS Account ID that owns the ECR repository. The Personal Access Token will come from running `aws ecr get-login-password` command. Refer to the AWS documentation for more information." |
| 3 | **Username** | required |
| 4 | **Personal Access Token** (**Personal Access Token or Password** for Docker) | a secret field, required |

- Buttons: **Cancel** and **Add Credential** (busy while the mutation runs).
- A server error shows under the fields. "already in use" marks Name: "The name _X_ is already in use. Please use a different name."
- It sends `createRegistryCredential(registryCredential: {name, registryType, username, authToken, ownerId})`: the registry and **no host**. Render's credential names a registry, not a host, so Google Artifact and AWS ECR ask for no host there.
- The edit form is the same dialog titled **Update Credential**, with the Registry shown read only (its icon and label) and no help link.
- Workspace Settings' Container Registry Credentials card opens a dialog with the same props (`ownerId`, `isOpen`, `closeModal`) from its **Add Credential** button.

## What bex takes from it (w4/m670)

- The dialog asks for Name, Registry, Username and Personal Access Token in Render's order, with Render's labels, its per-registry help and links, and its submit and cancel labels. Name is required, as Render requires it, though the API keeps defaulting a missing name to the host.
- bex stores a registry host for the pull secret. The three registries with one host send `registry` alone. Google Artifact and AWS ECR also ask for the **Registry host**, the `host` extension [ADR018](../ADR018/deployment-sources-iac.md) records for them, and send both. GraphQL `createRegistryCredential(registry:)` resolves them with REST's code.
- The help link goes to bex's own page, [bex.co/docs/platform/docker-deploys#use-a-private-registry](https://bex.co/docs/platform/docker-deploys#use-a-private-registry).
- Every Credential dropdown starts with Add credential and a separator. Choosing it opens the dialog in place, and the new credential is selected. bex keeps its explicit "No credential — public image" item, because its select has no clear button.
- A self-hosted registry (any other host) can still be added through REST, GraphQL and MCP, and its credentials list, bind, edit and delete as before. The Edit dialog shows its host as the registry.

## What bex takes from it (w4/735)

- The create page's Existing Image tab and Settings' Update Source image tab (Render's Update Image dialog) list only the credentials for the image's registry, by a port of Render's parser. Settings' Image registry credential card, the Docker build's dropdown and the Build Source's dropdowns list every credential, as Render's Settings, Docker-build and Build Source settings dropdowns do.
- A credential for a self-hosted host, which Render's enum cannot name, is listed only for an image on that host, or wherever every credential is listed.
- The chosen credential stays listed after the image moves to another registry, so the trigger still names it.
