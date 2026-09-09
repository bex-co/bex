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
| `postgres_ha_status_changed` | Successful PATCH that changed high availability (w3/m82) |
| `postgres_connection_pool_enabled_changed` | Successful PATCH that toggled the PgBouncer pooler (w3/m82) |
| `postgres_disk_size_changed` | Successful PATCH that changed the disk size (w3/m82) |
| `key_value_config_restart` | Successful PATCH of `maxmemoryPolicy`/`persistenceMode`, which the operator folds into the Valkey StatefulSet's args and therefore rolls the pod (w3/m82) |

Each successful write records one fixed, typed audit effect after the Kubernetes mutation succeeds. The existing durable webhook worker projects that row, so the delivery queue, retry behavior, and signing path remain shared with service events. The payload's `serviceId` is the immutable `dpg-…` or `red-…` resource id; `serviceName` is captured in the same audit row so a later rename cannot rewrite historical deliveries. The four w3/m82 configuration events additionally carry the value the field was set **to** in `GET /v1/events/{id}`'s `details`.

## Observed datastore facts (w3/m82)

The status-transition types below are no longer omitted. `datastore_event_facts` (migration 0107) is the Database/KeyValue twin of `service_event_facts`: the control-plane reconciler derives a small typed snapshot from `Database`/`KeyValue` status on each pass and records only the edges it crossed, against a per-datastore checkpoint. That is what makes a level-triggered 30-second poll emit one event per transition rather than one per tick, and it keeps the operator mechanism-only — bex-api observes the CR, the operator never writes to the control plane.

| Render type | Durable bex source |
| --- | --- |
| `postgres_unavailable` / `postgres_available` | Ready-condition availability edge on a `Database`, latched through the checkpoint |
| `key_value_unhealthy` / `key_value_available` | The same edge on a `KeyValue`, under Render's own asymmetric spelling |
| `postgres_backup_completed` / `postgres_backup_failed` | Terminal `Database.status.lastBackup` projected by the operator |
| `postgres_restore_succeeded` / `postgres_restore_failed` | Terminal outcome of a `Database` created as a recovery target |
| `postgres_upgrade_started` / `postgres_upgrade_succeeded` / `postgres_upgrade_failed` | Major-version transition between `spec.version` and the observed current version |

Two rules keep availability truthful rather than noisy. A datastore that has never reported Ready is **provisioning, not down** — the outage edge arms only after the first healthy observation, because a CNPG cluster with zero ready instances reports the same phase whether it is being created or has just lost its only instance. And a **suspended** datastore is intentional downtime: it is observed availability-empty, never unavailable.

## Honest omissions

`postgres_cluster_leader_changed`, `postgres_connection_pool_changed`, `postgres_disk_autoscaling_enabled_changed`, the three `postgres_pitr_checkpoint_*` types, `postgres_read_replica_stale`, and `postgres_wal_archive_failed` still require a durable source or a feature bex does not yet implement. `postgres_read_replicas_changed` is a distinct case: read replicas are create-time-only in bex (no patch verb writes them after create), so there is no mutation to source the event from — it stays unsupported until a replica add/remove verb exists. None of these is approximated by request acceptance.
