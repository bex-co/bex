# CLI sweep 1041 — no REST clone subresource (live, 2026-10-20)

One disposable free web service created and deleted (204; clone name list empty). No new bug filed.

- `POST /services/{id}/clone` → 404: cloning is a create-side operation (the CLI's `services create --from` and the create body's source fields), not a subresource verb. The guessed route's absence is the designed shape.
