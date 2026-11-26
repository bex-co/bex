# w4 · m670 — The Add credential dialog is Render's, and opens in place from the service form's Credential dropdown

**Worker:** worker4 **Goal:** a person deploying a private image adds its registry credential from the Credential dropdown without leaving the half-filled form, through Render's dialog (Name, Registry, Username, Personal Access Token), and the new credential is selected for them. **Status:** todo (t001–t003, t005–t007 done; t004 dev-4 half done; production half waits for the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Add credential dialog and its dropdown entry from Render's public dashboard bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | The dialog asks for Render's fields, and GraphQL `createRegistryCredential` takes `registry` as REST does — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Every Credential dropdown offers Add credential, opens the dialog in place and selects the new credential — **DONE** | 1h | t002 |
| [t004](t004.md) | dev-4 and production: add a credential from the create page's dropdown, deploy with it, then delete it | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 25m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 4h50m**.

## Definition of done

- On the create page's Existing Image source, in Settings → Image and on the Build Source create and settings pages, the Credential dropdown has Add credential. It opens the dialog in place; on success the new credential is selected and nothing else in the form changes.
- The dialog asks for Name, Registry (Docker Hub, GitHub Container Registry, GitLab Container Registry, Google Artifact Registry, AWS ECR), Username and Personal Access Token, with Render's labels, order and help link as t001 records them. Google Artifact Registry and ECR also ask for the registry host, the `host` extension ADR018 already records for them.
- GraphQL `createRegistryCredential` accepts `registry` and resolves it with the same code as REST's create (`createHost`, `lego/backend/internal/registrycreds/registry.go`).
- A credential created through the API for any other host still lists, binds, edits and deletes as before (w2/m14).
- All 8 dashboard languages carry the new strings.
- On production, in a first-party fixture workspace, a credential added from the create page's dropdown is selected, a private image deploys with it, and the service and the credential are deleted afterwards.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: deploying a prebuilt image, as [render.com/docs/deploying-an-image](https://render.com/docs/deploying-an-image) describes), item 3. Re-checked by hand at `origin/main` `28b4f18bbe` before filing: no open, blocked or done item on any workstream owns it. The in-place GitHub Credentials menu (w8/m31) covers repository sources only.
- **Render:** create step 4, "If the image is private, select or add a *Credential*"; "The *Credential* dropdown includes any _existing_ credentials … If you click *Add credential*, the following dialog appears", with the fields Name, Registry, Username and Personal Access Token ("Learn how to generate a personal access token"; Docker Hub alone also takes a password).
- **bex today:**
  - `RegistryCredentialSelect` (`dashboard/src/features/services/components/registry-credential-select.tsx`) lists None and the existing credentials. Its only way to add one is the Manage link to `/workspace/settings#registry-credentials` (`:87-93`), which leaves the form. It is used on the create page (`service-source-picker.tsx:785`), in Settings → Image (`registry-credential-section.tsx:51`) and on the Build Source pages (`create-build-source-page.tsx:594`, `build-source-settings-tab.tsx:488`, `:590`).
  - The dialog (`dashboard/src/features/registry-credentials/components/create-registry-credential-dialog.tsx:109-187`) is opened only from Workspace Settings (`registry-credentials-panel.tsx:58`). It asks for a free-text Registry host, Username, "Password or access token" and an optional Name, in that order.
  - REST create already takes Render's `registry` enum (w4/420, `lego/backend/internal/registrycreds/registry.go:31`); GraphQL `createRegistryCredential` takes only `host` (`lego/backend/internal/registrycreds/graphql.go:77`).
- **Recommendation, not a user decision:** the dialog offers Render's five registries only. A self-hosted registry stays available through REST, GraphQL and MCP, and its credentials list and edit as before. If t001 finds that Render's dialog accepts other registries, follow Render.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a private image service is set up from one page, as on Render, instead of abandoning the form for Workspace Settings and starting again.
- **Why now:** deploying a private image is the path new users hit right after a public one, and w4/m669 and w1/153 make its failures explicit. This makes the fix one click away.
- **Render parity:** included: GraphQL and the dashboard change, and REST, MCP and the dashboard must keep agreeing.

## Record — 2026-11-26

- **t001.** Render's dropdown and dialog, read from its public dashboard bundle (`index-D5V4ZgbE.js`, 459 chunks): [registry-credential-dialog.md](../../../docs/render-artifacts/registry-credential-dialog.md). Add credential is the dropdown's first item, with a separator; the dialog asks for Name (required), Registry (GitHub, GitLab, Docker, Google Artifact, AWS ECR), per-registry help with prefilled token links, Username and Personal Access Token ("or Password" for Docker), and sends `registryType` with no host; the caller selects the new credential. The Manage link stays in the help text. Render's image forms also filter the list by the image's registry: not taken here, filed as [w4/735](../735.md).
- **t002.** GraphQL `createRegistryCredential(registry:)` (host now optional) and MCP `create_registry_credential {registry}` resolve through REST's `createHost`, with its refusals; GraphQL `RegistryCredential.registry` reads the enum back, empty for a self-hosted host as REST omits it (`TestGraphQLCreateTakesRendersRegistryAsRESTDoes`, `TestMCP_CreateRegistryCredentialTakesRendersRegistry`, schema golden updated). The dialog is rebuilt in Render's order and words; Google Artifact and AWS ECR also ask for the Registry host, marked in place when it belongs to another registry (`hostForRegistryError`, bex-api's `renderRegistry` in `lib/registries.ts`). The Edit dialog follows Render's Update Credential order with the registry read only; a self-hosted host shows as itself and still saves. All 8 languages carry the strings.
- **t003.** `RegistryCredentialSelect` starts with Add credential for `can_manage` members, so the create page (Existing Image and Docker build), Settings → Image and both Build Source pages gain it at once. It opens the dialog over the page (focus lands in Name, the list does not take it back), and on success refetches and selects the new credential; cancelling keeps the selection. Tests: the create page keeps the image URL, name and plan and submits the new id; Settings → Image saves it; a member without the permission sees no entry.
- **t004, dev-4 half.** bex-api rebuilt from this tree (`dev-env.sh 4 up`), with a dev-mode OpenBao stood up in `dev-4-auth` for the walk (registry credentials need one; the agent leg that normally brings it is held by dev-9), a disposable verified identity and its workspace (disposable), and a headless browser of the run's own. On the create page, Existing Image `docker.io/library/nginx:alpine`, name `m670-walk-web` (disposable): Add credential opened the dialog with focus in Name; a Docker credential `m670 walk Docker` (disposable) with a dummy token was created and selected, and the image URL and name were unchanged, at 1440 and 390 px. Deploying created the service; REST read `registryCredentialId` and `registryCredential {id, name}` as the new credential with `registry: DOCKER`, and the deploy failed with "docker.io refused this service's registry credential … (HTTP 401)", so the selected credential was the one pulled with (no real token was used). The service, the three credentials the walks made, the identity and the OpenBao were deleted, and bex-api was restarted without it.
- **t005.** REST, GraphQL and MCP create the same credential from the same `registry`/`host` and refuse the same pairs; the dashboard sends GraphQL's. ADR018's Registry credentials row records the dialog, the dropdown and the two kept differences (the explicit "No credential" item, the Edit dialog's keep-the-token field); the registry filter is w4/735.
- **t006.** `/simplify` over the change found nothing to restructure (the list and detail reads already share one `toCredentialView` from t002). The review caught a reformat of three untouched dashboard files by a broad prettier run, which was reverted.
- **t007.** Reverting each guarded line failed its test: the GraphQL and MCP `createHost` calls and GraphQL's null `registry`; the select's sentinel guard, focus hold, permission gate and selection of the new id; the dialog's required name, host requirement, host sent for Google Artifact and ECR, Docker label, `onCreated`, and the GitHub link's prefilled name; the Edit dialog's registry display; the mismatch check and the `-docker.pkg.dev` match; the view's registry mapping; the hook's `registry` variable and its no-id failure.
