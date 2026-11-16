# w32 · m423 — Linking an environment group deploys the service even with Auto-Deploy off, as Render documents

**Worker:** worker32 **Goal:** a link to a repo-backed service whose Auto-Deploy is off deploys it at once with a recorded deploy, as Render does; unlinks and group edits keep waiting for the owner's next deploy. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A link deploys every service it links, Auto-Deploy on or off | 60m | — |
| t002 | Surfaces and copy say a link deploys now | 45m | t001 |
| t003 | ADR013 rule 3 and the ADR018 env-groups row describe the link rule | 20m | t001 |
| t004 | Render parity | 20m | t002, t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout (live replay on production) | 45m | t005, t006 |

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
