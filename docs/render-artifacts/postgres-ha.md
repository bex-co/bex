# Render Postgres HA / Failover / readReplicas — Verified API Reference

Captured 2026-07-12 against Render's live API (`api-docs.render.com`). Used as the parity reference for w1/m22 so field names are evidence-backed, not asserted. All three endpoints were verified via the public OpenAPI spec and per-endpoint reference pages.

---

## Create Postgres — `POST /v1/postgres`

Request body fields (full set; bex-relevant subset shown):

```json
{
  "name": "my-db",
  "plan": "basic-1gb",
  "version": "16",
  "databaseName": "mydb",
  "databaseUser": "mydb_user",
  "diskSizeGB": 10,
  "enableHighAvailability": true,
  "environmentId": "...",
  "ownerId": "...",
  "region": "oregon",
  "ipAllowList": [{"cidrBlock": "0.0.0.0/0", "description": ""}],
  "readReplicas": [{"name": "replica-1"}],
  "connectionPool": {...},
  "parameterOverrides": [],
  "datadogAPIKey": null,
  "datadogSite": null
}
```

Key HA fields:

- **`enableHighAvailability`** (`boolean`, default `false`) — provisions a replicated cluster (primary + standby). **Independent of `readReplicas`**: a database can have read replicas without HA, and HA does not automatically create queryable replicas.
- **`readReplicas`** (`array of {name: string}`) — each named replica is a separately addressable read-only resource with its own connection URL. **Not derived from HA standbys** — each is independently provisioned.

---

## Get Postgres — `GET /v1/postgres/{postgresId}`

Response body (relevant HA fields):

```json
{
  "id": "...",
  "name": "my-db",
  "plan": "basic-1gb",
  "status": "available",
  "highAvailabilityEnabled": true,
  "readReplicas": [
    {
      "name": "replica-1",
      "connectionInfo": { ... }
    }
  ],
  ...
}
```

Key read fields:

- **`highAvailabilityEnabled`** (`boolean`) — reports actual HA state (true when the replicated cluster is live; false when single-instance or HA is off).
- **`readReplicas`** — array of named replica objects, each with connection info.

---

## Failover — `POST /v1/postgres/{postgresId}/failover`

- **HTTP method + path:** `POST /v1/postgres/{postgresId}/failover`
- **Request body:** none (empty)
- **Response:** `202 Accepted` — "Service failed over successfully"
- **There is no `promote` endpoint.** Failover is the only operation; `promote` does not exist in Render's Postgres API.
- Error responses: 400, 401, 403, 404, 406, 410, 429, 500, 503

Description: triggers a CNPG switchover (promote a standby to primary). Only meaningful when `highAvailabilityEnabled` is true.

---

## Summary of field names

| Render field | Direction | bex mapping |
| --- | --- | --- |
| `enableHighAvailability` | write (create/update) | `spec.highAvailability` in Database CR; `enableHighAvailability` in API body |
| `highAvailabilityEnabled` | read | `status.highAvailabilityEnabled`; `PostgresView.HighAvailabilityEnabled` |
| `readReplicas: [{name}]` | write | `spec.readReplicas: [{name}]` in Database CR |
| `readReplicas: [{name, connectionInfo}]` | read | `status.readReplicaStatuses`; `PostgresView.ReadReplicas` |
| `POST .../failover` → 202 | action | `spec.failoverAt` (verb-as-timestamp); REST endpoint + MCP tool |

**No `promote` verb exists in Render's API** — only `failover`. Any prior bex notes mentioning `promote` were incorrect (this reference supersedes them).
