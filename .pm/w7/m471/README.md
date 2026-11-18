# w7 · m471 — Turning on a login requirement lists the members and keys it would block, as Render's SSO enforcement does

**Worker:** worker7 **Goal:** before an admin turns on "Require two-factor authentication" or "Require Google login" in a workspace, or an Org Owner turns one on for an org, the confirmation lists each member who would lose access and each API key and SSH key that would stop working, decided by the same checks the gate enforces. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Gate: assess a member and their keys against a requirement, outside a request](t001.md) | 45m | — |
| t002 | [Workspace read on GraphQL: `workspaceLoginRequirementImpact`](t002.md) | 1h | t001 |
| t003 | [Org read on GraphQL: `organizationLoginRequirementImpact`](t003.md) | 45m | t002 |
| t004 | [Dashboard: the workspace card asks before turning a requirement on, and lists who it would block](t004.md) | 1h15m | t002 |
| t005 | [Dashboard: the org Security card's confirmation lists who it would block](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](t006.md) | 30m | t005 |
| t007 | [Simplify](t007.md) | 30m | t006 |
| t008 | [Test coverage](t008.md) | 45m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- Turning on 2FA in a workspace with one member who has no second factor shows that member, their API keys bound to the workspace and their SSH keys in the confirmation before anything is written. A workspace where every member is enrolled shows that nobody would be blocked. The same holds for an org: members of any org workspace and the org's own members are counted.
- Turning on Google login lists members whose linked Google account does not match their email, and keys whose creation sign-in stamp the gate would refuse. Unstamped keys are not listed, because the gate exempts them.
- The list comes from `internal/twofactor`'s own checks and caches, not from a second copy of the rules. A lookup that cannot answer is shown as "could not check", never as an empty list, and the admin can still confirm.
- Turning a requirement off never asks, on both cards.
- Backend tests cover the assessment truth table and both reads, including authorization. Component tests cover both dialogs: listed, empty, loading and failed states. The dialog's loading state keeps the ready list's geometry.
- ADR012 §9a–§9c and `docs/render-artifacts/login-settings.md` describe the confirmation as shipped.

## Source + Goal linkage

- **Source:** promoted from [w7/086](../done/086.md) on 2026-11-17 (found by w7/m466 t001).
- **Render's behavior:** [saml-sso](https://render.com/docs/saml-sso), captured in [docs/render-artifacts/login-settings.md § Organizations](../../../docs/render-artifacts/login-settings.md#organizations-w7m466) on 2026-11-17: when SSO is enforced, "Each listed member will lose access to the org until they log in via SSO" and "Each listed API key will be invalidated until its owner logs in via SSO". No Render page says whether the 2FA or Google confirmation lists anyone. Applying the SSO list to bex's two requirements is an inference, recorded as such in ADR012 §9c ("The enable confirmation").
- **bex today:**
  - The workspace card does not confirm at all. Its switch sends the mutation as soon as it is toggled (`dashboard/src/features/workspaces/components/workspace-login-requirement-card.tsx:96`).
  - The org Security card confirms with one sentence and lists nobody (`dashboard/src/features/organizations/components/organization-security-card.tsx:104-113`).
  - No GraphQL field names who a requirement would block (`lego/backend/internal/api/testdata/schema.graphql` has only the setters, lines 1966-1967 and 1989-1990).
  - The only check before the write is the acting admin's lock-out guard (`lego/backend/internal/workspaces/service.go:1346-1404`).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)), workspace and org login settings ([ADR012 §9a](../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402), [§9b](../../../docs/ADR012-auth.md#9b-workspace-enforced-google-login-w7m465), [§9c](../../../docs/ADR012-auth.md#9c-organization-enforced-two-factor-authentication-and-google-login-w7m466)).
- **Expected outcome:** an admin or Owner sees who loses access before committing, instead of learning it from members' 403s afterwards.
- **Why now:** w7/m465 and w7/m466 just shipped both requirements on both scopes, with the gate's per-member checks and caches in one package. The list reuses them while they are fresh, and it is the last confirmation gap ADR012 §9c names.
- **Why a milestone, not the note's hour:** two scopes, each needing a gate assessment for members and two kinds of key, an authorized GraphQL read with its golden schema, a confirmation dialog (new on the workspace card), loading and error states with matching geometry, and eight locales.
- **No user decision gates it.** The list is Render's SSO behavior applied to the requirements bex has. Listing SSH keys is bex's addition, because bex's gate blocks them too (ADR012 §9a).
- **Out of scope:** REST and MCP (the setters themselves have none: ADR012 §9c Surfaces, ADR018); emailing the listed members; listing members' live sessions. For Google login, a member whose account matches but who is signed in another way is asked to sign in again; the dialog says so once instead of listing them.
- **Render parity included:** the change adds a GraphQL read and changes two dashboard confirmations.
