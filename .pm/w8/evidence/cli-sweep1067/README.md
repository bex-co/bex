# CLI sweep 1067 — post-fix interop: bulk and single-rule ops share one rule set (live, 2026-10-20)

One disposable static site created via the CLI recipe and deleted (204; GET 404). No new bug filed.

- Bulk `PUT` two headers → 200; single `DELETE` of one → 204 with only the other remaining; single `POST` re-adds it → 201; bulk `PUT []` clears all. The w8/137 single-rule operations and the pre-existing bulk pair operate on the same rule set without drift.
