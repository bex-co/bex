# CLI sweep 1221 — secret-file content may be empty (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed. All fixture contents were empty/dummy values.

- Named PUT with `content: ""` → 201 and GET returns the empty content: zero-byte secret files are valid.
- Omitting `content` entirely → 201 (defaults to empty, like the service-settings empty-string-as-unset pattern). An empty name segment in the path → 404.
