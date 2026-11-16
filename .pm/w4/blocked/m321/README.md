# w4 · m321 — The workspace audit log records the dashboard's own billing and audit reads, but not the reads Render audits: env var values and datastore connection info

**Worker:** worker4 **Goal:** the audit log answers "who did what" as Render's does. Viewing env var values and datastore connection info leaves a Render-named event. Admin pages that read billing status, invites or the audit log itself leave none. **Status:** blocked — t001–t004, t006–t008 and t010–t012 done (shipped). t005's live replay waits for the production deploy that pins t011/t012's commit, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Decide audit recording per verb, not per relation: a successful read gated by an admin relation stays unrecorded — **DONE** | 45m | — |
| [t002](done/t002.md) | Record Render's `ViewEnvVarValuesEvent` for every read that returns env var values — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Record Render's `ViewConnectionInfoEvent` for Postgres and Key Value connection info — **DONE** | 30m | t001 |
| [t010](done/t010.md) | One Render-named event per user action wherever Render's enum has one, including `CreateProjectEvent` and `CreateEnvironmentEvent`, which the mapping names but never emits — **DONE** | 50m | t001 |
| [t004](done/t004.md) | Ledger and artifact: ADR018's audit row and `audit-logs-api.md` state the read rules and the event mapping — **DONE** | 20m | t002, t003, t010 |
| [t011](done/t011.md) | A service or project create, and a project delete, name what they created or deleted: their rows read only `{"relation":"can_create"}` (round 311 adds env group, webhook and Blueprint rows) — **DONE** | 50m | t004 |
| [t012](done/t012.md) | A refused service create records a `CreateServerEvent` with status `success`: the row is written at authorization, before the 409, and env-group create shares the shape — **DONE** | 30m | t011 |
| [t005](t005.md) | Live verification in `bex-canary` | 30m | t004, t011, t012 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008, t011, t012 |

Twelve tasks, 375m (6h15m). t010 was added in round 58, t011 in round 144, t012 in round 155.

**Major (compliance).** Revealing a secret or a database password leaves no trail. In the same log, 43% of the rows are the dashboard's own billing-status reads. Render's audit log records the first and not the second.

## Finding

Live `/qa-find-bugs w4` loop, round 57, 2026-11-11 00:38–00:50 UTC (`muse.env`, workspace `bex-canary` on the Scale plan, pin `a0d5802594e9`).

Fixtures, both deleted (DELETE 204, then GET 404):

- Free image service `qa-20261110-au57` (`srv-dbprkhfbq3ic73c0a7e0`) with `QA_AUDIT_SECRET`.
- Free Key Value `qa-20261110-au57-kv` (`red-dbprkhfbq3ic73c0a7f0`).

All probes are in-page `fetch(…, {credentials:'include'})`. The durable evidence is the probes themselves.

### 1. Admin reads flood the log

`GET /v1/owners/tea-daif693dqjvc73e7as3g/audit-logs?startTime=2026-11-10T00:00:00Z&endTime=2026-11-11T00:45:00Z&limit=100`, paged to the end (9 pages), returned **861 rows**. The oldest was 2026-11-10T07:25:07Z.

| `event` (relation) | Rows | What triggers it |
| --- | --- | --- |
| `billing.Status` (`can_manage_billing`) | **370** | The dashboard's `BillingReadiness` query |
| `apps.PlanBlueprint` (`can_create`) | 26 | A Blueprint preview |
| `audit.List` (`can_manage`) | 12 | Reading the audit log itself |
| `members.ListInvites` (`can_manage`) | 10 | The Workspace Settings Team section |

The next largest is `CreateServerEvent`, with 66.

Two windows show where the rows come from:

- **Paging the audit log writes to it.** At 00:43:03–05, paging the log over REST wrote **9 `audit.List` rows**, one per page request.
- **Real changes are outnumbered.** In 00:15–00:35, a window with one service create, two deletes, two env saves and a manual deploy, `billing.Status` had 15 of the 21 rows.

The Workspace Settings → Audit Log table opens on `billing.Status` and `audit.List` rows ("puncsky@gmail.com session · billing.Status · Allowed"). Export as CSV covers the same rows.

### 2. Secret reveals are not recorded

At 00:44:05, I clicked the dashboard's "Reveal QA_AUDIT_SECRET", and it showed `qa-57-value`. Then `GET /v1/services/srv-dbprkhfbq3ic73c0a7e0/env-vars` returned `[{"envVar":{"key":"QA_AUDIT_SECRET","value":"qa-57-value",…}}]`.

At 00:44:26, `GET /v1/key-value/red-dbprkhfbq3ic73c0a7f0/connection-info` returned 200 with `internalConnectionString`, `externalConnectionString` and `cliCommand`. At about 00:44:34, I clicked the dashboard's "Reveal connection info".

`GET …/audit-logs?startTime=2026-11-11T00:43:00Z` then listed only:

- `billing.Status` ×2;
- `CreateRedisEvent` and `CreateServerEvent`;
- `members.ListInvites` ×2;
- `audit.List` ×9.

There was **no row** for either reveal. The window 00:15–00:35 shows the same thing for round 55, whose Export → Copy env vars and Download .env revealed 13 values at 00:26–00:27.

### Render

Render's audit log, [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (fetched 2026-11-11), records:

- `ViewEnvVarValuesEvent`: "logs when environment variable values are viewed";
- `ViewConnectionInfoEvent`: "tracks access to database connection URLs and passwords for both Postgres and Key Value instances";
- `StartShellEvent` and `EndShellEvent`.

Both view events are in the pinned enum (`lego/backend/internal/api/openapi/render-public-api-1.json`, `components.schemas.auditLog.properties.event`, 87 values). Neither appears anywhere in `lego/backend` outside that spec. No event in the enum covers reading billing status, listing invites or listing the audit log.

## Root cause

- **Recording is decided by relation, not verb.** `lego/backend/internal/core/audit.go:1063-1072` puts six relations in `writeRelations`, "Rel… constants that gate a mutation. These emit an audit event on every call, allowed or denied": `can_operate`, `can_create`, `can_manage_keys`, `can_manage_ssh_keys`, `can_manage` and `can_manage_billing`.
  - Only `readRelations` (`:1086-1090`: `can_view`, `can_view_logs`, `can_view_sensitive`) are recorded on denial only.
  - `emit` (`:1199-1211`) records the outcome for whichever class applies.
- **So reads gated by those relations are recorded on every call:**
  - `billing.(*Service).Status` (`lego/backend/internal/billing/service.go:132`) authorizes `can_manage_billing` (`:262`).
  - `audit.List` (`lego/backend/internal/audit/service.go:232`) and `members.ListInvites` (`lego/backend/internal/members/service.go:617`) authorize `can_manage`.
  - `apps.PlanBlueprint` (`lego/backend/internal/apps/blueprint_onboarding.go:268`) authorizes `can_create`. It computes a plan and appears to change nothing; t001 confirms that.
- **The dashboard asks for billing readiness constantly.** `BillingReadiness` comes from `PaymentSetupGate` (`dashboard/src/features/onboarding/components/payment-setup-gate.tsx:48`) on every signed-in page load. `use-billing-onboarding.ts:108-119` polls it every 15 s while active.
- **Sensitive reads are recorded only when denied.** The verbs that return secrets all authorize `can_view_sensitive`, which is recorded on denial only:
  - `secrets.ListEnvVars` (`lego/backend/internal/secrets/service.go:329`) and `GetEnvVar` (`:437`), through `readAuthorizedEnv` (`:348-358`);
  - the env-group value reads (`lego/backend/internal/envgroups/service.go:392`, `:886`, `:998`; `render.go:132`);
  - `postgres.PostgresConnectionInfo` (`lego/backend/internal/postgres/service.go:1035`);
  - `keyvalue.KeyValueConnectionInfo` (`lego/backend/internal/keyvalue/service.go:1288`).

  Nothing calls a typed effect recorder for a reveal, as `RecordDatabaseEffect` does for a create. The other recorders are at `audit.go:375-968`.

- **The masked list shares the read boundary.** `EnvVarKeys` (`secrets/service.go:767`, keys only) also goes through `readAuthorizedEnv`. So the new event cannot go at that boundary, or it would record on every Environment tab load.
- **ADR018's audit row (`docs/ADR018-render-parity.md:272`) states the relation rule.** "**Write**-tier verbs (`can_operate`/`can_create`/`can_manage_keys`/`can_manage`) are recorded allowed or denied; **read**-tier verbs … on **denial only**". It is silent on reads gated by a write-tier relation, and on Render's two view events.

## Round 58 addendum (2026-11-11 00:52–01:05 UTC)

I created project `qa-20261110-mv58` (`prj-dbprp3dhi5vc738givdg`, environments `qa-alpha` and `qa-beta`) with REST `POST /v1/projects`. I created `qa-20261110-mv58-svc` (`srv-dbprp5vbq3ic73c0a7mg`) in `qa-alpha`, then moved it to `qa-beta` with the project page's "Move selected resources to environment". Both were deleted afterwards (204, then 404).

- The move worked on every surface: REST `environmentId` changed, there was one `service_moved` event, and both project-page environment views were right.
- The audit log for that one move had **four** rows: `environments.SetServices` ×3 and `environments.MoveService`, all at 00:54:53. There was no `MoveEnvironmentResourceEvent`.
- The project create logged `projects.CreateWithEnvironments`, not the mapped `CreateProjectEvent`. Over 2026-11-10/11, `CreateProjectEvent` has 0 rows against 8 project creates.
- See t010 for the full count table.

## Definition of done

Signed in as the `bex-canary` admin on `dashboard.bex.co`, after the release pins this build:

1. **Admin reads leave no row.** Note `t0`. Open Billing, Workspace Settings (Team section) and the Audit Log section three times each, and page `GET /v1/owners/tea-daif693dqjvc73e7as3g/audit-logs` three times. Then `…/audit-logs?startTime=<t0>` lists no `billing.Status`, `members.ListInvites`, `audit.List` or `apps.PlanBlueprint` success row. Recorded mutations from the same window are unchanged, for example `CreateServerEvent` for a new `qa-` service.
2. **Viewing env var values leaves a row.** On an owned service, click Reveal on one variable, then Export → Download .env, then `GET /v1/services/{id}/env-vars`.
   - The log lists `ViewEnvVarValuesEvent` rows whose metadata names the service.
   - Opening the Environment tab without revealing anything adds none.
   - An env-group reveal names the group.
3. **Viewing connection info leaves a row.** On an owned Free Key Value and a Free Postgres, run `GET …/connection-info` and click the dashboard's "Reveal connection info". The log lists `ViewConnectionInfoEvent` rows naming each datastore.
4. **Export carries them.** Export as CSV for the window contains the new rows in the `event` column.
5. **Render-named events, one per action** (round 58, t010). Create a `qa-` project with two environments over REST, move a service between them, then delete both. The log lists exactly one `CreateProjectEvent`, one `MoveEnvironmentResourceEvent` and one `DeleteProjectEvent`, with no `projects.CreateWithEnvironments` or `environments.SetServices` rows.
6. **A refused create leaves no success row** (round 155, t012). Note `t0`, then `POST /v1/services` with the immutable name of an owned `qa-` service. It answers 409, and `…/audit-logs?startTime=<t0>` lists no `CreateServerEvent`.
7. **Denials stay recorded.** A denied read (for example a non-member's `GET …/env-vars`) still records as before; covered by a test, since it needs a second account live.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 57, 2026-11-11 (above). Render: [render.com/docs/audit-logs](https://render.com/docs/audit-logs) and the pinned OpenAPI enum.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)), plus the audit/compliance promise of [ADR012](../../../../docs/ADR012-auth.md). Audit logs are a paid Pro+ feature (w8/m138).
- **Expected outcome:**
  - An admin exporting the log for an audit sees who viewed which secret and which database password.
  - They no longer see hundreds of dashboard-generated billing reads.
- **Why now:**
  - w3/m305 just shipped Render's CSV export, which carries the noise straight into compliance exports.
  - Secret reveals stay invisible on every surface until this lands.
- **Render parity:** included. REST, GraphQL, the CSV export and the dashboard table all change, and MCP's reads that reveal values gain the event.
- **Dedupe:**
  - w4/m10, w4/m20 and w4/m26 (done) built the log, the denied-read rule and the event mapping. Their rule is the cause here, not a regression.
  - w5/m285 (done) audits webhook secret reads. It is the precedent for an audited reveal.
  - w3/m305 (blocked; awaiting its own live check) covers placement and CSV. Its DoD rows would pass with or without this fix.
  - w2/blocked/070 covers Stripe-portal payment changes. w10/blocked/002 is a live Render capture.
  - `grep -rn "ViewEnvVarValues\|ViewConnectionInfo" .pm docs lego/backend` finds only the pinned spec. No open item.
  - **Not deploy lag:** `git log a0d5802594e9..origin/main -- lego/backend/internal/core/audit.go lego/backend/internal/audit` is empty.

## Post-release probe (QA loop round 144, 2026-11-11)

Pin `8330744bd643` (`9126d59d7`, 20:02 UTC) contains `0e5a309c0`. Signed in as the `bex-canary` admin; `t0` = 2026-11-11T22:38:11Z.

Fixtures, all deleted afterwards (DELETE 204, then GET 404):

- Free image web service `qa-20261111-r144-aud` (`srv-dbqet40fss2c73f208m0`, `QA_AUDIT_ONE`/`QA_AUDIT_TWO`);
- Free Key Value `qa-20261111-r144-kv` (`red-dbqeta0fss2c73f208o0`);
- REST project `qa-20261111-r144-proj` (`prj-dbqevcso6rds73e87te0`, environments Production and Staging).

`GET …/audit-logs?startTime=<t0>` returned 12 rows.

- **DoD 1 passes.** Billing, Workspace Settings `#team` and `#audit-logs` were each opened three times, and the audit-logs API was paged three times. They added no `billing.Status`, `members.ListInvites`, `audit.List` or `apps.PlanBlueprint` row. `CreateServerEvent` for the new service was recorded.
- **DoD 2 passes (partly run).** Opening the Environment tab, then one dashboard Reveal (`QA_AUDIT_ONE`), then `GET …/env-vars`, gave exactly 2 `ViewEnvVarValuesEvent` rows. Their metadata is `{"service":"srv-dbqet40fss2c73f208m0","targetName":"qa-20261111-r144-aud"}`. Not run: Export → Download .env, and an env-group reveal.
- **DoD 3 passes for Key Value.** `GET …/connection-info` plus the dashboard's "Reveal connection info" gave 2 `ViewConnectionInfoEvent` rows naming `red-dbqeta0fss2c73f208o0` / `qa-20261111-r144-kv`. Not run for Postgres: the workspace's one Free Postgres slot was held by another session's `qa-20261111-699a-pg` (`FREE_INSTANCE_LIMIT`).
- **DoD 4 passes.** The dashboard's Export as CSV (2026-10-13 → 2026-11-11) contained every row above, with the new events in the `event` column. Its `actor` column holds the identity UUID while the dashboard shows the email; Render's export format was not captured.
- **DoD 5 passes on counts.** Create with two environments, add the service to Production, move it to Staging, delete the service, then delete the empty project. That gave exactly 1 `CreateProjectEvent`, 2 `MoveEnvironmentResourceEvent` (two moves were made) and 1 `DeleteProjectEvent`. No `projects.CreateWithEnvironments` or `environments.SetServices` rows appeared.
- **New, filed as t011.**
  - `CreateServerEvent`, `CreateProjectEvent` and `DeleteProjectEvent` carry only `{"relation":"can_create"}`, so the row doesn't say what was created or deleted.
  - By contrast, `CreateRedisEvent`, `DeleteRedisEvent` and `DeleteServerEvent` name their resource.
  - The dashboard's Resource column shows `workspace:tea-daif693dqjvc73e7as3g` for the delete rows.

## Round 155 addendum (2026-11-12 01:27–01:38 UTC, pin `8330744bd643`)

Fixtures, both deleted afterwards (GET 404):

- Starter private service `qa-20261112-r155-pserv` (`srv-dbqhbs5jiris73e60t0g`, `docker.io/traefik/whoami:v1.10.1`), renamed to `qa-20261112-r155-pserv-renamed` at 01:31:39;
- Starter background worker `qa-20261112-r155-worker` (`srv-dbqhcaiv3e1c73bps6h0`).

**New, filed as t012.** A create refused with 409 still writes `CreateServerEvent` with status `success`.

- The dashboard form, at 01:32:08, left `aud-dbqhe65jiris73e60t2g`.
- `POST /v1/services` at 01:37:21 (body in t012) answered `409 CONFLICT "name \"qa-20261112-r155-pserv\" is already in use"`. `…/audit-logs?startTime=2026-11-12T01:37:21.449Z` then listed exactly one row: `CreateServerEvent`, `success`, `{"relation":"can_create"}`.
- The 01:26–01:33 window holds three `CreateServerEvent` success rows for two services that exist.

## Unblock record and replay on `06cc3b02c` (`/loopx w4`, 2026-11-16 03:37–03:39 UTC)

- **The release gate cleared (verified evidence).** Production's scheduled deploy at 2026-11-15 22:49Z ran on `06cc3b02c`, and `git merge-base --is-ancestor 0e5a309c0 06cc3b02c` holds. t001–t004, t006–t008 and t010 (the code in `0e5a309c0`) are live.
- **t011 and t012 were still open work.** Both were filed after the milestone was parked, and neither had code. Both were implemented in this run; see their Outcome sections. With no user decision needed, nothing gated them.
- **Replay, signed in as the `bex-canary` admin over REST and GraphQL** (`scripts/qa-login.sh`). `t0` = 03:37:19Z.
  - Fixtures, all deleted afterwards (DELETE 204, then GET 404):
    - Free image web service `qa-20261116-m321-au` (`srv-dbt7krthdqoc73dn7gug`, `QA_AUDIT_ONE`/`QA_AUDIT_TWO`);
    - Free Postgres `qa-20261116-m321-pg` (`dpg-dbt7ks5hdqoc73dn7gvg`);
    - Free Key Value `qa-20261116-m321-kv` (`red-dbt7ks5c12ts73fj7ldg`);
    - env group `qa-20261116-m321-eg` (`evg-dbt7ksdc12ts73fj7lf0`, `QA_EG`).
  - `GET /v1/owners/tea-daif693dqjvc73e7as3g/audit-logs?startTime=<t0>` returned 12 rows.
- **DoD 1 passes.** Three rounds of the dashboard's own reads added no `billing.Status`, `members.ListInvites`, `audit.List` or `apps.PlanBlueprint` row: `workspaceBillingReadiness`, `workspaceInvites`, GraphQL `auditLogs` and the REST audit log.
- **DoD 2 passes, including the parts round 144 did not run.** The service had four reads: the GraphQL `envVar` reveal; the two `envVar` reads Export → Download .env makes (`service-environment-editor.tsx` `exportEnvironment`); and `GET …/env-vars`.
  - These gave exactly 4 `ViewEnvVarValuesEvent` rows `{"service":"srv-dbt7krthdqoc73dn7gug","targetName":"qa-20261116-m321-au"}`.
  - The keys-only `envVarKeys` read added none.
  - `GET /v1/env-groups/{id}/env-vars/QA_EG` gave one row `{"envGroup":"evg-dbt7ksdc12ts73fj7lf0","targetName":"qa-20261116-m321-eg"}`.
- **DoD 3 passes for Postgres and Key Value.** Each `GET …/connection-info` gave one `ViewConnectionInfoEvent` naming `dpg-dbt7ks5hdqoc73dn7gvg` or `red-dbt7ks5c12ts73fj7ldg`. A Key Value read made while the store was still creating answered 409 and recorded nothing.
- **DoD 4 passes on its source.** The dashboard's CSV export is built from GraphQL `auditLogs`, and for the window that query returned the new `ViewEnvVarValuesEvent` and `ViewConnectionInfoEvent` actions.
- **DoD 5** passed in round 144, and its code is unchanged except for t011's naming. It is re-run in t005.
- **DoD 6 still fails on `06cc3b02c`. This is expected: it is t012, fixed after this pin.** The refused `POST /v1/services` (409 at 03:38:11) wrote `CreateServerEvent` `success` `{"relation":"can_create"}`. The service and env group creates likewise read only `{"relation":"can_create"}` (t011).
- **Remaining gate, no user action:** the next production deploy must pin the commit `fix(audit): … (w4/m321 t011, t012)`. Check it with `git merge-base --is-ancestor <sha> <deployed sha>`. Then a worker re-runs t005 (DoD 5 and 6, plus t011's naming) and closes t009.

## Live replay over REST (`/qa-find-bugs w4` round 486, 2026-11-16 05:29–05:33 UTC, pin `14af7282d9ac`, `bex-canary`, the admin's dashboard session)

Every fixture was deleted afterwards (`GET` 404, project list empty). These are REST probes; the dashboard clicks (Reveal, Export → Download .env, "Reveal connection info", Export as CSV) were not driven.

- **DoD 1, partly: holds.** Three `GET /v1/owners/tea-daif693dqjvc73e7as3g/audit-logs?limit=5` added no row. The Billing and Team views were not opened.
- **DoD 2, partly: holds.** `GET /v1/services/srv-dbt99eidrmis73edarug/env-vars` added `ViewEnvVarValuesEvent {"service":"srv-dbt99eidrmis73edarug","targetName":"qa-20261116-r486-aud"}`.
- **DoD 3, REST: holds.** `GET …/connection-info` on a Free Postgres and a Free Key Value added:
  - `ViewConnectionInfoEvent {"database":"dpg-dbt9a7k0h0ts7389hbl0","targetName":"qa-20261116-r486-pg"}`;
  - `ViewConnectionInfoEvent {"keyvalue":"red-dbt9a7idrmis73edas30","targetName":"qa-20261116-r486-kv"}`.
- **DoD 5: holds.** The steps were:
  - `POST /v1/projects` with two environments;
  - `POST /v1/environments/{a}/resources`, then `{b}/resources`, with the service;
  - the service delete;
  - `DELETE /v1/projects/{id}`.

  The log listed one `CreateProjectEvent` (`project`, `targetName`), one `MoveEnvironmentResourceEvent` per move (two moves, two rows, each naming the `service`), and one `DeleteProjectEvent` (`project`, `targetName`). There were no `projects.CreateWithEnvironments` or `environments.SetServices` rows.
- **DoD 6: holds.** A second `POST /v1/services` with the owned service's name answered `409 CONFLICT "name … is already in use"`. The window holds only the one `CreateServerEvent` of the real create.
- **Not run:**
  - DoD 4 (the CSV export is built in the dashboard);
  - DoD 7 (needs a second account).
- **Observed, not judged:**
  - `MoveEnvironmentResourceEvent` names the service but not the environment it moved to.
  - `DeleteServerEvent` carries the service id without the `targetName` its create row has.
