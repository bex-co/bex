# CLI sweep 1133 — CLI reads/deploys track REST lifecycle (live, 2026-10-20)

One disposable free web service created via REST and deleted (204). No new bug filed.

- `bex services -o json` shows the REST-created fixture immediately; `bex deploys list <id> -o json` shows its initial deploy (`status: created`, `trigger: other`, image ref matching the create payload).
- `bex restart srv-000…` (unknown id) exits 1 with `received response code 404 (NOT_FOUND)` — the CLI surfaces the API's coded error rather than inventing its own.
- After REST DELETE, the fixture no longer appears in `bex services` output.

CLI, REST, and GraphQL all observe the same lifecycle with no caching lag.
