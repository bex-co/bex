# In a protected environment only Admins read datastore passwords, change datastore IP rules or touch environment groups' variables, as on Render

Why: Render's protected environments keep these to Admins, and on bex a Developer can do all three. Found by w4/m553 t002 on 2026-11-22 from the changelog entries of 2024-04-16 ("Designate environments as protected") and 2024-08-14 ("Added restrictions and audit tracking for viewing secret values") ([disposition table](../../../../docs/render-artifacts/render-changelog.md)).

- **Render** ([render.com/docs/projects](https://render.com/docs/projects), read 2026-11-22). Only Admins in a protected environment may do these:
  - modify access control IPs for any Postgres or Key Value instance;
  - view or modify env vars or secret files for any service or environment group;
  - view passwords or connection URLs for any Postgres or Key Value instance.
- **bex** already gates a protected service's own env vars and secret files (`lego/backend/internal/secrets/service.go`, w8/193). It audits the datastore credential views (w4/m321). The rest has no protection check:
  - `PostgresConnectionInfo` (`lego/backend/internal/postgres/service.go`) and `KeyValueConnectionInfo` (`lego/backend/internal/keyvalue/service.go`) check only `can_view_sensitive`, which a Developer holds (`deploy/gitops/authz/model.fga`).
  - Environment group reads and writes check `RelCanViewSensitive` and `RelCanCreate` (`lego/backend/internal/envgroups/service.go`).
  - `protectedDatabasePatchVerb` and `protectedKeyValuePatchVerb` (`postgres/protection.go`, `keyvalue/protection.go`) never cover `ipAllowList`.
  - ADR032 lists a linked group's content as a residual with no sign-off.
- **End state:** for a member of a protected environment, these three are refused with `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED` on REST, GraphQL and MCP for a non-Admin, as the other protected verbs are. The dashboard shows them disabled with that reason. Blueprint syncs keep Render's exception. ADR032's residual line is updated. Tests cover each verb for an Admin and a Developer.
