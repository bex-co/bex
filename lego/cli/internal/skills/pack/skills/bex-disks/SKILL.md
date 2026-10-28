---
name: bex-disks
description: >-
  Attaches and manages persistent disks on bex services—mount paths, sizing,
  snapshots, file transfers, and single-instance constraints. Use when the user
  needs persistent storage, file uploads, a custom database on disk, CMS media
  storage, or needs to understand why their service can't scale horizontally
  or use zero-downtime deploys.
  Trigger terms: persistent disk, disk, storage, mount path, sizeGB, SSD,
  file uploads, snapshots, disk restore, ephemeral filesystem.
license: MIT
compatibility: bex paid web services, private services, and background workers
metadata:
  author: bex
  version: "1.0.0"
  category: storage
---

# bex Persistent Disks

Persistent disks preserve files written beneath a service's configured mount path across deploys and restarts.

Before designing, configuring, resizing, transferring data to, snapshotting, restoring, or troubleshooting a disk, read `references/persistent-disks.md`.

## When to Use

- Storing **file uploads**, CMS media, or user-generated content
- Running a **self-managed database** (MySQL, MongoDB, ClickHouse) on bex
- Deploying **stateful infrastructure** (Elasticsearch, Kafka, RabbitMQ, Mattermost)
- Understanding **why scaling is blocked** or **zero-downtime deploys are disabled**
- **Restoring data** from a disk snapshot

For managed databases, prefer **bex Postgres** (bex-postgres) or **Key Value** (bex-keyvalue) over self-managed alternatives on disk.

## Core Decisions

- bex's default filesystem is ephemeral. Only files written beneath a disk's mount path persist across deploys and restarts.
- A disk-backed service is single-instance: it cannot use horizontal scaling or zero-downtime deploys.
- If file storage must be shared by scaled instances, use external object storage such as S3 or R2. Use bex Postgres for relational data and bex Key Value for managed cache or queue state.
- Choose the smallest current disk size that accommodates observed usage because capacity can increase but cannot decrease. For a small upload service, 1–5 GB is a reasonable starting range when supported by the current limits.

## Blueprint Setup

For a native Node.js upload service, mount a subdirectory such as `/opt/render/project/src/uploads`, not the source root itself, and configure the application to write uploads there:

```yaml
services:
  - type: web
    name: uploads
    runtime: node
    buildCommand: npm ci
    startCommand: npm start
    disk:
      name: uploads-data
      mountPath: /opt/render/project/src/uploads
      sizeGB: 5
```

Fetch the current disk documentation and Blueprint specification through `references/persistent-disks.md` before finalizing the mount path or size.

## Database Backups and File Recovery

Do not use a disk snapshot to recover a self-managed database such as MySQL: a filesystem snapshot can restore an inconsistent or corrupted database state. Use database-native backups such as `mysqldump` and test their restore procedure. Move backup files off the service with a currently supported transfer method such as SCP with SSH or Magic-Wormhole. Snapshot restores replace the entire disk and permanently discard changes made after the selected snapshot.

## References

| Document | Contents |
|----------|----------|
| `references/persistent-disks.md` | Current disk constraints, configuration, sizing, snapshots, restores, and transfers |

## Related Skills

- **bex-web-services** — Deploy lifecycle, health checks (disk disables zero-downtime)
- **bex-private-services** — Internal services with disks (Elasticsearch, MySQL)
- **bex-blueprints** — `disk` field reference in `render.yaml`
- **bex-postgres** — Managed database alternative (no disk management needed)

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
