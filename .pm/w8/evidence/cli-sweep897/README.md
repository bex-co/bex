# CLI sweep 897 — immutable identity fields + new pin lands (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PATCH {"ownerId": <other workspace>}`, `{"id": …}`, and `{"type":"private_service"}` → 400 unknown-field refusals; the owner read-back afterwards is unchanged. Identity, ownership, and type are create-time facts, not PATCH surface.
- **New production pin**: `414a5d4b9 chore(deploy): pin platform images to 3ede47d3c5a9` (03:15Z). Ancestry check: contains the w8/135 fix (`410b9ae7b`), not the w8/137 (`e8d28e88e`) or w8/138 (`d98da2870`) fixes. The 135 live acceptance was retested twice within ~5 minutes of the pin and still returns the old `400 invalid request body at /registry` — the fix commit's own acceptance matches the probe exactly, so the rollout had not completed; 135 stays open and will be retested in a later rotation before any closure.
