# CLI sweep 1080 — CLI logs read and unknown-resource error (live, 2026-10-20)

Read-only CLI probes on the baseline `hello-go` service. No new bug filed.

- `bex logs -r {hello-go} --limit 3 -o json` returns labeled log entries (type/resource/instance/container labels) matching the REST log shape from sweep 961.
- An unknown resource id fails client-side with a precise message: `no resource found with ID or name 'srv-000…'`. The macOS harness lacks `timeout(1)`; the non-streaming `--limit` form was used instead of `--tail`.
