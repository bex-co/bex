# CLI sweep 1191 — env-group link cycle; unlink is idempotent (live, 2026-10-20)

One disposable service and env group created and deleted (204 each, verified 404). No new bug filed.

- Link (`POST /env-groups/{id}/services/{serviceId}`, no body) → 200 with the group now carrying `serviceLinks: [{id, name, type: "web"}]` and an env-group revision.
- Unlink → 204, and a second unlink → 204 again: link removal is idempotent, in deliberate contrast to single env-key delete (sweep 1165), which 404s on the missing key. Each surface picks its own repeat semantics.
