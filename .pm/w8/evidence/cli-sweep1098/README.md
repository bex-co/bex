# CLI sweep 1098 — create is atomic; secret files inline at create (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A create carrying an invalid inline env key → coded 400 `ENVIRONMENT_VARIABLE_INVALID`, and no service exists afterwards (the name list is empty): create validates the whole body before writing anything.
- Create with inline `secretFiles` → 201; the list shows the file name only (content withheld, as in sweep 957).
