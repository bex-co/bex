# CLI sweep 1199 — the scale gate cannot be bypassed through PATCH (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH `serviceDetails.numInstances` (2 or 0) → 400 `unknown field "numInstances"`: instance count is not in the PATCH schema at all, so the free-plan ceiling from sweep 1136 can only be reached through the dedicated `/scale` endpoint, which enforces it. There is no second, ungated write path.
- `/services/{id}/previews` → 404: no previews collection on this route shape.
