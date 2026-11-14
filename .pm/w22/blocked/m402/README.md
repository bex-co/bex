# w22 · m402 — Workspace admins can require 2FA, as on Render: a member without it is blocked, API keys and SSH included

**Worker:** worker22 **Goal:** a workspace admin can turn on "require two-factor authentication" in Workspace Settings. While it is on, a member who has not enrolled TOTP or a passkey cannot use the workspace's resources or settings from the dashboard, REST, GraphQL, MCP or SSH, and neither can an API key or SSH key that member created. They are sent to enroll, and regain access the moment they do. Turning the setting on and off is audited. **Status:** blocked (t001–t005 and t007–t009 done and shipped; t006 and t010 remain)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design record: where the setting lives, what "has 2FA" means, which principals and routes it covers, the admin lock-out guard and the check's cache; amend ADR012, ADR024 and ADR018 — **DONE** | 45m | — |
| t002 | The setting: a workspace column and migration, an admin-only GraphQL mutation and read field, and an audit event named as Render names it — **DONE** | 45m | t001 |
| t003 | Enforcement in bex-api: a member without 2FA gets a named 403 on every REST, GraphQL and MCP route of the workspace, except the exempt ones — **DONE** | 75m | t002 |
| t004 | Enforcement at the SSH gateway and the ticket mints: service SSH, the browser shell, agent-session attach and sandbox connect refuse a member without 2FA, and a live session is re-checked — **DONE** | 45m | t003 |
| t005 | Dashboard: the admin toggle in Workspace Settings, the blocked-member page that leads to enrollment, and the member row's required/not-enabled state — **DONE** | 75m | t002, t003 |
| t006 | Live check on `dev-N` with three kinds of principal: an enrolled member, a member without 2FA (session, API key, SSH) and an admin enabling it | 45m | t004, t005 |
| t007 | Render parity — **DONE** | 20m | t004, t005 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout | 10m | t006, t008, t009 |

## Definition of done

- An admin of any workspace, on any plan, can turn the setting on and off in Workspace Settings. A non-admin cannot (named 403 on the mutation, no control in the UI).
- With it on, for a member who has no TOTP and no passkey enrolled: a dashboard session, a REST/GraphQL/MCP call carrying their session, an API key they created, and an SSH or shell ticket mint for a service in the workspace all get the named 403 `WORKSPACE_2FA_REQUIRED` (or the gateway's refusal), with the workspace id in its details. The dashboard shows a page that explains why and leads to Kratos enrollment in account settings. The member's other workspaces are unaffected.
- After the member enrolls, access returns within the check's cache lifetime (recorded in the ADR). A member who removes their last second factor while it is on is blocked again within the same bound, and a live SSH or shell session is closed at the gateway's next revalidation.
- The admin who enables it must have 2FA themselves, so the setting cannot lock out its only actor (a bex safety guard, recorded in the ADR as a recommendation of this milestone, not a Render fact).
- Enabling and disabling write an audit-log entry that REST `GET /v1/owners/{ownerId}/audit-logs` and GraphQL return under Render's event type `ChangeTeam2FAEnforcementEvent`.
- Every path above has an automated test, and the live check on `dev-N` is recorded in this README with the observed codes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w22` (2026-11-14), filed by `/pmx all to w22`.
  - **Render** ([render.com/docs/login-settings](https://render.com/docs/login-settings), read 2026-11-14): "Enforce two-factor authentication (2FA) from your **Workspace Settings** page"; "Only workspace admins can configure these enforcements"; "your team members can't access the workspace's resources or settings until they enable 2FA for their Render account"; "Team members with SSH or API keys can't use these keys to access workspace resources until they enable 2FA". The page lists no plan requirement for 2FA enforcement (the plan requirement it states is for SAML SSO).
  - **Pinned Render OpenAPI** (`lego/backend/internal/api/openapi/render-public-api-1.json`): the audit-log event type enum contains `ChangeOrg2FAEnforcementEvent` and `ChangeTeam2FAEnforcementEvent`.
  - **bex today:** Kratos has TOTP, WebAuthn and recovery codes with a `highest_available` AAL policy (ADR018 MFA row, w4/m11; ADR012 §9). `lego/backend/internal/workspaces/kratos.go` (~lines 191–199) derives `MFAEnabled` from the identity's `totp`/`webauthn` credentials, and `members/service.go` and `dashboard/src/features/team/components/member-row.tsx` only display it as a badge. There is no workspace setting, no refusal and no audit event. `grep -rniE "2fa.?enforc|WORKSPACE_2FA|Team2FA|Org2FA|secure login" .pm docs lego dashboard/src` finds only the OpenAPI enum.
- **Goal linkage:** ADR008 § Render parity first (Identity, workspaces and account); pillar 1, Render-compatible REST and GraphQL.
- **Expected outcome:** a workspace holding production credentials can require every member, and every key and SSH session a member holds, to be second-factor protected, as on Render.
- **Why now:** bex has paying workspaces (`.pm/w2/blocked/058.md` records 9 live subscriptions) whose members hold production credentials, and the second-factor foundation is shipped; the missing piece is only the enforcement layer. SSO/SAML/SCIM stay a non-goal (DO_NOT_DO): this is not SSO, and Render lists no plan gate for it, so it applies to every workspace.
- **Open recommendations, not user decisions** (this milestone's tasks implement them; the ADR in t001 records them as recommendations): "has 2FA" means an enrolled TOTP or passkey rather than an AAL2 session (Kratos already forces AAL2 on login for an enrolled user); the admin lock-out guard; exempting non-human platform principals. Nothing here diverges from Render's observable behavior, so no ADR018 divergence sign-off is needed. The sibling Render setting on the same page, allowed login methods (`ChangeTeamAllowedLoginMethodsEvent`), is not part of this milestone.
- **Render parity included:** the milestone adds a GraphQL field and mutation, a dashboard setting and an audit event, so t007 compares each surface with Render.

## Progress 2026-11-14 (/loopx w22)

- **Shipped (t001–t005, t007–t009):** the design in [ADR012 §9a](../../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402), with the as-built differences recorded there (gateway verb, bounds, rollout order, the S1–S5 hardening below). Setting: `tenants.require_two_factor` (migration 0260), `Workspace.requireTwoFactor`, `setWorkspaceRequireTwoFactor(workspaceId: String!, enabled: Boolean!)`, audit event `ChangeTeam2FAEnforcementEvent`. Enforcement: one hook on `core.Base` (`internal/twofactor`, `core.TwoFactorGate`) behind every authorization decision, so REST, GraphQL and MCP share it, with `403 WORKSPACE_2FA_REQUIRED`. The SSH gateway asks bex-api over `POST /v1/workspace-two-factor` (HMAC) and runs the same hook. Dashboard: Workspace Settings card, one blocked-member page (enroll link, workspace switcher, leave), member-row state.
- **Hardening found in review and fixed with tests that fail on the old code:** an API key whose creator's Kratos identity was deleted is refused (it had been exempt), an `oauth2` identity with no client id fails closed, an agent-session object with no session reader fails closed, the default workspace cannot be given a requirement nothing enforces, and a mis-wired key-creator lookup now logs.
- **Found when merging with upstream's inbound IP rules (w9/m388):** the exempt workspaces list could have handed a blocked member the workspace's IP rules through `Workspace.ipAllowList` and `GET /v1/owners`. They are now withheld for a workspace that blocks the caller, with a test that fails without the fix.
- **A real defect t009 found:** two admins toggling at once audited twice. `SetTenantRequireTwoFactor` is now a conditional update that reports `changed`.
- **Checks:** the real-dependency backend suite (`BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...`) passed in all 78 packages; backend lint, `hack/deadcode.sh`, gofmt and `go vet` are clean; `cd dashboard && yarn lint && yarn test` pass (685 files, 7195 tests). More than 25 deliberate mutations of the gate, the funnel hook, the exempt operations, the lock-out guard, the caches and the gateway revalidation were each caught by a test, with no residue.
- **Parity (t007):** no verified drift from Render; [ADR018](../../../../docs/ADR018-render-parity.md) has the row. REST and MCP are `—` by design: Render has no REST workspace-mutation API, and `owner.twoFactorAuthEnabled` is the user's own state, present only for `type=user`.
- **Not verified anywhere:** nothing ran against a live Kratos, Hydra or browser. The dashboard geometry and the enroll-then-return flow were checked in jsdom only.

## Gate: t006 (the live check)

**BLOCKED — needs, from the user or infra, a machine that can run the shared local cluster.** `scripts/dev-env.sh <N> up` requires the kind/CAPD cluster `bex` from `bash scripts/mock-cluster.sh` (kind, clusterctl, CAPD, CNPG, the operator, charts pulled from ghcr). The machine this session ran on has neither `kind` nor `clusterctl` and an 8 GiB Docker VM, and a ghcr pull returned 504 in the same session, so the stack could not be brought up. Everything t006 asks is unchanged: with real Kratos TOTP, an enrolled member (B), a member without 2FA (C), and an admin (A) on one workspace: A is refused enabling without 2FA, then enrolls and enables (one `ChangeTeam2FAEnforcementEvent`); C's dashboard session, REST/GraphQL, own API key, shell ticket and native SSH are all refused and C's other workspace still works; B's everything works; C enrolls and regains access within the documented bound (about 60 s API, about 90 s SSH); C removes TOTP and is blocked again, and an open shell is closed within about 150 s; A turns it off. Things only a live stack can show: that `MFAEnrolled` reads a real TOTP and a real passkey enrollment correctly (the webauthn `include_credential` assumption is the one the member badge already makes), that Hydra's `bex.co/created-by` stamp reads back for a fresh key, and the SSH banner in a real `ssh` client. Record the observed codes and timings under a "Live check" heading here, then run t010.

**Rollout note for whoever deploys:** bex-api must be running the new verb before the new SSH gateway, otherwise native SSH and ticket redemption are refused until it is (ADR012 §9a "Rollout order").
