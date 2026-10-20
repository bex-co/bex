# CLI sweep 911 — open-findings spot-checks at pin `3ede47d3c5a9` (live, 2026-10-20)

One disposable service created for the 138 probe and deleted (204). No new bug filed.

- w8/137: `POST /services/{synthetic}/headers` → 405 (fix unpinned).
- w8/138: override GET keys still `[notificationsToSend, previewNotificationsEnabled]` — no `serviceId` (fix unpinned).
- w8/139: REST KV hyphen policy → 400 (no fix yet).
- Pin unchanged (`414a5d4b9` → `3ede47d3c5a9`); all three findings stay open with their pre-fix answers recorded for diffing at closure.
