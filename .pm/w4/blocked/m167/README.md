# w4 · m167 — Loop5: redact secrets from Postgres Insights query text

**Worker:** worker4 **Goal:** Database Insights never publishes credential material — no password verifier or secret literal reaches any surface **Status:** blocked — t001–t004 done 2026-10-04; t005 live closeout remains

## Tasks (in order)

| id   | title                                              | est | depends_on |
| ---- | -------------------------------------------------- | --- | ---------- |
| t001 — **DONE** | Redact secret-bearing statements in Insights reads | 1h  | —          |
| t002 — **DONE** | Render parity across the Insights surfaces         | 30m | w4/m167/t001 |
| t003 — **DONE** | Simplify the touched backend code                  | 30m | w4/m167/t002 |
| t004 — **DONE** | Test coverage for Insights redaction               | 1h  | w4/m167/t002 |
| t005 | Closeout                                           | 15m | w4/m167/t004 |

## Definition of done

- Top Queries and Active Processes redact statements carrying secrets (at minimum `PASSWORD`/`ENCRYPTED PASSWORD` DDL verifiers) before they leave `bex-api`, on REST, GraphQL, and MCP alike.
- Ordinary query text (SELECT/INSERT/DDL without secrets) still renders verbatim; the existing `<insufficient privilege>` masking still holds.
- No dashboard change required beyond what the redacted payload already yields.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop5 2026-10-04 UTC (muse.env). Live evidence: fresh database `dpg-…9scg` (since deleted) showed `ALTER ROLE "dpg_…_user" WITH PASSWORD 'SCRAM-SHA-256$4096:…'` verbatim in Insights → Top queries — the user's SCRAM verifier, which enables offline brute-force of the database password, visible to every Insights reader.
- **Goal linkage:** tenant-data security (auth/credential hygiene); a credential leak in a routine observability table.
- **Expected outcome:** Insights query text is safe to show: secrets redacted at the read, on every surface.
- **Why now:** the leak is live on every fresh database (provisioning runs the `ALTER ROLE` itself, so the verifier sits in `pg_stat_statements` from minute one). Render parity task included (t002): user-facing observability surface with REST/GraphQL/MCP equivalents to check.
