# CLI sweep 1104 — MCP notification update lands in the REST override (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- MCP `update_service(notificationsToSend:"failure")` → 200, and the REST override GET immediately reads `failure` back — with the post-w8/138 `serviceId` present. The notification store is shared across REST, GraphQL (sweep 952), and MCP.
