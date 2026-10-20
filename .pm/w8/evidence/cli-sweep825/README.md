# CLI sweep 825 — CLI datastore commands (live, 2026-10-20)

Read-only CLI probes; baseline only read. No new bug filed.

- `bex postgres list -o json` → the canary baseline wrapped in the CLI's `{"data":[…]}` envelope with the full field set (`connectionPool: "none"`, dashboardUrl, ipAllowList, …); `postgres get <baseline-id>` → the same object singly.
- `bex keyvalues list -o json` → `{"data":[]}` (canary has no KV after sweep 810's cleanup).
- `postgres get dpg-000…` → a clean named error. One upstream-text note: the message advises `render workspace set …` — verbatim pinned `render-oss/cli` wording passing through the bex launcher, recorded here rather than filed (the launcher pins upstream text; it is not a bex-api behavior).
