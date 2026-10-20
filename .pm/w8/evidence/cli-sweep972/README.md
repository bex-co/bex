# CLI sweep 972 — pinned CLI read commands (live, 2026-10-20)

CLI probes through the isolated QA config against `bex-canary`. No fixture created. No new bug filed.

- `whoami`, `workspaces -o json` (all three of the user's workspaces), `services -o json`, and `deploys list {hello-go} -o json` all return well-formed data.
- Unknown commands fail cleanly (`unknown command … Run 'bex --help'`); `environments {unknown-project}` surfaces the API's coded 404 through the CLI (`received response code 404 (NOT_FOUND): project not found`).
- `projects -o json` on the now-empty workspace prints `null` rather than `[]` (table mode prints just the header). The CLI is the pinned Render CLI under the bex launcher, so its own output shape is the oracle here; a `null` empty list is recorded as an upstream-shaped wart, not filed.
