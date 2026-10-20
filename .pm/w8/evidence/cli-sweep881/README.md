# CLI sweep 881 — env-var list write discipline (live, 2026-10-20)

Disposable free web service created, exercised, deleted (204, then GET 404). No new bug filed.

- `PUT …/env-vars []` → 200 (empty replace is a valid clear).
- Duplicate keys in one PUT (`QA881_D` twice) → a single stored entry with the **last** value (`"2"`, confirmed by per-key GET with its `evr1_…` revision) — no duplicate-key 400 and no doubled row; last-wins within the request.
- Lowercase keys are accepted verbatim (`qa881_lower`) — bex does not impose an uppercase convention the pinned schema does not declare. Recorded; Render's own case discipline is unverified, and inventing a refusal would break the schema contract.
