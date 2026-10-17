# w8 · m87 — Make psql Ctrl+C stop external PostgreSQL queries

**Worker:** worker8 **Goal:** Ordinary external psql cancellation stops the actual query promptly and preserves its session. **Status:** blocked — t001 done ([design](design.md)); t002–t008 need the user to accept TLS termination at the shared PostgreSQL ingress

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Specify the shared-endpoint cancellation mediation design](done/t001.md) | 45m | — — **DONE 2026-10-16** |
| t002 | [Implement startup cancellation identity and exact backend dispatch](t002.md) | 90m | t001 |
| t003 | [Carry cancellation ownership across proxy instances and lifetimes](t003.md) | 60m | t002 |
| t004 | [Preserve certificate projection and every PostgreSQL endpoint alias](t004.md) | 60m | t003 |
| t005 | [Verify Render parity and connection-info consumers](t005.md) | 45m | t004 |
| t006 | [Simplify the changed PostgreSQL mediation path](t006.md) | 15m | t005 |
| t007 | [Test real cancellation across protocol and connection neighbors](t007.md) | 60m | t005 |
| t008 | [Verify fresh native Ctrl+C behavior and close the milestone](t008.md) | 60m | t006, t007 |

**Total:**8 tasks/435m. [Researched finding](finding.md) · [full native evidence](evidence/cli-sweep365/README.md).

## Definition of done

Fresh published Bex and unchanged pinned Render on the served fix open ordinary psql against an owned explicit Free PG18. An independent own-session observer establishes active PgSleep; real terminal Ctrl+C produces57014 within3s and ends the query before its12s duration, rollback and same PID/connection reuse succeed, and both native parents quit naturally0. The newer libpq cancellation still works. Local shared-ingress tests cover both protocol forms, cross-instance routing, exact upstream-server selection, alias/certificate/lifecycle neighbors and required suites. Connection-info/trust consumers remain coherent across REST/GraphQL/MCP/dashboard. Every owned resource, local process/config/history/cached credential is cleaned; original baseline and quotas are preserved. Unexercised modes/versions are explicit. Completion includes all standing tasks and real done-folder moves; dispatch text or implementation alone is insufficient.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI QA sweep365,2026-10-16; [two native settled failures, local control and modern cancellation control](finding.md).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [ADR018](../../../../docs/ADR018-render-parity.md) and [managed PostgreSQL ADR009](../../../../docs/ADR009-postgresql-management.md): supported interactive database journeys must behave correctly with the existing client contract.
- **Expected outcome:** Ctrl+C ends the chosen active statement promptly while keeping the SQL session usable on the shared external endpoint.
- **Why now:** Both normal clients report a cancellation dispatch while their actual queries run to completion. A documented modern API succeeds on the same database, isolating a concrete legacy ingress gap. The shared TLS passthrough cannot recover the missing route with a one-line header change, so this needs a reviewed design plus multi-instance and consumer verification.
- **Render parity:** Included for tenant-facing cancellation and all exposed connection-info/trust consumers. Eight tasks todo; scheduling only; no product implementation or new ship.
