# CLI sweep 730 — log streaming and text/status filters (live, 2026-10-19)

Read-only against the baseline `hello-go`.

- `bex logs --tail` in non-interactive mode does **not** refuse (contrast the ssh/kv-cli interactive guards, sweep 683): it opens the subscribe stream and waits silently for new entries; the probe was terminated by the harness after ~40s with no output and no error. Streaming is the pinned CLI's subscribe path, usable only with an external timeout in scripts.
- `--text GET` returned matching app log entries; `--status-code 200` returned request-log entries.
- `--status-code abc` → an empty page (permissive match-none, like sweep 694's REST level filter), not a client enum refusal.

No bug filed.
