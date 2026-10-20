# CLI sweep 999 — workflows CLI on an empty workspace (live, 2026-10-20)

CLI probes only; nothing created. No new bug filed.

- `bex workflows list -o json` on a workspace with no workflow services prints `null` — the same pinned-CLI empty-list shape recorded for `projects` in sweep 972 (the CLI binary is the oracle for its own output format). `workflows --help` documents the full subcommand family (`create`, `tasks`, `versions`, `dev`, `init`).
- No workflow fixture was created: workflow services are a heavier surface than this rotation slot calls for, and the empty-state behavior is the observation.
