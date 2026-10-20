# CLI sweep 754 — sandbox synthetic-target dialects (live, 2026-10-19)

No sandbox was created (creation spins real sandbox capacity; only refusal paths were probed).

- `ea sandboxes exec sbx-00000000000000000000` → 404 `SANDBOX_NOT_FOUND: sandbox not found`; `stop` on the same id → the identical coded refusal. The sandbox API resolves targets server-side with a dedicated error code.
- `snapshots list <id>` → CLI `unknown command` (its argument shape differs; not chased).

No bug filed.
