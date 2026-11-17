# w32 · m423 — Linking an environment group deploys the service even with Auto-Deploy off, as Render documents

**Worker:** worker32 **Goal:** a link to a repo-backed service whose Auto-Deploy is off deploys it at once with a recorded deploy, as Render does; unlinks and group edits keep waiting for the owner's next deploy. **Status:** blocked — release gate, clears by itself: t001–t006 are done and shipped in `d49c33edb`. [t007](t007.md) walks the definition of done on production once a "pin platform images to" commit contains it (§ Blocked).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A link deploys every service it links, Auto-Deploy on or off — **DONE** | 60m | — |
| [t002](done/t002.md) | Surfaces and copy say a link deploys now — **DONE** | 45m | t001 |
| [t003](done/t003.md) | ADR013 rule 3 and the ADR018 env-groups row describe the link rule — **DONE** | 20m | t001 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout (live replay on production) — **BLOCKED** (release gate) | 45m | t005, t006 |

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

Then [t007](t007.md) walks the definition of done in `bex-canary` as the QA user (`bash scripts/qa-login.sh`), deleting every fixture afterwards with by-id `GET` 404 evidence:

- A repo-backed web service from `examples/hello-go` with Auto-Deploy off and a group with `MESSAGE=grp-v1`: a link from the service's Environment tab opens exactly one deploy, which goes `live` and serves `grp-v1`, and no other deploy row turns `canceled`.
- The same link from REST, GraphQL `linkEnvGroup`, MCP `link_env_group` and `POST /v1/env-groups` with `serviceIds` each opens one deploy, and the create's `pendingServiceIds` does not list the service.
- A repo-backed static site with Auto-Deploy off: a link opens one deploy that rebuilds and serves the group's value.
- An unlink and a group delete on an Auto-Deploy-off service open no deploy, and the next manual deploy carries them.
- Controls unchanged: an Auto-Deploy-on and an image-backed service still deploy on a link, a suspended service's link opens no deploy, and a Blueprint sync's link waits.
