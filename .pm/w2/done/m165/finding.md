# Environment IDs returned by Bex fail pinned Render CLI selectors

- **Severity:** major. Environment-name selection works, but copying the returned ID into supported datastore commands fails.
- **Attribution:** server compatibility: Bex's ID producer violates the pinned client's ID discriminator. The observed error is client-side name resolution, before the datastore request. This is not a launcher-only fault or an invalid upstream request.
- **Versions:** installed `/opt/homebrew/bin/bex` v0.2.1; Render v2.27.0, commit `a764810a768202704e7206eb7b87a47211fcd98e`, module `v1.1.3-0.20260909214233-a764810a7682`. The unmodified same-pin Render executable reproduces against Bex. Server deployed revision unknown. Diagnosis checked against main `87704ec04` (relevant code unchanged from `d67095f00`).
- **Context:** 2026-10-02 07:54–08:03 UTC; `https://api.bex.co/v1/`; workspace `tea-d98210cbbpdc73dcrkvg`; ordinary QA human device OAuth, isolated owner-only CLI config; non-TTY JSON commands. Credentials were never passed in command arguments or filed here.
- **Estimate:** repair 50m, shared-boundary verification 45m, plus parity/simplify/tests/closeout; see milestone task table.

## Fixture and reproduction

Use a fresh nonce on retest. These historical fixtures are now deleted. The project/environment were created through the same QA user's REST API for setup because the pinned CLI exposes discovery, not project/environment creation. The successful datastore creates were real CLI operations. Before creation, IDs were absent from a recorded workspace baseline and free capacity was verified.

```http
POST /v1/projects
Content-Type: application/json
Authorization: Bearer <QA_HUMAN_ACCESS_TOKEN>

{"name":"qa-20261002-5a75c9-project","ownerId":"tea-d98210cbbpdc73dcrkvg","environments":[{"name":"qa-20261002-5a75c9-env"}]}
```

Setup returned HTTP 201, project `prj-davm6imde41s73canq00`, and environment `env-davm6imde41s73canq0g`. This setup is not counted as CLI creation coverage.

```sh
bex environments prj-davm6imde41s73canq00 -o json
bex keyvalues create --name qa-20261002-5a75c9-kvid --plan free --region oregon --project prj-davm6imde41s73canq00 --environment env-davm6imde41s73canq0g --confirm -o json
bex keyvalues create --name qa-20261002-5a75c9-kvname --plan free --region oregon --project prj-davm6imde41s73canq00 --environment qa-20261002-5a75c9-env --confirm -o json
bex keyvalues get red-davm776de41s73canq2g --environment env-davm6imde41s73canq0g -o json
bex keyvalues get red-davm776de41s73canq2g --environment qa-20261002-5a75c9-env -o json
bex keyvalues list --environment env-davm6imde41s73canq0g -o json
bex postgres get dpg-davm8pc5o9vs73dt7ntg --environment env-davm6imde41s73canq0g --project prj-davm6imde41s73canq00 -o json
bex postgres get dpg-davm8pc5o9vs73dt7ntg --environment qa-20261002-5a75c9-env --project prj-davm6imde41s73canq00 -o json
bex services --environment-ids env-davm6imde41s73canq0g -o json
```

The PG control was created with `bex postgres create --name qa-20261002-5a75c9-pg --plan free --disk-size-gb 1 --version 18 --project prj-davm6imde41s73canq00 --environment qa-20261002-5a75c9-env --ip-allow-list cidr=<QA_SOURCE_CIDR>,description=QA-current-source --confirm -o json`. `<QA_SOURCE_CIDR>` redacts this run's network source; the selector claim does not depend on connection readiness.

| Probe | Exit | Seconds | Observation |
| --- | --- | --- | --- |
| Environment discovery | 0 | 1.265 | Returns the existing `env-davm6imde41s73canq0g` |
| KV create by environment ID | 1 | 3.810 | Empty stdout; error below; no create request/resource |
| KV create by environment name | 0 | 2.506 | Creates `red-davm776de41s73canq2g` with that environment ID |
| KV get by environment ID | 1 | 6.884 | Same error |
| KV get by environment name | 0 | 8.567 | Correct ID/environment, status `available` |
| KV list by environment ID | 1 | 5.975 | Same error |
| PG get by environment ID | 1 | 5.128 | Same error |
| PG get by environment name | 0 | 3.521 | Correct ID/environment, status `creating` |
| Unmodified same-pin Render KV get by environment ID + project | 1 | 5.456 | Same error |
| Unmodified same-pin Render KV get by environment name + project | 0 | 3.382 | Correct KV/environment |
| Services raw environment-ID filter | 0 | — | Includes both owned KV and PG; bypasses the discriminator |

Every failed selector above produced empty stdout and this complete stderr:

```text
Error: no environment found with name "env-davm6imde41s73canq0g"
```

The unmodified Render control used `render-upstream keyvalues get red-davm776de41s73canq2g --project prj-davm6imde41s73canq00 --environment env-davm6imde41s73canq0g -o json` (then the same command with the environment name). Its separate private configuration supplied `RENDER_HOST=https://api.bex.co/v1/`, the same workspace, and the same human access token privately through `RENDER_API_KEY`. It never contacted Render production.

## Complete sanitized wire evidence

These are authenticated diagnostic replays using the installed client's `render-cli/2.27.0 (macOS - <OS_VERSION>)` User-Agent. The failing CLI's query is inferred from the pinned request builder, not an in-flight HTTP trace. Auth headers are omitted; no non-secret response fields are removed. The first replay proves the object exists, the second reproduces ID-as-name lookup, and the third is the name control. List bodies retain Render's cursor envelopes and empty-array types.

```http
GET /v1/environments/env-davm6imde41s73canq0g
Authorization: Bearer <QA_HUMAN_ACCESS_TOKEN>

HTTP 200
Date: Fri, 02 Oct 2026 07:57:16 GMT
Content-Type: application/json
CF-RAY: a44227b7fc21ad44-SJC

{"id":"env-davm6imde41s73canq0g","projectId":"prj-davm6imde41s73canq00","name":"qa-20261002-5a75c9-env","serviceIds":[],"databaseIds":[],"keyValueIds":["red-davm776de41s73canq2g"],"databasesIds":[],"redisIds":["red-davm776de41s73canq2g"],"envGroupIds":[],"protectedStatus":"unprotected","networkIsolationEnabled":false,"ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"Allow all (default)"},{"cidrBlock":"::/0","description":"Allow all (default)"}]}
```

```http
GET /v1/environments?name=env-davm6imde41s73canq0g&projectId=prj-davm6imde41s73canq00&limit=100
Authorization: Bearer <QA_HUMAN_ACCESS_TOKEN>

HTTP 200
Date: Fri, 02 Oct 2026 07:57:17 GMT
Content-Type: application/json
CF-RAY: a44227c42fc24801-SJC

[]
```

```http
GET /v1/environments?name=qa-20261002-5a75c9-env&projectId=prj-davm6imde41s73canq00&limit=100
Authorization: Bearer <QA_HUMAN_ACCESS_TOKEN>

HTTP 200
Date: Fri, 02 Oct 2026 07:57:19 GMT
Content-Type: application/json
CF-RAY: a44227cbaf5967a6-SJC

[{"environment":{"id":"env-davm6imde41s73canq0g","projectId":"prj-davm6imde41s73canq00","name":"qa-20261002-5a75c9-env","serviceIds":[],"databaseIds":[],"keyValueIds":["red-davm776de41s73canq2g"],"databasesIds":[],"redisIds":["red-davm776de41s73canq2g"],"envGroupIds":[],"protectedStatus":"unprotected","networkIsolationEnabled":false,"ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"Allow all (default)"},{"cidrBlock":"::/0","description":"Allow all (default)"}]},"cursor":"env-davm6imde41s73canq0g"}]
```

The successful CLI Key Value create returned this complete stdout (stderr empty):

```json
{
  "data": {
    "id": "red-davm776de41s73canq2g",
    "name": "qa-20261002-5a75c9-kvname",
    "plan": "free",
    "region": "fsn1",
    "status": "creating",
    "createdAt": "2026-10-02T07:56:12Z",
    "updatedAt": "2026-10-02T07:56:12.821804279Z",
    "version": "8",
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "ownerType": "team",
    "projectId": "prj-davm6imde41s73canq00",
    "environmentId": "env-davm6imde41s73canq0g",
    "ipAllowList": [],
    "maxmemoryPolicy": "allkeys_lru",
    "persistenceMode": "journal_snapshot"
  }
}
```

## Contract, root cause, and caller census

The exact pinned source is the oracle for the prefix, not current Render documentation. [Pinned ID validator](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/pkg/validate/id.go) lines 41–43 accepts only `evm-` plus 20 lowercase alphanumeric characters. [Pinned resolver](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/pkg/resolve/resolve.go) lines 225–241 sends recognized IDs to environment detail and validates workspace/project; lines 244–268 treat other strings as names, issue the list query, then emit the observed error for an empty list. Source inspected from the downloaded module at the exact pin. [Render environment retrieval documentation](https://api-docs.render.com/reference/retrieve-environment) documents the detail endpoint but is not evidence for the prefix grammar.

Backend paths below are relative to `lego/backend/internal/`:

- **Producer:** `id/id.go:80–84` deliberately mints `env-*`; `store/grouping_tx.go:117–126` calls `ids.New(ids.Environment)` at line 125. One mint helper, two wrappers (`store/environments.go:79–80`, `store/grouping_tx.go:143–144`), and three creation families: direct environment create (`environments/service.go:593,601`), project create with environments (`projects/service.go:420`), Blueprint grouping (`apps/deploy.go:1496`).
- **Serialization:** `environments/service.go:1162–1180` and `environments/rest.go:91–93` pass the stored ID through; `projects/service.go:150–162` exposes it in project membership. The returned value therefore fails the client discriminator before any detail lookup or target datastore mutation.
- **The name filter is correct:** `environments/rest.go:44–49` matches actual names. Do not make a `name` query silently interpret IDs to mask the producer mismatch.
- **Pinned consumers:** `pkg/keyvalue/create.go:50–78` resolves scope before building/sending create; `pkg/keyvalue/resolve.go:25–34` shares get/update/delete/suspend/resume resolution; `cmd/kvlist.go:93–111` resolves list scope. PG shares the same resolver through `pkg/postgres/service.go:129,140,221` and `pkg/postgres/resolve.go:74–86`. `pkg/environment/repo.go:18–33` calls detail/list and unwraps 200 responses; `pkg/client/client.go:189–191` sets list limit 100. Interactive KV create also calls the resolver (`pkg/tui/views/kvcreate.go:137`).
- **Counted impact:** 14 canonical commands: `{keyvalues,postgres} × {create,list,get,update,delete,suspend,resume}` with explicit `--environment <returned-ID>`, plus `kv`/`keyvalue`/`pg` aliases. Live evidence covers KV create/get/list and PG get; the remaining routes are source-traced impact, not executed mutations. Unscoped direct resource IDs, environment name + project, and `services --environment-ids` work.

## Repair target and identity safety

Return stable canonical `evm-*` IDs that the existing client can recognize, including for existing environments. Keep legacy `env-*` API inputs addressable as the same environment. A bounded backend alias/normalization contract is the proposed approach; implementation must settle and document its internal/public boundary under ADR020. Merely switching future mints leaves the live population broken. A wholesale SQL primary-key rewrite or scattered string replacements risk durable references; neither is an adequate unexamined fix. Do not patch, fork, or teach the launcher a request rewrite.

Audit and cover SQL `apps.environment_id` foreign keys (`store/migrations/0019_environments.up.sql:28`), App/PG/KV `core.LabelEnvironment`, environment-group `metadata.EnvironmentID`, Blueprint grouping/claims, protected confirmations, audit records, and all project/resource environment-ID serializers. `core/environment.go:38,43,52` and `environments/service.go:169–173` form a shared create-placement seam. REST/GraphQL/MCP and dashboard must use the same outward ID and accept the relevant legacy/canonical inputs consistently.

Normalize identity before the existing authorization/binding checks (`environments/service.go:1102–1113`), never in a way that bypasses workspace/project checks or reveals existence. Keep invalid/not-found/forbidden/unauthenticated neighbors consistent with the existing contract. Define collision behavior when both spellings could be present. Test legacy rows with real durable references, not only newly created environments.

A legacy literal `env-*` string will still be interpreted as a name by the unchanged upstream CLI. Compatibility means old API links remain usable and discovery returns its canonical `evm-*` alias; CLI callers must re-discover that ID. Do not promise to make the unchanged client recognize arbitrary old literals.

## Dedupe, limits, and cleanup

- Searched all open/done `.pm` for `evm-`, `IsEnvironmentID`, `no environment found`, environment-prefix and placement variants. Only unrelated mock `evm-qa` appeared in w4/done/m111/t006; no matching defect.
- `git log --all -S 'prefix: "env"' -- lego/backend/internal/id/id.go` reaches introduction `6d35ffaf1` (w1/m32), with no repair. Main through `87704ec04` retains the mismatch. Server deployment SHA is unknown; this is not a proven deploy-lag report.
- w8/m45 name/workspace scoping, w8/m47 service rename identity, and w6/m96 Blueprint/exs path compatibility concern different contracts. Installed-only launcher gaps in w9/066 do not explain an identical unmodified Render failure.
- Current `.pm/DO_NOT_DO.md` reviewed: Render-compatible backend work is in scope; building/forking the CLI is excluded.
- Not executed here: all 14 mutation variants, GraphQL/MCP/dashboard ID acceptance, stored-identity migration/alias collisions, or live cross-workspace denial. These are explicit implementation acceptance obligations. No database-connectivity claim is needed for this selector failure.
- Cleanup verified 2026-10-02 by authenticated detail/list reads: PG `dpg-davm8pc5o9vs73dt7ntg` and KV `red-davm776de41s73canq2g` absent. The environment had empty dependent resource arrays before deletion, then returned 404. The project had no remaining environments before deletion, then detail 404/list absence. The failed KV create produced no resource. No surviving fixture remains for this finding; recreate with a fresh nonce.
