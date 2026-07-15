# Render Postgres logical exports capture

Captured 2026-07-14 for `w2/m35` from Render's public OpenAPI and Postgres recovery documentation.

## REST contract

Render uses the singular collection path for both operations:

| operation | endpoint | response |
| --- | --- | --- |
| List Postgres exports | `GET /v1/postgres/{postgresId}/export` | `200` JSON array |
| Create Postgres export | `POST /v1/postgres/{postgresId}/export` | `202`, no documented response body |

The list item schema is:

```json
{
  "id": "string (required)",
  "createdAt": "RFC3339 date-time (required)",
  "url": "string (optional download URL)"
}
```

There is no request body. Render documents the create errors `400`, `401`, `403`, `404`, `406`, `410`, `429`, `500`, and `503`; the list documents `400`, `401`, `404`, `429`, `500`, and `503`.

Sources: [Render OpenAPI spec](https://api-docs.render.com/openapi/render-public-api-1.json), [List Postgres exports](https://api-docs.render.com/reference/list-postgres-export), [Create Postgres export](https://api-docs.render.com/reference/create-postgres-export).

## Artifact and lifecycle

- Render produces a logical `pg_dump` directory-format export packaged as `.dir.tar.gz`.
- The example filename is timestamp based: `2025-02-03T19_21Z.dir.tar.gz`.
- Restore extracts the archive and runs `pg_restore --format=directory` against the database directory inside it.
- Only one export can run for a Postgres instance at a time.
- Logical exports are unavailable on Render's Free instance type.
- Render retains each logical export for seven days after creation, independent of workspace plan.

Source: [Render Postgres recovery and backups](https://render.com/docs/postgresql-backups#logical-backups).

## Download behavior and bex decisions

Render's OpenAPI says the list returns a download URL but does not publish that URL's lifetime or whether it is a presigned object-store URL. The URL field is optional, consistent with it being absent until an export is ready.

bex matches the observable shape and makes the following explicit decisions where Render's public artifact is silent:

- `GET /v1/postgres/{id}/export`, GraphQL `databaseExports`, and MCP `list_postgres_exports` require `can_view_sensitive`, because a dump contains the entire database.
- An available export gets a fresh S3-compatible presigned URL on each authorized list read. The URL is never stored; it expires after 15 minutes or at the artifact deadline, whichever comes first.
- The shared export object is a safe superset of Render's schema: `id`, `createdAt`, and optional `url`, plus `status`, `urlExpiresAt`, `expiresAt`, `filename`, and `failureReason` for an honest UI/agent lifecycle.
- Status progresses `created` → `running` → `available` or `failed`; expiry uses `expiring` → `expired` and is not marked complete until the object-delete Job succeeds.
- The operator runs version-compatible `pg_dump --format=directory`, packages the output as `.dir.tar.gz`, and uploads it below `<BEX_DB_BACKUP_DESTINATION>/logical-exports/<database>/<export-id>/`. Dump bytes never transit bex-api.
- bex keeps the existing plural `/exports` endpoint as a compatibility alias; the documented Render-compatible route is singular `/export`.

The intentional product difference is plan eligibility: bex exposes exports whenever the Database reports a durable plan and configured backup store. That is the same gate as bex PITR; it is not coupled to Render's commercial plan names.

## Reproducible verification

Run `bash scripts/postgres-export-verify.sh`. The Docker-only check seeds a vanilla PostgreSQL 16 source, runs the same directory-format dump and tar command as the operator, uploads the artifact to a private MinIO bucket with the operator's pinned AWS CLI, asserts an unsigned object request gets `403`, downloads through a 15-minute signed URL, and restores into a separate vanilla PostgreSQL 16 target. It succeeds only when the restored row is `1:portable`.

The Go suites separately prove the Kubernetes Job shape and lifecycle (including failure and retention cleanup), `can_view_sensitive` denial, signed-URL expiry, and REST/GraphQL/MCP create/list parity. The dashboard test covers available-download, failed-reason, and in-progress disabled states.
