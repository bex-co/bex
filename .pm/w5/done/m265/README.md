# w5 · m265 — Postgres credential rotation: a new user becomes the default, as on Render

**Worker:** worker5 **Goal:** a customer rotates a Postgres password with zero downtime: the new credential becomes the default, connection strings follow it, and the old one can be deleted (the original owner is deactivated, not dropped). **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | CRD default-user pointer and original-owner deactivation; operator projects them to CNPG — **DONE** | 1h30m | — |
| t002 | Core + REST: `POST /v1/postgres/{id}/credentials` creates the new default; delete rules follow Render — **DONE** | 1h30m | t001 |
| t003 | Connection info and Blueprint `fromDatabase` follow the default user — **DONE** | 1h | t001 |
| t004 | GraphQL, MCP and dashboard Credentials section follow Render's flow — **DONE** | 1h30m | t002 |
| t005 | Live zero-downtime rotation walk on dev-5 or production — **DONE** | 45m | t003, t004 |
| t006 | ADR018 and ADR009 record the rotation contract — **DONE** | 20m | t005 |
| t007 | Render parity — **DONE** | 30m | t006 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007, t008 |
| t010 | Closeout — **DONE** | 10m | t009 |

Total: **10 tasks, about 8h20m**.

## Definition of done

- `POST /v1/postgres/{id}/credentials` (Render's shape) creates a user that `GET …/credentials` reports as `default:true`, and connection-info returns its connection strings.
- Deleting the current default returns a named 400; deleting the original owner revokes its login while its objects survive.
- A Blueprint-linked service picks up the new default on its next sync, and a live rotation keeps the service up.
- GraphQL, MCP and the dashboard Credentials section behave the same; ADR018 carries no unsigned `—` for this.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w5` 2026-11-05 item 1; Render OpenAPI `create-postgres-user` ("This becomes the database's new 'default' user"); render.com/docs/postgresql-credentials; ADR018 Render operation omissions row (unsigned `—`); `w4/done/168.md` mounted only GET/DELETE aliases.
- **Goal linkage:** ADR009 managed Postgres; ADR008 Render parity first.
- **Expected outcome:** customers rotate database passwords without downtime, through any surface.
- **Why now:** a security-relevant routine operation with no owner; the managed-role machinery (`spec.users`, per-issuance Secrets) already exists, so the increment is small.
- **Render parity:** included — REST, GraphQL, MCP and dashboard change.

## Progress (2026-11-05)

- **Shipped in code:** CRD `defaultUser` + `users[].ownerPrivileges`; operator role chain (owner + retired logins + earlier credentials), owner retirement (`login:false`, `disablePassword`), `<id>-default-app` Secret with `status.secretName` moving only once it is current, pooler following it; Core `CreateCredential`/`DeleteCredential` (owner retirement refused with 409 until the switch is applied); REST `POST /v1/postgres/{id}/credentials` (202, no body, an ordinary write as on Render); GraphQL `databaseCredentials`/`createDatabaseCredential`/`deleteDatabaseCredential`; MCP `list_/create_/delete_postgres_credential(s)`; Blueprint `fromDatabase` reading the current connection Secret; dashboard Credentials section.
- **Real CNPG 1.30.1** (`scripts/pg-credential-rotation-check.sh`, throwaway kind cluster, deleted afterwards): the new default reads/writes the owner's tables and creates objects; the retired owner is refused and keeps its table; after a second rotation and retiring the earlier credential, its tables stay reachable. PASS.
- **Render parity (t007):** REST matches Render's `create-postgres-user` (202, no body) and `delete-postgres-user`; GraphQL/MCP are bex additions (Render's MCP has no credential tool) and return the resulting list; the dashboard follows Render's Credentials section ("New default credential", default badge, delete locked on the default). One deliberate addition: retiring the owner answers 409 until the new default is applied, a few seconds after the create. Recorded in ADR018 and ADR009.
- **Simplify (t008):** three-way review applied — role chain for second rotations, owner-retirement gate, status-backed Blueprint references, `PoolerSecretName`, single fetch in `DeleteCredential`, shared GraphQL/MCP credential helpers, no extra reconcile read for never-rotated databases, dashboard cache write instead of refetch.
- **Tests (t009):** operator unit + fake-client projection tests, REST conformance flow, MCP rotation test, Blueprint reference test, dashboard hook/panel tests.
- **Remaining:** t005 — after deploy, rotate a disposable production database with a Blueprint-linked service through the Render-shaped REST path, sync, retire the old credential, confirm the service stays up and the old password is refused; then t010.

## Closeout (2026-11-05) — live production walk (t005)

Production `cc3c51e12` (pin `e3211740a`). A disposable free Postgres `w5-m265-rotation` (`dpg-dbmauekrv9ic73d23df0`) was created in the QA user's `tian-personal` workspace and driven through the Render-shaped REST path with `psql` over the external endpoint. No connection string or password was printed.

- Create: 201, `available`. The owner `dpg_dbmauekrv9ic73d23df0_user` logs in and creates a table.
- `POST …/credentials {"username":"rotated"}`: **202, empty body**. `GET …/credentials` shows `rotated` as `default:true` and the owner as `default:false`.
- Connection info then logs in as **`rotated`**, which reads the owner's table. The owner still works before deletion.
- `DELETE …/credentials/rotated` (the default): **400**. `DELETE …/credentials/<owner>`: **200**. A fresh owner login is then refused (`FATAL: role … is not permitted to log in`), while `rotated` still logs in and reads the owner's table. The credential list is `[rotated (default)]`.
- Cleanup: the database is deleted (204, then 404), no `w5-` leftovers remain, and the QA session is logged out.
- **Not walked live:** a Blueprint-linked service picking up the new default on its next sync. It is covered by `TestResolveDatabaseReferenceFollowsTheDefaultCredential` and the operator projection tests.

The definition of done holds: rotation through the Render path, a default that cannot be deleted, an owner retired without dropping its objects, and connection info following the new default.
