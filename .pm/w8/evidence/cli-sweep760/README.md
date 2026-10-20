# CLI sweep 760 — blueprint generate resource scoping (live, 2026-10-19)

Read-only generation; nothing created.

- `serviceIds:[synthetic]` → 404 `NOT_FOUND`; a mixed real+synthetic list → the same 404 (the selection resolves atomically, no partial manifest).
- Selection fields are `serviceIds`, `postgresIds`, `keyValueIds`, `envGroupIds` (handler struct, `lego/backend/internal/apps/rest.go:1385`). A `databaseIds` guess is not a field: it decoded to an empty selection and got the named `select at least one resource to generate a Blueprint from` refusal.
- `postgresIds:[<baseline pg>]` → 200 with a `databases:` manifest for the baseline (197 chars, config facts only).

No bug filed.
