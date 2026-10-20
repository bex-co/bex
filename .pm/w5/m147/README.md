# w5 · m147 — Web and private services get Render's `WEB_CONCURRENCY` for their plan

**Worker:** worker5 **Goal:** a web or private service sees Render's plan-derived `WEB_CONCURRENCY` at runtime, so Gunicorn, Uvicorn and Puma run as many workers as on Render, and a service's own value still wins. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's rule and decide its scope | 20m | — |
| t002 | The operator sets it at runtime, yielding to the service's own value | 1h | t001 |
| t003 | Plan changes roll it out; cutoff and override envtests | 45m | t002 |
| t004 | dev-5: a Gunicorn app on a 2-CPU plan runs 2 workers | 30m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

Total: **7 tasks, about 3h 50m**.

## Definition of done

- On dev-5, a new web service on a 2-CPU plan sees `WEB_CONCURRENCY=2` and Gunicorn starts 2 workers.
- A user-set value overrides it; build steps do not see it.

## Source + Goal linkage

- **Source:** third `/pm-brainstorm for w5` (2026-10-20), re-offered as the parity-only set and chosen by the user the same day. bex sets `WEB_CONCURRENCY` nowhere in `lego/`; Gunicorn, Uvicorn and Rails' generated `puma.rb` read it, and plans reach 8 CPU (`tiers.yaml:59-74`), so a migrated paid-plan app runs 1 worker where Render runs 2–8.
- **Goal linkage:** ADR060 runtime defaults (the w8/111–117 lineage, w5/162); Render parity.
- **Expected outcome:** paid-plan apps use the CPU they pay for without a custom start command.
- **Why now:** it finishes the runtime-defaults series w5/162 just extended; silent underuse of paid CPU is the worst way to find out.
- **Render parity:** omitted: runtime environment only; Render does not show it in the env editor, and no REST/GraphQL/MCP/UI field changes.
