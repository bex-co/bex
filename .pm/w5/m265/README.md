# w5 · m265 — Postgres credential rotation: a new user becomes the default, as on Render

**Worker:** worker5 **Goal:** a customer rotates a Postgres password with zero downtime: the new credential becomes the default, connection strings follow it, and the old one can be deleted (the original owner is deactivated, not dropped). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | CRD default-user pointer and original-owner deactivation; operator projects them to CNPG | 1h30m | — |
| t002 | Core + REST: `POST /v1/postgres/{id}/credentials` creates the new default; delete rules follow Render | 1h30m | t001 |
| t003 | Connection info and Blueprint `fromDatabase` follow the default user | 1h | t001 |
| t004 | GraphQL, MCP and dashboard Credentials section follow Render's flow | 1h30m | t002 |
| t005 | Live zero-downtime rotation walk on dev-5 or production | 45m | t003, t004 |
| t006 | ADR018 and ADR009 record the rotation contract | 20m | t005 |
| t007 | Render parity | 30m | t006 |
| t008 | Simplify | 20m | t007 |
| t009 | Test coverage | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

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
