# Render datastore outbound-webhook events

Captured 2026-07-15 from Render's public [Webhook documentation](https://render.com/docs/webhooks). This artifact pins the datastore portion of the outbound-webhook vocabulary used by bex.

## Payload contract

Every event uses Render's thin envelope:

```json
{
  "type": "postgres_created",
  "timestamp": "2026-07-15T12:00:00Z",
  "data": {
    "id": "evt-…",
    "serviceId": "dpg-…",
    "serviceName": "orders"
  }
}
```

`data.status` appears only on the documented ended events that define it. None of the datastore events implemented here carries status.

## Official vocabulary

Render documents these Postgres-specific types:

- `postgres_available`
- `postgres_backup_completed`
- `postgres_backup_failed`
- `postgres_backup_started`
- `postgres_cluster_leader_changed`
- `postgres_connection_pool_changed`
- `postgres_connection_pool_enabled_changed`
- `postgres_created`
- `postgres_credentials_created`
- `postgres_credentials_deleted`
- `postgres_disk_size_changed`
- `postgres_ha_status_changed`
- `postgres_pitr_checkpoint_completed`
- `postgres_pitr_checkpoint_failed`
- `postgres_pitr_checkpoint_started`
- `postgres_restarted`
- `postgres_restore_failed`
- `postgres_restore_succeeded`
- `postgres_unavailable`
- `postgres_upgrade_failed`
- `postgres_upgrade_started`
- `postgres_upgrade_succeeded`
- `postgres_read_replica_stale`
- `postgres_read_replicas_changed`
- `postgres_wal_archive_failed`
- `postgres_disk_autoscaling_enabled_changed`

Render documents only these Key Value-specific types:

- `key_value_available`
- `key_value_config_restart`
- `key_value_unhealthy`

Render separately documents the generic `plan_changed` type for an instance-type change. It does not publish a datastore-specific plan-change name; bex applies that exact generic type when a managed datastore's plan changes. This is an inference from the shared thin `serviceId` payload and instance-type semantics, recorded explicitly rather than presented as a datastore-specific promise in Render's prose.

The official list contains no managed-datastore suspend, resume, or delete event type, and no Key Value create event type. bex does not invent names for those lifecycle writes.

## Sourceable bex mapping

| Render type | Durable bex source |
| --- | --- |
| `postgres_created` | Successful Postgres CR creation |
| `postgres_restarted` | Successful Postgres restart-request write |
| `postgres_credentials_created` | Successful managed-role creation |
| `postgres_credentials_deleted` | Successful managed-role deletion |
| `postgres_backup_started` | Successful export/backup-request write |
| `plan_changed` | Successful changed plan write for a service, Postgres, or KV |

Each successful write records one fixed, typed audit effect after the Kubernetes mutation succeeds. The existing durable webhook worker projects that row, so the delivery queue, retry behavior, and signing path remain shared with service events. The payload's `serviceId` is the immutable `dpg-…` or `red-…` resource id; `serviceName` is captured in the same audit row so a later rename cannot rewrite historical deliveries.

## Honest omissions

`postgres_available`, `postgres_unavailable`, `key_value_available`, and `key_value_unhealthy` describe observed status transitions, not API write completion. Backup completion/failure and the remaining Postgres types likewise require a durable operator/status event source or a feature bex does not yet implement. The operator is mechanism-only and DB-free, so m26 does not create an operator-to-control-plane write path merely to manufacture these events. They remain omitted, not approximated by request acceptance.
