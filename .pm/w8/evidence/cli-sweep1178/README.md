# CLI sweep 1178 — deletion is final across every verb (live, 2026-10-20)

One disposable free web service created and deleted. No new bug filed.

- After the first DELETE (204): a second DELETE, PATCH, suspend, deploy trigger, and env-var list all → 404. No verb resurrects or partially operates on a deleted service, and the delete itself is not idempotent-blurred (the repeat reports absence).
