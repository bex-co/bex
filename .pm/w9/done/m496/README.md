# w9 · m496 — An API key carries a chosen subset of its creator's permissions, as Render's scoped keys do

**Worker:** worker9 **Goal:** a bex API key is minted with a chosen set of Render's per-area permissions, never more than its creator's current role allows; every request outside that set is refused on REST, GraphQL and MCP even when the creator holds it; the set can be changed after minting; and the dashboard's API keys page offers Render's permission picker with its All, Read only and Clear all presets. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: Render's permission catalog, how a key's grant is stored and enforced, what keys minted before it get; ADR024, ADR012 §8e and ADR018 amended — **DONE** | 1h | — |
| [t002](done/t002.md) | Catalog and storage: the permission catalog in `core`, a key's grant persisted at mint and read back — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Operation map, hosting areas: Services, Deploys, Monitoring, Postgres, Key Value, Environment groups, Projects, Blueprints — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | Operation map, the rest: Workspaces, Sandboxes, Build sources, Object storage, Audit logs and bex's own areas; a guard test fails on any unmapped operation — **DONE** | 1h30m | t003 |
| [t005](done/t005.md) | Enforce: a request outside the key's grant is refused on every surface and side entry point, ANDed with the creator ceiling — **DONE** | 1h30m | t004 |
| [t006](done/t006.md) | Mint and edit: `permissions` on REST, GraphQL and MCP create, a change-permissions verb, and a grant above the creator's role refused — **DONE** | 1h15m | t005 |
| [t007](done/t007.md) | Dashboard: Render's permission picker on create and on a gear-icon edit, presets, role-disabled rows and a grant summary per key — **DONE** | 2h | t006 |
| [t008](done/t008.md) | Security review of the scoped-key boundary, recorded in the security lineage — **DONE** | 45m | t005, t006 |
| [t009](done/t009.md) | Live check on dev-9 and production; Render's Terraform provider rerun with an All key — **DONE** | 1h30m | t007, t008 |
| [t010](done/t010.md) | Render parity — **DONE** | 30m | t009 |
| [t011](done/t011.md) | Simplify — **DONE** | 15m | t010 |
| [t012](done/t012.md) | Test coverage — **DONE** | 45m | t010, t011 |
| [t013](done/t013.md) | Closeout — **DONE** | 10m | t012 |

Total: **13 tasks, about 13h**.

## Security boundary (holds for every task)

- **m481's I1–I5 still hold** ([ADR024 § An API key acts with its creator's role](../../../../docs/ADR024-members.md#an-api-key-acts-with-its-creators-role-w9m481)): never more than the creator now, its own workspace only, no recorded creator no expansion, keys still do not mint credentials, member verbs still refuse key subjects.
- **I6: the grant can only refuse.** A key's decision is its grant AND its creator's current decision at the same freshness. No permission in a grant can lift a key above its creator or past I4 and I5.
- **I7: fail closed.** An operation with no mapped permission is refused to a key whose grant is not All, and an unreadable grant refuses the request (503, as an unreadable creator does), never falls back to the creator's role.
- **I8: a narrowing applies on the key's next request.** Editing a key's grant is a human-only verb (a key cannot widen its own or another key's grant), and the enforcement cache never keeps a removed permission longer than the creator ceiling keeps a removed role.

## Definition of done

- **On dev-9, a key minted with Services Read only** lists and reads services on REST, GraphQL and MCP, and is refused with Render's 403 shape when it creates or deletes a service, sets an environment variable or triggers a deploy. A key with Deploys Run and Services Read triggers and cancels a deploy and still cannot create a service.
- **The role caps the grant:** a Developer's mint naming "Create, update, and delete webhooks" is refused; an Admin's key granted Workspaces Read & write creates a webhook, and the same Admin's key without it is refused.
- **Editing applies at once:** narrowing a key's grant (REST, GraphQL, MCP and the dashboard) refuses the dropped permission on its next request; demoting its creator still narrows it as in m481.
- **Every operation is mapped:** every entry in `classifiedOps` (`lego/backend/internal/api/scope_matrix_ops.go`) maps to one or more catalog permissions or is recorded as needing none, and a guard test fails on an unmapped operation, as `TestScopeMatrixCoversLiveOperations` does for scope classes.
- **Existing keys keep working:** a key minted before this milestone acts as t001 decides (expected: All, which is today's behavior), and Render's Terraform provider on dev-9 still passes the m481 class-3 tests with an All key.
- **Dashboard:** the create dialog and a gear-icon edit dialog list Render's areas, permission names and levels, with All, Read only and Clear all; permissions the creator's role does not allow are disabled with the reason; each key's row summarizes its grant; the pending skeleton matches the ready page at desktop and narrow-mobile widths; all eight locales carry the strings.
- **On production as the QA user**, once the pin carries this milestone: a Services Read only key lists services and is refused a service create, its grant is widened and the create passes, then everything is deleted and the key revoked.
- **Docs:** ADR024, ADR012 §8e and ADR018's API keys row record the model and its security reasoning, and the security lineage records the review.

## Source + Goal linkage

- **Source:** promoted from [w9/114](../114.md) (2026-11-18), filed by [w9/m481](../m481/README.md) t009 as the variant m481 did not take.
  - Render ([render.com/docs/api-keys](https://render.com/docs/api-keys), re-read 2026-11-18): "API keys limit access to resources within a specific workspace, so you can grant each key only the permissions it needs." Create: name the key, "Select the workspace your key can access", "Grant your key permissions", with presets "All: Grants every permission your member role allows for every resource", "Read only: Grants Read access to every applicable resource" and "Clear all: Removes all permissions from the key". "Your member role in the selected workspace determines which permissions you can grant an API key." A key's permissions can be changed later ("Click the gear icon next to the key you want to update … Change your key's permissions, then click Save changes").
  - Render's tables: Services, Workflows, Sandboxes, Deploys, Monitoring, Postgres, Key Value, Object storage, Build sources, Workspaces, Projects, Environment groups, Blueprints and Audit logs, each permission offered at some of Read, Write, Read & write, Run (Deploys, Workflows) and Execute (Sandboxes). "Write" alone covers setting secret values without reading them (for example "Set environment variable and secret file values"). The note quoted an "Enable all" preset; the page now says "All".
  - bex (main `148fb564b`): a key acts with exactly its creator's current role (`lego/backend/internal/core/keyrole.go` `keyCeiling`); API keys are exempt from the per-operation dispatch gate (`lego/backend/internal/api/scope_matrix.go` `requireScopeClass`, `id.CapabilityExempt()`), which is the natural hook. Key metadata lives on the Hydra client (`lego/backend/internal/apikeys/service.go`, `bex.co/created-by`). The dashboard's create dialog is `dashboard/src/features/api-keys/components/create-api-key-dialog.tsx`.
- **Out of scope, owned elsewhere:** a Viewer, Contributor or Billing member minting a key ([w9/115](../115.md)), a key changing a member's role ([w9/116](../116.md)), a key creating a Postgres credential ([w9/117](../../117.md)) and Render's classic all-workspace keys ([w9/118](../../118.md)). The catalog lists those permissions anyway, and t001 records how the picker shows a permission bex does not yet let any key exercise. Render Workflows is a non-goal (`.pm/DO_NOT_DO.md`), so its area has no operations here.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); a least-privilege CI or Terraform key is impossible today.
- **Expected outcome:** a customer mints a key that can only read, or only deploy, and a leaked key is limited to what it was granted.
- **Why now:** m481 just made keys follow their creator's role, so the ceiling scoped keys sit under exists, and its t009 filed this as the remaining API keys gap.
- **Render parity:** included (t010). The change is customer-facing on REST, GraphQL, MCP and the dashboard's API keys page.
- **Security:** this changes what a credential may do, so t008 reviews it in the security lineage, and the boundary above is every task's acceptance floor.

## Unblocked (2026-11-19, `/loopx w9` worker)

- **Affected:** t009 (step 5), t013. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on `origin/main`, `19faf35f3` (`chore(deploy): pin platform images to 4b2cebd0bd03`, 2026-11-19 09:13Z), carries this milestone's commit: `git merge-base --is-ancestor e09ea5065 4b2cebd0bd03` holds, and no later m496 commit touches code. Production's `bex-system/bex-api` runs the pinned `bex-operator@sha256:4407341b60d6…` (2/2 ready). w9/m481 closed on the same pin with a key minted with no `permissions` (All under this milestone).
- **Status:** pending verification. A deployed bex-api is not proof that the definition of done holds on production.
- **Next:** run t009 step 5 as the QA user in `bex-canary`: a Services Read only key lists services and is refused a service create, its grant is widened and the create passes, then the service is deleted, the key revoked and the session logged out. Record it in t009, then t013.
- **Verified and closed (2026-11-19):** t009 step 5 passed on that pin as the QA user in `bex-canary` (11:10Z–11:12Z): a Services Read only key listed services on REST, GraphQL and MCP and was refused a service create (403 `api_key_permission_denied`, `services.manage`); widened to Services Read & write by the session, the same create passed (201); the service was deleted, the key revoked (then 401) and the session logged out. Every Definition-of-done bullet holds (evidence in [t009](done/t009.md), checked in [t013](done/t013.md)).
