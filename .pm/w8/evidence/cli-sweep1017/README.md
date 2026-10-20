# CLI sweep 1017 — platform subdomain cannot be orphaned (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- PATCH `renderSubdomainPolicy: "disabled"` with no verified custom domain → coded 400 explaining the invariant: `the platform subdomain can't be disabled until a custom domain is verified, since the service would have no address left`. The policy stays `enabled` and the `onbex.co` URL intact.
- A bogus policy value → 400; re-setting `enabled` → 200.
