# CLI sweep 814 — CLI pass-through surface notes (live, 2026-10-20)

Read-only CLI probes. No new bug filed.

- `bex whoami -o json` → the QA identity (`name:""`, the documented pre-trait degradation also seen in sweep 807's `GET /users`).
- `bex bogus814` → `Error: unknown command "bogus814"` with usage hint, exit non-zero — pinned upstream CLI behavior through the bex launcher.
- `bex services get srv-000…` is **not** a pinned-CLI subcommand (`services` has create/delete/instances/update); the upstream CLI treats the invocation as a list and returns the workspace page. Recorded as launcher pass-through parity, not a bex defect — the REST single-get contract was already verified directly (404 coded, sweep 800-era probes).
