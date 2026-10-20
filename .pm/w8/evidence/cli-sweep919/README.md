# CLI sweep 919 — deploy-hook invocation actually deploys (live, 2026-10-20)

Disposable free web services created, hook invoked, deleted (204s; GETs 404). The hook URL/token was handled in memory only, never printed. No new bug filed.

- Invoking the hook URL (GET, browser UA) → 200 `{"deploy":{"id":"dep-…"}}`, and the deploy list gains a `trigger: "deploy_hook"` / `bexTrigger: "deploy_hook"` row ahead of the create deploy; a POST to the same URL triggers another. Both verbs fire the hook.
- A forged token (`dhk-bogus…`) → uniform 401 `unauthorized` — no distinction between malformed and unknown tokens.
- First attempt from the default Python UA got Cloudflare `403 error code: 1010` (bot wall, before the API) — noted so it is not later mistaken for hook auth; with a browser UA the same URL answers 200.
