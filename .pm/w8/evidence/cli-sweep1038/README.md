# CLI sweep 1038 — suspend/resume leave an event trail (live, 2026-10-20)

One disposable free web service created, suspended, resumed, and deleted (204). No new bug filed.

- After a suspend/resume pair, the service event list records `suspender_added` then `suspender_removed` (newest first), alongside the deploy events — the control-plane actions from sweep 1032 are auditable from the resource's own event stream.
