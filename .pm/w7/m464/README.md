# w7 · m464 — Scale and Enterprise workspaces belong to an organization, as on Render: unlimited workspaces under one plan, Owner, Member and Guest roles, and an org audit log

**Worker:** worker7 **Goal:** a Scale or Enterprise workspace is the primary workspace of an organization. Its Owners add workspaces at no additional plan fee, invite Members and Guests, and read an org audit log, on GraphQL, MCP, the dashboard and Render's `GET /v1/organizations/{orgId}/audit-logs`. Downgrading the primary below Scale dissolves the org, as on Render. **Status:** in progress (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's organization model and record the design: a new ADR, ADR018, ADR030 §1/§9, ADR012 and the plan-table rows](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [store: organizations, their members and workspaces, with production's Scale and Enterprise workspaces backfilled as primaries](t002.md) | 1h30m | t001 |
| t003 | [authz: the OpenFGA `org` type takes Render's Owner, Member and Guest, and an Owner is not an implicit workspace admin](t003.md) | 1h | t002 |
| t004 | [GraphQL and MCP read the organization: its workspaces, primary, plan and members](t004.md) | 1h | t002, t003 |
| t005 | [Plan changes create and dissolve the org: an upgrade makes a primary, a secondary follows its org's plan, a downgrade meets Render's preconditions](t005.md) | 1h30m | t004 |
| t006 | [Org workspaces: an Owner creates them Public or Invite-only, Members join public ones, Owners join any as admin; support moves an existing workspace in](t006.md) | 1h30m | t004 |
| t007 | [Org members: invite as Member or Guest, change the org role, remove, and at most two Billing members per org](t007.md) | 1h30m | t004 |
| t008 | [The org audit log: Render's `list-organization-audit-logs`, GraphQL and the org events](t008.md) | 1h30m | t005, t006, t007 |
| t009 | [Dashboard: the workspace switcher shows the org, and the org's Workspaces page](t009.md) | 1h30m | t006 |
| t010 | [Dashboard: the org's Members page and its Settings with the Audit Logs CSV export](t010.md) | 1h30m | t007, t008 |
| t011 | [Dashboard: the plan picker, the change-plan dialog and a workspace's privacy follow the org](t011.md) | 1h | t005, t006 |
| t012 | [ADRs and the plan-table guard state the shipped rule](t012.md) | 30m | t008 |
| t013 | [Production: an org's lifecycle on a fixture workspace](t013.md) | 1h | t009, t010, t011, t012 |
| t014 | [Render parity](t014.md) | 45m | t009, t010, t011, t012 |
| t015 | [Simplify](t015.md) | 30m | t014 |
| t016 | [Test coverage](t016.md) | 1h | t014 |
| t017 | [Closeout](t017.md) | 10m | t013, t016 |

## Definition of done

- Changing a standalone workspace's plan to Scale or Enterprise creates an organization. That workspace becomes the primary, and the member who changed the plan becomes its Owner. After the migration, each of production's existing Scale and Enterprise workspaces is the primary of its own organization.
- An Owner can create any number of further workspaces in the org, each Public or Invite-only (Invite-only by default). Each one has the org's plan and that plan's features and pays no plan fee of its own. Usage is still billed per workspace. ADR030, the plan picker and `GET /pricing` say the Scale or Enterprise fee is charged once per organization, on the primary.
- Org roles are Owner, Member and Guest, on GraphQL, MCP and the dashboard. An Owner is not automatically a workspace admin, but can add themselves to any org workspace as admin. A Member can add themselves to a Public workspace. A Guest reaches only the one workspace they were invited to. An org has at most two members with the Billing role.
- A secondary workspace's plan cannot be changed by itself. Downgrading the primary below Scale is refused, by name, until Render's preconditions hold: the secondary workspaces are deleted, Billing, Viewer and Contributor members and pending invites are reassigned, and the inbound IP rules are deleted. Once they hold, the downgrade deletes the org, and the primary carries on as a standalone workspace.
- `GET /v1/organizations/{orgId}/audit-logs` serves Render's `auditLogWithCursor` rows to an Owner, and answers 403 to anyone else. The rows are Render's org events for member and workspace management. GraphQL and the org Settings CSV export read the same rows. The operation is gone from `renderOperationOmissions`.
- A new ADR, ADR018, ADR030 §1/§2/§9 and ADR012 state the rule. In `plan_table_test.go`, the `pricing: Workspaces`, `pricing: Organization user roles` and `features: Org-level audit logs` rows carry checked dispositions. `followUpOrganizations` is gone.
- On production, once the image pin carries the change, t013's walk passes: upgrade, a secondary workspace, a Member, a Guest, the org log, then a refused and a completed downgrade.

## Source + Goal linkage

- **Source:** promoted from [w7/077](../done/077.md) on 2026-11-17. w7/070 found it while it read the pricing table's `Workspaces: 1 | 1 | Unlimited | Unlimited` row. `lego/backend/internal/pricing/plan_table_test.go:232` (`followUpOrganizations`) names it.
- **Render's behavior** (read 2026-11-17):
  - [organizations](https://render.com/docs/organizations): orgs exist on Scale and Enterprise, and "An org can contain unlimited workspaces at no additional cost". Upgrading a workspace to Scale creates the org: that workspace becomes the **primary workspace**, and the member who upgraded becomes the Org Owner. Owners create workspaces as Public or Invite-only, and new ones are Invite-only by default. An Owner manages org settings and members, and "can add themselves to any workspace as an admin". Owners "are not automatically added to workspaces". A Guest "can access only the single workspace they're invited to". An org allows at most two members with the Billing role. Downgrading from Scale deletes the org. You must first delete the secondary workspaces, reassign Billing, Viewer and Contributor members and pending invites to Developer or Admin, and delete the inbound IP rules. The primary carries on as a standalone workspace. Support transfers existing workspaces into an org.
  - [audit-logs](https://render.com/docs/audit-logs): org audit logs are on Scale and higher. Only Owners can export them, from the org's Settings, 90 days per export. Retention starts at the upgrade. The events cover member management (`InviteToOrgEvent`, `AcceptOrgInviteEvent`, `AddOrgMemberEvent`, `RemoveOrgMemberEvent`, `JoinTeamEvent`, `ChangeOrgRoleEvent`, `ChangeOrgAllowedLoginMethodsEvent`, `ChangeOrg2FAEnforcementEvent`), workspace management (`CreateWorkspaceEvent`, `DeleteWorkspaceEvent`, `ChangeWorkspacePrivacyEvent`) and IdP management (SSO and SCIM).
  - Each workspace keeps its own Billing page, payment method, usage and invoices, and the org's plan is managed from the primary workspace's Billing page (render.com/docs/team-members and the organizations downgrade steps).
  - The pricing table has `Organization user roles: — | — | Owner, Member, Guest | Owner, Member, Guest` and `Audit logs: — | Workspace | Workspace and organization | …` (`docs/render-artifacts/pricing.md:189-197`). The pinned OpenAPI has one org operation, `list-organization-audit-logs` (`lego/backend/internal/api/openapi/render-public-api-1.json`).
- **bex today:**
  - There is no organization. `deploy/gitops/authz/model.fga:52-61` declares an `org` type and `admin from org` on `workspace`, but no code writes an org tuple. Unlike Render, that relation would make an org admin an implicit admin of every workspace.
  - `list-organization-audit-logs` is an omission (`lego/backend/internal/api/render_operation_coverage_test.go:62`).
  - `store.LimitsFor` lets one user own unlimited paid workspaces, each with its own plan.
- **Premise correction:** the note said a customer "pays bex's $349.30 once per workspace". bex charges no plan fee at all today: "No licensed monthly SKU; usage-meter contract only" (`docs/render-artifacts/new-workspace.md:43`; `docs/runbooks/stripe-billing-setup.md:62`). `Sheet.WorkspaceUSDPerMonth` has no caller outside `internal/pricing`. So no customer is charged twice today. The difference is in the listed price (the plan picker, `GET /pricing`, ADR030 §2) and in how a future licensed SKU must attach: once per org, on the primary.
- **Why no user decision gates it:** the note offered three options. Under [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) (user decision 2026-10-17, restated 2026-10-20), building organizations is the parity choice. "Bill per org without org features" falls short of parity. "Record no organizations" needs a sign-off that ADR018 does not have: ADR018's "bex has no organization scope" carries none, as the note itself says. SAML SSO, SCIM and domain capture stay out under `.pm/DO_NOT_DO.md` (Enterprise surfaces: SSO/SAML · SCIM). This milestone is a parity recommendation, not a user decision. The user can still sign off "no organizations" as a divergence through `/unblock w7`, which would retire this milestone.
- **Goal linkage:** Render parity first, the plan table ([ADR030 §9](../../../docs/ADR030-pricing.md)), members and roles ([ADR024](../../../docs/ADR024-members.md)), authz ([ADR012](../../../docs/ADR012-auth.md)), and the audit log ([ADR018](../../../docs/ADR018-render-parity.md) audit row).
- **Expected outcome:** a team on Scale or Enterprise runs many workspaces under one org and one plan, with Render's org roles and org audit log. The last three org rows of Render's plan table get a checked disposition instead of a follow-up or an unsigned non-goal.
- **Why now:** w7/m429 and w7/070 gave every other plan-table row a disposition, so these are the open ones. On 2026-11-17, production had 3 Scale workspaces and 1 Enterprise workspace, and one admin held two or more of them. That admin would sit in one org on Render.
- **Render parity included:** the change is visible on REST (the org audit-log route), GraphQL, MCP and the dashboard.
- **Out of scope, filed by t014 as their own items:** org-level 2FA enforcement and allowed login methods (`ChangeOrg2FAEnforcementEvent`, `ChangeOrgAllowedLoginMethodsEvent`; the Google-login half is w7/078), and Enterprise seat-based billing (custom terms). Attaching the licensed plan-fee SKU to Stripe Subscriptions is not filed anywhere yet; this milestone only states the rule it must follow.
