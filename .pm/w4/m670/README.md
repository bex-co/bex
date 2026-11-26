# w4 · m670 — The Add credential dialog is Render's, and opens in place from the service form's Credential dropdown

**Worker:** worker4 **Goal:** a person deploying a private image adds its registry credential from the Credential dropdown without leaving the half-filled form, through Render's dialog (Name, Registry, Username, Personal Access Token), and the new credential is selected for them. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Add credential dialog and its dropdown entry from Render's public dashboard bundle | 30m | — |
| [t002](t002.md) | The dialog asks for Render's fields, and GraphQL `createRegistryCredential` takes `registry` as REST does | 1h30m | t001 |
| [t003](t003.md) | Every Credential dropdown offers Add credential, opens the dialog in place and selects the new credential | 1h | t002 |
| [t004](t004.md) | dev-4 and production: add a credential from the create page's dropdown, deploy with it, then delete it | 30m | t003 |
| [t005](t005.md) | Render parity | 25m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
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
