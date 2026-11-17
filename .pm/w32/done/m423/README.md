# w32 · m423 — Linking an environment group deploys the service even with Auto-Deploy off, as Render documents

**Worker:** worker32 **Goal:** a link to a repo-backed service whose Auto-Deploy is off deploys it at once with a recorded deploy, as Render does; unlinks and group edits keep waiting for the owner's next deploy. **Status:** done (walked on production pin `55c120858af0`, 2026-11-17)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A link deploys every service it links, Auto-Deploy on or off — **DONE** | 60m | — |
| [t002](done/t002.md) | Surfaces and copy say a link deploys now — **DONE** | 45m | t001 |
| [t003](done/t003.md) | ADR013 rule 3 and the ADR018 env-groups row describe the link rule — **DONE** | 20m | t001 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](done/t007.md) | Closeout (live replay on production) — **DONE** | 45m | t005, t006 |

## Definition of done

On production, in `bex-canary`, with owned free fixtures deleted afterwards (by-id `GET` 404):

1. A repo-backed web service with Auto-Deploy off (`examples/hello-go`, which serves `$MESSAGE`) and a group with `MESSAGE=grp-v1`: linking the group from the service's Environment tab opens exactly one recorded deploy, which goes `live` and serves `grp-v1`. No other deploy row changes to `canceled`.
2. The same link from REST `POST /v1/env-groups/{id}/services/{serviceId}`, the GraphQL link mutation, the MCP link tool, and `POST /v1/env-groups` with `serviceIds` each opens one recorded deploy; the create answer's `pendingServiceIds` does not list the linked service.
3. A repo-backed static site with Auto-Deploy off: a link opens one recorded deploy that rebuilds and serves the group's value.
4. Unlink and group delete on an Auto-Deploy-off service still open no deploy and wait for the owner's next deploy.
5. Unchanged: Auto-Deploy-on and image-backed services (already deploy on link), suspended services (w4/m234), build sources (w2/m254), and links made by a Blueprint sync.
6. `cd lego/backend && go test ./...`, operator `make test` and `make lint`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for Render parity of render.com/docs/environment-variables, 2026-11-16, item 1; claims re-verified the same day before filing (`/pmx … to w32`).
- **Render reference:** render.com/docs/configure-environment-variables, read 2026-11-16. § Linking a group to a service: "That's it! Render kicks off a new deploy of your service to incorporate the values from the linked environment group." No Auto-Deploy condition. § Modifying a group: changes, "including deleting it", deploy "every linked service that has autodeploys enabled". The API reference pages for link, unlink and delete say nothing about deploys.
- **bex today:** `autoDeployGated` (`lego/backend/internal/envgroups/service.go:1180`) holds a link on a repo-backed Auto-Deploy-off service (`rollLinked` `:1201` → `rollout.StageGroupLink`), from w4/m239. That milestone's parity task (`.pm/w4/done/m239/done/t004.md`) compared only against the modify rule, so the link divergence was never recorded in ADR018 with a user sign-off. Image-backed services are never held (the Repo guard), so they already match Render.
- **Goal linkage:** ADR008 Render parity first; ADR004 every release has a recorded deploy row; ADR013 § Precedence and redeploy; ADR018 Environment groups row.
- **Expected outcome:** an owner who links a group sees it take effect immediately, as on Render, whatever the service's Auto-Deploy setting; the held-link state stops existing for links.
- **Why now:** an unsigned divergence resting on a misread Render rule. Auto-Deploy is on by default for repo-backed services, so the owners affected deliberately turned it off, and on Render their link still deploys. w4/536 (blocked on the release gate) adds an `undeployedChanges` signal for held changes; after this milestone it applies to unlinks and group deletes only.
- **Decision state:** deploying on every surface is this proposal's recommendation, resting on ADR008 Render parity first (Render's only statement is unconditional, and REST, GraphQL, MCP and the dashboard share one Core path). It is not a recorded user decision. Deploying from the dashboard only and keeping REST held would be a divergence that needs the user's sign-off in ADR018.
- **Render parity:** included: the change is user-facing on REST, GraphQL, MCP and the dashboard.

## Blocked (2026-11-17)

**Release gate (clears by itself; no decision needed).** t001–t006 shipped in `d49c33edb` (bex-api: `envgroups.rollLinked` deploys an owner's link, `rollout.DropStagedGroupLink`; the dashboard's link and unlink copy in eight locales; ADR013 rule 3, ADR018 and ADR004). Production runs the bex-api and dashboard images of the newest `chore(deploy): pin platform images to <sha>` commit on `main`. That pin was `b767149494fa` when this milestone parked, and it does not contain `d49c33edb`. Check:

```bash
git fetch origin main
pin=$(git log origin/main --grep='^chore(deploy): pin platform images to' -1 --format=%s | sed -E 's/.*images to ([0-9a-f]+).*/\1/')
git merge-base --is-ancestor d49c33edb "$pin" && echo "pinned: walk the definition of done"
```

Then [t007](done/t007.md) walks the definition of done in `bex-canary` as the QA user (`bash scripts/qa-login.sh`), deleting every fixture afterwards with by-id `GET` 404 evidence:

- A repo-backed web service from `examples/hello-go` with Auto-Deploy off and a group with `MESSAGE=grp-v1`: a link from the service's Environment tab opens exactly one deploy, which goes `live` and serves `grp-v1`, and no other deploy row turns `canceled`.
- The same link from REST, GraphQL `linkEnvGroup`, MCP `link_env_group` and `POST /v1/env-groups` with `serviceIds` each opens one deploy, and the create's `pendingServiceIds` does not list the service.
- A repo-backed static site with Auto-Deploy off: a link opens one deploy that rebuilds and serves the group's value.
- An unlink and a group delete on an Auto-Deploy-off service open no deploy, and the next manual deploy carries them.
- Controls unchanged: an Auto-Deploy-on and an image-backed service still deploy on a link, a suspended service's link opens no deploy, and a Blueprint sync's link waits.

## Unblocked (2026-11-17)

**Source: verified evidence (loopx w32), no user decision involved.** The release gate cleared. The newest `chore(deploy): pin platform images to <sha>` commit on `origin/main` is `9c79201b4` (`… to 55c120858af0`, 2026-11-17 07:47 UTC), and `git merge-base --is-ancestor d49c33edb 55c120858af0` succeeds. Production runs that pin: at 07:55 UTC every `bex-system` Deployment (`bex-api`, `bex-controller-manager`, `bex-activator`, …) and its pods ran `ghcr.io/bex-co/bex-operator@sha256:fe4ebdc061d0…`, and `dashboard` ran `ghcr.io/bex-co/bex-dashboard@sha256:f1226b93a76b…`, the digests `9c79201b4` writes into `deploy/gitops/base/bex.yaml` and `dashboard.yaml` (pods started 07:50–07:53 UTC). No gate remains. Next: t007 walks the definition of done in `bex-canary` as the QA user, deletes the fixtures (by-id `GET` 404), records the conclusions here and closes the milestone, or re-parks it with the failure named.

## Production replay (2026-11-17, t007)

Pin `55c120858af0` (carries `d49c33edb`), QA user, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), 07:57–08:36 UTC. REST and GraphQL were called with the QA session, MCP through `https://api.bex.co/mcp` with a per-call `workspaceId`, and the dashboard in headless Chrome (the account's UI language is Chinese). Fixtures: `qa-20261117-m423-web` (`srv-dbu0hvuc1l2s739d3k7g`, `bex-co/bex` `examples/hello-go`, Docker, free, Auto-Deploy off; served `OK` before any link) and group `qa-20261117-m423-g1` (`MESSAGE=grp-v1`, `QA_GRP=grp-v1`). Every deploy below went `live` with `trigger: service_updated` (a manual one: `api`), the row before it went `deactivated`, and no row on any fixture was ever `canceled`.

1. **Dashboard link (holds).** At 08:02:41 the web service's Environment tab, button "关联 qa-20261117-m423-g1" (Link), answered the toast "服务已关联 / 无论自动部署设置如何，服务都会立即使用该组的值进行部署。" (`envGroups.linkSuccess` / `linkChangeNote`: deploys now, whatever the Auto-Deploy setting). It opened exactly one deploy, `dep-dbu0k8dikg6s738lakp0`, live at 08:04:58, and the URL served `grp-v1`.
2. **Every other link surface (holds).** Each link below opened exactly one deploy that went live, and the URL served the group's value afterwards. The first three each followed an unlink (item 4).
   - GraphQL `linkEnvGroup` at 08:08:59: `dep-dbu0n6tikg6s738lalbg`, live at 08:11:10.
   - MCP `link_env_group` at 08:12:21: `dep-dbu0opdikg6s738lalvg`, live at 08:14:34.
   - REST `POST /v1/env-groups/{id}/services/{serviceId}` at 08:15:58: `dep-dbu0qfmc1l2s739d3oe0`, live at 08:18:10.
   - REST `POST /v1/env-groups` with `serviceIds: [web]` (`qa-20261117-m423-g2`, `MESSAGE=grp-v2`) at 08:22:46. The answer carried no `pendingServiceIds`. It opened `dep-dbu0tllikg6s738laog0`, live at 08:24:56, and the URL served `grp-v2`.
3. **Static site (holds).** `qa-20261117-m423-site` (`srv-dbu0i1dikg6s738lak6g`, `examples/static-site`, build `printf "grp=%s\n" "$QA_GRP" > v.txt`, Auto-Deploy off) served `/v.txt` = `grp=`. A REST link at 08:05:14 opened one deploy, `dep-dbu0letikg6s738lal0g`. It rebuilt, went live at 08:08:45, and `/v.txt` read `grp=grp-v1`.
4. **Unlink and group delete wait (holds).** Unlinks by REST (08:05:14 and 08:18:15), GraphQL `unlinkEnvGroup` (08:12:16) and MCP `unlink_env_group` (08:15:57) added no deploy row. After the 08:05 unlink the URL kept serving `grp-v1` for 3m45s, until the next link. After the 08:18 unlink it served `grp-v1` until the manual deploy `dep-dbu0sk6c1l2s739d3t1g` at 08:20:31, which served `OK`. Deleting g2 at 08:25:10 (by-id `GET` 404) also added no row. The URL kept serving `grp-v2` until the manual deploy `dep-dbu100tikg6s738lappg` at 08:27:46, which served `OK`. The web service ended with 8 rows: its first deploy, five links and two manual deploys.
5. **Controls unchanged (holds).**
   - **Image-backed:** `qa-20261117-m423-img` (`docker.io/traefik/whoami`, Auto-Deploy off). A REST link of g1 at 08:08:59 opened `dep-dbu0n6uc1l2s739d3li0`, live at 08:09:14.
   - **Auto-Deploy on:** the static site was switched to Auto-Deploy on. A link of `qa-20261117-m423-g3` at 08:12:47 opened `dep-dbu0p05ikg6s738lam40`, live at 08:16:30.
   - **Suspended:** the image service was suspended at 08:12:32. A REST link of g3 at 08:12:38 opened no deploy. So did `POST /v1/env-groups` with `serviceIds: [img]` at 08:16:14 (`qa-20261117-m423-g4`), which answered `pendingServiceIds: [img]`. Neither had a row by 08:30:12.
   - **Blueprint sync:** `POST /v1/blueprints/deploy` created `qa-20261117-m423-bp` (`srv-dbu0ng6c1l2s739d3lr0`, Auto-Deploy off) and group `qa-20261117-m423-bpg` (`MESSAGE=bp-v1`), unlinked. The service served `OK`. Re-applying the file with `fromGroup` at 08:12:10 linked the group: it lists the service in `serviceLinks`. The re-apply ran the existing-service path (`LinkEnvGroup`, `syncLink`), which Blueprint create, sync and auto-sync share. The service gained no deploy row and kept serving `OK` through 08:30:12.
   - **Build source:** `qa-20261117-m423-bs` (`bds-dbu120dikg6s738laqsg`, git, `examples/hello-go`) finished its first build at 08:33:43. `POST /v1/build-sources/{id}/env-groups/{envGroupId}` at 08:33:50 answered 200 and started no build by 08:35:51.
6. **Checks (hold).** On `55c120858af0` itself:
   - deploy run 44039100056 passed `test-backend` (`go test ./...` against real Postgres and OpenFGA), `test-operator` (`make test`) and `test-dashboard` (`yarn typecheck && yarn lint && yarn test`) before it built the images production runs;
   - `go lint` run 44039084268 (`make lint`, all four modules) passed.

**Cleanup.** Deleted all four services (web, site, img, bp), the build source and the groups g1, g3, g4, g5 and bpg; g2 was deleted during item 4. By-id `GET` answered 404 for every one, and no `qa-20261117-m423-*` resource remains in `bex-canary`'s service, group or build-source lists. Both of this run's Kratos sessions were revoked (`qa-login.sh --logout`, then 401), and their jars were removed. Other hunts' fixtures in the workspace were left alone.
