# CLI sweep 821 — GraphQL spot parity (live, 2026-10-20)

Read-only GraphQL probes. No new bug filed.

- `{ services { id name type } }` answers the caller-default-workspace list (the same documented default-workspace scoping as REST without `ownerId`, sweep 809 — the harness's workspace header does not rescope GraphQL either; workspace is an explicit argument on this surface).
- `{ service(id: srv-daif…) { id name notificationsToSend } }` resolves the canary baseline by id and carries `notificationsToSend: "default"` — the GraphQL side of the notifications contract is intact; [w8/138](../../138.md) is specifically the REST single-override object's missing `serviceId`.
- Unknown field → GraphQL `errors` with locations, HTTP 200 per GraphQL convention.
