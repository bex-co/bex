# w1 · m526 — Connect and disconnect several GitHub accounts from every starting point

**Worker:** worker1 **Goal:** a user who administers several GitHub accounts and orgs can connect each one to a workspace, disconnect it, and connect it again from every place the product offers GitHub, through one "Connect GitHub account" action that works whether or not the bex GitHub App is already installed on that account; walked live on production. **Status:** todo (t001–t009 done; t010 waits for the production pin)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record the single connect entry in ADR078 and ADR018 — **DONE** | 40m | — |
| t002 | Backend: the connect entry starts with GitHub OAuth and always returns to the account chooser — **DONE** | 90m | t001 |
| t003 | Backend: a GitHub redirect that carries `setup_action` is never handled as a claim — **DONE** | 45m | t001 |
| t004 | Dashboard: the account chooser marks connected accounts, offers a new install, and closes after a choice — **DONE** | 60m | t002 |
| t005 | Dashboard: one "Connect GitHub account" action at every starting point — **DONE** | 75m | t004 |
| t006 | Dashboard: Credentials menu focus, the agent composer's settings link, and repos that follow the connection set — **DONE** | 45m | — |
| t007 | Render parity — **DONE** | 30m | t003, t005, t006 |
| t008 | Simplify — **DONE** | 30m | t007 |
| t009 | Test coverage — **DONE** | 60m | t007 |
| t010 | Production walk: connect, disconnect and reconnect several GitHub accounts at every starting point | 2h | t008, t009 |
| t011 | Closeout | 15m | t010 |

## Definition of done

1. Every web starting point offers one connect action:
   - Account Settings → Integrations, also reached from the onboarding Getting started checklist;
   - the source picker on `/services/new` for all five service types;
   - the source picker on `/blueprints/new`;
   - the source picker in Service Settings → Update Source;
   - the agent new-session composer.

   From that action, an account that already has the bex GitHub App (`puncsky`, `bex-co`) connects without reinstalling and without GitHub's Configure dead end. An account without the App (`BlockEdenHQ`, after it is uninstalled) installs and connects. No starting point leaves the user on github.com with nothing bound.

2. The account chooser:
   - lists every installed account the authorizing GitHub user administers;
   - marks the ones already connected to this workspace;
   - offers "Install on another GitHub account";
   - disappears with a success message once a choice binds.
3. A connect started from a page with unsaved input keeps that input. These pages are the source-picker hosts and the agent composer. When the user returns, the page shows the new account's repos without a reload.
4. Disconnecting an account at any starting point removes only that account's repos, and connecting the same account again works.
5. A GitHub redirect that carries `setup_action` and no `installation_id` is never handled as a claim. For `setup_action=request`, which is an org member's install request, the user is told the request went to the org owners.
6. Opening the Credentials menu does not put keyboard focus on a Disconnect button. The agent composer's settings link opens the page that holds the GitHub card.
7. t010 records a pass for every starting point × {connect an installed account, install on a new account, disconnect, reconnect}. The walk runs on the production pin that carries this milestone's commits.

## Findings (live investigation on production, 2026-11-19)

Reproduced in a headed Playwright browser, signed in to `dashboard.bex.co` as `puncsky@gmail.com` and to GitHub as `puncsky`.

1. **An already-installed account dead-ends through "Connect another account".**
   - GitHub's install page (`/apps/bex-co/installations/select_target`) shows **Configure** for `puncsky` (`/settings/installations/154851602`) and `bex-co`, and drops the signed `state`. This is the known GitHub behaviour recorded in [ADR078 §3a](../../../docs/ADR078-github-workspace-connections.md).
   - Only "Claim installed account" completes for these accounts, and the source-picker Credentials menu (`dashboard/src/features/git/components/git-credentials-menu.tsx`) says nothing about it.
   - The `git.claimHint` hint went only into the Settings card, in `43fa31f34` (w2/m162). The menu had existed since w8/m31.
   - The same dead end follows several common steps:
     - installing the App directly on github.com;
     - binding the account in another workspace;
     - disconnecting it in bex. `Disconnect` (`lego/backend/internal/github/service.go:756`) only deletes the bex row; the App stays installed.
2. **Claim navigates a source-picker page away.**
   - `dashboard/src/features/git/hooks/use-claim-git.ts:44` sets `window.location.href`, and the callback always lands on `/settings`. A half-filled `/services/new` form is lost.
   - w8/m31 made only connect open a new tab and deferred a same-tab `return_to`.
3. **The claim chooser stays after a successful choice.**
   - `dashboard/src/features/git/components/connect-github-card.tsx:127` only refetches, and `git_claim_selection` stays in the URL. Clicking again reports the choice expired.
   - The chooser also lists accounts already connected (`bex-co`) without marking them. ADR078 §3a keeps them as candidates so that a repeated claim is idempotent.
4. **The Credentials menu opens with focus on "Disconnect bex-co".** Radix FocusScope skips links when it auto-focuses (`removeLinks`). The account name and the Configure entry are links, so the first control it can focus is Disconnect.
5. **The agent composer's GitHub callout** (`dashboard/src/features/agent-sessions/components/new-session-composer.tsx:543`):
   - connects in the same tab with no claim path;
   - its secondary link goes to `/workspace/settings`, which has no GitHub card. The card lives at `/settings#integrations`.
6. **The callback treats any return without `installation_id` as a claim** (`lego/backend/internal/github/rest.go:131`), and `setup_action` is read nowhere in `lego/`.
   - GitHub's setup-URL docs document only `installation_id`, and do not say whether a non-owner's install request redirects at all.
   - Not reproduced: it needs a GitHub account that is a member but not an owner of an org.
7. **The repo list in the form tab stayed stale after a new-tab install.**
   - The account list refreshed (the connections query polls). The repo list stayed stale until a reload, because it refetches only on window focus.
   - This may be a Playwright artifact: switching tabs there may not raise a focus event. t006 makes it robust either way.

Worked as designed:

- The first install on an org without the App (`BlockEdenHQ`) through "Connect another account": 27 repos appeared after a reload.
- The claim flow bound `puncsky` once it was chosen in the chooser.

Production state at filing: the `bex` workspace is connected to `bex-co`, `puncsky` (claimed 2026-11-19) and `BlockEdenHQ`. `BlockEdenHQ` was installed with All repositories during the investigation.

## Source + Goal linkage

- **Source:** a live production investigation on 2026-11-19 in a Claude Code session.
  - The user reported (in Chinese) that "Credentials → + Connect another account" would not connect their own `puncsky` GitHub account; only `bex-co` could be connected.
  - A multi-account walk and a root-cause pass followed; see Findings.
  - The user then asked `/pm` (in Chinese) to put the fix in w1 and to finish with a product check: deploy to production, walk connect and disconnect again at every starting point, and make sure users can connect and disconnect several GitHub accounts correctly.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), the Git connections row of [ADR018](../../../docs/ADR018-render-parity.md)).
  - On Render, one credentials control reaches every account the user has installed the App on, without reinstalling.
  - bex binds installations to workspaces ([ADR078 §1](../../../docs/ADR078-github-workspace-connections.md), a recorded divergence of mechanism), but it must reach the same observable outcome.
- **Expected outcome:**
  - A user with several GitHub accounts connects, disconnects and reconnects any of them from wherever they meet GitHub in the product.
  - The user never has to know that GitHub drops `state` for an installed account.
- **Why now:**
  - The w2/m162 N:N revision made already-installed accounts the common case (ADR078 §3a "The claim selector is not optional"), but every entry point still assumes a first install.
  - The dead end was hit live on production today by a real multi-account user.
  - Prior work this builds on: w5/m74 (N connections), w5/m75 (claim flow), w8/m31 (Credentials menu), w2/m162 (N:N and the chooser).
- **Render parity task included:** the milestone changes REST/GraphQL (the claim start and the chooser candidates) and the dashboard. MCP stays without connect and claim verbs, since they are browser ceremonies (ADR078 §5).
- **Deliberately left out:**
  - **"Configure in GitHub" stays the generic `github.com/apps/<slug>/installations/new` link.** It matches Render's deep link per ADR078.
  - **The mobile agent composer is unchanged.** It has no connect action and says "Connect GitHub on desktop".
  - **Other git providers (GitLab and others) are out of scope.** GitLab and Bitbucket integration are also recorded anti-goals in `.pm/DO_NOT_DO.md`.
