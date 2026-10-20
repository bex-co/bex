# CLI sweep 940 — hibernation event types still unfilterable (w8/060 family, re-confirmed) (live, 2026-10-20)

Disposable fixture created/deleted (204, then GET 404); baseline only read. No new bug filed.

- `GET /services/{hello-go}/events?type=service_hibernated` (and `service_woken`) → 400 with the full Render event-type enumeration: the feed **emits** these bex-extension types (the baseline's latest event is one, sweep 839) but the `type=` filter validator only admits Render's closed vocabulary, so the extension events can be listed yet never filtered for.
- This is the recorded [w8/060](../../done/060.md) drift (extension types outside Render's closed enum), re-confirmed at pin `3ede47d3c5a9` — not refiled.
