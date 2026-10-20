# CLI sweep 1066 — override list and single-object now agree (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Post-w8/138-fix consistency check: the workspace override list contains a row for the fresh fixture carrying `serviceId`, and the single-object GET (sweep 1033) carries the same id — the two answers that disagreed in the original filing now match.
