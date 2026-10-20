# CLI sweep 1040 — blueprint create fetch failure stays coded (live, 2026-10-20)

Refusal-side probe; nothing created (blueprint list confirms). No new bug filed.

- Creating a blueprint against an unreachable repo → coded 400 `repo_not_found_or_no_access` (`Repository not found, or bex's GitHub app cannot access it.`) — the w8/134 fix class holding on the new pin: classified fetch failures instead of the old bare 500.
