# w4 · m315 — A public datastore with no IP rules reads back as public on every surface: REST, GraphQL, MCP and the dashboard return its effective unrestricted rules, not an empty list that Render clients read as blocked

**Worker:** worker4 **Goal:** every read of a Key Value or Postgres allowlist reports the store's _effective_ external access. A public store with no saved rules reads `0.0.0.0/0` + `::/0`, the rule the Blueprint export already uses. A Render client therefore never sees "external connections blocked" for a store that accepts outside connections, and writing back what it read never cuts access. **Status:** blocked — t001, t002, t004, t005 and t006 done (shipped); t003 live replay waits for the next production deploy, then t007 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | One effective-access projection for every datastore allowlist read (REST, GraphQL, MCP, Blueprint export) — **DONE** | 50m | — |
| [t002](done/t002.md) | Dashboard Key Value Networking and Postgres Access control list the effective rules — **DONE** | 35m | t001 |
| [t003](t003.md) | Live verification on owned public Free Key Value and Postgres stores | 25m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

Seven tasks, 185m (3h05m). Major: the readback states network exposure wrongly in the unsafe direction (it says blocked while the store is open), and a no-op read-then-write disables access.

## Finding

Live `/qa-find-bugs w4` loop, round 17, 2026-11-10 12:15–12:35 UTC (`muse.env`, workspace `bex-canary`, pin `c12cd57facb1`).

**Fixtures.** Two Free stores, both created from the dashboard with the **Public access** switch on and no IP rules:

- Key Value `qa-20261110-kv17` (`red-dbpgp6dme2kc73f2l3s0`)
- Postgres `qa-20261110-pg17` (`dpg-dbpgsoc5ngvc73e8top0`)

Both were deleted afterwards. GET now returns 404, and the external endpoints fail TLS with "unexpected eof".

**Probes:**

- **Every read surface says the store has no rules.**
  - `GET /v1/key-value/red-dbpgp6dme2kc73f2l3s0` → `"public":true,"ipAllowList":[]`. The list item and the Render `GET /v1/redis/{id}` alias return the same.
  - `GET /v1/key-value/{id}/ip-allow-list` → `{"cidrs":[]}`.
  - GraphQL `keyValue(id){ public ipAllowList }` → `{"ipAllowList":[],"public":true}`.
  - `GET /v1/postgres/dpg-dbpgsoc5ngvc73e8top0` → `"public":true,"ipAllowList":[]`.
- **Both external endpoints still accept a connection from my address.**
  - `redis-cli --tls --sni red-dbpgp6dme2kc73f2l3s0.kv.bex.co -h red-dbpgp6dme2kc73f2l3s0.kv.bex.co -p 6379 PING`, sent without credentials, answers `NOAUTH Authentication required.`
  - `psql -w "host=dpg-dbpgsoc5ngvc73e8top0.db.bex.co port=5432 sslmode=require user=qa_probe dbname=qa_probe" -c 'select 1'` answers `fe_sendauth: no password supplied`.
  - In both cases the connection reached the server and was asked for credentials.
- **The dashboard depends on bex's own `public` flag.** The Key Value Networking panel reads "No IP rules. External access remains enabled; save this empty list to disable it." Render clients never read `public`.

**What a Render client concludes.**

- The pinned Render CLI's text renderer prints an empty list as "IP allow-list: empty (external connections blocked)" ([ADR009](../../../../docs/ADR009-postgresql-management.md) line 275, `render-oss/cli` `a764810a` `pkg/text/ipallowlist.go:17-20`).
- Render documents an empty list as external access disabled ([key-value#enabling-external-connections](https://render.com/docs/key-value#enabling-external-connections); a public Postgres's default is `0.0.0.0/0`, [postgresql-creating-connecting#restricting-external-access](https://render.com/docs/postgresql-creating-connecting#restricting-external-access)).
- bex's own write contract agrees: an empty-list update disables external access (`SetIPAllowList`, `keyvalue/service.go:946-950`, `postgres/access.go:59-63`; [ADR021](../../../../docs/ADR021-keyvalue-management.md) line 61).
- So a client that reads the list and writes it back unchanged turns external access off.

## Root cause

The read side returns the saved rules without applying public intent.

- **Views.** `kvView` (`lego/backend/internal/keyvalue/service.go:312-325`) and `pgView` (`postgres/service.go:579-604`) both use `core.AllowListOrEmpty(core.AllowListFromSpec(...))`. The REST Key Value wrapper repeats it (`keyvalue/rest.go:104`).
- **Allowlist reads.** `GetIPAllowList` (`keyvalue/service.go:936-944`, `postgres/access.go:47-56`) returns the raw list. Its own doc comment says "Public intent determines whether an empty list is disabled or explicitly unrestricted".
  - It feeds REST `GET …/ip-allow-list` (`keyvalue/rest.go:240-241`, `postgres/rest.go:355-356`).
  - It feeds GraphQL (`keyvalue/graphql.go:140`, `postgres/graphql.go:483`).
  - It feeds MCP (`postgres/mcp.go:367-374`).
- **The Blueprint export already does it right.** `datastoreAllowListEntries` (`lego/backend/internal/apps/blueprint_generate.go:922-933`, used at `:901` and `:909`) says: "Render has no separate public toggle, so export effective access … legacy public datastores with no rules need explicit unrestricted rules for both address families". It emits `0.0.0.0/0` + `::/0`. Today the export and the REST, GraphQL and MCP reads disagree about the same store.
- **The public-with-no-rules state is created on purpose.**
  - [ADR009](../../../../docs/ADR009-postgresql-management.md) line 273: GraphQL `createDatabase`, MCP `create_postgres` and the dashboard keep `public:false` by default, but "their explicit `public:true` still permits an empty unrestricted resource list".
  - The dashboard sends `public` (`dashboard/src/features/keyvalue/hooks/use-create-key-value.ts:21,78`).
  - w2/038 (done) fixed only the REST Postgres create _omission_, by saving `0.0.0.0/0` + `::/0` (`postgres/rest.go:235`; `core/cidr.go:214-215` "Allow all (default)").
  - ADR009 line 271 left existing public stores with no rules as they were saved.

## Target behavior

This is the parity-sufficient choice; no decision is parked.

- **Reads report effective external access**, by the same rule the export uses:
  - a public store with no saved rules reads `[{cidrBlock:"0.0.0.0/0"},{cidrBlock:"::/0"}]`, each with `core/cidr.go`'s "Allow all (default)" description;
  - a private store reads `[]`;
  - a store with rules reads exactly those rules.
- **Writes are unchanged.** A nonempty list makes the store public and an empty list makes it private, so writing back what was read keeps access.
- **One shared core helper** serves the export, `kvView`, `pgView` and both `GetIPAllowList` functions, so they cannot drift again.
- **No migration.** Saved data is not touched.

## Definition of done

Live on production after the release pins this build, with owned `qa-` fixtures in `bex-canary`:

- **Readback.** For a Free Key Value and a Free Postgres created from the dashboard with Public access on and no rules, each of these returns the two unrestricted rules `0.0.0.0/0` and `::/0` (not `[]`), while `public` stays `true`:
  - `GET /v1/key-value/<id>`
  - `GET /v1/redis/<id>`
  - `GET /v1/key-value/<id>/ip-allow-list`
  - `GET /v1/postgres/<id>` and `GET /v1/postgres/<id>/ip-allow-list`
  - GraphQL `keyValue` / `postgres`
- **Access unchanged.** `redis-cli --tls … PING` without credentials still answers `NOAUTH Authentication required.`, and `psql -w …` still answers `fe_sendauth: no password supplied`.
- **Round trip.** Writing the read-back list unchanged (`PUT …/ip-allow-list` with those rules) keeps external access. Clearing the list (empty) then makes both endpoints refuse new external connections.
- **Other stores unchanged.** A private store still reads `[]`, and a store with explicit rules reads exactly those rules.
- **Dashboard.** The Key Value Networking and Postgres Access control panels list the effective rules in en and zh, and their pending skeletons match the ready state at 1440 and 390.
- **Checks.** The backend real-DB suite, dashboard `yarn lint` and `yarn test` pass.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Unverified (carried into tasks)

- The pinned Render CLI's actual text for these two stores was not run this round. It is cited from source via ADR009 line 275 (t003 runs it).
- MCP allowlist tool output was read from source, not called (t001 tests it, t003 calls it).
- Render's dashboard wording for a `0.0.0.0/0` row was not seen (t004).

## Dedupe

- w2/038 (done) covered only the REST Postgres create omission.
- w2/m166 (done) made clear mean disable.
- w7/m5, w4/m116, w4/m233 and w4/222 are other allowlist facets: enforcement, publication, restarts and stale panels.
- No open item covers a public store with no rules reading back as empty.
- Not deploy lag: `git diff c12cd57facb1..origin/main` on `keyvalue/`, `postgres/`, `core/cidr.go` and `apps/blueprint_generate.go` changes no allowlist or `public` line (w8/237, w8/246 and w8/m149 touch other code).
- No `.pm/DO_NOT_DO.md` entry applies.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 17, 2026-11-10 (fixtures and probes above).
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); [ADR021](../../../../docs/ADR021-keyvalue-management.md) (Key Value), [ADR009](../../../../docs/ADR009-postgresql-management.md) (Postgres), [ADR018](../../../../docs/ADR018-render-parity.md) datastore access rows.
- **Expected outcome:** a Render client, the bex CLI, Terraform-style read-modify-write tooling and the dashboard all agree on whether a datastore is reachable from the internet. A no-op round trip can no longer cut access without warning.
- **Why now:** the readback states network exposure wrongly in the security-relevant direction (blocked while open). The read-then-write hazard also turns a harmless sync into an outage for any client that relies on external access.
- **Render parity included:** REST, GraphQL, MCP and the dashboard all change (t004).

## Post-release probe (QA loop round 124, 2026-11-11)

The release gate cleared at `6ee0197c7` (14:43 UTC): production is pinned to `f609163ada8e`, which contains `529395606`. A `/qa-find-bugs w4` round replayed the Key Value half of the DoD once. This is a record for t003, not a replacement for it, and t003 stays todo.

- **Fixture:** Free Key Value `qa-20261111-r124-kv` (`red-dbqag0npvbns739s7ie0`), created from the dashboard's New Key Value page with Public access switched on and no rules. Deleted afterwards (DELETE 204, then GET 404).
- **Readback (passes).** Once `available` (17:38):
  - `GET /v1/key-value/<id>` and `GET /v1/redis/<id>` → `public: true`, `ipAllowList: [{"cidrBlock":"0.0.0.0/0","description":""},{"cidrBlock":"::/0","description":""}]`;
  - `GET /v1/key-value/<id>/ip-allow-list` → `{"cidrs":["0.0.0.0/0","::/0"]}`;
  - GraphQL `keyValue(id:) { public ipAllowList }` → `{"ipAllowList":["0.0.0.0/0","::/0"],"public":true}`;
  - the dashboard's Networking panel lists the same two rules (en).
- **Access unchanged (passes).** `redis-cli --tls --sni <host> -h <host> -p 6379 PING` without credentials → `NOAUTH Authentication required.`
- **Round trip (passes).**
  - `PUT …/ip-allow-list {"cidrs":["0.0.0.0/0","::/0"]}` → 200. The readback is unchanged, and `PING` still answers `NOAUTH` 15 s later.
  - `PUT …/ip-allow-list {"cidrs":[]}` → 200, then `public: false`, `ipAllowList: []`. Two `PING`s 15–20 s later failed with `SSL_connect failed: unexpected eof while reading`.
- **Not run:**
  - the Postgres half: the workspace's one Free Postgres is another session's, and a paid one would be a purchase;
  - MCP;
  - a private store and an explicit-rules store;
  - the zh panel and the 1440/390 skeleton check;
  - the pinned CLI.

## Post-release probe (QA loop round 169, 2026-11-12): the Key Value half

Pin `e7a56b07e0d7` carries the fix. The fixture was the round's Free Key Value `qa-20261112-r169-kv` (`red-dbqkjnmrbf4s73estuhg`), created in the dashboard with Public access on and no rules. Deleted afterwards (GET 404).

- **Readback passes for Key Value:**
  - `GET /v1/key-value/<id>` and `GET /v1/redis/<id>` → `public: true`, `ipAllowList: [{"cidrBlock":"0.0.0.0/0"},{"cidrBlock":"::/0"}]`;
  - `GET …/ip-allow-list` → `{"cidrs":["0.0.0.0/0","::/0"]}`;
  - GraphQL `keyValue { ipAllowList }` → `["0.0.0.0/0","::/0"]`;
  - the dashboard Networking panel lists both rules (en).
- **Access:** the copied External URL connected with TLS/SNI and its credentials. The unauthenticated `NOAUTH` check was not run.
- **Not run:**
  - the Postgres half;
  - the round trip (`PUT` unchanged, then empty);
  - zh;
  - skeleton geometry;
  - the private and explicit-rules controls.
