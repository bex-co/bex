# w4/m143 verification — 2026-10-02

## Contract and scope

The service-page create dialog must initialize its environment and checked service
together when a new opening has resolved scope data. A pending lookup is not
Workspace scope. A failed lookup must expose retry, and refreshes must preserve
an open draft. Submitted service IDs must remain among the visible compatible
candidates. The existing backend scope-equality rule is unchanged.

Both production consumers are in scope: the uncontrolled `/env-groups` dialog and
the controlled service Environment panel. The latter has toolbar and panel
buttons, uses the common service route for web/private/worker/cron, and is reused
by the static-site alias. Postgres and Key Value do not render this component.
Route tracing and component tests are not live verification of every service type.

## Render comparison

The [Render group-scoping documentation](https://render.com/docs/configure-environment-variables#scoping-a-group-to-a-single-environment),
read on 2026-10-02, allows a workspace group to link across project environments;
an environment-scoped group is restricted to that environment. Bex's exact-scope
rule is stricter. This fix preserves it, with expansion left as a separate product
decision. ADR018 and m111's historical parity task now record that correction.

## Live API replay

Production replay: **2026-10-02 08:08:21–08:09:43 UTC**, workspace `tea-d98210cbbpdc73dcrkvg`, fixture prefix `qa-w4-m143-1002`. Each successful response returned the requested scope and exactly the selected service link. Every rejection was followed by an authorized group-list read confirming that the requested name was absent.

| API | Workspace → workspace | Environment → same environment | Workspace → environment | Environment → workspace |
| --- | --- | --- | --- | --- |
| REST | 201, linked | 201, linked | 409, named mismatch | 409, named mismatch |
| GraphQL | 200, linked | 200, linked | 200, resolver error | 200, resolver error |
| MCP | 200, linked | 200, linked | 200, tool error | 200, tool error |

All six failures carried `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`. MCP used the production streamable-HTTP `/mcp` endpoint and protocol `2025-06-18`, with explicit `workspaceId`. No backend code changed.

Cleanup: all exact-ID DELETEs below returned **204**, followed by **404** GETs. Both service App CRs were also absent on the production Kubernetes API.

| Kind | Deleted ID |
| --- | --- |
| env-groups | `evg-davmda6de41s73canqn0` |
| env-groups | `evg-davmd8ude41s73canqlg` |
| env-groups | `evg-davmd56de41s73canqjg` |
| env-groups | `evg-davmd3c5o9vs73dt7oe0` |
| env-groups | `evg-davmcvmde41s73canqh0` |
| env-groups | `evg-davmcuc5o9vs73dt7oc0` |
| services | `srv-davmctude41s73canqfg` |
| services | `srv-davmctk5o9vs73dt7oag` |
| environments | `env-davmctmde41s73canqeg` |
| projects | `prj-davmctede41s73canqd0` |

Two setup attempts were refused before service creation (name length, then privileged port). Their temporary project/environment pairs were also deleted with 204/404: `env-davmckmde41s73canq90`, `prj-davmckk5o9vs73dt7o5g`, `env-davmcnc5o9vs73dt7o80`, `prj-davmcn6de41s73canqb0`. The successful fixtures used port 8080 with `WHOAMI_PORT_NUMBER=8080`.

`qa-login.sh --logout` returned `ok logged-out`; this replay's cookie jar and storage state were removed. Local raw replay: `/tmp/bex-w4-m143-api-replay.json`; it contains only fixture requests and resource responses, no credentials.

## Implementation verification

Focused regressions passed: **5 test files / 71 tests** (create dialog, scope
index, service panel, ServiceEnvPage, and env-groups route). Coverage includes
scope resolving before and after controlled opening, both external buttons,
Workspace and explicit-environment list choices, failed/unauthorized/timeout
lookups and real retry, unavailable service data, cancel/reopen, edited drafts
under refresh, hidden incompatible IDs, and stale responses after workspace
navigation.

An isolated dashboard snapshot kept the new tests and replaced only the dialog
with baseline `d95d017bab794b353a5087442edfc1d9af21cec4`. Both first-opening
regressions failed; the before-open case expected `qa-env` and received
`Workspace (no Environment)`. Shared working-tree source was never restored.
Evidence: `/tmp/bex-m143-old-regression.log`.

The three simplify reviews (reuse, quality, efficiency) completed. Applied the
shared retry translation, removed redundant name validation, and replaced nested
selection scans with Set membership. The post-cleanup focused run passed all
**71 tests** again.

Validation:

- Full `yarn lint` passed, including typecheck, ESLint and knip.
- Full `yarn test` passed **456 files / 3810 tests** in 82.88s.
- After simplify, `yarn typecheck`, ESLint on every changed dashboard file, and
  the focused suite passed.
- `git diff --check`, repository Markdown formatting, and skill-layout validation
  passed. No generated-route drift appeared.
- After pulling `1287ee5bf`, the focused 71 tests and typecheck passed again;
  the incoming environment/service metadata additions coexist with this fix.

Local logs: `/tmp/bex-w4-m143-dashboard-lint.log`,
`/tmp/bex-w4-m143-dashboard-test.log`,
`/tmp/bex-w4-m143-simplified-test.log`,
`/tmp/bex-w4-m143-typecheck-final.log`, and
`/tmp/bex-w4-m143-eslint-final.log`.

## Remaining production gate

The patched dashboard must be released before its first-open clicks can be
observed in production. QA must hard-load the environment-scoped service page,
create through each external button, check cancel/reopen and a workspace-scoped
service, and repeat Workspace/explicit-environment list-page creation. Record
actual scope, visible checked service, submitted variables, persisted links, and
cleanup. Do not infer those UI results from component tests or API replay.
