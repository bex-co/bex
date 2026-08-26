# w6 · m99 — Auto-Deploy hint and reported state lie about push-deliverability for a repo the connected GitHub App doesn't grant

**Worker:** worker6 **Goal:** the Auto-Deploy hint text (dashboard) and the `autoDeploy`/`autoDeployTrigger` state (REST/GraphQL/MCP) for a repo-backed service only claim "redeploys automatically via the GitHub app" when THIS service's specific repo is actually covered by the workspace's connected GitHub App installation grant — never merely because the workspace has some GitHub connection and the repo string looks like a github.com URL. **Status:** todo

## Background (found live, 2026-08-25/26 `/qa-find-bugs` hunt, 7th run of the day)

Created a Web Service, `qa-20260825-rollback` (`srv-da75qchgoibs73ah006g`, deleted during this hunt's cleanup), via the **Public Git URL** tab from `https://github.com/puncsky/qa-20260825-rollback-test` — a personal repo that is **not** part of the workspace's connected `bex-co` GitHub App installation (confirmed: it never appeared in the dashboard's "GitHub" tab repo picker, which lists only the installation's granted repos). This is exactly the documented workaround for the still-open `w6/m97` clone-secret bug, which explicitly recommends Public Git URL for a repo the connection does not cover.

Pushed a second real commit (`64bb3b9`) to `master` and waited over 90 seconds at human pace: the Deploys tab stayed at "1 deploy" the entire time. Only a manual "Deploy latest commit" click produced a second deploy — confirming a push to this repo cannot trigger a deploy (expected: bex's GitHub App has no installation on this repo, so it can never deliver a signed push webhook for it).

Yet the service's own Settings → Build & Deploy card showed:

```
Auto-Deploy: On Commit
"A push to the tracked branch redeploys automatically via the GitHub app."
```

— a specific, false, mechanism claim. And the backend agrees with the false claim across every surface: the MCP `get_service` tool (first exercised this run, over `POST https://api.bex.co/mcp` using the QA browser session cookie — no prior hunt had driven MCP) returned, for this exact service:

```json
{"autoDeploy":"yes","autoDeployTrigger":"commit", ...}
```

REST and GraphQL report the identical stored field (`lego/backend/internal/apps/render.go:280-281`, mapping `a.AutoDeploy` via `yesNoEnum`/`triggerEnum`) — this is not a REST/GraphQL/MCP divergence (all three agree), it is all three agreeing with something false.

## Root cause

**Dashboard hint** (`dashboard/src/features/services/components/build-deploy-section.tsx:135`):

```ts
const viaGitHub = !!connection?.connected && /github\.com[/:]/i.test(repo);
```

This checks only (a) whether the workspace has **any** connected GitHub integration, and (b) whether the repo string's host looks like `github.com` — never whether **this specific repo** is among the connection's actual granted/installed repos. The same component already fetches that exact grant list via `useRepos()` (line ~144, feeding `repoOptions`) — it just never cross-references `repo` against it. The hint then picks copy at lines 179-181:

```ts
hint={viaGitHub ? t("services.autoDeployViaGitHub") : t("services.autoDeployViaWebhook")}
```

`services.autoDeployViaGitHub` = "A push to the tracked branch redeploys automatically via the GitHub app." — asserted whenever the coarse check passes, regardless of whether a push from this repo can ever reach bex.

**Backend state** (`lego/backend/internal/apps/service.go:2342`):

```go
autoDeploy = req.Repo != ""
```

`normalizeCreateDefaults` defaults `autoDeploy` to true for **any** repo-backed create — matching Render's own "on by default" behavior, so this default itself is not the defect (see Adjacent classes below) — but nothing anywhere in the App's lifecycle records or exposes whether the repo is actually covered by a GitHub App grant. `render.go:280-281` reports the stored boolean unconditionally as `"yes"`/`"commit"` over REST, GraphQL, and MCP alike, so a client reading any of the three surfaces sees the same unverifiable-until-you-push claim the dashboard hint makes.

**The exact grant-match signal already exists** and is computed on every deploy trigger: `lego/backend/internal/apps/clonesecret.go:106-121` `mintCloneSecret` calls `s.GitHub.CloneToken(ctx, workspaceID, repo)`, whose `ok` return is precisely "does this repo belong to this workspace's GitHub connection." This milestone's fix is to surface that same signal as an explicit, checkable field/state rather than re-deriving it, so the guarantee stays exact even as the grant list changes.

## Verified control case (not touched by this bug) and its own untested gap

`w2/m9` (the milestone that shipped Auto-Deploy + the hint text) verified the **working** case live: "Build & Deploy shows the Auto-Deploy switch with the 'redeploys automatically via the GitHub app' source line (connection live + github.com repo)" — a repo that genuinely was covered by the connection. It never constructed the case this hunt found: connection live, repo **is** a github.com URL, but **not** covered by that connection's grant. `viaGitHub`'s regex-plus-connected check cannot distinguish the two — verifying only the passing case left this gap open since `w2/m9` shipped.

## Blast radius

- **Frontend:** exactly one call site computes `viaGitHub` and selects the hint (`build-deploy-section.tsx:135,179-181`); it is shared by every resource type with a Build & Deploy card (web, private, background worker, static site, cron job — confirmed via the component's own doc comment listing all five).
- **Backend:** `autoDeploy`/`autoDeployTrigger` are reported from the one stored `a.AutoDeploy` field via `render.go:280-281` (REST + GraphQL share this mapper) and via the MCP `get_service`/`list_services` tools' structured content (same underlying service read path) — three surfaces, one source of truth, confirmed live-consistent (all three currently agree, all three currently wrong for an ungranted repo).
- **Not affected:** a repo actually covered by the connection (the `w2/m9` control case) — must stay exactly as-is; this milestone must not flip a correct "via GitHub app" claim to the manual-webhook one for a genuinely-covered repo.

## Adjacent classes

- **`autoDeploy: true` as a stored default is not the defect.** Render's own default is "on" for any repo-backed create regardless of webhook deliverability — bex matches that intentionally (`service.go:2339-2340`'s own comment). The defect is specifically the **mechanism claim** ("via the GitHub app" vs. "needs a manual webhook") layered on top of that boolean, not the boolean's default value. Do not change the default-on behavior.
- **A repo with no GitHub connection at all** (`connection?.connected` false) already and correctly shows the manual-webhook hint (`viaGitHub` is false) — unaffected, no change needed there.
- **A non-github.com repo host** (e.g. a self-hosted GitLab URL) already and correctly shows the manual-webhook hint — unaffected.

## Unverified this run

- Multi-installation workspaces (per ADR026, a workspace can have N GitHub App installations): whether `useRepos()`'s list already unions every installation's grants, or only the "active"/first one — the fix must check against the full union, not assume single-installation.
- Exact URL-normalization needed to match `repo` against a granted repo's `htmlUrl` (case, trailing slash, `.git` suffix) — not exercised this run; the fix should normalize both sides the same way rather than a naive `===`.
- The `w6/m97` control-case claim itself ("Public Git URL only sidesteps [the clone-secret bug] for repos the connection does not cover") is reconfirmed accurate by this hunt's own repro (the ungranted repo built and deployed cleanly via Public Git URL) — cited here for context, not re-verified as a separate task.

## Tasks (in order)

| id   | title                                                                                                                                                      | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend: surface the existing per-repo GitHub-grant-match signal (`GitHub.CloneToken`'s `ok`) as an explicit, checkable field so REST/GraphQL/MCP can report push-deliverability truthfully instead of the unconditional stored `autoDeploy` boolean alone | 45m | —          |
| t002 | Dashboard: fix `viaGitHub` (`build-deploy-section.tsx:135`) to derive from the per-repo grant signal (t001's field, normalized against `useRepos()`'s union list) instead of `connection.connected && /github\.com/.test(repo)`; update hint copy/props | 30m | t001       |
| t003 | Regression tests: an ungranted github.com repo + live connection → manual-webhook hint and non-"yes via GitHub" reporting; the `w2/m9` control case (granted repo) unchanged; REST/GraphQL/MCP agree on both cases | 40m | t002       |
| t004 | Render parity                                                                                                                                              | 20m | t003       |
| t005 | Simplify                                                                                                                                                   | 15m | t004       |
| t006 | Test coverage                                                                                                                                              | 20m | t004       |
| t007 | Closeout                                                                                                                                                   | 10m | t006       |

## Definition of done

- A fresh App (web, private, background worker, static site, or cron job — the DoD only requires two of the five, matching this board's usual live-verification bar) created from a github.com repo that is **not** covered by the workspace's connected GitHub App installation shows an Auto-Deploy hint stating delivery needs a manual webhook, never "via the GitHub app" — live-verifiable on Settings → Build & Deploy for such a service.
- The same service's REST `GET /v1/services/{id}`, GraphQL `server(id){autoDeploy autoDeployTrigger}`, and MCP `get_service` structured content all agree with that same per-repo signal — live-verifiable with the exact MCP JSON-RPC call this hunt used (`tools/call` → `get_service` → inspect `autoDeploy`/`autoDeployTrigger` alongside the new field).
- The `w2/m9` control case (a repo actually covered by the connection) is unchanged: still shows "via GitHub app," still reports `autoDeploy: "yes"` / `autoDeployTrigger: "commit"`, and a real signed push to it still redeploys — regression-tested, not just reasoned about.
- A real push to an ungranted repo's tracked branch still produces zero auto-triggered deploys (mechanics unchanged; only the product's claim about them changes).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 7th run of the day, 2026-08-25/26. Workspace `tea-d98210cbbpdc73dcrkvg`, service `srv-da75qchgoibs73ah006g` (`qa-20260825-rollback`, deleted during cleanup), repo `https://github.com/puncsky/qa-20260825-rollback-test` (made private during cleanup — GitHub CLI token lacked `delete_repo` scope, flagged in the hunt's Phase 8 report as an outstanding manual cleanup item, not a resource this milestone needs to touch). Evidence: the live push-then-wait repro (commit `64bb3b9`, Deploys tab stayed at "1 deploy" for >90s), the Settings page snapshot showing the false hint, and the raw MCP request/response quoted above (`tools/call get_service` → `"autoDeploy":"yes","autoDeployTrigger":"commit"`).
- **Goal linkage:** [ADR004](../../../docs/ADR004-app-deployment.md) (deploy is the platform's single most fundamental promise) and [ADR026](../../../docs/ADR026-github-integration.md) (governs exactly this push-to-deploy mechanism and its per-repo grant model). Intersects `w6/m97`'s own documented workaround: that milestone's README states "Public Git URL only sidesteps [the clone-secret self-rejection] for repos the connection does not cover" — this milestone is the previously-unverified cost of exactly that workaround combination (GitHub-connected workspace + an ungranted github.com repo), which `w6/m97` never itself needed to check since its bug is about the clone secret, not the auto-deploy hint.
- **Expected outcome:** a user who follows the `w6/m97` workaround (or independently pastes any github.com URL not covered by their installation) sees an accurate statement of whether/how push-to-deploy works for that specific service, instead of a specific false promise ("via the GitHub app") that silently never fires.
- **Why now:** freshly discovered, live-reproducing, multi-surface (UI + REST + GraphQL + MCP) misrepresentation of the platform's core "a later git push redeploys" promise (ADR008 pillar 4), for a state the current `m97` workaround actively steers users into. `w2/m9`'s own verification only ever exercised the passing case; this is the first time the failing case was checked.
- **Render parity task included:** yes (t004) — UI, REST, GraphQL, and MCP all report this state and must move together, and Render's own equivalent behavior (repo must belong to a connected account before a service can be created from it at all — Render has no public-git-URL-for-an-unrelated-account concept the way bex's Public Git URL tab does) should be recorded as the comparison point / documented divergence.
