# w1 · m526 — Connect and disconnect several GitHub accounts from every starting point

**Worker:** worker1 **Goal:** a user who administers several GitHub accounts and orgs can connect each one to a workspace, disconnect it, and connect it again from every place the product offers GitHub, through one "Connect GitHub account" action that works whether or not the bex GitHub App is already installed on that account; walked live on production. **Status:** blocked (t001–t009, t012 and t013 done; t010 waits for the production pin to carry `3391d36cc` to re-walk the agent composer row's disconnect and reconnect, and for the user's GitHub sudo-mode 2FA for the first install)

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
| t012 | Agent composer: a workspace without GitHub offers the connect action once the person reaches for a repository — **DONE** | 45m | t009 |
| t013 | Agent composer: the repo list and GitHub readiness follow the connection set while the composer is open — **DONE** | 45m | t012 |
| t010 | Production walk: connect, disconnect and reconnect several GitHub accounts at every starting point | 2h | t008, t009, t012, t013 |
| t011 | Closeout | 15m | t010 |

## Definition of done

1. Every web starting point offers one connect action:
   - Account Settings → Integrations, also reached from the onboarding Getting started checklist;
   - the source picker on `/services/new` for all five service types;
   - the source picker on `/blueprints/new`;
   - the source picker in Service Settings → Update Source;
   - the agent new-session composer.

   From that action, an account that already has the bex GitHub App (`puncsky`, `bex-co`) connects without reinstalling and without GitHub's Configure dead end. An account without the App installs and connects: one of the user's orgs that lacks it (the 2026-11-21 unblock; `BlockEdenHQ` stays installed). No starting point leaves the user on github.com with nothing bound.

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
   - GitHub's install page (`/apps/bex-co/installations/select_target`) shows **Configure** for `puncsky` (`/settings/installations/154851602`) and `bex-co`, and drops the signed `state`. This is the known GitHub behaviour recorded in [ADR078 §3a](../../../../docs/ADR078-github-workspace-connections.md).
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
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), the Git connections row of [ADR018](../../../../docs/ADR018-render-parity.md)).
  - On Render, one credentials control reaches every account the user has installed the App on, without reinstalling.
  - bex binds installations to workspaces ([ADR078 §1](../../../../docs/ADR078-github-workspace-connections.md), a recorded divergence of mechanism), but it must reach the same observable outcome.
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

## Unblock (2026-11-20)

- **Affected:** t010, then t011.
- **Source:** verified evidence (`/loopx w1` dispatch, gate check).
  - The newest `chore(deploy): pin platform images to …` on origin/main is `4e37ec03f`, which pins `12716d1d7097`. It came from deploy run https://github.com/bex-co/bex/actions/runs/44701654499, and every job in that run was green.
  - `git merge-base --is-ancestor 67e156851 12716d1d7097` holds, so production carries t001–t009.
- **Gate:** the `pin must carry` gate is cleared. The remaining work needs only the headed Playwright browser's bex and GitHub sessions. A GitHub sign-in or 2FA prompt, if one appears, needs the user.
- **Next:** t010's production walk, then t011's closeout if every matrix cell passes. First confirm that the live dashboard serves the new code: the Credentials menu shows a single connect action and no "Claim installed account" button.

## Park (2026-11-20)

- **Walk 1** ran on pin `12716d1d7097`; t010 holds the matrix.
  - Passed: Settings, all five `/services/new` types, `/blueprints/new` and Update Source, for connect, disconnect, reconnect and input kept. The Credentials menu focus check passed too.
  - Failed: the agent composer row. t012 fixes it in `210793c9b`.
- **Gate 1, which clears by itself:** ``pin must carry `210793c9b` ``. Then re-walk t010's agent composer row in a workspace with no GitHub connection (`bex-canary` has none):
  1. Add repository or session → Repositories.
  2. The callout appears.
  3. Connect GitHub account opens the chooser in a new tab.
  4. The repos appear in the composer.
  5. The "GitHub settings" link lands on `/settings#integrations`.
- **Gate 2, which needs the user:** the BlockEdenHQ first install, the matrix's "install on a new account" column. GitHub's sudo mode ("Confirm access": GitHub Mobile, passkey, authenticator or emailed code) guards uninstalling the App from BlockEdenHQ, so the user does one of these:
  - uninstalls the bex.co App from BlockEdenHQ first, in org settings → GitHub Apps → bex.co → Uninstall;
  - or is at the walk browser to approve that 2FA.

  The worker then installs the App on BlockEdenHQ again through bex's "Install on another GitHub account", checks that the binding lands in the walking workspace, and reconnects BlockEdenHQ to `bex`.
- Once both gates are walked and pass, t011 closes the milestone.

## Unblock (2026-11-21)

- **Affected:** t010, then t011.
- **Source:** verified evidence for gate 1; the user's reply plus the loop's own choice for gate 2.
  - **Gate 1 cleared.** The newest `chore(deploy): pin platform images to …` on origin/main is `7474e1c6a`, which pins `820edf28d22d`. `git merge-base --is-ancestor 210793c9b 820edf28d22d` holds, so production carries t012.
  - **Gate 2, the user's reply.** Shown that the BlockEdenHQ first install needed their GitHub 2FA, the user replied on 2026-11-21: "什么意思？你不能自己解决吗？" ("What do you mean? Can't you solve it yourself?").
- **Chosen behavior.** The `/loopx w1` orchestrator changed the first-install subject. This choice is the loop's, not the user's:
  - The App stays installed on BlockEdenHQ. Uninstalling needs GitHub's sudo-mode 2FA, which is the user's alone.
  - Instead, the bex.co App is installed on one of the user's GitHub orgs that does not have it yet. The org is taken from GitHub's install picker (`installations/select_target`), which shows it as an install target and not as Configure.
  - On GitHub's install page the worker chooses **Only select repositories** and selects exactly one repository, preferably a small or public one, so the grant stays minimal.
  - If GitHub turns the install into an install _request_ (`puncsky` is not an owner of that org), that cell becomes the live check of t003's `install_requested` path, and the worker tries the next org.
  - The install starts from bex's chooser ("Install on another GitHub account") in `bex-canary`. The worker confirms that the new account binds there and its repo appears, then disconnects it from `bex-canary`. The App stays installed on GitHub, since uninstalling needs the user's 2FA. t010 names the org and the one granted repo, so the user can remove it later.
- **Rationale.** The first-install mechanism is the same for any account without the App: GitHub's install page, the setup URL with `installation_id` and the signed `state`, then the bind in the workspace that started it. Which account is installed does not change what the cell checks. Render's GitHub connect also installs its App on an account from GitHub's own picker.
- **Verified before moving:** `gh api user/memberships/orgs` shows `puncsky` as admin of `cuckoo-network`, `sugarsnap-ai`, `jingyueAItec` and `MagicWandAI`, and GitHub's install picker lists each of them as an install target (`puncsky`, `bex-co` and `BlockEdenHQ` show Configure).
- **Remaining gates:** none known. If GitHub asks for a sign-in or 2FA during the install, that step needs the user, and the milestone parks with exactly that.
- **Next:**
  1. Re-walk t010's agent composer row in `bex-canary` while it has no GitHub connection.
  2. Walk the first install through the chooser.
  3. Record the "install on a new account" column for the other rows. One live first install through the chooser covers the mechanism they share.
  4. Run t011 if every cell passes.

## Park (2026-11-21)

- **Walk 2** ran on pin `820edf28d22d`; t010 holds its matrix.
  - Passed in the agent composer: connect through the callout (new tab, chooser, repos listed with no reload, input kept), reconnect after a reload, and the "GitHub settings" link landing on `/settings#integrations`.
  - Failed in the agent composer: disconnect and reconnect. A disconnect made in another tab left the account's 27 repos listed. After a same-tab disconnect of the last account, the composer kept its cached "connected" readiness and offered no connect action. t013 fixes both in `3391d36cc`.
  - The first install reached GitHub's install page for `cuckoo-network`, with Only select repositories and the one public repo `cuckoo-network/stable-diffusion-miner-docker`. Install & Authorize opened GitHub's **Confirm access**, its sudo mode. Installing on an org needs the user's 2FA, as uninstalling does. Nothing was installed.
- **Gate 1, which clears by itself:** ``pin must carry `3391d36cc` ``. Then a worker re-walks the composer row's disconnect and reconnect in `bex-canary`:
  1. Connect BlockEdenHQ through the callout.
  2. With the composer open, disconnect BlockEdenHQ from Settings in another tab. On returning to the composer's tab, its repos leave the repository list, and reaching for a repository shows the callout.
  3. Connect it again through the callout.
  4. Disconnect it from Settings in the same tab and go back to Agents. Reaching for a repository shows the callout again.
- **Gate 2, which needs the user:** GitHub's sudo-mode 2FA for the first install. The user does one of these:
  - is at the walk browser to approve **Confirm access** (GitHub Mobile, a passkey, an authenticator code or an emailed code) when the worker clicks Install & Authorize, after which GitHub asks again only after a few hours of inactivity;
  - or runs the install from bex themselves: `bex-canary` → Account Settings → Connect another account → Install on another GitHub account → `cuckoo-network` → Only select repositories → `stable-diffusion-miner-docker` → Install & Authorize → confirm.

  The worker then checks that `cuckoo-network` binds in `bex-canary` and its repo appears, disconnects it from `bex-canary`, and records that the App stays installed on `cuckoo-network` with that one repository, for the user to remove later.
- Once both gates are walked and pass, t011 closes the milestone.
