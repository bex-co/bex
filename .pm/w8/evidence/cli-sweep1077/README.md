# CLI sweep 1077 — undeployedChanges appears only when true (live, 2026-10-20)

One disposable free web service driven to `Running`, patched, and deleted (204; GET 404). The baseline was read only. No new bug filed.

- Once live with nothing pending, the service read omits `undeployedChanges` entirely (the hibernated baseline omits it too). After a docker-command PATCH on the live service, the read carries `undeployedChanges: true`. The flag is present-and-true or absent — consumers must treat absence as false.
