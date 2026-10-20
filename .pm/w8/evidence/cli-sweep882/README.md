# CLI sweep 882 — secret-file name traversal discipline (live, 2026-10-20)

Disposable free web service created, exercised, deleted (204, then GET 404). No new bug filed.

- `PUT …/secret-files/..%2Fqa882.txt` and `…/%2Fetc%2Fqa882` → coded 400 `SECRET_FILE_INVALID` quoting the name, the allowed charset regex (`[-._a-zA-Z0-9]+`), the no-`..`-prefix rule, and `params.field`/`maxLength` — path traversal into the secret store is refused by name before any write.
- A plain name → 201 (the pinned single-PUT `201 Created`, vs the bulk PUT's 200) and lists back; only the legitimate file existed.
