# CLI sweep 790 — dedicated-IPs non-goal vs outbound-IPs read (live, 2026-10-20)

Read-only probes; pin re-checked (`9d7ac8afa` → `ca30db3a3646`, unchanged). No new bug filed.

- `GET /dedicated-ips?ownerId=…` (and without `ownerId`) → 404 `not found`. Documented non-goal, not a bug: [ADR018](../../../docs/ADR018-render-parity.md) keeps workspace dedicated outbound IPs at `—` ("dedicated outbound IPs stay a `—` non-goal"); the route is simply not served.
- The implemented sibling works: `GET /services/{hello-go}/outbound-ips` → 200 `{"type":"shared","ips":[…]}` with the tenant pool's current node IPs, exactly the truthful shared-egress semantics ADR018 describes.
- `GET /webhooks?ownerId=…` → 200 `[]`.
