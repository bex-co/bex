# w9 · m514 — A Viewer, Contributor or Billing member mints an API key capped at their role, and manages their own keys, as on Render

**Worker:** worker9 **Goal:** every member role can mint an API key in its workspace, as on Render; the key acts with exactly its creator's current role and chosen grant (never more than their intersection, m481 and m496 unchanged); and a member below Developer lists, edits and revokes the keys they minted there, on REST, GraphQL, MCP and the dashboard. **Status:** done (t001–t005, t007–t009 shipped in `6253c7faf`; t006 checked on dev-9 and, on 2026-11-19, on production pin `c624a4f217ff`; t010 closed it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: who mints, who lists, edits and revokes, what Billing may grant; ADR024, ADR012 §8e and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Mint and seat: every member role mints a key at its own role, and reads the permission catalog marked for that role — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Own keys: a member below Developer lists, edits and revokes the keys they minted in the workspace, and no other — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Dashboard: the API keys panel works for every member role — **DONE** | 1h30m | t003 |
| [t005](done/t005.md) | Security review of the widened mint boundary, recorded in the security lineage — **DONE** | 45m | t003 |
| [t006](done/t006.md) | Live check on dev-9 and production — **DONE** | 1h30m | t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t009 |

Total: **10 tasks, about 8h**.

## Security boundary (holds for every task)

- **m481's I1–I5 and m496's I6–I8 hold unchanged** ([ADR024 § An API key acts with its creator's role](../../../../docs/ADR024-members.md#an-api-key-acts-with-its-creators-role-w9m481), [§ An API key carries a chosen set of permissions](../../../../docs/ADR024-members.md#an-api-key-carries-a-chosen-set-of-permissions-w9m496)). A key is allowed an operation only when its grant covers it **and** its creator is allowed the same thing at the same freshness, in the key's own workspace.
- **I9: minting widens who holds a key, never what a key may do.** A Viewer's key is refused everything a Viewer is refused (logs, sensitive fields, restart, create, every write); a Contributor's and a Billing member's likewise for their role. No new relation is granted to a key subject: the key holds its creator's role tuple, and the ceiling caps it.
- **I10: a member below Developer reaches only the keys they minted.** Listing, editing and revoking another member's key stays `can_manage_keys` (Developer or Admin). A key minted by a key is never "own" to a person.
- **I4 and I5 still hold for these keys**: a Viewer's key cannot mint a key, enroll an SSH key, or invite or re-role a member.

## Definition of done

- **On dev-9, on REST, GraphQL, MCP and the dashboard,** a Viewer, a Contributor and a Billing member in a Scale workspace each mint a key with no `permissions` (All). Each key is allowed exactly what its creator's role allows and refused the rest with the same 403 a person of that role gets. At least: the Viewer's key lists services and is refused service logs, a restart and a service create; the Contributor's key reads logs and restarts a service and is refused a service create and an environment variable read; the Billing member's key reads the workspace's Build Pipeline settings (billing data, `can_manage_billing`) and is refused service logs and a deploy.
- **The role caps the grant:** a Viewer's mint naming a Write or Run level is `403 api_key_permission_above_role`; Read only is accepted; the picker shows the above-role levels disabled with their reason.
- **The key follows its creator:** promoting the Viewer to Developer lets their key create a service on its next write; demoting them back refuses it; removing them from the workspace revokes the key (401).
- **Own keys:** the Viewer lists only the keys they minted in that workspace, edits one's grant and revokes it; their list, edit or revoke of another member's key is refused (404 or 403, the existing shape); a Developer and an Admin still list every key in the workspace.
- **I4 and I5:** a Viewer's key is refused a key mint, an SSH key enrollment and a member role change, as every key is.
- **The real-DB backend suite** extends `TestAPIKeyActsWithItsCreatorsRoleE2E` (`lego/backend/internal/api/apikey_creator_role_e2e_test.go`) to Viewer, Contributor and Billing creators on real Postgres and OpenFGA, and `TestKeyNeverExceedsItsCreatorOverRandomHistories` covers every role.
- **On production as the QA user,** once the pin carries this milestone: the QA user mints a Read only key in `bex-canary`, lists it and revokes it (the widened gates did not break the Admin's path). The below-Developer journey is replayed on production when a second QA identity is available; otherwise t006 records that it ran on dev-9 only, and why.
- **Docs:** ADR024, ADR012 §8e and ADR018's API keys row record the rule and its security reasoning, and the security lineage records the review.

## Source + Goal linkage

- **Source:** promoted from [w9/115](../../done/115.md) (2026-11-19), filed by w9/m481 t009 as a Render key form m481 did not take.
  - Render ([render.com/docs/api-keys](https://render.com/docs/api-keys), re-read 2026-11-19): "Your member role in the selected workspace determines which permissions you can grant an API key." The page restricts minting by role only for audit log keys ("Org Owners can create an audit log API key"). Keys are created, changed and revoked on each person's own **Account Settings** page ("Go to your Account Settings page in the Render Dashboard and click **Create API key**"), so every member manages the keys they minted. "Your key's access adjusts to match your new member role." Render's [team-members](https://render.com/docs/team-members) role table has no API key row.
  - bex (main `d6d29f9a7`): `apikeys.Service.CreateAPIKey` (`lego/backend/internal/apikeys/service.go:316`), `ListAPIKeys`, `RevokeAPIKey`, `UpdateAPIKeyPermissions` and `PermissionCatalog` all require `can_manage_keys` (`developer or admin`, `deploy/gitops/authz/model.fga:89`); `store.BindClientForCreator` (`lego/backend/internal/store/store.go:1068`) seats only `admin` or `developer` creators. A key already follows its creator to any role (`store.enqueueRoleReconciliation`, `internal/store/members.go:559`), so a Developer demoted to Viewer holds Viewer keys today but can no longer list or revoke them. The dashboard's API keys panel (`dashboard/src/features/api-keys/components/api-keys-panel.tsx`, on Account Settings) shows a Viewer its forbidden state.
- **Open question settled here (the note's):** who lists and revokes such keys. Render manages keys per person on Account Settings, so a member below Developer lists, edits and revokes their own keys. Developer and Admin keep bex's existing workspace-wide list and revoke; whether that view should narrow to one's own keys, as on Render, is a parity question t007 files rather than decides.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)): a Viewer cannot get a read-only key for a dashboard script or a status page.
- **Expected outcome:** any member mints a key no stronger than themselves, and can see and revoke it.
- **Why now:** m481 (a key acts with its creator's current role) and m496 (a chosen grant capped by that role) closed on production on 2026-11-19, so the ceiling a below-Developer key needs exists and has been reviewed; this was the remaining mint-side gap ADR018's API keys row names.
- **Render parity:** included (t007). The change is customer-facing on REST, GraphQL, MCP and the dashboard's API keys panel.
- **Security:** this widens who may hold a credential, so t005 reviews it in the security lineage, and the boundary above is every task's acceptance floor.

## Unblock and closeout (2026-11-19)

- **Finding (verified evidence):** the self-clearing gate holds. The pin `c81d2d447bd8` and the newer `c624a4f217ff` (`99bbc4239`) both carry `6253c7faf`, this milestone's commit (`git merge-base --is-ancestor 6253c7faf <pin>`), and production's `bex-system/bex-api` ran the `c624a4f217ff` image (`bex-operator@sha256:c588e4d2713c…`, both replicas ready).
- **t006's production step** then ran as the QA user in `bex-canary`: a Read only key minted (201, 21 Read grants), listed (200), refused a key mint and the key list (403 `api_key_permission_denied`), revoked (204, then 401), and the session logged out. The below-Developer journey ran on dev-9 only, because production has one QA identity, an Admin; see [t006](done/t006.md).
- **t010** confirmed every Definition-of-done bullet against t006's record and closed the milestone ([t010](done/t010.md)).
