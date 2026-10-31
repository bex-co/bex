# w2 · m162 — One GitHub account, many workspaces: the N:N connection model

**Worker:** worker2 **Goal:** the same GitHub account can back more than one workspace, and every GitHub-connect path that used to dead-end now completes or says exactly why **Status:** todo — t001–t009 done; resumed 2026-10-30: the user is available in this session for the GitHub sign-in step of the production two-workspace walk that the worker drives (t010)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                 | status      |
| ---- | ----------------------------------------------------------------------------------------- | --- | -------------------------- | ----------- |
| t001 | Migration + store: composite key, plural lookup, symmetric quota, one-way-door down guard | 45m | —                          | — **DONE**  |
| t002 | Service: drop the bound-elsewhere filter, bind N:N, two coded quota refusals               | 40m | w2/m162/t001               | — **DONE**  |
| t003 | Push-webhook fan-out with the fail-closed invariant restated (own commit)                  | 1h  | w2/m162/t001               | — **DONE**  |
| t004 | Deferred claim selection: proved-candidate memo + `selectGitClaim`                         | 1h  | w2/m162/t002               | — **DONE**  |
| t005 | Surfaces: claim selector + quota codes on REST/GraphQL/MCP, env inventory                  | 40m | w2/m162/t004               | — **DONE**  |
| t006 | Dashboard: account picker, honest callback copy, install-URL dead-end exit                 | 1h  | w2/m162/t005               | — **DONE**  |
| t007 | Render parity check across REST/GraphQL/MCP/dashboard                                      | 30m | w2/m162/t003, w2/m162/t006 | — **DONE**  |
| t008 | Simplify the code this milestone changed                                                   | 30m | w2/m162/t007               | — **DONE**  |
| t009 | Test coverage for N:N, fan-out, and claim selection                                        | 1h  | w2/m162/t007               | — **DONE**  |
| t010 | Closeout                                                                                   | 20m | w2/m162/t009               | open        |

## Definition of done

On `dev-2`, with the GitHub App configured:

1. **One account, two workspaces.** An installation already bound to workspace A binds to workspace B through the claim flow, on B's own three-proof sequence, without uninstalling, reinstalling, or disconnecting from A. `GET /v1/git/connections` lists it for both; `DELETE /v1/git/connections/{installationId}` on B leaves A's connection intact.
2. **No path dead-ends silently.** Claiming an installation bound elsewhere succeeds instead of reporting `no_claimable_installation`. An admin who administers several installations lands on an **account picker** instead of an unresolvable `ambiguous_installation`. The install-URL case (GitHub strips `state` for an already-installed account) routes the user to the claim flow instead of leaving them on a GitHub settings page with no feedback.
3. **Fan-out is bounded and fail-closed.** One `git push` to a repo tracked in both A and B produces one deploy in each, with independent replay claims. An unbound installation, a delivery with no installation id, and an unwired resolver each act on **nothing** in multitenant operation — never a global match.
4. **Both quotas refuse identically** on REST, GraphQL and MCP: `GIT_CONNECTION_LIMIT` (connections per workspace) and `GIT_INSTALLATION_WORKSPACE_LIMIT` (workspaces per installation), both admitted inside the insert's store transaction.
5. **Selection grants no new authority.** A `selectGitClaim` naming an installation outside the stored proved set, presented by a different subject, replayed, or past its TTL binds nothing.
6. **Rollback guard holds.** The down migration succeeds while every installation serves one workspace and refuses with a named exception once any serves two.
7. Backend + dashboard suites green; `docs/ADR078-github-workspace-connections.md` §2/§3a/§4a match what shipped.

A live production walk (two real workspaces, one real installation) is the final confirmation and is recorded in t010, but the DoD above is fully demonstrable on `dev-2`.

## Source + Goal linkage

- **Source:** live user report 2026-09-16 — "github account connect 和绑定好像有问题，无法多个 workspace 绑定同一个 github account，从而永远无法绑定". Code triage confirmed three independent dead ends (install URL strips `state`; the claim filter discarded bound-elsewhere installations and reported "install the App first"; direct github.com installs are a no-op). Design decision: [`docs/ADR078-github-workspace-connections.md`](../../../docs/ADR078-github-workspace-connections.md) §2 (cardinality reversed N:1 → N:N), §3a (claim candidate rule + deferred selection), §4a (fan-out fail-closed restatement), revised 2026-09-16; [`ADR026`](../../../docs/ADR026-github-integration.md) §4 and [`ADR018`](../../../docs/ADR018-render-parity.md) synced in the same change.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 3–4 — "which of my repos can you deploy?" and "a later git push redeploys" must hold for private repos with no per-repo setup. Neither holds for a user whose one GitHub account has to serve more than one workspace, which is the ordinary shape for anyone with a personal workspace and a team workspace.
- **Expected outcome:** the last *observable* Render behaviour bex lacked is closed — Render reaches it by never binding an installation, bex by proving each binding independently. The mechanism divergence (workspace-owned, not per-user credentials) stays deliberate and is now argued in ADR078 §1 rather than merely asserted.
- **Why now:** it is a reported, reproducible, total block — the affected user has no workaround short of disconnecting the other workspace, and the product's own error copy sends them the wrong way ("install the App first" when the App is installed). It also has to land as one change: the schema step only relaxes a constraint, so splitting it buys no safety, and a phased version would ship an `installation_claimed_elsewhere` error state that the same milestone then deletes.
- **Render parity task included** because this changes REST, GraphQL, MCP and the dashboard (claim selector, quota codes, callback copy).
- **No `DO_NOT_DO.md` conflict:** the GitHub App integration is sanctioned roadmap; the banned adjacent items are GitLab/Bitbucket providers and PR preview environments, neither of which this touches.

## Unblock 2026-10-29 — production credential gate cleared; resumed for the live walk

- **Affected:** `w2/m162/t010`. **Source:** evidence verified by `/unblock` (read-only, `hetzner-prod`, main `7fd6c9c4f`), plus an explicit user decision in `/unblock w2` on 2026-10-29: "I'll do it; the worker leads the walk with Playwright."
- **Evidence:** `bex-system/deploy/bex-api` injects `BEX_GITHUB_APP_CLIENT_ID`/`_SECRET` from secret `bex-github-app` (keys `client-id`, 20 bytes; `client-secret`, 40 bytes; values not read). So production's installation-admin verifier is configured, and ADR078 §7's start-time refusal no longer applies. The local `.env` still lacks the pair. That no longer blocks, because the walk runs on production. Production `git_connections` has 3 installations (`90623475`, `154851602`, `169230252`), each bound to exactly **one** workspace, so the N:N walk has never run live.
- **Decision:** run DoD points 1–2 on **production**. The worker drives the browser with Playwright. **The user** performs only the GitHub sign-in and App authorization/installation steps in that browser (no GitHub credential is stored in the repo or given to the worker).
- **Remaining gates:** the user must be present for the GitHub step. If they are not available when the worker reaches it, park only t010 again, with the prepared state recorded. That is a scheduling pause, not a new decision.
- **Worker next steps (t010):** (1) Prepare two production workspaces the user administers (reuse existing ones, e.g. `bex` plus a second first-party workspace, rather than creating a payment-gated one), and one throwaway repo under a GitHub account the user controls, with a fixture service in each workspace pointing at it. (2) In Playwright, start **Connect GitHub** in workspace 1 and hand the GitHub sign-in/authorize step to the user. Then do the same in workspace 2 with the same account (no dead end, the account picker offers it). (3) Confirm `git_connections` shows one installation with 2 workspaces. (4) Push once and confirm one deploy in each workspace. (5) Disconnect workspace 2, push again, and confirm workspace 1 still deploys. (6) Clean up the fixtures, record the evidence here, then finish t010's steps 3–5.

## Walk attempt 2026-10-29 — parked again (user not available for the GitHub step)

Prepared on production: QA session, `bex-canary` selected, **Settings → Integrations → Claim installed account** clicked. The browser reached GitHub's sign-in page for the bex App's OAuth (`client_id=Iv23lil8ErzBmFL5VUjf`), and the user was asked to sign in as `puncsky`. No answer came during the run. Fixtures were then cleaned up: the private repo `puncsky/bex-m162-walk` was deleted, the service `qa-20261029-m162-a` in `tian-personal` was deleted (`GET` → 404), and both QA sessions were logged out.

**To resume (about 20 minutes, the user present for step 2):**

1. Recreate the fixture. `gh repo create puncsky/<throwaway> --private` from `examples/hello-go`, then add it to installation `154851602`: `gh api -X PUT /user/installations/154851602/repositories/<repo_id>` (204). Create a free docker web service from it in `tian-personal` (already bound to `puncsky`).
2. Run `scripts/qa-login.sh --serve`, inject the cookies into Playwright, switch to `bex-canary`, open `/settings`, and click **Claim installed account**. **The user signs in to GitHub as `puncsky` and authorizes.**
3. Verify that `git_connections` (read-only) has `154851602` bound to both `tian-personal` and `tea-daif693dqjvc73e7as3g`. Create a second fixture service in `bex-canary`, push once, and confirm one deploy in each workspace. Disconnect `bex-canary`, push again, and confirm `tian-personal` still deploys.
4. Delete the fixtures and repo, log out, and record the evidence here. Then finish t010.

## Unblock 2026-10-30 — user available in this session; resumed

- **Affected:** `w2/m162/t010`. **Source:** explicit user decision in `/unblock w2` on 2026-10-30: available in this session for the GitHub sign-in step.
- **Decision:** unchanged from `## Unblock 2026-10-29` (production walk, the worker drives Playwright, the user only signs in to GitHub as `puncsky` and authorizes). Follow the four resume steps in `## Walk attempt 2026-10-29`.
- **Render basis:** on Render one GitHub account backs any number of workspaces with no reinstall; this walk proves the same observable behavior on bex.
- **Remaining gates:** the user's GitHub sign-in at step 2. Prepare step 1 first, then call the user at the sign-in page. If the user does not answer during the run, clean up the fixtures and park only t010 again (scheduling pause, not a new decision).
- **Acceptance:** `git_connections` shows `154851602` bound to both `tian-personal` and `bex-canary`; one push → one deploy in each; disconnect `bex-canary` → a second push still deploys in `tian-personal`; fixtures deleted; evidence recorded here; then t010 steps 3–5.
