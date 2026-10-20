# CLI sweep 703 — service list filters (live, 2026-10-19)

Read-only.

- `GET /services?type=bogus` → 400 `type must be one of static_site|web_service|private_service|background_worker|cron_job`; `?suspended=true` → 400 `suspended must be one of suspended|not_suspended` (enum values, not booleans). `?type=cron_job` → 200 with the workspace's cron services.
- `GET /services?name=hello-go` → `[]`, but `?ownerId=<canary>&name=hello-go` → the baseline. Without `ownerId` the list resolves the caller's **default** workspace (where `hello-go` does not live) — the documented owner-resolution rule (ADR018), applied consistently; the `X-Bex-Workspace` header does not substitute for the query owner on this route.
- `name[]=…` array spelling → 400 `unsupported query parameter "name[]" for list-services`.

No bug filed.
