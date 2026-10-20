# CLI sweep 1127 — lifecycle verbs are idempotent, restart refuses while suspended (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `resume` on a never-suspended service → 202 (no-op, not an error).
- `restart` on a running service → 200.
- `suspend` → 202; `restart` while suspended → 409 (state conflict, not a silent no-op).
- `resume` → 202, and a second `resume` → 202 again; final read-back is `not_suspended`.

The verb set distinguishes idempotent state requests (repeat freely) from an action that is meaningless in the current state (409).
