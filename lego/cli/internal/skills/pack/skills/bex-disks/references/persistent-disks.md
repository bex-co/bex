<!-- shared:persistent-disks -->
## Persistent disks

Use this guidance only for service-attached persistent disks and for architecture or configuration decisions directly affected by them. Do not apply it to bex Postgres or Key Value storage, object storage, build caches, or ordinary ephemeral filesystem tasks.

By default, a bex service's filesystem is ephemeral. Files created or changed at runtime are lost when the service is redeployed or restarted. Use a persistent disk only when a supported service must retain local filesystem data across those events.

Before designing, configuring, or troubleshooting a persistent disk, fetch the current documentation:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read the [Markdown reference](https://render.com/docs/disks.md) or its [HTML version](https://render.com/docs/disks).

Read the relevant sections before relying on details that can change, including supported service types, size limits and pricing, mount-path restrictions, snapshot cadence and retention, transfer methods, and current scaling or deployment limitations. If the document cannot be fetched, do not guess those details from memory.

A persistent disk belongs to one service and is mounted at one absolute path. Only files written under that mount path persist; the rest of the service filesystem remains ephemeral. Make sure the application writes every intended durable file beneath the configured mount path. A disk is not shared storage, and another service cannot access it.

Under the documented generally available behavior, a disk-backed service runs as a single instance and cannot use autoscaling or zero-downtime deploys. During a deploy, bex stops the existing instance before starting its replacement, so expect a short interruption. If the workload needs horizontal scaling, shared state, or stronger availability, use an external or managed datastore instead of a service-attached disk.

The disk is available only to the running service. Builds, pre-deploy commands, and one-off jobs run on separate compute and cannot access it. Do not design a build step, pre-deploy command, or one-off job that depends on files stored on the mounted disk.

The documented supported service types are paid web services, private services, and background workers. For other workload types, use an appropriate external datastore or reconsider the service architecture rather than assuming a disk can be attached.

Disk capacity can be increased but not decreased. Confirm current limits and pricing in the fetched documentation, and choose capacity using observed usage plus reasonable headroom.

Treat attaching, resizing, or restoring a disk as a stateful mutation. After attaching, verify that the service deploys successfully and that reads and writes use the mounted path.

Snapshots are a disaster-recovery mechanism, not a substitute for application-consistent database backups. A snapshot restore replaces the entire disk; it cannot restore individual files, and it discards changes made after that snapshot. Do not use a disk snapshot to recover a custom database, because a filesystem-level snapshot can restore the database into an inconsistent or corrupted state. Use the database engine's native backup and restore tools instead. Check the current documentation before promising a particular snapshot schedule, retention period, or restore procedure.

Mount paths must be absolute and must satisfy bex's current restrictions. Fetch the documentation instead of reproducing its forbidden-path list or runtime-specific path suggestions from memory. When declaring a disk in a Blueprint, also fetch the current Blueprint specification and follow its disk schema rather than relying on remembered syntax.
<!-- /shared:persistent-disks -->

## Storage-specific guidance

- Persistent disk data is encrypted at rest.
- Expect the service to restart when restoring a snapshot, in addition to the destructive replacement of disk contents described above.
- Disk billing is based on provisioned capacity rather than used capacity. Confirm the current rate and billing behavior in the fetched documentation before estimating cost.

Prefer a managed or external store when the workload does not require single-service local filesystem persistence:

| Need | Prefer |
|------|--------|
| Managed relational database | bex Postgres |
| Managed cache or queue | bex Key Value |
| Large or shared file storage | External object storage |
| Temporary build artifacts | The default ephemeral build filesystem |
