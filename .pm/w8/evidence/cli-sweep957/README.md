# CLI sweep 957 — secret files lifecycle (live, 2026-10-20)

Two disposable free web services created, exercised, deleted (204s; GETs 404). No new bug filed.

- Secret files use Render's PUT replace-set shape, not POST: a POST create correctly 405s, while `PUT /services/{id}/secret-files` with a two-file set → 200.
- The list endpoint returns names only (no content); a single-file GET returns the content to the owner (the documented sensitive read). The PUT response echoes what the caller just sent — no cross-tenant leak surface observed.
- Replace semantics hold: a second PUT naming only `a.txt` makes `b.txt` read 404.
- Path-traversal name (`../evil`) → coded 400 `SECRET_FILE_INVALID` with `params.field=name`, `maxLength=253`, and the validation regex spelled out.
- Single-file DELETE → 204, list empty afterwards.
