# CLI sweep 989 — rename conflicts and empty-name PATCH (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- Renaming onto an existing service's name (`hello-go`, the baseline — untouched) → coded 409 `CONFLICT` naming the name. The baseline was only read as the collision target; never modified.
- Renaming to the service's own current name → 200.
- `PATCH {name: ""}` → 200 and the name is unchanged on read-back: an empty string is treated as unset under PATCH semantics, not as a rename to blank.
