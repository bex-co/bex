# CLI sweep 1188 — subdomain policy cannot strand a service (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `renderSubdomainPolicy: disabled` with no verified custom domain → 400 explaining the service would have no address left. The guard prevents an unreachable-service state rather than applying the flag blindly.
- A bogus policy value → 400; `enabled` → 200.
