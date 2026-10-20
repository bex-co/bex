# CLI sweep 1192 — env-group secret files list names only (live, 2026-10-20)

One disposable env group created and deleted (204, verified 404). No new bug filed.

- Named secret-file PUT on the group → 200; the group read then lists `secretFiles: [{name}]` with no content field — group reads expose file names, never contents (the dedicated named GET remains the only content path, as on services, sweep 1139).
- Secret-file DELETE → 204. A batch var PUT with a digit-leading/dashed key → 400, matching service env-key discipline.
