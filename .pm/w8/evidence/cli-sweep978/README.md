# CLI sweep 978 — static create path + w8/137 spot check (live, 2026-10-20)

No fixture survived (the one create attempt was refused; nothing was created). No new bug filed.

- REST create with `runtime: "static"` is refused by name (`unsupported runtime "static"`) — the CLI recipe from sweep 780 remains the supported static-site create path, as recorded in sweep 801.
- [w8/137](../../137.md) spot check at pin `3ede47d3c5a9`: single-rule `POST …/headers` and `POST …/routes` still → 405. The fix on `origin/main` is not pinned; the finding stays open.
