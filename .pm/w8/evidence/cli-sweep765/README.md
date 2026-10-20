# CLI sweep 765 — platform-subdomain policy guard (live, 2026-10-19)

Disposable free web service `qa765-web` (`srv-dbbco9i720pc73c0kmo0`). Deleted and verified absent (REST 404).

- `PATCH serviceDetails.renderSubdomainPolicy: "denied"` → 400 `renderSubdomainPolicy must be enabled or disabled` (Render's enum).
- `"disabled"` → 400 `the platform subdomain can't be disabled until a custom domain is verified, since the service …` — the guard refuses to strand the service without a verified custom domain first.
- The service deployed live meanwhile (`urls:["https://qa765-web.onbex.co"]`, public 200).

No bug filed.
