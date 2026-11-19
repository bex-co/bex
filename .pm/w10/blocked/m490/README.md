# w10 · m490 — A renamed or transferred GitHub repository keeps deploying

**Worker:** worker10 **Goal:** a push to a GitHub repository that was renamed or transferred after a service or Blueprint was connected to it still deploys the service (or syncs the Blueprint), because bex matches GitHub's stable repository id rather than the URL it stored, and the stored `repo` follows the new name. **Status:** blocked (t001–t004 and t006–t008 done 2026-11-18; t005, the production rename check, waits for the production deploy that carries this milestone; then t009 closes it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Store GitHub's repository id for every git-backed service and Blueprint, at create or connect, and backfill existing ones through the installation token — **DONE** | 1h | — |
| [t002](done/t002.md) | An App-signed push matches services and Blueprints by repository id first, by URL only where no id is stored — **DONE** | 1h | t001 |
| [t003](done/t003.md) | A push whose repository id matches but whose URL differs moves the stored `repo` to the new canonical URL and records the change — **DONE** | 1h | t002 |
| [t004](done/t004.md) | A connected service whose repository the GitHub App can no longer read says so on Settings → Source — **DONE** | 45m | t001 |
| [t005](t005.md) | Production check: rename a QA repository, push, and the service deploys and reads the new URL — **BLOCKED (production deploy)** | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t003, t004 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t005, t008 |

Total: **9 tasks, about 5h55m**.

## Definition of done

1. A GitHub App-signed push to a repository renamed after its service was connected deploys that service, and to a Blueprint's repository syncs the Blueprint; a test drives a push whose payload URLs carry the new name against an App that stores the old one, and fails on today's code.
2. After that push, REST `GET /v1/services/{id}`, GraphQL, MCP and the dashboard's Settings → Source read the new repository URL.
3. Id matching stays inside the delivering installation's workspace scope (codex #7); a manual or Gitea push webhook, which carries no installation, still matches by URL only.
4. On production, after the deploy that carries this milestone, as the QA user in `bex-canary`: a QA repository connected to a service is renamed, a commit is pushed, the service opens a deploy within 2 minutes, and its `repo` reads the new URL. The QA repository and service are deleted afterwards.
5. ADR026 records the id-matching rule, and ADR018 records the rename behavior and its basis.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w10 for only one aspect of render parity (focus on the deep core values)` on 2026-11-17/18, item 1. The follow-up `/dont-code-but-answer` found that item's evidence was a repository rename, and the user chose option B ("按 repository id 跟随改名（方案 B）") with `/pmx` on 2026-11-18.
- **Evidence (verified 2026-11-18, read-only):**
  - `tianpan-v4-web` (`srv-da40m1qii7bs73drbqlg`) has `autoDeploy: true`, `branch: main` and `rootDir: projects/tianpan-v4-web`. Twelve commits touching that directory landed on `puncsky/tpkb` `main` from 2026-10-19, and none produced a git-triggered deploy, a `commit_ignored` event or a log line. Commit `7dd47f81e8` (2026-11-18 03:51:46Z) got a `200` from `POST /v1/webhooks/git` four seconds later and nothing else.
  - The repository was renamed: `gh api repos/puncsky/tianpan-v3` answers `full_name: puncsky/tpkb` (GitHub redirects the old name), and a local clone's remote still reads `puncsky/tianpan-v3`.
  - bex matches by URL only. `repoCandidates` (`lego/backend/internal/apps/webhook.go:948`) and `blueprintAutoSyncTargets` (`:875`) call `repoURLsMatch` (`:1177`, `core.CanonicalRepo`) on the stored repo. `gitRepository` (`:221`) does not decode the payload's `repository.id`, and a delivery with no candidate returns `200` and records nothing (`:567`). Builds kept working because GitHub redirects clones of the old name, so the service looked healthy.
  - The service recovered by hand: the audit log shows `apps.SetSourceAndRegistryCredential` at 2026-11-18 07:54:03Z and `deploys.Trigger` at 07:54:15Z. Its stored `repo` now reads `https://github.com/puncsky/tpkb`.
  - Of the 65 distinct `apps.repo`/`blueprints.repo` values on production, no other repository the checking token could read is renamed today; six answered 404 and could not be checked.
- **Render:** render.com/docs/github and render.com/docs/deploys say nothing about renamed or transferred repositories. They promise "Whenever you push or merge a change to that branch, by default Render automatically rebuilds and redeploys your service." Third-party write-ups, which cite no Render source, report that a rename or transfer breaks Render's connection until the user reconnects the repository (runxbuild.com/blog/render-github, codeables.dev, a 2026-10 GitHub issue). No Render capture exists to settle it (`w10/blocked/002`'s `RENDER_API_KEY` gate). Render's `PATCH /services/{id}` takes `repo` as a URL string.
- **Decision state:** the approach (option B: follow the rename by repository id) is the user's choice of 2026-11-18. It keeps Render's documented promise. If Render really does break on a rename, as the third-party reports say, this is behavior where bex is ahead of Render. t006 records it in ADR018 § bex ahead of Render, citing that choice and the unverified basis. Option A (break as Render reportedly does) and option C (warn only) were not chosen.
- **Goal linkage:** ADR008's mission ("`git push` becomes a running HTTPS service") and V0 roadmap item 3 (git push to deploy).
- **Expected outcome:** renaming or transferring a repository no longer stops its services from deploying, and no push goes unanswered without a trace because of a stale URL.
- **Why now:** one real service missed a month of pushes without any signal, and the only recovery was a manual source change. Every repository rename does the same thing to every service and Blueprint on it, silently.
- **Render parity:** included. The stored `repo` read by REST, GraphQL, MCP and the dashboard changes after a rename, and Settings → Source gains an access warning.

## Gate (2026-11-18)

Parked in `blocked/` with every implementable task done. t005 replays definition of done item 4 on production, which needs the production deploy that carries the commit `feat(api): a push follows a renamed GitHub repository by its id (w10/m490)`. The gate clears by itself: once the production pin carries that commit (`bash scripts/fetch-app-kubeconfig.sh`, or the `hetzner-prod` context, then the `bex-api` Deployment's image tag in `bex-system`), a worker runs t005 as the QA user and t009 closes the milestone. No user action is needed.
