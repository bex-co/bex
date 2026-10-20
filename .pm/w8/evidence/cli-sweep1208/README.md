# CLI sweep 1208 — rollback route exists; port-below-1024 fix awaits the pin (live, 2026-10-20)

Two disposable free web services created and deleted (204, verified 404). No new bug filed.

- `POST /services/{id}/rollback` is a real route: an empty body → 400 (`request body does not match the Render API schema`), a well-formed `{deployId}` on a just-created service → 409 (no live deploy to roll back from). Deploy detail reads 200; a per-deploy rollback guess → 404.
- Port `80` PATCH → 400 at the current pin (`9e9c28e82d0f`). The fix (`b01debc5d`, accept service ports below 1024 as Render does) has landed on `origin/main` but is not in the production pin yet — retest on the next pin instead of filing.
