# w9 · m539 — Every member's API keys list shows their own keys, as on Render; an Admin opts into the workspace's to revoke a demoted member's or a creator-less key

**Worker:** worker9 **Goal:** the API keys list on REST, GraphQL, MCP and the dashboard's Account Settings panel shows every caller the keys they minted, whatever their role, as Render's Account Settings does; a Developer lists, edits and revokes only their own keys; editing a key's grant is its creator's verb alone; and an Admin keeps one explicit, opt-in view of every key in the workspace, with revoke, so a demoted member's keys and keys with no recorded creator stay reachable. **Status:** blocked (t001–t005 and t007–t009 done and checked on dev-9 for every role; t006's production check waits for a pin carrying `704667da6`, then t010)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design and docs: the own-keys rule for every role, the Admin's opt-in view, the invariants; ADR024 and ADR119 amended — **DONE** | 40m | — |
| [t002](done/t002.md) | Service: `ListAPIKeys` own by default, `allMembers` for an Admin; revoking another's key needs an Admin; editing a grant is the creator's alone — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Adapters: REST `?allMembers=true`, GraphQL `allMembers`, MCP `allMembers`; the real-DB e2e on all three surfaces — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | Scripts that read a workspace-wide key list ask for it, and still clean up what they mint — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Dashboard: the panel lists the caller's keys; an Admin switches to every member's keys and revokes there — **DONE** | 1h30m | t003 |
| [t006](t006.md) | Live check on dev-9 (every role) and on production (the QA user, an Admin) | 1h | t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 7h45m**.

## Decision (2026-11-20, by the parity rules, not a user decision)

The note's open question had three options: narrow every role's list to their own keys, keep the workspace-wide view as a divergence, or add an Admin-only view of every key beside the own list. AGENTS.md's parity rules settle the first two. Render's per-person view is what every Render member sees, and it is reachable without any optional decision, so bex ships it. A divergence stands only with the user's sign-off in ADR018, and the workspace-wide default has none.

- **Render's evidence, re-read 2026-11-20.** [render.com/docs/api-keys](https://render.com/docs/api-keys): "Go to your Account Settings page in the Render Dashboard and click **Create API key**"; to change one, "Go to your Account Settings page in the Render Dashboard and scroll down to the API Keys section". The page names no role that sees, lists or revokes another member's keys, and no workspace-level key page. [team-members](https://render.com/docs/team-members): Admins manage team members ("Remove team member"), and the role table has no API key row. [saml-sso](https://render.com/docs/saml-sso) is the only place an org admin sees other members' keys, and it is a read-only report in the enforce-SSO confirmation ("a report of your org members who have yet to log in via SSO … along with all API keys owned by those members"). bex already has that report (the login-requirement impact list, `dashboard/src/features/workspaces/components/login-requirement-impact-list.tsx`). SCIM deprovisioning: "All API keys belonging to the deprovisioned member are immediately invalidated." Render's changelog ("Fine-grained API keys now generally available", 2026-11-18) adds no Admin key view.
- **A departed member's keys need no Admin view.** Every path that ends a membership revokes the keys its member minted in that workspace (w2/m163): `members.Service.endMembership` calls `disposeMemberKeys` first (`lego/backend/internal/members/service.go:1270`) for an Admin's removal (`Remove`, `:1178`), a member leaving (`LeaveWorkspace`, `:1310`) and an organization removal (`members/organizations.go:615`). Account deletion runs `CleanupSubject` for every workspace (`accounts/service.go:270`), and a workspace delete revokes its keys (w9/120).
- **A demoted member's keys and keys with no recorded creator do need one.** A demotion ends no membership: the key follows its creator down (m481 I1), as Render's does ("Your key's access adjusts to match your new member role"), so nothing revokes it. A key with no `bex.co/created-by` stamp is no one's own (m514), so no own view reaches it. Narrowing the Admin's view with no way back would leave those keys unrevokable from the product, so the Admin keeps one opt-in view of every key in the workspace, with revoke.
- **That opt-in view is a bex addition Render does not have.** t007 records it in ADR018 as a divergence **pending the user's sign-off**, not as a user decision. It also files one inbox note for the user with the stricter variant (drop the Admin view) and the looser one (an Admin may also edit another member's grant).
- **Narrowed with it, by the same evidence.** A Developer loses the list, revoke and edit of anyone else's key. Render gives a Developer none of them, and a Developer cannot remove a member, so they have no offboarding use either. Editing a key's grant becomes the creator's verb alone. Render's gear is on the creator's own Account Settings, and revoke is enough for offboarding.

## Security boundary (holds for every task)

- **I1–I12 hold unchanged:** m481's I1–I5, m496's I6–I8, m514's I9 and w9/116's I11–I12 ([ADR024](../../../../docs/ADR024-members.md)). A key acts with no more than its creator's current role and its grant, in its own workspace.
- **I10, restated for every role:** by default every caller lists only the keys they minted in the workspace the request acts in. A machine caller owns nothing, so an API key's default list is empty. Only the key's creator edits its grant.
- **I13 (new): only an Admin reaches another member's key or a creator-less key.** That reach is the workspace-wide list (asked for explicitly with `allMembers`) and the revoke of a key that is not the caller's. Both are decided by one `Authorize` against `can_manage` (Admin), audited once. A Developer, a Contributor, a Viewer and a Billing member get 403 for both. `requireKeyIn` still holds the key to the request's workspace.
- **Nothing widens.** No caller lists, edits or revokes a key it could not before. An Admin's key with an All grant keeps reaching what its creator reaches (it passes `allMembers`). Every refusal keeps today's shape: 403 for another member's key, `ErrNotInNamedWorkspace` for another workspace's, 404 for an unbound one.

## Definition of done

- **On REST, GraphQL and MCP**, with real Postgres and OpenFGA: each of the five roles lists exactly its own keys by default. An Admin's `allMembers` list holds every key in the workspace, including another member's and a creator-less key. Every other role's `allMembers` answers 403. A Developer revoking or editing another member's key answers 403. An Admin revokes another member's key and the key answers 401 on its next request, but editing that key's grant answers 403 `api_key_permissions_editor_forbidden`. An Admin's All key lists nothing by default and everything with `allMembers`.
- **The dashboard's API keys panel** shows a Developer only the keys they minted. An Admin sees their own keys and a switch to every member's keys, where each row names its creator and only the Admin's own rows carry the gear. The panel's pending state matches the ready state for an Admin and for a non-Admin, at desktop and narrow-mobile widths.
- **The scripts** that list or revoke keys as a bootstrap or Admin client (`scripts/auth-e2e.sh`, `scripts/verify-sandbox-isolation-live.sh`, `scripts/render-cli-auth-e2e.sh`, `scripts/auth-oauth21-e2e.sh`, `scripts/qa-mcp.py`) still pass and still remove every key they mint.
- **Checked on dev-9** for every role, and **on production** as the QA user (an Admin) once the pin carries this milestone. The QA user mints a key in `bex-canary`, sees it in the default list and the `allMembers` list on all three surfaces and the dashboard, revokes it, gets 401 from it, and logs the session out. No key the run did not mint is revoked or edited.
- **Docs:** ADR024's § Every member mints and manages their own API keys and ADR119 record the rule and I13. ADR018's API keys row loses its "Divergence, open" sentence and records the Admin's opt-in view as pending the user's sign-off. The sign-off note is filed.

## Source + Goal linkage

- **Source:** promoted from [w9/131](../../done/131.md) (2026-11-20), filed by w9/m514 t007. Today `apikeys.Service.ListAPIKeys` (`lego/backend/internal/apikeys/service.go:546`) shows every key bound to the workspace to a caller holding `can_manage_keys` (Developer or Admin, `:560`). `keyRelation` (`:662`) decides a verb on another member's key with `can_manage_keys`, and `UpdateAPIKeyPermissions` (`:717`) lets an Admin edit any key's grant.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). Every customer-facing behavior must match render.com in every workspace, and today a Developer and an Admin see a page no Render member sees.
- **Expected outcome:** a member opening Account Settings sees the keys they made, as on Render. A Developer can no longer revoke an Admin's or a teammate's CI key. An Admin still has one deliberate place to revoke a demoted member's or a creator-less key.
- **Why now:** w9/m514 just gave every role its own keys and left Developer and Admin on the old view. ADR018's API keys row carries this as its one open divergence. Every new member and key adds to the set a Developer can revoke by mistake.
- **Render parity:** included (t007). The change is customer-facing on REST, GraphQL, MCP and the dashboard.
- **Adjacent, out of scope:** [w9/132](../../done/132.md) (the API keys table is wider than its card) touches the same panel. Coordinate, but do not fold it in.
