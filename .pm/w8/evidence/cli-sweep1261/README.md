# CLI sweep 1261 — output-format matrix across read-only commands (live, 2026-10-20)

Read-only. Metadata reads of the baseline service and Postgres only; nothing was created or changed. No new bug filed.

- `services`, `deploys list`, `services instances`, `pg list`, `kv list`, `projects`, `workspaces`, `workspace current`, `whoami` and `pg get`: all exit 0. `-o json` parses as JSON, `-o yaml` parses as YAML, and `-o text` prints non-empty tables.
- `logs -o json` prints one indented JSON object per entry, concatenated rather than a JSON array, so a single `json.loads` fails with “Extra data”. This is the pinned client's own streaming writer (`cmd/logs.go:109–116`, `writeLog` → `json.MarshalIndent` per log), identical in Render's CLI, so it is an upstream format choice and not filed. `-o yaml` for logs parses as a multi-document stream.
