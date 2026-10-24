# w8 · m137 — Inbound IP rules for web services, static sites and environments are a Scale/Enterprise workspace feature, as on Render

**Worker:** worker8 **Goal:** Setting a non-empty inbound IP allowlist on a web service, static site or environment is refused unless the workspace is on Scale or Enterprise, as on Render. Postgres and Key Value allowlists stay available on every workspace plan. **Status:** blocked — t001, t002, t003, t004, t005, t006, t007 done 2026-10-23; release pipeline deploys bex-api and the dashboard; QA runs t008 in bex-canary (Hobby): services create --plan free --ip-allow-list exits 1 with the Scale-plan sentence and creates nothing, services update --ip-allow-list refused (empty list accepted), PATCH /v1/environments with a restricting list refused, kv update --ip-allow-list on a Free Key Value accepted, dashboard sections show the Scale hint; inventory production workspaces below Scale that hold a rule (count into ADR018); delete fixtures by ID with 404 proofs

## Tasks (in order)

| id   | title                                                                            | est | depends_on                 |
| ---- | -------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Admission: refuse non-empty service/static/environment allowlists below Scale     | 45m | —                          |
| t002 — **DONE** | Blueprint, clone and create paths carry the same refusal                          | 30m | w8/m137/t001               |
| t003 — **DONE** | Existing rules on Hobby/Pro workspaces and plan downgrades                        | 30m | w8/m137/t001               |
| t004 — **DONE** | Dashboard: the IP rules sections on Hobby/Pro say "Scale plan"                    | 30m | w8/m137/t001               |
| t005 — **DONE** | Render parity: REST/GraphQL/MCP/dashboard and ADR018 agree                        | 15m | w8/m137/t002, w8/m137/t003, w8/m137/t004 |
| t006 — **DONE** | Simplify the changed code                                                        | 15m | w8/m137/t005               |
| t007 — **DONE** | Test coverage: plan × resource type × empty/non-empty, downgrade, datastores     | 30m | w8/m137/t005               |
| t008 | Close out the milestone                                                          | 30m | w8/m137/t006, w8/m137/t007 |

## Definition of done

Live on production after the release pipeline deploys bex-api and the dashboard, in a Hobby workspace (`bex-canary`), with owned Free fixtures deleted by ID with 404 proofs:

- `bex services create --type web_service --plan free --image docker.io/traefik/whoami:v1.10.3 --env-var PORT=80 --ip-allow-list "cidr=203.0.113.0/24,description=x" --confirm -o json` exits 1 with a 400 naming the Scale plan, and creates nothing.
- `bex services update <id> --ip-allow-list …` on an existing Hobby web service → the same 400. An empty list is accepted.
- `PATCH /v1/environments/{id}` with a non-empty `ipAllowList` → the same 400.
- Control: `bex kv update <id> --ip-allow-list …` on a Free Key Value → accepted (datastore rules stay on every plan).
- The dashboard IP rules sections on Hobby show the Scale-plan hint.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep g7h2, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, platform pin `29d63e1f7bea`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`, GraphQL `plan: "hobby"`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). [render.com/docs/inbound-ip-rules](https://render.com/docs/inbound-ip-rules): web services and static sites, and environment- and workspace-level rules, are available to "Scale" and "Enterprise" orgs only; Render Postgres and Key Value rules are available to "All workspaces". Free is "not eligible for service/environment/workspace-level rules". The same restriction class as w8/m127 (free SMTP), w8/m130 and w8/m131, which shipped. ADR018:154 records no plan gate.
- **Expected outcome:** workspace plan features match Render's. Pricing parity ([ADR030](../../../docs/ADR030-pricing.md): Hobby/Pro/Scale) is meaningful only if Scale-only features are Scale-only.
- **Why now:** minor-to-major (pricing/feature parity, a Scale feature given away on Hobby). Render parity task included: REST/GraphQL/MCP/dashboard and Blueprint surfaces.

## Evidence (sweep g7h2)

```text
GraphQL (dashboard session) query { workspaces { id name plan } } → bex-canary tea-daif693dqjvc73e7as3g plan "hobby"
$ bex services create --type web_service --name qa-20261023-g7h2-ipal --image docker.io/traefik/whoami:v1.10.3 --plan free \
    --env-var PORT=80 --ip-allow-list "cidr=203.0.113.0/24,description=doc-only" --confirm -o json
exit 0 → ipAllowList [{"cidrBlock":"203.0.113.0/24","description":"doc-only"}]
deploy live; curl https://qa-20261023-g7h2-ipal.onbex.co/ from a non-listed client → 403 (enforced)
```

Environments created in the same workspace in sweeps b1c3/c8d1 also accepted `ipAllowList` (default `0.0.0.0/0` + `::/0`). Fixture deleted by ID afterwards.

## Root cause

`lego/backend/internal/apps/settings_check.go:107` (`checkIPAllowList`) validates entries per App type only, and `apps/service.go:4857` (`SetIPAllowList`) and create carry no workspace-plan check. The environment ACL write (`environments.Service.SetACL`, ADR032:79-84) accepts `ipAllowList` on any plan. Workspace plans exist (`internal/store/plans.go:35-45`: hobby, pro, scale, enterprise), so the gate has a data source.

## Fix (target)

- One predicate, e.g. `inboundIPRulesAllowed(workspacePlan)` (`scale`/`enterprise`), applied to non-empty allowlists on web/static App create/update/clone/Blueprint and environment create/update. Message: `inbound IP rules for web services, static sites and environments require the Scale plan or higher`. Empty lists are always accepted.
- t003: existing non-empty rules on Hobby/Pro workspaces stay enforced (never silently open a restricted service) and become read-only except clearing. A downgrade from Scale keeps existing rules and refuses new non-empty edits. Record the count of affected workspaces and this transition in ADR018.

## Blast radius

App and environment allowlist writers on REST, GraphQL, MCP, dashboard and Blueprints, every workspace below Scale. Postgres/Key Value allowlists unchanged.

## Unverified

- Render's exact API status and text for this refusal (no Render account); the target uses 400 like bex's other plan gates.
- Render's handling of existing rules after a downgrade.

## Dedupe

Searched `.pm` open and done, ADR018:154, ADR030 and ADR032 for allowlist + Scale/Enterprise/workspace plan: none. w8/m115 and w1/m150/m171 concern enforcement and logging, not plan eligibility.
