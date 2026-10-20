# CLI sweep 856 — duplicate service names refused with 409 (live, 2026-10-20)

Disposable free web service created, duplicate creates refused, service deleted (204; GET 404; name list `[]`). No new bug filed.

- Re-creating with the fixture's own name in the same workspace → coded 409 `CONFLICT`: `name "qa856-web" is already in use`.
- Creating with the baseline's name (`hello-go`, same workspace) → the same coded 409 — uniqueness is workspace-scoped and enforced at create, matching the `serviceNameAvailable` answer surface (sweep 855). The baseline was never touched.
