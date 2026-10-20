# CLI sweep 661 — log query validation (live, 2026-10-19)

Read-only against the baseline `hello-go` service; nothing mutated. (The `task`/`taskRun`/`sandbox` filter family is w8/132, fixed on main and awaiting the production pin — not re-probed here.)

Via the pinned CLI (`bex logs -r srv-daif6dsmg29s73d1umvg`):

| Probe | Result |
| --- | --- |
| `--limit 3 -o json` | 200, log entries with labels |
| `--direction sideways` | client enum refusal (`backward`, `forward`) |
| `--limit 99999` | server 400 `limit must be an integer between 1 and 100` |
| `--start not-a-time` | client refusal naming relative/RFC3339 formats |
| `--start 2027… --end 2026…` | server 400 `startTime must be before or equal to endTime` |
| synthetic resource id | client `no resource found with ID or name '…'` |
| `--level bogus` | client enum refusal listing the eight levels |

One wart recorded, not filed: `--limit 0` is silently treated as unset (default page returned) instead of the server's named 1–100 refusal — the pinned CLI's integer flag cannot distinguish 0 from absent, matching upstream flag semantics.

No bug filed.
