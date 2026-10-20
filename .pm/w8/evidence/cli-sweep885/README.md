# CLI sweep 885 — create-name boundary is exactly 30 (live, 2026-10-20)

Boundary-exact probes (names generated programmatically, lengths asserted — the sweep-714 lesson); the one created service was deleted (204). No new bug filed.

- Create with a 30-char name → 201; 31 chars → coded 400 `RESOURCE_NAME_INVALID` with the full rule; a leading hyphen → the same coded refusal. The identity-name convention (≤30, no leading/trailing hyphen) is enforced at exactly the documented bound, pairing with sweep 884's free-form post-create display label — create and rename deliberately use different conventions, each consistent within itself.
