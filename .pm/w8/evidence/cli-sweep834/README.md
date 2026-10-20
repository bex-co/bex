# CLI sweep 834 — IP allow list lives on environments/datastores, not service PATCH (live, 2026-10-20)

Disposable project+environment and a disposable web service created, cycled, deleted (204s; GETs 404). No new bug filed.

- `PATCH /services/{id} {"ipAllowList":[…]}` → 400 `unknown field "ipAllowList"` — correct layering, not a gap: service-level allow lists are environment-scoped on bex (datastores carry their own `/ip-allow-list` subresource, `lego/backend/internal/postgres/rest.go:348`, `keyvalue/rest.go:219`).
- Environment cycle: `PATCH /environments/{id}` with one `/32` entry → applied verbatim; bogus CIDR `999.1.1.1/32` → named 400 `"999.1.1.1/32" is not a valid CIDR`; restoring the allow-all pair round-trips.
