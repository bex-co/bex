# CLI sweep 1194 — project service-links write service placement (live, 2026-10-20)

Disposable projects/services created and deleted (204, verified 404). No new bug filed.

- Environment-scoped link guesses 404/405. The declared surface is project-scoped: `PUT /projects/{id}/service-links` with `{serviceIds: [...]}`.
- After linking a real service, the project read does not echo a link list — the placement is visible on the service itself, whose read gains `projectId`. The environment's own `serviceIds` stays `[]`: project placement and environment membership are separate layers.
- Re-PUT with `serviceIds: []` → 200 and the service's `projectId` disappears (replace-all unlink).
- Unknown service ids in the list are tolerated (200, dropped); Render's answer for that case is unverified against the pinned contract, so it is recorded, not filed.
