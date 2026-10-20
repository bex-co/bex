# CLI sweep 952 — GraphQL notification mutation writes the same store REST reads (live, 2026-10-20)

Two disposable free web services created (one per probe shape), exercised, deleted (204s; GETs 404). No new bug filed.

- The mutation's shape is `setNotificationsToSend(id:, value:)` (a named-argument guess with `serviceId:`/`notificationsToSend:` args got precise `Unknown argument` errors). The correct form → 200 with the service object; the REST override GET immediately reads `failure` back — one store across surfaces, and the GraphQL return carries the service `id` that the REST single-override object omits ([w8/138](../../138.md), seen from the other side).
- Reset to `default` via GraphQL verified before deletion.
