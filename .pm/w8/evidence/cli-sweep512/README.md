# CLI sweep 512 — Postgres read shapes

**Verdict: one minor bug filed** as [w8/129](../../129.md) (Postgres reads omit the Render-required `suspenders` and `role`). Same isolated session, `bex-canary`, HEAD build `c603c8ec2`, 2026-10-19T09:12–09:15Z. Read-only: the one baseline database was only ever read, never mutated, and no connection credentials were printed or recorded.

| probe | observed result |
| --- | --- |
| `postgres list -o json` | The single baseline database (`available`, `free`, v14). |
| `postgres get <dpg-id>` and by name, `-o json`/`-o text` | Both selectors resolve to the same record; text rendering is truthful (workspace name+id, plan, version, region, disk/HA states). |
| Field-level diff vs Render's vendored schema | Raw `GET /v1/postgres/{id}` omits three `postgresDetail.required` keys: `project`, `role`, `suspenders`. `suspenders`/`role` are values bex already knows (filed as w8/129 — the gap w6/m109 flagged and left unowned; services fixed the same field in w4/014). `project` is a separately allowlisted shape divergence, not filed here. The CLI surface shows the gap as `"suspenders": null`. |
| Everything else in the payload | `readReplicas: []`, `ipAllowList` serialized — the w6/m109 always-serialize rule holds for the fields it covered. |

## Safety and unrun checks

- Read-only sweep: no suspend/resume/update/delete ran against the pre-existing database, and secret-bearing connection-info commands were not run against it. The suspended-state `["user"]` leg of w8/129's acceptance needs an owned disposable database when the fix is implemented (the canary free-Postgres slot is held by this baseline database during the hunt).
- The filing in w8/129 is uncommitted (no `/ship` authorization in this loop request).
- Deploy lag means deployed revisions can precede local `main`; the missing fields are absent from HEAD's `PostgresView` struct, so lag does not explain them.
