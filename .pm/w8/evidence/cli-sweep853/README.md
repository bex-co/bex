# CLI sweep 853 — `renderSubdomainPolicy` toggle discipline (live, 2026-10-20)

Disposable free web services created, patched, deleted (204s; GETs 404). No new bug filed.

- Disabling the platform subdomain with no verified custom domain → named 400 `the platform subdomain can't be disabled until a custom domain is verified, since the service would have no address left` — a fail-closed safety refusal, identical via the nested `serviceDetails` and top-level spellings.
- Bogus value → 400 `renderSubdomainPolicy must be enabled or disabled`; re-asserting `enabled` → 200.
