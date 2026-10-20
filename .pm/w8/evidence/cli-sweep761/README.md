# CLI sweep 761 — audit-log query bounds (live, 2026-10-19)

Read-only.

- `limit=1000` → 200; `limit=1001` → 400 `limit must be an integer between 1 and 1000` (the sweep-679 bound is exact).
- `?action=bogus.action` → 400 `unsupported query parameter "action" for list-owner-audit-logs` — no action filter exists on the route, and the validator names the operation.

No bug filed.
