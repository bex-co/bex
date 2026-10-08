# Deleting a granted Postgres login hides it while its old password still reads data

Why: credential deletion is a revocation operation; a successful delete must prevent new authentication even when PostgreSQL cannot drop a role because tenant objects still reference it.

- **Severity:** major. This is continued data access through a credential reported deleted, confined to the affected tenant database and its existing grants; no elevated role or cross-tenant access was observed.
- **Source:** live qa-find-bugs loop, 2026-10-07 sweep 15; workspace bex / `tea-d98210cbbpdc73dcrkvg`, research HEAD `f7c8d338e`. Owned Free/public PostgreSQL18 `qa-20261007-r15-users-9b62ca` / `dpg-db346dhq73hc73ddrj7g`, SQL database `qa_r15_data`, owner `qa_r15_owner`, additional role `qa_r15_extra`.
- Two complete granted-role delete rounds reproduced the defect, including fresh dashboard reloads and independent new verified-TLS connections. The generated credentials stayed in a local memory-only probe and are omitted from every artifact and this record.

## Reproduce and durable probes

1. Create the owned Free/public Postgres with the names above. Copy its external connection info and public CA to a local SQL client; keep passwords out of output. As owner, execute:

```sql
CREATE TABLE qa_r15_probe(id integer PRIMARY KEY, marker text);
INSERT INTO qa_r15_probe VALUES(15,'qa-20261007-r15-persist');
```

2. Access control → Database username `qa_r15_extra` → Add user. Retain its one-time password locally. New TLS connection as that role: `SELECT current_user,current_database(),3 AS qa_r15` → `qa_r15_extra|qa_r15_data|3`. Both roles have LOGIN but no SUPERUSER, CREATEDB or CREATEROLE.
3. As owner: `GRANT SELECT,INSERT ON qa_r15_probe TO qa_r15_extra;`. A new extra-role connection can now SELECT the marker (before the grant, permission was refused).
4. Delete that user through its trash button. Reload the database page and reopen Access control. It says **No additional users**. Keep using the same saved password in a new psql process and SELECT again: it succeeds.

Second round, actual authenticated browser POST `https://api.bex.co/graphql`, HTTP200 at 13:17:33 UTC. Complete JSON request body:

```json
[
  {
    "operationName": "DeleteDatabaseUser",
    "variables": { "id": "dpg-db346dhq73hc73ddrj7g", "name": "qa_r15_extra" },
    "extensions": {
      "clientLibrary": { "name": "@apollo/client", "version": "4.1.3" }
    },
    "query": "mutation DeleteDatabaseUser($id: String!, $name: String!) {\n  deleteDatabaseUser(id: $id, name: $name)\n}"
  }
]
```

Complete response:

```json
[{ "data": { "deleteDatabaseUser": true } }]
```

After fresh reload, browser POST to that same GraphQL endpoint, complete request and complete HTTP200 response:

```json
{
  "request": {
    "query": "query { database(id:\"dpg-db346dhq73hc73ddrj7g\") { id status } databaseUsers(id:\"dpg-db346dhq73hc73ddrj7g\") { name } }"
  },
  "response": {
    "data": {
      "database": { "id": "dpg-db346dhq73hc73ddrj7g", "status": "available" },
      "databaseUsers": []
    }
  }
}
```

At 13:19:00 UTC, authenticated GET `/v1/postgres/dpg-db346dhq73hc73ddrj7g/users` → HTTP200, complete body `[]`; GET `/v1/postgres/dpg-db346dhq73hc73ddrj7g/credentials` → HTTP200, complete body `[{"username":"qa_r15_owner","default":true}]`.

At 13:20:26 UTC, **172.714 seconds after the second successful delete**, a new process with the same retained generated password and the product CA ran:

```sh
PGPASSWORD="$QA_R15_PASSWORD" PGSSLROOTCERT=/path/to/product-ca.pem \
  PGSSLMODE=verify-full PGCONNECT_TIMEOUT=6 \
  psql -h dpg-db346dhq73hc73ddrj7g.db.bex.co -p 5432 \
  -d qa_r15_data -U qa_r15_extra -X -v ON_ERROR_STOP=1 -A -t \
  -c 'SELECT id,marker FROM qa_r15_probe;'
```

Exit0, complete stdout `15|qa-20261007-r15-persist\n`, stderr empty. A separate new connection queried `pg_roles`; the extra role still had `rolcanlogin=true`.

Read-only cluster capture at 13:20:29 UTC: Database generation7, users=[], deletedUsers=["qa_r15_extra"], phaseReady, Ready=True/Provisioned with observedGeneration7. CNPG generation7 projects owner present/logintrue and extra ensureabsent. No `<database>-user-*` Secret remains. CNPG reports:

```json
{
  "byStatus": {
    "pending-reconciliation": ["qa_r15_extra"],
    "reconciled": ["qa_r15_owner"],
    "reserved": ["postgres", "streaming_replica", "cnpg_metrics_exporter"]
  },
  "cannotReconcile": {
    "qa_r15_extra": [
      "could not perform DELETE on role qa_r15_extra: 1 object in database qa_r15_data"
    ]
  },
  "passwordStatus": {
    "qa_r15_extra": { "resourceVersion": "125187910", "transactionID": 774 },
    "qa_r15_owner": { "transactionID": 756 }
  }
}
```

The screenshot shows the empty UI user list; the SQL probes, API bodies and cluster capture establish continued authentication. It is not an existing open connection or merely a stale row.

## Cause, consumer and concrete target

- `lego/backend/internal/postgres/access.go:202-240`: DeleteUser deletes the password Secret, removes spec.users, records spec.deletedUsers, then returns nil and emits the credentials-deleted effect. It never confirms or separately disables the installed SQL login.
- `access.go:84-98` ListUsers reads only spec.users. The four delete adapters map nil to success: REST users/credentials, GraphQL Boolean and MCP Deleted. Their shapes can express accepted intent; none represents observed SQL revocation.
- `lego/operator/internal/controller/database_controller.go:404-411` projects tombstones as ensureabsent, not as a login revocation. Its one production consumer at :556 becomes the CNPG Cluster spec through the one production cnpgClusterSpec caller at :929.
- **Actual dependency checked:** live CNPG Deployment runs `ghcr.io/cloudnative-pg/cloudnative-pg:1.30.0`. Opened its v1.30.0 sources: `roles/roles.go:170-172` chooses DELETE for ensureabsent before attribute comparison, `roles/runnable.go:271-274` invokes Delete, and `roles/postgres.go:182-199` executes DROP ROLE IF EXISTS without first disabling login. PostgreSQL18 refuses DROP ROLE while grants/ownership reference it. The failed statement leaves authentication intact. Adding loginfalse to an ensureabsent map would **not** fix this branch.

**Chosen target: retire deleted additional login roles as durable non-login roles.** Project a tombstoned non-reserved, non-active role with `ensure:present, login:false, disablePassword:true`, no passwordSecret. Retain the tombstone to keep the installed SQL role NOLOGIN with a NULL password and preserve tenant objects/grants. This intentionally replaces physical role removal with credential retirement for both granted and dependency-free roles; update the Database field comments, ADR009 and adapter descriptions accordingly. A retired role may remain in pg_roles but must never authenticate with its old password. Do not run DROP OWNED/CASCADE or silently delete tenant data.

The same pinned CNPG consumer supports this exact shape: contract.go:98-137 compares Login; contract.go:143-150 interprets disablePassword without a Secret as a NULL password; postgres.go:118-142 updates the role, :364-366 emits NOLOGIN, and :393-400 emits PASSWORD NULL. ensurepresent therefore reaches ALTER ROLE rather than the failing DROP branch. No SQL client or control-plane database dependency belongs in the operator; this remains a CR projection through CNPG, with no new enum required.

Preserve active-role and owner precedence, reserved-role filters, generation-specific issuance, optimistic locking and same-name recreation clearing the tombstone. Recreation must install a fresh password and LOGIN, invalidate the earlier password, and never adopt a stranded Secret. Native active-user/credential lists continue to omit retired logins.

**Before settle:** deletion is an accepted asynchronous revocation request. Change the dashboard success copy to “Revocation requested for {name}” and describe that accepted intent on the native GraphQL/MCP and REST compatibility surfaces; do not claim an observed completed SQL drop from the CR patch. Runtime revocation must converge within the normal reconcile window even with the table grant intact. Failed/slow CNPG reconciliation must not be relabeled as completed; tests must cover that boundary. The existing database Available badge describes database serving health, not successful retirement of a particular login.

## Scope, controls, dedupe and limits

Exhaustive production search found **four** direct DeleteUser calls: rest.go:381 (/users/{user}), rest.go:384 (/credentials/{user}), graphql.go:649 (deleteDatabaseUser), mcp.go:418 (delete_postgres_user). Apply retirement globally to that family. Public and private Postgres share it. Key Value auth and web/static/cron/worker/private App credentials do not use it. The one shared managedRoles production call also projects the live owner/other users, so passing roles need independent regression controls.

Controls actually observed:

- `postgres` input was disabled with aria-invalid=true; no reserved role was submitted.
- A valid extra login had no elevated attributes; ungranted SELECT was refused and granted SELECT succeeded.
- Fresh reload retained the user before deletion and discarded its one-time password.
- After revoking the grant and recreating the same name, the new password worked and the prior password failed. Deleting this dependency-free issuance removed the SQL role and refused new authentication; owner SELECT still returned the marker. The new retirement contract must preserve refusal/data while allowing a retained NOLOGIN role instead of requiring absence.
- Two granted-role deletes hid the user but retained fresh authentication/read access. No password Secret remained after the second delete.

Searched open/blocked/done across all workstreams for DROP ROLE/OWNED, dependency and deleted-login/credential terms; scanned 68 non-done README titles, DO_NOT_DO, the latest40 product commits and targeted DeletedUsers/disablePassword history. No item owns this case. ADR085 / commit48dc7e801 fixed stranded-secret resurrection and retry ordering: the passing recreation control confirms that fix, while SQL dependencies are its unhandled revocation case. This is a residual gap, not an asserted recurrence of the old Secret-adoption mechanism. w4/m170 covers reserved names/reconcile wedging, not ordinary granted logins. Prior w9/m89/m92 hunts do not cover it. No landed retirement fix was found on main.

**Render:** [credential documentation](https://render.com/docs/postgresql-credentials), checked 2026-10-07, promises removal of deleted credentials and describes deactivating the original role to preserve owned objects. Render's default-user rotation model differs from bex's additional-role model; this filing fixes revocation, not that separate default-user feature. PostgreSQL's [DROP ROLE contract](https://www.postgresql.org/docs/18/sql-droprole.html) and [CNPG1.30 role management](https://cloudnative-pg.io/docs/1.30/declarative_role_management/) explain dependency refusal. No authenticated Render delete was exercised.

**Adjacent classes:** missing role/database remains not-found through the authorized resource lookup; forbidden/unauthenticated callers keep existing non-disclosure and never change CRs/Secrets. Infrastructure/reconcile failure is distinct from accepted intent and completed retirement. Do not introduce role-existence disclosures before authorization.

**Unverified:** role-owned objects (only a grant was exercised), pools/read replicas, private-only endpoint, already-connected sessions, outage/restart persistence and concurrent delete/recreate. Regression work covers those relevant paths; NOLOGIN/password clearing revokes new authentication, not an asserted termination of established sessions. No paid fixture, backup, HA/replica or pooler was purchased.

**Evidence, verified on disk:** `.playwright-mcp/qa-20261007-r15-browser-evidence.json` (23 credential-redacted captures), `qa-20261007-r15-deleted-role-cluster.json`, `qa-20261007-r15-deleted-role-active.png`, and the six `qa-20261007-r15-cnpg-*.go` sources under that same ignored directory. This record's wire bodies and SQL command/results are the durable handoff evidence.

**Cleanup verified:** own database deleted through the dashboard, REST404/NOT_FOUND, workspace list empty for this id, expanded cluster inventory zero; memory-only probe terminated, only this run's Kratos session revoked (“ok logged-out”), own jar removed, browser about:blank. Pre-cleanup console errors0/warnings0; deliberate post-delete404 is an expected probe. No implementation was changed by the hunt.
