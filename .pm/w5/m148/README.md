# w5 · m148 — Services see Render's platform environment variables (`RENDER_*`)

**Worker:** worker5 **Goal:** a service on bex sees the `RENDER_*` variables Render documents, so Render's framework guides (Django `ALLOWED_HOSTS`, Rails hosts, commit-stamped builds) work unchanged. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's platform variable set and amend the "narrower set" decision | 40m | — |
| t002 | The operator injects the runtime set, yielding to the service's own values | 1h30m | t001 |
| t003 | Native and Docker builds see the build-phase subset | 1h | t001 |
| t004 | Blueprint and env writes accept a user-set RENDER_* where Render does | 45m | t001 |
| t005 | Envtests and a dev-5 Django app reading RENDER_EXTERNAL_HOSTNAME | 1h | t002, t003 |
| t006 | Simplify | 20m | t004, t005 |
| t007 | Test coverage | 45m | t004, t005, t006 |
| t008 | Closeout | 10m | t007 |

Total: **8 tasks, about 6h10m**.

## Definition of done

- A new web service sees `RENDER_EXTERNAL_URL` equal to its public URL and `RENDER_GIT_COMMIT` equal to the deployed commit; a service's own value wins.
- The pinned set (phase and source per variable) is recorded in ADR018.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); w4/m123 t005 chose a narrower set than Render.
- **Goal linkage:** ADR008 Render parity first; ADR004 runtime env.
- **Expected outcome:** apps that read Render's platform variables migrate without edits.
- **Why now:** the largest silent runtime gap left after m147's `WEB_CONCURRENCY`, and the yielding mechanism is fresh.
- **Render parity:** omitted: runtime/build environment only; Render does not list these in the env editor and no REST/GraphQL/MCP/UI field changes (t004 aligns write-path validation, covered by its own tests).
